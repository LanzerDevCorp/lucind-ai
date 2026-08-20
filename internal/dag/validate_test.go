package dag_test

import (
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
