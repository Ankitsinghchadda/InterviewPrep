package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	rzp "github.com/Ankitsinghchadda/InterviewPrep/internal/services/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
)

// BillingHandler owns the read side of the tier system (plan + usage) and
// the Razorpay payment flows (checkout, cancel, webhook).
type BillingHandler struct {
	Users    *repository.UserRepo
	Payments *repository.PaymentEventRepo
	Billing  *billing.Service
	Razorpay *rzp.Service // nil disables /checkout and /cancel; webhook route is unmounted

	// Razorpay plan ids (configured in Razorpay dashboard). Empty values
	// make the matching plan unavailable and /checkout returns 400.
	PlanMonthlyID  string
	PlanBiannualID string

	// AdminEmails mirrors the global allow-list; users in it see the
	// Pro snapshot (unlimited quotas, no expiry) regardless of the
	// stored plan, matching the override applied in auth middleware.
	AdminEmails map[string]struct{}
}

// Usage returns a snapshot the frontend uses to render the usage widget
// and the paywall modal. One indexed COUNT...GROUP BY query backs it.
func (h *BillingHandler) Usage(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.Users.GetByID(r.Context(), uid)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "load user")
		return
	}
	plan, expires := u.Plan, u.PlanExpiresAt
	if auth.IsAdmin(r.Context(), h.AdminEmails) {
		plan, expires = auth.PlanPro, nil
	}
	snap, err := h.Billing.Snapshot(r.Context(), uid, plan, expires)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "snapshot usage")
		return
	}
	response.OK(w, http.StatusOK, snap)
}

// Plans returns the public price list. Static for now — moves to DB-backed
// pricing only when we start running price experiments.
//
// Razorpay charges INR (priceINR is the actual billed amount). priceUSD is
// display-only for non-IN visitors; the customer's bank does the FX. If we
// later enable multi-currency on Subscriptions, this endpoint will branch
// on the requesting country to pick the right plan_id.
func (h *BillingHandler) Plans(w http.ResponseWriter, _ *http.Request) {
	response.OK(w, http.StatusOK, map[string]any{
		"billingCurrency": "INR",
		"plans": []map[string]any{
			{
				"id":             "monthly",
				"label":          "Pro Monthly",
				"priceINR":       2075,
				"priceUSD":       25,
				"intervalMonths": 1,
			},
			{
				"id":             "biannual",
				"label":          "Pro 6-month",
				"priceINR":       8300,
				"priceUSD":       100,
				"intervalMonths": 6,
				"savingsPct":     33,
			},
		},
	})
}

// ----------------------------------------------------------------------
// Razorpay Subscriptions
// ----------------------------------------------------------------------

type checkoutBody struct {
	Plan string `json:"plan"` // 'monthly' | 'biannual'
}

type checkoutResponse struct {
	SubscriptionID string `json:"subscriptionId"`
	KeyID          string `json:"keyId"`
	Plan           string `json:"plan"`
}

// Checkout creates a Razorpay Subscription for the requested tier and
// returns the subscription_id + public key the frontend Checkout.js
// needs. The actual upgrade only happens when subscription.activated
// lands on our webhook — never mark the user pro from this handler.
func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	if h.Razorpay == nil {
		response.Err(w, http.StatusServiceUnavailable, "payments_not_configured")
		return
	}
	uid, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body checkoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var planID string
	var totalCount int
	switch body.Plan {
	case "monthly":
		planID, totalCount = h.PlanMonthlyID, 12 // 12 monthly billings = 1y of commitment
	case "biannual":
		planID, totalCount = h.PlanBiannualID, 2 // 2 charges, 6 months apart
	default:
		response.Err(w, http.StatusBadRequest, "plan must be 'monthly' or 'biannual'")
		return
	}
	if planID == "" {
		response.Err(w, http.StatusServiceUnavailable, "plan_id not configured")
		return
	}

	sub, err := h.Razorpay.CreateSubscription(rzp.CreateSubscriptionInput{
		PlanID:     planID,
		TotalCount: totalCount,
		Notes: map[string]string{
			"user_id": uid,
			"plan":    body.Plan,
		},
	})
	if err != nil {
		log.Printf("razorpay checkout: %v", err)
		response.Err(w, http.StatusBadGateway, "failed to start checkout")
		return
	}

	// Stash the subscription id on the user row so a later cancel knows
	// what to talk to Razorpay about even before the activation webhook.
	if err := h.Users.UpgradeToPro(r.Context(), uid, body.Plan, sub.ID, time.Now().Add(24*time.Hour)); err != nil {
		// Non-fatal: the activation webhook will set the correct
		// plan/expiry. We just won't have the sub-id cached for an early
		// cancel.
		log.Printf("razorpay checkout: pre-record subscription failed: %v", err)
	}

	response.OK(w, http.StatusOK, checkoutResponse{
		SubscriptionID: sub.ID,
		KeyID:          h.Razorpay.KeyID(),
		Plan:           body.Plan,
	})
}

