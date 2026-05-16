package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
)

// safeRecord fires-and-forgets a usage event. Safe to call when Billing is
// nil (local dev / pre-PR2 wiring) or the request has no authenticated user.
// Returns no error — losing one usage row is strictly better than failing
// the user's request because of a metrics write.
func safeRecord(ctx context.Context, b *billing.Service, kind billing.Kind, meta map[string]any) {
	if b == nil {
		return
	}
	uid, ok := auth.UserID(ctx)
	if !ok || uid == "" {
		return
	}
	_ = b.Record(ctx, uid, kind, meta)
}

// checkQuota runs Billing.Check for the caller and writes a structured
// response on failure. Returns true when the caller is allowed to proceed.
//
// On ErrPlanRequired writes 403 {error:"plan_required", kind, requiredPlan:"pro"}.
// On ErrQuotaExceeded writes 402 {error:"quota_exceeded", kind}. Both shapes
// are what the frontend paywall interceptor watches for. Other errors are
// logged-and-allowed: a billing-service blip must not lock users out.
func checkQuota(w http.ResponseWriter, r *http.Request, b *billing.Service, kind billing.Kind) bool {
	if b == nil {
		return true
	}
	uid, ok := auth.UserID(r.Context())
	if !ok || uid == "" {
		return true
	}
	plan := auth.PlanFromContext(r.Context())
	err := b.Check(r.Context(), uid, plan, kind)
	if err == nil {
		return true
	}
	if errors.Is(err, billing.ErrPlanRequired) {
		response.JSON(w, http.StatusForbidden, map[string]any{
			"error":        "plan_required",
			"kind":         string(kind),
			"requiredPlan": "pro",
		})
		return false
	}
	if errors.Is(err, billing.ErrQuotaExceeded) {
		response.JSON(w, http.StatusPaymentRequired, map[string]any{
			"error": "quota_exceeded",
			"kind":  string(kind),
			"plan":  plan,
		})
		return false
	}
	// Unexpected DB error — fail open so the user can still finish their task.
	// We still log it (the caller logs the result), but don't punish them.
	return true
}
