package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
)

type ProfileHandler struct {
	Profiles      *repository.ProfileRepo
	Extractor     agent.ProfileExtractor
	MaxResumeBytes int64
}

// Get returns the caller's profile, or 200 with an empty payload when they
// haven't created one yet (so the UI can decide to send them to onboarding).
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	p, err := h.Profiles.Get(r.Context(), userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.OK(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	response.OK(w, http.StatusOK, p)
}

type upsertProfileBody struct {
	TargetRole      string   `json:"targetRole"`
	YearsExperience int      `json:"yearsExperience"`
	Seniority       string   `json:"seniority"`
	CurrentRole     string   `json:"currentRole"`
	TechStack       []string `json:"techStack"`
	TargetCompanies []string `json:"targetCompanies"`
	Goals           string   `json:"goals"`
	MarkOnboarded   bool     `json:"markOnboarded"`
}

// Upsert saves the manual-form profile values. When markOnboarded=true and the
// row had no onboarded_at, we stamp it now so the frontend can stop nagging.
func (h *ProfileHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body upsertProfileBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Seniority = strings.ToLower(strings.TrimSpace(body.Seniority))
	switch body.Seniority {
	case "", "junior", "mid", "senior", "staff", "principal":
	default:
		response.Err(w, http.StatusBadRequest, "invalid seniority")
		return
	}
	if body.YearsExperience < 0 {
		body.YearsExperience = 0
	}
	if body.YearsExperience > 60 {
		body.YearsExperience = 60
	}

	p, err := h.Profiles.Upsert(r.Context(), repository.UpsertProfileInput{
		UserID:          userID,
		TargetRole:      strings.TrimSpace(body.TargetRole),
		YearsExperience: body.YearsExperience,
		Seniority:       body.Seniority,
		CurrentRole:     strings.TrimSpace(body.CurrentRole),
		TechStack:       cleanStringList(body.TechStack, 30),
		TargetCompanies: cleanStringList(body.TargetCompanies, 30),
		Goals:           strings.TrimSpace(body.Goals),
		MarkOnboarded:   body.MarkOnboarded,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to save profile")
		return
	}
	response.OK(w, http.StatusOK, p)
}

// UploadResume accepts a multipart upload (field name "resume"), runs it
// through the AI extractor, and persists the extracted profile + raw text.
func (h *ProfileHandler) UploadResume(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	maxBytes := h.MaxResumeBytes
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024 // 10MB cap
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1024)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		response.Err(w, http.StatusBadRequest, "resume upload too large or malformed")
		return
	}
	file, header, err := r.FormFile("resume")
	if err != nil {
		response.Err(w, http.StatusBadRequest, "missing 'resume' file part")
		return
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to read resume")
		return
	}

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/pdf"
	}

	// Extraction can be slow — bound it.
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	extracted, err := h.Extractor.Extract(ctx, bytes, mime, header.Filename)
	if err != nil {
		response.Err(w, http.StatusBadGateway, "resume extraction failed: "+err.Error())
		return
	}

	// Persist resume text first so we don't lose it on subsequent upsert.
	if err := h.Profiles.UpdateResume(r.Context(), userID, extracted.ResumePlainText, header.Filename); err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to save resume")
		return
	}

	// Merge into structured fields. We don't mark onboarded — the user still
	// has to confirm in the UI.
	p, err := h.Profiles.Upsert(r.Context(), repository.UpsertProfileInput{
		UserID:          userID,
		TargetRole:      strings.TrimSpace(extracted.TargetRole),
		YearsExperience: extracted.YearsExperience,
		Seniority:       strings.ToLower(strings.TrimSpace(extracted.Seniority)),
		CurrentRole:     strings.TrimSpace(extracted.CurrentRole),
		TechStack:       cleanStringList(extracted.TechStack, 30),
		Goals:           strings.TrimSpace(extracted.Goals),
		MarkOnboarded:   false,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to save extracted profile")
		return
	}
	// Re-fetch so resume_text shows up in the response.
	if got, err := h.Profiles.Get(r.Context(), userID); err == nil {
		p = got
	}
	response.OK(w, http.StatusOK, p)
}

func cleanStringList(in []string, max int) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	return out
}
