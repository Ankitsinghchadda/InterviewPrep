package agent

import (
	"context"
	"strings"
	"time"
)

// Stub is a deterministic Reviewer used when AGENT_ENABLED=false. It mimics the
// shape of real output so the end-to-end UI flow works without GCP credentials.
type Stub struct{}

func (s Stub) Review(ctx context.Context, in ReviewInput) (*ReviewResult, error) {
	return s.ReviewStream(ctx, in, nil)
}

// ReviewStream emits the canned feedback in small chunks so the SSE pipeline
// can be exercised end-to-end without real Vertex credentials.
func (Stub) ReviewStream(ctx context.Context, in ReviewInput, onToken func(string)) (*ReviewResult, error) {
	words := len(strings.Fields(in.CandidateAnswer))
	score := 55.0
	switch {
	case words > 120:
		score = 78
	case words > 60:
		score = 68
	case words < 15:
		score = 32
	}

	result := &ReviewResult{
		Score: score,
		Strengths: []string{
			"You started speaking right away and stayed on topic.",
			"Mentioned at least one specific concept from the question.",
		},
		Improvements: []string{
			"Tie the answer back to a concrete, real-world example.",
			"Be explicit about the trade-offs an interviewer wants to hear.",
		},
		Feedback: "This is stub feedback for local development. Configure GOOGLE_CLOUD_PROJECT and set AGENT_ENABLED=true to get a real review from a Vertex AI agent.",
	}

	if onToken != nil {
		// Stream the feedback prose word-by-word so the UI can render the
		// typing effect. We don't bother streaming the score/strengths/
		// improvements — those land in the final structured result.
		for i, word := range strings.Fields(result.Feedback) {
			if i > 0 {
				word = " " + word
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(60 * time.Millisecond):
			}
			onToken(word)
		}
	} else {
		// No streaming — just simulate the agent latency.
		select {
		case <-time.After(1500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return result, nil
}
