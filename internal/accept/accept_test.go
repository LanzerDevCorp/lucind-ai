package accept

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
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

func TestAcceptDirtyPrimaryWithSkillsMatch(t *testing.T) {
	resultJSON := `{"packet_id":"lane-skills-match","status":"done","summary":"done","hard_stops":[{"hard_stop":"stop","fired":false}],"files_changed":[{"path":"allowed.txt","change":"created"}],"done_criteria":[{"criterion":"implemented","met":true}],"skills_loaded":["lucind-executor","lucind-apply"],"commit":"CANDIDATE"}`
	f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\ntest ! -e primary-only.txt\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	resultJSON = strings.Replace(resultJSON, "CANDIDATE", f.candidate, 1)

	contract := `{"version":"packet-author/v1","mode":"write","required_skills":["lucind-executor","lucind-apply"],"write_paths":["allowed.txt"],"read_only_paths":null,"done_criteria":["implemented"],"hard_stops":["stop"],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}`
	bindingJSON := `{"kind":"feature","feature":"feat-test","parent_ref":"refs/heads/feature-1","base_sha":"` + f.base + `","expected_parent_sha":"` + f.base + `"}`
	e := ledger.AuthoringEvidence{
		PacketDigest:     "packet-digest-skills",
		AuthoringMode:    "versioned",
		ContractVersion:  "packet-author/v1",
		Contract:         json.RawMessage(contract),
		Binding:          json.RawMessage(bindingJSON),
		Mode:             "write",
		CommitObligation: "required",
		WritePaths:       []string{"allowed.txt"},
		DoneCriteria:     []string{"implemented"},
		HardStops:        []string{"stop"},
		ResultPath:       ".lucind/result.json",
		ResultSchema:     ".lucind/result.schema.json",
		BaseCommit:       f.base,
		BaseTree:         f.candidateRow.BaseTree,
		CandidateCommit:  f.candidate,
		CandidateTree:    f.candidateRow.CandidateTree,
		Changes:          []candidatechange.Change{{Change: candidatechange.Created, Path: "allowed.txt"}},
		ResultHash:       hashValues("result:v1", resultJSON),
	}
	encoded, hash, err := ledger.FreezeAuthoringEvidence(e)
	if err != nil {
		t.Fatal(err)
	}
	matchRow := ledger.LaneCandidate{
		RunID:                    "run-1",
		LaneID:                   "lane-skills-match",
		PacketID:                 "lane-skills-match",
		PacketDigest:             "packet-digest-skills",
		PrimaryRoot:              f.root,
		WorktreePath:             filepath.Join(f.root+"-worktrees", "lane-skills-match"),
		BaseCommit:               f.base,
		BaseTree:                 f.candidateRow.BaseTree,
		CandidateCommit:          f.candidate,
		CandidateTree:            f.candidateRow.CandidateTree,
		AllowedPaths:             []string{"allowed.txt"},
		ResultPath:               ".lucind/result.json",
		ResultJSON:               resultJSON,
		ResultHash:               hashValues("result:v1", resultJSON),
		AuthoringEvidenceVersion: ledger.AuthoringEvidenceVersion,
		AuthoringEvidenceJSON:    encoded,
		AuthoringEvidenceHash:    hash,
		RecordedAt:               time.Now().UTC(),
	}
	if err := f.ledger.RegisterLane(context.Background(), ledger.Lane{RunID: "run-1", LaneID: "lane-skills-match", PacketID: "lane-skills-match", Executor: "agy", RoutingCondition: "test", Status: lane.Running}); err != nil {
		t.Fatal(err)
	}
	if err := f.ledger.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-skills-match", Feature: "feat-test", ParentRef: "refs/heads/feature-1", BaseSHA: f.base, ExpectedParentSHA: f.base}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := f.ledger.SetDoneCandidate(context.Background(), matchRow); err != nil {
		t.Fatal(err)
	}

	// Make primary repository dirty (staged uncommitted change on primary)
	writeFile(t, f.root, "primary-only.txt", "dirty on primary\n", 0o644)
	git(t, f.root, "add", "primary-only.txt")

	// Case 1: Matching skills on dirty primary succeeds
	receipt, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-skills-match"})
	if err != nil {
		t.Fatalf("Verify() on dirty primary with matching skills failed: %v", err)
	}
	if receipt.ReceiptID == "" {
		t.Fatalf("expected receipt, got empty")
	}

	// Case 2: Skills mismatch (shortfall) rejects candidate and creates no receipt
	mismatchResultJSON := strings.Replace(resultJSON, `"packet_id":"lane-skills-match"`, `"packet_id":"lane-mismatch"`, 1)
	mismatchResultJSON = strings.Replace(mismatchResultJSON, `"skills_loaded":["lucind-executor","lucind-apply"]`, `"skills_loaded":["lucind-executor"]`, 1)
	mismatchRow := matchRow
	mismatchRow.LaneID = "lane-mismatch"
	mismatchRow.PacketID = "lane-mismatch"
	mismatchRow.ResultJSON = mismatchResultJSON
	mismatchRow.ResultHash = hashValues("result:v1", mismatchResultJSON)
	mismatchEvidence := e
	mismatchEvidence.ResultHash = mismatchRow.ResultHash
	mismatchEncoded, mismatchHash, err := ledger.FreezeAuthoringEvidence(mismatchEvidence)
	if err != nil {
		t.Fatal(err)
	}
	mismatchRow.AuthoringEvidenceJSON = mismatchEncoded
	mismatchRow.AuthoringEvidenceHash = mismatchHash
	if err := f.ledger.RegisterLane(context.Background(), ledger.Lane{RunID: "run-1", LaneID: "lane-mismatch", PacketID: "lane-mismatch", Executor: "agy", RoutingCondition: "test", Status: lane.Running}); err != nil {
		t.Fatal(err)
	}
	if err := f.ledger.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-mismatch", Feature: "feat-test", ParentRef: "refs/heads/feature-1", BaseSHA: f.base, ExpectedParentSHA: f.base}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := f.ledger.SetDoneCandidate(context.Background(), mismatchRow); err != nil {
		t.Fatal(err)
	}

	if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-mismatch"}); err == nil {
		t.Fatal("Verify() with skills mismatch unexpectedly succeeded")
	}
	if _, err := f.ledger.FindAcceptanceReceipt(context.Background(), bindingHashForCandidate(t, verifierFixture{verifier: f.verifier, candidateRow: mismatchRow})); !errors.Is(err, ledger.ErrAcceptanceReceiptNotFound) {
		t.Fatalf("receipt persisted for rejected candidate: %v", err)
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

func TestVerifierSkipsChecksForNonWritingLanes(t *testing.T) {
	tests := []struct {
		name     string
		metadata ledger.LaneMetadata
	}{
		{
			name:     "lens lane role skips checks",
			metadata: ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", LaneRole: "lens"},
		},
		{
			name:     "read-only true skips checks",
			metadata: ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", LaneRole: "apply", ReadOnly: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\nexit 7\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
			if err := f.ledger.UpdateLaneMetadata(context.Background(), tt.metadata, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err != nil {
				t.Fatalf("Verify() for %s should skip the failing checks script: %v", tt.name, err)
			}
		})
	}
}

func TestVerifierRunsChecksForWritingOrUnspecifiedLanes(t *testing.T) {
	tests := []struct {
		name        string
		setMetadata bool
		metadata    ledger.LaneMetadata
	}{
		{
			name:        "declared apply role",
			setMetadata: true,
			metadata:    ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", LaneRole: "apply"},
		},
		{
			name:        "explicit empty role",
			setMetadata: true,
			metadata:    ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", LaneRole: ""},
		},
		{
			name:        "missing lane metadata",
			setMetadata: false,
		},
		{
			name:        "legacy sdd_phase explore without role fails closed and runs checks",
			setMetadata: true,
			metadata:    ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", SDDPhase: "explore"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\nexit 7\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
			if tt.setMetadata {
				if err := f.ledger.UpdateLaneMetadata(context.Background(), tt.metadata, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil {
				t.Fatalf("Verify() should still run checks and reject a failing script for %s", tt.name)
			}
		})
	}
}

func TestVerifierNonWritingLaneStillEnforcesScope(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\necho checks-ok\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"other.txt"})
	if err := f.ledger.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", LaneRole: "lens"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil {
		t.Fatal("Verify() with a non-writing lane role still accepted an out-of-scope change")
	}
}

