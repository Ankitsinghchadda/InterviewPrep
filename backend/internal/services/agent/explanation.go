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

// ExplainInput is the question being explained. ReferenceAnswer is the
// existing short grader answer — the explainer uses it as a hint, not a
// constraint, since its job is to teach the candidate, not grade them.
type ExplainInput struct {
	QuestionTitle   string
	QuestionBody    string
	ReferenceAnswer string
	Categories      []string
	Difficulty      string
}

// ExplainResult is the learner-facing rendering of an answer.
//
// Summary is a short, conversational rephrasing (2–4 sentences) shown above
// the fold. Markdown is the long-form writeup; it can use headings, lists,
// tables, code fences, and ```mermaid fences for diagrams when the topic
// benefits from one (system design, data flow, state machines, etc).
type ExplainResult struct {
	Summary  string `json:"summary"`
	Markdown string `json:"markdown"`
}

type Explainer interface {
	Explain(ctx context.Context, in ExplainInput) (*ExplainResult, error)
}

const explainerInstruction = `You are a senior engineer turned mentor, writing a learner-facing explanation of an interview question.

You will be given:
- The interview question and any context the interviewer would share.
- A short reference answer used internally for grading (treat it as a hint about depth, not as a script).
- Optional topic tags and difficulty.

Produce two things:
1. A "summary" — 2 to 4 sentences in plain conversational prose. Pretend you're explaining the heart of the answer to a smart friend in a coffee chat. No bullet lists, no headings, no markdown — just clear sentences.
2. A "markdown" body — a deeper explanation a learner could study from. This SHOULD use markdown features:
   - Short H2/H3 headings to break the content into sections (e.g. "Why this matters", "How to think about it", "Common pitfalls", "Worth name-dropping in the interview").
   - Bullet lists for trade-offs and rules of thumb.
   - Code fences (with language tags) for snippets.
   - Tables for comparisons when natural.
   - When — and ONLY when — the topic genuinely benefits from a picture (system design, request/data flow, sequence of calls, state machines, decision trees, simple algorithms), include ONE mermaid diagram inside a fenced block tagged ` + "`mermaid`" + `. Prefer ` + "`flowchart LR`" + ` or ` + "`sequenceDiagram`" + ` syntax. Keep the diagram small (under ~10 nodes). If a diagram would feel forced, skip it.

Style:
- Plain, direct, no fluff. No "In conclusion".
- Reference concrete tech and trade-offs by name; don't hand-wave.
- Treat the reader as competent but new to this specific topic.

Return EXACTLY this JSON (no prose around it):
{
  "summary":  "<2-4 sentence conversational summary>",
  "markdown": "<long-form markdown body, may include a single ` + "```mermaid" + ` block>"
}`

// VertexExplainer is the production Explainer backed by Vertex AI.
type VertexExplainer struct {
	runner *runner.Runner
}

