// Package buildcheck has no production code. It exists solely to record,
// as an executable and CI-visible test, the pure-Go build guarantee this
// project depends on: modernc.org/sqlite must compile and link with cgo
// disabled. This is the "Build: No cgo" row of design's testing strategy
// table, run as a real subprocess rather than trusted from memory.
package buildcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCGoDisabledBuildSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to `go build`; skipped in -short mode")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to resolve this test file's own path")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	cmd := exec.Command("go", "build", "-o", t.TempDir()+string(filepath.Separator), "./...")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CGO_ENABLED=0 go build ./... failed: %v\n%s", err, out)
	}
}
