package models

import "time"

type Question struct {
	ID                  string    `json:"id"`
	Slug                *string   `json:"slug,omitempty"`
	Title               string    `json:"title"`
	Body                string    `json:"body"`
	Answer              string    `json:"answer"`
	Difficulty          string    `json:"difficulty"`
	AnswerAudioURL      string    `json:"answerAudioUrl,omitempty"`
	ExplanationSummary  string    `json:"explanationSummary,omitempty"`
	ExplanationMarkdown string    `json:"explanationMarkdown,omitempty"`
	OwnerID             *string   `json:"ownerId,omitempty"`
	IsPublic            bool      `json:"isPublic"`
	Source              string    `json:"source"` // curated | user | adaptive
	Intent              string    `json:"intent,omitempty"`
	Categories          []string  `json:"categories"`
	CreatedAt           time.Time `json:"createdAt"`
}
