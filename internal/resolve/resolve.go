// Package resolve provides automated git merge conflict resolution using an
// LLM invoker bounded by conflict size and execution time.
package resolve

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MaxConflictLines is the maximum total lines within conflict marker blocks
// across all conflicted files that Resolve will attempt to resolve automatically.
const MaxConflictLines = 400

// Invoker runs an automated conflict resolution attempt in worktreePath with the given prompt.
type Invoker func(ctx context.Context, worktreePath, prompt string) (output string, err error)

// RealInvoker implements Invoker by executing claude headlessly with edit permissions.
var _ Invoker = RealInvoker

// RealInvoker executes claude with acceptEdits permission mode, passing the prompt via stdin.
func RealInvoker(ctx context.Context, worktreePath, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "sonnet", "--permission-mode", "acceptEdits", "--output-format", "json")
	cmd.Dir = worktreePath
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Resolve attempts to resolve all git merge conflicts in worktreePath using invoke.
// If no paths are conflicted, it returns resolved=true, "nothing to resolve", nil.
// If the total conflict marker lines across all conflicted files exceed MaxConflictLines,
// it returns resolved=false without calling invoke.
// If invoke returns an error or any conflict markers remain after invocation,
// it returns resolved=false, output, nil.
// If all conflict markers are successfully resolved, it stages the files and completes
// the merge commit, returning resolved=true, output, nil.
func Resolve(ctx context.Context, worktreePath string, invoke Invoker) (resolved bool, output string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = worktreePath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("resolve: git diff --diff-filter=U: %w: %s", err, strings.TrimSpace(string(out)))
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return true, "nothing to resolve", nil
	}

	var paths []string
	for _, line := range strings.Split(raw, "\n") {
		p := strings.TrimSpace(line)
		if p != "" {
			paths = append(paths, p)
		}
	}

	if len(paths) == 0 {
		return true, "nothing to resolve", nil
	}

	fileContents := make(map[string]string, len(paths))
	totalConflictLines := 0

	for _, p := range paths {
		filePath := filepath.Join(worktreePath, p)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return false, "", fmt.Errorf("resolve: read conflicted file %s: %w", p, err)
		}
		content := string(data)
		fileContents[p] = content
		totalConflictLines += countConflictLines(content)
	}

	if totalConflictLines > MaxConflictLines {
		return false, fmt.Sprintf("conflict exceeds %d-line bound (%d lines)", MaxConflictLines, totalConflictLines), nil
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Resolve all git merge conflict markers in the following files. You must resolve every marker in every listed file and touch no other file.\n\n")
	for _, p := range paths {
		promptBuilder.WriteString(fmt.Sprintf("File: %s\n```\n%s\n```\n\n", p, fileContents[p]))
	}
	prompt := promptBuilder.String()

	invOut, invErr := invoke(ctx, worktreePath, prompt)
	if invErr != nil {
		return false, invOut, nil
	}

	for _, p := range paths {
		filePath := filepath.Join(worktreePath, p)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return false, invOut, fmt.Errorf("resolve: re-read file %s: %w", p, err)
		}
		if hasConflictMarkers(string(data)) {
			return false, invOut, nil
		}
	}

	addArgs := append([]string{"add", "--"}, paths...)
	addCmd := exec.CommandContext(ctx, "git", addArgs...)
	addCmd.Dir = worktreePath
	if addOut, err := addCmd.CombinedOutput(); err != nil {
		return false, invOut, fmt.Errorf("resolve: git add: %w: %s", err, strings.TrimSpace(string(addOut)))
	}

	commitCmd := exec.CommandContext(ctx, "git", "commit", "--no-edit")
	commitCmd.Dir = worktreePath
	if commitOut, err := commitCmd.CombinedOutput(); err != nil {
		return false, invOut, fmt.Errorf("resolve: git commit: %w: %s", err, strings.TrimSpace(string(commitOut)))
	}

	return true, invOut, nil
}

func countConflictLines(content string) int {
	var total int
	inConflict := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if !inConflict {
			if strings.HasPrefix(line, "<<<<<<<") {
				inConflict = true
				total++
			}
		} else {
			total++
			if strings.HasPrefix(line, ">>>>>>>") {
				inConflict = false
			}
		}
	}
	return total
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
