// Package fixture generates a deterministic three-hunk overlap toy that
// forces overlap.ClassRequired through existing DefaultThresholds.
package fixture

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

// ToyPath is the repo-relative path of the three-hunk conflicting file.
const ToyPath = "toy.go"

// GeneratorOptions configures GenerateFixture.
type GeneratorOptions struct {
	RepoRoot   string
	Ledger     *ledger.Ledger
	FeatureAID string
	FeatureBID string
	ParentRefA string
	ParentRefB string
	// SharedBase, when true, registers one base_sha on both features and
	// writes a common history. When false, features get divergent base_sha
	// values and unrelated git histories so Evaluate with an empty common
	// base returns ErrNoMergeBase instead of ClassRequired.
	SharedBase bool
}

// Fixture is the generated repo, registered features, and tip SHAs.
type Fixture struct {
	RepoRoot    string
	BaseSHA     string
	FeatureA    feature.Feature
	FeatureB    feature.Feature
	FeatureASHA string
	FeatureBSHA string
	ToyPath     string
}

// GenerateFixture writes two leased features and a three-hunk toy file
// (business, slice-literal union, rename-versus-edit).
func GenerateFixture(ctx context.Context, opts GeneratorOptions) (*Fixture, error) {
	if opts.RepoRoot == "" {
		return nil, fmt.Errorf("fixture: RepoRoot is required")
	}
	if opts.Ledger == nil {
		return nil, fmt.Errorf("fixture: Ledger is required")
	}
	if opts.FeatureAID == "" {
		opts.FeatureAID = "feat-conflict-a"
	}
	if opts.FeatureBID == "" {
		opts.FeatureBID = "feat-conflict-b"
	}
	if opts.ParentRefA == "" {
		opts.ParentRefA = "refs/heads/feature-conflict-a"
	}
	if opts.ParentRefB == "" {
		opts.ParentRefB = "refs/heads/feature-conflict-b"
	}

	if err := gitInit(opts.RepoRoot); err != nil {
		return nil, err
	}

	if opts.SharedBase {
		return generateShared(ctx, opts)
	}
	return generateDivergent(ctx, opts)
}

