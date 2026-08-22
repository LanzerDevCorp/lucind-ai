package dag_test

import (
	"errors"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dag"
)

func TestValidate_DuplicatePacketID(t *testing.T) {
	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:           "apply-ledger",
				Executor:     "agy",
				RoutedBy:     "first",
				AllowedPaths: []string{"internal/ledger/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-ledger.md",
			},
			{
				ID:           "apply-ledger",
				Executor:     "agy",
				RoutedBy:     "second",
				AllowedPaths: []string{"internal/serve/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-serve.md",
			},
		},
	}

	err := dag.Validate(d)
	if err == nil {
		t.Fatal("expected error for duplicate packet ID, got nil")
	}
}

func TestValidate_EmptyAllowedPaths(t *testing.T) {
	tests := []struct {
		name         string
		allowedPaths []string
	}{
		{
			name:         "nil allowed_paths",
			allowedPaths: nil,
		},
		{
			name:         "empty slice allowed_paths",
			allowedPaths: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dag.DAG{
				Change: "test-change",
				Packets: []dag.Node{
					{
						ID:           "apply-ledger",
						Executor:     "agy",
						RoutedBy:     "first",
						AllowedPaths: tt.allowedPaths,
						DependsOn:    []string{},
						BodyPath:     "bodies/apply-ledger.md",
					},
				},
			}

			err := dag.Validate(d)
			if err == nil {
				t.Fatalf("expected error for empty/omitted allowed_paths in test %q, got nil", tt.name)
			}
		})
	}
}

func TestValidate_ValidDAG(t *testing.T) {
	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:           "apply-ledger",
				Executor:     "agy",
				RoutedBy:     "first",
				AllowedPaths: []string{"internal/ledger/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-ledger.md",
			},
			{
				ID:           "apply-serve",
				Executor:     "agy",
				RoutedBy:     "second",
				AllowedPaths: []string{"internal/serve/"},
				DependsOn:    []string{"apply-ledger"},
				BodyPath:     "bodies/apply-serve.md",
			},
		},
	}

	if err := dag.Validate(d); err != nil {
		t.Fatalf("unexpected error for valid DAG: %v", err)
	}
}

func TestValidate_ReadOnlyFoundedByDependency(t *testing.T) {
	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:           "apply-red",
				Executor:     "agy",
				RoutedBy:     "writes failing test",
				AllowedPaths: []string{"test/foo_test.go"},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-red.md",
			},
			{
				ID:           "apply-green",
				Executor:     "agy",
				RoutedBy:     "implements to make test pass",
				AllowedPaths: []string{"impl/foo.go"},
				ReadOnly:     []string{"test/foo_test.go"},
				DependsOn:    []string{"apply-red"},
				BodyPath:     "bodies/apply-green.md",
			},
		},
	}

	if err := dag.Validate(d); err != nil {
		t.Fatalf("unexpected error for valid read_only DAG: %v", err)
	}
}

func TestValidate_ReadOnlySelfContradiction(t *testing.T) {
	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:           "apply-contradict",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"impl/foo.go"},
				ReadOnly:     []string{"impl/foo.go"},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-contradict.md",
			},
		},
	}

	err := dag.Validate(d)
	if err == nil {
		t.Fatal("expected error for read_only/allowed_paths self-contradiction, got nil")
	}
	if !errors.Is(err, dag.ErrReadOnlySelfContradiction) {
		t.Fatalf("expected ErrReadOnlySelfContradiction, got: %v", err)
	}
}

func TestValidate_ReadOnlyUnfounded(t *testing.T) {
	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:           "apply-red",
				Executor:     "agy",
				RoutedBy:     "writes failing test",
				AllowedPaths: []string{"test/foo_test.go"},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-red.md",
			},
			{
				ID:           "apply-green",
				Executor:     "agy",
				RoutedBy:     "implements to make test pass",
				AllowedPaths: []string{"impl/foo.go"},
				ReadOnly:     []string{"some/path.go"},
				DependsOn:    []string{"apply-red"},
				BodyPath:     "bodies/apply-green.md",
			},
		},
	}

	err := dag.Validate(d)
	if err == nil {
		t.Fatal("expected error for unfounded read_only assertion, got nil")
	}
	if !errors.Is(err, dag.ErrReadOnlyUnfounded) {
		t.Fatalf("expected ErrReadOnlyUnfounded, got: %v", err)
	}
}

func TestValidate_EmptyReadOnlyPath(t *testing.T) {
	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:           "apply-green",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"impl/foo.go"},
				ReadOnly:     []string{"  "},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-green.md",
			},
		},
	}

	err := dag.Validate(d)
	if err == nil {
		t.Fatal("expected error for empty read_only path, got nil")
	}
	if !errors.Is(err, dag.ErrEmptyAllowedPaths) {
		t.Fatalf("expected ErrEmptyAllowedPaths, got: %v", err)
	}
}
