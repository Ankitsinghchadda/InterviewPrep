package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

type CollectionHandler struct {
	Repo *repository.CollectionRepo
}

// List returns the caller's collections. The default Saved row is lazily
// created here so the user always sees at least one entry.
func (h *CollectionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := h.Repo.EnsureDefault(r.Context(), userID); err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load collections")
		return
	}
	out, err := h.Repo.List(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load collections")
		return
	}
	response.OK(w, http.StatusOK, out)
}

func (h *CollectionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	c, err := h.Repo.Get(r.Context(), id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "collection not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load collection")
		return
	}
	response.OK(w, http.StatusOK, c)
}

type createCollectionBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func (h *CollectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body createCollectionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		response.Err(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Name) > 80 {
		response.Err(w, http.StatusBadRequest, "name is too long")
		return
	}
	c, err := h.Repo.Create(r.Context(), repository.CreateCollectionInput{
		UserID:      userID,
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
	})
	if errors.Is(err, repository.ErrDuplicate) {
		response.Err(w, http.StatusConflict, "a collection with that name already exists")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create collection")
		return
	}
	response.OK(w, http.StatusCreated, c)
}

type updateCollectionBody struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
}

func (h *CollectionHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	var body updateCollectionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			response.Err(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		if len(trimmed) > 80 {
			response.Err(w, http.StatusBadRequest, "name is too long")
			return
		}
		body.Name = &trimmed
	}
	c, err := h.Repo.Update(r.Context(), id, userID, repository.UpdateCollectionInput{
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
	})
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "collection not found")
		return
	}
	if errors.Is(err, repository.ErrDuplicate) {
		response.Err(w, http.StatusConflict, "a collection with that name already exists")
		return
	}
	if errors.Is(err, repository.ErrInUse) {
		response.Err(w, http.StatusBadRequest, "the default collection cannot be renamed")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to update collection")
		return
	}
	response.OK(w, http.StatusOK, c)
}

func (h *CollectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	err := h.Repo.Delete(r.Context(), id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "collection not found")
		return
	}
	if errors.Is(err, repository.ErrInUse) {
		response.Err(w, http.StatusBadRequest, "the default collection cannot be deleted")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to delete collection")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addQuestionBody struct {
	QuestionID string `json:"questionId"`
}

func (h *CollectionHandler) AddQuestion(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	var body addQuestionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.QuestionID = strings.TrimSpace(body.QuestionID)
	if body.QuestionID == "" {
		response.Err(w, http.StatusBadRequest, "questionId is required")
		return
	}
	if err := h.Repo.AddQuestion(r.Context(), id, userID, body.QuestionID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.Err(w, http.StatusNotFound, "collection or question not found")
			return
		}
		response.Err(w, http.StatusInternalServerError, "failed to add question")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CollectionHandler) RemoveQuestion(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	qid := chi.URLParam(r, "qid")
	if err := h.Repo.RemoveQuestion(r.Context(), id, userID, qid); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.Err(w, http.StatusNotFound, "collection not found")
			return
		}
		response.Err(w, http.StatusInternalServerError, "failed to remove question")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
