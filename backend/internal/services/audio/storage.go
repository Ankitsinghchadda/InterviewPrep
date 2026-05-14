// Package audio provides storage for user-recorded answer audio.
// The Storage interface lets us swap a local-FS implementation in dev for
// cloud storage (GCS) in prod without touching call sites.
package audio

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Storage interface {
	// Save persists the audio under a deterministic key derived from owner + id
	// and returns an opaque storage key (e.g., "userID/submissionID.webm").
	Save(ctx context.Context, ownerID, id, ext string, r io.Reader) (key string, err error)
	// Read returns a reader for the stored audio. Caller closes it.
	Read(ctx context.Context, key string) (io.ReadCloser, error)
}

type LocalFS struct {
	Root string
}

func NewLocalFS(root string) (*LocalFS, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create audio root %q: %w", root, err)
	}
	return &LocalFS{Root: root}, nil
}

func (s *LocalFS) Save(_ context.Context, ownerID, id, ext string, r io.Reader) (string, error) {
	if ext == "" {
		ext = "webm"
	}
	dir := filepath.Join(s.Root, sanitize(ownerID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	key := filepath.Join(sanitize(ownerID), sanitize(id)+"."+ext)
	full := filepath.Join(s.Root, key)
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		_ = os.Remove(full)
		return "", err
	}
	return key, nil
}

func (s *LocalFS) Read(_ context.Context, key string) (io.ReadCloser, error) {
	// Guard against traversal: require key to be within the configured root.
	clean := filepath.Clean(key)
	if clean != key || filepath.IsAbs(clean) {
		return nil, fmt.Errorf("invalid storage key")
	}
	return os.Open(filepath.Join(s.Root, clean))
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_':
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "anon"
	}
	return string(out)
}
