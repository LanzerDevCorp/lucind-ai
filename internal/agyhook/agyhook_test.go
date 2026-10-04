package agyhook

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

const laneID = "20260101-120000-abcd"

const validResult = `{"packet_id":"x","status":"done","summary":"ok","hard_stops":[]}`
const blockedResult = `{"packet_id":"x","status":"blocked","summary":"stuck","hard_stops":[]}`

// newLaneRepo creates a git repository with a running lane and isolates the
// attest key/state locations under the temp dir.
func newLaneRepo(t *testing.T, allow ...string) string {
	t.Helper()
	root := t.TempDir()
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := lane.Save(root, lane.Lane{ID: laneID, Cwd: root, Allow: allow, Status: lane.StatusRunning}); err != nil {
		t.Fatalf("save lane: %v", err)
	}
	return root
}

func preToolUse(t *testing.T, laneEnv, root, tool string, args map[string]any) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"workspacePaths": []string{root},
		"toolCall":       map[string]any{"name": tool, "args": args},
	})
	return decode(t, PreToolUse(context.Background(), laneEnv, payload))
}

func decode(t *testing.T, out []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("output %q is not JSON: %v", out, err)
	}
	return m
}

func TestPreToolUse(t *testing.T) {
	root := newLaneRepo(t, "src/**", "docs/*.md")
	keyPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lucind-ai", "attest.key")
	attDir := filepath.Join(os.Getenv("XDG_STATE_HOME"), "lucind-ai", "attestations", "abc")

	tests := []struct {
		name     string
		laneEnv  string
		tool     string
		args     map[string]any
		decision string
		reason   string
	}{
		{"no lane env allows anything", "", "write_to_file", map[string]any{"TargetFile": "/etc/passwd"}, "allow", ""},
		{"write inside allow", laneID, "write_to_file", map[string]any{"TargetFile": filepath.Join(root, "src/a/b.go")}, "allow", ""},
		{"edit inside allow", laneID, "replace_file_content", map[string]any{"TargetFile": filepath.Join(root, "docs/x.md")}, "allow", ""},
		{"multi edit outside allow", laneID, "multi_replace_file_content", map[string]any{"TargetFile": filepath.Join(root, "main.go")}, "deny", "outside"},
		{"nested path not matched by single star", laneID, "write_to_file", map[string]any{"TargetFile": filepath.Join(root, "docs/a/x.md")}, "deny", "outside"},
		{"write outside repo", laneID, "write_to_file", map[string]any{"TargetFile": "/etc/passwd"}, "deny", "outside"},
		{"dot dot escape", laneID, "write_to_file", map[string]any{"TargetFile": filepath.Join(root, "src/../main.go")}, "deny", "outside"},
		{"own result.json", laneID, "write_to_file", map[string]any{"TargetFile": lane.ResultPath(root, laneID)}, "allow", ""},
		{"other lane result.json", laneID, "write_to_file", map[string]any{"TargetFile": lane.ResultPath(root, "20260101-120000-ffff")}, "deny", "outside"},
		{"lane.json is protected", laneID, "write_to_file", map[string]any{"TargetFile": lane.LanePath(root, laneID)}, "deny", "outside"},
		{"write without a path", laneID, "write_to_file", map[string]any{}, "deny", "path"},
		{"run_command allowed", laneID, "run_command", map[string]any{"CommandLine": "echo hi", "Cwd": root}, "allow", ""},
		{"read of unrelated file allowed", laneID, "view_file", map[string]any{"AbsolutePath": filepath.Join(root, "main.go")}, "allow", ""},
		{"read key via view_file", laneID, "view_file", map[string]any{"AbsolutePath": keyPath}, "deny", "attest"},
		{"cat key via run_command", laneID, "run_command", map[string]any{"CommandLine": "cat " + keyPath}, "deny", "attest"},
		{"key tail spelled with variable", laneID, "run_command", map[string]any{"CommandLine": "cat $HOME/.config/lucind-ai/attest.key"}, "deny", "attest"},
		{"write into attestations dir", laneID, "write_to_file", map[string]any{"TargetFile": filepath.Join(attDir, "1.json")}, "deny", "attest"},
		{"list attestations dir", laneID, "run_command", map[string]any{"CommandLine": "ls " + attDir}, "deny", "attest"},
		{"lucind attest run allowed", laneID, "run_command", map[string]any{"CommandLine": "lucind-ai attest run -- sh lucind-checks.sh"}, "allow", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := preToolUse(t, tc.laneEnv, root, tc.tool, tc.args)
			if got["decision"] != tc.decision {
				t.Fatalf("decision = %v (%v), want %s", got["decision"], got["reason"], tc.decision)
			}
			if tc.decision == "deny" {
				reason, _ := got["reason"].(string)
				if !strings.Contains(reason, tc.reason) {
					t.Fatalf("reason %q does not contain %q", reason, tc.reason)
				}
			}
		})
	}
}

func TestPreToolUse_SymlinkEscapeDenied(t *testing.T) {
	root := newLaneRepo(t, "src/**")
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "src", "link")); err != nil {
		t.Fatal(err)
	}
	got := preToolUse(t, laneID, root, "write_to_file", map[string]any{"TargetFile": filepath.Join(root, "src", "link", "x.go")})
	if got["decision"] != "deny" {
		t.Fatalf("symlink escape must be denied, got %v", got)
	}
}

