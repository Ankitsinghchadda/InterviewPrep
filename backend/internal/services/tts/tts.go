// Package tts synthesizes reference-answer audio for interview questions and
// publishes it to a public GCS bucket. The returned URL can be used directly
// as <audio src=...> on the front-end.
package tts

import "context"

// Synthesizer turns answer text into an audio file and returns its public URL.
// Implementations should be safe for concurrent use.
type Synthesizer interface {
	// Synthesize generates audio for `text` and stores it under a stable key
	// derived from `questionID`. Returns the public URL of the stored audio.
	Synthesize(ctx context.Context, questionID, text string) (publicURL string, err error)
}

// Stub is the no-op implementation used when GCP isn't configured. It always
// returns an empty URL with no error so callers can treat audio as optional.
type Stub struct{}

func (Stub) Synthesize(_ context.Context, _, _ string) (string, error) { return "", nil }
