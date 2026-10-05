package accept

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

// Accept verifies a lane and writes the acceptance receipt.
// It returns verdict ("accepted" or "rejected"), the written receipt, reasons (if rejected), and any system error.
func Accept(ctx context.Context, repoRoot, laneID string) (string, lane.Receipt, []string, error) {
	if repoRoot == "" {
		repoRoot = "."
	}
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", lane.Receipt{}, nil, fmt.Errorf("resolve repo root: %w", err)
	}
	repoRoot = absRoot

	// 1. Load lane from .lucind/lanes/<id>/lane.json
	l, err := lane.Load(repoRoot, laneID)
	if err != nil {
		return "", lane.Receipt{}, nil, fmt.Errorf("load lane %s: %w", laneID, err)
	}

	var reasons []string

	// 2. Validate the current turn's result file against schema using result.Read
	resultFileName := lane.ResultFileName(l.Turn)
	resultRelPath := filepath.ToSlash(filepath.Join(".lucind", "lanes", laneID, resultFileName))
	env, err := result.Read(os.DirFS(repoRoot), resultRelPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			reasons = append(reasons, fmt.Sprintf("%s missing: %v", resultFileName, err))
		} else {
			reasons = append(reasons, fmt.Sprintf("%s schema-invalid: %v", resultFileName, err))
		}
	} else if env.Status != "done" {
		reasons = append(reasons, fmt.Sprintf("result status is %q, expected \"done\"", env.Status))
	}

	// 3. final_tree = tree hash now. changed_files = diff base_tree..final_tree
	finalTree, err := attest.TreeHash(ctx, repoRoot)
	if err != nil {
		return "", lane.Receipt{}, nil, fmt.Errorf("compute final tree hash: %w", err)
	}

	changedFiles := []string{}
	if l.BaseTree != finalTree {
		cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", "--name-only", l.BaseTree, finalTree)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", lane.Receipt{}, nil, fmt.Errorf("git diff %s..%s: %w: %s", l.BaseTree, finalTree, err, strings.TrimSpace(string(out)))
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				changedFiles = append(changedFiles, line)
			}
		}
	}

	// 4. Every changed file must match >= 1 allow glob in lane.Allow, EXCEPT paths under .lucind/ (or .lucind)
	for _, file := range changedFiles {
		slashPath := filepath.ToSlash(file)
		if slashPath == ".lucind" || strings.HasPrefix(slashPath, ".lucind/") {
			continue
		}
		if !lane.MatchAny(l.Allow, slashPath) {
			reasons = append(reasons, fmt.Sprintf("changed file %s not in allowlist", slashPath))
		}
	}

	// 5. Attestation check or run lane checks
	evidence := []lane.CheckEvidence{}
	for i, c := range l.Checks {
		attestCmd := lane.CheckCommand(c)
		_, attestPath, found, err := attest.FindValidAttestation(ctx, repoRoot, attestCmd, finalTree, nil, "")
		if err == nil && found {
			evidence = append(evidence, lane.CheckEvidence{
				Check:       c,
				Attestation: &attestPath,
			})
			continue
		}

		laneDir := lane.LaneDir(repoRoot, laneID)
		if err := os.MkdirAll(laneDir, 0755); err != nil {
			return "", lane.Receipt{}, nil, fmt.Errorf("create lane dir %s: %w", laneDir, err)
		}
		checkLogPath := filepath.Join(laneDir, fmt.Sprintf("check-%d.log", i))

		cmd := exec.CommandContext(ctx, "sh", "-c", c)
		cmd.Dir = repoRoot
		out, cmdErr := cmd.CombinedOutput()
		if err := os.WriteFile(checkLogPath, out, 0644); err != nil {
			return "", lane.Receipt{}, nil, fmt.Errorf("write check log %s: %w", checkLogPath, err)
		}

		evidence = append(evidence, lane.CheckEvidence{
			Check:    c,
			CheckLog: &checkLogPath,
		})

		if cmdErr != nil {
			trimmedOut := strings.TrimSpace(string(out))
			if trimmedOut == "" {
				reasons = append(reasons, fmt.Sprintf("check %q failed", c))
			} else {
				reasons = append(reasons, fmt.Sprintf("check %q failed: %s", c, trimmedOut))
			}
		}
	}

	// 6. Write receipt.json atomically via lane.WriteReceipt
	var verdict string
	if len(reasons) == 0 {
		verdict = lane.VerdictAccepted
		l.Status = lane.StatusAccepted
	} else {
		verdict = lane.VerdictRejected
		l.Status = lane.StatusRejected
	}

	now := time.Now().UTC()
	receipt := lane.Receipt{
		Version:      1,
		Lane:         laneID,
		FinalTree:    finalTree,
		BaseTree:     l.BaseTree,
		ChangedFiles: changedFiles,
		Verdict:      verdict,
		Reasons:      reasons,
		Evidence:     evidence,
		CreatedAt:    now,
	}

	if err := lane.WriteReceipt(repoRoot, laneID, receipt); err != nil {
		return verdict, receipt, reasons, fmt.Errorf("write receipt: %w", err)
	}

	l.UpdatedAt = now
	if err := lane.Save(repoRoot, l); err != nil {
		return verdict, receipt, reasons, fmt.Errorf("save lane: %w", err)
	}

	return verdict, receipt, reasons, nil
}

// Run executes the accept workflow for a lane and writes human-readable status to stdout/stderr.
// It returns 0 on accept, 1 on reject or error.
func Run(ctx context.Context, repoRoot, laneID string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	verdict, _, reasons, err := Accept(ctx, repoRoot, laneID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "accept: %v\n", err)
		return 1
	}

	if verdict == lane.VerdictAccepted {
		_, _ = fmt.Fprintf(stdout, "lane %s accepted\n", laneID)
		return 0
	}

	_, _ = fmt.Fprintf(stderr, "lane %s rejected\n", laneID)
	for _, r := range reasons {
		_, _ = fmt.Fprintf(stderr, "  - %s\n", r)
	}
	return 1
}
