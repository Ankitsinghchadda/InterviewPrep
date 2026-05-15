// Package embeddings is a thin wrapper around Vertex AI text-embedding-005,
// reusing the same genai client + ADC setup as internal/services/agent.
package embeddings

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"
)

// TaskType values understood by the embed-content endpoint. Choosing the
// right one improves recall measurably: documents are embedded with
// RETRIEVAL_DOCUMENT, the user's search box query with RETRIEVAL_QUERY.
const (
	TaskRetrievalDocument = "RETRIEVAL_DOCUMENT"
	TaskRetrievalQuery    = "RETRIEVAL_QUERY"
)

// Dim is the output dimensionality of text-embedding-005. Must match the
// `vector(N)` column type in the questions table.
const Dim = 768

// DefaultModel is Vertex AI's general-purpose English embedding model.
const DefaultModel = "text-embedding-005"

// MaxBatch is the per-request input cap for text-embedding-005. The Vertex
// endpoint accepts up to 250; we stay a bit below to leave headroom.
const MaxBatch = 200

type Client struct {
	gc    *genai.Client
	model string
}

// New constructs an embeddings client backed by Vertex AI. project/location
// are typically the same values used elsewhere in the app (see config.Config).
func New(ctx context.Context, project, location, model string) (*Client, error) {
	if project == "" {
		return nil, errors.New("embeddings: GOOGLE_CLOUD_PROJECT is required")
	}
	if model == "" {
		model = DefaultModel
	}
	gc, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	})
	if err != nil {
		return nil, fmt.Errorf("embeddings: new genai client: %w", err)
	}
	return &Client{gc: gc, model: model}, nil
}

// Embed returns one embedding per input text, in the same order. Inputs are
// batched at MaxBatch per request — pass any number; the caller doesn't need
// to chunk. Empty strings are embedded as-is (the model handles them).
func (c *Client) Embed(ctx context.Context, texts []string, task string) ([][]float32, error) {
	if c == nil {
		return nil, errors.New("embeddings: nil client")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += MaxBatch {
		end := start + MaxBatch
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[start:end]
		contents := make([]*genai.Content, len(chunk))
		for i, t := range chunk {
			contents[i] = genai.NewContentFromText(t, genai.RoleUser)
		}
		cfg := &genai.EmbedContentConfig{TaskType: task}
		resp, err := c.gc.Models.EmbedContent(ctx, c.model, contents, cfg)
		if err != nil {
			return nil, fmt.Errorf("embeddings: embed content: %w", err)
		}
		if len(resp.Embeddings) != len(chunk) {
			return nil, fmt.Errorf("embeddings: model returned %d vectors for %d inputs",
				len(resp.Embeddings), len(chunk))
		}
		for _, e := range resp.Embeddings {
			if e == nil || len(e.Values) == 0 {
				return nil, errors.New("embeddings: empty vector in response")
			}
			out = append(out, e.Values)
		}
	}
	return out, nil
}

// EmbedOne is a convenience wrapper for the single-text case.
func (c *Client) EmbedOne(ctx context.Context, text, task string) ([]float32, error) {
	out, err := c.Embed(ctx, []string{text}, task)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("embeddings: no vector returned")
	}
	return out[0], nil
}