func generateShared(ctx context.Context, opts GeneratorOptions) (*Fixture, error) {
	if err := os.WriteFile(filepath.Join(opts.RepoRoot, ToyPath), []byte(toyBase), 0o644); err != nil {
		return nil, fmt.Errorf("fixture: write base toy: %w", err)
	}
	if err := gitRun(opts.RepoRoot, "add", ToyPath); err != nil {
		return nil, err
	}
	if err := gitRun(opts.RepoRoot, "commit", "-m", "fixture base"); err != nil {
		return nil, err
	}
	baseSHA, err := gitOut(opts.RepoRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	branchA := strings.TrimPrefix(opts.ParentRefA, "refs/heads/")
	branchB := strings.TrimPrefix(opts.ParentRefB, "refs/heads/")

	if err := gitRun(opts.RepoRoot, "checkout", "-b", branchA); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(opts.RepoRoot, ToyPath), []byte(toyA), 0o644); err != nil {
		return nil, fmt.Errorf("fixture: write feature A toy: %w", err)
	}
	if err := gitRun(opts.RepoRoot, "add", ToyPath); err != nil {
		return nil, err
	}
	if err := gitRun(opts.RepoRoot, "commit", "-m", "feature A three hunks"); err != nil {
		return nil, err
	}
	shaA, err := gitOut(opts.RepoRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	if err := gitRun(opts.RepoRoot, "checkout", "-B", branchB, baseSHA); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(opts.RepoRoot, ToyPath), []byte(toyB), 0o644); err != nil {
		return nil, fmt.Errorf("fixture: write feature B toy: %w", err)
	}
	if err := gitRun(opts.RepoRoot, "add", ToyPath); err != nil {
		return nil, err
	}
	if err := gitRun(opts.RepoRoot, "commit", "-m", "feature B three hunks"); err != nil {
		return nil, err
	}
	shaB, err := gitOut(opts.RepoRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	return registerFeatures(ctx, opts, baseSHA, baseSHA, shaA, shaB)
}

func generateDivergent(ctx context.Context, opts GeneratorOptions) (*Fixture, error) {
	branchA := strings.TrimPrefix(opts.ParentRefA, "refs/heads/")
	branchB := strings.TrimPrefix(opts.ParentRefB, "refs/heads/")

	if err := gitRun(opts.RepoRoot, "checkout", "--orphan", branchA); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(opts.RepoRoot, ToyPath), []byte(toyA), 0o644); err != nil {
		return nil, fmt.Errorf("fixture: write orphan A toy: %w", err)
	}
	if err := gitRun(opts.RepoRoot, "add", ToyPath); err != nil {
		return nil, err
	}
	if err := gitRun(opts.RepoRoot, "commit", "-m", "orphan A"); err != nil {
		return nil, err
	}
	shaA, err := gitOut(opts.RepoRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	if err := gitRun(opts.RepoRoot, "checkout", "--orphan", branchB); err != nil {
		return nil, err
	}
	_ = gitRun(opts.RepoRoot, "rm", "-f", ToyPath)
	if err := os.WriteFile(filepath.Join(opts.RepoRoot, ToyPath), []byte(toyB), 0o644); err != nil {
		return nil, fmt.Errorf("fixture: write orphan B toy: %w", err)
	}
	if err := gitRun(opts.RepoRoot, "add", ToyPath); err != nil {
		return nil, err
	}
	if err := gitRun(opts.RepoRoot, "commit", "-m", "orphan B"); err != nil {
		return nil, err
	}
	shaB, err := gitOut(opts.RepoRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	return registerFeatures(ctx, opts, shaA, shaB, shaA, shaB)
}

func registerFeatures(ctx context.Context, opts GeneratorOptions, baseA, baseB, shaA, shaB string) (*Fixture, error) {
	svc := feature.NewService(opts.Ledger)
	featA, err := svc.Create(ctx, opts.FeatureAID, opts.ParentRefA, baseA, shaA)
	if err != nil {
		return nil, fmt.Errorf("fixture: create feature A: %w", err)
	}
	featB, err := svc.Create(ctx, opts.FeatureBID, opts.ParentRefB, baseB, shaB)
	if err != nil {
		return nil, fmt.Errorf("fixture: create feature B: %w", err)
	}

	shared := ""
	if baseA == baseB {
		shared = baseA
	}
	return &Fixture{
		RepoRoot:    opts.RepoRoot,
		BaseSHA:     shared,
		FeatureA:    featA,
		FeatureB:    featB,
		FeatureASHA: shaA,
		FeatureBSHA: shaB,
		ToyPath:     ToyPath,
	}, nil
}

func gitInit(dir string) error {
	if err := gitRun(dir, "init", "-b", "main"); err != nil {
		return err
	}
	if err := gitRun(dir, "config", "user.email", "fixture@lucind.ai"); err != nil {
		return err
	}
	return gitRun(dir, "config", "user.name", "fixture")
}

func gitRun(dir string, args ...string) error {
	_, err := gitOut(dir, args...)
	return err
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{
		"-c", "user.email=fixture@lucind.ai",
		"-c", "user.name=fixture",
	}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("fixture: git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func pad(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "// pad-%02d keeps hunks farther apart than NearbyHunkLines\n", i)
	}
	return b.String()
}

var (
	toyBase = "package toy\n\n" +
		"func Price() int {\n\treturn 10\n}\n\n" +
		pad(25) +
		"var Tags = []string{\n\t\"keep\",\n}\n\n" +
		pad(25) +
		"func Helper() string {\n\treturn \"base\"\n}\n"

	toyA = "package toy\n\n" +
		"func Price() int {\n\treturn 100 // tier A\n}\n\n" +
		pad(25) +
		"var Tags = []string{\n\t\"keep\",\n\t\"from-a\",\n}\n\n" +
		pad(25) +
		"func HelperRenamed() string {\n\treturn \"base\"\n}\n"

	toyB = "package toy\n\n" +
		"func Price() int {\n\treturn 200 // enterprise\n}\n\n" +
		pad(25) +
		"var Tags = []string{\n\t\"keep\",\n\t\"from-b\",\n}\n\n" +
		pad(25) +
		"func Helper() string {\n\treturn \"edited-by-b\"\n}\n"
)
