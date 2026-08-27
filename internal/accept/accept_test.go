package accept

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

type verifierFixture struct {
	root, base, candidate string
	ledger                *ledger.Ledger
	verifier              *Verifier
	candidateRow          ledger.LaneCandidate
}

func newVerifierFixture(t *testing.T, resultJSON, checksScript string, changed map[string]string, allowed []string) verifierFixture {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.name", "accept-test")
	git(t, root, "config", "user.email", "accept-test@example.com")
	if checksScript == "" {
		checksScript = "#!/bin/sh\necho checks-ok\n"
	}
	writeFile(t, root, "lucind-checks.sh", checksScript, 0o755)
	writeFile(t, root, "seed.txt", "seed\n", 0o644)
	git(t, root, "add", "lucind-checks.sh", "seed.txt")
	git(t, root, "commit", "-m", "seed")
	base := gitOut(t, root, "rev-parse", "HEAD")
	for path, content := range changed {
		writeFile(t, root, path, content, 0o755)
	}
	git(t, root, "add", "--all")
	git(t, root, "commit", "-m", "candidate")
	candidate := gitOut(t, root, "rev-parse", "HEAD")

	l, err := ledger.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	if err := l.RegisterLane(context.Background(), ledger.Lane{RunID: "run-1", LaneID: "lane-1", PacketID: "lane-1", Executor: "agy", RoutingCondition: "test", Status: lane.Running}); err != nil {
		t.Fatal(err)
	}
	row := ledger.LaneCandidate{
		RunID: "run-1", LaneID: "lane-1", PacketID: "lane-1", PacketDigest: "packet-digest",
		PrimaryRoot: root, WorktreePath: filepath.Join(root+"-worktrees", "lane-1"),
		BaseCommit: base, BaseTree: gitOut(t, root, "rev-parse", base+"^{tree}"),
		CandidateCommit: candidate, CandidateTree: gitOut(t, root, "rev-parse", candidate+"^{tree}"),
		AllowedPaths: allowed, ResultPath: ".lucind/result.json", ResultJSON: resultJSON,
		ResultHash: hashValues("result:v1", resultJSON), RecordedAt: time.Now().UTC(),
	}
	if err := l.SetDoneCandidate(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	return verifierFixture{root: root, base: base, candidate: candidate, ledger: l, verifier: NewVerifier(root, l), candidateRow: row}
}

func validResult(paths ...string) string {
	files := ""
	for i, path := range paths {
		if i > 0 {
			files += ","
		}
		files += `{"path":"` + path + `","change":"modified"}`
	}
	return `{"packet_id":"lane-1","status":"done","summary":"mechanical candidate","hard_stops":[{"hard_stop":"stop","fired":false}],"files_changed":[` + files + `],"done_criteria":[{"criterion":"implemented","met":true}]}`
}

func TestVerifierPersistsCompleteReceiptAndReusesExactBinding(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	refsBefore := gitOut(t, f.root, "show-ref")
	receipt, err := f.verifier.Verify(context.Background(), AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if receipt.ReceiptID == "" || receipt.BindingHash == "" || receipt.ResultHash != f.candidateRow.ResultHash || receipt.Cleanup != "removed" {
		t.Fatalf("incomplete receipt: %+v", receipt)
	}
	if receipt.Binding.CandidateTree != f.candidateRow.CandidateTree || receipt.Binding.PacketDigest != "packet-digest" {
		t.Fatalf("incomplete binding: %+v", receipt.Binding)
	}
	reused, err := f.verifier.Verify(context.Background(), AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"})
	if err != nil || reused != receipt {
		t.Fatalf("exact cache reuse = %+v, %v; want %+v", reused, err, receipt)
	}
	if refsAfter := gitOut(t, f.root, "show-ref"); refsAfter != refsBefore {
		t.Fatalf("acceptance mutated refs\nbefore: %s\nafter: %s", refsBefore, refsAfter)
	}
}

func TestVerifierRejectsInvalidEvidenceWithoutReceipt(t *testing.T) {
	tests := []struct {
		name, result string
		allowed      []string
	}{
		{name: "invalid schema", result: `{`, allowed: []string{"allowed.txt"}},
		{name: "packet mismatch", result: strings.Replace(validResult("allowed.txt"), "lane-1", "other", 1), allowed: []string{"allowed.txt"}},
		{name: "hard stop", result: strings.Replace(validResult("allowed.txt"), `"fired":false`, `"fired":true`, 1), allowed: []string{"allowed.txt"}},
		{name: "unmet criterion", result: strings.Replace(validResult("allowed.txt"), `"met":true`, `"met":false`, 1), allowed: []string{"allowed.txt"}},
		{name: "undeclared change", result: validResult(), allowed: []string{"allowed.txt"}},
		{name: "out of scope", result: validResult("allowed.txt"), allowed: []string{"other.txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVerifierFixture(t, tt.result, "", map[string]string{"allowed.txt": "candidate\n"}, tt.allowed)
			if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"}); err == nil {
				t.Fatal("Verify() error = nil")
			}
			if _, err := f.ledger.FindAcceptanceReceipt(context.Background(), bindingHashForCandidate(t, f)); !errors.Is(err, ledger.ErrAcceptanceReceiptNotFound) {
				t.Fatalf("receipt exists after rejection: %v", err)
			}
		})
	}
}

func TestVerifierTreatsDocumentationLikeFilesAsScopeOnly(t *testing.T) {
	paths := []string{"requirements.txt", "CMakeLists.txt", "guide.md", "guide.mdx", "README.sh"}
	changed := make(map[string]string, len(paths))
	for _, path := range paths {
		changed[path] = "touch SHOULD_NOT_EXIST\n"
	}
	f := newVerifierFixture(t, validResult(paths...), "#!/bin/sh\necho root-check-only\n", changed, paths)
	if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.root, "SHOULD_NOT_EXIST")); !os.IsNotExist(err) {
		t.Fatalf("documentation-like candidate was executed: %v", err)
	}
}

