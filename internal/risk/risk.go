// Package risk defines a minimal deterministic risk classifier for a candidate change
// set and the mapping from risk tier to required verification.
//
// Judges via cursor-agent are NOT implemented yet; this package only declares the plan.
package risk

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
)

type Tier string

const (
	TierPassive Tier = "passive"
	TierMedium  Tier = "medium"
	TierHigh    Tier = "high"
)

// Input describes one candidate change set.
type Input struct {
	Changes []candidatechange.Change // canonical changed paths (Created/Modified/Deleted/Copied)
	// Read returns the candidate-side bytes of a path (the new content). Used only to prove passive content.
	// A nil Read or a Read error for a path that needs content proof forces HIGH.
	Read func(path string) ([]byte, error)
}

type Result struct {
	Tier    Tier
	Reasons []string // stable, sorted, machine-friendly reason codes like "path_token:auth", "process_boundary:internal/x/y.go", "deleted:internal/z.go", "classifier_error:<why>"
}

var highTokens = map[string]bool{
	"auth":     true,
	"update":   true,
	"security": true,
	"webhook":  true,
	"payments": true,
}

var sensitiveLocations = []string{
	"internal/ledger/",
	"internal/attest/",
	"internal/accept/",
	".github/workflows/",
	"Makefile",
	"go.mod",
	"go.sum",
	"lucind-checks.sh",
	".claude-plugin/",
	"plugin/claude-code/.claude-plugin/",
}

// Classify classifies the risk tier of a candidate change set.
// Rules are evaluated deterministically; reasons list every rule that fired (sorted, de-duplicated);
// the tier is the highest one reached. Failure of any kind classifies as HIGH.
func Classify(in Input) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Result{
				Tier:    TierHigh,
				Reasons: []string{"classifier_error:panic"},
			}
		}
	}()

	// Rule 1: Empty Changes => HIGH with reason classifier_error:no_changes (fail closed).
	if len(in.Changes) == 0 {
		return Result{
			Tier:    TierHigh,
			Reasons: []string{"classifier_error:no_changes"},
		}
	}

	var reasons []string
	currentTier := TierPassive

	// Rule 2: HIGH path tokens
	for _, c := range in.Changes {
		for _, p := range changePaths(c) {
			for _, tok := range splitTokens(p) {
				lower := strings.ToLower(tok)
				if highTokens[lower] {
					reasons = append(reasons, fmt.Sprintf("path_token:%s:%s", lower, p))
					elevateTier(&currentTier, TierHigh)
				}
			}
		}
	}

	// Rule 3: HIGH sensitive locations
	for _, c := range in.Changes {
		for _, p := range changePaths(c) {
			if isSensitivePath(p) {
				reasons = append(reasons, fmt.Sprintf("sensitive_path:%s", p))
				elevateTier(&currentTier, TierHigh)
			}
		}
	}

	// Rule 4: HIGH process/network/permission signals in Go sources
	for _, c := range in.Changes {
		if c.Change == candidatechange.Deleted {
			continue
		}
		path := c.Path
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			if in.Read == nil {
				reasons = append(reasons, fmt.Sprintf("classifier_error:read:%s", path))
				elevateTier(&currentTier, TierHigh)
				continue
			}
			data, err := in.Read(path)
			if err != nil {
				reasons = append(reasons, fmt.Sprintf("classifier_error:read:%s", path))
				elevateTier(&currentTier, TierHigh)
				continue
			}
			// Import literals are matched with their quotes so ordinary words such as
			// "internet" or "network" in comments and strings do not count.
			if bytes.Contains(data, []byte(`"os/exec"`)) ||
				bytes.Contains(data, []byte("exec.Command")) ||
				bytes.Contains(data, []byte(`"syscall"`)) ||
				bytes.Contains(data, []byte("syscall.")) {
				reasons = append(reasons, fmt.Sprintf("process_boundary:%s", path))
				elevateTier(&currentTier, TierHigh)
			}
			if bytes.Contains(data, []byte(`"net/http"`)) ||
				bytes.Contains(data, []byte(`"net"`)) {
				reasons = append(reasons, fmt.Sprintf("network:%s", path))
				elevateTier(&currentTier, TierHigh)
			}
			if bytes.Contains(data, []byte("os.Chmod")) ||
				bytes.Contains(data, []byte("os.Chown")) {
				reasons = append(reasons, fmt.Sprintf("permissions:%s", path))
				elevateTier(&currentTier, TierHigh)
			}
			if bytes.Contains(data, []byte("os.Remove")) ||
				bytes.Contains(data, []byte("os.RemoveAll")) {
				reasons = append(reasons, fmt.Sprintf("deletion_api:%s", path))
				elevateTier(&currentTier, TierHigh)
			}
		}
	}

	// Rule 5: MEDIUM floor for any Deleted change, active content, etc.
	for _, c := range in.Changes {
		if c.Change == candidatechange.Deleted {
			reasons = append(reasons, fmt.Sprintf("deleted:%s", c.Path))
			elevateTier(&currentTier, TierMedium)
		}
		for _, p := range changePaths(c) {
			if isActiveContentPath(p) {
				reasons = append(reasons, fmt.Sprintf("active_content:%s", p))
				elevateTier(&currentTier, TierMedium)
			}
		}
	}

	// Rule 6: PASSIVE only if EVERY change is byte-proven ordinary doc or image
	allPassive := true
	for _, c := range in.Changes {
		if c.Change == candidatechange.Deleted {
			allPassive = false
		}
		if !isPassivePath(c.Path) {
			allPassive = false
		}
		if c.SourcePath != "" && !isPassivePath(c.SourcePath) {
			allPassive = false
		}
	}

	// Prove candidate bytes for documentation or image files
	for _, c := range in.Changes {
		if c.Change == candidatechange.Deleted || !isPassivePath(c.Path) {
			continue
		}
		if in.Read == nil {
			reasons = append(reasons, "classifier_error:nil_read")
			elevateTier(&currentTier, TierHigh)
			continue
		}
		data, err := in.Read(c.Path)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("classifier_error:read:%s", c.Path))
			elevateTier(&currentTier, TierHigh)
			continue
		}
		if !provePassiveContent(c.Path, data) {
			reasons = append(reasons, fmt.Sprintf("classifier_error:unproven:%s", c.Path))
			elevateTier(&currentTier, TierHigh)
		}
	}

	if allPassive && currentTier != TierHigh {
		currentTier = TierPassive
	} else if currentTier != TierHigh {
		// A change that is neither passive nor high is MEDIUM.
		currentTier = TierMedium
	}

	return Result{
		Tier:    currentTier,
		Reasons: dedupeAndSort(reasons),
	}
}

