package handlers

import (
	"errors"
	"net/http"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

type CategoryHandler struct {
	Repo *repository.CategoryRepo
}

// List returns categories. ?kind=role|topic filters by kind.
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "role" && kind != "topic" {
		response.Err(w, http.StatusBadRequest, "kind must be 'role' or 'topic'")
		return
	}
	cats, err := h.Repo.List(r.Context(), kind)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	response.OK(w, http.StatusOK, cats)
}

// GetBySlug returns one category by slug.
func (h *CategoryHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, err := h.Repo.GetBySlug(r.Context(), slug)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "category not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load category")
		return
	}
	response.OK(w, http.StatusOK, cat)
}
