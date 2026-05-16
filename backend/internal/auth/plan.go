package auth

import "context"

// PlanFree and PlanPro are the two billable tiers. Stored on users.plan and
// loaded into request context by the authenticator middleware.
const (
	PlanFree = "free"
	PlanPro  = "pro"
)

const ctxPlan ctxKey = 100

// WithPlan stamps the user's current plan onto the request context. Returns
// the original context unchanged if plan is empty so callers don't accidentally
// set a zero value that defeats the PlanFree fallback in PlanFromContext.
func WithPlan(ctx context.Context, plan string) context.Context {
	if plan == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxPlan, plan)
}

// PlanFromContext returns the caller's plan, defaulting to PlanFree when no
// plan was stamped on the context (e.g. unauthenticated paths or before the
// tier middleware lands in PR 2).
func PlanFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxPlan).(string); ok && v != "" {
		return v
	}
	return PlanFree
}
