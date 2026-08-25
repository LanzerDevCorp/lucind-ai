// Package fixture provides deterministic synthetic journey fixtures, packets,
// check scripts with seeded out-of-scope defects, tree hash calculation, and commit-ancestry
// verification for Stability Campaign trials.
package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

var (
	// ErrAncestryViolation is returned when commit ancestry invariants are not satisfied.
	ErrAncestryViolation = errors.New("fixture: ancestry violation")
	// ErrContaminatedTarget is returned when cross-target contamination is detected.
	ErrContaminatedTarget = errors.New("fixture: cross-target contamination detected")
)

// ChangeAPacket returns the structured Packet definition for Change A.
func ChangeAPacket() packet.Packet {
	return packet.Packet{
		ID:           "stability-change-a",
		Executor:     "agy",
		RoutedBy:     "stability campaign trial change A",
		Model:        "gemini-3.7-flash-high",
		AllowedPaths: []string{"fixture/change_a.txt"},
		Body:         "# Packet stability-change-a\n\n## Goal\n\nImplement Change A functionality in fixture/change_a.txt.\n",
	}
}

// ChangeAPacketMarkdown returns the raw frontmatter-delimited Markdown for Change A.
func ChangeAPacketMarkdown() string {
	return "---\n" +
		"id: stability-change-a\n" +
		"executor: agy\n" +
		"routed_by: stability campaign trial change A\n" +
		"model: gemini-3.7-flash-high\n" +
		"allowed_paths: [\"fixture/change_a.txt\"]\n" +
		"---\n\n" +
		"# Packet stability-change-a\n\n" +
		"## Goal\n\n" +
		"Implement Change A functionality in fixture/change_a.txt.\n"
}

// ChangeBPacket returns the structured Packet definition for Change B.
func ChangeBPacket() packet.Packet {
	return packet.Packet{
		ID:           "stability-change-b",
		Executor:     "agy",
		RoutedBy:     "stability campaign trial change B",
		Model:        "gemini-3.7-flash-high",
		AllowedPaths: []string{"fixture/change_b.txt"},
		Body: "# Packet stability-change-b\n\n" +
			"## Goal\n\n" +
			"Implement Change B functionality in fixture/change_b.txt.\n\n" +
			"## Required final step\n\n" +
			"After writing fixture/change_b.txt and before finishing this task, run\n" +
			"the shell command `sleep 5` and wait for it to complete. Do this as your\n" +
			"last action, after the file write, before ending the task.\n",
	}
}

// ChangeBPacketMarkdown returns the raw frontmatter-delimited Markdown for Change B.
func ChangeBPacketMarkdown() string {
	return "---\n" +
		"id: stability-change-b\n" +
		"executor: agy\n" +
		"routed_by: stability campaign trial change B\n" +
		"model: gemini-3.7-flash-high\n" +
		"allowed_paths: [\"fixture/change_b.txt\"]\n" +
		"---\n\n" +
		"# Packet stability-change-b\n\n" +
		"## Goal\n\n" +
		"Implement Change B functionality in fixture/change_b.txt.\n\n" +
		"## Required final step\n\n" +
		"After writing fixture/change_b.txt and before finishing this task, run\n" +
		"the shell command `sleep 5` and wait for it to complete. Do this as your\n" +
		"last action, after the file write, before ending the task.\n"
}

// FixChangePacket returns the structured Packet definition for the remediation Fix Change.
func FixChangePacket() packet.Packet {
	return packet.Packet{
		ID:           "stability-fix-a",
		Executor:     "agy",
		RoutedBy:     "stability campaign remediation for change A defect",
		Model:        "gemini-3.7-flash-high",
		AllowedPaths: []string{"fixture/defect.txt"},
		Body:         "# Packet stability-fix-a\n\n## Goal\n\nRemediate seeded defect in fixture/defect.txt by setting STATUS=FIXED.\n",
	}
}

