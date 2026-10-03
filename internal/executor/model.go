package executor

import (
	"fmt"
	"os"
)

const defaultModel = "gemini-3.7-flash-high"

var knownModels = []string{
	"gemini-3.7-flash-high",
	"gemini-3.8-flash-high",
	"gemini-3.8-flash-medium",
	"gemini-3.1-pro-high",
	"claude-opus-4-6-thinking",
}

// DefaultModel returns the default model.
func DefaultModel() string {
	return defaultModel
}

// KnownModels returns a copy of the supported model names.
func KnownModels() []string {
	models := make([]string, len(knownModels))
	copy(models, knownModels)
	return models
}

// IsValidModel reports whether m is one of the known models.
func IsValidModel(m string) bool {
	for _, km := range knownModels {
		if m == km {
			return true
		}
	}
	return false
}

// ResolveModel resolves the model identifier by following precedence:
// 1) flagModel if non-empty
// 2) LUCIND_AGY_MODEL environment variable if non-empty
// 3) DefaultModel()
// It validates the resolved model against KnownModels() and returns an error if unrecognized.
func ResolveModel(flagModel string) (string, error) {
	chosen := flagModel
	if chosen == "" {
		chosen = os.Getenv("LUCIND_AGY_MODEL")
	}
	if chosen == "" {
		chosen = DefaultModel()
	}

	if !IsValidModel(chosen) {
		return "", fmt.Errorf("unknown model %q; known models: %v", chosen, knownModels)
	}

	return chosen, nil
}
