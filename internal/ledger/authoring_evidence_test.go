package ledger

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
)

func TestFreezeAuthoringEvidenceAndPersistVersionedCandidate(t *testing.T) {
	e := AuthoringEvidence{PacketDigest: "packet-digest", AuthoringMode: "versioned", ContractVersion: "packet-author/v1",
		Contract: json.RawMessage(`{"version":"packet-author/v1"}`), Binding: json.RawMessage(`{"kind":"feature"}`), Mode: "write", CommitObligation: "required",
		WritePaths: []string{"src"}, ReadOnlyPaths: []string{"docs"}, DoneCriteria: []string{"criterion"}, HardStops: []string{"stop"},
		ResultPath: ".lucind/result.json", ResultSchema: ".lucind/result.schema.json", BaseCommit: "base", BaseTree: "base-tree",
		CandidateCommit: "candidate", CandidateTree: "candidate-tree", Changes: []candidatechange.Change{{Change: candidatechange.Copied, SourcePath: "src/a", Path: "src/b"}}, ResultHash: "result-hash"}
	encoded, hash, err := FreezeAuthoringEvidence(e)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeAuthoringEvidence(AuthoringEvidenceVersion, encoded, hash)
	if err != nil || decoded.Hash != hash || decoded.Changes[0].SourcePath != "src/a" {
		t.Fatalf("DecodeAuthoringEvidence() = %+v, %v", decoded, err)
	}

	l := openTestLedger(t)
	ctx := context.Background()
	if err := l.RegisterLane(ctx, Lane{RunID: "r", LaneID: "l", PacketID: "p", Executor: "agy", RoutingCondition: "test", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	c := testLaneCandidate()
	c.RunID, c.LaneID, c.PacketID = "r", "l", "p"
	c.AuthoringEvidenceVersion, c.AuthoringEvidenceJSON, c.AuthoringEvidenceHash = AuthoringEvidenceVersion, encoded, hash
	if err := l.SetDoneCandidate(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := l.GetLaneCandidate(ctx, "r", "l")
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthoringEvidenceHash != hash {
		t.Fatalf("evidence hash = %q, want %q", got.AuthoringEvidenceHash, hash)
	}
}

func TestSchemaV10AddsAuthoringEvidenceAndPreservesLegacyReads(t *testing.T) {
	root := createV9SchemaFixture(t, context.Background())
	l, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	assertTableColumns(t, l.db, "packet_author_shadow_attempts", []string{"id", "run_id", "lane_id", "input_hash", "specialist_identity", "failure_class", "valid", "equivalent", "diff_json", "manual_digest", "specialist_digest", "replay_stable", "latency_ms", "created_at"})
	assertTableColumns(t, l.db, "packet_author_shadow_reviews", []string{"attempt_id", "reviewer", "review_ms", "created_at"})
	legacy, err := l.GetLaneCandidate(context.Background(), "legacy-run", "legacy-lane")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.AuthoringEvidenceVersion != LegacyAuthoringVersion || legacy.AuthoringEvidenceJSON != "" || legacy.AuthoringEvidenceHash != "" {
		t.Fatalf("legacy evidence = %+v", legacy)
	}
	var version int
	if err := l.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil || version != 10 {
		t.Fatalf("schema version = %d, %v", version, err)
	}
}
