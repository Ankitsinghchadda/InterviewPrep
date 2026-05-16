// Package billing wraps the Razorpay Go SDK with the small surface this
// app actually uses: create a Subscription, cancel a Subscription, and
// verify the HMAC signature on incoming webhooks.
//
// Razorpay's SDK is map[string]interface{}-typed end-to-end. This wrapper
// hides that and returns the few fields we care about. Anything more
// exotic (offers, addons, pause/resume) is a future feature and not
// here.
package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	razorpay "github.com/razorpay/razorpay-go"
)

// Service is a thin wrapper around razorpay.Client + webhook secret.
// Construct one in main.go and inject into handlers.
type Service struct {
	client        *razorpay.Client
	keyID         string
	webhookSecret string
}

// New constructs the wrapper. Returns nil if keyID is empty so callers
// can keep payments off in dev without nil-checking every call site.
// Handlers MUST nil-check this — see handlers/billing.go.
func New(keyID, keySecret, webhookSecret string) *Service {
	if keyID == "" || keySecret == "" {
		return nil
	}
	return &Service{
		client:        razorpay.NewClient(keyID, keySecret),
		keyID:         keyID,
		webhookSecret: webhookSecret,
	}
}

// KeyID returns the public key the frontend Checkout.js needs to load
// the modal. Safe to expose to clients; the secret never leaves the
// server.
func (s *Service) KeyID() string {
	return s.keyID
}

// CreateSubscriptionInput is everything we feed to Razorpay's Subscription
// create endpoint. TotalCount caps the number of billing cycles —
// monthly: 12 (one year up front), biannual: 2 (two 6-month renewals).
// After total_count cycles Razorpay marks the sub "completed" and stops
// charging.
type CreateSubscriptionInput struct {
	PlanID     string
	TotalCount int
	Notes      map[string]string // small metadata, surfaced in webhook payloads
}

// SubscriptionResult is the subset of Razorpay's response handlers care
// about. The full raw map is also returned as Raw for downstream code
// (e.g. webhook handler stash) that wants more fields.
type SubscriptionResult struct {
	ID     string
	Status string
	Raw    map[string]interface{}
}

func (s *Service) CreateSubscription(in CreateSubscriptionInput) (*SubscriptionResult, error) {
	if s == nil {
		return nil, errors.New("razorpay: service not configured")
	}
	if in.PlanID == "" {
		return nil, errors.New("razorpay: plan_id is required")
	}
	if in.TotalCount <= 0 {
		in.TotalCount = 12
	}
	data := map[string]interface{}{
		"plan_id":         in.PlanID,
		"total_count":     in.TotalCount,
		"customer_notify": 1,
	}
	if len(in.Notes) > 0 {
		notes := make(map[string]interface{}, len(in.Notes))
		for k, v := range in.Notes {
			notes[k] = v
		}
		data["notes"] = notes
	}
	resp, err := s.client.Subscription.Create(data, nil)
	if err != nil {
		return nil, fmt.Errorf("razorpay: create subscription: %w", err)
	}
	id, _ := resp["id"].(string)
	status, _ := resp["status"].(string)
	if id == "" {
		return nil, fmt.Errorf("razorpay: create subscription returned no id (raw=%v)", resp)
	}
	return &SubscriptionResult{ID: id, Status: status, Raw: resp}, nil
}

// CancelSubscription cancels at the end of the current billing cycle by
// default — the user keeps access until plan_expires_at. Pass
// immediate=true to terminate access immediately (rare; used only when
// the user opens a chargeback or we're winding down).
func (s *Service) CancelSubscription(subID string, immediate bool) error {
	if s == nil {
		return errors.New("razorpay: service not configured")
	}
	if subID == "" {
		return errors.New("razorpay: subscription_id is required")
	}
	cancelAtCycleEnd := 1
	if immediate {
		cancelAtCycleEnd = 0
	}
	_, err := s.client.Subscription.Cancel(subID, map[string]interface{}{
		"cancel_at_cycle_end": cancelAtCycleEnd,
	}, nil)
	if err != nil {
		return fmt.Errorf("razorpay: cancel subscription %s: %w", subID, err)
	}
	return nil
}

// VerifyWebhookSignature returns true when the X-Razorpay-Signature
// header on a webhook call matches an HMAC-SHA256 of the raw body
// using the webhook secret. Constant-time compare to avoid timing
// oracles. ALL webhook handlers MUST call this before trusting body
// contents — otherwise an attacker can forge subscription.activated.
func (s *Service) VerifyWebhookSignature(rawBody []byte, providedHex string) bool {
	if s == nil || s.webhookSecret == "" || providedHex == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(providedHex))
}
