package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/embeddings"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/submissions"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

type QuestionHandler struct {
	Repo              *repository.QuestionRepo
	Profiles          *repository.ProfileRepo
	Collections       *repository.CollectionRepo // ?in_collection= filter + GET /{id}/collections
	Submitter         *submissions.Service
	Synth             tts.Synthesizer
	Embedder          embeddings.Embedder     // nil disables semantic search + dedup + auto-embed
	AnswerGenerator   agent.AnswerGenerator   // nil rejects blank-answer submissions
	QuestionGenerator agent.QuestionGenerator // nil disables POST /questions/generate
	Billing           *billing.Service
	MaxAudioBytes     int64
	WarnThreshold     float64 // cosine sim ≥ this surfaces a row as "similar" (default 0.78)
	BlockThreshold    float64 // cosine sim ≥ this triggers a 409 on save unless ?force=true (default 0.88)
}

// List returns questions. Query params:
//   q=<text>                   when non-empty, runs hybrid semantic + keyword search
//   categories=docker,backend  (comma-separated slugs, OR-match)
//   difficulty=easy|medium|hard
//   mine=true                  (only the caller's questions)
//   in_collection=<uuid>       only questions in the given collection (must be owned by caller)
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

	if cid := strings.TrimSpace(r.URL.Query().Get("in_collection")); cid != "" {
		if userID == "" || h.Collections == nil {
			response.Err(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := h.Collections.Get(r.Context(), cid, userID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				response.Err(w, http.StatusNotFound, "collection not found")
				return
			}
			response.Err(w, http.StatusInternalServerError, "failed to load collection")
			return
		}
		f.CollectionID = cid
	}

	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		h.searchQuestions(w, r, q, f)
		return
	}

	qs, err := h.Repo.List(r.Context(), f)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to list questions")
		return
	}
	response.OK(w, http.StatusOK, qs)
}

// CollectionsForQuestion returns the ids of the caller's collections that
// already include this question. Used by the bookmark / "Save to..." menu to
// render which collections are toggled on without scanning the full membership
// in the client.
func (h *QuestionHandler) CollectionsForQuestion(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.Collections == nil {
		response.OK(w, http.StatusOK, []string{})
		return
	}
	id := chi.URLParam(r, "id")
	ids, err := h.Collections.CollectionIDsForQuestion(r.Context(), userID, id)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load collection membership")
		return
	}
	response.OK(w, http.StatusOK, ids)
}

