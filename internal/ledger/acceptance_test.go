package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testLaneCandidate() LaneCandidate {
	return LaneCandidate{
		RunID: "run-1", LaneID: "lane-1", PacketID: "packet-1",
		PacketDigest: "packet-digest", PrimaryRoot: "/repo", WorktreePath: "/repo-worktrees/lane-1",
		BaseCommit: "base-commit", BaseTree: "base-tree", CandidateCommit: "candidate-commit",
		CandidateTree: "candidate-tree", AllowedPaths: []string{"cmd/lucind-ai", "internal/accept"},
		ResultPath: ".lucind/result.json", ResultJSON: `{}`, ResultHash: "result-hash",
		RecordedAt: time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
	}
}

func testAcceptanceReceipt() AcceptanceReceipt {
	return AcceptanceReceipt{
		ReceiptID: "receipt-1", BindingHash: "binding-hash", ResultHash: "result-hash",
		ChecksHash: "checks-hash", Cleanup: "removed", CreatedAt: time.Date(2026, 8, 26, 12, 1, 0, 0, time.UTC),
		Binding: AcceptanceBinding{
			RunID: "run-1", LaneID: "lane-1", PacketID: "packet-1", PacketDigest: "packet-digest",
			BaseCommit: "base-commit", BaseTree: "base-tree", CandidateCommit: "candidate-commit",
			CandidateTree: "candidate-tree", AllowedPathsHash: "paths-hash",
			CheckPolicyHash: "policy-hash", EnvironmentHash: "environment-hash", BindingVersion: "binding:v2",
			ContractVersion: "packet-author/v1", AuthoringEvidenceVersion: AuthoringEvidenceVersion, AuthoringEvidenceHash: "evidence-hash",
		},
	}
}

func TestSetDoneCandidateIsAtomicAndImmutable(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	if err := l.RegisterLane(ctx, Lane{RunID: "run-1", LaneID: "lane-1", PacketID: "packet-1", Executor: "agy", RoutingCondition: "test", Status: "running"}); err != nil {
		t.Fatal(err)
	}

	candidate := testLaneCandidate()
	if err := l.SetDoneCandidate(ctx, candidate); err != nil {
		t.Fatalf("SetDoneCandidate() error = %v", err)
	}
	got, err := l.GetLaneCandidate(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("GetLaneCandidate() error = %v", err)
	}
	if got.CandidateCommit != candidate.CandidateCommit || got.BaseTree != candidate.BaseTree || len(got.AllowedPaths) != 2 {
		t.Fatalf("candidate = %+v, want complete persisted identity", got)
	}
	changed := candidate
	changed.CandidateCommit = "other"
	if err := l.SetDoneCandidate(ctx, changed); !errors.Is(err, ErrImmutableAcceptanceEvidence) {
		t.Fatalf("changed SetDoneCandidate() error = %v, want ErrImmutableAcceptanceEvidence", err)
	}
	got, _ = l.GetLaneCandidate(ctx, "run-1", "lane-1")
	if got.CandidateCommit != candidate.CandidateCommit {
		t.Fatalf("immutable candidate changed to %q", got.CandidateCommit)
	}
}

func TestAcceptanceReceiptInsertAndExactReuse(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	receipt := testAcceptanceReceipt()

	inserted, err := l.InsertAcceptanceReceipt(ctx, receipt)
	if err != nil || inserted.ReceiptID != receipt.ReceiptID {
		t.Fatalf("InsertAcceptanceReceipt() = %+v, %v", inserted, err)
	}
	reused, err := l.InsertAcceptanceReceipt(ctx, receipt)
	if err != nil || reused != inserted {
		t.Fatalf("exact reuse = %+v, %v, want %+v", reused, err, inserted)
	}
	got, err := l.FindAcceptanceReceipt(ctx, receipt.BindingHash)
	if err != nil || got != receipt {
		t.Fatalf("FindAcceptanceReceipt() = %+v, %v, want %+v", got, err, receipt)
	}

	mismatch := receipt
	mismatch.ReceiptID = "receipt-2"
	mismatch.Binding.CandidateTree = "different-tree"
	if _, err := l.InsertAcceptanceReceipt(ctx, mismatch); !errors.Is(err, ErrAcceptanceBindingMismatch) {
		t.Fatalf("mismatch error = %v, want ErrAcceptanceBindingMismatch", err)
	}
	if count := acceptanceReceiptCount(t, l); count != 1 {
		t.Fatalf("receipt count = %d, want 1 after rejected mismatch", count)
	}
}

func TestAcceptanceRowsRejectUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	if _, err := l.db.ExecContext(ctx, `INSERT INTO lane_candidates
		(run_id,lane_id,packet_id,packet_digest,primary_root,worktree_path,base_commit,base_tree,candidate_commit,candidate_tree,allowed_paths,result_path,result_json,result_hash,recorded_at)
		VALUES ('r','l','p','pd','/r','/w','bc','bt','cc','ct','[]','.lucind/result.json','{}','rh','2026-08-26T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE lane_candidates SET candidate_tree='changed' WHERE run_id='r' AND lane_id='l'`,
		`DELETE FROM lane_candidates WHERE run_id='r' AND lane_id='l'`,
	} {
		if _, err := l.db.ExecContext(ctx, statement); err == nil {
			t.Fatalf("immutable statement succeeded: %s", statement)
		}
	}
}

func acceptanceReceiptCount(t *testing.T, l *Ledger) int {
	t.Helper()
	var count int
	if err := l.db.QueryRow(`SELECT COUNT(*) FROM acceptance_receipts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
