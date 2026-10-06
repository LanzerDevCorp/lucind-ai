package accept

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

// fakeTrees is a Trees implementation that needs no git repository.
type fakeTrees struct {
	finalTree string
	changed   []string
	err       error
}

func (f fakeTrees) TreeHash(context.Context, string) (string, error) {
	return f.finalTree, nil
}

func (f fakeTrees) ChangedFiles(context.Context, string, string, string) ([]string, error) {
	return f.changed, f.err
}

// newFakeLane saves a lane under a plain temp dir and writes a result for its current turn.
func newFakeLane(t *testing.T, allow []string, resultStatus string) (string, lane.Lane) {
	t.Helper()
	root := t.TempDir()
	l := lane.Lane{
		ID:       "20261003-120000-abcd",
		BaseTree: "base-tree",
		Allow:    allow,
		Status:   lane.StatusRunning,
		Turn:     1,
	}
	if err := lane.Save(root, l); err != nil {
		t.Fatalf("save lane: %v", err)
	}
	body := fmt.Sprintf(`{"lane_id": %q, "status": %q, "summary": "done.", "hard_stops": []}`, l.ID, resultStatus)
	resPath := lane.ResultFilePath(root, l)
	if err := os.MkdirAll(filepath.Dir(resPath), 0o755); err != nil {
		t.Fatalf("mkdir lane dir: %v", err)
	}
	if err := os.WriteFile(resPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write result: %v", err)
	}
	return root, l
}

func TestAcceptWithTrees_RejectsFileOutsideAllowlist(t *testing.T) {
	root, l := newFakeLane(t, []string{"src/**"}, "done")
	trees := fakeTrees{finalTree: "final-tree", changed: []string{"src/a.go", "docs/readme.md"}}

	verdict, receipt, reasons, err := acceptWithTrees(context.Background(), root, l.ID, trees)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictRejected {
		t.Fatalf("verdict = %q, want %q", verdict, lane.VerdictRejected)
	}
	want := "changed file docs/readme.md not in allowlist"
	if !slices.Contains(reasons, want) {
		t.Fatalf("reasons = %v, want %q", reasons, want)
	}
	if len(reasons) != 1 {
		t.Fatalf("reasons = %v, want exactly one", reasons)
	}
	if receipt.FinalTree != "final-tree" || receipt.BaseTree != "base-tree" {
		t.Fatalf("receipt trees = %s..%s", receipt.BaseTree, receipt.FinalTree)
	}
}

func TestAcceptWithTrees_IgnoresLucindStateFiles(t *testing.T) {
	root, l := newFakeLane(t, []string{"src/**"}, "done")
	trees := fakeTrees{finalTree: "final-tree", changed: []string{".lucind/lanes/x/lane.json", ".lucind", "src/a.go"}}

	verdict, _, reasons, err := acceptWithTrees(context.Background(), root, l.ID, trees)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q, reasons = %v, want accepted", verdict, reasons)
	}
}

func TestAcceptWithTrees_AcceptsInScopeDoneWithoutChecks(t *testing.T) {
	root, l := newFakeLane(t, []string{"src/**", "cmd/**"}, "done")
	trees := fakeTrees{finalTree: "final-tree", changed: []string{"src/a.go", "cmd/main.go"}}

	verdict, receipt, reasons, err := acceptWithTrees(context.Background(), root, l.ID, trees)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted || len(reasons) != 0 {
		t.Fatalf("verdict = %q, reasons = %v, want accepted", verdict, reasons)
	}
	if !slices.Equal(receipt.ChangedFiles, []string{"src/a.go", "cmd/main.go"}) {
		t.Fatalf("receipt.ChangedFiles = %v", receipt.ChangedFiles)
	}
	saved, err := lane.Load(root, l.ID)
	if err != nil {
		t.Fatalf("load lane: %v", err)
	}
	if saved.Status != lane.StatusAccepted {
		t.Fatalf("lane status = %q, want %q", saved.Status, lane.StatusAccepted)
	}
}

func TestAcceptWithTrees_PropagatesChangedFilesError(t *testing.T) {
	root, l := newFakeLane(t, []string{"src/**"}, "done")
	trees := fakeTrees{finalTree: "final-tree", err: fmt.Errorf("boom")}

	_, _, _, err := acceptWithTrees(context.Background(), root, l.ID, trees)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want boom", err)
	}
}
