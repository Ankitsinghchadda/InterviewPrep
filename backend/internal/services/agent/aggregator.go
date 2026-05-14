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

// AggregateInput is one row of per-question feedback fed into the interviewer
// aggregator agent.
type AggregateInput struct {
	QuestionTitle string
	Difficulty    string
	Score         float64
	Strengths     []string
	Improvements  []string
	Feedback      string
}

// AggregateResult is the overall mock-interview verdict.
type AggregateResult struct {
	Score   float64 `json:"score"`   // 0..100 overall
	Summary string  `json:"summary"` // 2-3 sentence written-to-candidate summary
}

// Aggregator combines per-question reviews into an overall interview result.
type Aggregator interface {
	Aggregate(ctx context.Context, items []AggregateInput) (*AggregateResult, error)
}

const aggregatorInstruction = `You are a senior interviewer writing the final report for a candidate's mock interview.

You are given the per-question scores and feedback from each answer. Synthesize:
- An overall score from 0 to 100. Weigh all questions; do NOT just average — give more weight to clarity of communication and demonstrated understanding. Be honest. If half the answers were weak the overall should reflect that.
- A 2-3 sentence "summary" written directly to the candidate in second person, calling out their main strength and the single most important thing to work on.

Return EXACTLY this JSON shape with no surrounding prose:
{
  "score": <int 0..100>,
  "summary": "<2-3 sentences, second person>"
}`

type VertexAggregator struct {
	runner *runner.Runner
}

func NewVertexAggregator(ctx context.Context, project, location, modelName string) (*VertexAggregator, error) {
	if project == "" {
		return nil, errors.New("vertex aggregator: GOOGLE_CLOUD_PROJECT is required")
	}
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}

	clientCfg := &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	}

	model, err := gemini.NewModel(ctx, modelName, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("vertex aggregator: build model: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "interview_aggregator",
		Description: "Synthesizes per-question feedback into an overall mock interview verdict.",
		Model:       model,
		Instruction: aggregatorInstruction,
		OutputSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"score":   {Type: genai.TypeNumber, Description: "Overall interview score 0..100"},
				"summary": {Type: genai.TypeString, Description: "2-3 sentences, second person"},
			},
			Required: []string{"score", "summary"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex aggregator: build llmagent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex aggregator: build runner: %w", err)
	}
	return &VertexAggregator{runner: r}, nil
}

func (v *VertexAggregator) Aggregate(ctx context.Context, items []AggregateInput) (*AggregateResult, error) {
	if len(items) == 0 {
		return nil, errors.New("aggregator: no items")
	}

	prompt := buildAggregatePrompt(items)
	msg := genai.NewContentFromText(prompt, genai.RoleUser)

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, "aggregator", "agg-"+randomID(), msg, agent.RunConfig{}) {
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
		return nil, fmt.Errorf("vertex aggregator: run: %w", runErr)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("vertex aggregator: empty model response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("vertex aggregator: no JSON object: %s", trunc(raw, 200))
	}
	var out AggregateResult
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("vertex aggregator: parse JSON: %w", err)
	}
	if out.Score < 0 {
		out.Score = 0
	}
	if out.Score > 100 {
		out.Score = 100
	}
	return &out, nil
}

func buildAggregatePrompt(items []AggregateInput) string {
	var b strings.Builder
	b.WriteString("Per-question results from the mock interview:\n\n")
	for i, it := range items {
		fmt.Fprintf(&b, "Question %d — %s (%s)\n", i+1, it.QuestionTitle, it.Difficulty)
		fmt.Fprintf(&b, "  Score: %.0f / 100\n", it.Score)
		if len(it.Strengths) > 0 {
			b.WriteString("  Strengths: ")
			b.WriteString(strings.Join(it.Strengths, "; "))
			b.WriteString("\n")
		}
		if len(it.Improvements) > 0 {
			b.WriteString("  Improvements: ")
			b.WriteString(strings.Join(it.Improvements, "; "))
			b.WriteString("\n")
		}
		if it.Feedback != "" {
			b.WriteString("  Reviewer note: ")
			b.WriteString(it.Feedback)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Return the JSON described in your instructions.")
	return b.String()
}

// StubAggregator returns a simple averaged score and canned summary. Used when
// AGENT_ENABLED=false so the wizard works without GCP credentials.
type StubAggregator struct{}

func (StubAggregator) Aggregate(_ context.Context, items []AggregateInput) (*AggregateResult, error) {
	if len(items) == 0 {
		return &AggregateResult{Score: 0, Summary: "No answers were submitted."}, nil
	}
	var total float64
	for _, it := range items {
		total += it.Score
	}
	avg := total / float64(len(items))
	return &AggregateResult{
		Score: avg,
		Summary: "This is stub aggregation for local development — your overall score is the average of your per-question scores. " +
			"Configure GOOGLE_CLOUD_PROJECT and set AGENT_ENABLED=true to get a real synthesized review from a Vertex AI agent.",
	}, nil
}
