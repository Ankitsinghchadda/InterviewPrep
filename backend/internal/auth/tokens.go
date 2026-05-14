package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret       []byte
	accessTTL    time.Duration
	refreshTTL   time.Duration
	issuer       string
}

func NewTokenManager(secret []byte, accessTTL, refreshTTL time.Duration, issuer string) *TokenManager {
	return &TokenManager{secret: secret, accessTTL: accessTTL, refreshTTL: refreshTTL, issuer: issuer}
}

func (tm *TokenManager) AccessTTL() time.Duration  { return tm.accessTTL }
func (tm *TokenManager) RefreshTTL() time.Duration { return tm.refreshTTL }

func (tm *TokenManager) MintAccess(userID, email string, now time.Time) (string, error) {
	claims := AccessClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(tm.secret)
}

func (tm *TokenManager) ParseAccess(raw string) (*AccessClaims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &AccessClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// NewRefreshToken returns a high-entropy opaque token (raw) and its SHA-256 hash for storage.
// Only the hash is persisted; the raw value lives in the user's cookie.
func NewRefreshToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}

func HashRefreshToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
