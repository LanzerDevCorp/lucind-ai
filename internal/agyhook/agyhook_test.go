package agyhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

const laneID = "20260101-120000-abcd"

const validResult = `{"lane_id":"x","status":"done","summary":"ok","hard_stops":[]}`
const blockedResult = `{"lane_id":"x","status":"blocked","summary":"stuck","hard_stops":[]}`

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
	l := loadLane(t, root)

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
		{"own result.json", laneID, "write_to_file", map[string]any{"TargetFile": lane.ResultFilePath(root, l)}, "allow", ""},
		{"other lane result.json", laneID, "write_to_file", map[string]any{"TargetFile": lane.ResultFilePath(root, lane.Lane{ID: "20260101-120000-ffff"})}, "deny", "outside"},
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
	l := loadLane(t, root)
	if err := os.WriteFile(lane.ResultFilePath(root, l), []byte(body), 0o644); err != nil {
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
			if !strings.Contains(reason, lane.ResultFilePath(root, l)) {
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

func TestStop_RetryLogging(t *testing.T) {
	t.Run("missing result writes retry log with reason", func(t *testing.T) {
		root := newLaneRepo(t)
		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		logContent := string(logBytes)
		wantLog1 := "stop: retry 1/2: the file does not exist"
		if !strings.Contains(logContent, wantLog1) {
			t.Fatalf("hook.log does not contain %q; got:\n%s", wantLog1, logContent)
		}

		// Second retry
		got = stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		logBytes, err = os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		logContent = string(logBytes)
		wantLog2 := "stop: retry 2/2: the file does not exist"
		if !strings.Contains(logContent, wantLog2) {
			t.Fatalf("hook.log does not contain %q; got:\n%s", wantLog2, logContent)
		}
	})

	t.Run("invalid result writes retry log with error message", func(t *testing.T) {
		root := newLaneRepo(t)
		writeResult(t, root, `{"status":"done"}`)
		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		logContent := string(logBytes)
		if !strings.Contains(logContent, "stop: retry 1/2:") || !strings.Contains(logContent, "schema") {
			t.Fatalf("hook.log missing retry line with schema error; got:\n%s", logContent)
		}
	})

	t.Run("valid result writes no retry log lines", func(t *testing.T) {
		root := newLaneRepo(t)
		writeResult(t, root, validResult)
		if got := stop(t, laneID, root); len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		logContent := string(logBytes)
		if strings.Contains(logContent, "stop: retry") {
			t.Fatalf("hook.log should not contain any retry lines for valid result; got:\n%s", logContent)
		}
	})
}

func TestStop_PayloadLogging(t *testing.T) {
	t.Run("payload logged with all fields and fullyIdle true", func(t *testing.T) {
		root := newLaneRepo(t)
		payload := []byte(`{
			"workspacePaths": ["` + root + `"],
			"executionNum": 3,
			"terminationReason": "goal_achieved",
			"fullyIdle": true,
			"error": ""
		}`)
		got := decode(t, Stop(context.Background(), laneID, payload))
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		want := `stop: payload executionNum=3 terminationReason=goal_achieved fullyIdle=true error=""`
		if !strings.Contains(string(logBytes), want) {
			t.Fatalf("hook.log does not contain %q; got:\n%s", want, string(logBytes))
		}
	})

	t.Run("payload logged with fullyIdle false before early return", func(t *testing.T) {
		root := newLaneRepo(t)
		payload := []byte(`{
			"workspacePaths": ["` + root + `"],
			"executionNum": 1,
			"terminationReason": "interrupted",
			"fullyIdle": false,
			"error": "user aborted"
		}`)
		got := decode(t, Stop(context.Background(), laneID, payload))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		want := `stop: payload executionNum=1 terminationReason=interrupted fullyIdle=false error="user aborted"`
		if !strings.Contains(string(logBytes), want) {
			t.Fatalf("hook.log does not contain %q; got:\n%s", want, string(logBytes))
		}
	})

	t.Run("payload logged with fullyIdle absent/unset", func(t *testing.T) {
		root := newLaneRepo(t)
		payload := []byte(`{
			"workspacePaths": ["` + root + `"],
			"executionNum": 42,
			"terminationReason": "max_turns",
			"error": "something went wrong"
		}`)
		got := decode(t, Stop(context.Background(), laneID, payload))
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		want := `stop: payload executionNum=42 terminationReason=max_turns fullyIdle=unset error="something went wrong"`
		if !strings.Contains(string(logBytes), want) {
			t.Fatalf("hook.log does not contain %q; got:\n%s", want, string(logBytes))
		}
	})

	t.Run("invalid json writes nothing to hook.log", func(t *testing.T) {
		root := newLaneRepo(t)
		got := decode(t, Stop(context.Background(), laneID, []byte("not valid json")))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		logPath := filepath.Join(lane.LaneDir(root, laneID), "hook.log")
		if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("hook.log should not exist, got err: %v", err)
		}
	})

	t.Run("invalid lane id writes nothing to hook.log", func(t *testing.T) {
		root := newLaneRepo(t)
		payload := []byte(`{
			"workspacePaths": ["` + root + `"],
			"executionNum": 1,
			"terminationReason": "goal_achieved",
			"fullyIdle": true,
			"error": ""
		}`)
		got := decode(t, Stop(context.Background(), "../../invalid", payload))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		logPath := filepath.Join(lane.LaneDir(root, laneID), "hook.log")
		if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("hook.log should not exist, got err: %v", err)
		}
	})
}

