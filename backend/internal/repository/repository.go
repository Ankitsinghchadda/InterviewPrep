package repository

import "database/sql"

type Repository struct {
	DB          *sql.DB
	Users       *UserRepo
	Tokens      *RefreshTokenRepo
	Categories  *CategoryRepo
	Questions   *QuestionRepo
	Interviews  *InterviewRepo
	Submissions *SubmissionRepo
	Profiles    *ProfileRepo
	Stats       *StatsRepo
}

func New(db *sql.DB) *Repository {
	return &Repository{
		DB:          db,
		Users:       &UserRepo{DB: db},
		Tokens:      &RefreshTokenRepo{DB: db},
		Categories:  &CategoryRepo{DB: db},
		Questions:   &QuestionRepo{DB: db},
		Interviews:  &InterviewRepo{DB: db},
		Submissions: &SubmissionRepo{DB: db},
		Profiles:    &ProfileRepo{DB: db},
		Stats:       &StatsRepo{DB: db},
	}
}
