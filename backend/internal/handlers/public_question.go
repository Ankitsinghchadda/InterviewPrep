package handlers

import (
	"errors"
	"net/http"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

// PublicQuestionHandler serves the no-auth read path for public questions.
// Used by SEO crawlers and unauthenticated visitors clicking a sitemap link.
// The frontend hides the answer/practice surfaces until login, but the API
// itself returns the full row — the gating is purely a UX layer and the
// content is intentionally crawlable so Google can index it.
type PublicQuestionHandler struct {
	Repo *repository.QuestionRepo
}

func (h *PublicQuestionHandler) Get(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	if idOrSlug == "" {
		response.Err(w, http.StatusBadRequest, "missing id or slug")
		return
	}

	q, err := h.Repo.GetPublicByIDOrSlug(r.Context(), idOrSlug)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 5-minute browser/CDN cache. Questions change rarely; a short TTL is the
	// difference between fresh edits showing up in 5 min vs. immediately, and
	// it dramatically reduces DB load if a question goes viral.
	w.Header().Set("Cache-Control", "public, max-age=300")
	response.OK(w, http.StatusOK, q)
}
