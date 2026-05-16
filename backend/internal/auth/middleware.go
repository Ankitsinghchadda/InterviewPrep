package auth

import (
	"context"
	"net/http"
	"time"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxEmail
)

// PlanLoader is the interface the authenticator uses to enrich the request
// context with the caller's billing tier. UserRepo satisfies this.
type PlanLoader interface {
	LoadPlan(ctx context.Context, userID string) (plan string, expiresAt *time.Time, err error)
}

// Authenticator returns chi-compatible middleware that requires a valid access JWT in the cookie.
// On success the user id is stored in the request context and accessible via UserID(ctx).
//
// When plans is non-nil, the user's plan and expiry are also loaded from
// the database and stamped onto the context (PlanFromContext). Expired Pro
// users are downgraded in-flight to free so the agent router and quota
// service see the correct tier without waiting for a cron job. Plan-load
// failures are non-fatal: we fall through with PlanFree so a transient DB
// blip doesn't lock everyone out.
//
// Admins (emails in the admins allow-list) are treated as Pro regardless
// of the stored plan, so the team can exercise paid features without
// running through Razorpay. Pass nil for admins to disable the override.
func Authenticator(tm *TokenManager, plans PlanLoader, admins map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(AccessCookieName)
			if err != nil || c.Value == "" {
				unauthorized(w)
				return
			}
			claims, err := tm.ParseAccess(c.Value)
			if err != nil {
				unauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxEmail, claims.Email)
			if plans != nil {
				if plan, expiresAt, err := plans.LoadPlan(ctx, claims.UserID); err == nil {
					if plan == PlanPro && expiresAt != nil && time.Now().After(*expiresAt) {
						plan = PlanFree
					}
					if IsAdmin(ctx, admins) {
						plan = PlanPro
					}
					ctx = WithPlan(ctx, plan)
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

func UserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxUserID).(string)
	return v, ok
}

func Email(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxEmail).(string)
	return v, ok
}
