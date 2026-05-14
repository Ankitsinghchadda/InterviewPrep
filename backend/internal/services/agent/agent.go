// Package agent wraps the Google ADK reviewer agent used to score and
// critique a candidate's spoken answer to an interview question.
//
// The Reviewer interface lets us run a real Vertex AI agent in production
// and a deterministic stub in environments without GCP credentials.
package agent

import "context"

// ReviewInput is everything the reviewer needs to grade one answer.
type ReviewInput struct {
	QuestionTitle    string
	QuestionBody     string
	ReferenceAnswer  string
	CandidateAnswer  string // transcript
	Categories       []string
	Difficulty       string // easy | medium | hard
}

// ReviewResult is the structured output we persist to answer_submissions.
type ReviewResult struct {
	Score        float64  `json:"score"`        // 0..100
	Strengths    []string `json:"strengths"`    // 2-4 items
	Improvements []string `json:"improvements"` // 2-4 items
	Feedback     string   `json:"feedback"`     // 2-3 sentence prose
}

// Reviewer scores a candidate's answer against a reference.
type Reviewer interface {
	// Review runs the agent and returns the structured result once complete.
	Review(ctx context.Context, in ReviewInput) (*ReviewResult, error)
	// ReviewStream runs the same agent but invokes onToken with partial text
	// chunks as the model emits them. The final ReviewResult (parsed from the
	// full output) is still returned at the end. onToken is best-effort —
	// implementations may call it many times, once, or never.
	ReviewStream(ctx context.Context, in ReviewInput, onToken func(string)) (*ReviewResult, error)
}
