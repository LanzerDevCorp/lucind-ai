// Package conflicttriage is an advisory, fail-open agent that explains
// ClassRequired overlap and records risk without the resolver's fail-closed
// contract. It does not encode a numeric risk formula; callers pin a band.
package conflicttriage

// Risk bands recorded on a triage payload. There is no numeric mapping.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// Hunk kinds for the three-hunk fixture shape. Mechanical kinds resolve
// deterministically; business hunks with no technical criterion are ARBITRARY.
const (
	HunkKindBusiness     = "business"
	HunkKindSliceUnion   = "slice_union"
	HunkKindRenameVsEdit = "rename_vs_edit"
)

// ResolutionArbitrary flags a business hunk with no technical selection criterion.
const ResolutionArbitrary = "ARBITRARY"

// VerifyBudgetExample is the wall-clock verify-budget shape: "~N min: <cmd>".
const VerifyBudgetExample = "~4 min: ./lucind-checks.sh"

// HunkDecision is one hunk's classification and proposed resolution.
type HunkDecision struct {
	HunkID     string `json:"hunk_id"`
	Kind       string `json:"kind"`
	Resolution string `json:"resolution"`
	Rationale  string `json:"rationale"`
}

// TriagePayload is the JSON stored in reconcile.Candidate.Output.
// ProposedSHA is advisory only; CandidateSHA stays human-owned.
type TriagePayload struct {
	CauseSummary  string         `json:"cause_summary"`
	HunkDecisions []HunkDecision `json:"hunk_decisions"`
	RiskBand      string         `json:"risk_band"` // low | medium | high
	VerifyBudget  string         `json:"verify_budget"`
	ProposedSHA   string         `json:"proposed_sha"`
}