func TestVerifierAttestationReuse(t *testing.T) {
	ctx := context.Background()

	t.Run("valid attestation skips check and persists attested receipt", func(t *testing.T) {
		f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
		checkCalled := false
		f.verifier.check = func(ctx context.Context, dir string) (bool, string, error) {
			checkCalled = true
			return true, "checks-ok", nil
		}
		f.verifier.hasAttestation = func(ctx context.Context, root, command, treeHash string) (bool, error) {
			if command != "sh lucind-checks.sh" {
				t.Errorf("expected command 'sh lucind-checks.sh', got %q", command)
			}
			if treeHash != f.candidateRow.CandidateTree {
				t.Errorf("expected treeHash %q, got %q", f.candidateRow.CandidateTree, treeHash)
			}
			return true, nil
		}

		receipt, err := f.verifier.Verify(ctx, AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"})
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if checkCalled {
			t.Fatalf("expected v.check NOT to be called when valid attestation exists")
		}
		if receipt.ReceiptID == "" || receipt.Cleanup != "removed" {
			t.Fatalf("unexpected receipt: %+v", receipt)
		}
		expectedChecksHash := hashValues("checks:v1", "attest:v1", "attested:"+f.candidateRow.CandidateTree)
		if receipt.ChecksHash != expectedChecksHash {
			t.Fatalf("expected ChecksHash %q, got %q", expectedChecksHash, receipt.ChecksHash)
		}
	})

	t.Run("no attestation runs checks as fallback", func(t *testing.T) {
		f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
		checkCalled := false
		f.verifier.check = func(ctx context.Context, dir string) (bool, string, error) {
			checkCalled = true
			return true, "checks-ok", nil
		}
		f.verifier.hasAttestation = func(ctx context.Context, root, command, treeHash string) (bool, error) {
			return false, nil
		}

		receipt, err := f.verifier.Verify(ctx, AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"})
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if !checkCalled {
			t.Fatalf("expected v.check to be called when no attestation exists")
		}
		attestedChecksHash := hashValues("checks:v1", "attest:v1", "attested:"+f.candidateRow.CandidateTree)
		if receipt.ChecksHash == attestedChecksHash {
			t.Fatalf("ChecksHash must differ between attested and fallback check runs")
		}
	})

	t.Run("attestation lookup error falls back to running checks", func(t *testing.T) {
		f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
		checkCalled := false
		f.verifier.check = func(ctx context.Context, dir string) (bool, string, error) {
			checkCalled = true
			return true, "checks-ok", nil
		}
		f.verifier.hasAttestation = func(ctx context.Context, root, command, treeHash string) (bool, error) {
			return false, errors.New("simulated lookup error")
		}

		_, err := f.verifier.Verify(ctx, AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"})
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if !checkCalled {
			t.Fatalf("expected v.check to be called on lookup error")
		}
	})

	t.Run("attestation lookup error fails closed when fallback checks fail", func(t *testing.T) {
		f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
		checkCalled := false
		f.verifier.check = func(ctx context.Context, dir string) (bool, string, error) {
			checkCalled = true
			return false, "tests failed", nil
		}
		f.verifier.hasAttestation = func(ctx context.Context, root, command, treeHash string) (bool, error) {
			return false, errors.New("simulated lookup error")
		}

		_, err := f.verifier.Verify(ctx, AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"})
		if err == nil {
			t.Fatalf("expected Verify() to fail when checks fail")
		}
		if !checkCalled {
			t.Fatalf("expected v.check to be called")
		}
	})

	t.Run("default wiring reads real attestation from state dir", func(t *testing.T) {
		configDir := t.TempDir()
		stateDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configDir)
		t.Setenv("XDG_STATE_HOME", stateDir)

		f := newVerifierFixture(t, validResult("allowed.txt"), "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
		checkCalled := false
		f.verifier.check = func(ctx context.Context, dir string) (bool, string, error) {
			checkCalled = true
			return true, "checks-ok", nil
		}

		key, err := attest.LoadOrCreateKey("")
		if err != nil {
			t.Fatalf("LoadOrCreateKey failed: %v", err)
		}
		commonDir, err := attest.RepoCommonDir(ctx, f.root)
		if err != nil {
			t.Fatalf("RepoCommonDir failed: %v", err)
		}
		repoID := attest.RepoID(commonDir)
		logDir, err := attest.ResolveStateDir(repoID)
		if err != nil {
			t.Fatalf("ResolveStateDir failed: %v", err)
		}
		entry := attest.Entry{
			Version:    1,
			RepoID:     repoID,
			Command:    "sh lucind-checks.sh",
			ExitCode:   0,
			TreeHash:   f.candidateRow.CandidateTree,
			StartedAt:  time.Now().Add(-1 * time.Second).Format(time.RFC3339Nano),
			FinishedAt: time.Now().Format(time.RFC3339Nano),
		}
		entry.MAC = attest.ComputeMAC(entry, key)
		if _, err := attest.WriteEntry(logDir, entry); err != nil {
			t.Fatalf("WriteEntry failed: %v", err)
		}

		receipt, err := f.verifier.Verify(ctx, AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"})
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if checkCalled {
			t.Fatalf("expected v.check NOT to be called when real on-disk attestation exists")
		}
		expectedChecksHash := hashValues("checks:v1", "attest:v1", "attested:"+f.candidateRow.CandidateTree)
		if receipt.ChecksHash != expectedChecksHash {
			t.Fatalf("expected ChecksHash %q, got %q", expectedChecksHash, receipt.ChecksHash)
		}
	})
}

func TestAcceptDispatcherCommitObligation(t *testing.T) {
	contract := `{"version":"packet-author/v1","mode":"write","commit_message":"feat: add allowed","verification":["sh lucind-checks.sh"],"write_paths":["allowed.txt"],"done_criteria":["implemented"],"hard_stops":["stop"],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}`

	setupCandidate := func(t *testing.T, envelopeCommit string, candidateCommitEqualBase bool, mode string) (*Verifier, AcceptanceRequest) {
		t.Helper()
		f := newVerifierFixture(t, "", "", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
		candidateSHA := f.candidate
		candidateTree := f.candidateRow.CandidateTree
		changes := []candidatechange.Change{{Change: candidatechange.Created, Path: "allowed.txt"}}
		filesJSON := `[{"path":"allowed.txt","change":"created"}]`
		if candidateCommitEqualBase {
			candidateSHA = f.base
			candidateTree = f.candidateRow.BaseTree
			changes = []candidatechange.Change{}
			filesJSON = `[]`
		}
		if envelopeCommit == "@candidate" {
			envelopeCommit = candidateSHA
		}
		resultJSON := `{"packet_id":"lane-disp","status":"done","summary":"done","hard_stops":[{"hard_stop":"stop","fired":false}],"files_changed":` + filesJSON + `,"done_criteria":[{"criterion":"implemented","met":true}],"commit":"` + envelopeCommit + `"}`
		bindingJSON := `{"kind":"feature","feature":"feat-test","parent_ref":"refs/heads/feature-1","base_sha":"` + f.base + `","expected_parent_sha":"` + f.base + `"}`
		e := ledger.AuthoringEvidence{
			PacketDigest:     "packet-digest-disp",
			AuthoringMode:    "versioned",
			ContractVersion:  "packet-author/v1",
			Contract:         json.RawMessage(contract),
			Binding:          json.RawMessage(bindingJSON),
			Mode:             mode,
			CommitObligation: "dispatcher",
			WritePaths:       []string{"allowed.txt"},
			DoneCriteria:     []string{"implemented"},
			HardStops:        []string{"stop"},
			ResultPath:       ".lucind/result.json",
			ResultSchema:     ".lucind/result.schema.json",
			BaseCommit:       f.base,
			BaseTree:         f.candidateRow.BaseTree,
			CandidateCommit:  candidateSHA,
			CandidateTree:    candidateTree,
			Changes:          changes,
			ResultHash:       hashValues("result:v1", resultJSON),
		}
		encoded, hash, err := ledger.FreezeAuthoringEvidence(e)
		if err != nil {
			t.Fatal(err)
		}
		row := ledger.LaneCandidate{
			RunID:                    "run-1",
			LaneID:                   "lane-disp",
			PacketID:                 "lane-disp",
			PacketDigest:             "packet-digest-disp",
			PrimaryRoot:              f.root,
			WorktreePath:             filepath.Join(f.root+"-worktrees", "lane-disp"),
			BaseCommit:               f.base,
			BaseTree:                 f.candidateRow.BaseTree,
			CandidateCommit:          candidateSHA,
			CandidateTree:            candidateTree,
			AllowedPaths:             []string{"allowed.txt"},
			ResultPath:               ".lucind/result.json",
			ResultJSON:               resultJSON,
			ResultHash:               hashValues("result:v1", resultJSON),
			AuthoringEvidenceVersion: ledger.AuthoringEvidenceVersion,
			AuthoringEvidenceJSON:    encoded,
			AuthoringEvidenceHash:    hash,
			RecordedAt:               time.Now().UTC(),
		}
		if err := f.ledger.RegisterLane(context.Background(), ledger.Lane{RunID: "run-1", LaneID: "lane-disp", PacketID: "lane-disp", Executor: "agy", RoutingCondition: "test", Status: lane.Running}); err != nil {
			t.Fatal(err)
		}
		if err := f.ledger.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-disp", Feature: "feat-test", ParentRef: "refs/heads/feature-1", BaseSHA: f.base, ExpectedParentSHA: f.base}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if err := f.ledger.SetDoneCandidate(context.Background(), row); err != nil {
			t.Fatal(err)
		}
		return f.verifier, AcceptanceRequest{"run-1", "lane-disp"}
	}

	t.Run("accepted when envelope commit is empty and candidate != base", func(t *testing.T) {
		v, req := setupCandidate(t, "", false, "write")
		_, err := v.Verify(context.Background(), req)
		if err != nil {
			t.Fatalf("expected verification to succeed, got %v", err)
		}
	})

	t.Run("rejected when envelope commit equals candidate (worker must not commit)", func(t *testing.T) {
		v, req := setupCandidate(t, "@candidate", false, "write")
		_, err := v.Verify(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), "write commit mismatch") {
			t.Fatalf("expected write commit mismatch error, got %v", err)
		}
	})

	t.Run("rejected when candidate == base", func(t *testing.T) {
		v, req := setupCandidate(t, "", true, "write")
		_, err := v.Verify(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), "write commit mismatch") {
			t.Fatalf("expected write commit mismatch error, got %v", err)
		}
	})

	t.Run("rejected when envelope commit does not match candidate", func(t *testing.T) {
		v, req := setupCandidate(t, "wrong-commit-sha", false, "write")
		_, err := v.Verify(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), "write commit mismatch") {
			t.Fatalf("expected write commit mismatch error, got %v", err)
		}
	})

	t.Run("rejected when mode is not write", func(t *testing.T) {
		v, req := setupCandidate(t, "", false, "read-only")
		_, err := v.Verify(context.Background(), req)
		if err == nil {
			t.Fatal("expected error when mode is read-only for dispatcher commit obligation")
		}
	})
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
