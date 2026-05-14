package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type UserRepo struct {
	DB *sql.DB
}

type GoogleProfile struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	PictureURL    string
}

// UpsertFromGoogle inserts or updates a user keyed on google_sub.
// It returns the canonical user row after the write.
func (r *UserRepo) UpsertFromGoogle(ctx context.Context, p GoogleProfile) (*models.User, error) {
	const q = `
		INSERT INTO users (id, google_sub, email, email_verified, name, picture_url, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			email_verified = EXCLUDED.email_verified,
			name = EXCLUDED.name,
			picture_url = EXCLUDED.picture_url,
			updated_at = NOW(),
			last_login_at = NOW()
		RETURNING id, google_sub, email, email_verified, name, picture_url, created_at, updated_at, last_login_at
	`
	id := uuid.NewString()
	var u models.User
	var last sql.NullTime
	err := r.DB.QueryRowContext(ctx, q, id, p.Sub, p.Email, p.EmailVerified, p.Name, p.PictureURL).
		Scan(&u.ID, &u.GoogleSub, &u.Email, &u.EmailVerified, &u.Name, &u.PictureURL, &u.CreatedAt, &u.UpdatedAt, &last)
	if err != nil {
		return nil, err
	}
	if last.Valid {
		t := last.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	const q = `
		SELECT id, google_sub, email, email_verified, name, picture_url, created_at, updated_at, last_login_at
		FROM users WHERE id = $1
	`
	var u models.User
	var last sql.NullTime
	err := r.DB.QueryRowContext(ctx, q, id).
		Scan(&u.ID, &u.GoogleSub, &u.Email, &u.EmailVerified, &u.Name, &u.PictureURL, &u.CreatedAt, &u.UpdatedAt, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if last.Valid {
		t := last.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}

// Touch updates last_login_at without changing other fields.
func (r *UserRepo) Touch(ctx context.Context, id string, at time.Time) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE users SET last_login_at = $1, updated_at = NOW() WHERE id = $2`, at, id)
	return err
}