func TestStop_ProgressAwareRetryBudget(t *testing.T) {
	t.Run("two quick stops exhaust budget on third", func(t *testing.T) {
		t.Cleanup(func() { now = time.Now })
		fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		now = func() time.Time { return fakeTime }

		root := newLaneRepo(t)

		// First stop at fakeTime
		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 1: got %v, want continue", got)
		}
		l := loadLane(t, root)
		if l.Retries != 1 || l.Continues != 1 {
			t.Fatalf("stop 1: retries=%d continues=%d, want 1/1", l.Retries, l.Continues)
		}
		if l.LastStopAt == nil || !l.LastStopAt.Equal(fakeTime) {
			t.Fatalf("stop 1: last_stop_at=%v, want %v", l.LastStopAt, fakeTime)
		}

		// Second stop at fakeTime + 5s (quick)
		fakeTime = fakeTime.Add(5 * time.Second)
		got = stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 2: got %v, want continue", got)
		}
		l = loadLane(t, root)
		if l.Retries != 2 || l.Continues != 2 {
			t.Fatalf("stop 2: retries=%d continues=%d, want 2/2", l.Retries, l.Continues)
		}

		// Third stop at fakeTime + 10s (quick)
		fakeTime = fakeTime.Add(5 * time.Second)
		got = stop(t, laneID, root)
		if len(got) != 0 {
			t.Fatalf("stop 3: got %v, want {}", got)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusFailed {
			t.Fatalf("stop 3: status=%s, want failed", l.Status)
		}

		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		wantLog := "stop: lane marked failed (retries, retries=2)"
		if !strings.Contains(string(logBytes), wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, string(logBytes))
		}
	})

	t.Run("two stops separated by RetryQuietWindow reset budget", func(t *testing.T) {
		t.Cleanup(func() { now = time.Now })
		fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		now = func() time.Time { return fakeTime }

		root := newLaneRepo(t)

		// First stop at fakeTime
		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 1: got %v, want continue", got)
		}
		l := loadLane(t, root)
		if l.Retries != 1 || l.Continues != 1 {
			t.Fatalf("stop 1: retries=%d continues=%d, want 1/1", l.Retries, l.Continues)
		}

		// Advance past RetryQuietWindow (65s)
		gap := RetryQuietWindow + 5*time.Second
		fakeTime = fakeTime.Add(gap)
		got = stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 2: got %v, want continue", got)
		}
		l = loadLane(t, root)
		// Reset to 0 then incremented to 1
		if l.Retries != 1 || l.Continues != 2 {
			t.Fatalf("stop 2: retries=%d continues=%d, want 1/2", l.Retries, l.Continues)
		}
		if l.Status != lane.StatusRunning {
			t.Fatalf("stop 2: status=%s, want running", l.Status)
		}

		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		logContent := string(logBytes)
		wantReset := fmt.Sprintf("stop: retry budget reset after %v without a counted stop", gap)
		if !strings.Contains(logContent, wantReset) {
			t.Fatalf("hook.log missing reset log %q; got:\n%s", wantReset, logContent)
		}
	})

	t.Run("quick-slow-quick sequence keeps lane running", func(t *testing.T) {
		t.Cleanup(func() { now = time.Now })
		fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		now = func() time.Time { return fakeTime }

		root := newLaneRepo(t)

		// Stop 1 at t0
		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 1: got %v, want continue", got)
		}
		l := loadLane(t, root)
		if l.Retries != 1 || l.Continues != 1 {
			t.Fatalf("stop 1: retries=%d continues=%d, want 1/1", l.Retries, l.Continues)
		}

		// Quick stop at t0 + 5s (Retries becomes 2)
		fakeTime = fakeTime.Add(5 * time.Second)
		got = stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 2: got %v, want continue", got)
		}
		l = loadLane(t, root)
		if l.Retries != 2 || l.Continues != 2 {
			t.Fatalf("stop 2: retries=%d continues=%d, want 2/2", l.Retries, l.Continues)
		}

		// Slow stop: gap of 1m48s (108s > 60s RetryQuietWindow)
		fakeTime = fakeTime.Add(108 * time.Second)
		got = stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 3: got %v, want continue", got)
		}
		l = loadLane(t, root)
		// Reset to 0, then incremented to 1
		if l.Retries != 1 || l.Continues != 3 {
			t.Fatalf("stop 3: retries=%d continues=%d, want 1/3", l.Retries, l.Continues)
		}

		// Quick stop: gap of 3s
		fakeTime = fakeTime.Add(3 * time.Second)
		got = stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("stop 4: got %v, want continue", got)
		}
		l = loadLane(t, root)
		// Incremented to 2
		if l.Retries != 2 || l.Continues != 4 {
			t.Fatalf("stop 4: retries=%d continues=%d, want 2/4", l.Retries, l.Continues)
		}
		if l.Status != lane.StatusRunning {
			t.Fatalf("status=%s, want running", l.Status)
		}
	})

	t.Run("total cap MaxTotalContinues fails lane even with long gaps", func(t *testing.T) {
		t.Cleanup(func() { now = time.Now })
		fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		now = func() time.Time { return fakeTime }

		root := newLaneRepo(t)

		// Perform MaxTotalContinues (10) stops, each with gap > RetryQuietWindow
		for i := 1; i <= MaxTotalContinues; i++ {
			fakeTime = fakeTime.Add(RetryQuietWindow + 10*time.Second)
			got := stop(t, laneID, root)
			if got["decision"] != "continue" {
				t.Fatalf("stop %d: got %v, want continue", i, got)
			}
			l := loadLane(t, root)
			if l.Continues != i {
				t.Fatalf("stop %d: continues=%d, want %d", i, l.Continues, i)
			}
		}

		// Stop 11: total cap exhausted
		fakeTime = fakeTime.Add(RetryQuietWindow + 10*time.Second)
		got := stop(t, laneID, root)
		if len(got) != 0 {
			t.Fatalf("stop 11: got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusFailed {
			t.Fatalf("stop 11: status=%s, want failed", l.Status)
		}

		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		wantLog := "stop: lane marked failed (total continues, retries=0)"
		if !strings.Contains(string(logBytes), wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, string(logBytes))
		}
	})

	t.Run("fullyIdle=false stops never change Retries Continues or LastStopAt", func(t *testing.T) {
		t.Cleanup(func() { now = time.Now })
		fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		now = func() time.Time { return fakeTime }

		root := newLaneRepo(t)
		stopTime := fakeTime.Add(-10 * time.Minute)
		l := loadLane(t, root)
		l.Retries = 1
		l.Continues = 3
		l.LastStopAt = &stopTime
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}

		payload, _ := json.Marshal(map[string]any{
			"workspacePaths": []string{root},
			"fullyIdle":      false,
			"executionNum":   2,
		})
		got := decode(t, Stop(context.Background(), laneID, payload))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}

		l = loadLane(t, root)
		if l.Retries != 1 {
			t.Fatalf("retries changed: got %d, want 1", l.Retries)
		}
		if l.Continues != 3 {
			t.Fatalf("continues changed: got %d, want 3", l.Continues)
		}
		if l.LastStopAt == nil || !l.LastStopAt.Equal(stopTime) {
			t.Fatalf("lastStopAt changed: got %v, want %v", l.LastStopAt, stopTime)
		}
		if l.Status != lane.StatusRunning {
			t.Fatalf("status changed: got %s, want running", l.Status)
		}
	})

	t.Run("valid result is marked done without touching counters", func(t *testing.T) {
		t.Cleanup(func() { now = time.Now })
		fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		now = func() time.Time { return fakeTime }

		root := newLaneRepo(t)
		stopTime := fakeTime.Add(-5 * time.Minute)
		l := loadLane(t, root)
		l.Retries = 1
		l.Continues = 2
		l.LastStopAt = &stopTime
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}

		writeResult(t, root, validResult)
		got := stop(t, laneID, root)
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}

		l = loadLane(t, root)
		if l.Status != lane.StatusDone {
			t.Fatalf("status=%s, want done", l.Status)
		}
		if l.Retries != 1 {
			t.Fatalf("retries changed: got %d, want 1", l.Retries)
		}
		if l.Continues != 2 {
			t.Fatalf("continues changed: got %d, want 2", l.Continues)
		}
		if l.LastStopAt == nil || !l.LastStopAt.Equal(stopTime) {
			t.Fatalf("lastStopAt changed: got %v, want %v", l.LastStopAt, stopTime)
		}

		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		wantLog := "stop: lane marked done (retries=1)"
		if !strings.Contains(string(logBytes), wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, string(logBytes))
		}
	})

	t.Run("lane.json without new fields loads and behaves like Retries=0", func(t *testing.T) {
		t.Cleanup(func() { now = time.Now })
		fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		now = func() time.Time { return fakeTime }

		root := t.TempDir()
		if real, err := filepath.EvalSymlinks(root); err == nil {
			root = real
		}
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
		t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
		if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v: %s", err, out)
		}
		dir := lane.LaneDir(root, laneID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Write lane.json with only legacy fields (no retries, no last_stop_at, no continues)
		legacyJSON := `{"id":"` + laneID + `","cwd":"` + root + `","status":"running","created_at":"2026-01-01T12:00:00Z","updated_at":"2026-01-01T12:00:00Z"}`
		if err := os.WriteFile(filepath.Join(dir, "lane.json"), []byte(legacyJSON), 0o644); err != nil {
			t.Fatal(err)
		}

		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}

		l := loadLane(t, root)
		if l.Retries != 1 {
			t.Fatalf("retries=%d, want 1", l.Retries)
		}
		if l.Continues != 1 {
			t.Fatalf("continues=%d, want 1", l.Continues)
		}
		if l.LastStopAt == nil || !l.LastStopAt.Equal(fakeTime) {
			t.Fatalf("lastStopAt=%v, want %v", l.LastStopAt, fakeTime)
		}
		if l.Status != lane.StatusRunning {
			t.Fatalf("status=%s, want running", l.Status)
		}
	})
}