// FixChangePacketMarkdown returns the raw frontmatter-delimited Markdown for the Fix Change.
func FixChangePacketMarkdown() string {
	return "---\n" +
		"id: stability-fix-a\n" +
		"executor: agy\n" +
		"routed_by: stability campaign remediation for change A defect\n" +
		"model: gemini-3.7-flash-high\n" +
		"allowed_paths: [\"fixture/defect.txt\"]\n" +
		"---\n\n" +
		"# Packet stability-fix-a\n\n" +
		"## Goal\n\n" +
		"Remediate seeded defect in fixture/defect.txt by setting STATUS=FIXED.\n"
}

// DefectContent returns the initial content of fixture/defect.txt containing the seeded defect.
func DefectContent() string {
	return "STATUS=DEFECTIVE\nERROR=E_OUT_OF_SCOPE_DEFECT\nDESCRIPTION=Seeded deterministic defect in shared stability fixture\n"
}

// ChangeAContent returns the initial content of fixture/change_a.txt.
func ChangeAContent() string {
	return "CHANGE_A=PENDING\n"
}

// ChangeBContent returns the initial content of fixture/change_b.txt.
func ChangeBContent() string {
	return "CHANGE_B=PENDING\n"
}

// CheckScriptContent returns the content of fixture/check.sh.
func CheckScriptContent() string {
	return "#!/bin/sh\n" +
		"set -e\n" +
		"TARGET=\"$1\"\n" +
		"if [ \"$TARGET\" = \"change-a\" ] || [ \"$TARGET\" = \"A\" ]; then\n" +
		"  if grep -q \"STATUS=DEFECTIVE\" fixture/defect.txt 2>/dev/null; then\n" +
		"    echo \"CHECK FAILURE: Seeded defect present in fixture/defect.txt\" >&2\n" +
		"    exit 1\n" +
		"  fi\n" +
		"  echo \"CHECK SUCCESS: Change A verified\"\n" +
		"  exit 0\n" +
		"elif [ \"$TARGET\" = \"change-b\" ] || [ \"$TARGET\" = \"B\" ]; then\n" +
		"  echo \"CHECK SUCCESS: Change B verified\"\n" +
		"  exit 0\n" +
		"else\n" +
		"  echo \"UNKNOWN TARGET: $TARGET\" >&2\n" +
		"  exit 2\n" +
		"fi\n"
}

// MaterializeFixtures initializes the standard fixture files and check script under dir/fixture.
func MaterializeFixtures(dir string) error {
	fixtureDir := filepath.Join(dir, "fixture")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		return fmt.Errorf("fixture: mkdir %s: %w", fixtureDir, err)
	}

	files := []struct {
		relPath string
		content string
		perm    os.FileMode
	}{
		{"fixture/defect.txt", DefectContent(), 0o644},
		{"fixture/change_a.txt", ChangeAContent(), 0o644},
		{"fixture/change_b.txt", ChangeBContent(), 0o644},
		{"fixture/check.sh", CheckScriptContent(), 0o755},
	}

	for _, f := range files {
		targetPath := filepath.Join(dir, filepath.FromSlash(f.relPath))
		if err := os.WriteFile(targetPath, []byte(f.content), f.perm); err != nil {
			return fmt.Errorf("fixture: write %s: %w", f.relPath, err)
		}
	}

	return nil
}

// RunCheck executes the fixture check script for a given target within worktreePath.
func RunCheck(ctx context.Context, worktreePath, target string) (string, error) {
	scriptPath := filepath.Join(worktreePath, "fixture", "check.sh")
	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath, target)
	cmd.Dir = worktreePath

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	err := cmd.Run()
	output := combined.String()
	if err != nil {
		return output, fmt.Errorf("fixture: check failed for target %q: %w", target, err)
	}
	return output, nil
}

// DefaultFixturePaths returns the default list of relative paths included in fixture tree hashing.
func DefaultFixturePaths() []string {
	return []string{
		"fixture/change_a.txt",
		"fixture/change_b.txt",
		"fixture/check.sh",
		"fixture/defect.txt",
	}
}

// ComputeFixtureTreeHash computes the deterministic SHA-256 tree hash over DefaultFixturePaths in dir.
func ComputeFixtureTreeHash(dir string) (string, error) {
	return ComputeTreeHash(dir, DefaultFixturePaths())
}

