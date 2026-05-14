package services

import "github.com/Ankitsinghchadda/InterviewPrep/internal/repository"

type InterviewService struct {
	repo *repository.Repository
}

func NewInterviewService(r *repository.Repository) *InterviewService {
	return &InterviewService{repo: r}
}
