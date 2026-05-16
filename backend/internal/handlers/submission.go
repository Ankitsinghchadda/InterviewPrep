package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/submissions"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

type SubmissionHandler struct {
	Repo   *repository.SubmissionRepo
	Broker *submissions.Broker
}

func (h *SubmissionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	s, err := h.Repo.Get(r.Context(), id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "submission not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load submission")
		return
	}
	response.OK(w, http.StatusOK, s)
}

// ListForQuestion returns the caller's submission history for a single
// question, newest first. Used by QuestionDetail to surface past attempts
// (so the user can read prior feedback) and to detect an in-flight submission
// to resume — the frontend filters on status client-side.
func (h *SubmissionHandler) ListForQuestion(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	questionID := chi.URLParam(r, "id")
	out, err := h.Repo.ListForUserAndQuestion(r.Context(), userID, questionID, 50)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to list submissions")
		return
	}
	response.OK(w, http.StatusOK, out)
}

func (h *SubmissionHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	out, err := h.Repo.ListForUser(r.Context(), userID, 25)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to list submissions")
		return
	}
	response.OK(w, http.StatusOK, out)
}

// Stream is the SSE endpoint for a submission's review pipeline.
//
// Event types written to the client:
//   - "transcript"   — final transcript (one-shot)
//   - "review_token" — partial text chunk as the agent generates
//   - "review_done"  — final structured submission row as JSON (one-shot, terminal)
//   - "error"        — pipeline failure message (terminal)
//
// Late-subscriber behavior: if the submission is already complete/failed when
// the client opens the stream, we read the DB row and emit a single
// review_done (or error) event then close. This makes the endpoint safe to
// call after a page refresh.
func (h *SubmissionHandler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")

	sub, err := h.Repo.Get(r.Context(), id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "submission not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load submission")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Err(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	write := func(eventType, data string) bool {
		// SSE wire format: `event:` line, then one or more `data:` lines,
		// terminated by a blank line. Each newline in `data` needs its own
		// data-prefix per the spec.
		if _, err := fmt.Fprintf(w, "event: %s\n", eventType); err != nil {
			return false
		}
		for _, line := range strings.Split(data, "\n") {
			if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
				return false
			}
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Already done? Emit a one-shot snapshot and close.
	if sub.Status == "complete" || sub.Status == "failed" {
		if sub.Transcript != "" {
			write("transcript", sub.Transcript)
		}
		if sub.Status == "complete" {
			payload, _ := json.Marshal(sub)
			write("review_done", string(payload))
		} else {
			write("error", firstNonEmpty(sub.ErrorMessage, "submission failed"))
		}
		return
	}

	// Active pipeline — subscribe and forward events. Keepalive comments every
	// 20s prevent intermediaries from idle-closing the connection.
	if h.Broker == nil {
		write("error", "streaming broker not configured")
		return
	}

	ch, unsubscribe := h.Broker.Subscribe(id)
	defer unsubscribe()

	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				// Broker closed for this submission. Re-fetch DB state and emit
				// the terminal event so reconnect-mid-stream clients converge.
				if final, err := h.Repo.Get(r.Context(), id, userID); err == nil {
					if final.Status == "complete" {
						payload, _ := json.Marshal(final)
						write("review_done", string(payload))
					} else if final.Status == "failed" {
						write("error", firstNonEmpty(final.ErrorMessage, "submission failed"))
					}
				}
				return
			}
			if !write(ev.Kind, ev.Data) {
				return // client disconnected
			}
			if ev.Kind == "review_done" || ev.Kind == "error" {
				return
			}
		case <-keepalive.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
