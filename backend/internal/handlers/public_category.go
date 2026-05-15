package handlers

import (
	"errors"
	"net/http"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

// PublicCategoryHandler serves the no-auth topic-landing surface used by SEO.
// /topics/:slug is the highest-value SEO surface — these pages target queries
// like "javascript interview questions" which have far more search volume
// than any individual question.
type PublicCategoryHandler struct {
	Categories *repository.CategoryRepo
	Questions  *repository.QuestionRepo
}

// PublicCategoryDetail is the response shape: category metadata + every
// public question in it. Frontend renders this as a single page so Google
// indexes the category name, description, AND every question title under it.
type PublicCategoryDetail struct {
	Category  models.Category   `json:"category"`
	Questions []models.Question `json:"questions"`
}

// List returns every category. No auth required.
func (h *PublicCategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	cats, err := h.Categories.List(r.Context(), "")
	if err != nil {
		response.Err(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	response.OK(w, http.StatusOK, cats)
}

// Get returns one category + every public question linked to it.
func (h *PublicCategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		response.Err(w, http.StatusBadRequest, "missing slug")
		return
	}

	cat, err := h.Categories.GetBySlug(r.Context(), slug)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "category not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, err.Error())
		return
	}

	qs, err := h.Questions.ListPublicByCategorySlug(r.Context(), slug, 200)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=300")
	response.OK(w, http.StatusOK, PublicCategoryDetail{Category: *cat, Questions: qs})
}
