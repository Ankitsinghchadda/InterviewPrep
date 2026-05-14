// Package stt provides speech-to-text transcription.
package stt

import (
	"context"
	"fmt"
	"strings"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
)

type Transcriber interface {
	// Transcribe converts spoken audio to text. mimeType is the source content type
	// (e.g., "audio/webm" or "audio/mp4"). Returns a best-effort transcript.
	Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)
}

// Google is a Google Cloud Speech-to-Text Transcriber.
type Google struct {
	client   *speech.Client
	language string
}

func NewGoogle(ctx context.Context, language string) (*Google, error) {
	if language == "" {
		language = "en-US"
	}
	c, err := speech.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("speech client: %w", err)
	}
	return &Google{client: c, language: language}, nil
}

func (g *Google) Close() error { return g.client.Close() }

func (g *Google) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	enc, rate := encodingForMime(mimeType)
	cfg := &speechpb.RecognitionConfig{
		Encoding:                   enc,
		SampleRateHertz:            rate,
		LanguageCode:               g.language,
		EnableAutomaticPunctuation: true,
		Model:                      "latest_long",
	}
	src := &speechpb.RecognitionAudio{
		AudioSource: &speechpb.RecognitionAudio_Content{Content: audio},
	}

	// Sync Recognize caps at 60s of audio. We let the recorder go up to 90s and
	// users can speak slower — so always go through LongRunningRecognize.
	// LRR accepts inline content up to ~10MB, plenty for sub-2-minute opus
	// (~0.5MB) so no GCS upload is needed. We block on op.Wait().
	op, err := g.client.LongRunningRecognize(ctx, &speechpb.LongRunningRecognizeRequest{
		Config: cfg,
		Audio:  src,
	})
	if err != nil {
		return "", err
	}
	resp, err := op.Wait(ctx)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, r := range resp.Results {
		if len(r.Alternatives) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(r.Alternatives[0].Transcript)
	}
	return strings.TrimSpace(b.String()), nil
}

func encodingForMime(mime string) (speechpb.RecognitionConfig_AudioEncoding, int32) {
	switch {
	case strings.HasPrefix(mime, "audio/webm"):
		// Browser MediaRecorder default; opus at 48kHz.
		return speechpb.RecognitionConfig_WEBM_OPUS, 48000
	case strings.HasPrefix(mime, "audio/ogg"):
		return speechpb.RecognitionConfig_OGG_OPUS, 48000
	case strings.HasPrefix(mime, "audio/mp4"), strings.HasPrefix(mime, "audio/m4a"):
		// MP4 isn't natively supported by RecognitionConfig; ENCODING_UNSPECIFIED
		// lets Google detect from a few common containers, sample rate auto.
		return speechpb.RecognitionConfig_ENCODING_UNSPECIFIED, 0
	case strings.HasPrefix(mime, "audio/wav"), strings.HasPrefix(mime, "audio/x-wav"):
		return speechpb.RecognitionConfig_LINEAR16, 16000
	default:
		return speechpb.RecognitionConfig_ENCODING_UNSPECIFIED, 0
	}
}

// Stub returns a canned transcript. Used when GCP isn't configured so the rest
// of the pipeline still works end-to-end in local dev.
type Stub struct{}

func (Stub) Transcribe(_ context.Context, _ []byte, _ string) (string, error) {
	return "[stub transcript — set AGENT_ENABLED=true and configure GCP to enable real STT]", nil
}
