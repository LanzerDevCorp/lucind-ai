package dag

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrDuplicateID        = errors.New("dag: duplicate packet id")
	ErrEmptyAllowedPaths  = errors.New("dag: allowed_paths must not be empty at split time")
	ErrUnknownDependency  = errors.New("dag: depends_on references unknown packet id")
)

// Validate checks the semantic constraints on a DAG:
// 1. All packet IDs are unique.
// 2. All packets have non-empty allowed_paths (with non-empty path strings).
// 3. All depends_on references point to valid packet IDs in the DAG (and not to self).
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

	return nil
}
