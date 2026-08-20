package dag

import (
	"errors"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

var (
	ErrCycleDetected = errors.New("dag: cycle detected in dependency graph")
)

// Waves computes the execution waves of a DAG using Kahn's algorithm.
// Each wave contains packets whose dependencies were satisfied in earlier waves.
// YAML declaration order is preserved within each wave.
// After grouping, each wave is checked for pairwise disjoint allowed_paths via
// packet.DisjointAllowedPaths; same-wave path overlap returns an error.
func Waves(d DAG) ([][]Node, error) {
	if err := Validate(d); err != nil {
		return nil, err
	}

	if len(d.Packets) == 0 {
		return nil, nil
	}

	inDegree := make(map[string]int, len(d.Packets))
	dependents := make(map[string][]string, len(d.Packets))

	for _, p := range d.Packets {
		// Deduplicate dependencies to accurately compute in-degree
		uniqueDeps := make(map[string]bool, len(p.DependsOn))
		for _, dep := range p.DependsOn {
			if !uniqueDeps[dep] {
				uniqueDeps[dep] = true
				dependents[dep] = append(dependents[dep], p.ID)
			}
		}
		inDegree[p.ID] = len(uniqueDeps)
	}

	placed := make(map[string]bool, len(d.Packets))
	var waves [][]Node

	for len(placed) < len(d.Packets) {
		var currentWave []Node
		for _, p := range d.Packets {
			if !placed[p.ID] && inDegree[p.ID] == 0 {
				currentWave = append(currentWave, p)
			}
		}

		if len(currentWave) == 0 {
			return nil, ErrCycleDetected
		}

		for _, node := range currentWave {
			placed[node.ID] = true
			for _, depID := range dependents[node.ID] {
				inDegree[depID]--
			}
		}

		// Check pairwise disjoint allowed_paths within this wave
		ps := make([]packet.Packet, len(currentWave))
		for i, n := range currentWave {
			ps[i] = packet.Packet{
				ID:           n.ID,
				AllowedPaths: n.AllowedPaths,
			}
		}
		if err := packet.DisjointAllowedPaths(ps); err != nil {
			return nil, err
		}

		waves = append(waves, currentWave)
	}

	return waves, nil
}
