package models

import "time"

type Profile struct {
	UserID          string     `json:"userId"`
	TargetRole      string     `json:"targetRole"`           // category slug
	YearsExperience int        `json:"yearsExperience"`
	Seniority       string     `json:"seniority"`            // junior | mid | senior | staff | principal
	CurrentRole     string     `json:"currentRole"`
	TechStack       []string   `json:"techStack"`
	TargetCompanies []string   `json:"targetCompanies"`
	Goals           string     `json:"goals"`
	ResumeText      string     `json:"resumeText,omitempty"`
	ResumeFilename  string     `json:"resumeFilename,omitempty"`
	OnboardedAt     *time.Time `json:"onboardedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// IsOnboarded reports whether the user has completed initial setup.
func (p *Profile) IsOnboarded() bool {
	return p != nil && p.OnboardedAt != nil
}
