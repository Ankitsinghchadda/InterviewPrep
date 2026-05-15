package embeddings

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// Embedder is the small surface needed by handlers/repos. The Vertex *Client
// in this package already satisfies it; Stub is the offline fallback.
type Embedder interface {
	EmbedOne(ctx context.Context, text, task string) ([]float32, error)
}

// Stub returns a deterministic, hash-based pseudo-embedding so the dedup UX
// can be exercised end-to-end without GCP. It is intentionally simple: it
// tokenizes the text, hashes each token into one of Dim buckets, and L2-
// normalizes the result. Two strings sharing many tokens land near each other
// under cosine similarity, which is enough to demo the warn/block flow.
type Stub struct{}

func (Stub) EmbedOne(_ context.Context, text, _ string) ([]float32, error) {
	vec := make([]float32, Dim)
	tokens := tokenize(text)
	if len(tokens) == 0 {
		// Empty input still needs a unit-norm vector or pgvector's cosine ops
		// will reject NaN. Put all the weight on a single dim.
		vec[0] = 1
		return vec, nil
	}
	for _, t := range tokens {
		h := fnv.New32a()
		_, _ = h.Write([]byte(t))
		idx := int(h.Sum32()) % Dim
		if idx < 0 {
			idx += Dim
		}
		vec[idx] += 1
	}
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := float32(math.Sqrt(sum))
	if norm == 0 {
		vec[0] = 1
		return vec, nil
	}
	for i := range vec {
		vec[i] /= norm
	}
	return vec, nil
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) >= 2 {
			out = append(out, f)
		}
	}
	return out
}
