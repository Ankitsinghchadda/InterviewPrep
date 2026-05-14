package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/lib/pq"
)

type ProfileRepo struct {
	DB *sql.DB
}

// Get returns the profile for a user. Returns ErrNotFound for users who have
// never completed onboarding (no row yet).
func (r *ProfileRepo) Get(ctx context.Context, userID string) (*models.Profile, error) {
	const q = `
		SELECT user_id, target_role, years_experience, seniority, current_title,
		       tech_stack, target_companies, goals, resume_text, resume_filename,
		       onboarded_at, created_at, updated_at
		FROM user_profiles WHERE user_id = $1
	`
	row := r.DB.QueryRowContext(ctx, q, userID)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

type UpsertProfileInput struct {
	UserID          string
	TargetRole      string
	YearsExperience int
	Seniority       string
	CurrentRole     string
	TechStack       []string
	TargetCompanies []string
	Goals           string
	MarkOnboarded   bool
}

// Upsert writes profile data without touching resume fields (those are set by
// UpdateResume). MarkOnboarded=true sets onboarded_at if currently NULL.
func (r *ProfileRepo) Upsert(ctx context.Context, in UpsertProfileInput) (*models.Profile, error) {
	// Normalize nil slices: the schema is NOT NULL with default '{}', but
	// pq.Array(nil) serializes to SQL NULL which violates that constraint.
	// Callers can legitimately omit either array (e.g. resume extractor doesn't
	// populate TargetCompanies) so we silently treat nil as empty.
	if in.TechStack == nil {
		in.TechStack = []string{}
	}
	if in.TargetCompanies == nil {
		in.TargetCompanies = []string{}
	}
	const q = `
		INSERT INTO user_profiles (
		    user_id, target_role, years_experience, seniority, current_title,
		    tech_stack, target_companies, goals,
		    onboarded_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
		    CASE WHEN $9 THEN NOW() ELSE NULL END, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
		    target_role       = EXCLUDED.target_role,
		    years_experience  = EXCLUDED.years_experience,
		    seniority         = EXCLUDED.seniority,
		    current_title      = EXCLUDED.current_title,
		    tech_stack        = EXCLUDED.tech_stack,
		    target_companies  = EXCLUDED.target_companies,
		    goals             = EXCLUDED.goals,
		    onboarded_at      = COALESCE(user_profiles.onboarded_at,
		                                 CASE WHEN $9 THEN NOW() ELSE NULL END),
		    updated_at        = NOW()
		RETURNING user_id, target_role, years_experience, seniority, current_title,
		          tech_stack, target_companies, goals, resume_text, resume_filename,
		          onboarded_at, created_at, updated_at
	`
	row := r.DB.QueryRowContext(ctx, q,
		in.UserID, in.TargetRole, in.YearsExperience, in.Seniority, in.CurrentRole,
		pq.Array(in.TechStack), pq.Array(in.TargetCompanies), in.Goals, in.MarkOnboarded,
	)
	return scanProfile(row)
}

// UpdateResume stores extracted resume text and the original filename.
func (r *ProfileRepo) UpdateResume(ctx context.Context, userID, text, filename string) error {
	const q = `
		INSERT INTO user_profiles (user_id, resume_text, resume_filename, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
		    resume_text     = EXCLUDED.resume_text,
		    resume_filename = EXCLUDED.resume_filename,
		    updated_at      = NOW()
	`
	_, err := r.DB.ExecContext(ctx, q, userID, text, filename)
	return err
}

func scanProfile(row rowScanner) (*models.Profile, error) {
	var (
		p           models.Profile
		tech        pq.StringArray
		companies   pq.StringArray
		onboardedAt sql.NullTime
	)
	if err := row.Scan(
		&p.UserID, &p.TargetRole, &p.YearsExperience, &p.Seniority, &p.CurrentRole,
		&tech, &companies, &p.Goals, &p.ResumeText, &p.ResumeFilename,
		&onboardedAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	p.TechStack = []string(tech)
	p.TargetCompanies = []string(companies)
	if p.TechStack == nil {
		p.TechStack = []string{}
	}
	if p.TargetCompanies == nil {
		p.TargetCompanies = []string{}
	}
	if onboardedAt.Valid {
		t := onboardedAt.Time
		p.OnboardedAt = &t
	}
	return &p, nil
}
