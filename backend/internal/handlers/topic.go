package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/embeddings"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

type CategoryHandler struct {
	Repo              *repository.CategoryRepo
	Questions         *repository.QuestionRepo
	QuestionGenerator agent.QuestionGenerator // nil disables auto-seed on Create
	Embedder          embeddings.Embedder
	Synth             tts.Synthesizer
}

// seedCount is the number of AI questions auto-generated for each new admin-
// created topic. Matches the empty-state "Generate 5 questions with AI" knob
// on the Questions page so both flows feel like the same product gesture.
const seedCount = 5

// seedTimeout caps the background goroutine. Generous so a slow Vertex call
// plus per-question embedding + TTS still fits.
const seedTimeout = 90 * time.Second

// slugRe matches kebab-case slugs: lowercase letters, digits, hyphens.
var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type createCategoryReq struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Kind        string `json:"kind"` // "role" | "topic"
	Description string `json:"description"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sortOrder"`
}

// Create inserts a new category (admin-gated upstream by the route).
// Defaults: kind="topic", sortOrder=100. Slug is normalised to lowercase.
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createCategoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		response.Err(w, http.StatusBadRequest, "name is required")
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		response.Err(w, http.StatusBadRequest, "slug is required")
		return
	}
	if !slugRe.MatchString(slug) {
		response.Err(w, http.StatusBadRequest, "slug must be lowercase letters, digits, and hyphens (kebab-case)")
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "topic"
	}
	if kind != "role" && kind != "topic" {
		response.Err(w, http.StatusBadRequest, "kind must be 'role' or 'topic'")
		return
	}
	sortOrder := req.SortOrder
	if sortOrder == 0 {
		sortOrder = 100
	}

	cat, err := h.Repo.Create(r.Context(), models.Category{
		Slug:        slug,
		Name:        name,
		Kind:        kind,
		Description: strings.TrimSpace(req.Description),
		Icon:        strings.TrimSpace(req.Icon),
		SortOrder:   sortOrder,
	})
	if errors.Is(err, repository.ErrDuplicate) {
		response.Err(w, http.StatusConflict, "slug already exists")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create category")
		return
	}
	response.OK(w, http.StatusCreated, cat)

	// Fire-and-forget: seed the new topic with starter AI questions so the
	// admin can navigate to /questions?categories=<slug> and find content
	// instead of an empty state. Runs on a fresh context (the request one
	// gets cancelled when the response goroutine returns).
	if h.QuestionGenerator != nil && h.Questions != nil {
		go func(slug string) {
			ctx, cancel := context.WithTimeout(context.Background(), seedTimeout)
			defer cancel()
			if _, err := seedAIQuestionsForCategories(
				ctx, h.Questions, h.QuestionGenerator, h.Embedder, h.Synth,
				[]string{slug}, seedCount, "",
			); err != nil {
				log.Printf("seed questions for %q: %v", slug, err)
			}
		}(cat.Slug)
	}
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
