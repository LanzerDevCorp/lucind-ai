package resolve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

// DefaultTimeout is the standard execution timeout for candidate conflict resolution.
const DefaultTimeout = 5 * time.Minute

// Sentinel errors returned by candidate resolution operations.
var (
	ErrConflictBoundExceeded = errors.New("resolve: conflict exceeds line bound")
	ErrConflictMarkersRemain = errors.New("resolve: conflict markers remain in worktree")
	ErrOutOfScopeEdits       = errors.New("resolve: edits outside declared allowed_paths")
	ErrSemanticAmbiguity     = errors.New("resolve: semantic ambiguity detected")
)

// CandidateOptions configures a candidate merge resolution run.
type CandidateOptions struct {
	WorktreePath     string
	BaseSHA          string
	AllowedPaths     []string
	Invoker          Invoker
	Timeout          time.Duration
	MaxConflictLines int
}

// CandidateOutcome captures the result of resolving candidate merge conflicts.
type CandidateOutcome struct {
	Resolved      bool
	Output        string
	FailureReason string
}

// ScanConflictMarkers scans all non-ignored, non-metadata files in worktreePath for git conflict markers.
// It returns whether markers were found and the relative paths of the offending files.
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

		// Only check regular files
		if !d.Type().IsRegular() {
			return nil
		}

		// Skip binary files or files that cannot be read
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		// Quick check for null bytes (binary)
		if bytes.IndexByte(data, 0) != -1 {
			return nil
		}

		if hasConflictMarkers(string(data)) {
			markerFiles = append(markerFiles, filepath.ToSlash(rel))
		}
		return nil
	})

	if err != nil {
		return false, nil, fmt.Errorf("resolve: scan conflict markers: %w", err)
	}

	return len(markerFiles) > 0, markerFiles, nil
}

// EnforceAllowedPaths inspects the actual git diff of the worktree against baseSHA
// using a 4-way diff union (committed-since-base, unstaged, staged, untracked).
// If any changed path falls outside allowedPaths, it returns the offending paths and ErrOutOfScopeEdits.
func EnforceAllowedPaths(ctx context.Context, worktreePath, baseSHA string, allowedPaths []string) ([]string, error) {
	if strings.TrimSpace(baseSHA) == "" {
		return nil, errors.New("resolve: missing base SHA for allowed_paths check")
	}

	var diffOut, unstagedOut, stagedOut, lsOut []byte

	diffCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "diff", "--name-status", "-z", "--diff-filter=ACDMRT", "-M", baseSHA, "HEAD")
	var diffStderr strings.Builder
	diffCmd.Stderr = &diffStderr
	if out, err := diffCmd.Output(); err == nil {
		diffOut = out
	}

	unstagedCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "diff", "--name-status", "-z", "--diff-filter=ACDMRT", "-M")
	var unstagedStderr strings.Builder
	unstagedCmd.Stderr = &unstagedStderr
	if out, err := unstagedCmd.Output(); err == nil {
		unstagedOut = out
	}

	stagedCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "diff", "--cached", "--name-status", "-z", "--diff-filter=ACDMRT", "-M")
	var stagedStderr strings.Builder
	stagedCmd.Stderr = &stagedStderr
	if out, err := stagedCmd.Output(); err == nil {
		stagedOut = out
	}

	lsCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "ls-files", "-z", "-o", "--exclude-standard")
	var lsStderr strings.Builder
	lsCmd.Stderr = &lsStderr
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

