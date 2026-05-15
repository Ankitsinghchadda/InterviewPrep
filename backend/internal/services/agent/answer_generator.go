package agent

import (
	"context"
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

// GenerateAnswerInput is everything the generator needs to draft a strong
// reference answer for a freshly authored interview question.
type GenerateAnswerInput struct {
	Title      string
	Body       string
	Difficulty string
	Categories []string
}

// AnswerGenerator produces a reference answer for a new question. The output
// is plain prose (no markdown) so it round-trips through the existing reviewer
// + TTS pipeline unchanged.
type AnswerGenerator interface {
	Generate(ctx context.Context, in GenerateAnswerInput) (string, error)
}

const answerGeneratorInstruction = `You are a senior engineer drafting a reference answer for an interview question. The answer will be used by an automated reviewer to grade candidates and by a text-to-speech voice to play back to the candidate, so write for the ear, not the eye.

You will be given:
- The question.
- Optional extra context the interviewer would share.
- Optional difficulty and topic tags.

Write a 200–400 word answer that a strong senior candidate would give. Plain prose, full sentences. No markdown headings, no bullet lists, no code fences — those don't read well aloud. Cover the core idea, name the trade-offs an interviewer wants to hear, and anchor it with a concrete example or number when it sharpens the answer. End on the most important takeaway.

Return ONLY the answer text. No preamble, no quotes, no JSON.`

// VertexAnswerGenerator is the production AnswerGenerator backed by Vertex AI.
type VertexAnswerGenerator struct {
	runner *runner.Runner
}

func NewVertexAnswerGenerator(ctx context.Context, project, location, modelName string) (*VertexAnswerGenerator, error) {
	if project == "" {
		return nil, errors.New("vertex answer generator: GOOGLE_CLOUD_PROJECT is required")
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
		return nil, fmt.Errorf("vertex answer generator: build model: %w", err)
	}
	a, err := llmagent.New(llmagent.Config{
		Name:        "answer_generator",
		Description: "Drafts a strong reference answer for a freshly authored interview question.",
		Model:       model,
		Instruction: answerGeneratorInstruction,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex answer generator: build agent: %w", err)
	}
	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex answer generator: build runner: %w", err)
	}
	return &VertexAnswerGenerator{runner: r}, nil
}

func (v *VertexAnswerGenerator) Generate(ctx context.Context, in GenerateAnswerInput) (string, error) {
	prompt := buildAnswerGenPrompt(in)
	msg := genai.NewContentFromText(prompt, genai.RoleUser)

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, "answer-gen", "gen-"+randomID(), msg, agent.RunConfig{}) {
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
		return "", fmt.Errorf("vertex answer generator: run: %w", runErr)
	}
	out := strings.TrimSpace(raw)
	if out == "" {
		return "", errors.New("vertex answer generator: empty model response")
	}
	return out, nil
}

func buildAnswerGenPrompt(in GenerateAnswerInput) string {
	var b strings.Builder
	b.WriteString("Question: ")
	b.WriteString(in.Title)
	b.WriteString("\n")
	if in.Body != "" {
		b.WriteString("Context: ")
		b.WriteString(in.Body)
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
	b.WriteString("\nWrite the reference answer now. Plain prose only.")
	return b.String()
}

// StubAnswerGenerator returns a canned reference answer so the save path works
// without GCP credentials. It tailors a couple of sentences to the inputs so
// the dev UI doesn't show the same paragraph for every question.
type StubAnswerGenerator struct{}

func (StubAnswerGenerator) Generate(_ context.Context, in GenerateAnswerInput) (string, error) {
	topic := "this topic"
	if len(in.Categories) > 0 {
		topic = in.Categories[0]
	}
	level := "a strong"
	switch in.Difficulty {
	case "easy":
		level = "a clear and grounded"
	case "hard":
		level = "a deeply considered"
	}
	q := strings.TrimSpace(in.Title)
	if q == "" {
		q = "the question"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s answer would start by reframing what the interviewer is really asking when they say \"%s\". ", level, q)
	fmt.Fprintf(&b, "The core idea is to lead with the trade-off, not the implementation: name the constraint you're optimizing for, then explain the mechanism that follows from it. ")
	fmt.Fprintf(&b, "In the context of %s, that usually means picking one of consistency, latency, or operational simplicity and being explicit about what you're giving up in return. ", topic)
	b.WriteString("A good candidate will anchor the answer in a concrete example — a real system they've worked on, a specific number, or a failure mode they've seen — instead of staying abstract. ")
	b.WriteString("They'll also name at least one thing they would change about their first design, which signals self-awareness. ")
	b.WriteString("The takeaway: clear framing, one explicit trade-off, one concrete example, and a single honest reflection at the end. ")
	b.WriteString("(This is a locally generated stub — set AGENT_ENABLED=true with Vertex AI configured to get a real answer.)")
	return b.String(), nil
}
