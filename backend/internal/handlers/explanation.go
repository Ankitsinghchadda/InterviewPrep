package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

// ExplanationHandler serves the learner-facing explanation for a question.
// First call invokes the explainer agent and persists the result; subsequent
// calls return the cached row instantly.
type ExplanationHandler struct {
	Questions *repository.QuestionRepo
	Explainer agent.Explainer
	Billing   *billing.Service
}

type explanationResponse struct {
	Summary  string `json:"summary"`
	Markdown string `json:"markdown"`
}

// explainLocks serializes lazy generation per question so two concurrent
// requests don't both call the LLM for the same row.
var explainLocks sync.Map // map[questionID]*sync.Mutex

func lockForExplanation(id string) *sync.Mutex {
	if v, ok := explainLocks.Load(id); ok {
		return v.(*sync.Mutex)
	}
	mu := &sync.Mutex{}
	actual, _ := explainLocks.LoadOrStore(id, mu)
	return actual.(*sync.Mutex)
}

// Generate is POST /api/v1/questions/{id}/explanation — synchronous on first
// call, idempotent on subsequent calls.
func (h *ExplanationHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserID(r.Context()); !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.Explainer == nil {
		response.Err(w, http.StatusServiceUnavailable, "explanation service is not configured")
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
	// Private question — only the owner may generate.
	if q.OwnerID != nil {
		userID, _ := auth.UserID(r.Context())
		if *q.OwnerID != userID {
			response.Err(w, http.StatusNotFound, "question not found")
			return
		}
	}

	if q.ExplanationMarkdown != "" {
		response.OK(w, http.StatusOK, explanationResponse{
			Summary:  q.ExplanationSummary,
			Markdown: q.ExplanationMarkdown,
		})
		return
	}

	mu := lockForExplanation(id)
	mu.Lock()
	defer mu.Unlock()

	// Re-check inside the lock to pick up a concurrent caller's write.
	q, err = h.Questions.Get(r.Context(), id)
	if err == nil && q.ExplanationMarkdown != "" {
		response.OK(w, http.StatusOK, explanationResponse{
			Summary:  q.ExplanationSummary,
			Markdown: q.ExplanationMarkdown,
		})
		return
	}

	if !checkQuota(w, r, h.Billing, billing.KindExplanation) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	out, err := h.Explainer.Explain(ctx, agent.ExplainInput{
		QuestionTitle:   q.Title,
		QuestionBody:    q.Body,
		ReferenceAnswer: q.Answer,
		Categories:      q.Categories,
		Difficulty:      q.Difficulty,
	})
	if err != nil {
		response.Err(w, http.StatusBadGateway, "explanation failed: "+err.Error())
		return
	}
	if err := h.Questions.UpdateExplanation(r.Context(), id, out.Summary, out.Markdown); err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to persist explanation")
		return
	}
	safeRecord(r.Context(), h.Billing, billing.KindExplanation, map[string]any{"question_id": id})
	response.OK(w, http.StatusOK, explanationResponse{
		Summary:  out.Summary,
		Markdown: out.Markdown,
	})
}
