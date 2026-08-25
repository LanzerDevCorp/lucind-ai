// Package lanecheck has no production code. It exists solely to exercise
// lucind-lane-check.sh's `--verify-citations` range validation as a real
// subprocess -- the same convention internal/buildcheck uses to prove the
// CGO_ENABLED=0 build guarantee as an executable, CI-visible test rather
// than trusting a shell script's behavior from memory. There is no
// pre-existing Go or shell test harness for this script; a table-driven Go
// test that shells out to it was chosen because it fits that established
// buildcheck precedent, gives every case its own named subtest and
// isolated temp directory, and needs no new test framework.
package lanecheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// citationManifest returns a minimal draft document with citation as the
// sole row of its "## Citation Manifest" table -- exactly the shape
// lucind-lane-check.sh's --verify-citations scans.
func citationManifest(citation string) string {
	return "## Citation Manifest\n\n" +
		"| citation | claim |\n" +
		"|---|---|\n" +
		"| `" + citation + "` | claim |\n"
}

// TestVerifyCitationsRangeValidation proves --verify-citations validates the
// END of a cited line range, not just the start (the reported defect: a
// range like "1-32" against a 31-line file used to PASS because only the
// start line, 1, was ever compared against the file's line count). It also
// covers the range-order and en-dash-separator decisions made alongside that
// fix.
func TestVerifyCitationsRangeValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to lucind-lane-check.sh; skipped in -short mode")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to resolve this test file's own path")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	scriptPath := filepath.Join(repoRoot, "lucind-lane-check.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("lucind-lane-check.sh not found at %s: %v", scriptPath, err)
	}

	// target.txt is a synthetic fixture this test fully controls -- 5 lines,
	// deliberately not the real internal/lane/status_test.go the defect
	// report reproduced against, since that file's own line count is free
	// to change and must not silently break this regression coverage.
	const targetLineCount = 5
	targetContent := strings.Repeat("line\n", targetLineCount)

	cases := []struct {
		name         string
		citation     string
		wantExitZero bool
		wantContains string
	}{
		{
			name:         "single line within range passes",
			citation:     "target.txt:3",
			wantExitZero: true,
			wantContains: "PASS  citation target.txt:3: file exists, has 5 lines",
		},
		{
			name:         "single line past end of file fails",
			citation:     "target.txt:6",
			wantExitZero: false,
			wantContains: "FAIL  citation target.txt:6: file has 5 lines, cited range ends at line 6 which is out of range",
		},
		{
			name:         "range fully inside file passes",
			citation:     "target.txt:1-5",
			wantExitZero: true,
			wantContains: "PASS  citation target.txt:1-5: file exists, has 5 lines",
		},
		{
			name:         "range end past file's line count fails (the reported defect)",
			citation:     "target.txt:1-6",
			wantExitZero: false,
			wantContains: "FAIL  citation target.txt:1-6: file has 5 lines, cited range ends at line 6 which is out of range",
		},
		{
			name:         "reversed range (start after end) fails as malformed",
			citation:     "target.txt:5-1",
			wantExitZero: false,
			wantContains: "FAIL  citation target.txt:5-1: malformed range, start line 5 is after end line 1",
		},
		{
			name:         "en dash range separator within bounds passes",
			citation:     "target.txt:1–5",
			wantExitZero: true,
			wantContains: "file exists, has 5 lines",
		},
		{
			name:         "en dash range separator past bounds fails (not silently skipped)",
			citation:     "target.txt:1–6",
			wantExitZero: false,
			wantContains: "cited range ends at line 6 which is out of range",
		},
		{
			name:         "unparseable start line is skipped, not failed",
			citation:     "target.txt:abc",
			wantExitZero: true,
			wantContains: "SKIP  citation target.txt:abc: could not parse a line number",
		},
		{
			name:         "unparseable end of range is skipped, not failed",
			citation:     "target.txt:3-abc",
			wantExitZero: true,
			wantContains: "SKIP  citation target.txt:3-abc: could not parse the end of the line range",
		},
		{
			name:         "missing cited file still fails",
			citation:     "does-not-exist.txt:1-3",
			wantExitZero: false,
			wantContains: "FAIL  citation does-not-exist.txt:1-3: file does not exist",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "target.txt"), []byte(targetContent), 0o644); err != nil {
				t.Fatalf("write target.txt: %v", err)
			}
			draftPath := filepath.Join(dir, "draft.md")
			if err := os.WriteFile(draftPath, []byte(citationManifest(tc.citation)), 0o644); err != nil {
				t.Fatalf("write draft.md: %v", err)
			}

			cmd := exec.Command("sh", scriptPath,
				"--file", draftPath,
				"--verify-citations",
				"--skip-git",
				"--skip-result",
			)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()

			exitZero := err == nil
			if !exitZero {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitZero = exitErr.ExitCode() == 0
				} else {
					t.Fatalf("lucind-lane-check.sh did not run: %v\n%s", err, out)
				}
			}

			if exitZero != tc.wantExitZero {
				t.Errorf("exit zero = %v, want %v (exit code %s)\noutput:\n%s",
					exitZero, tc.wantExitZero, exitCodeString(err), out)
			}
			if !strings.Contains(string(out), tc.wantContains) {
				t.Errorf("output does not contain %q\noutput:\n%s", tc.wantContains, out)
			}
		})
	}
}

func exitCodeString(err error) string {
	if err == nil {
		return "0"
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return strconv.Itoa(exitErr.ExitCode())
	}
	return err.Error()
}
