package judges

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

// TestGateLiveCursor runs one real blind-judge pass through cursor-agent on a diff with an
// obvious bug. Opt in with LUCIND_JUDGES_LIVE=1 (needs a logged-in cursor-agent).
func TestGateLiveCursor(t *testing.T) {
	if os.Getenv("LUCIND_JUDGES_LIVE") != "1" {
		t.Skip("set LUCIND_JUDGES_LIVE=1 to run against the real cursor-agent")
	}
	dir := t.TempDir()
	sh := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	sh("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sh("add", "-A")
	sh("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\n// Sub returns a minus b.\nfunc Sub(a, b int) int { return a + b }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	g := Gate{Plan: risk.PlanFor(risk.TierMedium), Models: []string{"claude-sonnet-5-thinking-high"}, Run: CursorRunner("")}
	st, reason := g.PreCommit(ctx, dir, packet.Packet{Body: "Add a Sub function that subtracts b from a."})
	t.Logf("status=%v reason=%q", st, reason)
	if st != lane.Failed {
		t.Fatalf("status = %v, want failed (the judge should flag Sub returning a+b)", st)
	}
}
