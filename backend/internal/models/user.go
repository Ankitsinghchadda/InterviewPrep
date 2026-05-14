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