func changePaths(c candidatechange.Change) []string {
	var paths []string
	if c.Path != "" {
		paths = append(paths, c.Path)
	}
	if c.SourcePath != "" && c.SourcePath != c.Path {
		paths = append(paths, c.SourcePath)
	}
	return paths
}

func splitTokens(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func isSensitivePath(p string) bool {
	for _, loc := range sensitiveLocations {
		if strings.HasSuffix(loc, "/") {
			if strings.HasPrefix(p, loc) || p == strings.TrimSuffix(loc, "/") {
				return true
			}
		} else {
			if p == loc {
				return true
			}
		}
	}
	return false
}

func isActiveContentPath(p string) bool {
	if p == "plugin" || strings.HasPrefix(p, "plugin/") ||
		p == ".agents" || strings.HasPrefix(p, ".agents/") ||
		p == ".claude" || strings.HasPrefix(p, ".claude/") {
		return true
	}
	base := filepath.Base(p)
	switch base {
	case "SKILL.md", "CLAUDE.md", "AGENTS.md", "GEMINI.md":
		return true
	}
	return false
}

func isPassivePath(p string) bool {
	if isActiveContentPath(p) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".md", ".txt", ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}

func provePassiveContent(path string, data []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".txt":
		return bytes.IndexByte(data, 0) < 0 && utf8.Valid(data)
	case ".png":
		return bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	case ".jpg", ".jpeg":
		return bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff})
	case ".gif":
		return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
	default:
		return false
	}
}

func elevateTier(current *Tier, next Tier) {
	rank := map[Tier]int{
		TierPassive: 0,
		TierMedium:  1,
		TierHigh:    2,
	}
	if rank[next] > rank[*current] {
		*current = next
	}
}

func dedupeAndSort(reasons []string) []string {
	if len(reasons) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(reasons))
	var unique []string
	for _, r := range reasons {
		if !seen[r] {
			seen[r] = true
			unique = append(unique, r)
		}
	}
	sort.Strings(unique)
	return unique
}
