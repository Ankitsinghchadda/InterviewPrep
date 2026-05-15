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

// GenerateInput tells the generator which slice of the question library to
// expand. ExistingTitles seeds an "avoid these" list so the model doesn't
// regenerate questions we already have.
type GenerateInput struct {
	Categories     []string // required: target category slugs (e.g. ["kubernetes", "devops"])
	Difficulty     string   // optional: easy | medium | hard; empty = mix
	Count          int      // 1..10; defaults to 5
	ExistingTitles []string // optional: titles to avoid duplicating
}

// QuestionGenerator produces fresh Q+A pairs for the public catalog.
type QuestionGenerator interface {
	Generate(ctx context.Context, in GenerateInput) (*DesignedInterview, error)
}

const generatorInstruction = `You are extending an interview-prep question library.

You'll receive: a set of topic/role category slugs, a difficulty, a count, and an "AVOID" list of existing question titles in that slice of the library.

Generate exactly the requested number of ORIGINAL interview questions for those categories. Each item must include a strong reference answer that an AI reviewer can grade candidates against.

Return EXACTLY this JSON (no prose around it):
{
  "questions": [
    {
      "title": "<the question, asked the way an interviewer would ask it>",
      "body": "<optional 1-line interviewer note: hints, what to listen for>",
      "answer": "<a strong reference answer — bullet-points worth of substance, not a one-liner>",
      "difficulty": "<easy | medium | hard>",
      "intent": "technical",
      "categories": ["<echo the category slugs you were given>"]
    }
  ]
}

Rules:
- Do NOT repeat, rephrase, or paraphrase any question in the AVOID list.
- Cover meaningfully different aspects of the topic — don't pile multiple questions on the same sub-area.
- Each "answer" should read like a senior engineer's reference: concrete trade-offs, named techniques, mention common pitfalls.
- Honor the requested difficulty for every item if one is given; otherwise spread across easy/medium/hard.
- Output exactly the requested number of questions.`

type VertexQuestionGenerator struct {
	runner *runner.Runner
}

func NewVertexQuestionGenerator(ctx context.Context, project, location, modelName string) (*VertexQuestionGenerator, error) {
	if project == "" {
		return nil, errors.New("vertex question generator: GOOGLE_CLOUD_PROJECT is required")
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
		return nil, fmt.Errorf("vertex question generator: build model: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "question_generator",
		Description: "Generates original interview questions for a given set of categories.",
		Model:       model,
		Instruction: generatorInstruction,
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
						Required: []string{"title", "answer", "difficulty"},
					},
				},
			},
			Required: []string{"questions"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex question generator: build agent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex question generator: build runner: %w", err)
	}
	return &VertexQuestionGenerator{runner: r}, nil
}

func (v *VertexQuestionGenerator) Generate(ctx context.Context, in GenerateInput) (*DesignedInterview, error) {
	if in.Count <= 0 {
		in.Count = 5
	}
	prompt := buildGeneratorPrompt(in)
	msg := genai.NewContentFromText(prompt, genai.RoleUser)

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, "generator", "gen-"+randomID(), msg, agent.RunConfig{}) {
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
		return nil, fmt.Errorf("vertex question generator: run: %w", runErr)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("vertex question generator: empty model response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("vertex question generator: no JSON: %s", trunc(raw, 200))
	}
	var out DesignedInterview
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("vertex question generator: parse json: %w", err)
	}
	return &out, nil
}

func buildGeneratorPrompt(in GenerateInput) string {
	var b strings.Builder
	b.WriteString("Categories: ")
	b.WriteString(strings.Join(in.Categories, ", "))
	b.WriteString("\n")
	if in.Difficulty != "" {
		fmt.Fprintf(&b, "Difficulty: %s\n", in.Difficulty)
	} else {
		b.WriteString("Difficulty: mix of easy/medium/hard\n")
	}
	fmt.Fprintf(&b, "Count: %d\n", in.Count)

	if len(in.ExistingTitles) > 0 {
		b.WriteString("\nAVOID — do not repeat or paraphrase any of these existing titles:\n")
		for _, t := range in.ExistingTitles {
			b.WriteString("- ")
			b.WriteString(trunc(t, 240))
			b.WriteString("\n")
		}
	}

	b.WriteString("\nGenerate the JSON described in your instructions now.")
	return b.String()
}

// StubQuestionGenerator returns a small, deterministic batch so the feature
// remains usable when AGENT_ENABLED=false (no Vertex credentials).
type StubQuestionGenerator struct{}

func (StubQuestionGenerator) Generate(_ context.Context, in GenerateInput) (*DesignedInterview, error) {
	if in.Count <= 0 {
		in.Count = 5
	}
	topic := "the topic"
	if len(in.Categories) > 0 {
		topic = in.Categories[0]
	}
	diff := in.Difficulty
	if diff == "" {
		diff = "medium"
	}
	out := make([]PlannedQuestion, 0, in.Count)
	for i := 0; i < in.Count; i++ {
		out = append(out, PlannedQuestion{
			Title:      fmt.Sprintf("[stub %d] Walk me through how you'd reason about %s in a production system.", i+1, topic),
			Body:       "Stub generator placeholder (set AGENT_ENABLED=true for AI-authored questions).",
			Answer:     fmt.Sprintf("A strong answer ties %s back to concrete production concerns: failure modes, observability, and the trade-offs the candidate has weighed in real systems.", topic),
			Difficulty: diff,
			Intent:     "technical",
			Categories: in.Categories,
		})
	}
	return &DesignedInterview{Questions: out}, nil
}
