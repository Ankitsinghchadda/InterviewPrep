package handlers

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
)

type AuthHandler struct {
	Service     *auth.Service
	Users       *repository.UserRepo
	Cookies     auth.CookieConfig
	FrontendURL string
	DefaultPath string
	AdminEmails map[string]struct{}
}

// Login starts the Google OAuth flow. Sets a state cookie + optional post-login redirect cookie,
// then 302s to Google.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, url, err := h.Service.BuildLoginURL()
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to start oauth")
		return
	}
	h.Cookies.SetStateCookie(w, state)

	if redirect := r.URL.Query().Get("redirect"); isSafeRedirect(redirect) {
		h.Cookies.SetRedirectCookie(w, redirect)
	}

	http.Redirect(w, r, url, http.StatusFound)
}

// Callback verifies state, exchanges code, issues a session, then redirects back to the frontend.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if errMsg := q.Get("error"); errMsg != "" {
		h.failRedirect(w, r, errMsg)
		return
	}

	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		h.failRedirect(w, r, "missing_params")
		return
	}

	stateCookie, err := r.Cookie(auth.StateCookieName)
	if err != nil || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		h.failRedirect(w, r, "invalid_state")
		return
	}

	user, err := h.Service.ExchangeCode(r.Context(), code)
	if err != nil {
		log.Printf("oauth exchange failed: %v", err)
		h.failRedirect(w, r, "exchange_failed")
		return
	}

	access, refresh, err := h.Service.IssueSession(r.Context(), user, r.UserAgent(), clientIP(r))
	if err != nil {
		log.Printf("issue session failed: %v", err)
		h.failRedirect(w, r, "session_failed")
		return
	}

	tm := h.Service.Tokens()
	h.Cookies.SetAccessCookie(w, access, tm.AccessTTL())
	h.Cookies.SetRefreshCookie(w, refresh, tm.RefreshTTL())

	// Consume redirect cookie.
	redirect := h.DefaultPath
	if c, err := r.Cookie(auth.RedirectCookie); err == nil && isSafeRedirect(c.Value) {
		redirect = c.Value
	}
	http.SetCookie(w, &http.Cookie{Name: auth.StateCookieName, Value: "", Path: "/auth", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: auth.RedirectCookie, Value: "", Path: "/auth", MaxAge: -1})

	http.Redirect(w, r, h.FrontendURL+redirect, http.StatusFound)
}

// Refresh rotates the refresh token and returns a new access cookie.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(auth.RefreshCookieName)
	if err != nil || c.Value == "" {
		response.Err(w, http.StatusUnauthorized, "no refresh token")
		return
	}
	access, newRefresh, user, err := h.Service.Refresh(r.Context(), c.Value, r.UserAgent(), clientIP(r))
	if err != nil {
		log.Printf("refresh failed: %v", err)
		h.Cookies.ClearAuthCookies(w)
		switch {
		case errors.Is(err, auth.ErrTokenReused):
			response.Err(w, http.StatusUnauthorized, "session terminated")
		default:
			response.Err(w, http.StatusUnauthorized, "invalid refresh token")
		}
		return
	}
	tm := h.Service.Tokens()
	h.Cookies.SetAccessCookie(w, access, tm.AccessTTL())
	h.Cookies.SetRefreshCookie(w, newRefresh, tm.RefreshTTL())
	response.OK(w, http.StatusOK, user)
}

// Logout revokes the refresh token and clears cookies.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.RefreshCookieName); err == nil && c.Value != "" {
		_ = h.Service.Revoke(r.Context(), c.Value)
	}
	h.Cookies.ClearAuthCookies(w)
	response.OK(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Me returns the authenticated user with an `isAdmin` flag derived from the
// ADMIN_EMAILS allow-list. The flag is informational only — the backend still
// enforces admin gating on every admin route.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.Users.GetByID(r.Context(), uid)
	if err != nil {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	response.OK(w, http.StatusOK, struct {
		*models.User
		IsAdmin bool `json:"isAdmin"`
	}{
		User:    user,
		IsAdmin: auth.IsAdmin(r.Context(), h.AdminEmails),
	})
}

func (h *AuthHandler) failRedirect(w http.ResponseWriter, r *http.Request, reason string) {
	h.Cookies.ClearAuthCookies(w)
	u, _ := url.Parse(h.FrontendURL + "/login")
	q := u.Query()
	q.Set("error", reason)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// isSafeRedirect rejects absolute URLs and protocol-relative URLs, allowing only same-app paths.
func isSafeRedirect(p string) bool {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.Index(fwd, ","); i > 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	return r.RemoteAddr
}
