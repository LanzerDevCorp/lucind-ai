package dag_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dag"
)

func TestWaves_CycleDetected(t *testing.T) {
	d := dag.DAG{
		Change: "test-cycle",
		Packets: []dag.Node{
			{
				ID:           "A",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/a/"},
				DependsOn:    []string{"B"},
				BodyPath:     "bodies/a.md",
			},
			{
				ID:           "B",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/b/"},
				DependsOn:    []string{"A"},
				BodyPath:     "bodies/b.md",
			},
		},
	}

	_, err := dag.Waves(d)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cycle") {
		t.Errorf("expected error message to mention cycle, got: %v", err)
	}
}

func TestWaves_OrderingAndYAMLOrderPreserved(t *testing.T) {
	// B depends on A, C depends on neither, A disjoint from C
	d := dag.DAG{
		Change: "test-ordering",
		Packets: []dag.Node{
			{
				ID:           "A",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/a/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/a.md",
			},
			{
				ID:           "B",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/b/"},
				DependsOn:    []string{"A"},
				BodyPath:     "bodies/b.md",
			},
			{
				ID:           "C",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/c/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/c.md",
			},
		},
	}

	waves, err := dag.Waves(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}

	// Wave 1 should contain A and C (in YAML order)
	if len(waves[0]) != 2 || waves[0][0].ID != "A" || waves[0][1].ID != "C" {
		t.Errorf("expected wave 1 to be [A, C], got: %v", packetIDs(waves[0]))
	}

	// Wave 2 should contain B
	if len(waves[1]) != 1 || waves[1][0].ID != "B" {
		t.Errorf("expected wave 2 to be [B], got: %v", packetIDs(waves[1]))
	}
}

func TestWaves_SameWaveOverlapRejected(t *testing.T) {
	// No depends_on edge, internal/foo/ vs internal/foo/bar.go -> error
	d := dag.DAG{
		Change: "test-overlap",
		Packets: []dag.Node{
			{
				ID:           "P1",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/foo/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/p1.md",
			},
			{
				ID:           "P2",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/foo/bar.go"},
				DependsOn:    []string{},
				BodyPath:     "bodies/p2.md",
			},
		},
	}

	_, err := dag.Waves(d)
	if err == nil {
		t.Fatal("expected error for same-wave path overlap, got nil")
	}
}

