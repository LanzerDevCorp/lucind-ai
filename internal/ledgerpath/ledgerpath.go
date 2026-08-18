// Package ledgerpath is pure path logic for locating the single lane ledger
// database. It resolves and validates the ledger path against a primary
// repository root, and never touches git or the filesystem.
//
// Resolve is wired into internal/ledger.Open, which closes the "Single
// ledger location" requirement's first scenario: Open takes a primary
// repository root, not a database path, so the database always lands under
// "<primaryRoot>/.lucind/". Validate remains unwired: rejecting a candidate
// that lives inside a lane's worktree instead of the primary repository
// (the requirement's second scenario) needs distinguishing a worktree from
// a repository root, which needs git awareness this slice does not have.
// That is explicitly deferred to the dispatch slice (see design-corrections
// #1714 §correction 2 and tasks #1715 task 4.3) — it is not silently
// treated as closed here.
package ledgerpath

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrLedgerOutsidePrimaryRepo is returned by Validate when a candidate path
// does not live under the primary repository root's .lucind directory —
// for example, a worktree-shaped path such as
// "<repo>-worktrees/<lane>/.lucind/lucind.db".
var ErrLedgerOutsidePrimaryRepo = errors.New("ledgerpath: candidate is outside the primary repository's .lucind directory")

const (
	ledgerDirName  = ".lucind"
	ledgerFileName = "lucind.db"
)

// Resolve returns the lane ledger database path for the given primary
// repository root: "<primaryRoot>/.lucind/lucind.db".
func Resolve(primaryRoot string) string {
	return filepath.Join(primaryRoot, ledgerDirName, ledgerFileName)
}

// Validate reports whether candidate is a path under the primary repository
// root's .lucind directory. It rejects any candidate outside that
// directory, including a worktree's own .lucind path, by returning
// ErrLedgerOutsidePrimaryRepo.
func Validate(candidate, primaryRoot string) error {
	root := filepath.Join(primaryRoot, ledgerDirName)

	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return ErrLedgerOutsidePrimaryRepo
	}
	// rel == "." means candidate is the .lucind directory itself (no file
	// segment) — not a valid ledger path. rel == ".." or a "../" prefix
	// means candidate escapes .lucind entirely.
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ErrLedgerOutsidePrimaryRepo
	}

	return nil
}
