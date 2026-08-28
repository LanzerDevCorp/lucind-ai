package run

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

func TestEnforceAllowedPathsUsesCanonicalFourWayCopyAwareChanges(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(t *testing.T, root string)
		allowed    []string
		expectPath string
	}{
		{
			name: "copy checks source endpoint",
			prepare: func(t *testing.T, root string) {
				writeScopeFile(t, root, "allowed-copy.txt", []byte("copy me\n"))
				commitScopeChange(t, root, "copy source")
			},
			allowed:    []string{"allowed-copy.txt"},
			expectPath: "outside-copy-source.txt",
		},
		{
			name: "rename checks source endpoint",
			prepare: func(t *testing.T, root string) {
				runScopeGit(t, root, "mv", "outside-rename.txt", "allowed-rename.txt")
				commitScopeChange(t, root, "rename source")
			},
			allowed:    []string{"allowed-rename.txt"},
			expectPath: "outside-rename.txt",
		},
		{
			name: "preserves whitespace in path",
			prepare: func(t *testing.T, root string) {
				writeScopeFile(t, root, " leading-and-trailing ", []byte("whitespace\n"))
				commitScopeChange(t, root, "add whitespace path")
			},
			allowed:    []string{"leading-and-trailing"},
			expectPath: " leading-and-trailing ",
		},
		{
			name: "includes staged path",
			prepare: func(t *testing.T, root string) {
				writeScopeFile(t, root, "staged-outside.txt", []byte("staged\n"))
				runScopeGit(t, root, "add", "staged-outside.txt")
			},
			allowed:    []string{"allowed-only.txt"},
			expectPath: "staged-outside.txt",
		},
		{
			name: "inspects every committed change from base",
			prepare: func(t *testing.T, root string) {
				writeScopeFile(t, root, "earlier-outside.txt", []byte("earlier\n"))
				commitScopeChange(t, root, "first change")
				writeScopeFile(t, root, "allowed-later.txt", []byte("later\n"))
				commitScopeChange(t, root, "second change")
			},
			allowed:    []string{"allowed-later.txt"},
			expectPath: "earlier-outside.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, baseSHA := newScopeRepo(t)
			tt.prepare(t, root)

			status, reason := enforceAllowedPaths(context.Background(), Deps{}, root, baseSHA, packet.Packet{
				AllowedPaths: tt.allowed,
			})
			if status != lane.Deviated {
				t.Fatalf("status = %s, want %s (reason: %s)", status, lane.Deviated, reason)
			}
			if !strings.Contains(reason, tt.expectPath) {
				t.Fatalf("reason = %q, want it to contain path %q", reason, tt.expectPath)
			}
		})
	}
}

func TestEnforceAllowedPathsKeepsMultipleInScopeCommitsDone(t *testing.T) {
	root, baseSHA := newScopeRepo(t)
	writeScopeFile(t, root, "allowed-one.txt", []byte("one\n"))
	commitScopeChange(t, root, "first allowed change")
	writeScopeFile(t, root, "allowed-two.txt", []byte("two\n"))
	commitScopeChange(t, root, "second allowed change")

	status, reason := enforceAllowedPaths(context.Background(), Deps{}, root, baseSHA, packet.Packet{
		AllowedPaths: []string{"allowed-one.txt", "allowed-two.txt"},
	})
	if status != lane.Done {
		t.Fatalf("status = %s, want %s (reason: %s)", status, lane.Done, reason)
	}
}

func TestEnforceAllowedPathsRejectsRenameAcrossReadOnlyInputScope(t *testing.T) {
	root, baseSHA := newScopeRepo(t)
	runScopeGit(t, root, "mv", "outside-rename.txt", "allowed-rename.txt")
	commitScopeChange(t, root, "rename read-only input")

	status, reason := enforceAllowedPaths(context.Background(), Deps{}, root, baseSHA, packet.Packet{
		AllowedPaths:  []string{"allowed-rename.txt"},
		ReadOnlyPaths: []string{"outside-rename.txt"},
	})
	if status != lane.Deviated || !strings.Contains(reason, "outside-rename.txt") {
		t.Fatalf("status/reason = %s/%q, want deviated with read-only source", status, reason)
	}
}

func newScopeRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	runScopeGit(t, root, "init", "-q")
	runScopeGit(t, root, "config", "user.email", "scope-test@example.com")
	runScopeGit(t, root, "config", "user.name", "Scope Test")
	writeScopeFile(t, root, "base.txt", []byte("base\n"))
	writeScopeFile(t, root, "outside-copy-source.txt", []byte("copy me\n"))
	writeScopeFile(t, root, "outside-rename.txt", []byte("rename me\n"))
	commitScopeChange(t, root, "base")
	baseSHA := strings.TrimSpace(runScopeGit(t, root, "rev-parse", "HEAD"))
	return root, baseSHA
}

func writeScopeFile(t *testing.T, root, name string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), contents, 0o644); err != nil {
		t.Fatalf("write %q: %v", name, err)
	}
}

func commitScopeChange(t *testing.T, root, message string) {
	t.Helper()
	runScopeGit(t, root, "add", "--all")
	runScopeGit(t, root, "commit", "-qm", message)
}

func runScopeGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
