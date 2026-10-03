package router

import (
	"context"
	"fmt"

	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

// Signals contains ONLY numbers and booleans.
// This is a data-protection rule: Jev must never receive text, paths, code, or identifiers.
type Signals struct {
	AllowedPathCount int  `json:"allowed_path_count"`
	NewFile          bool `json:"new_file"`
	RiskTierLevel    int  `json:"risk_tier_level"` // 0 passive, 1 medium, 2 high
	ReadOnly         bool `json:"read_only"`
	// Signals declared by the orchestrator. UnderstoodDeclared distinguishes "not understood"
	// from "not declared".
	UnderstoodDeclared bool `json:"understood_declared"`
	Understood         bool `json:"understood"`
	OpenDesign         bool `json:"open_design"`
	EstimatedLookups   int  `json:"estimated_lookups"`
}

// maxInlineLookups mirrors the dispatch-threshold evidence budget for inline work.
const maxInlineLookups = 5

// Decision represents the routing choice produced by a Router.
type Decision struct {
	Route      string   `json:"route"`      // inline, worker, fanout
	Source     string   `json:"source"`     // deterministic or jev
	Confidence float64  `json:"confidence"` // 0.0 - 1.0
	Reasons    []string `json:"reasons,omitempty"`
}

// Router chooses an execution route for a given set of numeric/boolean signals.
type Router interface {
	Route(ctx context.Context, s Signals) (Decision, error)
}

// SignalsFromRiskTier maps a risk.Tier to a numeric level:
// passive 0, medium 1, high 2, unknown 2.
func SignalsFromRiskTier(t risk.Tier) int {
	switch t {
	case risk.TierPassive:
		return 0
	case risk.TierMedium:
		return 1
	case risk.TierHigh:
		return 2
	default:
		return 2
	}
}

// Deterministic provides the baseline and permanent fallback routing logic.
// It never produces fanout and never errors.
type Deterministic struct{}

// Route evaluates hard signals: AllowedPathCount >= 2, NewFile, RiskTierLevel >= 2.
// If any hard signal holds, it routes to worker; otherwise inline.
func (d Deterministic) Route(ctx context.Context, s Signals) (Decision, error) {
	var reasons []string

	if s.AllowedPathCount >= 2 {
		reasons = append(reasons, fmt.Sprintf("allowed_paths:%d", s.AllowedPathCount))
	}
	if s.NewFile {
		reasons = append(reasons, "new_file")
	}
	if s.RiskTierLevel >= 2 {
		reasons = append(reasons, "risk_tier:high")
	}
	if s.UnderstoodDeclared && !s.Understood {
		reasons = append(reasons, "understood:false")
	}
	if s.OpenDesign {
		reasons = append(reasons, "open_design:true")
	}
	if s.EstimatedLookups > maxInlineLookups {
		reasons = append(reasons, fmt.Sprintf("estimated_lookups:%d", s.EstimatedLookups))
	}

	route := "inline"
	if len(reasons) > 0 {
		route = "worker"
	}

	return Decision{
		Route:      route,
		Source:     "deterministic",
		Confidence: 1.0,
		Reasons:    reasons,
	}, nil
}
