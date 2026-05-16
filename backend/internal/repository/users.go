package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInUse    = errors.New("row is referenced by another table")
)

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

// userSelectColumns is the full column set GetByID and UpsertFromGoogle
// share. Centralized so adding a column only requires updating Scan() in
// scanUser below — not the SQL in every method.
const userSelectColumns = `id, google_sub, email, email_verified, name, picture_url,
	created_at, updated_at, last_login_at,
	plan, plan_period, plan_started_at, plan_expires_at,
	razorpay_customer_id, razorpay_subscription_id`

func scanUser(row rowScanner) (*models.User, error) {
	var u models.User
	var last, started, expires sql.NullTime
	err := row.Scan(
		&u.ID, &u.GoogleSub, &u.Email, &u.EmailVerified, &u.Name, &u.PictureURL,
		&u.CreatedAt, &u.UpdatedAt, &last,
		&u.Plan, &u.PlanPeriod, &started, &expires,
		&u.RazorpayCustomerID, &u.RazorpaySubscriptionID,
	)
	if err != nil {
		return nil, err
	}
	if last.Valid {
		t := last.Time
		u.LastLoginAt = &t
	}
	if started.Valid {
		t := started.Time
		u.PlanStartedAt = &t
	}
	if expires.Valid {
		t := expires.Time
		u.PlanExpiresAt = &t
	}
	return &u, nil
}

// UpsertFromGoogle inserts or updates a user keyed on google_sub.
// It returns the canonical user row after the write.
func (r *UserRepo) UpsertFromGoogle(ctx context.Context, p GoogleProfile) (*models.User, error) {
	q := `
		INSERT INTO users (id, google_sub, email, email_verified, name, picture_url, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			email_verified = EXCLUDED.email_verified,
			name = EXCLUDED.name,
			picture_url = EXCLUDED.picture_url,
			updated_at = NOW(),
			last_login_at = NOW()
		RETURNING ` + userSelectColumns
	id := uuid.NewString()
	return scanUser(r.DB.QueryRowContext(ctx, q, id, p.Sub, p.Email, p.EmailVerified, p.Name, p.PictureURL))
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	q := `SELECT ` + userSelectColumns + ` FROM users WHERE id = $1`
	u, err := scanUser(r.DB.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// UpgradeToPro flips a user to the paid tier. Idempotent: re-calling
// with the same subscription_id rolls plan_expires_at forward without
// minting a new plan_started_at.
func (r *UserRepo) UpgradeToPro(ctx context.Context, userID, period, subscriptionID string, expiresAt time.Time) error {
	const q = `
		UPDATE users SET
			plan = 'pro',
			plan_period = $2,
			plan_started_at = COALESCE(plan_started_at, NOW()),
			plan_expires_at = $3,
			razorpay_subscription_id = COALESCE(NULLIF($4, ''), razorpay_subscription_id),
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.DB.ExecContext(ctx, q, userID, period, expiresAt, subscriptionID)
	return err
}

// Downgrade moves a user back to free, typically after plan_expires_at
// passes or a Razorpay subscription.cancelled webhook (post grace).
func (r *UserRepo) Downgrade(ctx context.Context, userID string) error {
	const q = `
		UPDATE users SET
			plan = 'free',
			plan_period = '',
			plan_expires_at = NULL,
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.DB.ExecContext(ctx, q, userID)
	return err
}

// LoadPlan returns just the billing fields used by the auth middleware to
// stamp the plan onto every authenticated request. Smaller payload than
// GetByID — one indexed lookup, two columns.
func (r *UserRepo) LoadPlan(ctx context.Context, userID string) (string, *time.Time, error) {
	const q = `SELECT plan, plan_expires_at FROM users WHERE id = $1`
	var plan string
	var expires sql.NullTime
	if err := r.DB.QueryRowContext(ctx, q, userID).Scan(&plan, &expires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, ErrNotFound
		}
		return "", nil, err
	}
	if expires.Valid {
		t := expires.Time
		return plan, &t, nil
	}
	return plan, nil, nil
}

// Touch updates last_login_at without changing other fields.
func (r *UserRepo) Touch(ctx context.Context, id string, at time.Time) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE users SET last_login_at = $1, updated_at = NOW() WHERE id = $2`, at, id)
	return err
}
