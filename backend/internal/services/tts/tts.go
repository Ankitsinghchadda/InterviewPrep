// Package tts synthesizes reference-answer audio for interview questions and
// publishes it to a public GCS bucket. The returned URL can be used directly
// as <audio src=...> on the front-end.
package tts

import "context"

// Synthesizer turns text into an audio file and returns its public URL.
// Implementations should be safe for concurrent use. Two flavours are exposed:
// the reference ANSWER (often male voice, longer) and the question PROMPT —
// the interviewer reading the question aloud, typically a different voice so
// the candidate hears two distinct speakers.
type Synthesizer interface {
	// Synthesize generates audio for `text` and stores it under a stable key
	// derived from `questionID`. Returns the public URL of the stored audio.
	Synthesize(ctx context.Context, questionID, text string) (publicURL string, err error)

	// SynthesizePrompt generates the interviewer-asking-the-question audio
	// using the prompt voice (typically female). Stored under a separate key
	// so it never collides with the reference-answer audio.
	SynthesizePrompt(ctx context.Context, questionID, text string) (publicURL string, err error)
}

// Stub is the no-op implementation used when GCP isn't configured. It always
// returns an empty URL with no error so callers can treat audio as optional.
type Stub struct{}

func (Stub) Synthesize(_ context.Context, _, _ string) (string, error)       { return "", nil }
func (Stub) SynthesizePrompt(_ context.Context, _, _ string) (string, error) { return "", nil }
