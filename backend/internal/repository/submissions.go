package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type SubmissionRepo struct {
	DB *sql.DB
}

type CreateSubmissionInput struct {
	UserID      string
	QuestionID  string
	InterviewID *string
	AudioURL    string
}

func (r *SubmissionRepo) Create(ctx context.Context, in CreateSubmissionInput) (*models.Submission, error) {
	id := uuid.NewString()
	const q = `
		INSERT INTO answer_submissions (id, user_id, question_id, interview_id, audio_url, status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		RETURNING created_at, updated_at
	`
	var created, updated sql.NullTime
	err := r.DB.QueryRowContext(ctx, q, id, in.UserID, in.QuestionID, in.InterviewID, in.AudioURL).
		Scan(&created, &updated)
	if err != nil {
		return nil, err
	}
	s := &models.Submission{
		ID:          id,
		UserID:      in.UserID,
		QuestionID:  in.QuestionID,
		InterviewID: in.InterviewID,
		AudioURL:    in.AudioURL,
		Status:      "pending",
	}
	if created.Valid {
		s.CreatedAt = created.Time
	}
	if updated.Valid {
		s.UpdatedAt = updated.Time
	}
	return s, nil
}

func (r *SubmissionRepo) UpdateStatus(ctx context.Context, id, status, errMsg string) error {
	_, err := r.DB.ExecContext(ctx, `
		UPDATE answer_submissions
		SET status = $1, error_message = $2, updated_at = NOW()
		WHERE id = $3
	`, status, errMsg, id)
	return err
}

func (r *SubmissionRepo) SetTranscript(ctx context.Context, id, transcript string) error {
	_, err := r.DB.ExecContext(ctx, `
		UPDATE answer_submissions
		SET transcript = $1, status = 'reviewing', updated_at = NOW()
		WHERE id = $2
	`, transcript, id)
	return err
}

type ReviewResult struct {
	Score        float64
	Feedback     string
	Strengths    []string
	Improvements []string
}

func (r *SubmissionRepo) SetReview(ctx context.Context, id string, rr ReviewResult) error {
	_, err := r.DB.ExecContext(ctx, `
		UPDATE answer_submissions
		SET score = $1, feedback = $2, strengths = $3, improvements = $4,
		    status = 'complete', updated_at = NOW()
		WHERE id = $5
	`, rr.Score, rr.Feedback, slugArray(rr.Strengths), slugArray(rr.Improvements), id)
	return err
}

func (r *SubmissionRepo) Get(ctx context.Context, id, userID string) (*models.Submission, error) {
	const q = `
		SELECT id, user_id, question_id, interview_id, audio_url, transcript, feedback,
		       strengths, improvements, score, status, error_message, created_at, updated_at
		FROM answer_submissions WHERE id = $1 AND user_id = $2
	`
	row := r.DB.QueryRowContext(ctx, q, id, userID)
	s, err := scanSubmission(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *SubmissionRepo) ListForInterview(ctx context.Context, interviewID, userID string) ([]models.Submission, error) {
	const q = `
		SELECT id, user_id, question_id, interview_id, audio_url, transcript, feedback,
		       strengths, improvements, score, status, error_message, created_at, updated_at
		FROM answer_submissions
		WHERE interview_id = $1 AND user_id = $2
		ORDER BY created_at
	`
	rows, err := r.DB.QueryContext(ctx, q, interviewID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Submission{}
	for rows.Next() {
		s, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *SubmissionRepo) ListForUser(ctx context.Context, userID string, limit int) ([]models.Submission, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	const q = `
		SELECT id, user_id, question_id, interview_id, audio_url, transcript, feedback,
		       strengths, improvements, score, status, error_message, created_at, updated_at
		FROM answer_submissions WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.DB.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Submission{}
	for rows.Next() {
		s, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func scanSubmission(row rowScanner) (*models.Submission, error) {
	var (
		s             models.Submission
		interviewID   sql.NullString
		score         sql.NullFloat64
		strengths     pq.StringArray
		improvements  pq.StringArray
	)
	if err := row.Scan(
		&s.ID, &s.UserID, &s.QuestionID, &interviewID, &s.AudioURL, &s.Transcript, &s.Feedback,
		&strengths, &improvements, &score, &s.Status, &s.ErrorMessage, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if interviewID.Valid {
		v := interviewID.String
		s.InterviewID = &v
	}
	if score.Valid {
		v := score.Float64
		s.Score = &v
	}
	s.Strengths = []string(strengths)
	s.Improvements = []string(improvements)
	if s.Strengths == nil {
		s.Strengths = []string{}
	}
	if s.Improvements == nil {
		s.Improvements = []string{}
	}
	return &s, nil
}
