package run_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

func TestAdmissionRejectsMissingFeatureTargetWithoutLegacyMain(t *testing.T) {
	tests := []struct {
		name string
		pkt  packet.Packet
	}{
		{
			name: "all four target fields empty",
			pkt: packet.Packet{
				ID:       "lane-1",
				Executor: "agy",
				RoutedBy: "test",
				Body:     "prompt",
			},
		},
		{
			name: "missing feature",
			pkt: packet.Packet{
				ID:                "lane-1",
				Executor:          "agy",
				RoutedBy:          "test",
				ParentRef:         "refs/heads/feature/foo",
				BaseSHA:           "1111111111111111111111111111111111111111",
				ExpectedParentSHA: "2222222222222222222222222222222222222222",
				Body:              "prompt",
			},
		},
		{
			name: "missing parent_ref",
			pkt: packet.Packet{
				ID:                "lane-1",
				Executor:          "agy",
				RoutedBy:          "test",
				Feature:           "foo",
				BaseSHA:           "1111111111111111111111111111111111111111",
				ExpectedParentSHA: "2222222222222222222222222222222222222222",
				Body:              "prompt",
			},
		},
		{
			name: "missing base_sha",
			pkt: packet.Packet{
				ID:                "lane-1",
				Executor:          "agy",
				RoutedBy:          "test",
				Feature:           "foo",
				ParentRef:         "refs/heads/feature/foo",
				ExpectedParentSHA: "2222222222222222222222222222222222222222",
				Body:              "prompt",
			},
		},
		{
			name: "missing expected_parent_sha",
			pkt: packet.Packet{
				ID:        "lane-1",
				Executor:  "agy",
				RoutedBy:  "test",
				Feature:   "foo",
				ParentRef: "refs/heads/feature/foo",
				BaseSHA:   "1111111111111111111111111111111111111111",
				Body:      "prompt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtCalled := false
			deps := newTestDeps(t, t.TempDir(), func(string) fs.FS { return fstest.MapFS{} }, &fakeExecutor{})
			deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
				wtCalled = true
				return worktree.Worktree{Path: t.TempDir(), BaseSHA: "1111111111111111111111111111111111111111"}, nil
			}

			_, err := run.Execute(context.Background(), deps, tt.pkt)
			if err == nil {
				t.Fatalf("Execute() error = nil, want ErrMissingFeatureTarget")
			}
			if !errors.Is(err, run.ErrMissingFeatureTarget) {
				t.Errorf("Execute() error = %v, want wrapping %v", err, run.ErrMissingFeatureTarget)
			}
			if wtCalled {
				t.Errorf("deps.CreateWorktree was called, but admission check must fail before worktree creation")
			}
		})
	}
}

func TestAdmissionAcceptsExplicitTargetFields(t *testing.T) {
	pkt := packet.Packet{
		ID:                "lane-1",
		Executor:          "agy",
		RoutedBy:          "test",
		Feature:           "user-auth",
		ParentRef:         "refs/heads/feature/user-auth",
		BaseSHA:           "1111111111111111111111111111111111111111",
		ExpectedParentSHA: "2222222222222222222222222222222222222222",
		Body:              "prompt",
	}

	wtDir := t.TempDir()
	fsys := func(string) fs.FS {
		return fstest.MapFS{
			"result.json": &fstest.MapFile{Data: []byte(doneEnvelopeJSON)},
		}
	}
	exec := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, fsys, exec)
	wtCalled := false
	deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
		wtCalled = true
		return worktree.Worktree{Path: wtDir, BaseSHA: "1111111111111111111111111111111111111111"}, nil
	}

	report, err := run.Execute(context.Background(), deps, pkt)
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}
	if !wtCalled {
		t.Errorf("deps.CreateWorktree was not called")
	}
	if report.LaneID != "lane-1" {
		t.Errorf("report.LaneID = %q, want %q", report.LaneID, "lane-1")
	}
}

func TestAdmissionAcceptsLegacyMainWithExpectedParentSHA(t *testing.T) {
	pkt := packet.Packet{
		ID:                "lane-1",
		Executor:          "agy",
		RoutedBy:          "test",
		LegacyMain:        true,
		ExpectedParentSHA: "2222222222222222222222222222222222222222",
		Body:              "prompt",
	}

	wtDir := t.TempDir()
	fsys := func(string) fs.FS {
		return fstest.MapFS{
			"result.json": &fstest.MapFile{Data: []byte(doneEnvelopeJSON)},
		}
	}
	exec := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, fsys, exec)
	wtCalled := false
	deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
		wtCalled = true
		return worktree.Worktree{Path: wtDir, BaseSHA: "1111111111111111111111111111111111111111"}, nil
	}

	report, err := run.Execute(context.Background(), deps, pkt)
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}
	if !wtCalled {
		t.Errorf("deps.CreateWorktree was not called")
	}
	if report.LaneID != "lane-1" {
		t.Errorf("report.LaneID = %q, want %q", report.LaneID, "lane-1")
	}
}

func TestAdmissionRejectsLegacyMainWithoutExpectedParentSHA(t *testing.T) {
	pkt := packet.Packet{
		ID:         "lane-1",
		Executor:   "agy",
		RoutedBy:   "test",
		LegacyMain: true,
		// ExpectedParentSHA omitted
		Body: "prompt",
	}

	wtCalled := false
	deps := newTestDeps(t, t.TempDir(), func(string) fs.FS { return fstest.MapFS{} }, &fakeExecutor{})
	deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
		wtCalled = true
		return worktree.Worktree{Path: t.TempDir(), BaseSHA: "1111111111111111111111111111111111111111"}, nil
	}

	_, err := run.Execute(context.Background(), deps, pkt)
	if err == nil {
		t.Fatalf("Execute() error = nil, want ErrMissingFeatureTarget")
	}
	if !errors.Is(err, run.ErrMissingFeatureTarget) {
		t.Errorf("Execute() error = %v, want wrapping %v", err, run.ErrMissingFeatureTarget)
	}
	if wtCalled {
		t.Errorf("deps.CreateWorktree was called, but admission check must fail before worktree creation")
	}
}
