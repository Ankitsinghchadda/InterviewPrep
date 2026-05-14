package handlers

import (
	"errors"
	"io"
	"net/http"
)

// maxClientTranscriptBytes caps the optional client-supplied transcript field
// on multipart submissions to keep the request bounded.
const maxClientTranscriptBytes = 10 * 1024

// readClientTranscript pulls an optional "transcript" form value off a request
// that has already been parsed by readAudioPart. Truncates defensively.
func readClientTranscript(r *http.Request) string {
	v := r.FormValue("transcript")
	if len(v) > maxClientTranscriptBytes {
		v = v[:maxClientTranscriptBytes]
	}
	return v
}

// readAudioPart parses a multipart form, enforces a size cap, and returns the
// "audio" file part plus its MIME type. Caller MUST close the returned reader.
func readAudioPart(w http.ResponseWriter, r *http.Request, maxBytes int64) (io.ReadCloser, string, error) {
	if maxBytes <= 0 {
		maxBytes = 15 * 1024 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1024)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		return nil, "", errors.New("audio upload too large or malformed")
	}
	file, header, err := r.FormFile("audio")
	if err != nil {
		return nil, "", errors.New("missing 'audio' file part")
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "audio/webm"
	}
	return file, mime, nil
}
