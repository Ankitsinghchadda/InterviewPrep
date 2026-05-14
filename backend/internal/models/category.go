package models

import "time"

type Category struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"` // "role" | "topic"
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
}