// Cancel marks the user's subscription cancel_at_cycle_end with
// Razorpay. Access continues through plan_expires_at — the actual
// downgrade happens on the cancellation/completion webhook (or on the
// next authenticated request after expiry, see auth/middleware.go).
func (h *BillingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if h.Razorpay == nil {
		response.Err(w, http.StatusServiceUnavailable, "payments_not_configured")
		return
	}
	uid, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.Users.GetByID(r.Context(), uid)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "load user")
		return
	}
	if u.RazorpaySubscriptionID == "" {
		response.Err(w, http.StatusBadRequest, "no active subscription")
		return
	}
	if err := h.Razorpay.CancelSubscription(u.RazorpaySubscriptionID, false); err != nil {
		log.Printf("razorpay cancel: %v", err)
		response.Err(w, http.StatusBadGateway, "failed to cancel")
		return
	}
	response.OK(w, http.StatusOK, map[string]any{
		"cancelled":     true,
		"accessUntil":   u.PlanExpiresAt,
		"subscriptionId": u.RazorpaySubscriptionID,
	})
}

// ----------------------------------------------------------------------
// Razorpay webhooks
// ----------------------------------------------------------------------

// webhookEnvelope is the slice of the Razorpay webhook payload we
// actually read. The full body lands in payment_events.payload — this
// struct is only for routing logic.
type webhookEnvelope struct {
	Event   string `json:"event"` // e.g. 'subscription.activated'
	Payload struct {
		Subscription struct {
			Entity struct {
				ID         string  `json:"id"`
				Status     string  `json:"status"`
				CurrentEnd int64   `json:"current_end"` // unix seconds
				PlanID     string  `json:"plan_id"`
				Notes      map[string]any `json:"notes"`
			} `json:"entity"`
		} `json:"subscription"`
		Payment struct {
			Entity struct {
				ID       string `json:"id"`
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
	CreatedAt int64 `json:"created_at"`
	ID        string `json:"id"`
}

// Webhook handles every Razorpay event we care about. It is mounted
// OUTSIDE the auth-required tree (no cookie sent by Razorpay) — the
// only auth is the HMAC signature.
//
// Idempotency: we use Razorpay's `x-razorpay-event-id` header as the
// unique key on payment_events. Replays return 200 without side
// effects.
func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	if h.Razorpay == nil {
		response.Err(w, http.StatusServiceUnavailable, "payments_not_configured")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 256*1024))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "read body")
		return
	}

	sig := r.Header.Get("X-Razorpay-Signature")
	if !h.Razorpay.VerifyWebhookSignature(body, sig) {
		log.Printf("razorpay webhook: signature mismatch")
		response.Err(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	var env webhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Razorpay sends an event id on a header; some events don't carry
	// one and we fall back to the body's `id` field. Either way the
	// pair (provider, provider_event_id) must be unique in our DB.
	eventID := r.Header.Get("X-Razorpay-Event-Id")
	if eventID == "" {
		eventID = env.ID
	}
	if eventID == "" {
		response.Err(w, http.StatusBadRequest, "missing event id")
		return
	}

	sub := env.Payload.Subscription.Entity
	userID := ""
	if v, ok := sub.Notes["user_id"].(string); ok {
		userID = v
	}

	// Record first so replays are short-circuited at the DB layer.
	err = h.Payments.Insert(r.Context(), repository.InsertPaymentEventInput{
		UserID:          userID,
		Provider:        "razorpay",
		ProviderEventID: eventID,
		EventType:       env.Event,
		Amount:          env.Payload.Payment.Entity.Amount,
		Currency:        env.Payload.Payment.Entity.Currency,
		Payload:         body,
	})
	if errors.Is(err, repository.ErrDuplicateEvent) {
		// Replay — already processed. Tell Razorpay to stop retrying.
		response.OK(w, http.StatusOK, map[string]any{"duplicate": true})
		return
	}
	if err != nil {
		log.Printf("razorpay webhook: persist event %s failed: %v", eventID, err)
		// Return 500 so Razorpay retries — better to double-process
		// than to lose the event entirely.
		response.Err(w, http.StatusInternalServerError, "persist failed")
		return
	}

	// Dispatch by event type. Anything we don't handle is still
	// recorded above for audit but otherwise ignored.
	switch env.Event {
	case "subscription.activated", "subscription.charged":
		if userID == "" {
			break
		}
		// Determine the period from notes when present so the user row
		// reflects the right tier label. Default to monthly.
		period := "monthly"
		if v, ok := sub.Notes["plan"].(string); ok && (v == "monthly" || v == "biannual") {
			period = v
		}
		expiry := time.Unix(sub.CurrentEnd, 0)
		if expiry.IsZero() || expiry.Before(time.Now()) {
			// current_end can be missing on activated; default to one
			// month ahead so the user isn't locked out while we wait
			// for the next charge event.
			expiry = time.Now().Add(31 * 24 * time.Hour)
		}
		if err := h.Users.UpgradeToPro(r.Context(), userID, period, sub.ID, expiry); err != nil {
			log.Printf("razorpay webhook: upgrade user %s failed: %v", userID, err)
		}
	case "subscription.cancelled", "subscription.completed":
		// Don't kick the user out mid-cycle: plan_expires_at is left
		// in place and the auth middleware downgrades on read once it
		// passes. We DO clear the subscription id so future "cancel"
		// clicks know there's nothing left to cancel.
		if userID != "" {
			_, err := h.Users.GetByID(r.Context(), userID)
			if err == nil {
				_ = h.Users.UpgradeToPro(r.Context(), userID, "", "", time.Unix(sub.CurrentEnd, 0))
			}
		}
	case "subscription.halted", "payment.failed":
		// Grace period: take no action immediately. A retry from
		// Razorpay will arrive within ~3 days; if not, the rollover at
		// plan_expires_at downgrades the user automatically.
	default:
		// recorded above; nothing to do
	}

	response.OK(w, http.StatusOK, map[string]any{"received": true})
}
