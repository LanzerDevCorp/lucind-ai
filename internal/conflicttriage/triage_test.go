package conflicttriage_test

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

	"github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/resolve"
)

func openTriageLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func sampleTriageEvidence() *overlap.Evidence {
	now := time.Date(2026, 8, 23, 18, 0, 0, 0, time.UTC)
	ev := &overlap.Evidence{
		Version:     "v1",
		BaseSHA:     "base111111111111111111111111111111111111",
		FeatureASHA: "src222222222222222222222222222222222222",
		FeatureBSHA: "tgt333333333333333333333333333333333333",
		Class:       overlap.ClassRequired,
		Rationale:   []string{"predicted Git merge conflict detected by merge-tree"},
		Signals: overlap.Signals{
			PredictedConflict: true,
			ConflictPaths:     []string{"toy.go"},
			SharedPaths:       []string{"toy.go"},
		},
		Thresholds: overlap.DefaultThresholds(),
		CreatedAt:  now,
	}
	hash, _ := ev.ComputeHash()
	ev.Hash = hash
	return ev
}

func approveRunningCandidate(t *testing.T, svc *reconcile.Service, candidateID string) reconcile.Candidate {
	t.Helper()
	ctx := context.Background()
	ev := sampleTriageEvidence()
	req, err := svc.CreateRequest(ctx, reconcile.CreateRequestParams{
		ID:            "req-" + candidateID,
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	_, cand, err := svc.Approve(ctx, reconcile.ApproveParams{
		RequestID:     req.ID,
		SourceFeature: "feature-source",
		TargetFeature: "feature-target",
		Actor:         "local:testuser",
		AllowedPaths:  []string{"toy.go"},
		CandidateID:   candidateID,
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	return cand
}

func initTriageRepo(t *testing.T) (repo, baseSHA string) {
	t.Helper()
	repo = t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "triage-test@example.com")
	runGit(t, repo, "config", "user.name", "triage-test")
	if err := os.WriteFile(filepath.Join(repo, "toy.go"), []byte("package toy\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(toy.go): %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	baseSHA = runGit(t, repo, "rev-parse", "HEAD")
	return repo, baseSHA
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-c", "user.email=triage-test@example.com",
		"-c", "user.name=triage-test",
	}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestTriageAgent_BusinessHunkPinsHighRisk(t *testing.T) {
	ctx := context.Background()
	l := openTriageLedger(t)
	svc := reconcile.NewService(l)
	cand := approveRunningCandidate(t, svc, "cand-business")
	repo, baseSHA := initTriageRepo(t)

	invoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		payload := conflicttriage.TriagePayload{
			CauseSummary: "pricing rule conflict",
			HunkDecisions: []conflicttriage.HunkDecision{
				{
					HunkID:     "hunk-business",
					Kind:       conflicttriage.HunkKindBusiness,
					Resolution: "take-ours",
					Rationale:  "no technical criterion; picked a side",
				},
				{
					HunkID:     "hunk-slice",
					Kind:       conflicttriage.HunkKindSliceUnion,
					Resolution: "union",
					Rationale:  "union both slice literals",
				},
			},
			RiskBand:     conflicttriage.RiskMedium,
			VerifyBudget: "",
			ProposedSHA:  "proposed-not-candidate-sha",
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		return string(raw), resolve.ErrSemanticAmbiguity
	}

	result, err := conflicttriage.RunTriage(ctx, conflicttriage.RunOptions{
		CandidateID:  cand.ID,
		WorktreePath: repo,
		BaseSHA:      baseSHA,
		AllowedPaths: []string{"toy.go"},
		Invoker:      invoker,
		Service:      svc,
	})
	if err != nil {
		t.Fatalf("RunTriage() error = %v, want nil (fail-open must not return ErrSemanticAmbiguity)", err)
	}
	if errors.Is(err, resolve.ErrSemanticAmbiguity) {
		t.Fatalf("RunTriage() returned ErrSemanticAmbiguity, want fail-open")
	}

	business := false
	for _, h := range result.Payload.HunkDecisions {
		if h.Kind == conflicttriage.HunkKindBusiness {
			business = true
			if h.Resolution != conflicttriage.ResolutionArbitrary {
				t.Errorf("business hunk resolution = %q, want ARBITRARY", h.Resolution)
			}
		}
	}
	if !business {
		t.Fatalf("payload missing business hunk: %+v", result.Payload.HunkDecisions)
	}
	if result.Payload.RiskBand != conflicttriage.RiskHigh {
		t.Errorf("RiskBand = %q, want %q (business hunk pins high; must not lower)", result.Payload.RiskBand, conflicttriage.RiskHigh)
	}
	if !strings.HasPrefix(result.Payload.VerifyBudget, "~") || !strings.Contains(result.Payload.VerifyBudget, " min: ") {
		t.Errorf("VerifyBudget = %q, want ~N min: <cmd>", result.Payload.VerifyBudget)
	}

	got, err := svc.GetCandidate(ctx, cand.ID)
	if err != nil {
		t.Fatalf("GetCandidate: %v", err)
	}
	if got.CandidateSHA != "" {
		t.Errorf("CandidateSHA = %q, want empty (proposed_sha only, never CandidateSHA)", got.CandidateSHA)
	}
	if got.Status != reconcile.CandidateStatusRunning {
		t.Errorf("Status = %v, want still %v", got.Status, reconcile.CandidateStatusRunning)
	}
	var stored conflicttriage.TriagePayload
	if err := json.Unmarshal([]byte(got.Output), &stored); err != nil {
		t.Fatalf("Candidate.Output is not TriagePayload JSON: %v (%q)", err, got.Output)
	}
	if stored.ProposedSHA != "proposed-not-candidate-sha" {
		t.Errorf("stored ProposedSHA = %q, want proposed-not-candidate-sha", stored.ProposedSHA)
	}
	if stored.RiskBand != conflicttriage.RiskHigh {
		t.Errorf("stored RiskBand = %q, want high", stored.RiskBand)
	}
}

func TestTriageAgent_InvariantViolationsFailCandidate(t *testing.T) {
	ctx := context.Background()
	l := openTriageLedger(t)
	svc := reconcile.NewService(l)
	cand := approveRunningCandidate(t, svc, "cand-invariants")
	repo, baseSHA := initTriageRepo(t)

	if err := os.WriteFile(filepath.Join(repo, "toy.go"), []byte("package toy\n<<<<<<< HEAD\nleft\n=======\nright\n>>>>>>> other\n"), 0o644); err != nil {
		t.Fatalf("WriteFile leftover markers: %v", err)
	}

	invoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		payload := conflicttriage.TriagePayload{
			CauseSummary: "leftover markers",
			HunkDecisions: []conflicttriage.HunkDecision{{
				HunkID:     "hunk-business",
				Kind:       conflicttriage.HunkKindBusiness,
				Resolution: conflicttriage.ResolutionArbitrary,
				Rationale:  "arbitrary",
			}},
			RiskBand:     conflicttriage.RiskHigh,
			VerifyBudget: conflicttriage.VerifyBudgetExample,
			ProposedSHA:  "deadbeef",
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}

	_, err := conflicttriage.RunTriage(ctx, conflicttriage.RunOptions{
		CandidateID:  cand.ID,
		WorktreePath: repo,
		BaseSHA:      baseSHA,
		AllowedPaths: []string{"toy.go"},
		Invoker:      invoker,
		Service:      svc,
	})
	if err == nil {
		t.Fatalf("RunTriage() error = nil, want invariant failure")
	}

	got, err := svc.GetCandidate(ctx, cand.ID)
	if err != nil {
		t.Fatalf("GetCandidate: %v", err)
	}
	if got.Status != reconcile.CandidateStatusFailed {
		t.Errorf("Status = %v, want %v", got.Status, reconcile.CandidateStatusFailed)
	}
	if got.Output == "" {
		t.Errorf("Output empty after invariant failure, want JSON retained for auditability")
	}
	if got.CandidateSHA != "" {
		t.Errorf("CandidateSHA = %q, want empty", got.CandidateSHA)
	}
}
