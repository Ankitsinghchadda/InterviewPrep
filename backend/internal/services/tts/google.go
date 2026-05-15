package tts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	ttspb "cloud.google.com/go/texttospeech/apiv1/texttospeechpb"

	"cloud.google.com/go/storage"
)

// ttsByteLimit is a few hundred bytes shy of Google TTS's 5000-byte input cap
// so we have headroom for any whitespace normalization the API does internally.
const ttsByteLimit = 4800

// Google is a Synthesizer backed by Cloud Text-to-Speech + GCS. The TTS
// produces MP3 bytes which are then written to the configured public bucket;
// the public object URL is returned to the caller.
type Google struct {
	tts     *texttospeech.Client
	storage *storage.Client
	bucket  string
	voice   string // e.g. en-US-Neural2-D
}

// NewGoogle constructs the TTS service. Both clients use Application Default
// Credentials, matching the rest of the GCP-backed services in the app.
func NewGoogle(ctx context.Context, bucket, voice string) (*Google, error) {
	if bucket == "" {
		return nil, errors.New("tts: AUDIO_BUCKET is required")
	}
	if voice == "" {
		voice = "en-US-Neural2-D"
	}
	tc, err := texttospeech.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("tts: build text-to-speech client: %w", err)
	}
	sc, err := storage.NewClient(ctx)
	if err != nil {
		_ = tc.Close()
		return nil, fmt.Errorf("tts: build storage client: %w", err)
	}
	return &Google{tts: tc, storage: sc, bucket: bucket, voice: voice}, nil
}

// Close releases the underlying GCP clients.
func (g *Google) Close() {
	if g == nil {
		return
	}
	if g.tts != nil {
		_ = g.tts.Close()
	}
	if g.storage != nil {
		_ = g.storage.Close()
	}
}

func (g *Google) Synthesize(ctx context.Context, questionID, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("tts: empty text")
	}
	if questionID == "" {
		return "", errors.New("tts: empty question id")
	}
	text = truncateForTTS(text, ttsByteLimit)

	// Voice locale is derived from the voice name's "en-US-..." prefix so
	// callers only need to set TTS_VOICE.
	langCode := languageFromVoice(g.voice)
	req := &ttspb.SynthesizeSpeechRequest{
		Input: &ttspb.SynthesisInput{
			InputSource: &ttspb.SynthesisInput_Text{Text: text},
		},
		Voice: &ttspb.VoiceSelectionParams{
			LanguageCode: langCode,
			Name:         g.voice,
		},
		AudioConfig: &ttspb.AudioConfig{
			AudioEncoding: ttspb.AudioEncoding_MP3,
		},
	}
	resp, err := g.tts.SynthesizeSpeech(ctx, req)
	if err != nil {
		return "", fmt.Errorf("tts: synthesize: %w", err)
	}
	if len(resp.AudioContent) == 0 {
		return "", errors.New("tts: empty audio response")
	}

	key := "questions/" + questionID + ".mp3"
	obj := g.storage.Bucket(g.bucket).Object(key)
	w := obj.NewWriter(ctx)
	w.ContentType = "audio/mpeg"
	w.CacheControl = "public, max-age=31536000, immutable"
	if _, err := w.Write(resp.AudioContent); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("tts: upload write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("tts: upload close: %w", err)
	}

	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", g.bucket, key), nil
}

// truncateForTTS shortens `text` to at most `limit` bytes for the Google TTS
// API's 5000-byte input cap. Prefers the last sentence terminator (. ! ?)
// before the limit so the cut sounds natural; otherwise falls back to the
// last word boundary, and finally to a rune-safe hard cut. An ellipsis is
// appended so the listener can tell the reference was abridged.
func truncateForTTS(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	// Reserve room for the ellipsis suffix.
	const suffix = "…"
	budget := limit - len(suffix)
	if budget <= 0 {
		return suffix
	}

	cut := text[:budget]
	// Sentence boundary preferred.
	if i := strings.LastIndexAny(cut, ".!?"); i > budget/2 {
		return text[:i+1] + " " + suffix
	}
	// Word boundary next.
	if i := strings.LastIndexAny(cut, " \n\t"); i > budget/2 {
		return strings.TrimRight(text[:i], " \n\t") + suffix
	}
	// Hard cut at a UTF-8 rune boundary.
	for budget > 0 && !utf8.RuneStart(text[budget]) {
		budget--
	}
	return text[:budget] + suffix
}

// languageFromVoice extracts "en-US" from a name like "en-US-Neural2-D". The
// TTS API ignores LanguageCode when the voice Name is set, but the field is
// required, so we derive it rather than asking the caller to pass it.
func languageFromVoice(voice string) string {
	parts := strings.SplitN(voice, "-", 3)
	if len(parts) < 2 {
		return "en-US"
	}
	return parts[0] + "-" + parts[1]
}
