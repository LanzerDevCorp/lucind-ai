package integrate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

var (
	ErrConflictMarkersRemain = errors.New("integrate: conflict markers remain in worktree")
	ErrOutOfScopeEdits       = errors.New("integrate: edits outside declared allowed_paths")
	ErrSemanticAmbiguity     = errors.New("integrate: semantic ambiguity detected")
)

// ScanConflictMarkers scans all non-ignored, non-metadata files in worktreePath for git conflict markers.
func ScanConflictMarkers(worktreePath string) (bool, []string, error) {
	var markerFiles []string

	err := filepath.WalkDir(worktreePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(worktreePath, path)
		if relErr != nil {
			return relErr
		}

		if d.IsDir() {
			if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) ||
				rel == ".lucind" || strings.HasPrefix(rel, ".lucind"+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		if bytes.IndexByte(data, 0) != -1 {
			return nil
		}

		if hasConflictMarkers(string(data)) {
			markerFiles = append(markerFiles, filepath.ToSlash(rel))
		}
		return nil
	})

	if err != nil {
		return false, nil, fmt.Errorf("integrate: scan conflict markers: %w", err)
	}

	return len(markerFiles) > 0, markerFiles, nil
}

func hasConflictMarkers(content string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "=======" || strings.HasPrefix(line, "<<<<<<<") || strings.HasPrefix(line, ">>>>>>>") {
			return true
		}
	}
	return false
}

// EnforceAllowedPaths inspects the actual git diff of the worktree against baseSHA.
func EnforceAllowedPaths(ctx context.Context, worktreePath, baseSHA string, allowedPaths []string) ([]string, error) {
	if strings.TrimSpace(baseSHA) == "" {
		return nil, errors.New("integrate: missing base SHA for allowed_paths check")
	}

	var diffOut, unstagedOut, stagedOut, lsOut []byte

	diffCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "diff", "--name-status", "-z", "--diff-filter=ACDMRT", "-M", baseSHA, "HEAD")
	if out, err := diffCmd.Output(); err == nil {
		diffOut = out
	}

	unstagedCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "diff", "--name-status", "-z", "--diff-filter=ACDMRT", "-M")
	if out, err := unstagedCmd.Output(); err == nil {
		unstagedOut = out
	}

	stagedCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "diff", "--cached", "--name-status", "-z", "--diff-filter=ACDMRT", "-M")
	if out, err := stagedCmd.Output(); err == nil {
		stagedOut = out
	}

	lsCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "ls-files", "-z", "-o", "--exclude-standard")
	if out, err := lsCmd.Output(); err == nil {
		lsOut = out
	}

	seen := make(map[string]bool)
	var changedPaths []string

	addPaths := func(paths []string) {
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			if strings.HasPrefix(path, ".lucind/") || path == ".lucind" {
				continue
			}
			if !seen[path] {
				seen[path] = true
				changedPaths = append(changedPaths, path)
			}
		}
	}

	addPaths(parseDiffNameStatusZ(diffOut))
	addPaths(parseDiffNameStatusZ(unstagedOut))
	addPaths(parseDiffNameStatusZ(stagedOut))
	addPaths(parseLSFilesZ(lsOut))

	var offending []string
	for _, path := range changedPaths {
		if !packet.PathInScope(path, allowedPaths) {
			offending = append(offending, path)
		}
	}

	if len(offending) > 0 {
		return offending, fmt.Errorf("%w: %s", ErrOutOfScopeEdits, strings.Join(offending, ", "))
	}

	return nil, nil
}

func parseDiffNameStatusZ(output []byte) []string {
	if len(output) == 0 {
		return nil
	}

	tokens := bytes.Split(output, []byte{0})
	var paths []string

	i := 0
	for i < len(tokens) {
		token := string(tokens[i])
		if token == "" {
			i++
			continue
		}

		status := token
		i++

		if len(status) > 0 && (status[0] == 'R' || status[0] == 'C') {
			if i < len(tokens) && len(tokens[i]) > 0 {
				paths = append(paths, string(tokens[i]))
				i++
			}
			if i < len(tokens) && len(tokens[i]) > 0 {
				paths = append(paths, string(tokens[i]))
				i++
			}
		} else {
			if i < len(tokens) && len(tokens[i]) > 0 {
				paths = append(paths, string(tokens[i]))
				i++
			}
		}
	}

	return paths
}

func parseLSFilesZ(output []byte) []string {
	if len(output) == 0 {
		return nil
	}

	tokens := bytes.Split(output, []byte{0})
	var paths []string
	for _, tok := range tokens {
		if len(tok) > 0 {
			paths = append(paths, string(tok))
		}
	}
	return paths
}
