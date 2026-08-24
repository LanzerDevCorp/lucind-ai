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

func TestRubric_ParsesPayloadAmongConcatenatedJSONObjects(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess stub test in -short mode")
	}
	ctx := context.Background()
	payload := distinctPayloadJSON(t)

	// Reproduces the real opencode CLI (v1.18.21, "run --format json"):
	// several back-to-back top-level JSON objects on stdout with no
	// separator, rather than a single clean document. Naive first-brace-to-
	// last-brace slicing would span all three and fail to parse.
	noiseBefore := `{"type":"event","status":"init"}`
	noiseAfter := `{"type":"event","status":"done"}`
	stubScript := "#!/bin/sh\nprintf '%s' '" + noiseBefore + payload + noiseAfter + "'\nexit 0\n"
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
	if !got.Passed {
		t.Fatalf("Passed = false, want true when the real payload is concatenated with noise objects; reason=%q", got.Reason)
	}
}

func TestRubric_ParsesPayloadNestedInResultEnvelope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess stub test in -short mode")
	}
	ctx := context.Background()
	payload := distinctPayloadJSON(t)

	// Reproduces the real claude CLI (2.1.241, "--print --output-format
	// json"): stdout is one top-level result envelope (is_error, usage,
	// session_id, ...) whose "result" field is a *string* containing the
	// model's reply -- a markdown-fenced ```json code block wrapping the
	// real TriagePayload, followed by prose. The real answer is nested
	// inside a string value, not present as a top-level field.
	envelope := map[string]any{
		"type":     "result",
		"is_error": false,
		"result":   "```json\n" + payload + "\n```\n\nThree hunks, three distinct kinds, three distinct resolutions.",
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	stubScript := "#!/bin/sh\nprintf '%s' " + shellQuote(string(raw)) + "\nexit 0\n"
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
	if !got.Passed {
		t.Fatalf("Passed = false, want true when the real payload is nested in a result envelope's string field; reason=%q", got.Reason)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