// searchQuestions powers the ?q=… branch of List. Embeds the query with the
// RETRIEVAL_QUERY task type when the Embedder is available; otherwise it
// degrades gracefully to keyword-only ranking.
func (h *QuestionHandler) searchQuestions(
	w http.ResponseWriter, r *http.Request, q string, f repository.ListQuestionsFilter,
) {
	sf := repository.SearchFilter{ListQuestionsFilter: f, Query: q}

	if h.Embedder != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		vec, err := h.Embedder.EmbedOne(ctx, q, embeddings.TaskRetrievalQuery)
		if err != nil {
			log.Printf("search: embed query failed (falling back to keyword-only): %v", err)
		} else {
			sf.QueryEmbedding = vec
		}
	}

	hits, err := h.Repo.Search(r.Context(), sf)
	if err != nil {
		log.Printf("search: repo failed: %v", err)
		response.Err(w, http.StatusInternalServerError, "search failed")
		return
	}
	response.OK(w, http.StatusOK, hits)
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
	if body.Title == "" {
		response.Err(w, http.StatusBadRequest, "title is required")
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

	force := r.URL.Query().Get("force") == "true"

	// Embed title+body so we can soft-block near-duplicates before saving.
	// Best-effort: if the embedder is missing or fails, we fall through and
	// the question is created without the dedup check.
	queryVec := h.embedForDedup(r.Context(), body.Title, body.Body)
	if !force && queryVec != nil {
		matches, err := h.Repo.FindSimilar(r.Context(), repository.FindSimilarFilter{
			Embedding: queryVec,
			OwnerID:   userID,
			Limit:     3,
			MinScore:  h.blockThreshold(),
		})
		if err != nil {
			log.Printf("create question: FindSimilar failed: %v", err)
		} else if len(matches) > 0 {
			response.OK(w, http.StatusConflict, map[string]any{
				"error":   "similar_question_exists",
				"matches": matches,
			})
			return
		}
	}

	// Adding a question (with or without auto-answer) burns one
	// question_add slot. AnswerGen on this path rolls into the same
	// budget — we don't double-charge.
	if !checkQuota(w, r, h.Billing, billing.KindQuestionAdd) {
		return
	}

	// Auto-generate the reference answer when the user leaves it blank. The
	// frontend's Zod schema makes the field optional; the server decides
	// whether to draft one or reject.
	if body.Answer == "" {
		if h.AnswerGenerator == nil {
			response.Err(w, http.StatusBadRequest, "answer is required (no generator configured)")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		gen, err := h.AnswerGenerator.Generate(ctx, agent.GenerateAnswerInput{
			Title:      body.Title,
			Body:       body.Body,
			Difficulty: body.Difficulty,
			Categories: body.CategorySlugs,
		})
		if err != nil {
			log.Printf("create question: answer generation failed: %v", err)
			response.Err(w, http.StatusBadGateway, "failed to generate reference answer; please try again or write one yourself")
			return
		}
		body.Answer = strings.TrimSpace(gen)
		// AnswerGen rolls into the question-add budget — same call surface.
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
	embedAndStore(h.Embedder, h.Repo, q)
	safeRecord(r.Context(), h.Billing, billing.KindQuestionAdd, nil)
	response.OK(w, http.StatusCreated, q)
}

// Similar returns up to 5 existing questions whose embeddings are close to the
// title+body the user is currently typing. Powers the live "looks similar to"
// panel in the create dialog. Cheap enough to call on every debounced keystroke.
func (h *QuestionHandler) Similar(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if len(body.Title) < 8 {
		response.OK(w, http.StatusOK, []repository.SimilarQuestion{})
		return
	}

	vec := h.embedForDedup(r.Context(), body.Title, body.Body)
	if vec == nil {
		response.OK(w, http.StatusOK, []repository.SimilarQuestion{})
		return
	}

	matches, err := h.Repo.FindSimilar(r.Context(), repository.FindSimilarFilter{
		Embedding: vec,
		OwnerID:   userID,
		Limit:     5,
		MinScore:  h.warnThreshold(),
	})
	if err != nil {
		log.Printf("similar: FindSimilar failed: %v", err)
		response.Err(w, http.StatusInternalServerError, "failed to find similar questions")
		return
	}
	response.OK(w, http.StatusOK, matches)
}

// GenerateAnswer drafts a reference answer for an unsaved question. The
// frontend uses this for the "Generate draft" button in the create dialog so
// the user can preview/edit before saving. No persistence — purely a helper.
func (h *QuestionHandler) GenerateAnswer(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserID(r.Context()); !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Title         string   `json:"title"`
		Body          string   `json:"body"`
		Difficulty    string   `json:"difficulty"`
		CategorySlugs []string `json:"categories"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		response.Err(w, http.StatusBadRequest, "title is required")
		return
	}
	if h.AnswerGenerator == nil {
		response.Err(w, http.StatusServiceUnavailable, "answer generator is not configured")
		return
	}
	if !checkQuota(w, r, h.Billing, billing.KindAnswerGen) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	gen, err := h.AnswerGenerator.Generate(ctx, agent.GenerateAnswerInput{
		Title:      body.Title,
		Body:       body.Body,
		Difficulty: body.Difficulty,
		Categories: body.CategorySlugs,
	})
	if err != nil {
		log.Printf("generate-answer: %v", err)
		response.Err(w, http.StatusBadGateway, "failed to generate reference answer")
		return
	}
	safeRecord(r.Context(), h.Billing, billing.KindAnswerGen, nil)
	response.OK(w, http.StatusOK, map[string]string{"answer": strings.TrimSpace(gen)})
}

// Generate produces a batch of AI-authored questions for the given category
// slugs and persists them as public catalog rows (owner_id NULL,
// source='ai-generated'). Used when a user filters the library by a skill that
// has no curated questions and clicks "Generate with AI", or to grow any
// topic with more questions.
//
// Duplicate handling is layered:
//  1. Existing titles for the same categories are sent to the model as an
//     AVOID list (prompt-side).
//  2. After generation, every candidate title is normalized and dropped if it
//     matches any existing title OR any earlier item in this batch.
func (h *QuestionHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserID(r.Context()); !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Categories []string `json:"categories"`
		Difficulty string   `json:"difficulty"`
		Count      int      `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cats := make([]string, 0, len(body.Categories))
	for _, s := range body.Categories {
		s = strings.TrimSpace(s)
		if s != "" {
			cats = append(cats, s)
		}
	}
	if len(cats) == 0 {
		response.Err(w, http.StatusBadRequest, "at least one category slug is required")
		return
	}
	if body.Difficulty != "" {
		switch body.Difficulty {
		case "easy", "medium", "hard":
		default:
			response.Err(w, http.StatusBadRequest, "difficulty must be easy, medium, or hard")
			return
		}
	}
	if body.Count <= 0 {
		body.Count = 5
	}
	if body.Count > 10 {
		body.Count = 10
	}
	if h.QuestionGenerator == nil {
		response.Err(w, http.StatusServiceUnavailable, "question generator is not configured")
		return
	}
	if !checkQuota(w, r, h.Billing, billing.KindQuestionGen) {
		return
	}

	created, err := seedAIQuestionsForCategories(
		r.Context(),
		h.Repo,
		h.QuestionGenerator,
		h.Embedder,
		h.Synth,
		cats,
		body.Count,
		body.Difficulty,
	)
	if err != nil {
		log.Printf("generate questions: %v", err)
		response.Err(w, http.StatusBadGateway, "failed to generate questions")
		return
	}
	safeRecord(r.Context(), h.Billing, billing.KindQuestionGen, map[string]any{"count": len(created)})

	response.OK(w, http.StatusCreated, created)
}

// normalizeTitle lowercases, trims, collapses internal whitespace, and strips
// trailing terminal punctuation so superficially-different question titles
// compare equal during dedup.
func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimRight(s, "?.!")
	return s
}

// embedForDedup runs a short-deadline EmbedOne on title+body. Returns nil
// (not an error) when the embedder is unavailable or the call fails — both
// degrade gracefully to "skip the dedup check".
func (h *QuestionHandler) embedForDedup(parent context.Context, title, body string) []float32 {
	if h.Embedder == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	text := repository.BuildEmbedText(title, body, "")
	vec, err := h.Embedder.EmbedOne(ctx, text, embeddings.TaskRetrievalQuery)
	if err != nil {
		log.Printf("embed-for-dedup: %v", err)
		return nil
	}
	return vec
}

func (h *QuestionHandler) warnThreshold() float64 {
	if h.WarnThreshold <= 0 {
		return 0.78
	}
	return h.WarnThreshold
}

func (h *QuestionHandler) blockThreshold() float64 {
	if h.BlockThreshold <= 0 {
		return 0.88
	}
	return h.BlockThreshold
}

// embedAndStore embeds a freshly-created question in the background and
// persists the vector. Best-effort: failure logs but does not fail the create.
// Pattern mirrors synthesizeAndStore for the audio TTS goroutine.
func embedAndStore(c embeddings.Embedder, repo *repository.QuestionRepo, q *models.Question) {
	if c == nil || q == nil {
		return
	}
	text := repository.BuildEmbedText(q.Title, q.Body, q.Answer)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		vec, err := c.EmbedOne(ctx, text, embeddings.TaskRetrievalDocument)
		if err != nil {
			log.Printf("embed: question %s failed: %v", q.ID, err)
			return
		}
		if err := repo.UpdateEmbedding(ctx, q.ID, vec); err != nil {
			log.Printf("embed: persist for %s failed: %v", q.ID, err)
		}
	}()
}

