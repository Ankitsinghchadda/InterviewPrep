package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type InterviewRepo struct {
	DB *sql.DB
}

type CreateInterviewInput struct {
	UserID          string
	Mode            string // 'topic' (default) | 'adaptive' | 'live'
	CategoryIDs     []string
	QuestionIDs     []string // ordered list; position assigned from index
	DurationSeconds int      // live mode only; 0 = no timer
	JobDescription  string   // live mode only; empty when not provided
}

// Create persists a new interview and its ordered question links.
func (r *InterviewRepo) Create(ctx context.Context, in CreateInterviewInput) (*models.Interview, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	mode := in.Mode
	if mode == "" {
		mode = "topic"
	}

	id := uuid.NewString()
	const q = `
		INSERT INTO interviews (id, user_id, mode, category_ids, status, duration_seconds, job_description)
		VALUES ($1, $2, $3, $4, 'in_progress', $5, $6)
		RETURNING started_at
	`
	var startedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, q, id, in.UserID, mode, uuidArray(in.CategoryIDs), in.DurationSeconds, in.JobDescription).Scan(&startedAt); err != nil {
		return nil, err
	}

	if len(in.QuestionIDs) > 0 {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO interview_questions (interview_id, question_id, position) VALUES ($1, $2, $3)`)
		if err != nil {
			return nil, err
		}
		defer stmt.Close()
		for i, qid := range in.QuestionIDs {
			if _, err := stmt.ExecContext(ctx, id, qid, i); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	out := &models.Interview{
		ID:              id,
		UserID:          in.UserID,
		Mode:            mode,
		CategoryIDs:     in.CategoryIDs,
		Status:          "in_progress",
		DurationSeconds: in.DurationSeconds,
		JobDescription:  in.JobDescription,
	}
	if startedAt.Valid {
		out.StartedAt = startedAt.Time
	}
	return out, nil
}

func (r *InterviewRepo) Get(ctx context.Context, id, userID string) (*models.Interview, error) {
	const q = `
		SELECT id, user_id, mode, category_ids, status, score, summary, started_at, finished_at, duration_seconds, job_description
		FROM interviews WHERE id = $1 AND user_id = $2
	`
	row := r.DB.QueryRowContext(ctx, q, id, userID)
	iv, err := scanInterview(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return iv, err
}

// ListForUser returns the user's interviews newest-first.
func (r *InterviewRepo) ListForUser(ctx context.Context, userID string, limit int) ([]models.Interview, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	const q = `
		SELECT id, user_id, mode, category_ids, status, score, summary, started_at, finished_at, duration_seconds, job_description
		FROM interviews WHERE user_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`
	rows, err := r.DB.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Interview{}
	for rows.Next() {
		iv, err := scanInterview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *iv)
	}
	return out, rows.Err()
}

func scanInterview(row rowScanner) (*models.Interview, error) {
	var (
		iv      models.Interview
		cats    pq.StringArray
		score   sql.NullFloat64
		summary sql.NullString
		fin     sql.NullTime
	)
	if err := row.Scan(
		&iv.ID, &iv.UserID, &iv.Mode, &cats, &iv.Status, &score, &summary, &iv.StartedAt, &fin, &iv.DurationSeconds, &iv.JobDescription,
	); err != nil {
		return nil, err
	}
	iv.CategoryIDs = []string(cats)
	if iv.CategoryIDs == nil {
		iv.CategoryIDs = []string{}
	}
	if score.Valid {
		v := score.Float64
		iv.Score = &v
	}
	if summary.Valid {
		iv.Summary = summary.String
	}
	if fin.Valid {
		t := fin.Time
		iv.FinishedAt = &t
	}
	return &iv, nil
}

// Complete marks the interview completed with aggregate score + summary.
func (r *InterviewRepo) Complete(ctx context.Context, id string, score float64, summary string) error {
	_, err := r.DB.ExecContext(ctx, `
		UPDATE interviews
		SET status = 'completed', score = $1, summary = $2, finished_at = NOW()
		WHERE id = $3
	`, score, summary, id)
	return err
}

// AppendQuestion links a question to an interview at the given position. Used
// by live mode to grow the question list one turn at a time. Idempotent on PK.
func (r *InterviewRepo) AppendQuestion(ctx context.Context, interviewID, questionID string, position int) error {
	const q = `
		INSERT INTO interview_questions (interview_id, question_id, position)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`
	_, err := r.DB.ExecContext(ctx, q, interviewID, questionID, position)
	return err
}

// QuestionsFor returns the ordered list of questions attached to an interview.
func (r *InterviewRepo) QuestionsFor(ctx context.Context, interviewID string) ([]models.Question, error) {
	const q = `
		SELECT ` + selectColumns + `
		FROM interview_questions iq
		JOIN questions q ON q.id = iq.question_id
		WHERE iq.interview_id = $1
		ORDER BY iq.position
	`
	rows, err := r.DB.QueryContext(ctx, q, interviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		qm, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *qm)
	}
	return out, rows.Err()
}
