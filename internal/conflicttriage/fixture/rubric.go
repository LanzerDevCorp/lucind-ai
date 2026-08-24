package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// Pinned judges for the offline rubric. Production triage runtime is unset.
const (
	JudgeClaudeExecutor   = "claude"
	JudgeClaudeModel      = "claude-opus-5"
	JudgeOpencodeExecutor = "opencode"
	JudgeOpencodeModel    = "openai/gpt-5.6-sol"
)

// RubricOptions runs the same fixture through the two pinned judges.
type RubricOptions struct {
	Claude       executor.Executor
	Opencode     executor.Executor
	WorktreePath string
}

// RubricResult is the A/B grade. ProposedSHA and human timing are ignored.
type RubricResult struct {
	Passed   bool
	Reason   string
	Claude   JudgeResult
	Opencode JudgeResult
}

// JudgeResult is one pinned judge's parsed payload and grade.
type JudgeResult struct {
	Executor string
	Model    string
	Payload  conflicttriage.TriagePayload
	Passed   bool
	Reason   string
}

// EvaluateRubric grades both pinned judges offline. It does not select a
// production triage executor or model.
func EvaluateRubric(ctx context.Context, opts RubricOptions) (RubricResult, error) {
	if opts.Claude == nil || opts.Opencode == nil {
		return RubricResult{}, errors.New("fixture: both Claude and Opencode judges are required")
	}

	claude, err := runJudge(ctx, opts.Claude, JudgeClaudeExecutor, JudgeClaudeModel, opts.WorktreePath)
	if err != nil {
		return RubricResult{}, fmt.Errorf("fixture: claude judge: %w", err)
	}
	opencode, err := runJudge(ctx, opts.Opencode, JudgeOpencodeExecutor, JudgeOpencodeModel, opts.WorktreePath)
	if err != nil {
		return RubricResult{}, fmt.Errorf("fixture: opencode judge: %w", err)
	}

	out := RubricResult{Claude: claude, Opencode: opencode}
	if !claude.Passed {
		out.Reason = "claude: " + claude.Reason
		return out, nil
	}
	if !opencode.Passed {
		out.Reason = "opencode: " + opencode.Reason
		return out, nil
	}
	out.Passed = true
	return out, nil
}

func runJudge(ctx context.Context, exec executor.Executor, name, model, worktree string) (JudgeResult, error) {
	outcome, err := exec.Run(ctx, executor.Request{
		Prompt:       rubricPrompt(worktree),
		WorktreePath: worktree,
		Model:        model,
	})
	if err != nil {
		return JudgeResult{}, err
	}
	payload, err := parsePayload(outcome.Stdout)
	if err != nil {
		return JudgeResult{}, err
	}
	passed, reason := gradePayload(payload)
	return JudgeResult{
		Executor: name,
		Model:    model,
		Payload:  payload,
		Passed:   passed,
		Reason:   reason,
	}, nil
}

func parsePayload(stdout string) (conflicttriage.TriagePayload, error) {
	raw := strings.TrimSpace(stdout)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var p conflicttriage.TriagePayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return conflicttriage.TriagePayload{}, fmt.Errorf("parse TriagePayload: %w", err)
	}
	return p, nil
}

func gradePayload(p conflicttriage.TriagePayload) (bool, string) {
	if len(p.HunkDecisions) != 3 {
		return false, fmt.Sprintf("want 3 hunk decisions, got %d", len(p.HunkDecisions))
	}
	if uniformHunkScoring(p.HunkDecisions) {
		return false, "uniform hunk scoring"
	}
	var business *conflicttriage.HunkDecision
	mechanical := 0
	for i := range p.HunkDecisions {
		h := &p.HunkDecisions[i]
		if h.Kind == conflicttriage.HunkKindBusiness {
			business = h
			continue
		}
		mechanical++
	}
	if business == nil {
		return false, "business hunk not distinguished"
	}
	if business.Resolution != conflicttriage.ResolutionArbitrary {
		return false, "business hunk is not ARBITRARY"
	}
	if mechanical != 2 {
		return false, "business hunk not separated from two mechanical controls"
	}
	return true, ""
}

func uniformHunkScoring(hs []conflicttriage.HunkDecision) bool {
	sameKind := true
	sameResolution := true
	for i := 1; i < len(hs); i++ {
		if hs[i].Kind != hs[0].Kind {
			sameKind = false
		}
		if hs[i].Resolution != hs[0].Resolution {
			sameResolution = false
		}
	}
	return sameKind || sameResolution
}

func rubricPrompt(worktree string) string {
	prompt := "Grade this three-hunk fixture. Separate the business hunk from the two mechanical controls " +
		"(slice-literal union and rename-versus-edit) and declare ARBITRARY on the business hunk. " +
		"Do not grade proposed_sha. Do not time a human. Emit JSON matching TriagePayload.\n"
	if ev := toyEvidence(worktree); ev != "" {
		prompt += "\nFixture evidence (both sides of " + ToyPath + "):\n" + ev
	}
	return prompt
}

func toyEvidence(worktree string) string {
	if worktree == "" {
		return ""
	}
	refs, err := gitOut(worktree, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil || refs == "" {
		return ""
	}
	var b strings.Builder
	for _, ref := range strings.Split(refs, "\n") {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		body, err := gitOut(worktree, "show", ref+":"+ToyPath)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "\n--- %s ---\n%s\n", ref, body)
	}
	return b.String()
}
