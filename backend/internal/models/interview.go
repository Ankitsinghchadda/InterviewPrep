package models

import "time"

type Interview struct {
	ID              string     `json:"id"`
	UserID          string     `json:"userId"`
	Mode            string     `json:"mode"` // topic | adaptive | live
	CategoryIDs     []string   `json:"categoryIds"`
	Status          string     `json:"status"` // in_progress | completed | abandoned
	Score           *float64   `json:"score,omitempty"`
	Summary         string     `json:"summary,omitempty"`
	StartedAt       time.Time  `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	DurationSeconds int        `json:"durationSeconds,omitempty"` // live mode: 900|1800|2700; 0 for topic/adaptive
	JobDescription  string     `json:"jobDescription,omitempty"`  // live mode only; empty when the candidate didn't paste one

	// Optional, populated by GetInterview-style endpoints:
	Questions   []Question   `json:"questions,omitempty"`
	Submissions []Submission `json:"submissions,omitempty"`
}
