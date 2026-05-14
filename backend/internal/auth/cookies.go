package auth

import (
	"net/http"
	"time"
)

const (
	AccessCookieName  = "ip_access"
	RefreshCookieName = "ip_refresh"
	StateCookieName   = "ip_oauth_state"
	RedirectCookie    = "ip_oauth_redirect"
)

type CookieConfig struct {
	Domain string
	Secure bool
}

// SetAccessCookie writes the access JWT as HttpOnly. Path "/" so all API routes see it.
func (c CookieConfig) SetAccessCookie(w http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    token,
		Path:     "/",
		Domain:   c.Domain,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// SetRefreshCookie scopes the refresh token cookie to /auth so it isn't sent on every API call.
func (c CookieConfig) SetRefreshCookie(w http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/auth",
		Domain:   c.Domain,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c CookieConfig) SetStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    state,
		Path:     "/auth",
		Domain:   c.Domain,
		MaxAge:   600,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c CookieConfig) SetRedirectCookie(w http.ResponseWriter, redirect string) {
	http.SetCookie(w, &http.Cookie{
		Name:     RedirectCookie,
		Value:    redirect,
		Path:     "/auth",
		Domain:   c.Domain,
		MaxAge:   600,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c CookieConfig) ClearAuthCookies(w http.ResponseWriter) {
	c.clear(w, AccessCookieName, "/")
	c.clear(w, RefreshCookieName, "/auth")
	c.clear(w, StateCookieName, "/auth")
	c.clear(w, RedirectCookie, "/auth")
}

func (c CookieConfig) clear(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		Domain:   c.Domain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