func NewVertexExplainer(ctx context.Context, project, location, modelName string) (*VertexExplainer, error) {
	if project == "" {
		return nil, errors.New("vertex explainer: GOOGLE_CLOUD_PROJECT is required")
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
		return nil, fmt.Errorf("vertex explainer: build model: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "answer_explainer",
		Description: "Turns a short interview reference answer into a learner-facing explanation with optional mermaid diagram.",
		Model:       model,
		Instruction: explainerInstruction,
		OutputSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"summary":  {Type: genai.TypeString, Description: "2-4 sentence conversational summary"},
				"markdown": {Type: genai.TypeString, Description: "Long-form markdown body, may include a single ```mermaid block"},
			},
			Required: []string{"summary", "markdown"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex explainer: build agent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           "interview-prep",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex explainer: build runner: %w", err)
	}
	return &VertexExplainer{runner: r}, nil
}

func (v *VertexExplainer) Explain(ctx context.Context, in ExplainInput) (*ExplainResult, error) {
	prompt := buildExplainPrompt(in)
	msg := genai.NewContentFromText(prompt, genai.RoleUser)

	var (
		raw    string
		runErr error
	)
	for ev, err := range v.runner.Run(ctx, "explainer", "explain-"+randomID(), msg, agent.RunConfig{}) {
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
		return nil, fmt.Errorf("vertex explainer: run: %w", runErr)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("vertex explainer: empty model response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("vertex explainer: no JSON: %s", trunc(raw, 200))
	}
	var out ExplainResult
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("vertex explainer: parse json: %w", err)
	}
	out.Summary = strings.TrimSpace(out.Summary)
	out.Markdown = strings.TrimSpace(out.Markdown)
	if out.Summary == "" || out.Markdown == "" {
		return nil, errors.New("vertex explainer: model returned empty summary or markdown")
	}
	return &out, nil
}

func buildExplainPrompt(in ExplainInput) string {
	var b strings.Builder
	b.WriteString("Interview question: ")
	b.WriteString(in.QuestionTitle)
	b.WriteString("\n")
	if in.QuestionBody != "" {
		b.WriteString("Interviewer's context: ")
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
		b.WriteString("\nInternal grader reference (depth hint, not a script):\n")
		b.WriteString(in.ReferenceAnswer)
		b.WriteString("\n")
	}
	b.WriteString("\nWrite the summary + markdown explanation now. Return ONLY the JSON described in your instructions.")
	return b.String()
}

// StubExplainer returns a deterministic explanation so the UI works without
// GCP credentials. It includes a mermaid diagram when the topic tags hint
// at system design, so the diagram-rendering path is exercised locally.
type StubExplainer struct{}

func (StubExplainer) Explain(_ context.Context, in ExplainInput) (*ExplainResult, error) {
	wantsDiagram := false
	for _, c := range in.Categories {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "system") || strings.Contains(lc, "architecture") || strings.Contains(lc, "design") {
			wantsDiagram = true
			break
		}
	}

	summary := "This question is really asking whether you can frame the problem, name a couple of concrete trade-offs, and ground your answer in something specific. A strong answer leads with the core idea, then earns the depth by naming real tools, numbers, or failure modes."
	if in.ReferenceAnswer != "" {
		summary = "In short: " + firstSentence(in.ReferenceAnswer) + " The interviewer is mostly listening for how you frame the trade-offs and whether you can point at a concrete example instead of staying abstract."
	}

	var md strings.Builder
	md.WriteString("## Why this matters\n\n")
	md.WriteString("Interviewers ask this to see whether you can take a fuzzy prompt and turn it into a small number of crisp decisions. The actual technology matters less than your ability to name the trade-off you're making and why.\n\n")
	md.WriteString("## How to think about it\n\n")
	md.WriteString("- Start with the **core idea in one sentence** — don't dive into details before the interviewer agrees on the frame.\n")
	md.WriteString("- Name **at least one explicit trade-off** (consistency vs availability, latency vs cost, simplicity vs flexibility).\n")
	md.WriteString("- Anchor it in a **concrete example** from work you've actually done.\n")
	md.WriteString("- End with the **one thing you'd do differently next time**.\n\n")
	if wantsDiagram {
		md.WriteString("## A small picture\n\n")
		md.WriteString("```mermaid\nflowchart LR\n    Client -->|request| API\n    API --> Queue[(Queue)]\n    Queue --> Worker\n    Worker --> DB[(Database)]\n    Worker --> Cache[(Cache)]\n```\n\n")
	}
	md.WriteString("## Common pitfalls\n\n")
	md.WriteString("- Listing technologies without saying *why* you'd pick them.\n")
	md.WriteString("- Designing for scale you'll never reach instead of the real constraint.\n")
	md.WriteString("- Forgetting to mention what *would* make you reconsider the choice.\n\n")
	md.WriteString("> This is a stub explanation. Set `AGENT_ENABLED=true` and configure Vertex AI to get a tailored explanation for this exact question.\n")

	return &ExplainResult{
		Summary:  summary,
		Markdown: md.String(),
	}, nil
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	for _, sep := range []string{". ", "! ", "? "} {
		if i := strings.Index(s, sep); i > 0 {
			return s[:i+1]
		}
	}
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
