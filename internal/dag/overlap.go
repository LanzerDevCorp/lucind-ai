package dag

import (
	"errors"
	"fmt"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

var ErrUnorderedOverlap = errors.New("dag: overlapping allowed_paths without a depends_on path")

// reaches reports whether to is reachable from from by following DependsOn
// edges (from is an ancestor of to, i.e. following dependents).
// Unexported; consumed only by ValidateGlobalOverlap.
func reaches(dependents map[string][]string, from, to string) bool {
	visited := make(map[string]bool)
	queue := []string{from}
	visited[from] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, next := range dependents[curr] {
			if next == to {
				return true
			}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// hasOverlap reports whether two path lists have overlapping components.
func hasOverlap(pathsA, pathsB []string) bool {
	for _, pa := range pathsA {
		if packet.PathInScope(pa, pathsB) {
			return true
		}
	}
	for _, pb := range pathsB {
		if packet.PathInScope(pb, pathsA) {
			return true
		}
	}
	return false
}

// ValidateGlobalOverlap rejects any pair of packets whose allowed_paths
// overlap under packet.PathInScope unless one reaches the other.
func ValidateGlobalOverlap(d DAG) error {
	dependents := make(map[string][]string, len(d.Packets))
	for _, p := range d.Packets {
		uniqueDeps := make(map[string]bool, len(p.DependsOn))
		for _, dep := range p.DependsOn {
			if !uniqueDeps[dep] {
				uniqueDeps[dep] = true
				dependents[dep] = append(dependents[dep], p.ID)
			}
		}
	}

	for i := 0; i < len(d.Packets); i++ {
		for j := i + 1; j < len(d.Packets); j++ {
			a := d.Packets[i]
			b := d.Packets[j]

			if hasOverlap(a.AllowedPaths, b.AllowedPaths) {
				if !reaches(dependents, a.ID, b.ID) && !reaches(dependents, b.ID, a.ID) {
					return fmt.Errorf("%w: %q and %q", ErrUnorderedOverlap, a.ID, b.ID)
				}
			}
		}
	}
	return nil
}
