package models

import "time"

type User struct {
	ID            string     `json:"id"`
	GoogleSub     string     `json:"-"`
	Email         string     `json:"email"`
	EmailVerified bool       `json:"emailVerified"`
	Name          string     `json:"name"`
	PictureURL    string     `json:"pictureUrl,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`

	Plan                   string     `json:"plan"`
	PlanPeriod             string     `json:"planPeriod,omitempty"`
	PlanStartedAt          *time.Time `json:"planStartedAt,omitempty"`
	PlanExpiresAt          *time.Time `json:"planExpiresAt,omitempty"`
	RazorpayCustomerID     string     `json:"-"`
	RazorpaySubscriptionID string     `json:"-"`
}

type RefreshToken struct {
	ID           string
	UserID       string
	TokenHash    []byte
	ParentID     *string
	UserAgent    string
	IPAddress    string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	ReplacedByID *string
}
