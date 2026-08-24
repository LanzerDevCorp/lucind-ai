package conflicttriage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/resolve"
)

// RunOptions configures an advisory triage pass. Invoker is required so
// production executor/model selection stays out of this package.
type RunOptions struct {
	CandidateID  string
	WorktreePath string
	BaseSHA      string
	AllowedPaths []string
	Invoker      TriageInvoker
	Service      *reconcile.Service
}

// RunResult is the persisted advisory payload and the candidate it was written to.
type RunResult struct {
	Payload   TriagePayload
	Candidate reconcile.Candidate
}

// RunTriage invokes the advisory agent, writes JSON to Candidate.Output only,
// fail-opens on semantic ambiguity, then runs the resolver's invariant helpers
// as read-only checks. It never writes CandidateSHA.
func RunTriage(ctx context.Context, opts RunOptions) (RunResult, error) {
	if opts.Service == nil {
		return RunResult{}, errors.New("conflicttriage: reconcile service is required")
	}
	if opts.Invoker == nil {
		return RunResult{}, errors.New("conflicttriage: TriageInvoker is required")
	}

	raw, invErr := opts.Invoker(ctx, opts.WorktreePath, triagePrompt(opts.AllowedPaths))
	payload := decodeOrFailOpen(raw)
	pinBusinessHigh(&payload)
	if !validVerifyBudget(payload.VerifyBudget) {
		payload.VerifyBudget = VerifyBudgetExample
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return RunResult{}, fmt.Errorf("conflicttriage: marshal payload: %w", err)
	}

	cand, err := opts.Service.UpdateCandidateOutput(ctx, opts.CandidateID, string(encoded))
	if err != nil {
		return RunResult{}, err
	}

	// Fail-open: invoker errors (including ErrSemanticAmbiguity) do not abort
	// after JSON is persisted. Invariants below still apply.
	_ = invErr

	if hasMarkers, markerFiles, scanErr := resolve.ScanConflictMarkers(opts.WorktreePath); scanErr != nil {
		return failCandidate(ctx, opts, cand, payload, fmt.Errorf("conflicttriage: scan conflict markers: %w", scanErr))
	} else if hasMarkers {
		reason := fmt.Sprintf("conflict markers remain in worktree: %s", strings.Join(markerFiles, ", "))
		return failCandidate(ctx, opts, cand, payload, errors.New(reason))
	}

	if offending, scopeErr := resolve.EnforceAllowedPaths(ctx, opts.WorktreePath, opts.BaseSHA, opts.AllowedPaths); scopeErr != nil && !errors.Is(scopeErr, resolve.ErrOutOfScopeEdits) {
		return failCandidate(ctx, opts, cand, payload, fmt.Errorf("conflicttriage: enforce allowed paths: %w", scopeErr))
	} else if len(offending) > 0 {
		return failCandidate(ctx, opts, cand, payload, fmt.Errorf("%w: %s", resolve.ErrOutOfScopeEdits, strings.Join(offending, ", ")))
	}

	return RunResult{Payload: payload, Candidate: cand}, nil
}

func failCandidate(ctx context.Context, opts RunOptions, cand reconcile.Candidate, payload TriagePayload, cause error) (RunResult, error) {
	updated, err := opts.Service.UpdateCandidateStatus(ctx, cand.ID, reconcile.CandidateStatusFailed, "", cause.Error())
	if err != nil {
		return RunResult{Payload: payload, Candidate: cand}, fmt.Errorf("conflicttriage: mark candidate failed: %w", err)
	}
	return RunResult{Payload: payload, Candidate: updated}, cause
}

func decodeOrFailOpen(raw string) TriagePayload {
	var payload TriagePayload
	if strings.TrimSpace(raw) == "" {
		return failOpenPayload()
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return failOpenPayload()
	}
	return payload
}

func failOpenPayload() TriagePayload {
	return TriagePayload{
		CauseSummary: "triage invoker failed open; business hunk flagged ARBITRARY",
		HunkDecisions: []HunkDecision{{
			HunkID:     "hunk-business",
			Kind:       HunkKindBusiness,
			Resolution: ResolutionArbitrary,
			Rationale:  "no technical selection criterion",
		}},
		RiskBand:     RiskHigh,
		VerifyBudget: VerifyBudgetExample,
	}
}

func pinBusinessHigh(p *TriagePayload) {
	hasBusiness := false
	for i := range p.HunkDecisions {
		if p.HunkDecisions[i].Kind == HunkKindBusiness {
			hasBusiness = true
			p.HunkDecisions[i].Resolution = ResolutionArbitrary
		}
	}
	if hasBusiness {
		p.RiskBand = RiskHigh
	}
}

func validVerifyBudget(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "~") {
		return false
	}
	return strings.Contains(s, " min: ")
}

func triagePrompt(allowedPaths []string) string {
	var b strings.Builder
	b.WriteString("Explain this ClassRequired overlap. Resolve mechanical hunks deterministically. ")
	b.WriteString("For a business hunk with no technical selection criterion, flag ARBITRARY, pin risk high, ")
	b.WriteString("and state verify cost as wall-clock duration plus a concrete command (~N min: <cmd>). ")
	b.WriteString("Emit JSON matching TriagePayload. Do not fail closed on semantic ambiguity.\n")
	if len(allowedPaths) > 0 {
		b.WriteString("Allowed paths: ")
		b.WriteString(strings.Join(allowedPaths, ", "))
		b.WriteString("\n")
	}
	return b.String()
}
