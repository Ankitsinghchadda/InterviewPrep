package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type QuestionRepo struct {
	DB *sql.DB
}

type ListQuestionsFilter struct {
	CategorySlugs []string // any-of match (OR across slugs)
	Difficulty    string
	OwnerID       string // when set, also includes questions owned by this user (private)
	OnlyMine      bool   // when true, return ONLY this user's questions
	Limit         int
}

// selectColumns is shared across SELECTs so all paths populate the same fields.
const selectColumns = `
	q.id, q.slug, q.title, q.body, q.answer, q.difficulty, q.answer_audio_url,
	q.owner_id, q.is_public, q.source, q.intent, q.created_at,
	COALESCE(
	    (SELECT array_agg(c.slug ORDER BY c.sort_order)
	     FROM question_categories qc
	     JOIN categories c ON c.id = qc.category_id
	     WHERE qc.question_id = q.id),
	    ARRAY[]::text[]
	) AS categories
`

// List returns curated + user questions matching the filter. Adaptive questions
// are intentionally excluded — they only live inside their owning interview.
func (r *QuestionRepo) List(ctx context.Context, f ListQuestionsFilter) ([]models.Question, error) {
	args := []any{}
	conds := []string{`q.source <> 'adaptive'`}

	if f.OnlyMine {
		if f.OwnerID == "" {
			return []models.Question{}, nil
		}
		args = append(args, f.OwnerID)
		conds = append(conds, `q.owner_id = $`+itoa(len(args)))
	} else if f.OwnerID != "" {
		args = append(args, f.OwnerID)
		conds = append(conds, `(q.is_public = TRUE OR q.owner_id = $`+itoa(len(args))+`)`)
	} else {
		conds = append(conds, `q.is_public = TRUE AND q.owner_id IS NULL`)
	}

	if f.Difficulty != "" {
		args = append(args, f.Difficulty)
		conds = append(conds, `q.difficulty = $`+itoa(len(args)))
	}

	if len(f.CategorySlugs) > 0 {
		args = append(args, pq.Array(f.CategorySlugs))
		conds = append(conds, `q.id IN (
			SELECT qc.question_id FROM question_categories qc
			JOIN categories c ON c.id = qc.category_id
			WHERE c.slug = ANY($`+itoa(len(args))+`)
		)`)
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	args = append(args, limit)

	query := `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY q.created_at DESC
		LIMIT $` + itoa(len(args))

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

func (r *QuestionRepo) Get(ctx context.Context, id string) (*models.Question, error) {
	const query = `SELECT ` + selectColumns + ` FROM questions q WHERE q.id = $1`
	row := r.DB.QueryRowContext(ctx, query, id)
	q, err := scanQuestion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return q, err
}

type CreateQuestionInput struct {
	Title         string
	Body          string
	Answer        string
	Difficulty    string
	OwnerID       string   // required for user-created
	Source        string   // 'user' (default) | 'adaptive'
	Intent        string   // free-form tag (e.g. "introduction", "behavioral", "technical")
	CategorySlugs []string // resolved to ids server-side
	IsPublic      bool     // user questions default to FALSE
}

// Create inserts a user-owned (private) question and links it to the given categories.
func (r *QuestionRepo) Create(ctx context.Context, in CreateQuestionInput) (*models.Question, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	source := in.Source
	if source == "" {
		source = "user"
	}

	id := uuid.NewString()
	const insertQ = `
		INSERT INTO questions (id, title, body, answer, difficulty, owner_id, is_public, source, intent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := tx.ExecContext(ctx, insertQ,
		id, in.Title, in.Body, in.Answer, in.Difficulty,
		in.OwnerID, in.IsPublic, source, in.Intent,
	); err != nil {
		return nil, err
	}

	if len(in.CategorySlugs) > 0 {
		const linkQ = `
			INSERT INTO question_categories (question_id, category_id)
			SELECT $1, c.id FROM categories c WHERE c.slug = ANY($2)
			ON CONFLICT DO NOTHING
		`
		if _, err := tx.ExecContext(ctx, linkQ, id, pq.Array(in.CategorySlugs)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

// UpdateAudioURL persists the public URL for a question's synthesized
// reference-answer audio. Idempotent — runs even if the row is missing.
func (r *QuestionRepo) UpdateAudioURL(ctx context.Context, id, url string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE questions SET answer_audio_url = $1 WHERE id = $2`, url, id)
	return err
}

// Delete removes a question, but only if it belongs to the caller. Returns
// ErrNotFound when the row doesn't exist OR the owner doesn't match — those
// callers shouldn't be able to distinguish the two.
func (r *QuestionRepo) Delete(ctx context.Context, id, ownerID string) error {
	res, err := r.DB.ExecContext(ctx,
		`DELETE FROM questions WHERE id = $1 AND owner_id = $2`, id, ownerID)
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

// PickRandom selects N random curated + user-owned questions matching the
// given category slugs. Adaptive questions are excluded.
func (r *QuestionRepo) PickRandom(ctx context.Context, categorySlugs []string, ownerID string, n int) ([]models.Question, error) {
	if n <= 0 {
		return nil, nil
	}
	args := []any{}
	conds := []string{`q.source <> 'adaptive'`}

	if ownerID != "" {
		args = append(args, ownerID)
		conds = append(conds, `(q.is_public = TRUE OR q.owner_id = $`+itoa(len(args))+`)`)
	} else {
		conds = append(conds, `q.is_public = TRUE AND q.owner_id IS NULL`)
	}

	if len(categorySlugs) > 0 {
		args = append(args, pq.Array(categorySlugs))
		conds = append(conds, `q.id IN (
			SELECT qc.question_id FROM question_categories qc
			JOIN categories c ON c.id = qc.category_id
			WHERE c.slug = ANY($`+itoa(len(args))+`)
		)`)
	}

	args = append(args, n)
	query := `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY random()
		LIMIT $` + itoa(len(args))

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

// Recommended returns up to N curated questions matched to the user's profile.
// Strategy: prefer questions in their target_role or tech-stack categories AND
// that they haven't recently submitted an answer to.
func (r *QuestionRepo) Recommended(ctx context.Context, userID string, categorySlugs []string, limit int) ([]models.Question, error) {
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	args := []any{userID}
	conds := []string{
		`q.source = 'curated'`,
		`q.is_public = TRUE`,
		`q.id NOT IN (
			SELECT question_id FROM answer_submissions WHERE user_id = $1
		)`,
	}
	if len(categorySlugs) > 0 {
		args = append(args, pq.Array(categorySlugs))
		conds = append(conds, `q.id IN (
			SELECT qc.question_id FROM question_categories qc
			JOIN categories c ON c.id = qc.category_id
			WHERE c.slug = ANY($`+itoa(len(args))+`)
		)`)
	}
	args = append(args, limit)
	query := `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY random()
		LIMIT $` + itoa(len(args))
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

// rowScanner abstracts *sql.Row and *sql.Rows so scanQuestion accepts both.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanQuestion(row rowScanner) (*models.Question, error) {
	var (
		q     models.Question
		slug  sql.NullString
		owner sql.NullString
		cats  pq.StringArray
	)
	if err := row.Scan(
		&q.ID, &slug, &q.Title, &q.Body, &q.Answer, &q.Difficulty, &q.AnswerAudioURL,
		&owner, &q.IsPublic, &q.Source, &q.Intent, &q.CreatedAt, &cats,
	); err != nil {
		return nil, err
	}
	if slug.Valid {
		s := slug.String
		q.Slug = &s
	}
	if owner.Valid {
		s := owner.String
		q.OwnerID = &s
	}
	q.Categories = []string(cats)
	if q.Categories == nil {
		q.Categories = []string{}
	}
	return &q, nil
}

func itoa(i int) string { return strconv.Itoa(i) }
