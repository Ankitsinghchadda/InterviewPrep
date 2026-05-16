package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// ExtractedProfile is what the resume extractor agent returns. The handler
// upserts these fields into the user_profiles row.
type ExtractedProfile struct {
	YearsExperience int      `json:"yearsExperience"`
	Seniority       string   `json:"seniority"` // junior | mid | senior | staff | principal | ""
	CurrentRole     string   `json:"currentRole"`
	TargetRole      string   `json:"targetRole"`
	TechStack       []string `json:"techStack"`
	Goals           string   `json:"goals"`
	ResumePlainText string   `json:"resumePlainText"`
}

type ProfileExtractor interface {
	Extract(ctx context.Context, fileBytes []byte, mimeType, filename string) (*ExtractedProfile, error)
}

const profileExtractorInstruction = `You are extracting structured profile data from a candidate's resume.

Return EXACTLY this JSON shape (no prose around it):
{
  "yearsExperience": <integer 0..50, best estimate from work history>,
  "seniority": "<one of: junior | mid | senior | staff | principal | "">",
  "currentRole": "<current job title and company, e.g. 'Backend Engineer at Acme'>",
  "targetRole": "<one of: frontend | backend | fullstack | system-architect | devops>",
  "techStack": ["Go", "Postgres", "Kubernetes", ...],
  "goals": "<one sentence about what kind of role they're aiming for>",
  "resumePlainText": "<a clean plain-text version of the entire resume>"
}

Rules:
- Only fill what's actually in the resume. Use empty strings / empty arrays if missing.
- "seniority" should reflect total experience: 0-2y = junior, 3-5y = mid, 6-9y = senior, 10+ = staff/principal.
- "techStack" is up to 15 items: languages, frameworks, infra/tools the candidate has hands-on experience with.
- "resumePlainText" is REQUIRED. Strip layout, but preserve section structure (one section per blank-line).`

type VertexProfileExtractor struct {
	runner *runner.Runner
}

func NewProfileExtractor(ctx context.Context, backend Backend, modelName, project, location, apiKey string) (*VertexProfileExtractor, error) {
	model, err := BuildGeminiModel(ctx, backend, modelName, project, location, apiKey)
	if err != nil {
		return nil, fmt.Errorf("profile extractor: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "resume_extractor",
		Description: "Extracts a structured candidate profile from an uploaded resume.",
		Model:       model,
		Instruction: profileExtractorInstruction,
		OutputSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"yearsExperience": {Type: genai.TypeInteger},
				"seniority":       {Type: genai.TypeString},
				"currentRole":     {Type: genai.TypeString},
				"targetRole":      {Type: genai.TypeString},
				"techStack":       {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"goals":           {Type: genai.TypeString},
				"resumePlainText": {Type: genai.TypeString},
			},
			Required: []string{"yearsExperience", "techStack", "resumePlainText"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex extractor: build agent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex extractor: build runner: %w", err)
	}
	return &VertexProfileExtractor{runner: r}, nil
}

func (v *VertexProfileExtractor) Extract(ctx context.Context, fileBytes []byte, mimeType, filename string) (*ExtractedProfile, error) {
	if len(fileBytes) == 0 {
		return nil, errors.New("vertex extractor: empty file")
	}

	hint := fmt.Sprintf("Filename: %s\nExtract structured profile data from the attached resume.", filename)
	parts := []*genai.Part{
		{Text: hint},
		{InlineData: &genai.Blob{Data: fileBytes, MIMEType: mimeType}},
	}
	msg := genai.NewContentFromParts(parts, genai.RoleUser)

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, "extractor", "extract-"+randomID(), msg, agent.RunConfig{}) {
		if err != nil {
			runErr = err
			break
		}
		if ev == nil || !ev.IsFinalResponse() || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p != nil && p.Text != "" {
				raw += p.Text
			}
		}
	}
	if runErr != nil {
		return nil, fmt.Errorf("vertex extractor: run: %w", runErr)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("vertex extractor: empty model response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("vertex extractor: no JSON: %s", trunc(raw, 200))
	}
	var out ExtractedProfile
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("vertex extractor: parse json: %w", err)
	}
	if out.YearsExperience < 0 {
		out.YearsExperience = 0
	}
	if out.YearsExperience > 60 {
		out.YearsExperience = 60
	}
	return &out, nil
}

// StubProfileExtractor returns a canned profile and the raw bytes as text.
// Used when AGENT_ENABLED=false; lets the upload UX still work end-to-end.
type StubProfileExtractor struct{}

func (StubProfileExtractor) Extract(_ context.Context, fileBytes []byte, _, _ string) (*ExtractedProfile, error) {
	// Try to decode as UTF-8 text; if it's binary (PDF), don't include it.
	plain := ""
	if isProbablyText(fileBytes) {
		plain = string(fileBytes)
	} else {
		plain = "[stub extractor — binary file uploaded; configure AGENT_ENABLED=true to extract resume text via Vertex AI]"
	}
	return &ExtractedProfile{
		YearsExperience: 3,
		Seniority:       "mid",
		CurrentRole:     "Software Engineer",
		TargetRole:      "backend",
		TechStack:       []string{"Go", "Postgres", "Docker"},
		Goals:           "Aiming for a senior backend role.",
		ResumePlainText: plain,
	}, nil
}

func isProbablyText(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	// PDFs start with %PDF-; that's a clear non-text signal.
	if len(b) > 4 && string(b[:4]) == "%PDF" {
		return false
	}
	for _, c := range b[:min(len(b), 256)] {
		if c == 0 {
			return false
		}
	}
	return true
}
