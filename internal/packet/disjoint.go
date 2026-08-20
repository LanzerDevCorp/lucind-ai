package packet

import (
	"fmt"
	"strings"
)

// PathInScope reports whether path falls within the scope of any entry in
// allowed, using a component-boundary prefix match rule over repo-relative
// POSIX paths. Trailing slashes are normalized so "internal/ledger" and
// "internal/ledger/" are equivalent prefixes. Non-component prefixes (e.g.
// "internal/led" vs "internal/ledger/foo.go") do not match.
func PathInScope(path string, allowed []string) bool {
	cleanPath := strings.TrimRight(path, "/")
	for _, a := range allowed {
		cleanA := strings.TrimRight(a, "/")
		if cleanPath == cleanA || strings.HasPrefix(cleanPath, cleanA+"/") {
			return true
		}
	}
	return false
}

// DisjointAllowedPaths validates that all packets in ps with declared (non-empty)
// AllowedPaths are pairwise disjoint under the component-boundary prefix match
// rule. If two packets have overlapping path scopes, an error naming both packet
// IDs is returned. Packets with empty or omitted AllowedPaths are treated as
// undeclared and skipped.
func DisjointAllowedPaths(ps []Packet) error {
	for i := 0; i < len(ps); i++ {
		if len(ps[i].AllowedPaths) == 0 {
			continue
		}
		for j := i + 1; j < len(ps); j++ {
			if len(ps[j].AllowedPaths) == 0 {
				continue
			}
			for _, pathA := range ps[i].AllowedPaths {
				for _, pathB := range ps[j].AllowedPaths {
					if PathInScope(pathB, []string{pathA}) || PathInScope(pathA, []string{pathB}) {
						return fmt.Errorf("packet: overlapping allowed_paths between %q (%s) and %q (%s)", ps[i].ID, pathA, ps[j].ID, pathB)
					}
				}
			}
		}
	}
	return nil
}
