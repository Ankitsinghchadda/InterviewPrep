package agent

import (
	"context"
	"crypto/rand"
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

// Vertex is an ADK-backed Reviewer that talks to Vertex AI.
type Vertex struct {
	runner *runner.Runner
}

const reviewerInstruction = `You are an honest, experienced senior interviewer evaluating a candidate's spoken answer to a technical interview question.

You are given:
- The question and (when available) a reference answer.
- The candidate's transcribed spoken answer.

Score the candidate from 0 to 100 based on technical accuracy, depth, structure, and how clearly the answer communicates the idea. Be honest — don't inflate scores for surface-level answers.

Return EXACTLY this JSON shape (no prose around it):
{
  "score": <int 0..100>,
  "strengths": [<2-4 short, specific bullets calling out things they did well>],
  "improvements": [<2-4 short, specific, actionable bullets — what to add or change>],
  "feedback": "<2-3 sentence prose summary, written to the candidate in second person>"
}

Rules:
- "strengths" and "improvements" should each contain 2 to 4 items.
- Be specific. Avoid generic phrases like "good answer".
- If the answer is empty or off-topic, score low (under 30) and explain why.`

// NewReviewer constructs a Reviewer backed by an ADK llmagent + runner.
// backend selects Vertex AI (free tier, flash) or the Gemini API (paid tier,
// pro). The caller passes whichever set of credentials is relevant.
func NewReviewer(ctx context.Context, backend Backend, modelName, project, location, apiKey string) (*Vertex, error) {
	model, err := BuildGeminiModel(ctx, backend, modelName, project, location, apiKey)
	if err != nil {
		return nil, fmt.Errorf("reviewer: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "answer_reviewer",
		Description: "Reviews a candidate's interview answer and returns a structured score with strengths and improvements.",
		Model:       model,
		Instruction: reviewerInstruction,
		OutputSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"score":        {Type: genai.TypeNumber, Description: "Overall score 0..100"},
				"strengths":    {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"improvements": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"feedback":     {Type: genai.TypeString, Description: "Short prose feedback"},
			},
			Required: []string{"score", "strengths", "improvements", "feedback"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex: build llmagent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex: build runner: %w", err)
	}

	return &Vertex{runner: r}, nil
}

func (v *Vertex) Review(ctx context.Context, in ReviewInput) (*ReviewResult, error) {
	return v.ReviewStream(ctx, in, nil)
}

// ReviewStream runs the agent in SSE mode and forwards partial text chunks to
// onToken as they arrive. The final, complete model output is parsed into the
// ReviewResult at the end. onToken may be nil (equivalent to Review).
func (v *Vertex) ReviewStream(ctx context.Context, in ReviewInput, onToken func(string)) (*ReviewResult, error) {
	prompt := buildReviewPrompt(in)
	msg := genai.NewContentFromText(prompt, genai.RoleUser)

	// One ephemeral session per request keeps reviews independent.
	userID := "reviewer"
	sessionID := "review-" + randomID()
	cfg := agent.RunConfig{StreamingMode: agent.StreamingModeSSE}

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, userID, sessionID, msg, cfg) {
		if err != nil {
			runErr = err
			break
		}
		if ev == nil || ev.Content == nil {
			continue
		}

		// Partial events: forward the delta text to onToken so the caller can
		// stream it to the client. ADK only sets Partial=true for in-flight
		// text deltas in SSE mode.
		if ev.Partial {
			if onToken != nil {
				for _, p := range ev.Content.Parts {
					if p != nil && p.Text != "" {
						onToken(p.Text)
					}
				}
			}
			continue
		}

		// Final event: accumulate the full text for JSON parsing.
		if !ev.IsFinalResponse() {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p != nil && p.Text != "" {
				raw += p.Text
			}
		}
	}
	if runErr != nil {
		return nil, fmt.Errorf("vertex: run: %w", runErr)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("vertex: empty model response")
	}

	return parseReviewJSON(raw)
}

func buildReviewPrompt(in ReviewInput) string {
	var b strings.Builder
	b.WriteString("Question: ")
	b.WriteString(in.QuestionTitle)
	b.WriteString("\n")
	if in.QuestionBody != "" {
		b.WriteString("Context: ")
		b.WriteString(in.QuestionBody)
		b.WriteString("\n")
	}
	if in.Difficulty != "" {
		b.WriteString("Difficulty: ")
		b.WriteString(in.Difficulty)
		b.WriteString("\n")
	}
	if len(in.Categories) > 0 {
		b.WriteString("Topics: ")
		b.WriteString(strings.Join(in.Categories, ", "))
		b.WriteString("\n")
	}
	if in.ReferenceAnswer != "" {
		b.WriteString("\nReference answer (what a strong answer covers):\n")
		b.WriteString(in.ReferenceAnswer)
		b.WriteString("\n")
	}
	b.WriteString("\nCandidate's spoken answer (transcribed):\n")
	if strings.TrimSpace(in.CandidateAnswer) == "" {
		b.WriteString("(empty — the candidate said nothing or audio was inaudible)")
	} else {
		b.WriteString(in.CandidateAnswer)
	}
	b.WriteString("\n\nReturn the JSON described in your instructions.")
	return b.String()
}

func parseReviewJSON(s string) (*ReviewResult, error) {
	// Models sometimes wrap JSON in ```json fences or prose. Extract the
	// outermost object substring.
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object in model output: %s", trunc(s, 200))
	}
	payload := s[start : end+1]

	var out ReviewResult
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		return nil, fmt.Errorf("parse review json: %w (raw=%q)", err, trunc(payload, 200))
	}
	if out.Score < 0 {
		out.Score = 0
	}
	if out.Score > 100 {
		out.Score = 100
	}
	if out.Strengths == nil {
		out.Strengths = []string{}
	}
	if out.Improvements == nil {
		out.Improvements = []string{}
	}
	return &out, nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// randomID returns a short ID derived from crypto/rand — fine for session keys.
func randomID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	var b [12]byte
	_, _ = rand.Read(b[:])
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b[:])
}