func TestPreToolUse_InternalErrors(t *testing.T) {
	t.Run("lane missing denies and logs", func(t *testing.T) {
		root := newLaneRepo(t, "src/**")
		got := preToolUse(t, "20260101-120000-0000", root, "write_to_file", map[string]any{"TargetFile": filepath.Join(root, "src/a.go")})
		if got["decision"] != "deny" {
			t.Fatalf("got %v, want deny", got)
		}
		log, err := os.ReadFile(filepath.Join(lane.LaneDir(root, "20260101-120000-0000"), "hook.log"))
		if err != nil || len(log) == 0 {
			t.Fatalf("hook.log missing or empty: %v", err)
		}
	})
	t.Run("malformed stdin in lane denies", func(t *testing.T) {
		newLaneRepo(t)
		got := decode(t, PreToolUse(context.Background(), laneID, []byte("not json")))
		if got["decision"] != "deny" {
			t.Fatalf("got %v, want deny", got)
		}
	})
	t.Run("malformed stdin outside lane fails open", func(t *testing.T) {
		got := decode(t, PreToolUse(context.Background(), "", []byte("not json")))
		if got["decision"] != "allow" {
			t.Fatalf("got %v, want allow", got)
		}
	})
	t.Run("invalid lane id in env denies", func(t *testing.T) {
		root := newLaneRepo(t)
		got := preToolUse(t, "../../etc", root, "run_command", map[string]any{"CommandLine": "ls"})
		if got["decision"] != "deny" {
			t.Fatalf("got %v, want deny", got)
		}
	})
}

func stop(t *testing.T, laneEnv, root string) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"workspacePaths": []string{root}, "fullyIdle": true, "executionNum": 1})
	return decode(t, Stop(context.Background(), laneEnv, payload))
}

func writeResult(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(lane.ResultPath(root, laneID), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadLane(t *testing.T, root string) lane.Lane {
	t.Helper()
	l, err := lane.Load(root, laneID)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestStop(t *testing.T) {
	t.Run("no lane env ends", func(t *testing.T) {
		got := decode(t, Stop(context.Background(), "", []byte(`{}`)))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
	})
	t.Run("valid done result marks done", func(t *testing.T) {
		root := newLaneRepo(t)
		writeResult(t, root, validResult)
		if got := stop(t, laneID, root); len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		if s := loadLane(t, root).Status; s != lane.StatusDone {
			t.Fatalf("status = %s, want done", s)
		}
	})
	t.Run("valid non-done result is final failed without retry", func(t *testing.T) {
		root := newLaneRepo(t)
		writeResult(t, root, blockedResult)
		if got := stop(t, laneID, root); len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusFailed || l.Retries != 0 {
			t.Fatalf("status=%s retries=%d, want failed/0", l.Status, l.Retries)
		}
	})
	for _, retries := range []int{0, 1} {
		t.Run("missing result continues and increments retries", func(t *testing.T) {
			root := newLaneRepo(t)
			l := loadLane(t, root)
			l.Retries = retries
			if err := l.Save(root); err != nil {
				t.Fatal(err)
			}
			got := stop(t, laneID, root)
			if got["decision"] != "continue" {
				t.Fatalf("got %v, want continue", got)
			}
			reason, _ := got["reason"].(string)
			if !strings.Contains(reason, lane.ResultPath(root, laneID)) {
				t.Fatalf("reason %q must name the result path", reason)
			}
			l = loadLane(t, root)
			if l.Status != lane.StatusRunning || l.Retries != retries+1 {
				t.Fatalf("status=%s retries=%d, want running/%d", l.Status, l.Retries, retries+1)
			}
		})
	}
	t.Run("schema-invalid result continues with the schema error", func(t *testing.T) {
		root := newLaneRepo(t)
		writeResult(t, root, `{"status":"done"}`)
		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		if reason, _ := got["reason"].(string); !strings.Contains(reason, "schema") {
			t.Fatalf("reason %q should carry the schema error", reason)
		}
	})
	t.Run("invalid at retries 2 fails", func(t *testing.T) {
		root := newLaneRepo(t)
		l := loadLane(t, root)
		l.Retries = 2
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}
		if got := stop(t, laneID, root); len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		if s := loadLane(t, root).Status; s != lane.StatusFailed {
			t.Fatalf("status = %s, want failed", s)
		}
	})
	t.Run("not fully idle is a no-op", func(t *testing.T) {
		root := newLaneRepo(t)
		payload, _ := json.Marshal(map[string]any{"workspacePaths": []string{root}, "fullyIdle": false})
		if got := decode(t, Stop(context.Background(), laneID, payload)); len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 0 {
			t.Fatalf("lane changed: %+v", l)
		}
	})
	t.Run("lane already terminal is untouched", func(t *testing.T) {
		root := newLaneRepo(t)
		l := loadLane(t, root)
		l.Status = lane.StatusTimeout
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}
		if got := stop(t, laneID, root); len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		if s := loadLane(t, root).Status; s != lane.StatusTimeout {
			t.Fatalf("status = %s, want timeout", s)
		}
	})
	t.Run("lane missing ends and logs", func(t *testing.T) {
		root := newLaneRepo(t)
		got := stop(t, "20260101-120000-0000", root)
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		if _, err := os.Stat(filepath.Join(lane.LaneDir(root, "20260101-120000-0000"), "hook.log")); err != nil {
			t.Fatalf("hook.log missing: %v", err)
		}
	})
}
