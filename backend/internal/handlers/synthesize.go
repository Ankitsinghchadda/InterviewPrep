package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

// synthesizeAndStore generates reference-answer audio in the background and
// persists the public URL on the question row. Best-effort: failures are
// logged but never propagated, since audio is a nice-to-have and the question
// itself is already saved by the time this runs.
func synthesizeAndStore(synth tts.Synthesizer, questions *repository.QuestionRepo, q *models.Question) {
	if synth == nil || q == nil || q.Answer == "" {
		return
	}
	go func(id, answer string) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		url, err := synth.Synthesize(ctx, id, answer)
		if err != nil {
			log.Printf("tts: synthesize %s: %v", id, err)
			return
		}
		if url == "" {
			return
		}
		if err := questions.UpdateAudioURL(ctx, id, url); err != nil {
			log.Printf("tts: persist audio url for %s: %v", id, err)
		}
	}(q.ID, q.Answer)
}

// synthesizePromptAndStore generates the interviewer-asking-the-question audio
// (the question Title, read aloud in the prompt voice) in the background and
// persists the public URL on the question row. Best-effort, same shape as
// synthesizeAndStore — failures are logged but never propagated. Used by the
// live interview flow so the candidate can hear the question while reading it.
func synthesizePromptAndStore(synth tts.Synthesizer, questions *repository.QuestionRepo, q *models.Question) {
	if synth == nil || q == nil || q.Title == "" {
		return
	}
	go func(id, title string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		url, err := synth.SynthesizePrompt(ctx, id, title)
		if err != nil {
			log.Printf("tts: synthesize prompt %s: %v", id, err)
			return
		}
		if url == "" {
			return
		}
		if err := questions.UpdatePromptAudioURL(ctx, id, url); err != nil {
			log.Printf("tts: persist prompt audio url for %s: %v", id, err)
		}
	}(q.ID, q.Title)
}

// audioGenLocks serializes lazy generation per question to avoid two concurrent
// requests both calling the TTS API for the same row.
var audioGenLocks sync.Map // map[questionID]*sync.Mutex

func lockForQuestion(id string) *sync.Mutex {
	if v, ok := audioGenLocks.Load(id); ok {
		return v.(*sync.Mutex)
	}
	mu := &sync.Mutex{}
	actual, _ := audioGenLocks.LoadOrStore(id, mu)
	return actual.(*sync.Mutex)
}

// AudioHandler handles lazy synthesis for questions that don't already have an
// answer_audio_url. Used to backfill curated/old questions on demand.
type AudioHandler struct {
	Questions *repository.QuestionRepo
	Synth     tts.Synthesizer
	Billing   *billing.Service
}

type synthesizeAudioResponse struct {
	AudioURL string `json:"audioUrl"`
}

// Generate is POST /api/v1/questions/{id}/audio — synchronous on first call,
// idempotent on subsequent calls. Returns the public URL.
func (h *AudioHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserID(r.Context()); !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.Synth == nil {
		response.Err(w, http.StatusServiceUnavailable, "audio synthesis is not configured")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Err(w, http.StatusBadRequest, "missing question id")
		return
	}

	q, err := h.Questions.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load question")
		return
	}
	if q.AnswerAudioURL != "" {
		response.OK(w, http.StatusOK, synthesizeAudioResponse{AudioURL: q.AnswerAudioURL})
		return
	}

	mu := lockForQuestion(id)
	mu.Lock()
	defer mu.Unlock()

	// Re-check inside the lock so a concurrent caller's write is picked up.
	q, err = h.Questions.Get(r.Context(), id)
	if err == nil && q.AnswerAudioURL != "" {
		response.OK(w, http.StatusOK, synthesizeAudioResponse{AudioURL: q.AnswerAudioURL})
		return
	}

	// Charge quota only when we're actually going to call the TTS API.
	// Cache hits above stay free.
	if !checkQuota(w, r, h.Billing, billing.KindTTS) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	url, err := h.Synth.Synthesize(ctx, id, q.Answer)
	if err != nil {
		response.Err(w, http.StatusBadGateway, "tts failed: "+err.Error())
		return
	}
	if url == "" {
		response.Err(w, http.StatusBadGateway, "tts returned empty url")
		return
	}
	if err := h.Questions.UpdateAudioURL(r.Context(), id, url); err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to persist audio url")
		return
	}
	safeRecord(r.Context(), h.Billing, billing.KindTTS, map[string]any{"question_id": id})
	response.OK(w, http.StatusOK, synthesizeAudioResponse{AudioURL: url})
}

// GeneratePrompt is POST /api/v1/questions/{id}/prompt-audio — synchronous on
// first call, idempotent on subsequent calls. Returns the public URL of the
// interviewer voice reading the question aloud. Used as a fallback by the live
// interview UI when the background synthesis hasn't completed yet, and for
// older live questions that pre-date the auto-generate path.
func (h *AudioHandler) GeneratePrompt(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserID(r.Context()); !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.Synth == nil {
		response.Err(w, http.StatusServiceUnavailable, "audio synthesis is not configured")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Err(w, http.StatusBadRequest, "missing question id")
		return
	}

	q, err := h.Questions.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load question")
		return
	}
	if q.PromptAudioURL != "" {
		response.OK(w, http.StatusOK, synthesizeAudioResponse{AudioURL: q.PromptAudioURL})
		return
	}

	// Use a distinct lock namespace per kind so a concurrent answer-audio
	// request doesn't block prompt synthesis for the same question.
	mu := lockForQuestion("prompt:" + id)
	mu.Lock()
	defer mu.Unlock()

	q, err = h.Questions.Get(r.Context(), id)
	if err == nil && q.PromptAudioURL != "" {
		response.OK(w, http.StatusOK, synthesizeAudioResponse{AudioURL: q.PromptAudioURL})
		return
	}

	if !checkQuota(w, r, h.Billing, billing.KindTTS) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	url, err := h.Synth.SynthesizePrompt(ctx, id, q.Title)
	if err != nil {
		response.Err(w, http.StatusBadGateway, "tts failed: "+err.Error())
		return
	}
	if url == "" {
		response.Err(w, http.StatusBadGateway, "tts returned empty url")
		return
	}
	if err := h.Questions.UpdatePromptAudioURL(r.Context(), id, url); err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to persist audio url")
		return
	}
	safeRecord(r.Context(), h.Billing, billing.KindTTS, map[string]any{"question_id": id, "kind": "prompt"})
	response.OK(w, http.StatusOK, synthesizeAudioResponse{AudioURL: url})
}
