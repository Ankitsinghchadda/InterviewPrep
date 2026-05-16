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

// DesignInput is what the designer agent sees about the candidate.
type DesignInput struct {
	TargetRole      string
	YearsExperience int
	Seniority       string
	CurrentRole     string
	TechStack       []string
	Goals           string
	ResumeText      string
	JobDescription  string // live mode only; empty in the adaptive flow
	Count           int    // total questions in the plan
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
      "answer": "<the model answer the candidate should hear back — first person, plain prose, 150–300 words, what a strong candidate would actually SAY>",
      "difficulty": "<easy | medium | hard, matched to seniority>",
      "intent": "<introduction | behavioral | technical | system_design | follow_up>",
      "categories": ["<optional category slugs from this set: frontend, backend, fullstack, system-architect, devops, javascript, react, typescript, css, node, go, databases, system-design, docker, kubernetes, ci-cd, security, behavioral>"]
    }
  ]
}

Rules:
- Reference SPECIFICS from their resume/experience (project names, company, tech) when relevant — don't be generic.
- "answer" must be a first-person model answer the candidate could speak verbatim — full sentences, plain prose, 150–300 words, anchored with at least one concrete example or number. NO meta-language like "A strong answer should..." or "The candidate should...". NO bullets, headings, or code fences (this text is also read aloud by TTS).
- Difficulty escalates from intro → behavioral → technical → system_design.
- Output exactly the requested number of questions.`

type VertexInterviewDesigner struct {
	runner *runner.Runner
}

func NewInterviewDesigner(ctx context.Context, backend Backend, modelName, project, location, apiKey string) (*VertexInterviewDesigner, error) {
	model, err := BuildGeminiModel(ctx, backend, modelName, project, location, apiKey)
	if err != nil {
		return nil, fmt.Errorf("designer: %w", err)
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
			Answer:     "Sure — I've been building software for the last several years, mostly on the backend side. In my current role I own a service that sits in the request path for a meaningful share of our traffic, and the work I'm most proud of is a piece of that where we replaced a synchronous fan-out with a queue-backed pipeline and cut p99 latency roughly in half. Before that I was at a smaller team where I got a lot of breadth — I touched everything from the database layer up to the frontend, which I think made me a better engineer because I stopped seeing the stack as somebody else's problem. What I'm looking for next is a place where the engineering bar is genuinely high and there's real ownership — I'd rather go deep on one or two hard problems than touch ten shallow ones. The reason this role caught my eye is the problem space lines up with what I want to get better at, and the team seems to actually invest in engineering quality, not just talk about it.",
			Difficulty: "easy",
			Intent:     "introduction",
			Categories: []string{role, "behavioral"},
		},
		{
			Title:      "Tell me about a project from your most recent role that you're proud of and the hardest technical decision you made.",
			Body:       "Probe for ownership, trade-offs, and what they'd do differently.",
			Answer:     "The project I'm proudest of is a rewrite of our ingestion pipeline. The old system was a single synchronous path that fanned out to four downstream consumers, and once traffic doubled it started timing out under spikes. The hardest decision was whether to patch it with a thread-pool tuning pass — which I could ship in a week — or do the right thing and put a queue in front. I went with the queue, even though it meant a month of work and an at-least-once semantics conversation with every downstream team. The explicit trade-off I made was throughput-and-resilience versus exactly-once delivery — we picked throughput plus idempotency keys, because reconciliation was cheaper than back-pressuring the producers. After it shipped, p99 dropped about 60% and we stopped paging on traffic spikes. If I did it again I'd invest earlier in load tests — we caught one bug in production that a serious load test would have flagged in staging.",
			Difficulty: "medium",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
		},
		{
			Title:      fmt.Sprintf("How would you explain %s to someone joining your team who's new to it?", tech),
			Body:       "Tests both depth of knowledge and ability to communicate it.",
			Answer:     fmt.Sprintf("The way I'd start is with the model %s actually uses under the hood — because most of the foot-guns a newcomer hits come from assuming it works like whatever language they came from. So I'd walk through how memory and execution flow, then give them one small example where the obvious mental model gives the wrong answer — those moments are when the model actually sticks. From there I'd talk about real-world strengths: the things %s makes easy that would be a pain elsewhere, and where the ecosystem is genuinely strong. I'd also be honest about what it's NOT a great fit for, because a junior engineer reaching for the wrong tool is usually how teams accumulate the kind of tech debt that's hard to unwind. The thing I'd land on is: read the standard library source for one or two packages you use every day, because that's where you actually internalize idiomatic style.", tech, tech),
			Difficulty: "medium",
			Intent:     "technical",
			Categories: []string{role, strings.ToLower(strings.ReplaceAll(tech, " ", "-"))},
		},
		{
			Title:      "Design a system that handles 1 million write requests per minute with idempotent semantics.",
			Body:       "Standard system-design probe. Encourage thinking out loud.",
			Answer:     "Sure — at a million writes per minute, that's about seventeen thousand per second, so I'm thinking distributed from the start, not one beefy node. The first thing I'd put in is an idempotency key carried by the client on every request, typically a UUID, and the API layer would check that key against a dedup store before touching the database. For the dedup store I'd start with Redis with a TTL on each key, because the latency budget at the edge is tight; if persistence matters I'd back it with a unique index on the primary database as defense in depth. After the dedup check I'd push the write onto a partitioned queue — Kafka or equivalent — keyed by the customer or entity id so all writes for the same entity stay in order. Workers consume per-partition and apply writes. The trade-offs I'd call out explicitly: this gives me at-least-once delivery, so every consumer below the queue has to be idempotent too; and the dedup window is bounded by the TTL, so a replay after the TTL expires would slip through unless I make the unique index the real source of truth. For observability I'd publish queue lag and dedup-hit-rate — those two numbers tell you everything about whether the system is healthy.",
			Difficulty: "hard",
			Intent:     "system_design",
			Categories: []string{"system-design"},
		},
		{
			Title:      "Tell me about a time you disagreed with a teammate. How did you resolve it?",
			Body:       "Listen for empathy + concrete resolution.",
			Answer:     "Sure — a few months ago I disagreed with a teammate on whether to add a feature flag for a refactor we were rolling out. They argued the flag added complexity and a rollback path we'd never use; I thought without it we'd be one bad merge away from a Friday-night page. We talked it through and I realized their underlying concern was that flags get added and never removed — they were pushing back on a real anti-pattern, not on the rollout itself. So we agreed on the flag, but we also agreed on a removal date written into the ticket, and I owned the cleanup PR. It shipped, the flag came out two weeks later, and we ended up using the rollback exactly once when a downstream service started returning a slightly different schema — so the flag paid for itself. The thing I learned is that the strongest position is usually the one that takes the other person's objection seriously and bakes the answer into the plan, instead of arguing the objection away.",
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
			Answer:     "To pick up on what I said earlier — the part I wanted to expand on is the trade-off I glossed over. The reason I didn't lead with it is that in the projects I had in mind, the load profile let us get away with a simpler approach for a long time. Once you scale past that point, though, you really do have to pick a side, and the right move is to be explicit about which guarantee you're willing to weaken — usually latency for consistency, or development speed for operational simplicity. A concrete example would be moving a write path from synchronous double-writes to an async outbox: it bought us throughput, but we had to invest real work into reconciling eventual consistency for downstream consumers.",
			Difficulty: "medium",
			Intent:     "follow_up",
			Categories: []string{role},
		})
	}
	return &DesignedInterview{Questions: out}, nil
}
