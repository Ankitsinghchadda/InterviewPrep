package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// DesignInput is what the designer agent sees about the candidate.
type DesignInput struct {
	TargetRole      string
	YearsExperience int
	Seniority       string
	CurrentRole     string
	TechStack       []string
	Goals           string
	ResumeText      string
	Count           int // total questions in the plan
}

// PlannedQuestion is one question in the adaptive interview plan.
type PlannedQuestion struct {
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Answer     string   `json:"answer"`
	Difficulty string   `json:"difficulty"`
	Intent     string   `json:"intent"`     // introduction | behavioral | technical | system_design | follow_up
	Categories []string `json:"categories"` // optional category slugs
}

type DesignedInterview struct {
	Questions []PlannedQuestion `json:"questions"`
}

type InterviewDesigner interface {
	Design(ctx context.Context, in DesignInput) (*DesignedInterview, error)
}

const designerInstruction = `You are designing a realistic mock interview for a specific candidate.

You will get the candidate's profile (target role, experience, tech stack, goals) and an optional resume excerpt.

Build an ordered list of questions that mimics a real interview flow:
1. An "introduction" question — warm, behavioral, tailored to them (e.g. "Walk me through your work at [their company]").
2. A "behavioral" question that draws on something specific from their resume or experience.
3. A "technical" deep-dive question on their PRIMARY listed technology.
4. A second "technical" question on a different listed technology — or a code/design problem matched to their seniority.
5. A "system_design" question scoped appropriately for their seniority (smaller scope for junior/mid, broader for senior+).
6+. If more questions requested, add "follow_up" questions that explore depth on already-covered areas, or another technical/behavioral as needed.

Return EXACTLY this JSON (no prose around it):
{
  "questions": [
    {
      "title": "<the question, asked the way an interviewer would ask it>",
      "body": "<optional 1-line interviewer note: hints, what to listen for>",
      "answer": "<a strong reference answer the AI reviewer will grade against — be specific>",
      "difficulty": "<easy | medium | hard, matched to seniority>",
      "intent": "<introduction | behavioral | technical | system_design | follow_up>",
      "categories": ["<optional category slugs from this set: frontend, backend, fullstack, system-architect, devops, javascript, react, typescript, css, node, go, databases, system-design, docker, kubernetes, ci-cd, security, behavioral>"]
    }
  ]
}

Rules:
- Reference SPECIFICS from their resume/experience (project names, company, tech) when relevant — don't be generic.
- "answer" must be a useful reference: bullet-points worth of substance, not a one-liner.
- Difficulty escalates from intro → behavioral → technical → system_design.
- Output exactly the requested number of questions.`

type VertexInterviewDesigner struct {
	runner *runner.Runner
}

func NewVertexInterviewDesigner(ctx context.Context, project, location, modelName string) (*VertexInterviewDesigner, error) {
	if project == "" {
		return nil, errors.New("vertex designer: GOOGLE_CLOUD_PROJECT is required")
	}
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}
	cfg := &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	}
	model, err := gemini.NewModel(ctx, modelName, cfg)
	if err != nil {
		return nil, fmt.Errorf("vertex designer: build model: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "interview_designer",
		Description: "Designs a tailored mock interview plan from a candidate profile.",
		Model:       model,
		Instruction: designerInstruction,
		OutputSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"questions": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"title":      {Type: genai.TypeString},
							"body":       {Type: genai.TypeString},
							"answer":     {Type: genai.TypeString},
							"difficulty": {Type: genai.TypeString},
							"intent":     {Type: genai.TypeString},
							"categories": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
						},
						Required: []string{"title", "answer", "difficulty", "intent"},
					},
				},
			},
			Required: []string{"questions"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex designer: build agent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex designer: build runner: %w", err)
	}
	return &VertexInterviewDesigner{runner: r}, nil
}

func (v *VertexInterviewDesigner) Design(ctx context.Context, in DesignInput) (*DesignedInterview, error) {
	if in.Count <= 0 {
		in.Count = 5
	}
	prompt := buildDesignerPrompt(in)
	msg := genai.NewContentFromText(prompt, genai.RoleUser)

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, "designer", "design-"+randomID(), msg, agent.RunConfig{}) {
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
		return nil, fmt.Errorf("vertex designer: run: %w", runErr)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("vertex designer: empty model response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("vertex designer: no JSON: %s", trunc(raw, 200))
	}
	var out DesignedInterview
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("vertex designer: parse json: %w", err)
	}
	return &out, nil
}