// ResolveCandidateMerge resolves merge conflicts in opts.WorktreePath using opts.Invoker,
// strictly bounded by conflict line count and timeout, and verifying conflict markers and allowed paths.
func ResolveCandidateMerge(ctx context.Context, opts CandidateOptions) (CandidateOutcome, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	maxLines := opts.MaxConflictLines
	if maxLines <= 0 {
		maxLines = MaxConflictLines
	}

	invoker := opts.Invoker
	if invoker == nil {
		invoker = RealInvoker
	}

	cmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "diff", "--name-only", "--diff-filter=U")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return CandidateOutcome{Resolved: false, FailureReason: fmt.Sprintf("git diff --diff-filter=U: %v: %s", err, strings.TrimSpace(string(out)))}, nil
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		// Nothing in unmerged state; verify markers and scope
		if hasMarkers, markerFiles, _ := ScanConflictMarkers(opts.WorktreePath); hasMarkers {
			return CandidateOutcome{
				Resolved:      false,
				FailureReason: fmt.Sprintf("conflict markers remain in worktree: %s", strings.Join(markerFiles, ", ")),
			}, nil
		}
		if offending, _ := EnforceAllowedPaths(ctx, opts.WorktreePath, opts.BaseSHA, opts.AllowedPaths); len(offending) > 0 {
			return CandidateOutcome{
				Resolved:      false,
				FailureReason: fmt.Sprintf("actual diff touched paths outside declared allowed_paths: %s", strings.Join(offending, ", ")),
			}, nil
		}
		return CandidateOutcome{Resolved: true, Output: "nothing to resolve"}, nil
	}

	var paths []string
	for _, line := range strings.Split(raw, "\n") {
		p := strings.TrimSpace(line)
		if p != "" {
			paths = append(paths, p)
		}
	}

	fileContents := make(map[string]string, len(paths))
	totalConflictLines := 0

	for _, p := range paths {
		filePath := filepath.Join(opts.WorktreePath, p)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return CandidateOutcome{Resolved: false, FailureReason: fmt.Sprintf("read conflicted file %s: %v", p, err)}, nil
		}
		content := string(data)
		fileContents[p] = content
		totalConflictLines += countConflictLines(content)
	}

	if totalConflictLines > maxLines {
		abortCmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "merge", "--abort")
		_ = abortCmd.Run()
		return CandidateOutcome{
			Resolved:      false,
			FailureReason: fmt.Sprintf("conflict exceeds %d-line bound (%d lines)", maxLines, totalConflictLines),
		}, nil
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Resolve all git merge conflict markers in the following files. You must resolve every marker in every listed file and touch no other file.\n")
	promptBuilder.WriteString("The resolver MUST NOT choose direction or invent business semantics and MUST fail closed on semantic ambiguity. Do not guess unproven business decisions.\n\n")
	if len(opts.AllowedPaths) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("Approved allowed paths: %s\n\n", strings.Join(opts.AllowedPaths, ", ")))
	}
	for _, p := range paths {
		promptBuilder.WriteString(fmt.Sprintf("File: %s\n```\n%s\n```\n\n", p, fileContents[p]))
	}
	prompt := promptBuilder.String()

	invOut, invErr := invoker(ctx, opts.WorktreePath, prompt)
	if invErr != nil {
		abortCmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "merge", "--abort")
		_ = abortCmd.Run()
		reason := invErr.Error()
		if errors.Is(invErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = fmt.Sprintf("resolver timeout exceeded (%v): %v", timeout, invErr)
		}
		if invOut != "" && !strings.Contains(reason, invOut) {
			reason = fmt.Sprintf("%s: %s", reason, strings.TrimSpace(invOut))
		}
		return CandidateOutcome{Resolved: false, Output: invOut, FailureReason: reason}, nil
	}

	// Verify conflict markers across all files in worktree
	if hasMarkers, markerFiles, _ := ScanConflictMarkers(opts.WorktreePath); hasMarkers {
		abortCmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "merge", "--abort")
		_ = abortCmd.Run()
		return CandidateOutcome{
			Resolved:      false,
			Output:        invOut,
			FailureReason: fmt.Sprintf("conflict markers remain in worktree: %s", strings.Join(markerFiles, ", ")),
		}, nil
	}

	// Verify allowed paths before staging/committing
	if offending, _ := EnforceAllowedPaths(ctx, opts.WorktreePath, opts.BaseSHA, opts.AllowedPaths); len(offending) > 0 {
		abortCmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "merge", "--abort")
		_ = abortCmd.Run()
		return CandidateOutcome{
			Resolved:      false,
			Output:        invOut,
			FailureReason: fmt.Sprintf("actual diff touched paths outside declared allowed_paths: %s", strings.Join(offending, ", ")),
		}, nil
	}

	// Stage resolved files
	addArgs := append([]string{"-C", opts.WorktreePath, "add", "--"}, paths...)
	addCmd := exec.CommandContext(ctx, "git", addArgs...)
	if addOut, err := addCmd.CombinedOutput(); err != nil {
		abortCmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "merge", "--abort")
		_ = abortCmd.Run()
		return CandidateOutcome{Resolved: false, Output: invOut, FailureReason: fmt.Sprintf("git add: %v: %s", err, strings.TrimSpace(string(addOut)))}, nil
	}

	// Complete merge commit
	commitCmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "commit", "--no-edit")
	if commitOut, err := commitCmd.CombinedOutput(); err != nil {
		abortCmd := exec.CommandContext(ctx, "git", "-C", opts.WorktreePath, "merge", "--abort")
		_ = abortCmd.Run()
		return CandidateOutcome{Resolved: false, Output: invOut, FailureReason: fmt.Sprintf("git commit: %v: %s", err, strings.TrimSpace(string(commitOut)))}, nil
	}

	// Post-commit check of allowed paths
	if offending, _ := EnforceAllowedPaths(ctx, opts.WorktreePath, opts.BaseSHA, opts.AllowedPaths); len(offending) > 0 {
		return CandidateOutcome{
			Resolved:      false,
			Output:        invOut,
			FailureReason: fmt.Sprintf("actual diff touched paths outside declared allowed_paths: %s", strings.Join(offending, ", ")),
		}, nil
	}

	return CandidateOutcome{Resolved: true, Output: invOut}, nil
}
