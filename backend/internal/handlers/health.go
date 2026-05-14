package handlers

import (
	"net/http"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
)

func Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}
