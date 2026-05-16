package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// PaymentEventRepo persists Razorpay webhook deliveries. provider_event_id
// is UNIQUE, so duplicate replays of the same webhook silently no-op —
// that's the idempotency contract.
type PaymentEventRepo struct {
	DB *sql.DB
}

type InsertPaymentEventInput struct {
	UserID          string // optional — Razorpay event may pre-date our user mapping
	Provider        string // 'razorpay'
	ProviderEventID string // razorpay event id; UNIQUE
	EventType       string // e.g. 'subscription.activated'
	Amount          int64  // smallest unit (paise/cents), 0 if N/A
	Currency        string // INR / USD
	Payload         []byte // full raw payload as JSON
}

// ErrDuplicateEvent is returned when an event id has already been
// recorded. Handlers should treat this as success (200 OK) so Razorpay
// stops retrying the delivery.
var ErrDuplicateEvent = errors.New("payment event already recorded")

func (r *PaymentEventRepo) Insert(ctx context.Context, in InsertPaymentEventInput) error {
	const q = `
		INSERT INTO payment_events (user_id, provider, provider_event_id, event_type, amount, currency, payload)
		VALUES (NULLIF($1, '')::uuid, $2, $3, $4, NULLIF($5, 0), NULLIF($6, ''), $7::jsonb)
		ON CONFLICT (provider_event_id) DO NOTHING
	`
	res, err := r.DB.ExecContext(ctx, q,
		in.UserID, in.Provider, in.ProviderEventID, in.EventType, in.Amount, in.Currency, in.Payload,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrDuplicateEvent
	}
	return nil
}

// looksLikeUUID is a tiny guard for the NULLIF...::uuid cast above —
// callers can pass an empty string when the event isn't associated with
// a known user yet, but anything else must be a UUID or Postgres will
// reject the row. Kept here in case a handler wants to validate first.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	return strings.Count(s, "-") == 4
}

var _ = looksLikeUUID // currently unused; keep for future webhook code
