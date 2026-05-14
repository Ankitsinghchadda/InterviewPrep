package services

import "github.com/Ankitsinghchadda/InterviewPrep/internal/repository"

type TopicService struct {
	repo *repository.Repository
}

func NewTopicService(r *repository.Repository) *TopicService {
	return &TopicService{repo: r}
}
