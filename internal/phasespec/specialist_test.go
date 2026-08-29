package phasespec_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/phasespec"
)

func TestPhaseSpecialist(t *testing.T) {
	tempDir := t.TempDir()
	change := "skill-provisioning-and-phase-specialist"

	// Mock gentle-ai sdd-status json output in propose phase
	statusJSON := []byte(`{
  "schemaName": "gentle-ai.sdd-status",
  "schemaVersion": 1,
  "changeName": "` + change + `",
  "artifactStore": "openspec",
  "planningHome": {
    "mode": "repo-local",
    "path": "/workspace/openspec"
  },
  "changeRoot": "/workspace/openspec/changes/` + change + `",
  "artifactPaths": {
    "proposal": ["/workspace/openspec/changes/` + change + `/proposal.md"],
    "design": [],
    "tasks": []
  },
  "artifacts": {
    "proposal": "missing",
    "design": "missing",
    "tasks": "missing"
  },
  "dependencies": {
    "proposal": "ready",
    "design": "blocked",
    "tasks": "blocked"
  },
  "nextRecommended": "propose",
  "blockedReasons": []
}`)

	querier := &mockStatusQuerier{output: statusJSON}
	adapter := phasespec.NewAdapter(querier, tempDir)

	// Step 1: Attempt synthesis with unmerged lenses -> must fail and write nothing
	unmergedReq := phasespec.SynthesizeRequest{
		ChangeName: change,
		Phase:      "propose",
		LensStates: map[string]phasespec.LensState{
			"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
			"lens-b": {ID: "lens-b", Accepted: true, Merged: false},
			"lens-c": {ID: "lens-c", Accepted: false, Merged: false},
		},
		Content: []byte("# Premature Proposal\n"),
	}

	_, err := adapter.Synthesize(context.Background(), unmergedReq)
	if err == nil {
		t.Fatal("expected premature synthesis to be gated, got nil error")
	}

	// Verify no artifact written
	if files := countFilesInDir(t, tempDir); files != 0 {
		t.Fatalf("expected 0 files written during gated synthesis, got %d", files)
	}

	// Step 2: All lenses accepted and merged -> synthesis succeeds and writes canonical artifact
	mergedReq := phasespec.SynthesizeRequest{
		ChangeName: change,
		Phase:      "propose",
		LensStates: map[string]phasespec.LensState{
			"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
			"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
			"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
		},
		Content: []byte("# Canonical Proposal\n"),
	}

	res, err := adapter.Synthesize(context.Background(), mergedReq)
	if err != nil {
		t.Fatalf("expected successful synthesis, got %v", err)
	}
	if !res.Written {
		t.Fatal("expected res.Written = true")
	}

	expectedRel := filepath.Join("openspec", "changes", change, "proposal.md")
	if res.ArtifactPath != expectedRel {
		t.Fatalf("res.ArtifactPath = %q, want %q", res.ArtifactPath, expectedRel)
	}

	writtenBytes, err := os.ReadFile(filepath.Join(tempDir, expectedRel))
	if err != nil {
		t.Fatalf("failed reading written artifact: %v", err)
	}
	if string(writtenBytes) != "# Canonical Proposal\n" {
		t.Fatalf("unexpected content: %q", string(writtenBytes))
	}
}
