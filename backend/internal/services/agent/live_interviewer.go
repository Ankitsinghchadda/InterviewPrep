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

// LiveTurn is one Q&A pair from the running interview conversation,
// enriched with the reviewer agent's verdict so the live interviewer can
// decide whether to follow up, pivot to a new axis, or close.
type LiveTurn struct {
	QuestionTitle   string
	QuestionIntent  string // introduction|behavioral|technical|system_design|follow_up|closing
	CandidateAnswer string // transcript of the user's spoken reply

	AnswerScore     *float64 // 0-100 from reviewer; nil if review failed or wasn't run
	AnswerStrengths []string
	AnswerGaps      []string // mirrors Submission.Improvements
	TurnDurationSec int      // submission.UpdatedAt - question.CreatedAt
}

// NextQuestionInput is everything the live interviewer agent sees about the
// in-flight interview when deciding the next question.
type NextQuestionInput struct {
	Profile          DesignInput // reuse the designer's input shape (resume, role, tech, etc.)
	History          []LiveTurn  // ordered Q&A so far; empty when IsFirst
	TimeRemainingSec int
	TotalDurationSec int  // full session duration; lets the agent reason about pacing
	IsFirst          bool // first call: MUST produce a warm intro question

	// Derived in the handler from history so the agent doesn't have to
	// re-compute them from the transcript.
	IntentsCovered             map[string]int // e.g. {"introduction":1,"behavioral":2,"technical":1}
	AvgScoreSoFar              *float64       // nil until at least one scored turn
	AvgTurnSec                 int            // running avg; defaults to 240 when empty
	ExpectedRemainingQuestions int            // floor(TimeRemainingSec / max(AvgTurnSec,60))
}

// NextQuestion is the one question produced by the live interviewer agent.
type NextQuestion struct {
	Title             string   `json:"title"`
	Body              string   `json:"body"`
	Answer            string   `json:"answer"` // reference answer for the reviewer agent to grade against
	Difficulty        string   `json:"difficulty"`
	Intent            string   `json:"intent"`
	Categories        []string `json:"categories"`
	ShouldWrap        bool     `json:"should_wrap"`         // agent signals "we're done, ask client to call /complete"
	IsFollowUp        bool     `json:"is_follow_up"`        // true when probing the prior answer; handler forces Intent="follow_up"
	FollowUpRationale string   `json:"follow_up_rationale"` // short, server-side only; never shown to candidate mid-interview
}

// LiveInterviewer produces the next question, given the running transcript.
type LiveInterviewer interface {
	NextQuestion(ctx context.Context, in NextQuestionInput) (*NextQuestion, error)
}