func TestVerifierUsesFrozenDetachedCandidateDespitePrimaryState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, verifierFixture)
	}{
		{name: "staged", mutate: func(t *testing.T, f verifierFixture) {
			writeFile(t, f.root, "primary-only.txt", "dirty\n", 0o644)
			git(t, f.root, "add", "primary-only.txt")
		}},
		{name: "commit-a", mutate: func(t *testing.T, f verifierFixture) {
			writeFile(t, f.root, "seed.txt", "primary changed\n", 0o644)
			git(t, f.root, "commit", "-am", "primary moved")
		}},
		{name: "empty-index", mutate: func(t *testing.T, f verifierFixture) { git(t, f.root, "read-tree", "--empty") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\ntest ! -e primary-only.txt\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
			tt.mutate(t, f)
			if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err != nil {
				t.Fatalf("Verify() observed primary state: %v", err)
			}
		})
	}
}

func TestVerifierBindingDifferencePreventsCacheReuse(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	checks := 0
	f.verifier.check = func(ctx context.Context, path string) (bool, string, error) {
		checks++
		return true, "ok", nil
	}
	first, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LANG", "acceptance-cache-difference")
	second, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"})
	if err != nil {
		t.Fatal(err)
	}
	if checks != 2 || first.ReceiptID == second.ReceiptID || first.Binding.EnvironmentHash == second.Binding.EnvironmentHash {
		t.Fatalf("binding difference reused cache: checks=%d first=%+v second=%+v", checks, first, second)
	}
}

func TestVerifierRejectsRootOrObjectIdentityMismatch(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	tests := []struct {
		name string
		edit func(*Verifier, *ledger.LaneCandidate)
	}{
		{name: "relative root", edit: func(v *Verifier, _ *ledger.LaneCandidate) { v.primaryRoot = "relative" }},
		{name: "foreign root", edit: func(_ *Verifier, c *ledger.LaneCandidate) { c.PrimaryRoot = t.TempDir() }},
		{name: "candidate tree mismatch", edit: func(_ *Verifier, c *ledger.LaneCandidate) { c.CandidateTree = c.BaseTree }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := *f.verifier
			row := f.candidateRow
			tt.edit(&v, &row)
			v.loadCandidate = func(context.Context, string, string) (ledger.LaneCandidate, error) { return row, nil }
			if _, err := v.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil {
				t.Fatal("Verify() error = nil")
			}
		})
	}
}

func TestVerifierCheckFailureAndForeignIsolationPersistNoReceipt(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\nexit 7\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil {
		t.Fatal("exit 7 accepted")
	}

	f2 := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	f2.verifier.newID = func() string { return "fixed" }
	foreign := f2.verifier.isolationPath("lane-1", "fixed")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, foreign, "foreign.txt", "preserve\n", 0o644)
	if _, err := f2.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil {
		t.Fatal("foreign isolation accepted")
	}
	if data, err := os.ReadFile(filepath.Join(foreign, "foreign.txt")); err != nil || string(data) != "preserve\n" {
		t.Fatalf("foreign isolation changed: %q, %v", data, err)
	}
}

func TestVerifierCleanupMarkerMismatchRejectsAndPreservesIsolation(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	f.verifier.newID = func() string { return "cleanup-mismatch" }
	f.verifier.check = func(_ context.Context, path string) (bool, string, error) {
		if err := os.WriteFile(filepath.Join(path, ownerMarkerName), []byte(`{"Token":"foreign"}`), 0o600); err != nil {
			return false, "", err
		}
		return true, "ok", nil
	}
	if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("Verify() error = %v", err)
	}
	isolation := f.verifier.isolationPath("lane-1", "cleanup-mismatch")
	if _, err := os.Stat(isolation); err != nil {
		t.Fatalf("mismatched isolation was removed: %v", err)
	}
	if _, err := f.ledger.FindAcceptanceReceipt(context.Background(), bindingHashForCandidate(t, f)); !errors.Is(err, ledger.ErrAcceptanceReceiptNotFound) {
		t.Fatalf("receipt persisted despite cleanup failure: %v", err)
	}
}

func bindingHashForCandidate(t *testing.T, f verifierFixture) string {
	t.Helper()
	binding, err := f.verifier.binding(f.candidateRow)
	if err != nil {
		return "not-created"
	}
	return bindingHash(binding)
}

func git(t *testing.T, dir string, args ...string) { t.Helper(); _ = gitOut(t, dir, args...) }
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func writeFile(t *testing.T, root, path, content string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
