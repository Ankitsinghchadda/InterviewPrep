package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type CollectionRepo struct {
	DB *sql.DB
}

const defaultCollectionName = "Saved"

// EnsureDefault returns the user's default ("Saved") collection, creating it
// if missing. Called lazily on first /collections access so we never have to
// touch every user record up-front.
func (r *CollectionRepo) EnsureDefault(ctx context.Context, userID string) (*models.Collection, error) {
	// Race-safe path: try to read first; if absent, insert with ON CONFLICT
	// so concurrent first-time visits never collide on the (user_id, name) unique.
	got, err := r.getDefault(ctx, userID)
	if err == nil {
		return got, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	id := uuid.NewString()
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO collections (id, user_id, name, is_default)
		VALUES ($1, $2, $3, TRUE)
		ON CONFLICT (user_id, name) DO NOTHING
	`, id, userID, defaultCollectionName)
	if err != nil {
		return nil, err
	}
	return r.getDefault(ctx, userID)
}

func (r *CollectionRepo) getDefault(ctx context.Context, userID string) (*models.Collection, error) {
	const q = `
		SELECT c.id, c.user_id, c.name, c.description, c.color, c.is_default,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_questions cq WHERE cq.collection_id = c.id)::int
		FROM collections c
		WHERE c.user_id = $1 AND c.is_default = TRUE
		LIMIT 1
	`
	row := r.DB.QueryRowContext(ctx, q, userID)
	c, err := scanCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// List returns the caller's collections newest-first, default first. The
// default row is forced to the top so the UI doesn't have to sort.
func (r *CollectionRepo) List(ctx context.Context, userID string) ([]models.Collection, error) {
	const q = `
		SELECT c.id, c.user_id, c.name, c.description, c.color, c.is_default,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_questions cq WHERE cq.collection_id = c.id)::int
		FROM collections c
		WHERE c.user_id = $1
		ORDER BY c.is_default DESC, c.created_at DESC
	`
	rows, err := r.DB.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Collection{}
	for rows.Next() {
		c, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *CollectionRepo) Get(ctx context.Context, id, userID string) (*models.Collection, error) {
	const q = `
		SELECT c.id, c.user_id, c.name, c.description, c.color, c.is_default,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_questions cq WHERE cq.collection_id = c.id)::int
		FROM collections c
		WHERE c.id = $1 AND c.user_id = $2
	`
	row := r.DB.QueryRowContext(ctx, q, id, userID)
	c, err := scanCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

type CreateCollectionInput struct {
	UserID      string
	Name        string
	Description string
	Color       string
}

// Create inserts a user-created collection. The default Saved row is created
// only through EnsureDefault — callers cannot create their own is_default.
// Returns ErrDuplicate when the name is already taken for this user.
func (r *CollectionRepo) Create(ctx context.Context, in CreateCollectionInput) (*models.Collection, error) {
	id := uuid.NewString()
	const q = `
		INSERT INTO collections (id, user_id, name, description, color, is_default)
		VALUES ($1, $2, $3, $4, $5, FALSE)
	`
	_, err := r.DB.ExecContext(ctx, q, id, in.UserID, strings.TrimSpace(in.Name), in.Description, in.Color)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return r.Get(ctx, id, in.UserID)
}

type UpdateCollectionInput struct {
	Name        *string
	Description *string
	Color       *string
}

// Update modifies the name/description/color of a user-owned collection.
// The default Saved row cannot be renamed; callers should block this at the
// handler layer (returns ErrInUse here when attempted to keep failure modes
// distinct from "not found").
func (r *CollectionRepo) Update(ctx context.Context, id, userID string, in UpdateCollectionInput) (*models.Collection, error) {
	existing, err := r.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if existing.IsDefault && in.Name != nil && strings.TrimSpace(*in.Name) != existing.Name {
		return nil, ErrInUse
	}

	sets := []string{}
	args := []any{}
	if in.Name != nil {
		args = append(args, strings.TrimSpace(*in.Name))
		sets = append(sets, "name = $"+itoa(len(args)))
	}
	if in.Description != nil {
		args = append(args, *in.Description)
		sets = append(sets, "description = $"+itoa(len(args)))
	}
	if in.Color != nil {
		args = append(args, *in.Color)
		sets = append(sets, "color = $"+itoa(len(args)))
	}
	if len(sets) == 0 {
		return existing, nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, id, userID)
	q := "UPDATE collections SET " + strings.Join(sets, ", ") +
		" WHERE id = $" + itoa(len(args)-1) + " AND user_id = $" + itoa(len(args))
	if _, err := r.DB.ExecContext(ctx, q, args...); err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return r.Get(ctx, id, userID)
}

// Delete removes a collection. The default Saved row is not deletable —
// returns ErrInUse so handlers translate that to 400.
func (r *CollectionRepo) Delete(ctx context.Context, id, userID string) error {
	existing, err := r.Get(ctx, id, userID)
	if err != nil {
		return err
	}
	if existing.IsDefault {
		return ErrInUse
	}
	res, err := r.DB.ExecContext(ctx,
		`DELETE FROM collections WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddQuestion inserts (collection_id, question_id) idempotently. Returns
// ErrNotFound if the collection isn't owned by the caller; the FK to questions
// guards against bogus question ids.
func (r *CollectionRepo) AddQuestion(ctx context.Context, collectionID, userID, questionID string) error {
	if _, err := r.Get(ctx, collectionID, userID); err != nil {
		return err
	}
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO collection_questions (collection_id, question_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, collectionID, questionID)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (r *CollectionRepo) RemoveQuestion(ctx context.Context, collectionID, userID, questionID string) error {
	if _, err := r.Get(ctx, collectionID, userID); err != nil {
		return err
	}
	_, err := r.DB.ExecContext(ctx, `
		DELETE FROM collection_questions
		WHERE collection_id = $1 AND question_id = $2
	`, collectionID, questionID)
	return err
}

// CollectionIDsForQuestion returns the ids of the caller's collections that
// already include this question. Used to drive the "saved to..." indicator
// on QuestionDetail without an extra round-trip.
func (r *CollectionRepo) CollectionIDsForQuestion(ctx context.Context, userID, questionID string) ([]string, error) {
	const q = `
		SELECT c.id
		FROM collections c
		JOIN collection_questions cq ON cq.collection_id = c.id
		WHERE c.user_id = $1 AND cq.question_id = $2
	`
	rows, err := r.DB.QueryContext(ctx, q, userID, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func scanCollection(row rowScanner) (*models.Collection, error) {
	var c models.Collection
	if err := row.Scan(
		&c.ID, &c.UserID, &c.Name, &c.Description, &c.Color, &c.IsDefault,
		&c.CreatedAt, &c.UpdatedAt, &c.QuestionCount,
	); err != nil {
		return nil, err
	}
	return &c, nil
}
