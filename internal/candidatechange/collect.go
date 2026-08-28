// Package candidatechange derives the canonical changed-path evidence shared by
// scope enforcement, result validation, and Acceptance.
package candidatechange

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Kind string

const (
	Created  Kind = "created"
	Modified Kind = "modified"
	Deleted  Kind = "deleted"
	Copied   Kind = "copied"
)

// Change is canonical JSON evidence. Copies retain both endpoints; renames are
// represented as a deleted source and created destination.
type Change struct {
	Change     Kind   `json:"change"`
	SourcePath string `json:"source_path,omitempty"`
	Path       string `json:"path"`
}

type Request struct {
	Root, BaseCommit, CandidateCommit string
	IncludeWorktree                   bool
}

// Collect executes Git only through argv and returns deterministic, unique
// committed changes plus optional staged, unstaged, and untracked state.
func Collect(ctx context.Context, request Request) ([]Change, error) {
	root, err := canonicalRoot(ctx, request.Root)
	if err != nil {
		return nil, err
	}
	if request.BaseCommit == "" || request.CandidateCommit == "" {
		return nil, errors.New("candidatechange: base and candidate commits are required")
	}
	args := []string{"diff", "--name-status", "-z", "-M", "-C", "--find-copies-harder", "--diff-filter=ACDMRT", request.BaseCommit, request.CandidateCommit, "--"}
	out, err := git(ctx, root, args...)
	if err != nil {
		return nil, err
	}
	changes, err := parse(out)
	if err != nil {
		return nil, err
	}
	if request.IncludeWorktree {
		for _, extra := range [][]string{
			{"diff", "--name-status", "-z", "-M", "-C", "--find-copies-harder", "--diff-filter=ACDMRT", "--"},
			{"diff", "--cached", "--name-status", "-z", "-M", "-C", "--find-copies-harder", "--diff-filter=ACDMRT", request.CandidateCommit, "--"},
		} {
			out, err = git(ctx, root, extra...)
			if err != nil {
				return nil, err
			}
			parsed, parseErr := parse(out)
			if parseErr != nil {
				return nil, parseErr
			}
			changes = append(changes, parsed...)
		}
		out, err = git(ctx, root, "ls-files", "-z", "-o", "--exclude-standard")
		if err != nil {
			return nil, err
		}
		for _, token := range bytes.Split(out, []byte{0}) {
			if len(token) > 0 {
				changes = append(changes, Change{Change: Created, Path: string(token)})
			}
		}
	}
	return canonical(changes), nil
}

// OutOfScope checks every independently authoritative endpoint.
func OutOfScope(changes []Change, allowed []string) []string {
	set := map[string]struct{}{}
	for _, change := range changes {
		for _, path := range []string{change.SourcePath, change.Path} {
			if path != "" && !inScope(path, allowed) {
				set[path] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func canonicalRoot(ctx context.Context, value string) (string, error) {
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("candidatechange: absolute root: %w", err)
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("candidatechange: resolve root: %w", err)
	}
	top, err := git(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	resolvedTop, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil || filepath.Clean(resolvedTop) != filepath.Clean(root) {
		return "", errors.New("candidatechange: selector is not the canonical repository root")
	}
	return filepath.Clean(root), nil
}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("candidatechange: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func parse(out []byte) ([]Change, error) {
	tokens := bytes.Split(out, []byte{0})
	changes := make([]Change, 0, len(tokens)/2)
	for i := 0; i < len(tokens) && len(tokens[i]) > 0; {
		status := string(tokens[i])
		i++
		if status == "" || i >= len(tokens) {
			return nil, errors.New("candidatechange: malformed name-status output")
		}
		kind := status[0]
		if kind == 'R' || kind == 'C' {
			if i+1 >= len(tokens) || len(tokens[i]) == 0 || len(tokens[i+1]) == 0 {
				return nil, errors.New("candidatechange: malformed rename or copy")
			}
			source, destination := string(tokens[i]), string(tokens[i+1])
			i += 2
			if kind == 'R' {
				changes = append(changes, Change{Change: Deleted, Path: source}, Change{Change: Created, Path: destination})
			} else {
				changes = append(changes, Change{Change: Copied, SourcePath: source, Path: destination})
			}
			continue
		}
		path := string(tokens[i])
		i++
		mapped := map[byte]Kind{'A': Created, 'M': Modified, 'T': Modified, 'D': Deleted}[kind]
		if mapped == "" {
			return nil, fmt.Errorf("candidatechange: unsupported status %q", status)
		}
		changes = append(changes, Change{Change: mapped, Path: path})
	}
	return changes, nil
}

func canonical(changes []Change) []Change {
	set := make(map[Change]struct{}, len(changes))
	for _, change := range changes {
		if change.Path == ".lucind" || strings.HasPrefix(change.Path, ".lucind/") || change.SourcePath == ".lucind" || strings.HasPrefix(change.SourcePath, ".lucind/") {
			continue
		}
		set[change] = struct{}{}
	}
	out := make([]Change, 0, len(set))
	for change := range set {
		out = append(out, change)
	}
	rank := map[Kind]int{Created: 0, Modified: 1, Deleted: 2, Copied: 3}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if rank[out[i].Change] != rank[out[j].Change] {
			return rank[out[i].Change] < rank[out[j].Change]
		}
		return out[i].SourcePath < out[j].SourcePath
	})
	return out
}

func inScope(path string, allowed []string) bool {
	path = strings.TrimRight(path, "/")
	for _, prefix := range allowed {
		prefix = strings.TrimRight(prefix, "/")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