// Recommendations returns three performance-aware buckets for the dashboard:
// weakAreas, levelUp, and goalGaps. Each item carries a Reason string the UI
// shows beneath the question card.
func (h *QuestionHandler) Recommendations(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	goalSlugs := []string{}
	if h.Profiles != nil {
		if p, err := h.Profiles.Get(r.Context(), userID); err == nil && p != nil {
			if p.TargetRole != "" {
				goalSlugs = append(goalSlugs, p.TargetRole)
			}
			for _, t := range p.TechStack {
				if s := slugify(t); s != "" {
					goalSlugs = append(goalSlugs, s)
				}
			}
		}
	}

	out, err := h.Repo.Recommendations(r.Context(), repository.RecommendInput{
		UserID:    userID,
		GoalSlugs: goalSlugs,
		PerBucket: 4,
	})
	if err != nil {
		log.Printf("recommendations: %v", err)
		response.Err(w, http.StatusInternalServerError, "failed to load recommendations")
		return
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
		if errors.Is(err, repository.ErrInUse) {
			response.Err(w, http.StatusConflict,
				"this question is part of an interview and can't be deleted")
			return
		}
		log.Printf("delete question %s: %v", id, err)
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
	if !checkQuota(w, r, h.Billing, billing.KindRecordingReview) {
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
	// Record at submit time: cost is committed once the SSE stream kicks
	// off — we don't want a failed transcription to leave a hole in the
	// user's weekly budget either, but the alternative (record on stream
	// completion) leaks free reviews on disconnect.
	safeRecord(r.Context(), h.Billing, billing.KindRecordingReview, map[string]any{"submission_id": sub.ID})
	response.OK(w, http.StatusAccepted, sub)
}
