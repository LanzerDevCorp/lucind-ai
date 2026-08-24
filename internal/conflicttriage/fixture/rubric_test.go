package fixture_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage"
	"github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage/fixture"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

func writeJudgeStub(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
	return path
}

func distinctPayloadJSON(t *testing.T) string {
	t.Helper()
	p := conflicttriage.TriagePayload{
		CauseSummary: "three-hunk fixture",
		HunkDecisions: []conflicttriage.HunkDecision{
			{HunkID: "hunk-business", Kind: conflicttriage.HunkKindBusiness, Resolution: conflicttriage.ResolutionArbitrary, Rationale: "no technical criterion"},
			{HunkID: "hunk-slice", Kind: conflicttriage.HunkKindSliceUnion, Resolution: "union", Rationale: "union both literals"},
			{HunkID: "hunk-rename", Kind: conflicttriage.HunkKindRenameVsEdit, Resolution: "keep-rename", Rationale: "rename is mechanical"},
		},
		RiskBand:     conflicttriage.RiskHigh,
		VerifyBudget: conflicttriage.VerifyBudgetExample,
		ProposedSHA:  "must-not-be-graded",
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func uniformPayloadJSON(t *testing.T) string {
	t.Helper()
	p := conflicttriage.TriagePayload{
		CauseSummary: "uniform scoring",
		HunkDecisions: []conflicttriage.HunkDecision{
			{HunkID: "hunk-business", Kind: conflicttriage.HunkKindBusiness, Resolution: conflicttriage.ResolutionArbitrary, Rationale: "same"},
			{HunkID: "hunk-slice", Kind: conflicttriage.HunkKindBusiness, Resolution: conflicttriage.ResolutionArbitrary, Rationale: "same"},
			{HunkID: "hunk-rename", Kind: conflicttriage.HunkKindBusiness, Resolution: conflicttriage.ResolutionArbitrary, Rationale: "same"},
		},
		RiskBand:     conflicttriage.RiskHigh,
		VerifyBudget: conflicttriage.VerifyBudgetExample,
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRubric_GradesDistinctThreeHunkClassification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess stub test in -short mode")
	}
	ctx := context.Background()
	payload := distinctPayloadJSON(t)
	claudeArgv := filepath.Join(t.TempDir(), "claude-argv.txt")
	opencodeArgv := filepath.Join(t.TempDir(), "opencode-argv.txt")

	claudeStub := writeJudgeStub(t, "claude-stub.sh", "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \""+claudeArgv+"\"; done\nprintf '%s\\n' '"+payload+"'\nexit 0\n")
	opencodeStub := writeJudgeStub(t, "opencode-stub.sh", "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \""+opencodeArgv+"\"; done\nprintf '%s\\n' '"+payload+"'\nexit 0\n")

	got, err := fixture.EvaluateRubric(ctx, fixture.RubricOptions{
		Claude:       executor.Claude{Binary: claudeStub, WaitDelay: 50 * time.Millisecond},
		Opencode:     executor.Opencode{Binary: opencodeStub, WaitDelay: 50 * time.Millisecond},
		WorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("EvaluateRubric: %v", err)
	}
	if !got.Passed {
		t.Fatalf("Passed = false, want true for distinct three-hunk classification; reason=%q", got.Reason)
	}

	claudeArgs, err := os.ReadFile(claudeArgv)
	if err != nil {
		t.Fatalf("claude argv: %v", err)
	}
	if !strings.Contains(string(claudeArgs), fixture.JudgeClaudeModel) {
		t.Errorf("claude argv = %q, want model %q", claudeArgs, fixture.JudgeClaudeModel)
	}
	if strings.Contains(string(claudeArgs), "openai/") || strings.Contains(string(claudeArgs), "gemini-") || strings.Contains(string(claudeArgs), "cursor-") {
		t.Errorf("claude argv leaked another provider: %q", claudeArgs)
	}

	opencodeArgs, err := os.ReadFile(opencodeArgv)
	if err != nil {
		t.Fatalf("opencode argv: %v", err)
	}
	if !strings.Contains(string(opencodeArgs), fixture.JudgeOpencodeModel) {
		t.Errorf("opencode argv = %q, want model %q", opencodeArgs, fixture.JudgeOpencodeModel)
	}
	if strings.Contains(string(opencodeArgs), "claude-opus") || strings.Contains(string(opencodeArgs), "gemini-") || strings.Contains(string(opencodeArgs), "cursor-") {
		t.Errorf("opencode argv leaked another provider: %q", opencodeArgs)
	}
}

func TestRubric_RejectsUniformHunkScoring(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess stub test in -short mode")
	}
	ctx := context.Background()
	payload := uniformPayloadJSON(t)
	stubScript := "#!/bin/sh\nprintf '%s\\n' '" + payload + "'\nexit 0\n"
	claudeStub := writeJudgeStub(t, "claude-stub.sh", stubScript)
	opencodeStub := writeJudgeStub(t, "opencode-stub.sh", stubScript)

	got, err := fixture.EvaluateRubric(ctx, fixture.RubricOptions{
		Claude:       executor.Claude{Binary: claudeStub, WaitDelay: 50 * time.Millisecond},
		Opencode:     executor.Opencode{Binary: opencodeStub, WaitDelay: 50 * time.Millisecond},
		WorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("EvaluateRubric: %v", err)
	}
	if got.Passed {
		t.Fatalf("Passed = true, want false for uniform hunk scoring")
	}
}

func TestRubric_PresentsGenerateFixtureEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess stub test in -short mode")
	}
	ctx := context.Background()
	l := openFixtureLedger(t)
	repo := t.TempDir()
	if _, err := fixture.GenerateFixture(ctx, fixture.GeneratorOptions{
		RepoRoot:   repo,
		Ledger:     l,
		FeatureAID: "feat-conflict-a",
		FeatureBID: "feat-conflict-b",
		ParentRefA: "refs/heads/feature-conflict-a",
		ParentRefB: "refs/heads/feature-conflict-b",
		SharedBase: true,
	}); err != nil {
		t.Fatalf("GenerateFixture: %v", err)
	}

	payload := distinctPayloadJSON(t)
	claudeArgv := filepath.Join(t.TempDir(), "claude-argv.txt")
	opencodeArgv := filepath.Join(t.TempDir(), "opencode-argv.txt")
	claudeStub := writeJudgeStub(t, "claude-stub.sh", "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \""+claudeArgv+"\"; done\nprintf '%s\\n' '"+payload+"'\nexit 0\n")
	opencodeStub := writeJudgeStub(t, "opencode-stub.sh", "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \""+opencodeArgv+"\"; done\nprintf '%s\\n' '"+payload+"'\nexit 0\n")

	if _, err := fixture.EvaluateRubric(ctx, fixture.RubricOptions{
		Claude:       executor.Claude{Binary: claudeStub, WaitDelay: 50 * time.Millisecond},
		Opencode:     executor.Opencode{Binary: opencodeStub, WaitDelay: 50 * time.Millisecond},
		WorktreePath: repo,
	}); err != nil {
		t.Fatalf("EvaluateRubric: %v", err)
	}

	claudeArgs, err := os.ReadFile(claudeArgv)
	if err != nil {
		t.Fatalf("claude argv: %v", err)
	}
	opencodeArgs, err := os.ReadFile(opencodeArgv)
	if err != nil {
		t.Fatalf("opencode argv: %v", err)
	}
	for _, mark := range []string{"tier A", "enterprise", "from-a", "from-b", "HelperRenamed", "edited-by-b"} {
		if !strings.Contains(string(claudeArgs), mark) {
			t.Errorf("claude prompt missing fixture evidence %q; argv=%q", mark, claudeArgs)
		}
		if !strings.Contains(string(opencodeArgs), mark) {
			t.Errorf("opencode prompt missing fixture evidence %q; argv=%q", mark, opencodeArgs)
		}
	}
}