// ComputeTreeHash computes a deterministic SHA-256 content hash of relativePaths under dir.
func ComputeTreeHash(dir string, relativePaths []string) (string, error) {
	paths := make([]string, len(relativePaths))
	copy(paths, relativePaths)
	sort.Strings(paths)

	h := sha256.New()
	for _, relPath := range paths {
		fullPath := filepath.Join(dir, filepath.FromSlash(relPath))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("fixture: read file %s: %w", relPath, err)
		}
		h.Write([]byte(filepath.ToSlash(relPath)))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// IsAncestor reports whether ancestorSHA is an ancestor of descendantSHA in repoDir.
func IsAncestor(ctx context.Context, repoDir, ancestorSHA, descendantSHA string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", ancestorSHA, descendantSHA)
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("fixture: git merge-base --is-ancestor failed: %w: %s", err, strings.TrimSpace(stderr.String()))
}

// VerifyAncestryIsolation verifies Git commit ancestry isolation between target A and target B:
//  1. baseSHA must be an ancestor of targetASHA and targetBSHA.
//  2. If fixSHA is provided, baseSHA must be an ancestor of fixSHA, fixSHA must be an ancestor
//     of targetASHA, and fixSHA must NOT be an ancestor of targetBSHA (unless fixSHA == baseSHA).
//  3. targetASHA and targetBSHA must not be ancestors of each other.
//  4. The common merge base between targetASHA and targetBSHA must equal baseSHA.
func VerifyAncestryIsolation(ctx context.Context, repoDir, baseSHA, targetASHA, targetBSHA, fixSHA string) error {
	// 1. Base is ancestor of target A
	isBaseInA, err := IsAncestor(ctx, repoDir, baseSHA, targetASHA)
	if err != nil {
		return err
	}
	if !isBaseInA {
		return fmt.Errorf("%w: base %s is not an ancestor of target A %s", ErrAncestryViolation, baseSHA, targetASHA)
	}

	// 2. Base is ancestor of target B
	isBaseInB, err := IsAncestor(ctx, repoDir, baseSHA, targetBSHA)
	if err != nil {
		return err
	}
	if !isBaseInB {
		return fmt.Errorf("%w: base %s is not an ancestor of target B %s", ErrAncestryViolation, baseSHA, targetBSHA)
	}

	// 3. Fix commit ancestry checks
	if fixSHA != "" {
		isBaseInFix, err := IsAncestor(ctx, repoDir, baseSHA, fixSHA)
		if err != nil {
			return err
		}
		if !isBaseInFix {
			return fmt.Errorf("%w: base %s is not an ancestor of fix %s", ErrAncestryViolation, baseSHA, fixSHA)
		}

		isFixInA, err := IsAncestor(ctx, repoDir, fixSHA, targetASHA)
		if err != nil {
			return err
		}
		if !isFixInA {
			return fmt.Errorf("%w: fix %s is not an ancestor of target A %s", ErrAncestryViolation, fixSHA, targetASHA)
		}

		if fixSHA != baseSHA {
			isFixInB, err := IsAncestor(ctx, repoDir, fixSHA, targetBSHA)
			if err != nil {
				return err
			}
			if isFixInB {
				return fmt.Errorf("%w: target B %s contains fix %s commits", ErrContaminatedTarget, targetBSHA, fixSHA)
			}
		}
	}

	// 4. Cross-target isolation checks
	isAInB, err := IsAncestor(ctx, repoDir, targetASHA, targetBSHA)
	if err != nil {
		return err
	}
	if isAInB {
		return fmt.Errorf("%w: target B %s contains target A %s commits", ErrContaminatedTarget, targetBSHA, targetASHA)
	}

	isBInA, err := IsAncestor(ctx, repoDir, targetBSHA, targetASHA)
	if err != nil {
		return err
	}
	if isBInA {
		return fmt.Errorf("%w: target A %s contains target B %s commits", ErrContaminatedTarget, targetASHA, targetBSHA)
	}

	// 5. Unique merge base must match baseSHA
	mb, err := overlap.FindUniqueMergeBase(ctx, repoDir, targetASHA, targetBSHA)
	if err != nil {
		return err
	}
	if mb != baseSHA {
		return fmt.Errorf("%w: merge base between target A and target B is %s, want base %s", ErrAncestryViolation, mb, baseSHA)
	}

	return nil
}
