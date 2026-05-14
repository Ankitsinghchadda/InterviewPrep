package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
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
	response.OK(w, http.StatusOK, synthesizeAudioResponse{AudioURL: url})
}
