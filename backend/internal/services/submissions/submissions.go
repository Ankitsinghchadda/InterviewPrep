// Package submissions runs the audio → STT → AI review pipeline.
// It's shared between the per-question Practice endpoint and the in-interview
// answer endpoint so the async behavior, status transitions, and error handling
// stay identical across both flows.
package submissions

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/audio"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/stt"
)

type Service struct {
	Submissions *repository.SubmissionRepo
	Storage     audio.Storage
	Transcriber stt.Transcriber
	Reviewer    agent.Reviewer
	// Broker fans out transcript + review-token + completion events to SSE
	// subscribers. Nil-safe: a missing broker just disables streaming.
	Broker *Broker
}

type SubmitInput struct {
	UserID      string
	Question    *models.Question
	InterviewID *string // optional — set for in-interview answers
	Audio       io.Reader
	MimeType    string
	// Transcript is an optional client-supplied transcript (e.g., from the
	// browser's Web Speech API). When present we skip server-side STT and
	// proceed directly to the AI review.
	Transcript string
}

// Submit persists the audio, creates a pending submission row, and kicks off
// the async STT + review goroutine. Returns the submission immediately so the
// caller can respond with the submission id for polling.
func (s *Service) Submit(ctx context.Context, in SubmitInput) (*models.Submission, error) {
	sub, err := s.Submissions.Create(ctx, repository.CreateSubmissionInput{
		UserID:      in.UserID,
		QuestionID:  in.Question.ID,
		InterviewID: in.InterviewID,
	})
	if err != nil {
		return nil, err
	}

	key, err := s.Storage.Save(ctx, in.UserID, sub.ID, extFromMime(in.MimeType), in.Audio)
	if err != nil {
		_ = s.Submissions.UpdateStatus(ctx, sub.ID, "failed", "audio storage error")
		return nil, err
	}
	sub.AudioURL = key

	go s.process(sub.ID, key, in.MimeType, in.Question, in.Transcript)
	return sub, nil
}

// process runs STT (or accepts a client-supplied transcript) then the reviewer
// agent, streaming intermediate events through the broker. Runs with a fresh
// context — the HTTP request context is dead by the time the goroutine starts.
func (s *Service) process(submissionID, audioKey, mimeType string, question *models.Question, clientTranscript string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	publish := func(kind, data string) {
		if s.Broker != nil {
			s.Broker.Publish(submissionID, Event{Kind: kind, Data: data})
		}
	}
	defer func() {
		if s.Broker != nil {
			s.Broker.Close(submissionID)
		}
	}()

	var transcript string

	if clientTranscript != "" {
		// Client-supplied (e.g., browser Web Speech) — skip STT entirely.
		transcript = clientTranscript
	} else {
		if err := s.Submissions.UpdateStatus(ctx, submissionID, "transcribing", ""); err != nil {
			log.Printf("submission %s: update status failed: %v", submissionID, err)
		}

		rc, err := s.Storage.Read(ctx, audioKey)
		if err != nil {
			log.Printf("submission %s: read audio failed: %v", submissionID, err)
			_ = s.Submissions.UpdateStatus(ctx, submissionID, "failed", "could not read stored audio")
			publish("error", "could not read stored audio")
			return
		}
		audioBytes, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			_ = s.Submissions.UpdateStatus(ctx, submissionID, "failed", "could not load audio bytes")
			publish("error", "could not load audio bytes")
			return
		}

		t, err := s.Transcriber.Transcribe(ctx, audioBytes, mimeType)
		if err != nil {
			log.Printf("submission %s: STT failed: %v", submissionID, err)
			_ = s.Submissions.UpdateStatus(ctx, submissionID, "failed", "transcription failed: "+err.Error())
			publish("error", "transcription failed")
			return
		}
		transcript = t
	}

	if err := s.Submissions.SetTranscript(ctx, submissionID, transcript); err != nil {
		log.Printf("submission %s: persist transcript failed: %v", submissionID, err)
	}
	publish("transcript", transcript)

	onToken := func(delta string) {
		publish("review_token", delta)
	}
	review, err := s.Reviewer.ReviewStream(ctx, agent.ReviewInput{
		QuestionTitle:   question.Title,
		QuestionBody:    question.Body,
		ReferenceAnswer: question.Answer,
		CandidateAnswer: transcript,
		Categories:      question.Categories,
		Difficulty:      question.Difficulty,
	}, onToken)
	if err != nil {
		log.Printf("submission %s: review failed: %v", submissionID, err)
		_ = s.Submissions.UpdateStatus(ctx, submissionID, "failed", "agent review failed: "+err.Error())
		publish("error", "review failed")
		return
	}

	if err := s.Submissions.SetReview(ctx, submissionID, repository.ReviewResult{
		Score:        review.Score,
		Feedback:     review.Feedback,
		Strengths:    review.Strengths,
		Improvements: review.Improvements,
	}); err != nil {
		log.Printf("submission %s: persist review failed: %v", submissionID, err)
	}

	if payload, err := json.Marshal(review); err == nil {
		publish("review_done", string(payload))
	}
}

func extFromMime(mime string) string {
	switch {
	case len(mime) >= 10 && mime[:10] == "audio/webm":
		return "webm"
	case len(mime) >= 9 && mime[:9] == "audio/ogg":
		return "ogg"
	case len(mime) >= 9 && mime[:9] == "audio/mp4":
		return "m4a"
	case len(mime) >= 9 && mime[:9] == "audio/m4a":
		return "m4a"
	case len(mime) >= 9 && mime[:9] == "audio/wav":
		return "wav"
	case len(mime) >= 10 && mime[:10] == "audio/mpeg":
		return "mp3"
	default:
		return "bin"
	}
}
