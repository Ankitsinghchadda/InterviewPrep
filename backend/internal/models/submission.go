package models

import "time"

type Submission struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	QuestionID   string    `json:"questionId"`
	InterviewID  *string   `json:"interviewId,omitempty"`
	AudioURL     string    `json:"audioUrl,omitempty"`
	Transcript   string    `json:"transcript,omitempty"`
	Feedback     string    `json:"feedback,omitempty"`
	Strengths    []string  `json:"strengths,omitempty"`
	Improvements []string  `json:"improvements,omitempty"`
	Score        *float64  `json:"score,omitempty"`
	Status       string    `json:"status"` // pending | transcribing | reviewing | complete | failed
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
