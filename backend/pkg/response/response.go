package response

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func OK(w http.ResponseWriter, status int, data any) {
	JSON(w, status, Envelope{Data: data})
}

func Err(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, Envelope{Error: msg})
}
