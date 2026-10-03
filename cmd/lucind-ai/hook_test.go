package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyhooks"
)

func writeValidEnvelope(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := `{
  "packet_id": "lane-test",
  "status": "done",
  "summary": "Minimal valid envelope for testing.",
  "hard_stops": []
}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write envelope: %v", err)
	}
}

func TestHookStop_NotFullyIdle(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	resultPath := filepath.Join(t.TempDir(), "result.json")

	payload := `{"terminationReason":"NO_TOOL_CALL","fullyIdle":false}`
	var stdout, stderr bytes.Buffer
	code := hookDispatch(context.Background(), []string{
		"stop",
		"--state-dir", stateDir,
		"--result", resultPath,
	}, strings.NewReader(payload), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Fatalf("stdout = %q, want \"{}\"", stdout.String())
	}

	// Verify no files were created
	if entries, err := os.ReadDir(stateDir); err == nil && len(entries) > 0 {
		t.Fatalf("stateDir has files: %v, expected none", entries)
	}
}

func TestHookStop_ValidEnvelope(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	resultPath := filepath.Join(t.TempDir(), "result.json")
	writeValidEnvelope(t, resultPath)

	payload := `{"terminationReason":"NO_TOOL_CALL","fullyIdle":true}`
	var stdout, stderr bytes.Buffer
	code := hookDispatch(context.Background(), []string{
		"stop",
		"--state-dir", stateDir,
		"--result", resultPath,
	}, strings.NewReader(payload), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Fatalf("stdout = %q, want \"{}\"", stdout.String())
	}

	done, exists, err := agyhooks.ReadDone(stateDir)
	if err != nil {
		t.Fatalf("ReadDone failed: %v", err)
	}
	if !exists {
		t.Fatalf("done.json was not created")
	}
	if done.Status != "valid" {
		t.Errorf("done.Status = %q, want %q", done.Status, "valid")
	}
	if done.TerminationReason != "NO_TOOL_CALL" {
		t.Errorf("done.TerminationReason = %q, want %q", done.TerminationReason, "NO_TOOL_CALL")
	}
	if done.At == "" {
		t.Errorf("done.At is empty")
	}
	if done.Error != "" {
		t.Errorf("done.Error = %q, want empty", done.Error)
	}
}

func TestHookStop_ContinuesCounterAndInvalid(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	resultPath := filepath.Join(t.TempDir(), "missing-result.json")

	payload := `{"terminationReason":"NO_TOOL_CALL","fullyIdle":true}`

	// 1. First invocation: missing envelope => continue JSON with reason, counter becomes 1
	{
		var stdout, stderr bytes.Buffer
		code := hookDispatch(context.Background(), []string{
			"stop",
			"--state-dir", stateDir,
			"--result", resultPath,
			"--max-continues", "2",
		}, strings.NewReader(payload), &stdout, &stderr)

		if code != 0 {
			t.Fatalf("invocation 1 exit code = %d, want 0", code)
		}

		var resp struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
			t.Fatalf("invocation 1 stdout is not valid JSON: %v (raw: %q)", err, stdout.String())
		}
		if resp.Decision != "continue" {
			t.Errorf("invocation 1 decision = %q, want %q", resp.Decision, "continue")
		}
		if !strings.Contains(resp.Reason, "missing or invalid") {
			t.Errorf("invocation 1 reason missing 'missing or invalid': %q", resp.Reason)
		}
		if !strings.Contains(resp.Reason, resultPath) {
			t.Errorf("invocation 1 reason missing result path: %q", resp.Reason)
		}
		if !strings.Contains(resp.Reason, ".lucind/result.schema.json") {
			t.Errorf("invocation 1 reason missing schema reference: %q", resp.Reason)
		}
		if !strings.Contains(resp.Reason, "valid envelope before stopping") {
			t.Errorf("invocation 1 reason missing call to action: %q", resp.Reason)
		}

		cData, err := os.ReadFile(filepath.Join(stateDir, "continues"))
		if err != nil {
			t.Fatalf("read continues: %v", err)
		}
		if strings.TrimSpace(string(cData)) != "1" {
			t.Errorf("continues = %q, want \"1\"", string(cData))
		}

		_, exists, _ := agyhooks.ReadDone(stateDir)
		if exists {
			t.Fatalf("done.json should not exist after invocation 1")
		}
	}

	// 2. Second invocation: invalid/missing => continue (counter becomes 2)
	{
		var stdout, stderr bytes.Buffer
		code := hookDispatch(context.Background(), []string{
			"stop",
			"--state-dir", stateDir,
			"--result", resultPath,
			"--max-continues", "2",
		}, strings.NewReader(payload), &stdout, &stderr)

		if code != 0 {
			t.Fatalf("invocation 2 exit code = %d, want 0", code)
		}

		var resp struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
			t.Fatalf("invocation 2 stdout is not valid JSON: %v (raw: %q)", err, stdout.String())
		}
		if resp.Decision != "continue" {
			t.Errorf("invocation 2 decision = %q, want %q", resp.Decision, "continue")
		}

		cData, err := os.ReadFile(filepath.Join(stateDir, "continues"))
		if err != nil {
			t.Fatalf("read continues: %v", err)
		}
		if strings.TrimSpace(string(cData)) != "2" {
			t.Errorf("continues = %q, want \"2\"", string(cData))
		}

		_, exists, _ := agyhooks.ReadDone(stateDir)
		if exists {
			t.Fatalf("done.json should not exist after invocation 2")
		}
	}

	// 3. Third invocation: counter == 2 >= max-continues (2) => {} and done.json invalid
	{
		var stdout, stderr bytes.Buffer
		code := hookDispatch(context.Background(), []string{
			"stop",
			"--state-dir", stateDir,
			"--result", resultPath,
			"--max-continues", "2",
		}, strings.NewReader(payload), &stdout, &stderr)

		if code != 0 {
			t.Fatalf("invocation 3 exit code = %d, want 0", code)
		}
		if strings.TrimSpace(stdout.String()) != "{}" {
			t.Fatalf("invocation 3 stdout = %q, want \"{}\"", stdout.String())
		}

		done, exists, err := agyhooks.ReadDone(stateDir)
		if err != nil {
			t.Fatalf("ReadDone failed: %v", err)
		}
		if !exists {
			t.Fatalf("done.json was not created on third invocation")
		}
		if done.Status != "invalid" {
			t.Errorf("done.Status = %q, want %q", done.Status, "invalid")
		}
		if done.TerminationReason != "NO_TOOL_CALL" {
			t.Errorf("done.TerminationReason = %q, want %q", done.TerminationReason, "NO_TOOL_CALL")
		}
		if done.Error == "" {
			t.Errorf("done.Error is empty, want error text")
		}
		if done.At == "" {
			t.Errorf("done.At is empty")
		}
	}
}

func TestHookStop_GarbageStdin(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	resultPath := filepath.Join(t.TempDir(), "result.json")

	var stdout, stderr bytes.Buffer
	code := hookDispatch(context.Background(), []string{
		"stop",
		"--state-dir", stateDir,
		"--result", resultPath,
	}, strings.NewReader("not valid json at all"), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Fatalf("stdout = %q, want \"{}\"", stdout.String())
	}

	done, exists, err := agyhooks.ReadDone(stateDir)
	if err != nil {
		t.Fatalf("ReadDone failed: %v", err)
	}
	if !exists {
		t.Fatalf("done.json was not created")
	}
	if done.Status != "hook_error" {
		t.Errorf("done.Status = %q, want %q", done.Status, "hook_error")
	}
	if done.Error == "" {
		t.Errorf("done.Error is empty, want decode error")
	}
}

func TestHookStop_RelativeResult(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")

	payload := `{"terminationReason":"NO_TOOL_CALL","fullyIdle":true}`
	var stdout, stderr bytes.Buffer
	code := hookDispatch(context.Background(), []string{
		"stop",
		"--state-dir", stateDir,
		"--result", "relative/path/result.json",
	}, strings.NewReader(payload), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Fatalf("stdout = %q, want \"{}\"", stdout.String())
	}

	done, exists, err := agyhooks.ReadDone(stateDir)
	if err != nil {
		t.Fatalf("ReadDone failed: %v", err)
	}
	if !exists {
		t.Fatalf("done.json was not created")
	}
	if done.Status != "hook_error" {
		t.Errorf("done.Status = %q, want %q", done.Status, "hook_error")
	}
	if done.TerminationReason != "NO_TOOL_CALL" {
		t.Errorf("done.TerminationReason = %q, want %q", done.TerminationReason, "NO_TOOL_CALL")
	}
	if !strings.Contains(done.Error, "absolute") {
		t.Errorf("done.Error = %q, want error mentioning absolute path", done.Error)
	}
}

func TestHookStop_PipeExecution(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	resultPath := filepath.Join(t.TempDir(), "result.json")
	writeValidEnvelope(t, resultPath)

	pr, pw := io.Pipe()
	go func() {
		_ = json.NewEncoder(pw).Encode(map[string]any{
			"terminationReason": "STOP_REASON",
			"fullyIdle":         true,
		})
		_ = pw.Close()
	}()

	var stdout, stderr bytes.Buffer
	code := hookDispatch(context.Background(), []string{
		"stop",
		"--state-dir", stateDir,
		"--result", resultPath,
	}, pr, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}

	var parsed map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("stdout was not valid JSON: %v (raw: %q)", err, stdout.String())
	}

	done, exists, err := agyhooks.ReadDone(stateDir)
	if err != nil || !exists {
		t.Fatalf("ReadDone: exists=%t, err=%v", exists, err)
	}
	if done.Status != "valid" || done.TerminationReason != "STOP_REASON" {
		t.Errorf("done = %+v, want valid with STOP_REASON", done)
	}
}

func TestHookDispatch_EdgeCases(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// No subcommand
	code := hookDispatch(context.Background(), nil, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("no args: code=%d, stdout=%q", code, stdout.String())
	}

	// Unknown subcommand
	stdout.Reset()
	stderr.Reset()
	code = hookDispatch(context.Background(), []string{"unknown"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("unknown subcommand: code=%d, stdout=%q", code, stdout.String())
	}

	// Missing state-dir
	stdout.Reset()
	stderr.Reset()
	code = hookDispatch(context.Background(), []string{"stop"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("missing state-dir: code=%d, stdout=%q", code, stdout.String())
	}
}
