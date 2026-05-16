package agent

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// Backend selects which Google AI surface the agent talks to.
//
// BackendVertex uses Vertex AI under the active GCP project (Application
// Default Credentials). BackendGeminiAPI uses the public Gemini API endpoint
// (generativelanguage.googleapis.com) authenticated with a raw API key —
// this path is reserved for paid users so we can ship premium models like
// gemini-2.5-pro without giving them Vertex quota.
type Backend int

const (
	BackendVertex Backend = iota
	BackendGeminiAPI
)

func (b Backend) String() string {
	if b == BackendGeminiAPI {
		return "gemini-api"
	}
	return "vertex"
}

// BuildGeminiModel constructs a model.LLM wired to the requested backend.
// Each agent constructor calls this and feeds the result into llmagent.New.
//
// For BackendVertex: project is required, location defaults to us-central1.
// For BackendGeminiAPI: apiKey is required.
func BuildGeminiModel(ctx context.Context, b Backend, modelName, project, location, apiKey string) (model.LLM, error) {
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}
	cfg := &genai.ClientConfig{}
	switch b {
	case BackendVertex:
		if project == "" {
			return nil, errors.New("agent: BackendVertex requires GOOGLE_CLOUD_PROJECT")
		}
		cfg.Backend = genai.BackendVertexAI
		cfg.Project = project
		cfg.Location = location
	case BackendGeminiAPI:
		if apiKey == "" {
			return nil, errors.New("agent: BackendGeminiAPI requires GEMINI_API_KEY")
		}
		cfg.Backend = genai.BackendGeminiAPI
		cfg.APIKey = apiKey
	default:
		return nil, fmt.Errorf("agent: unknown backend %d", b)
	}
	model, err := gemini.NewModel(ctx, modelName, cfg)
	if err != nil {
		return nil, fmt.Errorf("agent: build gemini model (%s, %s): %w", b, modelName, err)
	}
	return model, nil
}
