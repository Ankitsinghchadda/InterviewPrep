package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
)

// UsageRepo persists usage_events rows. The schema is append-only —
// COUNTing is fast because of idx_usage_user_kind_time.
type UsageRepo struct {
	DB *sql.DB
}

func (r *UsageRepo) CountInWindow(ctx context.Context, userID string, kind billing.Kind, since time.Time) (int, error) {
	const q = `
		SELECT COUNT(*) FROM usage_events
		WHERE user_id = $1 AND kind = $2 AND occurred_at > $3
	`
	var n int
	if err := r.DB.QueryRowContext(ctx, q, userID, string(kind), since).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *UsageRepo) Insert(ctx context.Context, userID string, kind billing.Kind, metadata []byte) error {
	const q = `
		INSERT INTO usage_events (user_id, kind, metadata)
		VALUES ($1, $2, COALESCE($3::jsonb, '{}'::jsonb))
	`
	var meta interface{}
	if len(metadata) > 0 {
		meta = metadata
	}
	_, err := r.DB.ExecContext(ctx, q, userID, string(kind), meta)
	return err
}

func (r *UsageRepo) CountsByKind(ctx context.Context, userID string, since time.Time) (map[billing.Kind]int, error) {
	const q = `
		SELECT kind, COUNT(*) FROM usage_events
		WHERE user_id = $1 AND occurred_at > $2
		GROUP BY kind
	`
	rows, err := r.DB.QueryContext(ctx, q, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[billing.Kind]int)
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[billing.Kind(k)] = n
	}
	return out, rows.Err()
}
