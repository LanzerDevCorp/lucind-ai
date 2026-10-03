package executor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

func herdrWorkspaceIDs(t *testing.T) map[string]bool {
	t.Helper()
	out, err := exec.Command("herdr", "workspace", "list").Output()
	if err != nil {
		t.Fatalf("herdr workspace list: %v", err)
	}
	var resp struct {
		Result struct {
			Workspaces []struct {
				ID string `json:"workspace_id"`
			} `json:"workspaces"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("parse workspace list: %v", err)
	}
	ids := map[string]bool{}
	for _, w := range resp.Result.Workspaces {
		ids[w.ID] = true
	}
	return ids
}

// TestHerdrAgyInteractiveLive runs a real interactive agy in a real herdr pane. It trusts the
// temp worktree in the REAL Antigravity settings file for the duration of the run (and removes it).
// Opt in with LUCIND_HERDR_LIVE=1 inside a herdr pane (HERDR_ENV=1); it spends a little agy quota.
//
// The prompt asks for an envelope that is missing required fields, so the Stop hook must answer
// "continue" and the agent must repair it before the lane is considered done.
func TestHerdrAgyInteractiveLive(t *testing.T) {
	if os.Getenv("LUCIND_HERDR_LIVE") != "1" || os.Getenv("HERDR_ENV") != "1" {
		t.Skip("set LUCIND_HERDR_LIVE=1 inside herdr (HERDR_ENV=1) to run against the real agy")
	}
	_, thisFile, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	bin := filepath.Join(t.TempDir(), "lucind-ai")
	build := exec.Command("go", "build", "-o", bin, "./cmd/lucind-ai")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build lucind-ai: %v\n%s", err, out)
	}

	repo, err := os.MkdirTemp("", "lucind-live-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(repo) })
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	// A worker rule the prompt never mentions: if real agy loads the lane rules, hello.txt will
	// carry the word.
	rulesSrc := "<!-- lucind:rules audience=worker -->\n## Marker rule\n\n- Every file you create in this lane must contain the word BANANA, in addition to anything else it must contain.\n"
	if err := os.WriteFile(filepath.Join(repo, "lucind-rules.md"), []byte(rulesSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	before := herdrWorkspaceIDs(t)
	t.Cleanup(func() {
		for id := range herdrWorkspaceIDs(t) {
			if !before[id] {
				_ = exec.Command("herdr", "workspace", "close", id).Run()
			}
		}
	})

	prompt := "Do two things in the current directory. 1) Create hello.txt containing the word hi. " +
		"2) Create the file .lucind/result.json containing exactly this JSON and nothing else: " +
		`{"packet_id":"live","status":"done"}` + " . Then stop."
	h := HerdrAgy{Interactive: true, HookBinary: bin}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	outcome, err := h.Run(ctx, Request{Prompt: prompt, WorktreePath: repo, Model: "gemini-3.8-flash-medium", AllowedPaths: []string{"hello.txt", ".lucind/result.json"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("outcome = %+v", outcome)
	if outcome.ExitCode != 0 || outcome.TimedOut {
		t.Fatalf("outcome = %+v, want exit 0 and no timeout", outcome)
	}
	hello, err := os.ReadFile(filepath.Join(repo, "hello.txt"))
	if err != nil {
		t.Fatalf("hello.txt missing: %v", err)
	}
	if !strings.Contains(strings.ToUpper(string(hello)), "BANANA") {
		t.Errorf("the lane rules did not reach agy: hello.txt = %q, want it to contain BANANA", hello)
	}
	env, err := result.Read(os.DirFS(repo), ".lucind/result.json")
	if err != nil {
		t.Fatalf("the Stop hook must have made the agent repair the envelope, but it is still invalid: %v", err)
	}
	t.Logf("final envelope: packet_id=%s status=%s", env.PacketID, env.Status)
	if _, err := os.Stat(filepath.Join(repo, ".agents", "hooks.json")); !os.IsNotExist(err) {
		t.Errorf("hooks.json must be removed after the lane (stat err = %v)", err)
	}
}
