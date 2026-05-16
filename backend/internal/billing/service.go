package billing

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrQuotaExceeded is returned by Check when the caller has hit their
// rolling-window cap for the requested kind. Handlers should map this
// to HTTP 402 with a structured "quota_exceeded" body so the frontend
// can open the paywall modal.
var ErrQuotaExceeded = errors.New("billing: quota exceeded")

// ErrPlanRequired is returned by Check for kinds the user's plan
// blocks entirely (Limit == LimitBlocked). Handlers should map this
// to HTTP 403 with a structured "plan_required" body.
var ErrPlanRequired = errors.New("billing: plan required")

// UsageRepo is the persistence surface for usage_events.
type UsageRepo interface {
	// CountInWindow returns how many events of kind happened for user
	// since `since`. Used to compare against the active quota Limit.
	CountInWindow(ctx context.Context, userID string, kind Kind, since time.Time) (int, error)

	// Insert appends one event. Metadata is best-effort: failures here
	// must not break the user's API call — log and continue.
	Insert(ctx context.Context, userID string, kind Kind, metadata []byte) error

	// CountsByKind returns the per-kind counts for one user inside `since`.
	// Backs the GET /usage endpoint in one query.
	CountsByKind(ctx context.Context, userID string, since time.Time) (map[Kind]int, error)
}

// Service is the entry point handlers call. It's intentionally tiny —
// the only state is the repo and a clock (overridable for tests).
type Service struct {
	Repo  UsageRepo
	Clock func() time.Time
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now().UTC()
}

// Check verifies that the user has remaining quota for kind. Returns
// ErrPlanRequired if the kind is blocked entirely on their plan,
// ErrQuotaExceeded if they've hit the rolling cap, or nil if allowed.
// Pro users with Limit == LimitUnlimited skip the count query.
func (s *Service) Check(ctx context.Context, userID, plan string, kind Kind) error {
	q := QuotaFor(plan, kind)
	if q.Limit == LimitUnlimited {
		return nil
	}
	if q.Limit == LimitBlocked {
		return ErrPlanRequired
	}
	since := s.now().Add(-q.Window)
	used, err := s.Repo.CountInWindow(ctx, userID, kind, since)
	if err != nil {
		return err
	}
	if used >= q.Limit {
		return ErrQuotaExceeded
	}
	return nil
}

// Record inserts one usage_events row. Call after the AI action
// succeeds so failed attempts don't burn quota. Errors are returned
// but callers typically just log and continue — losing one event is
// less bad than failing the user's API call.
func (s *Service) Record(ctx context.Context, userID string, kind Kind, meta map[string]any) error {
	var raw []byte
	if len(meta) > 0 {
		b, err := json.Marshal(meta)
		if err == nil {
			raw = b
		}
	}
	return s.Repo.Insert(ctx, userID, kind, raw)
}

// UsageSnapshot is the read model behind GET /api/v1/usage.
type UsageSnapshot struct {
	Plan          string            `json:"plan"`
	PlanExpiresAt *time.Time        `json:"planExpiresAt,omitempty"`
	WindowStart   time.Time         `json:"windowStart"`
	Quotas        map[Kind]UsageRow `json:"quotas"`
}

type UsageRow struct {
	Used      int  `json:"used"`
	Limit     int  `json:"limit"`     // -1 means unlimited
	Remaining int  `json:"remaining"` // -1 means unlimited; clamped at 0 otherwise
	Blocked   bool `json:"blocked"`   // true when this kind is not available on the user's plan at all
}

// Snapshot is the read-side of usage: one row per Kind, used + limit + remaining.
func (s *Service) Snapshot(ctx context.Context, userID, plan string, planExpiresAt *time.Time) (*UsageSnapshot, error) {
	windowStart := s.now().Add(-Week)
	counts, err := s.Repo.CountsByKind(ctx, userID, windowStart)
	if err != nil {
		return nil, err
	}
	rows := make(map[Kind]UsageRow, len(AllKinds))
	for _, k := range AllKinds {
		q := QuotaFor(plan, k)
		used := counts[k]
		row := UsageRow{Used: used, Limit: q.Limit}
		switch q.Limit {
		case LimitUnlimited:
			row.Remaining = LimitUnlimited
		case LimitBlocked:
			row.Blocked = true
			row.Remaining = 0
		default:
			r := q.Limit - used
			if r < 0 {
				r = 0
			}
			row.Remaining = r
		}
		rows[k] = row
	}
	return &UsageSnapshot{
		Plan:          plan,
		PlanExpiresAt: planExpiresAt,
		WindowStart:   windowStart,
		Quotas:        rows,
	}, nil
}