func buildDesignerPrompt(in DesignInput) string {
	var b strings.Builder
	b.WriteString("Candidate profile\n")
	if in.TargetRole != "" {
		fmt.Fprintf(&b, "- Target role: %s\n", in.TargetRole)
	}
	if in.CurrentRole != "" {
		fmt.Fprintf(&b, "- Current role: %s\n", in.CurrentRole)
	}
	if in.YearsExperience > 0 {
		fmt.Fprintf(&b, "- Years of experience: %d\n", in.YearsExperience)
	}
	if in.Seniority != "" {
		fmt.Fprintf(&b, "- Seniority: %s\n", in.Seniority)
	}
	if len(in.TechStack) > 0 {
		fmt.Fprintf(&b, "- Tech stack: %s\n", strings.Join(in.TechStack, ", "))
	}
	if in.Goals != "" {
		fmt.Fprintf(&b, "- Goals: %s\n", in.Goals)
	}
	if in.ResumeText != "" {
		b.WriteString("\nResume excerpt:\n")
		// Cap to keep token usage reasonable.
		b.WriteString(trunc(in.ResumeText, 4000))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nBuild a %d-question interview plan now. Return the JSON described in your instructions.", in.Count)
	return b.String()
}

// StubInterviewDesigner returns a fixed plan slightly tailored to the input.
type StubInterviewDesigner struct{}

func (StubInterviewDesigner) Design(_ context.Context, in DesignInput) (*DesignedInterview, error) {
	tech := "your primary technology"
	if len(in.TechStack) > 0 {
		tech = in.TechStack[0]
	}
	role := in.TargetRole
	if role == "" {
		role = "backend"
	}
	if in.Count <= 0 {
		in.Count = 5
	}

	base := []PlannedQuestion{
		{
			Title:      "Walk me through your career so far and what you're looking for next.",
			Body:       "Listen for clarity, recency of work, and motivation for moving roles.",
			Answer:     "A strong answer is a 90-second arc: the candidate frames each role they've held in terms of impact, ties their growth into a thesis about what they want next, and ends with why this role specifically. They name technologies and projects without rambling.",
			Difficulty: "easy",
			Intent:     "introduction",
			Categories: []string{role, "behavioral"},
		},
		{
			Title:      "Tell me about a project from your most recent role that you're proud of and the hardest technical decision you made.",
			Body:       "Probe for ownership, trade-offs, and what they'd do differently.",
			Answer:     "STAR-style: clear context, technical detail on the problem, specific trade-off they weighed (eg consistency vs availability, sync vs async), the outcome with a concrete metric, and an honest reflection on what they'd change.",
			Difficulty: "medium",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
		},
		{
			Title:      fmt.Sprintf("How would you explain %s to someone joining your team who's new to it?", tech),
			Body:       "Tests both depth of knowledge and ability to communicate it.",
			Answer:     fmt.Sprintf("They should start from the model %s uses (memory, execution, runtime), call out its real-world strengths, identify the common foot-guns juniors hit, and give a small concrete example. Bonus for noting what NOT to use it for.", tech),
			Difficulty: "medium",
			Intent:     "technical",
			Categories: []string{role, strings.ToLower(strings.ReplaceAll(tech, " ", "-"))},
		},
		{
			Title:      "Design a system that handles 1 million write requests per minute with idempotent semantics.",
			Body:       "Standard system-design probe. Encourage thinking out loud.",
			Answer:     "Strong answer covers: API design with an idempotency key, sharded write path (queue + worker fan-out), a deduplication store (Redis or DB unique index on key), retry/timeout semantics, observability, and at least one explicit trade-off (consistency window, cost of dedup window, replay strategy).",
			Difficulty: "hard",
			Intent:     "system_design",
			Categories: []string{"system-design"},
		},
		{
			Title:      "Tell me about a time you disagreed with a teammate. How did you resolve it?",
			Body:       "Listen for empathy + concrete resolution.",
			Answer:     "A good answer picks a real technical or product disagreement (not personality), shows they understood the other side, used data or a small experiment to break the tie, and ends with an honest reflection. Avoids making the teammate the villain.",
			Difficulty: "easy",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
		},
	}

	// Truncate or pad to requested count.
	if in.Count <= len(base) {
		return &DesignedInterview{Questions: base[:in.Count]}, nil
	}
	out := append([]PlannedQuestion{}, base...)
	for i := len(out); i < in.Count; i++ {
		out = append(out, PlannedQuestion{
			Title:      "Walk me through a follow-up question on something you mentioned earlier.",
			Body:       "Adaptive follow-up placeholder (set AGENT_ENABLED=true for tailored content).",
			Answer:     "An honest, specific extension of an earlier topic with concrete trade-offs.",
			Difficulty: "medium",
			Intent:     "follow_up",
			Categories: []string{role},
		})
	}
	return &DesignedInterview{Questions: out}, nil
}
