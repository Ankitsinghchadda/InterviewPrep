package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/submissions"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

type QuestionHandler struct {
	Repo          *repository.QuestionRepo
	Profiles      *repository.ProfileRepo
	Submitter     *submissions.Service
	Synth         tts.Synthesizer
	MaxAudioBytes int64
}

// List returns questions. Query params:
//   categories=docker,backend  (comma-separated slugs, OR-match)
//   difficulty=easy|medium|hard
//   mine=true                  (only the caller's questions)
//   limit=50
func (h *QuestionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	f := repository.ListQuestionsFilter{
		OwnerID:    userID,
		Difficulty: r.URL.Query().Get("difficulty"),
		OnlyMine:   r.URL.Query().Get("mine") == "true",
	}
	if cats := r.URL.Query().Get("categories"); cats != "" {
		for _, s := range strings.Split(cats, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				f.CategorySlugs = append(f.CategorySlugs, s)
			}
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			f.Limit = n
		}
	}

	qs, err := h.Repo.List(r.Context(), f)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to list questions")
		return
	}
	response.OK(w, http.StatusOK, qs)
}

func (h *QuestionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	q, err := h.Repo.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load question")
		return
	}
	if q.OwnerID != nil {
		userID, _ := auth.UserID(r.Context())
		if *q.OwnerID != userID {
			response.Err(w, http.StatusNotFound, "question not found")
			return
		}
	}
	response.OK(w, http.StatusOK, q)
}

type createQuestionBody struct {
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	Answer        string   `json:"answer"`
	Difficulty    string   `json:"difficulty"`
	CategorySlugs []string `json:"categories"`
}

func (h *QuestionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body createQuestionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Answer = strings.TrimSpace(body.Answer)
	if body.Title == "" || body.Answer == "" {
		response.Err(w, http.StatusBadRequest, "title and answer are required")
		return
	}
	if body.Difficulty == "" {
		body.Difficulty = "medium"
	}
	switch body.Difficulty {
	case "easy", "medium", "hard":
	default:
		response.Err(w, http.StatusBadRequest, "difficulty must be easy, medium, or hard")
		return
	}

	q, err := h.Repo.Create(r.Context(), repository.CreateQuestionInput{
		Title:         body.Title,
		Body:          body.Body,
		Answer:        body.Answer,
		Difficulty:    body.Difficulty,
		OwnerID:       userID,
		CategorySlugs: body.CategorySlugs,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create question")
		return
	}
	synthesizeAndStore(h.Synth, h.Repo, q)
	response.OK(w, http.StatusCreated, q)
}

// Recommended returns a small set of curated questions tailored to the user's
// profile (target role + tech stack). Falls back to a generic random pick when
// no profile is set yet.
func (h *QuestionHandler) Recommended(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	categorySlugs := []string{}
	if h.Profiles != nil {
		if p, err := h.Profiles.Get(r.Context(), userID); err == nil && p != nil {
			if p.TargetRole != "" {
				categorySlugs = append(categorySlugs, p.TargetRole)
			}
			for _, t := range p.TechStack {
				categorySlugs = append(categorySlugs, slugify(t))
			}
		}
	}

	out, err := h.Repo.Recommended(r.Context(), userID, categorySlugs, 6)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load recommendations")
		return
	}
	// If the user has no profile and no matching curated questions, fall back
	// to a small generic random sample so the dashboard never feels empty.
	if len(out) == 0 {
		out, _ = h.Repo.PickRandom(r.Context(), nil, userID, 6)
	}
	response.OK(w, http.StatusOK, out)
}

// slugify lowercases and normalizes a free-text tech name into a candidate
// category slug. The repo's category JOIN just filters by exact match, so
// non-matching tokens silently drop out — they don't break the query.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ', r == '_', r == '/', r == '.':
			out = append(out, '-')
		case r == '-':
			out = append(out, '-')
		}
	}
	return string(out)
}

// Delete removes a user-owned question. Seeded (owner_id IS NULL) questions
// are never deletable through this endpoint; the repo returns ErrNotFound for
// any row whose owner_id doesn't match the caller.
func (h *QuestionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Repo.Delete(r.Context(), id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.Err(w, http.StatusNotFound, "question not found")
			return
		}
		response.Err(w, http.StatusInternalServerError, "failed to delete question")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SubmitAnswer is the standalone Practice endpoint. The mock-interview flow
// uses InterviewHandler.SubmitAnswer instead so submissions get linked to the
// interview row.
func (h *QuestionHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	questionID := chi.URLParam(r, "id")

	question, err := h.Repo.Get(r.Context(), questionID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load question")
		return
	}
	if question.OwnerID != nil && *question.OwnerID != userID {
		response.Err(w, http.StatusNotFound, "question not found")
		return
	}

	file, mimeType, err := readAudioPart(w, r, h.MaxAudioBytes)
	if err != nil {
		response.Err(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()
	clientTranscript := readClientTranscript(r)

	sub, err := h.Submitter.Submit(r.Context(), submissions.SubmitInput{
		UserID:     userID,
		Question:   question,
		Audio:      file,
		MimeType:   mimeType,
		Transcript: clientTranscript,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create submission")
		return
	}
	response.OK(w, http.StatusAccepted, sub)
}