func TestPreToolUse_Turn(t *testing.T) {
	root := newLaneRepo(t, "src/**")
	l := loadLane(t, root)
	l.Turn = 2
	if err := l.Save(root); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		target   string
		decision string
		reason   string
	}{
		{
			name:     "turn 2 write to result-2.json allowed",
			target:   lane.ResultFilePath(root, l),
			decision: "allow",
		},
		{
			name:     "turn 2 write to result-1.json denied",
			target:   filepath.Join(lane.LaneDir(root, laneID), "result-1.json"),
			decision: "deny",
			reason:   "outside",
		},
		{
			name:     "turn 2 write to legacy result.json denied",
			target:   filepath.Join(lane.LaneDir(root, laneID), "result.json"),
			decision: "deny",
			reason:   "outside",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := preToolUse(t, laneID, root, "write_to_file", map[string]any{"TargetFile": tc.target})
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

func TestStop_Turn(t *testing.T) {
	t.Run("turn 2 with only valid done result-1.json treated as missing", func(t *testing.T) {
		root := newLaneRepo(t)
		l := loadLane(t, root)
		l.Turn = 2
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}
		r1Path := filepath.Join(lane.LaneDir(root, laneID), "result-1.json")
		if err := os.WriteFile(r1Path, []byte(validResult), 0o644); err != nil {
			t.Fatal(err)
		}

		got := stop(t, laneID, root)
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		reason, _ := got["reason"].(string)
		wantPath := filepath.Join(lane.LaneDir(root, laneID), "result-2.json")
		if !strings.Contains(reason, wantPath) {
			t.Fatalf("reason %q must name current turn file %q", reason, wantPath)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 1 {
			t.Fatalf("status=%s retries=%d, want running/1", l.Status, l.Retries)
		}
	})

	t.Run("turn 2 with valid done result-2.json marks done", func(t *testing.T) {
		root := newLaneRepo(t)
		l := loadLane(t, root)
		l.Turn = 2
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}
		r2Path := filepath.Join(lane.LaneDir(root, laneID), "result-2.json")
		if err := os.WriteFile(r2Path, []byte(validResult), 0o644); err != nil {
			t.Fatal(err)
		}

		got := stop(t, laneID, root)
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusDone {
			t.Fatalf("status=%s, want done", l.Status)
		}
	})

	t.Run("legacy lane with turn 0 and valid result.json marks done", func(t *testing.T) {
		root := newLaneRepo(t)
		l := loadLane(t, root)
		if l.Turn != 0 {
			t.Fatalf("turn=%d, want 0", l.Turn)
		}
		rPath := filepath.Join(lane.LaneDir(root, laneID), "result.json")
		if err := os.WriteFile(rPath, []byte(validResult), 0o644); err != nil {
			t.Fatal(err)
		}

		got := stop(t, laneID, root)
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusDone {
			t.Fatalf("status=%s, want done", l.Status)
		}
	})
}

func TestStop_RawPayload(t *testing.T) {
	t.Run("raw payload logged as single line", func(t *testing.T) {
		root := newLaneRepo(t)
		formattedPayload := []byte("{\n  \"workspacePaths\": [\"" + root + "\"],\n  \"fullyIdle\": true,\n  \"executionNum\": 1,\n  \"customField\": \"hello world\"\n}")
		got := decode(t, Stop(context.Background(), laneID, formattedPayload))
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		logLines := strings.Split(string(logBytes), "\n")
		var rawLine string
		for _, line := range logLines {
			if strings.Contains(line, "stop: raw payload ") {
				rawLine = line
				break
			}
		}
		if rawLine == "" {
			t.Fatalf("hook.log missing 'stop: raw payload ' line; got:\n%s", string(logBytes))
		}
		expectedSubstring := `stop: raw payload {"workspacePaths":["` + root + `"],"fullyIdle":true,"executionNum":1,"customField":"hello world"}`
		if !strings.Contains(rawLine, expectedSubstring) {
			t.Fatalf("raw line does not contain expected compacted json; got %q", rawLine)
		}
	})

	t.Run("payload over 4096 bytes is truncated with marker", func(t *testing.T) {
		root := newLaneRepo(t)
		bigValue := strings.Repeat("a", 5000)
		payload := []byte(`{"workspacePaths":["` + root + `"],"fullyIdle":true,"executionNum":1,"big":"` + bigValue + `"}`)
		got := decode(t, Stop(context.Background(), laneID, payload))
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
		if err != nil {
			t.Fatalf("reading hook.log: %v", err)
		}
		logLines := strings.Split(string(logBytes), "\n")
		var rawLine string
		for _, line := range logLines {
			if strings.Contains(line, "stop: raw payload ") {
				rawLine = line
				break
			}
		}
		if rawLine == "" {
			t.Fatalf("hook.log missing 'stop: raw payload ' line; got:\n%s", string(logBytes))
		}
		if !strings.HasSuffix(rawLine, "[truncated]") {
			t.Fatalf("raw line must end with [truncated]; got %q", rawLine)
		}
		idx := strings.Index(rawLine, "stop: raw payload ")
		payloadPart := rawLine[idx+len("stop: raw payload ") : len(rawLine)-len("[truncated]")]
		if len(payloadPart) > 4096 {
			t.Fatalf("truncated payload part length %d exceeds 4096", len(payloadPart))
		}
	})
}

func createFakeMainTranscript(t *testing.T, dir, laneID string) string {
	t.Helper()
	path := filepath.Join(dir, "main_transcript.jsonl")
	step := fmt.Sprintf(
		`{"source":"USER_EXPLICIT","type":"USER_INPUT","content":"lucind-lane: %s turn: 1\nTask content..."}`+"\n",
		laneID,
	)
	if err := os.WriteFile(path, []byte(step), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func createFakeWorkerTranscript(t *testing.T, dir, parentID string) string {
	t.Helper()
	path := filepath.Join(dir, "worker_transcript.jsonl")
	step := fmt.Sprintf(
		`{"source":"SYSTEM","type":"SYSTEM_MESSAGE","content":"The following is a <SYSTEM_MESSAGE> not actually sent by the user... sender=%s priority=MESSAGE_PRIORITY_HIGH"}`+"\n",
		parentID,
	)
	if err := os.WriteFile(path, []byte(step), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func stopWithConv(t *testing.T, laneEnv, root, convID, transcriptPath string, fullyIdle *bool) map[string]any {
	t.Helper()
	payloadMap := map[string]any{
		"workspacePaths": []string{root},
		"executionNum":   1,
	}
	if fullyIdle != nil {
		payloadMap["fullyIdle"] = *fullyIdle
	}
	if convID != "" {
		payloadMap["conversationId"] = convID
	}
	if transcriptPath != "" {
		payloadMap["transcriptPath"] = transcriptPath
	}
	payload, _ := json.Marshal(payloadMap)
	return decode(t, Stop(context.Background(), laneEnv, payload))
}

func boolPtr(b bool) *bool { return &b }

func readHookLog(t *testing.T, root, laneID string) string {
	t.Helper()
	logBytes, err := os.ReadFile(filepath.Join(lane.LaneDir(root, laneID), "hook.log"))
	if err != nil {
		t.Fatalf("reading hook.log: %v", err)
	}
	return string(logBytes)
}

func TestStop_MainConversation(t *testing.T) {
	t.Run("worker stop fullyIdle true is ignored", func(t *testing.T) {
		root := newLaneRepo(t)
		transcript := createFakeWorkerTranscript(t, t.TempDir(), "parent-1")
		got := stopWithConv(t, laneID, root, "worker-1", transcript, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 0 {
			t.Fatalf("status=%s retries=%d, want running/0", l.Status, l.Retries)
		}
		logContent := readHookLog(t, root, laneID)
		wantLog := "stop: ignored worker conversation worker-1"
		if !strings.Contains(logContent, wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, logContent)
		}
	})

	t.Run("worker stop fullyIdle false is ignored", func(t *testing.T) {
		root := newLaneRepo(t)
		transcript := createFakeWorkerTranscript(t, t.TempDir(), "parent-1")
		got := stopWithConv(t, laneID, root, "worker-2", transcript, boolPtr(false))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 0 {
			t.Fatalf("status=%s retries=%d, want running/0", l.Status, l.Retries)
		}
		logContent := readHookLog(t, root, laneID)
		wantLog := "stop: ignored worker conversation worker-2"
		if !strings.Contains(logContent, wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, logContent)
		}
	})

	t.Run("main stop fullyIdle false is ignored", func(t *testing.T) {
		root := newLaneRepo(t)
		transcript := createFakeMainTranscript(t, t.TempDir(), laneID)
		got := stopWithConv(t, laneID, root, "main-1", transcript, boolPtr(false))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 0 {
			t.Fatalf("status=%s retries=%d, want running/0", l.Status, l.Retries)
		}
		logContent := readHookLog(t, root, laneID)
		wantLog := "stop: main conversation not fully idle"
		if !strings.Contains(logContent, wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, logContent)
		}
	})

	t.Run("main stop fullyIdle true marks done when result valid", func(t *testing.T) {
		root := newLaneRepo(t)
		writeResult(t, root, validResult)
		transcript := createFakeMainTranscript(t, t.TempDir(), laneID)
		got := stopWithConv(t, laneID, root, "main-1", transcript, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusDone {
			t.Fatalf("status=%s, want done", l.Status)
		}
		logContent := readHookLog(t, root, laneID)
		wantLog := "stop: lane marked done"
		if !strings.Contains(logContent, wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, logContent)
		}
	})

	t.Run("main stop fullyIdle true continues when result missing", func(t *testing.T) {
		root := newLaneRepo(t)
		transcript := createFakeMainTranscript(t, t.TempDir(), laneID)
		got := stopWithConv(t, laneID, root, "main-1", transcript, boolPtr(true))
		if got["decision"] != "continue" {
			t.Fatalf("got %v, want continue", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 1 {
			t.Fatalf("status=%s retries=%d, want running/1", l.Status, l.Retries)
		}
		logContent := readHookLog(t, root, laneID)
		wantLog := "stop: retry 1/2:"
		if !strings.Contains(logContent, wantLog) {
			t.Fatalf("hook.log missing %q; got:\n%s", wantLog, logContent)
		}
	})

	t.Run("continuation turn with different main conversation ID", func(t *testing.T) {
		root := newLaneRepo(t)
		l := loadLane(t, root)
		l.Turn = 1
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}
		writeResult(t, root, validResult)

		// Turn 1 completes with main-1
		transcript1 := createFakeMainTranscript(t, t.TempDir(), laneID)
		got := stopWithConv(t, laneID, root, "main-1", transcript1, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("turn 1 stop: got %v, want {}", got)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusDone {
			t.Fatalf("turn 1 status=%s, want done", l.Status)
		}

		// Continue lane to turn 2
		l.Turn = 2
		l.Status = lane.StatusRunning
		l.Retries = 0
		l.Continues = 0
		if err := l.Save(root); err != nil {
			t.Fatal(err)
		}

		// Worker stop from turn 1 is ignored
		workerTranscript := createFakeWorkerTranscript(t, t.TempDir(), "main-1")
		got = stopWithConv(t, laneID, root, "worker-turn1", workerTranscript, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("worker stop: got %v, want {}", got)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 0 {
			t.Fatalf("after worker stop: status=%s retries=%d, want running/0", l.Status, l.Retries)
		}

		// Write turn 2 result
		r2Path := filepath.Join(lane.LaneDir(root, laneID), "result-2.json")
		if err := os.WriteFile(r2Path, []byte(validResult), 0o644); err != nil {
			t.Fatal(err)
		}

		// Turn 2 main conversation main-2 decides turn 2
		transcript2 := createFakeMainTranscript(t, t.TempDir(), laneID)
		got = stopWithConv(t, laneID, root, "main-2", transcript2, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("turn 2 stop: got %v, want {}", got)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusDone {
			t.Fatalf("turn 2 status=%s, want done", l.Status)
		}
	})

	t.Run("fallback when transcript unreadable or marker unknown", func(t *testing.T) {
		unknownTranscript := filepath.Join(t.TempDir(), "unknown.jsonl")
		if err := os.WriteFile(unknownTranscript, []byte(`{"source":"OTHER","content":"something"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		missingTranscript := filepath.Join(t.TempDir(), "nonexistent.jsonl")

		cases := []struct {
			name           string
			transcriptPath string
		}{
			{"unknown marker", unknownTranscript},
			{"missing transcript file", missingTranscript},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := newLaneRepo(t)

				// fullyIdle=false never decides
				got := stopWithConv(t, laneID, root, "conv-unknown", tc.transcriptPath, boolPtr(false))
				if len(got) != 0 {
					t.Fatalf("fullyIdle=false: got %v, want {}", got)
				}
				l := loadLane(t, root)
				if l.Status != lane.StatusRunning || l.Retries != 0 {
					t.Fatalf("fullyIdle=false: status=%s retries=%d, want running/0", l.Status, l.Retries)
				}
				logContent := readHookLog(t, root, laneID)
				wantFallbackLog := "stop: transcript unreadable or marker unknown"
				if !strings.Contains(logContent, wantFallbackLog) {
					t.Fatalf("hook.log missing %q; got:\n%s", wantFallbackLog, logContent)
				}

				// fullyIdle=true proceeds (in this case, missing result causes retry)
				got = stopWithConv(t, laneID, root, "conv-unknown", tc.transcriptPath, boolPtr(true))
				if got["decision"] != "continue" {
					t.Fatalf("fullyIdle=true: got %v, want continue", got)
				}
				l = loadLane(t, root)
				if l.Status != lane.StatusRunning || l.Retries != 1 {
					t.Fatalf("fullyIdle=true: status=%s retries=%d, want running/1", l.Status, l.Retries)
				}
			})
		}
	})

	t.Run("caching per conversation ID in conversations dir", func(t *testing.T) {
		root := newLaneRepo(t)
		tempDir := t.TempDir()
		workerTranscript := createFakeWorkerTranscript(t, tempDir, "parent-x")
		mainTranscript := createFakeMainTranscript(t, tempDir, laneID)

		// 1. Worker classification and cache
		got := stopWithConv(t, laneID, root, "worker-cached", workerTranscript, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("worker stop: got %v, want {}", got)
		}
		cacheFile := filepath.Join(lane.LaneDir(root, laneID), "conversations", "worker-cached")
		data, err := os.ReadFile(cacheFile)
		if err != nil {
			t.Fatalf("reading cache file %s: %v", cacheFile, err)
		}
		if string(data) != "worker" {
			t.Fatalf("cached role = %q, want worker", string(data))
		}

		// Delete worker transcript; subsequent stop must use cache
		if err := os.Remove(workerTranscript); err != nil {
			t.Fatal(err)
		}
		got = stopWithConv(t, laneID, root, "worker-cached", workerTranscript, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("worker stop after transcript delete: got %v, want {}", got)
		}
		l := loadLane(t, root)
		if l.Status != lane.StatusRunning || l.Retries != 0 {
			t.Fatalf("status=%s retries=%d, want running/0", l.Status, l.Retries)
		}

		// 2. Main classification and cache
		got = stopWithConv(t, laneID, root, "main-cached", mainTranscript, boolPtr(false))
		if len(got) != 0 {
			t.Fatalf("main stop: got %v, want {}", got)
		}
		mainCacheFile := filepath.Join(lane.LaneDir(root, laneID), "conversations", "main-cached")
		data, err = os.ReadFile(mainCacheFile)
		if err != nil {
			t.Fatalf("reading main cache file %s: %v", mainCacheFile, err)
		}
		if string(data) != "main" {
			t.Fatalf("cached role = %q, want main", string(data))
		}

		// Delete main transcript, write valid result; subsequent stop must use cache and mark done
		if err := os.Remove(mainTranscript); err != nil {
			t.Fatal(err)
		}
		writeResult(t, root, validResult)
		got = stopWithConv(t, laneID, root, "main-cached", mainTranscript, boolPtr(true))
		if len(got) != 0 {
			t.Fatalf("main stop after transcript delete: got %v, want {}", got)
		}
		l = loadLane(t, root)
		if l.Status != lane.StatusDone {
			t.Fatalf("status=%s, want done", l.Status)
		}
	})
}

func TestClassifyConversation(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		content  string
		wantRole role
		wantErr  bool
	}{
		{
			name:     "main conversation with USER_EXPLICIT and marker",
			source:   "USER_EXPLICIT",
			content:  "lucind-lane: " + laneID + " turn: 1\nTask description",
			wantRole: roleMain,
		},
		{
			name:     "main conversation with empty source and marker",
			source:   "",
			content:  "lucind-lane: " + laneID,
			wantRole: roleMain,
		},
		{
			name:     "old read and follow pointer is not main",
			source:   "USER_EXPLICIT",
			content:  "Read and follow /repo/.lucind/lanes/" + laneID + "/brief.md",
			wantRole: roleUnknown,
			wantErr:  true,
		},
		{
			name:     "worker with SYSTEM source",
			source:   "SYSTEM",
			content:  "The following is a <SYSTEM_MESSAGE> not actually sent by the user... sender=parent-1 priority=MESSAGE_PRIORITY_HIGH",
			wantRole: roleWorker,
		},
		{
			name:     "other source with marker is not main",
			source:   "OTHER",
			content:  "lucind-lane: " + laneID,
			wantRole: roleUnknown,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "transcript.jsonl")
			step := map[string]any{
				"source":  tt.source,
				"content": tt.content,
			}
			data, err := json.Marshal(step)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}

			role, err := classifyConversation(laneID, path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("classifyConversation() err = %v, wantErr %v", err, tt.wantErr)
			}
			if role != tt.wantRole {
				t.Fatalf("classifyConversation() role = %v, want %v", role, tt.wantRole)
			}
		})
	}
}



