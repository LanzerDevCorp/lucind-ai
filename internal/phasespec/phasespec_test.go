package phasespec_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/phasespec"
)

type mockStatusQuerier struct {
	output []byte
	err    error
}

func (m *mockStatusQuerier) QueryStatus(_ context.Context, _ string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

func validStatusJSON(change string) []byte {
	return []byte(`{
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
}

func countFilesInDir(t *testing.T, dir string) int {
	t.Helper()
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("countFilesInDir failed: %v", err)
	}
	return count
}

func TestSpecialistFailsClosedOnMalformedStatusJSON(t *testing.T) {
	malformedInputs := []struct {
		name string
		json string
	}{
		{"empty", ""},
		{"whitespace only", "   \n\t  "},
		{"truncated json", `{"schemaName": "gentle-ai.sdd-status", "schemaVersion":`},
		{"non-json garbage", "<html><body>502 Bad Gateway</body></html>"},
		{"wrong schemaName", `{"schemaName": "other.schema", "schemaVersion": 1, "changeName": "my-change"}`},
		{"missing schemaName", `{"schemaVersion": 1, "changeName": "my-change"}`},
		{"missing changeName", `{"schemaName": "gentle-ai.sdd-status", "schemaVersion": 1, "changeName": ""}`},
		{"invalid schemaVersion zero", `{"schemaName": "gentle-ai.sdd-status", "schemaVersion": 0, "changeName": "my-change"}`},
		{"invalid schemaVersion negative", `{"schemaName": "gentle-ai.sdd-status", "schemaVersion": -1, "changeName": "my-change"}`},
		{"multiple json documents", `{"schemaName": "gentle-ai.sdd-status", "schemaVersion": 1, "changeName": "c1"} {"extra": true}`},
	}

	for _, tt := range malformedInputs {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// 1. Direct parser check
			_, err := phasespec.ParseStatus([]byte(tt.json))
			if err == nil {
				t.Fatalf("ParseStatus expected error for malformed JSON %q, got nil", tt.name)
			}
			if !errors.Is(err, phasespec.ErrMalformedStatus) {
				t.Fatalf("ParseStatus error = %v, want ErrMalformedStatus", err)
			}

			// 2. Adapter synthesize check: must fail closed without writing files
			querier := &mockStatusQuerier{output: []byte(tt.json)}
			adapter := phasespec.NewAdapter(querier, tempDir)

			req := phasespec.SynthesizeRequest{
				ChangeName: "my-change",
				Phase:      "propose",
				LensStates: map[string]phasespec.LensState{
					"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
					"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
					"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
				},
				Content: []byte("# Propose Canonical\n"),
			}

			res, err := adapter.Synthesize(context.Background(), req)
			if err == nil {
				t.Fatalf("Synthesize expected error for malformed JSON %q, got result %#v", tt.name, res)
			}
			if !errors.Is(err, phasespec.ErrMalformedStatus) {
				t.Fatalf("Synthesize error = %v, want ErrMalformedStatus", err)
			}

			// Verify zero files created in tempDir
			if files := countFilesInDir(t, tempDir); files != 0 {
				t.Fatalf("filesystem mutation detected: %d files created in tempDir", files)
			}
		})
	}
}

func TestSpecialistFailsClosedOnCLIError(t *testing.T) {
	tempDir := t.TempDir()
	cliErr := errors.New("exit status 1: sdd-status command failed")
	querier := &mockStatusQuerier{err: cliErr}
	adapter := phasespec.NewAdapter(querier, tempDir)

	req := phasespec.SynthesizeRequest{
		ChangeName: "my-change",
		Phase:      "design",
		LensStates: map[string]phasespec.LensState{
			"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
			"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
			"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
		},
		Content: []byte("# Design Canonical\n"),
	}

	_, err := adapter.Synthesize(context.Background(), req)
	if err == nil {
		t.Fatal("Synthesize expected error on CLI error, got nil")
	}
	if !errors.Is(err, phasespec.ErrCLIExecution) {
		t.Fatalf("error = %v, want ErrCLIExecution", err)
	}

	// Verify zero files created in tempDir
	if files := countFilesInDir(t, tempDir); files != 0 {
		t.Fatalf("filesystem mutation detected: %d files created in tempDir", files)
	}
}

func TestSynthesisGatedUntilLensesAcceptedAndMerged(t *testing.T) {
	tests := []struct {
		name        string
		reqLenses   []string
		states      map[string]phasespec.LensState
		shouldAllow bool
		errContains string
	}{
		{
			name:      "all three default lenses accepted and merged",
			reqLenses: nil, // defaults to lens-a, lens-b, lens-c
			states: map[string]phasespec.LensState{
				"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
				"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
				"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
			},
			shouldAllow: true,
		},
		{
			name:      "lens-c missing from states",
			reqLenses: nil,
			states: map[string]phasespec.LensState{
				"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
				"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
			},
			shouldAllow: false,
			errContains: `lens "lens-c" is missing`,
		},
		{
			name:      "lens-b not accepted",
			reqLenses: nil,
			states: map[string]phasespec.LensState{
				"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
				"lens-b": {ID: "lens-b", Accepted: false, Merged: true},
				"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
			},
			shouldAllow: false,
			errContains: `lens "lens-b" is not accepted`,
		},
		{
			name:      "lens-a not merged",
			reqLenses: nil,
			states: map[string]phasespec.LensState{
				"lens-a": {ID: "lens-a", Accepted: true, Merged: false},
				"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
				"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
			},
			shouldAllow: false,
			errContains: `lens "lens-a" is not merged`,
		},
		{
			name:      "custom required lenses all merged",
			reqLenses: []string{"lens-1", "lens-2"},
			states: map[string]phasespec.LensState{
				"lens-1": {ID: "lens-1", Accepted: true, Merged: true},
				"lens-2": {ID: "lens-2", Accepted: true, Merged: true},
			},
			shouldAllow: true,
		},
		{
			name:      "custom required lenses with unmerged lens",
			reqLenses: []string{"lens-1", "lens-2"},
			states: map[string]phasespec.LensState{
				"lens-1": {ID: "lens-1", Accepted: true, Merged: true},
				"lens-2": {ID: "lens-2", Accepted: true, Merged: false},
			},
			shouldAllow: false,
			errContains: `lens "lens-2" is not merged`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// 1. Direct eligibility check
			err := phasespec.CheckSynthesisEligibility("propose", tt.reqLenses, tt.states)
			if tt.shouldAllow {
				if err != nil {
					t.Fatalf("CheckSynthesisEligibility expected nil, got %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("CheckSynthesisEligibility expected error, got nil")
				}
				if !errors.Is(err, phasespec.ErrPrematureSynthesis) {
					t.Fatalf("error = %v, want ErrPrematureSynthesis", err)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			}

			// 2. Adapter synthesize execution check
			querier := &mockStatusQuerier{output: validStatusJSON("test-change")}
			adapter := phasespec.NewAdapter(querier, tempDir)

			req := phasespec.SynthesizeRequest{
				ChangeName:     "test-change",
				Phase:          "propose",
				RequiredLenses: tt.reqLenses,
				LensStates:     tt.states,
				Content:        []byte("# Proposal Content\n"),
			}

			res, err := adapter.Synthesize(context.Background(), req)
			if tt.shouldAllow {
				if err != nil {
					t.Fatalf("Synthesize unexpected error: %v", err)
				}
				if !res.Written {
					t.Fatal("expected artifact to be written")
				}
				expectedFile := filepath.Join(tempDir, "openspec", "changes", "test-change", "proposal.md")
				content, readErr := os.ReadFile(expectedFile)
				if readErr != nil {
					t.Fatalf("failed to read written file: %v", readErr)
				}
				if string(content) != "# Proposal Content\n" {
					t.Fatalf("file content mismatch: got %q", string(content))
				}
			} else {
				if err == nil {
					t.Fatal("Synthesize expected error for unmerged lenses, got nil")
				}
				if !errors.Is(err, phasespec.ErrPrematureSynthesis) {
					t.Fatalf("error = %v, want ErrPrematureSynthesis", err)
				}
				// Verify zero files created in tempDir
				if files := countFilesInDir(t, tempDir); files != 0 {
					t.Fatalf("filesystem mutation detected: %d files created in tempDir", files)
				}
			}
		})
	}
}

func TestConsumesStatusAndWritesCanonicalArtifact(t *testing.T) {
	phaseCases := []struct {
		phase            string
		expectedFilename string
	}{
		{"explore", "explore.md"},
		{"propose", "proposal.md"},
		{"proposal", "proposal.md"},
		{"spec", "spec.md"},
		{"specs", "spec.md"},
		{"design", "design.md"},
		{"tasks", "tasks.md"},
		{"apply", "apply-progress.md"},
		{"verify", "verify-report.md"},
		{"archive", "archive-report.md"},
	}

	for _, pc := range phaseCases {
		t.Run(pc.phase, func(t *testing.T) {
			tempDir := t.TempDir()
			change := "my-feature-change"
			querier := &mockStatusQuerier{output: validStatusJSON(change)}
			adapter := phasespec.NewAdapter(querier, tempDir)

			content := "# Phase " + pc.phase + "\nCanonical document body.\n"
			req := phasespec.SynthesizeRequest{
				ChangeName: change,
				Phase:      pc.phase,
				LensStates: map[string]phasespec.LensState{
					"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
					"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
					"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
				},
				Content: []byte(content),
			}

			res, err := adapter.Synthesize(context.Background(), req)
			if err != nil {
				t.Fatalf("Synthesize failed for phase %q: %v", pc.phase, err)
			}
			if !res.Written {
				t.Fatalf("expected Written=true for phase %q", pc.phase)
			}

			expectedRelPath := filepath.Join("openspec", "changes", change, pc.expectedFilename)
			if res.ArtifactPath != expectedRelPath {
				t.Fatalf("res.ArtifactPath = %q, want %q", res.ArtifactPath, expectedRelPath)
			}

			fullPath := filepath.Join(tempDir, expectedRelPath)
			readBytes, err := os.ReadFile(fullPath)
			if err != nil {
				t.Fatalf("failed to read artifact file %q: %v", fullPath, err)
			}
			if string(readBytes) != content {
				t.Fatalf("content mismatch: got %q, want %q", string(readBytes), content)
			}

			// Verify ONLY one file was created
			if files := countFilesInDir(t, tempDir); files != 1 {
				t.Fatalf("expected exactly 1 file created, got %d", files)
			}
		})
	}
}

func TestPhaseAlreadyCompleteNoRedundantDispatch(t *testing.T) {
	tempDir := t.TempDir()
	change := "completed-change"

	completeStatusJSON := []byte(`{
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
    "proposal": ["/workspace/openspec/changes/` + change + `/proposal.md"]
  },
  "artifacts": {
    "proposal": "done",
    "design": "missing",
    "tasks": "missing"
  },
  "dependencies": {
    "proposal": "all_done",
    "design": "ready",
    "tasks": "blocked"
  },
  "nextRecommended": "design",
  "blockedReasons": []
}`)

	querier := &mockStatusQuerier{output: completeStatusJSON}
	adapter := phasespec.NewAdapter(querier, tempDir)

	req := phasespec.SynthesizeRequest{
		ChangeName: change,
		Phase:      "proposal",
		LensStates: map[string]phasespec.LensState{
			"lens-a": {ID: "lens-a", Accepted: true, Merged: true},
			"lens-b": {ID: "lens-b", Accepted: true, Merged: true},
			"lens-c": {ID: "lens-c", Accepted: true, Merged: true},
		},
		Content: []byte("# New Redundant Proposal\n"),
	}

	res, err := adapter.Synthesize(context.Background(), req)
	if err != nil {
		t.Fatalf("Synthesize failed on already complete phase: %v", err)
	}
	if res.Written {
		t.Fatal("expected Written=false on already complete phase, got true")
	}

	// Verify no files were created
	if files := countFilesInDir(t, tempDir); files != 0 {
		t.Fatalf("expected 0 files created, got %d", files)
	}
}

func TestPathTraversalAndSecurityGuards(t *testing.T) {
	tempDir := t.TempDir()
	querier := &mockStatusQuerier{output: validStatusJSON("valid-change")}
	adapter := phasespec.NewAdapter(querier, tempDir)

	maliciousRequests := []struct {
		name    string
		req     phasespec.SynthesizeRequest
		wantErr error
	}{
		{
			name: "path traversal in change name",
			req: phasespec.SynthesizeRequest{
				ChangeName: "../../../etc",
				Phase:      "propose",
				Content:    []byte("evil"),
			},
			wantErr: phasespec.ErrInvalidChange,
		},
		{
			name: "slash in change name",
			req: phasespec.SynthesizeRequest{
				ChangeName: "foo/bar",
				Phase:      "propose",
				Content:    []byte("evil"),
			},
			wantErr: phasespec.ErrInvalidChange,
		},
		{
			name: "empty change name",
			req: phasespec.SynthesizeRequest{
				ChangeName: "",
				Phase:      "propose",
				Content:    []byte("evil"),
			},
			wantErr: phasespec.ErrInvalidChange,
		},
		{
			name: "invalid phase name",
			req: phasespec.SynthesizeRequest{
				ChangeName: "valid-change",
				Phase:      "hack-phase",
				Content:    []byte("evil"),
			},
			wantErr: phasespec.ErrInvalidPhase,
		},
	}

	for _, tt := range maliciousRequests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := adapter.Synthesize(context.Background(), tt.req)
			if err == nil {
				t.Fatalf("expected error for malicious request %q, got nil", tt.name)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if files := countFilesInDir(t, tempDir); files != 0 {
				t.Fatalf("filesystem mutation detected: %d files created in tempDir", files)
			}
		})
	}
}
