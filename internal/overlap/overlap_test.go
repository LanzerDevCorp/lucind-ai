package overlap

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper to run git commands in tests
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\nOutput: %s", strings.Join(args, " "), dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test Runner")
	gitRun(t, dir, "config", "user.email", "test@lucind.ai")
	return dir
}

func TestCaptureRaw_PredictableDiff(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	// Base commit with base.txt and common.txt
	err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("line 1\nline 2\nline 3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(repoDir, "common.txt"), []byte("common line\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "base commit")
	baseSHA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featA: modifies base.txt line 1, adds onlyA.txt
	gitRun(t, repoDir, "checkout", "-b", "featA")
	err = os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("line 1 edited by A\nline 2\nline 3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(repoDir, "onlyA.txt"), []byte("alpha\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featA commit")
	shaA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featB (off baseSHA): modifies base.txt line 1 (conflicting with A), adds onlyB.txt
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "featB")
	err = os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("line 1 edited by B\nline 2\nline 3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(repoDir, "onlyB.txt"), []byte("beta\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featB commit")
	shaB := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featC (off baseSHA): clean merge with featA (modifies common.txt only)
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "featC")
	err = os.WriteFile(filepath.Join(repoDir, "common.txt"), []byte("common line edited by C\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featC commit")
	shaC := gitRun(t, repoDir, "rev-parse", "HEAD")

	// 1. Verify FindUniqueMergeBase
	mb, err := FindUniqueMergeBase(ctx, repoDir, shaA, shaB)
	if err != nil {
		t.Fatalf("FindUniqueMergeBase failed: %v", err)
	}
	if mb != baseSHA {
		t.Fatalf("FindUniqueMergeBase = %s, want %s", mb, baseSHA)
	}

	// 2. Capture raw diff between A and B (conflicting)
	rawAB, err := CaptureRaw(ctx, repoDir, baseSHA, shaA, shaB)
	if err != nil {
		t.Fatalf("CaptureRaw AB failed: %v", err)
	}

	if rawAB.BaseSHA != baseSHA || rawAB.FeatureASHA != shaA || rawAB.FeatureBSHA != shaB {
		t.Errorf("SHAs mismatch: got base=%s a=%s b=%s, want %s %s %s",
			rawAB.BaseSHA, rawAB.FeatureASHA, rawAB.FeatureBSHA, baseSHA, shaA, shaB)
	}

	// Check name-status -z
	if !strings.Contains(rawAB.NameStatusA, "base.txt") || !strings.Contains(rawAB.NameStatusA, "onlyA.txt") {
		t.Errorf("NameStatusA missing expected files: %q", rawAB.NameStatusA)
	}
	if !strings.Contains(rawAB.NameStatusB, "base.txt") || !strings.Contains(rawAB.NameStatusB, "onlyB.txt") {
		t.Errorf("NameStatusB missing expected files: %q", rawAB.NameStatusB)
	}

	// Check numstat -z
	if !strings.Contains(rawAB.NumstatA, "base.txt") || !strings.Contains(rawAB.NumstatA, "onlyA.txt") {
		t.Errorf("NumstatA missing expected files: %q", rawAB.NumstatA)
	}

	// Check zero-context diff hunks
	if !strings.Contains(rawAB.DiffU0A, "@@ -1") || !strings.Contains(rawAB.DiffU0A, "+line 1 edited by A") {
		t.Errorf("DiffU0A missing expected hunk: %s", rawAB.DiffU0A)
	}

	// Check merge-tree conflict
	if !rawAB.MergeTreeConflict {
		t.Errorf("expected MergeTreeConflict=true for conflicting branches AB")
	}
	if !strings.Contains(rawAB.MergeTreeOut, "CONFLICT") && !strings.Contains(rawAB.MergeTreeOut, "base.txt") {
		t.Errorf("expected MergeTreeOut to mention conflict/base.txt, got: %s", rawAB.MergeTreeOut)
	}

	// 3. Capture raw diff between A and C (clean merge)
	rawAC, err := CaptureRaw(ctx, repoDir, baseSHA, shaA, shaC)
	if err != nil {
		t.Fatalf("CaptureRaw AC failed: %v", err)
	}
	if rawAC.MergeTreeConflict {
		t.Errorf("expected MergeTreeConflict=false for clean branches AC, got true. Output:\n%s", rawAC.MergeTreeOut)
	}
}

func TestNormalizeChanges_PathAndLabels(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	// Base commit with files for each condition
	mustWrite := func(rel string, data []byte, mode os.FileMode) {
		p := filepath.Join(repoDir, rel)
		if err := os.WriteFile(p, data, mode); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("plain.txt", []byte("plain text\n"), 0o644)
	mustWrite("to_rename.txt", []byte("rename this file\n"), 0o644)
	mustWrite("to_delete.txt", []byte("delete this file\n"), 0o644)
	mustWrite("mode_only.txt", []byte("mode only test\n"), 0o644)
	mustWrite("bin.dat", []byte{0x00, 0x01, 0x02, 0x03}, 0o644)
	mustWrite("gen.go", []byte("// Code generated by protoc-gen-go. DO NOT EDIT.\npackage test\n"), 0o644)
	mustWrite("script.sh", []byte("#!/bin/sh\necho hello\n"), 0o644)
	if err := os.Symlink("plain.txt", filepath.Join(repoDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "base commit")
	baseSHA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featA with all changes:
	gitRun(t, repoDir, "checkout", "-b", "featA")
	// 1. rename
	gitRun(t, repoDir, "mv", "to_rename.txt", "renamed.txt")
	// 2. delete
	gitRun(t, repoDir, "rm", "to_delete.txt")
	// 3. binary change
	mustWrite("bin.dat", []byte{0x00, 0xFF, 0xFE, 0xFD}, 0o644)
	gitRun(t, repoDir, "add", "bin.dat")
	// 4. mode-only (chmod +x without content change)
	if err := os.Chmod(filepath.Join(repoDir, "mode_only.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", "mode_only.txt")
	// 5. generated file modification
	mustWrite("gen.go", []byte("// Code generated by protoc-gen-go. DO NOT EDIT.\npackage test\n// v2\n"), 0o644)
	gitRun(t, repoDir, "add", "gen.go")
	// 6. symlink modification
	os.Remove(filepath.Join(repoDir, "link.txt"))
	if err := os.Symlink("mode_only.txt", filepath.Join(repoDir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", "link.txt")
	// 7. executable creation / chmod +x on script.sh
	if err := os.Chmod(filepath.Join(repoDir, "script.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", "script.sh")

	gitRun(t, repoDir, "commit", "-m", "featA changes")
	shaA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Dummy clean branch featB
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "featB")
	mustWrite("dummy.txt", []byte("dummy\n"), 0o644)
	gitRun(t, repoDir, "add", "dummy.txt")
	gitRun(t, repoDir, "commit", "-m", "featB commit")
	shaB := gitRun(t, repoDir, "rev-parse", "HEAD")

	raw, err := CaptureRaw(ctx, repoDir, baseSHA, shaA, shaB)
	if err != nil {
		t.Fatalf("CaptureRaw failed: %v", err)
	}

	changes, err := NormalizeChanges(ctx, repoDir, baseSHA, shaA, raw, true)
	if err != nil {
		t.Fatalf("NormalizeChanges failed: %v", err)
	}

	// 1. Assert paths are sorted
	for i := 1; i < len(changes); i++ {
		if changes[i].Path < changes[i-1].Path {
			t.Errorf("changes not sorted: %s before %s", changes[i].Path, changes[i-1].Path)
		}
	}

	byPath := make(map[string]PathChange)
	for _, c := range changes {
		byPath[c.Path] = c
	}

	hasLabel := func(c PathChange, target ChangeLabel) bool {
		for _, l := range c.Labels {
			if l == target {
				return true
			}
		}
		return false
	}

	// Check rename
	if c, ok := byPath["renamed.txt"]; !ok {
		t.Errorf("missing renamed.txt in normalized changes")
	} else {
		if !hasLabel(c, LabelRenameDelete) {
			t.Errorf("renamed.txt missing LabelRenameDelete, got labels: %v", c.Labels)
		}
		if c.OldPath != "to_rename.txt" {
			t.Errorf("renamed.txt OldPath = %q, want to_rename.txt", c.OldPath)
		}
	}

	// Check delete
	if c, ok := byPath["to_delete.txt"]; !ok {
		t.Errorf("missing to_delete.txt in normalized changes")
	} else {
		if !hasLabel(c, LabelRenameDelete) {
			t.Errorf("to_delete.txt missing LabelRenameDelete, got labels: %v", c.Labels)
		}
		if c.Status != "D" {
			t.Errorf("to_delete.txt Status = %q, want D", c.Status)
		}
	}

	// Check binary
	if c, ok := byPath["bin.dat"]; !ok {
		t.Errorf("missing bin.dat in normalized changes")
	} else {
		if !hasLabel(c, LabelBinary) {
			t.Errorf("bin.dat missing LabelBinary, got labels: %v", c.Labels)
		}
		if !c.Binary {
			t.Errorf("bin.dat Binary = false, want true")
		}
	}

	// Check mode-only
	if c, ok := byPath["mode_only.txt"]; !ok {
		t.Errorf("missing mode_only.txt in normalized changes")
	} else {
		if !hasLabel(c, LabelModeOnly) {
			t.Errorf("mode_only.txt missing LabelModeOnly, got labels: %v", c.Labels)
		}
		if c.AddedLines != 0 || c.DeletedLines != 0 {
			t.Errorf("mode_only.txt has added=%d deleted=%d, want 0/0", c.AddedLines, c.DeletedLines)
		}
	}

	// Check generated
	if c, ok := byPath["gen.go"]; !ok {
		t.Errorf("missing gen.go in normalized changes")
	} else {
		if !hasLabel(c, LabelGenerated) {
			t.Errorf("gen.go missing LabelGenerated, got labels: %v", c.Labels)
		}
	}

	// Check symlink
	if c, ok := byPath["link.txt"]; !ok {
		t.Errorf("missing link.txt in normalized changes")
	} else {
		if !hasLabel(c, LabelSymlinkSubmodule) {
			t.Errorf("link.txt missing LabelSymlinkSubmodule, got labels: %v", c.Labels)
		}
	}

	// Check executable
	if c, ok := byPath["script.sh"]; !ok {
		t.Errorf("missing script.sh in normalized changes")
	} else {
		if !hasLabel(c, LabelExecutable) {
			t.Errorf("script.sh missing LabelExecutable, got labels: %v", c.Labels)
		}
	}
}

func TestComputeMetrics_HandComputedRatio(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	mustWriteLines := func(rel string, n int) {
		p := filepath.Join(repoDir, rel)
		var sb strings.Builder
		for i := 1; i <= n; i++ {
			sb.WriteString(fmt.Sprintf("%s line %d\n", rel, i))
		}
		if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Base commit
	mustWriteLines("shared.txt", 5)
	mustWriteLines("shared2.txt", 5)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "base commit")
	baseSHA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featA:
	// - shared.txt: add 20 lines (20 lines changed)
	// - shared2.txt: add 10 lines (10 lines changed)
	// - onlyA.txt: add 30 lines (30 lines changed)
	// Total A lines = 60. Shared A lines = 30. Hotspot A = 30/60 = 0.50.
	// Paths A = 3. Shared paths = 2. Path ratio A = 2/3 = 0.6666...
	gitRun(t, repoDir, "checkout", "-b", "featA")
	mustWriteLines("shared.txt", 25)
	mustWriteLines("shared2.txt", 15)
	mustWriteLines("onlyA.txt", 30)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featA changes")
	shaA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featB (off base):
	// - shared.txt: add 10 lines (10 lines changed)
	// - shared2.txt: add 15 lines (15 lines changed)
	// - onlyB.txt: add 75 lines (75 lines changed)
	// Total B lines = 100. Shared B lines = 25. Hotspot B = 25/100 = 0.25.
	// Paths B = 3. Shared paths = 2. Path ratio B = 2/3 = 0.6666...
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "featB")
	mustWriteLines("shared.txt", 15)
	mustWriteLines("shared2.txt", 20)
	mustWriteLines("onlyB.txt", 75)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featB commit")
	shaB := gitRun(t, repoDir, "rev-parse", "HEAD")

	raw, err := CaptureRaw(ctx, repoDir, baseSHA, shaA, shaB)
	if err != nil {
		t.Fatalf("CaptureRaw failed: %v", err)
	}

	changesA, err := NormalizeChanges(ctx, repoDir, baseSHA, shaA, raw, true)
	if err != nil {
		t.Fatalf("NormalizeChanges A failed: %v", err)
	}
	changesB, err := NormalizeChanges(ctx, repoDir, baseSHA, shaB, raw, false)
	if err != nil {
		t.Fatalf("NormalizeChanges B failed: %v", err)
	}

	metrics := ComputeMetrics(changesA, changesB)

	if metrics.TotalLinesA != 60 {
		t.Errorf("TotalLinesA = %d, want 60", metrics.TotalLinesA)
	}
	if metrics.SharedLinesA != 30 {
		t.Errorf("SharedLinesA = %d, want 30", metrics.SharedLinesA)
	}
	if metrics.TotalLinesB != 100 {
		t.Errorf("TotalLinesB = %d, want 100", metrics.TotalLinesB)
	}
	if metrics.SharedLinesB != 25 {
		t.Errorf("SharedLinesB = %d, want 25", metrics.SharedLinesB)
	}

	// Hotspot A = 30 / 60 = 0.50
	if math.Abs(metrics.HotspotWeightA-0.50) > 1e-6 {
		t.Errorf("HotspotWeightA = %f, want 0.50", metrics.HotspotWeightA)
	}
	// Hotspot B = 25 / 100 = 0.25
	if math.Abs(metrics.HotspotWeightB-0.25) > 1e-6 {
		t.Errorf("HotspotWeightB = %f, want 0.25", metrics.HotspotWeightB)
	}
	// Overall Hotspot = max(0.50, 0.25) = 0.50
	if math.Abs(metrics.HotspotWeight-0.50) > 1e-6 {
		t.Errorf("HotspotWeight = %f, want 0.50", metrics.HotspotWeight)
	}

	// Path ratios
	if math.Abs(metrics.PathRatioA-(2.0/3.0)) > 1e-6 {
		t.Errorf("PathRatioA = %f, want %f", metrics.PathRatioA, 2.0/3.0)
	}
	if math.Abs(metrics.PathRatioB-(2.0/3.0)) > 1e-6 {
		t.Errorf("PathRatioB = %f, want %f", metrics.PathRatioB, 2.0/3.0)
	}

	// Shared paths
	wantShared := []string{"shared.txt", "shared2.txt"}
	if len(metrics.SharedPaths) != len(wantShared) || metrics.SharedPaths[0] != wantShared[0] || metrics.SharedPaths[1] != wantShared[1] {
		t.Errorf("SharedPaths = %v, want %v", metrics.SharedPaths, wantShared)
	}
}

func TestClassification_ThresholdsAndBoundaries(t *testing.T) {
	thresholds := DefaultThresholds()

	tests := []struct {
		name      string
		signals   Signals
		wantClass Class
	}{
		{
			name: "predicted conflict triggers required",
			signals: Signals{
				PredictedConflict: true,
				ConflictPaths:     []string{"foo.go"},
			},
			wantClass: ClassRequired,
		},
		{
			name: "rename/delete collision triggers required",
			signals: Signals{
				RenameDeleteCollision: true,
				CollisionPaths:        []string{"bar.txt"},
			},
			wantClass: ClassRequired,
		},
		{
			name: "shared binary triggers required",
			signals: Signals{
				SharedBinary: true,
				BinaryPaths:  []string{"image.png"},
			},
			wantClass: ClassRequired,
		},
		{
			name: "intersecting hunks triggers required",
			signals: Signals{
				IntersectingHunks: true,
				SharedPaths:       []string{"main.go"},
			},
			wantClass: ClassRequired,
		},
		{
			name: "nearby hunks exactly at 3 lines distance triggers required",
			signals: Signals{
				NearbyHunks:     true,
				MinHunkDistance: 3,
				SharedPaths:     []string{"main.go"},
			},
			wantClass: ClassRequired,
		},
		{
			name: "hunks at 4 lines distance (disjoint) falls back to warning",
			signals: Signals{
				NearbyHunks:         false,
				MinHunkDistance:     4,
				SharedDisjointPaths: true,
				SharedPaths:         []string{"main.go"},
				HotspotWeight:       0.10,
			},
			wantClass: ClassWarning,
		},
		{
			name: "hotspot weight exactly 0.50 triggers required",
			signals: Signals{
				HotspotWeight: 0.50,
			},
			wantClass: ClassRequired,
		},
		{
			name: "hotspot weight at 0.499 falls back to warning",
			signals: Signals{
				HotspotWeight: 0.499,
			},
			wantClass: ClassWarning,
		},
		{
			name: "hotspot weight exactly 0.20 triggers warning",
			signals: Signals{
				HotspotWeight: 0.20,
			},
			wantClass: ClassWarning,
		},
		{
			name: "hotspot weight at 0.199 without shared paths is informational",
			signals: Signals{
				HotspotWeight: 0.199,
			},
			wantClass: ClassInformational,
		},
		{
			name: "shared disjoint paths with hotspot < 0.20 triggers warning",
			signals: Signals{
				SharedDisjointPaths: true,
				SharedPaths:         []string{"shared.txt"},
				HotspotWeight:       0.05,
			},
			wantClass: ClassWarning,
		},
		{
			name: "completely disjoint changes is informational",
			signals: Signals{
				HotspotWeight: 0.0,
				PathsA:        []string{"a.txt"},
				PathsB:        []string{"b.txt"},
			},
			wantClass: ClassInformational,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotClass, rationale := Classify(tc.signals, thresholds)
			if gotClass != tc.wantClass {
				t.Errorf("Classify() = %q, want %q", gotClass, tc.wantClass)
			}
			if len(rationale) == 0 {
				t.Errorf("Classify() returned empty rationale")
			}
		})
	}
}

func TestEvaluate_StructuralEvidenceOmission(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	// Base commit
	p := filepath.Join(repoDir, "file.txt")
	if err := os.WriteFile(p, []byte("line 1\nline 2\nline 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "base commit")
	baseSHA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featA: modifies line 1
	gitRun(t, repoDir, "checkout", "-b", "featA")
	if err := os.WriteFile(p, []byte("line 1 edit A\nline 2\nline 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featA commit")
	shaA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featB: clean disjoint change in other.txt
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "featB")
	if err := os.WriteFile(filepath.Join(repoDir, "other.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featB commit")
	shaB := gitRun(t, repoDir, "rev-parse", "HEAD")

	fixedTime := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)

	// 1. Evaluate with default StubStructuralProvider (unavailable)
	ev, err := Evaluate(ctx, repoDir, baseSHA, shaA, shaB, WithClock(func() time.Time { return fixedTime }))
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Classification proceeds using deterministic Git evidence alone
	if ev.Class != ClassInformational {
		t.Errorf("Class = %q, want %q", ev.Class, ClassInformational)
	}

	// Structural evidence omission must be disclosed
	if !ev.Structural.Omitted {
		t.Errorf("Structural.Omitted = false, want true")
	}
	if ev.Structural.Available {
		t.Errorf("Structural.Available = true, want false")
	}
	if ev.Structural.Status != "unavailable" {
		t.Errorf("Structural.Status = %q, want unavailable", ev.Structural.Status)
	}
	if !strings.Contains(ev.Structural.Disclosure, "unavailable") {
		t.Errorf("Structural.Disclosure missing 'unavailable': %q", ev.Structural.Disclosure)
	}

	// Evidence JSON and Hash verification
	jsonStr, err := ev.JSON()
	if err != nil {
		t.Fatalf("ev.JSON() failed: %v", err)
	}
	if !strings.Contains(jsonStr, `"version": "v1"`) || !strings.Contains(jsonStr, `"class": "informational"`) {
		t.Errorf("ev.JSON() missing expected fields: %s", jsonStr)
	}
	expectedHash, err := ev.ComputeHash()
	if err != nil {
		t.Fatalf("ComputeHash failed: %v", err)
	}
	if ev.Hash != expectedHash {
		t.Errorf("ev.Hash = %s, want %s", ev.Hash, expectedHash)
	}

	// 2. Evaluate with a failing StructuralProvider
	failingProvider := customStructuralProvider{
		err: errors.New("codegraph daemon connection timeout"),
	}
	ev2, err := Evaluate(ctx, repoDir, baseSHA, shaA, shaB, WithStructuralProvider(failingProvider))
	if err != nil {
		t.Fatalf("Evaluate with failing provider returned unexpected error: %v", err)
	}
	if !ev2.Structural.Omitted || !strings.Contains(ev2.Structural.Disclosure, "connection timeout") {
		t.Errorf("expected disclosure of provider failure, got: %+v", ev2.Structural)
	}
}

type customStructuralProvider struct {
	evidence StructuralEvidence
	err      error
}

func (c customStructuralProvider) GetStructuralEvidence(ctx context.Context, repoDir, baseSHA, shaA, shaB string) (StructuralEvidence, error) {
	if c.err != nil {
		return StructuralEvidence{}, c.err
	}
	return c.evidence, nil
}

func TestEvaluate_FullFlowAndEvidenceRecording(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	// Base commit with multiple files
	p1 := filepath.Join(repoDir, "shared.txt")
	_ = os.WriteFile(p1, []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20\n"), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "base")
	baseSHA := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featA: modifies lines 1-2 (2 lines out of 20)
	gitRun(t, repoDir, "checkout", "-b", "featA")
	_ = os.WriteFile(p1, []byte("1 edited\n2 edited\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20\n"), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featA")
	_ = gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featB: modifies lines 19-20 (distance > 3 lines, disjoint)
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "featB")
	_ = os.WriteFile(p1, []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19 edited\n20 edited\n"), 0o644)
	// Also add 80 lines in otherB.txt so hotspot weight = 2 / 82 = 0.024 < 0.20
	var sb strings.Builder
	for i := 1; i <= 80; i++ {
		sb.WriteString(fmt.Sprintf("line %d\n", i))
	}
	_ = os.WriteFile(filepath.Join(repoDir, "otherB.txt"), []byte(sb.String()), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featB")
	shaB := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Branch featA also adds 80 lines in otherA.txt
	gitRun(t, repoDir, "checkout", "featA")
	_ = os.WriteFile(filepath.Join(repoDir, "otherA.txt"), []byte(sb.String()), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "featA extra")
	shaA2 := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Test Evaluate: shared disjoint paths with hotspot < 0.20 and hunks > 3 lines apart -> Warning
	ev, err := Evaluate(ctx, repoDir, "", shaA2, shaB)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if ev.Class != ClassWarning {
		t.Errorf("ev.Class = %q, want %q", ev.Class, ClassWarning)
	}
	if !ev.Signals.SharedDisjointPaths {
		t.Errorf("Signals.SharedDisjointPaths = false, want true")
	}
	if ev.Signals.IntersectingHunks || ev.Signals.NearbyHunks {
		t.Errorf("expected no intersecting or nearby hunks, got inter=%v nearby=%v", ev.Signals.IntersectingHunks, ev.Signals.NearbyHunks)
	}
	if len(ev.Rationale) == 0 {
		t.Errorf("ev.Rationale is empty")
	}
	if ev.BaseSHA != baseSHA {
		t.Errorf("ev.BaseSHA = %s, want %s", ev.BaseSHA, baseSHA)
	}

	// Verify WithThresholds option
	customThresh := Thresholds{
		HotspotRequired: 0.80,
		HotspotWarning:  0.40,
		NearbyHunkLines: 5,
	}
	evCustom, err := Evaluate(ctx, repoDir, baseSHA, shaA2, shaB, WithThresholds(customThresh))
	if err != nil {
		t.Fatalf("Evaluate custom thresholds failed: %v", err)
	}
	if evCustom.Thresholds.HotspotRequired != 0.80 {
		t.Errorf("Thresholds not applied: %+v", evCustom.Thresholds)
	}
}

func TestClass_Valid(t *testing.T) {
	for _, c := range []Class{ClassRequired, ClassWarning, ClassInformational} {
		if !c.Valid() {
			t.Errorf("Class %q should be valid", c)
		}
	}
	if Class("unknown").Valid() {
		t.Errorf("Class 'unknown' should not be valid")
	}
}

func TestDefaultThresholds(t *testing.T) {
	dt := DefaultThresholds()
	if dt.HotspotRequired != 0.50 {
		t.Errorf("HotspotRequired = %f, want 0.50", dt.HotspotRequired)
	}
	if dt.HotspotWarning != 0.20 {
		t.Errorf("HotspotWarning = %f, want 0.20", dt.HotspotWarning)
	}
	if dt.NearbyHunkLines != 3 {
		t.Errorf("NearbyHunkLines = %d, want 3", dt.NearbyHunkLines)
	}
}

func TestFindUniqueMergeBase_Errors(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	// Commit 1 on main
	_ = os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("a\n"), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "initial")
	c1 := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Create an orphan branch with unrelated history
	gitRun(t, repoDir, "checkout", "--orphan", "unrelated")
	gitRun(t, repoDir, "rm", "-rf", ".")
	_ = os.WriteFile(filepath.Join(repoDir, "u.txt"), []byte("u\n"), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "unrelated initial")
	cUnrelated := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Assert ErrNoMergeBase
	_, err := FindUniqueMergeBase(ctx, repoDir, c1, cUnrelated)
	if !errors.Is(err, ErrNoMergeBase) {
		t.Errorf("FindUniqueMergeBase on unrelated histories = %v, want ErrNoMergeBase", err)
	}

	// Construct criss-cross merge history with multiple merge bases
	// B1 off c1
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "b1")
	_ = os.WriteFile(filepath.Join(repoDir, "b1.txt"), []byte("b1\n"), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "b1")
	shaB1 := gitRun(t, repoDir, "rev-parse", "HEAD")

	// B2 off c1
	gitRun(t, repoDir, "checkout", "main")
	gitRun(t, repoDir, "checkout", "-b", "b2")
	_ = os.WriteFile(filepath.Join(repoDir, "b2.txt"), []byte("b2\n"), 0o644)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "b2")
	_ = gitRun(t, repoDir, "rev-parse", "HEAD")

	// Merge b2 into b1 -> m1
	gitRun(t, repoDir, "checkout", "b1")
	gitRun(t, repoDir, "merge", "--no-ff", "-m", "merge b2 into b1", "b2")
	shaM1 := gitRun(t, repoDir, "rev-parse", "HEAD")

	// Merge b1 into b2 -> m2
	gitRun(t, repoDir, "checkout", "b2")
	gitRun(t, repoDir, "merge", "--no-ff", "-m", "merge b1 into b2", shaB1)
	shaM2 := gitRun(t, repoDir, "rev-parse", "HEAD")

	// m1 and m2 have two merge bases: shaB1 and shaB2
	_, err = FindUniqueMergeBase(ctx, repoDir, shaM1, shaM2)
	if !errors.Is(err, ErrMultipleMergeBases) {
		t.Errorf("FindUniqueMergeBase on criss-cross merge = %v, want ErrMultipleMergeBases", err)
	}
}




