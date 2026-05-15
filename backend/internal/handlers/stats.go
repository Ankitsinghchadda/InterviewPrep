package handlers

import (
	"log"
	"net/http"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
)

type StatsHandler struct {
	Repo     *repository.StatsRepo
	Profiles *repository.ProfileRepo
}

// Overview returns the full dashboard payload for the authenticated user.
func (h *StatsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetRole := ""
	var stack []string
	if h.Profiles != nil {
		if p, err := h.Profiles.Get(r.Context(), userID); err == nil && p != nil {
			targetRole = p.TargetRole
			stack = p.TechStack
		}
	}

	out, err := h.Repo.Overview(r.Context(), userID, targetRole, stack)
	if err != nil {
		log.Printf("stats overview: %v", err)
		response.Err(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	response.OK(w, http.StatusOK, out)
}
