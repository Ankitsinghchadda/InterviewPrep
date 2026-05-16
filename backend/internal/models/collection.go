package models

import "time"

// Collection is a user-owned named list of questions. Every user gets a
// lazily-created default "Saved" collection (IsDefault = true) the first
// time they hit /collections — it's the one-click bookmark target.
type Collection struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Color         string    `json:"color,omitempty"`
	IsDefault     bool      `json:"isDefault"`
	QuestionCount int       `json:"questionCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
