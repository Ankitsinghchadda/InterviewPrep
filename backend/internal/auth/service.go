package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

var (
	ErrInvalidState  = errors.New("invalid oauth state")
	ErrInvalidToken  = errors.New("invalid refresh token")
	ErrTokenReused   = errors.New("refresh token reuse detected")
	ErrTokenExpired  = errors.New("refresh token expired")
	ErrTokenRevoked  = errors.New("refresh token revoked")
)

// googleUserInfoURL returns OpenID userinfo (verified by virtue of the bearer token).
const googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

type Service struct {
	tokens *TokenManager
	users  *repository.UserRepo
	rts    *repository.RefreshTokenRepo
	oauth  *oauth2.Config
}

func NewService(tokens *TokenManager, users *repository.UserRepo, rts *repository.RefreshTokenRepo, clientID, clientSecret, redirectURL string) *Service {
	return &Service{
		tokens: tokens,
		users:  users,
		rts:    rts,
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     googleoauth.Endpoint,
			Scopes:       []string{"openid", "email", "profile"},
		},
	}
}

func (s *Service) Tokens() *TokenManager { return s.tokens }

// BuildLoginURL returns (state, url). The caller is responsible for setting state as an
// HttpOnly cookie and verifying it on callback (CSRF protection).
func (s *Service) BuildLoginURL() (state, url string, err error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	state = base64.RawURLEncoding.EncodeToString(b)
	url = s.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("prompt", "select_account"))
	return state, url, nil
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// ExchangeCode trades an authorization code for a userinfo profile and upserts the user.
func (s *Service) ExchangeCode(ctx context.Context, code string) (*models.User, error) {
	tok, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}
	client := s.oauth.Client(ctx, tok)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo status %d: %s", resp.StatusCode, string(body))
	}
	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	if info.Sub == "" || info.Email == "" {
		return nil, errors.New("userinfo missing required fields")
	}
	return s.users.UpsertFromGoogle(ctx, repository.GoogleProfile{
		Sub:           info.Sub,
		Email:         info.Email,
		EmailVerified: info.EmailVerified,
		Name:          info.Name,
		PictureURL:    info.Picture,
	})
}

// IssueSession mints an access JWT and a fresh refresh token (persisted as a hash).
func (s *Service) IssueSession(ctx context.Context, user *models.User, userAgent, ip string) (access, refresh string, err error) {
	now := time.Now()
	access, err = s.tokens.MintAccess(user.ID, user.Email, now)
	if err != nil {
		return "", "", err
	}
	raw, hash, err := NewRefreshToken()
	if err != nil {
		return "", "", err
	}
	err = s.rts.Create(ctx, repository.CreateTokenInput{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: hash,
		UserAgent: userAgent,
		IPAddress: ip,
		ExpiresAt: now.Add(s.tokens.RefreshTTL()),
	})
	if err != nil {
		return "", "", err
	}
	return access, raw, nil
}

// Refresh rotates the supplied refresh token, returning a new access+refresh pair.
// Detects reuse: if the supplied token is already revoked, the whole family is killed.
func (s *Service) Refresh(ctx context.Context, rawRefresh, userAgent, ip string) (access, newRefresh string, user *models.User, err error) {
	hash := HashRefreshToken(rawRefresh)
	row, err := s.rts.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", "", nil, ErrInvalidToken
		}
		return "", "", nil, err
	}
	if row.RevokedAt != nil {
		// Reuse of a revoked token: nuke the family.
		_ = s.rts.RevokeFamily(ctx, row.UserID)
		return "", "", nil, ErrTokenReused
	}
	if time.Now().After(row.ExpiresAt) {
		return "", "", nil, ErrTokenExpired
	}

	user, err = s.users.GetByID(ctx, row.UserID)
	if err != nil {
		return "", "", nil, err
	}

	now := time.Now()
	raw, newHash, err := NewRefreshToken()
	if err != nil {
		return "", "", nil, err
	}
	access, err = s.tokens.MintAccess(user.ID, user.Email, now)
	if err != nil {
		return "", "", nil, err
	}
	err = s.rts.Rotate(ctx, row.ID, repository.CreateTokenInput{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: newHash,
		ParentID:  &row.ID,
		UserAgent: userAgent,
		IPAddress: ip,
		ExpiresAt: now.Add(s.tokens.RefreshTTL()),
	})
	if err != nil {
		return "", "", nil, err
	}
	return access, raw, user, nil
}

// Revoke invalidates a single refresh token (logout for one device).
func (s *Service) Revoke(ctx context.Context, rawRefresh string) error {
	row, err := s.rts.GetByHash(ctx, HashRefreshToken(rawRefresh))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.rts.Revoke(ctx, row.ID)
}
