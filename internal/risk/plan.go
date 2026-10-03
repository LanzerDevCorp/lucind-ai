// Package risk defines a minimal deterministic risk classifier for a candidate change
// set and the mapping from risk tier to required verification.
//
// The judges themselves run in internal/judges; this package only declares the plan.
package risk

// Plan describes the verification obligations required for a risk tier.
//
// The judges themselves run in internal/judges; this package only declares the plan.
type Plan struct {
	Tier               Tier
	StructuralReadback bool // true only for passive
	Attestation        bool // true for medium and high
	Judges             int  // passive 0, medium 1, high 2 (blind judges from different model families)
	ClaudeReview       bool // true only for high
}

// PlanFor returns the verification plan for the given tier.
// An unknown or empty tier yields the HIGH plan (fail closed).
func PlanFor(t Tier) Plan {
	switch t {
	case TierPassive:
		return Plan{
			Tier:               TierPassive,
			StructuralReadback: true,
			Attestation:        false,
			Judges:             0,
			ClaudeReview:       false,
		}
	case TierMedium:
		return Plan{
			Tier:               TierMedium,
			StructuralReadback: false,
			Attestation:        true,
			Judges:             1,
			ClaudeReview:       false,
		}
	case TierHigh:
		return Plan{
			Tier:               TierHigh,
			StructuralReadback: false,
			Attestation:        true,
			Judges:             2,
			ClaudeReview:       true,
		}
	default:
		return Plan{
			Tier:               TierHigh,
			StructuralReadback: false,
			Attestation:        true,
			Judges:             2,
			ClaudeReview:       true,
		}
	}
}
