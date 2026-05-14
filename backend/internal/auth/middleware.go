package auth

import (
	"context"
	"net/http"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxEmail
)

// Authenticator returns chi-compatible middleware that requires a valid access JWT in the cookie.
// On success the user id is stored in the request context and accessible via UserID(ctx).
func Authenticator(tm *TokenManager) func(http.Handler) http.Handler {
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