const liveInterviewerInstruction = `You are conducting a live mock interview as a senior, friendly-but-incisive interviewer. You produce ONE next question per call.

You will be shown:
- The candidate profile (target role, experience, tech stack, goals) and an optional resume excerpt.
- An OPTIONAL target job description the candidate is preparing for. When present, prefer questions that probe the specific skills, responsibilities, and stack mentioned in it — still grounded in the candidate's resume. Do NOT read the JD aloud or quote it verbatim; weave its requirements into your questions. When absent, behave as before.
- The total interview duration and the remaining time.
- A coverage map: how many questions you've asked of each intent.
- The candidate's running average answer score and a per-turn reviewer verdict (score, strengths, gaps).
- The running Q&A transcript so far.
- An estimate of how many more questions you're likely to fit in the time remaining.

On each turn, choose ONE of three moves. Be deliberate — explain your choice in follow_up_rationale (one short sentence, server-side only).

(A) FOLLOW-UP — set is_follow_up=true, intent="follow_up". Pick this when AT LEAST ONE is true:
    - The last answer's score is missing or < 60, AND time_remaining_sec > 120, AND the reviewer listed at least one concrete gap worth probing.
    - The last answer was strong overall but skipped a specific dimension the reviewer flagged (e.g., trade-offs, metrics, failure modes).
    Anchor the question to a specific phrase the candidate said or a listed gap — do NOT ask a generic deeper question.
    Do not follow up twice in a row on the same prior turn (check the transcript).

(B) PIVOT — set is_follow_up=false, intent ∈ {"behavioral","technical","system_design"}. Pick this when AT LEAST ONE is true:
    - The last answer scored >= 70 (solid — move on).
    - A required coverage axis is still empty (see Coverage targets below).
    - The same intent has been asked twice in a row.
    Reference SPECIFICS from the resume or earlier answers. Do NOT be generic.

(C) CLOSE — set should_wrap=true, intent="closing". Pick this when AT LEAST ONE is true:
    - time_remaining_sec < 90.
    - expected_remaining_questions <= 1 AND the coverage targets for this duration are met.
    - avg_score_so_far is below 40 AND time_remaining_sec < 240. End on dignity, not by piling on.
    Ask a brief reflective / "any questions for me?" closer.

Coverage targets by total interview duration:
- 15 min: introduction + 1 behavioral + 1 technical.
- 30 min: introduction + 1 behavioral + 2 technical (one of the technical may be system_design).
- 45 min: introduction + 1 behavioral + 2 technical + 1 system_design.

Difficulty rules: escalate over time, but stay matched to the candidate's seniority. If avg_score_so_far is low, do not crank difficulty higher — hold or slightly ease.

The "answer" field is the model answer the candidate should HEAR back. Write it as if the candidate themselves were giving a strong response — first person, plain prose, full sentences, 150–300 words. NO meta-language like "A strong answer should..." or "The candidate should mention...". NO bullet lists, headings, or code fences (this text is also read aloud by TTS). Anchor it with at least one concrete example, number, or named technology. End on the most important takeaway.

Output JSON only, exactly this shape:

{
  "title": "<the question, the way an interviewer would ask it>",
  "body": "<optional 1-line note: what to listen for>",
  "answer": "<the model answer a strong candidate would say, in first person, plain prose, 150–300 words>",
  "difficulty": "<easy|medium|hard>",
  "intent": "<introduction|behavioral|technical|system_design|follow_up|closing>",
  "categories": ["<optional slugs: frontend, backend, fullstack, system-architect, devops, javascript, react, typescript, css, node, go, databases, system-design, docker, kubernetes, ci-cd, security, behavioral>"],
  "should_wrap": false,
  "is_follow_up": false,
  "follow_up_rationale": "<one short sentence: why you chose follow-up vs pivot vs close>"
}`

// VertexLiveInterviewer is a Vertex AI / ADK-backed LiveInterviewer.
type VertexLiveInterviewer struct {
	runner *runner.Runner
}

