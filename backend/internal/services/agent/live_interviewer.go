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

// LiveTurn is one Q&A pair from the running interview conversation.
type LiveTurn struct {
	QuestionTitle   string
	QuestionIntent  string // introduction|behavioral|technical|system_design|follow_up
	CandidateAnswer string // transcript of the user's spoken reply
}

// NextQuestionInput is everything the live interviewer agent sees about the
// in-flight interview when deciding the next question.
type NextQuestionInput struct {
	Profile          DesignInput // reuse the designer's input shape (resume, role, tech, etc.)
	History          []LiveTurn  // ordered Q&A so far; empty when IsFirst
	TimeRemainingSec int
	IsFirst          bool // first call: MUST produce a warm intro question
}

// NextQuestion is the one question produced by the live interviewer agent.
type NextQuestion struct {
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Answer     string   `json:"answer"` // reference answer for the reviewer agent to grade against
	Difficulty string   `json:"difficulty"`
	Intent     string   `json:"intent"`
	Categories []string `json:"categories"`
	ShouldWrap bool     `json:"should_wrap"` // agent signals "we're done, ask client to call /complete"
}

// LiveInterviewer produces the next question, given the running transcript.
type LiveInterviewer interface {
	NextQuestion(ctx context.Context, in NextQuestionInput) (*NextQuestion, error)
}

const liveInterviewerInstruction = `You are conducting a live mock interview as a senior, friendly-but-incisive interviewer. You produce ONE next question per call.

You will be shown:
- The candidate profile (target role, experience, tech stack, goals) and an optional resume excerpt.
- The running Q&A so far, in order.
- The remaining time in seconds.
- Whether this is the FIRST question of the interview.

Rules:
1. If is_first=true: produce a warm "Tell me about yourself" / "Walk me through your background" question tailored to their target role. Intent = "introduction".
2. Otherwise: build directly on what the candidate just said. Drill into a specific claim, ask for trade-offs, or — if the topic is exhausted — pivot to a fresh axis (behavioral, technical depth, system design).
3. Reference SPECIFICS from their resume / earlier answers when possible. Do NOT be generic.
4. Difficulty should escalate over time but stay matched to seniority.
5. Time budget: if time_remaining_sec < 180 AND the interview has already covered an intro + at least one behavioral + at least one technical question, set should_wrap=true and ask a closing question (e.g., "Any questions for me?" or a brief reflection prompt).
6. If time_remaining_sec < 60, always set should_wrap=true.
7. Output JSON only, exactly this shape:

{
  "title": "<the question, the way an interviewer would ask it>",
  "body": "<optional 1-line note: what to listen for>",
  "answer": "<a strong reference answer the AI reviewer will grade against — be specific, not a one-liner>",
  "difficulty": "<easy|medium|hard>",
  "intent": "<introduction|behavioral|technical|system_design|follow_up>",
  "categories": ["<optional slugs: frontend, backend, fullstack, system-architect, devops, javascript, react, typescript, css, node, go, databases, system-design, docker, kubernetes, ci-cd, security, behavioral>"],
  "should_wrap": false
}`

// VertexLiveInterviewer is a Vertex AI / ADK-backed LiveInterviewer.
type VertexLiveInterviewer struct {
	runner *runner.Runner
}

