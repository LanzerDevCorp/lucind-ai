package dispatch

import (
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// ResolveModel resolves the model name using the precedence:
// 1. flagModel if non-empty
// 2. os.Getenv("LUCIND_AGY_MODEL") if non-empty
// 3. Default: "gemini-3.7-flash-high"
// It validates against executor.KnownModels().
func ResolveModel(flagModel string) (string, error) {
	return executor.ResolveModel(flagModel)
}