func NewLiveInterviewer(ctx context.Context, backend Backend, modelName, project, location, apiKey string) (*VertexLiveInterviewer, error) {
	model, err := BuildGeminiModel(ctx, backend, modelName, project, location, apiKey)
	if err != nil {
		return nil, fmt.Errorf("live interviewer: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "live_interviewer",
		Description: "Generates the next question in a live mock interview from the running transcript.",
		Model:       model,
		Instruction: liveInterviewerInstruction,
		OutputSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"title":               {Type: genai.TypeString},
				"body":                {Type: genai.TypeString},
				"answer":              {Type: genai.TypeString},
				"difficulty":          {Type: genai.TypeString},
				"intent":              {Type: genai.TypeString},
				"categories":          {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"should_wrap":         {Type: genai.TypeBoolean},
				"is_follow_up":        {Type: genai.TypeBoolean},
				"follow_up_rationale": {Type: genai.TypeString},
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
	if p.JobDescription != "" {
		b.WriteString("\nTarget job description (candidate pasted this — tailor questions to the skills/responsibilities here, but don't quote it back):\n")
		b.WriteString(trunc(p.JobDescription, 4000))
		b.WriteString("\n")
	}

	if in.TotalDurationSec > 0 {
		fmt.Fprintf(&b, "\nTotal interview duration: %d seconds (~%d min)\n", in.TotalDurationSec, in.TotalDurationSec/60)
	}
	fmt.Fprintf(&b, "Time remaining: %d seconds\n", in.TimeRemainingSec)
	fmt.Fprintf(&b, "Is first question: %t\n", in.IsFirst)

	if len(in.IntentsCovered) > 0 {
		// Render intents in a stable order so prompt caching has a chance.
		order := []string{"introduction", "behavioral", "technical", "system_design", "follow_up", "closing"}
		seen := map[string]bool{}
		parts := make([]string, 0, len(in.IntentsCovered))
		for _, k := range order {
			if v, ok := in.IntentsCovered[k]; ok && v > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", k, v))
				seen[k] = true
			}
		}
		for k, v := range in.IntentsCovered {
			if !seen[k] && v > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", k, v))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "Coverage so far: %s\n", strings.Join(parts, ", "))
		}
	}
	if in.AvgScoreSoFar != nil {
		fmt.Fprintf(&b, "Avg answer score so far: %.0f (0-100 scale)\n", *in.AvgScoreSoFar)
	}
	if in.AvgTurnSec > 0 {
		fmt.Fprintf(&b, "Avg turn duration: %ds; expected remaining questions: %d\n", in.AvgTurnSec, in.ExpectedRemainingQuestions)
	}

	if len(in.History) > 0 {
		b.WriteString("\nRunning Q&A transcript:\n")
		for i, t := range in.History {
			fmt.Fprintf(&b, "[%d] Interviewer (%s): %s\n", i+1, t.QuestionIntent, t.QuestionTitle)
			ans := strings.TrimSpace(t.CandidateAnswer)
			if ans == "" {
				ans = "(no audible answer)"
			}
			fmt.Fprintf(&b, "    Candidate: %s\n", trunc(ans, 1500))
			if t.AnswerScore != nil || len(t.AnswerStrengths) > 0 || len(t.AnswerGaps) > 0 {
				var rb strings.Builder
				rb.WriteString("    Reviewer: ")
				if t.AnswerScore != nil {
					fmt.Fprintf(&rb, "score=%.0f", *t.AnswerScore)
				} else {
					rb.WriteString("score=n/a")
				}
				if len(t.AnswerStrengths) > 0 {
					fmt.Fprintf(&rb, ", strengths=%s", formatList(t.AnswerStrengths, 3))
				}
				if len(t.AnswerGaps) > 0 {
					fmt.Fprintf(&rb, ", gaps=%s", formatList(t.AnswerGaps, 3))
				}
				rb.WriteString("\n")
				b.WriteString(rb.String())
			}
		}
	}

	b.WriteString("\nProduce the next question now as JSON per your instructions. Pick exactly one of follow-up / pivot / close and explain in follow_up_rationale.")
	return b.String()
}

// formatList renders up to n items as a compact bracketed list, e.g. ["clear arc","no metric"].
func formatList(items []string, n int) string {
	if n <= 0 || len(items) == 0 {
		return "[]"
	}
	if len(items) > n {
		items = items[:n]
	}
	cleaned := make([]string, 0, len(items))
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		cleaned = append(cleaned, fmt.Sprintf("%q", trunc(s, 80)))
	}
	return "[" + strings.Join(cleaned, ",") + "]"
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
			Answer:     fmt.Sprintf("Sure — I've been building software for the last several years, most recently focused on backend systems and the kinds of problems that show up when traffic grows faster than the original design assumed. In my current role I lead a small team and own a service that handles a meaningful share of our request volume; the work I'm most proud of is a rewrite that cut our p99 latency by about half by replacing a synchronous fan-out with a queue-backed pipeline. Outside the day job I read a lot of post-mortems, because that's where you learn what actually breaks at scale rather than what looks elegant in a diagram. What I'm looking for next is a place where the engineering bar is high and there's real ownership — I'd rather go deep on one or two hard problems than touch ten shallow ones. %s is interesting to me specifically because the problem space lines up with what I want to get better at over the next couple of years.", role),
			Difficulty: "easy",
			Intent:     "introduction",
			Categories: []string{"behavioral"},
			ShouldWrap: false,
		}, nil
	}

	// Wrap when time is short, or when the candidate is bombing badly and
	// there's not much time left (better to close on dignity).
	avgLow := in.AvgScoreSoFar != nil && *in.AvgScoreSoFar < 40
	if in.TimeRemainingSec < 90 || (avgLow && in.TimeRemainingSec < 240) {
		return &NextQuestion{
			Title:             "We're near the end — do you have any questions for me, or anything you wish I had asked?",
			Body:              "Closing prompt. Listen for thoughtful questions and self-awareness.",
			Answer:            "Yeah, a couple of things actually. First, how is success measured for this role in the first six months — is it more about shipping a specific project, or about ramping into the broader system? Second, when I was answering the system-design question I didn't get into how I'd handle backpressure on the queue side, so if you have a moment I'd love to share that briefly, because I think it's the part of the answer I felt strongest about. And honestly, I'd want to ask the same thing I look for whenever I join a team: what's the team's relationship with on-call and incidents — because that tells me a lot about whether engineering quality is something the org actually invests in or just talks about. Thanks for the conversation, this was genuinely a good set of questions.",
			Difficulty:        "easy",
			Intent:            "closing",
			Categories:        []string{"behavioral"},
			ShouldWrap:        true,
			FollowUpRationale: "Time tight or avg score low; closing gently.",
		}, nil
	}

	// Follow-up path: if the last answer was weak and the reviewer flagged a
	// gap, probe it instead of pivoting.
	if n := len(in.History); n > 0 {
		last := in.History[n-1]
		weak := last.AnswerScore != nil && *last.AnswerScore < 60
		if weak && len(last.AnswerGaps) > 0 && in.TimeRemainingSec > 120 {
			gap := strings.TrimSpace(last.AnswerGaps[0])
			if gap != "" {
				diff := "easy"
				if last.QuestionIntent == "technical" || last.QuestionIntent == "system_design" {
					diff = "medium"
				}
				return &NextQuestion{
					Title:             fmt.Sprintf("Let's stay on that for a moment — earlier you mentioned %s. Can you go deeper on the trade-off you skipped?", trunc(last.QuestionTitle, 80)),
					Body:              fmt.Sprintf("Probe the gap the reviewer flagged: %s", trunc(gap, 120)),
					Answer:            fmt.Sprintf("Sure — to go deeper on that: the specific trade-off I should have named is around %s. The reason I didn't lean on it initially is that in the projects I was thinking of, the load profile let us cheat a little — we got away with a simpler approach for a long time. But once you scale past that point you really do have to pick a side, and in my experience the right move is to be explicit about what you're trading: usually latency for consistency, or development speed for operational simplicity. A concrete example would be the time we switched a write path from synchronous double-writes to an async outbox — that bought us throughput, but it meant we had to invest real work into reconciling eventual consistency for downstream consumers. So yeah, the framing I should have used is: which guarantee am I willing to weaken, and what do I owe the next engineer who has to operate this.", trunc(gap, 200)),
					Difficulty:        diff,
					Intent:            "follow_up",
					Categories:        []string{"behavioral"},
					ShouldWrap:        false,
					IsFollowUp:        true,
					FollowUpRationale: fmt.Sprintf("Prior answer scored low and reviewer flagged: %s", trunc(gap, 80)),
				}, nil
			}
		}
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
			Answer:     "The project I'm proudest of is one where we rewrote the ingestion pipeline for our analytics service. The old system was a single synchronous path that fanned out to four downstream consumers, and once traffic doubled it started timing out under spikes. The hardest decision was whether to bandage it with a thread-pool tuning pass — which I could have shipped in a week — or do the right thing and put a queue in front. I went with the queue, even though it meant a month of work and an at-least-once semantics conversation with every downstream team. The trade-off I made explicit was throughput-and-resilience versus exactly-once delivery — we picked throughput plus idempotency keys, because reconciliation was cheaper than back-pressuring the producers. After it shipped, p99 latency dropped about 60% and we stopped paging on traffic spikes. If I did it again I'd invest earlier in load tests; we caught one bug in production that a serious load test would have flagged in staging.",
			Difficulty: "medium",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
		},
		{
			Title:      fmt.Sprintf("Pick the hardest bug you've debugged in %s and walk me through how you found it.", tech),
			Body:       "Tests depth + debugging instinct.",
			Answer:     fmt.Sprintf("The worst one in %s was an intermittent data corruption that only surfaced under load. The symptom was that one in maybe ten thousand records would come back with mismatched fields — never the same combination twice. My first hypothesis was a serialization bug, but logs ruled that out because the on-the-wire payload was correct. What I did next was add structured tracing with a request ID that followed the record through every handler, and that's when I noticed the corruption was concentrated on requests that hit a specific worker. From there it took maybe two hours to reproduce locally and find a shared mutable map being written without a lock — concurrent writes from two goroutines were corrupting each other. The thing that surprised me was how long it took me to question my mental model of which structures were already safe; I had assumed the map was per-request, but it was actually a package-level singleton initialized at startup. After the fix I added a race-detector job to CI so that class of bug fails the build instead of leaking to production.", tech),
			Difficulty: "medium",
			Intent:     "technical",
			Categories: []string{strings.ToLower(strings.ReplaceAll(tech, " ", "-"))},
		},
		{
			Title:      "Design a system that handles 1 million write requests per minute with idempotent semantics.",
			Body:       "Standard system-design probe. Encourage thinking out loud.",
			Answer:     "Sure — at a million writes per minute, that's roughly seventeen thousand per second, so I'm thinking distributed from the start, not a single beefy node. The first thing I'd put in is an idempotency key carried by the client on every request — typically a UUID — and the API layer would check that key against a dedup store before touching the database. For the dedup store I'd start with Redis with a TTL on each key, because the latency budget at the edge is tight; if persistence matters I'd back it with a unique index on the primary database as a defense in depth. After the dedup check I'd push the write onto a partitioned queue — Kafka or equivalent — keyed by the customer or entity id so all writes for the same entity land on the same partition and stay in order. Workers consume per-partition and apply writes. The big trade-offs I'd call out: this gives me at-least-once delivery, so every consumer below the queue has to be idempotent too; and the dedup window is bounded by the TTL, so a malicious replay after the TTL expires would slip through unless I make the unique index the source of truth. For observability I'd publish queue lag and dedup-hit-rate metrics — those two numbers tell you everything about whether the system is healthy.",
			Difficulty: "hard",
			Intent:     "system_design",
			Categories: []string{"system-design"},
		},
		{
			Title:      "Tell me about a time you disagreed with a teammate. How did you resolve it?",
			Body:       "Listen for empathy + a concrete resolution.",
			Answer:     "Sure — a few months ago I disagreed with a teammate on whether to add a feature flag for a refactor we were rolling out. They argued the flag added complexity and a rollback path we'd never use; I thought without it we'd be one bad merge away from a Friday-night page. We talked it through and I realized their underlying concern was that flags get added and never removed — they were pushing back on a real anti-pattern, not on the rollout itself. So we agreed on the flag, but we also agreed on a removal date written into the ticket, and I owned the cleanup PR. It shipped, the flag came out two weeks later, and we ended up using the rollback exactly once when a downstream service started returning a slightly different schema — so the flag paid for itself. The thing I learned from that disagreement is that the strongest position is usually the one that takes the other person's objection seriously and bakes the answer to it into the plan, instead of arguing the objection away.",
			Difficulty: "easy",
			Intent:     "behavioral",
			Categories: []string{"behavioral"},
		},
	}
	idx := len(in.History) % len(pool)
	q := pool[idx]
	q.ShouldWrap = in.TimeRemainingSec < 120
	q.FollowUpRationale = "Stub pivoting through canned pool."
	return &q, nil
}
