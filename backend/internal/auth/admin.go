package auth

import (
	"context"
	"net/http"
	"strings"
)

// RequireAdmin gates a route to email addresses listed in ADMIN_EMAILS.
// The set is provided by config and compared case-insensitively against the
// access-token claim.
func RequireAdmin(admins map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAdmin(r.Context(), admins) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"admin only"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsAdmin reports whether the authenticated email (from request context) is in
// the admin allow-list. Returns false when the email is missing or empty.
func IsAdmin(ctx context.Context, admins map[string]struct{}) bool {
	email, ok := Email(ctx)
	if !ok || email == "" {
		return false
	}
	_, ok = admins[strings.ToLower(email)]
	return ok
}
