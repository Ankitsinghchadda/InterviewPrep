package services

import "github.com/Ankitsinghchadda/InterviewPrep/internal/repository"

type QuestionService struct {
	repo *repository.Repository
}

func NewQuestionService(r *repository.Repository) *QuestionService {
	return &QuestionService{repo: r}
}
