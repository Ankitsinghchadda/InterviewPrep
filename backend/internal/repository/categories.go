package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
)

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