func NewVertexLiveInterviewer(ctx context.Context, project, location, modelName string) (*VertexLiveInterviewer, error) {
	if project == "" {
		return nil, errors.New("vertex live: GOOGLE_CLOUD_PROJECT is required")
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
		return nil, fmt.Errorf("vertex live: build model: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "live_interviewer",
		Description: "Generates the next question in a live mock interview from the running transcript.",
		Model:       model,
		Instruction: liveInterviewerInstruction,
		OutputSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"title":       {Type: genai.TypeString},
				"body":        {Type: genai.TypeString},
				"answer":      {Type: genai.TypeString},
				"difficulty":  {Type: genai.TypeString},
				"intent":      {Type: genai.TypeString},
				"categories":  {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"should_wrap": {Type: genai.TypeBoolean},
			},
			Required: []string{"title", "answer", "difficulty", "intent"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex live: build agent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex live: build runner: %w", err)
	}
	return &VertexLiveInterviewer{runner: r}, nil
}

func (v *VertexLiveInterviewer) NextQuestion(ctx context.Context, in NextQuestionInput) (*NextQuestion, error) {
	prompt := buildLiveInterviewerPrompt(in)
	msg := genai.NewContentFromText(prompt, genai.RoleUser)

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, "live", "live-"+randomID(), msg, agent.RunConfig{}) {
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
		return nil, fmt.Errorf("vertex live: run: %w", runErr)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("vertex live: empty model response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("vertex live: no JSON: %s", trunc(raw, 200))
	}
	var out NextQuestion
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("vertex live: parse json: %w", err)
	}
	return &out, nil
}

func buildLiveInterviewerPrompt(in NextQuestionInput) string {
	var b strings.Builder

	b.WriteString("Candidate profile\n")
	p := in.Profile
	if p.TargetRole != "" {
		fmt.Fprintf(&b, "- Target role: %s\n", p.TargetRole)
	}
	if p.CurrentRole != "" {
		fmt.Fprintf(&b, "- Current role: %s\n", p.CurrentRole)
	}
	if p.YearsExperience > 0 {
		fmt.Fprintf(&b, "- Years of experience: %d\n", p.YearsExperience)
	}
	if p.Seniority != "" {
		fmt.Fprintf(&b, "- Seniority: %s\n", p.Seniority)
	}
	if len(p.TechStack) > 0 {
		fmt.Fprintf(&b, "- Tech stack: %s\n", strings.Join(p.TechStack, ", "))
	}
	if p.Goals != "" {
		fmt.Fprintf(&b, "- Goals: %s\n", p.Goals)
	}
	if p.ResumeText != "" {
		b.WriteString("\nResume excerpt:\n")
		b.WriteString(trunc(p.ResumeText, 4000))
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "\nTime remaining: %d seconds\n", in.TimeRemainingSec)
	fmt.Fprintf(&b, "Is first question: %t\n", in.IsFirst)

	if len(in.History) > 0 {
		b.WriteString("\nRunning Q&A transcript:\n")
		for i, t := range in.History {
			fmt.Fprintf(&b, "[%d] Interviewer (%s): %s\n", i+1, t.QuestionIntent, t.QuestionTitle)
			ans := strings.TrimSpace(t.CandidateAnswer)
			if ans == "" {
				ans = "(no audible answer)"
			}
			fmt.Fprintf(&b, "    Candidate: %s\n", trunc(ans, 1500))
		}
	}

	b.WriteString("\nProduce the next question now as JSON per your instructions.")
	return b.String()
}

// StubLiveInterviewer returns canned questions so the live flow works end-to-end
// without Vertex configured. The intent set matches the real agent.
type StubLiveInterviewer struct{}

func (StubLiveInterviewer) NextQuestion(_ context.Context, in NextQuestionInput) (*NextQuestion, error) {
	if in.IsFirst {
		role := in.Profile.TargetRole
		if role == "" {
			role = "the role you're targeting"
		}
		return &NextQuestion{
			Title:      "To kick things off — walk me through your background and what you're looking for next.",
			Body:       "Listen for clarity, a 60-90s arc, and motivation for moving roles.",
			Answer:     fmt.Sprintf("A strong opener: 60-90s, ties past roles to a thesis about what they want next, ends with why %s specifically. Names technologies and projects without rambling.", role),
			Difficulty: "easy",
			Intent:     "introduction",
			Categories: []string{"behavioral"},
			ShouldWrap: false,
		}, nil
	}

	// Wrap when time is short.
	if in.TimeRemainingSec < 60 {
		return &NextQuestion{
			Title:      "We're near the end — do you have any questions for me, or anything you wish I had asked?",
			Body:       "Closing prompt. Listen for thoughtful questions and self-awareness.",
			Answer:     "Good answers ask about team dynamics, technical scope, or how success is measured, and acknowledge a gap they didn't get to cover.",
			Difficulty: "easy",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
			ShouldWrap: true,
		}, nil
	}

	tech := "your primary technology"
	if len(in.Profile.TechStack) > 0 {
		tech = in.Profile.TechStack[0]
	}

	// Rotate through a small pool keyed by transcript length.
	pool := []NextQuestion{
		{
			Title:      "Tell me about a project from your most recent role you're proud of, and the hardest technical decision you made.",
			Body:       "Probe for ownership, trade-offs, and what they'd do differently.",
			Answer:     "STAR-style: clear context, technical detail, a specific trade-off (consistency/availability, sync/async), a concrete outcome metric, and an honest reflection.",
			Difficulty: "medium",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
		},
		{
			Title:      fmt.Sprintf("Pick the hardest bug you've debugged in %s and walk me through how you found it.", tech),
			Body:       "Tests depth + debugging instinct.",
			Answer:     "Strong answers describe what they observed, what they hypothesized, how they isolated the variable, and what surprised them.",
			Difficulty: "medium",
			Intent:     "technical",
			Categories: []string{strings.ToLower(strings.ReplaceAll(tech, " ", "-"))},
		},
		{
			Title:      "Design a system that handles 1 million write requests per minute with idempotent semantics.",
			Body:       "Standard system-design probe. Encourage thinking out loud.",
			Answer:     "Strong answer covers: idempotency keys, sharded write path (queue + worker fan-out), dedup store (Redis or DB unique index), retries/timeouts, observability, and at least one explicit trade-off.",
			Difficulty: "hard",
			Intent:     "system_design",
			Categories: []string{"system-design"},
		},
		{
			Title:      "Tell me about a time you disagreed with a teammate. How did you resolve it?",
			Body:       "Listen for empathy + a concrete resolution.",
			Answer:     "Picks a real technical or product disagreement, shows they understood the other side, used data or a small experiment, and ends with honest reflection.",
			Difficulty: "easy",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
		},
	}
	idx := len(in.History) % len(pool)
	q := pool[idx]
	q.ShouldWrap = in.TimeRemainingSec < 180
	return &q, nil
}