func TestWaves_SameWaveSiblingAccepted(t *testing.T) {
	// No edge, internal/foo/ vs internal/bar/ -> same wave
	d := dag.DAG{
		Change: "test-sibling",
		Packets: []dag.Node{
			{
				ID:           "P1",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/foo/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/p1.md",
			},
			{
				ID:           "P2",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/bar/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/p2.md",
			},
		},
	}

	waves, err := dag.Waves(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if len(waves[0]) != 2 {
		t.Fatalf("expected 2 packets in wave 0, got %d", len(waves[0]))
	}
}

func TestWaves_SameWaveComponentBoundary(t *testing.T) {
	// No edge, internal/led vs internal/ledger/foo.go -> treated disjoint, same wave
	d := dag.DAG{
		Change: "test-component-boundary",
		Packets: []dag.Node{
			{
				ID:           "P1",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/led"},
				DependsOn:    []string{},
				BodyPath:     "bodies/p1.md",
			},
			{
				ID:           "P2",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/ledger/foo.go"},
				DependsOn:    []string{},
				BodyPath:     "bodies/p2.md",
			},
		},
	}

	waves, err := dag.Waves(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if len(waves[0]) != 2 {
		t.Fatalf("expected 2 packets in wave 0, got %d", len(waves[0]))
	}
}

func TestWaves_CrossWaveOverlapAllowedWithEdge(t *testing.T) {
	// B depends_on: [A], overlapping allowed_paths -> two waves, A before B
	d := dag.DAG{
		Change: "test-cross-wave-overlap",
		Packets: []dag.Node{
			{
				ID:           "A",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/foo/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/a.md",
			},
			{
				ID:           "B",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/foo/bar.go"},
				DependsOn:    []string{"A"},
				BodyPath:     "bodies/b.md",
			},
		},
	}

	waves, err := dag.Waves(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if len(waves[0]) != 1 || waves[0][0].ID != "A" {
		t.Errorf("expected wave 0 to have [A], got: %v", packetIDs(waves[0]))
	}
	if len(waves[1]) != 1 || waves[1][0].ID != "B" {
		t.Errorf("expected wave 1 to have [B], got: %v", packetIDs(waves[1]))
	}
}

func packetIDs(nodes []dag.Node) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

func TestWaves_CrossWaveOverlapWithoutEdgeRejected(t *testing.T) {
	tests := []struct {
		name    string
		dag     dag.DAG
		wantIDs []string
	}{
		{
			name: "unordered overlap across unrelated waves rejected (3 packets)",
			dag: dag.DAG{
				Change: "test-cross-wave-unordered",
				Packets: []dag.Node{
					{
						ID:           "A",
						Executor:     "agy",
						RoutedBy:     "test",
						AllowedPaths: []string{"internal/foo/"},
						DependsOn:    []string{},
						BodyPath:     "bodies/a.md",
					},
					{
						ID:           "B",
						Executor:     "agy",
						RoutedBy:     "test",
						AllowedPaths: []string{"internal/bar/"},
						DependsOn:    []string{},
						BodyPath:     "bodies/b.md",
					},
					{
						ID:           "C",
						Executor:     "agy",
						RoutedBy:     "test",
						AllowedPaths: []string{"internal/foo/bar.go"},
						DependsOn:    []string{"B"},
						BodyPath:     "bodies/c.md",
					},
				},
			},
			wantIDs: []string{"A", "C"},
		},
		{
			name: "diamond with overlap between independent branches (4 packets)",
			dag: dag.DAG{
				Change: "test-diamond-overlap",
				Packets: []dag.Node{
					{
						ID:           "A",
						Executor:     "agy",
						RoutedBy:     "test",
						AllowedPaths: []string{"internal/a/"},
						DependsOn:    []string{},
						BodyPath:     "bodies/a.md",
					},
					{
						ID:           "B",
						Executor:     "agy",
						RoutedBy:     "test",
						AllowedPaths: []string{"internal/b/"},
						DependsOn:    []string{},
						BodyPath:     "bodies/b.md",
					},
					{
						ID:           "C",
						Executor:     "agy",
						RoutedBy:     "test",
						AllowedPaths: []string{"internal/common/"},
						DependsOn:    []string{"A"},
						BodyPath:     "bodies/c.md",
					},
					{
						ID:           "D",
						Executor:     "agy",
						RoutedBy:     "test",
						AllowedPaths: []string{"internal/common/sub.go"},
						DependsOn:    []string{"B"},
						BodyPath:     "bodies/d.md",
					},
				},
			},
			wantIDs: []string{"C", "D"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dag.Waves(tt.dag)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, dag.ErrUnorderedOverlap) {
				t.Fatalf("expected ErrUnorderedOverlap, got: %v", err)
			}
			for _, id := range tt.wantIDs {
				if !strings.Contains(err.Error(), id) {
					t.Errorf("expected error message to contain packet ID %q, got: %v", id, err)
				}
			}
		})
	}
}

func TestWaves_CrossWaveOverlapAllowedWithTransitiveEdge(t *testing.T) {
	// A -> B -> C: C depends on B, B depends on A. A and C overlap on internal/foo/
	d := dag.DAG{
		Change: "test-transitive-overlap",
		Packets: []dag.Node{
			{
				ID:           "A",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/foo/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/a.md",
			},
			{
				ID:           "B",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/bar/"},
				DependsOn:    []string{"A"},
				BodyPath:     "bodies/b.md",
			},
			{
				ID:           "C",
				Executor:     "agy",
				RoutedBy:     "test",
				AllowedPaths: []string{"internal/foo/bar.go"},
				DependsOn:    []string{"B"},
				BodyPath:     "bodies/c.md",
			},
		},
	}

	waves, err := dag.Waves(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(waves))
	}
	if len(waves[0]) != 1 || waves[0][0].ID != "A" {
		t.Errorf("expected wave 0 to have [A], got: %v", packetIDs(waves[0]))
	}
	if len(waves[1]) != 1 || waves[1][0].ID != "B" {
		t.Errorf("expected wave 1 to have [B], got: %v", packetIDs(waves[1]))
	}
	if len(waves[2]) != 1 || waves[2][0].ID != "C" {
		t.Errorf("expected wave 2 to have [C], got: %v", packetIDs(waves[2]))
	}
}

