package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/lib/pq"
)

// ErrDuplicate is returned when a unique constraint is violated (e.g., the
// slug is already taken). Handlers translate this into a 409 Conflict.
var ErrDuplicate = errors.New("duplicate")

type CategoryRepo struct {
	DB *sql.DB
}

// List returns categories optionally filtered by kind ("role" or "topic"). Pass "" for all.
func (r *CategoryRepo) List(ctx context.Context, kind string) ([]models.Category, error) {
	q := `
		SELECT id, slug, name, kind, description, icon, sort_order, created_at
		FROM categories
	`
	args := []any{}
	if kind != "" {
		q += ` WHERE kind = $1`
		args = append(args, kind)
	}
	q += ` ORDER BY sort_order, name`

	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Category{}
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Kind, &c.Description, &c.Icon, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CategoryRepo) GetBySlug(ctx context.Context, slug string) (*models.Category, error) {
	const q = `
		SELECT id, slug, name, kind, description, icon, sort_order, created_at
		FROM categories WHERE slug = $1
	`
	var c models.Category
	err := r.DB.QueryRowContext(ctx, q, slug).
		Scan(&c.ID, &c.Slug, &c.Name, &c.Kind, &c.Description, &c.Icon, &c.SortOrder, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create inserts a new category. Returns ErrDuplicate when slug collides.
// Caller is responsible for validating/normalising the slug.
func (r *CategoryRepo) Create(ctx context.Context, in models.Category) (*models.Category, error) {
	const q = `
		INSERT INTO categories (slug, name, kind, description, icon, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, slug, name, kind, description, icon, sort_order, created_at
	`
	var c models.Category
	err := r.DB.QueryRowContext(ctx, q,
		strings.ToLower(strings.TrimSpace(in.Slug)),
		strings.TrimSpace(in.Name),
		in.Kind,
		in.Description,
		in.Icon,
		in.SortOrder,
	).Scan(&c.ID, &c.Slug, &c.Name, &c.Kind, &c.Description, &c.Icon, &c.SortOrder, &c.CreatedAt)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return &c, nil
}

// IDsBySlugs maps a list of slugs to the matching category IDs.
// Order of the result matches the input order; missing slugs are skipped.
func (r *CategoryRepo) IDsBySlugs(ctx context.Context, slugs []string) ([]string, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	rows, err := r.DB.QueryContext(ctx,
		`SELECT slug, id FROM categories WHERE slug = ANY($1)`, slugArray(slugs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySlug := map[string]string{}
	for rows.Next() {
		var s, id string
		if err := rows.Scan(&s, &id); err != nil {
			return nil, err
		}
		bySlug[s] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(slugs))
	for _, s := range slugs {
		if id, ok := bySlug[s]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}
