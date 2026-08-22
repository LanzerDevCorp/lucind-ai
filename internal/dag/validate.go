package dag

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrDuplicateID               = errors.New("dag: duplicate packet id")
	ErrEmptyAllowedPaths         = errors.New("dag: allowed_paths must not be empty at split time")
	ErrUnknownDependency         = errors.New("dag: depends_on references unknown packet id")
	ErrReadOnlySelfContradiction = errors.New("dag: read_only path also appears in the packet's own allowed_paths")
	ErrReadOnlyUnfounded         = errors.New("dag: read_only path is not owned by any transitive dependency's allowed_paths")
)

// Validate checks the semantic constraints on a DAG:
//  1. All packet IDs are unique.
//  2. All packets have non-empty allowed_paths (with non-empty path strings).
//  3. All depends_on references point to valid packet IDs in the DAG (and not to self).
//  4. All read_only paths are non-empty, do not appear in the packet's own
//     allowed_paths, and are each owned by at least one transitive dependency's
//     allowed_paths.
func Validate(d DAG) error {
	seen := make(map[string]bool, len(d.Packets))
	for _, p := range d.Packets {
		if p.ID == "" {
			return ErrMissingID
		}
		if seen[p.ID] {
			return fmt.Errorf("%w: %q", ErrDuplicateID, p.ID)
		}
		seen[p.ID] = true

		if len(p.AllowedPaths) == 0 {
			return fmt.Errorf("%w for packet %q", ErrEmptyAllowedPaths, p.ID)
		}
		for _, path := range p.AllowedPaths {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("%w for packet %q: contains empty path", ErrEmptyAllowedPaths, p.ID)
			}
		}
		for _, path := range p.ReadOnly {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("%w for packet %q: contains empty read_only path", ErrEmptyAllowedPaths, p.ID)
			}
		}
	}

	for _, p := range d.Packets {
		for _, dep := range p.DependsOn {
			if dep == p.ID {
				return fmt.Errorf("dag: packet %q cannot depend on itself", p.ID)
			}
			if !seen[dep] {
				return fmt.Errorf("%w: packet %q depends on unknown %q", ErrUnknownDependency, p.ID, dep)
			}
		}
	}

	byID := make(map[string]Node, len(d.Packets))
	for _, p := range d.Packets {
		byID[p.ID] = p
	}

	for _, p := range d.Packets {
		if len(p.ReadOnly) == 0 {
			continue
		}

		ownPaths := make(map[string]bool, len(p.AllowedPaths))
		for _, path := range p.AllowedPaths {
			ownPaths[path] = true
		}

		ancestorPaths := transitiveAllowedPaths(p, byID)

		for _, path := range p.ReadOnly {
			if ownPaths[path] {
				return fmt.Errorf("%w for packet %q: %q", ErrReadOnlySelfContradiction, p.ID, path)
			}
			if !ancestorPaths[path] {
				return fmt.Errorf("%w for packet %q: %q", ErrReadOnlyUnfounded, p.ID, path)
			}
		}
	}

	return nil
}

// transitiveAllowedPaths walks p's depends_on graph transitively and returns
// the union of every ancestor's allowed_paths.
func transitiveAllowedPaths(p Node, byID map[string]Node) map[string]bool {
	result := make(map[string]bool)
	visited := make(map[string]bool)

	var walk func(id string)
	walk = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true

		node, ok := byID[id]
		if !ok {
			return
		}
		for _, path := range node.AllowedPaths {
			result[path] = true
		}
		for _, dep := range node.DependsOn {
			walk(dep)
		}
	}

	for _, dep := range p.DependsOn {
		walk(dep)
	}

	return result
}
