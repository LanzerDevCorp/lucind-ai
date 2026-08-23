package executor_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// runCursorStream dispatches a stub emitting the given stream and returns the
// decoded progress messages in order.
func runCursorStream(t *testing.T, script string) []string {
	t.Helper()

	stub := writeCursorStub(t, script)
	progress := make(chan executor.ProgressEvent, 32)
	outcome, err := (executor.CursorAgent{Binary: stub}).Run(context.Background(), executor.Request{
		Prompt:       "probe",
		WorktreePath: t.TempDir(),
		Progress:     progress,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", outcome.ExitCode)
	}

	var got []string
	for len(progress) > 0 {
		got = append(got, (<-progress).Message)
	}
	return got
}

// TestCursorAgentRunEmitsToolLifecycleEvents closes the gap that made every
// synthesis lane unmeasurable. cursor-agent is the executor every fan-out
// synthesis runs on, and profiling five real lanes found it had emitted zero
// tool events across 17115 lane_progress rows -- the ledger held nothing but
// narrated thinking text, so 43-71% of each lane's wall clock sat in silences
// that could not be attributed to tool latency or to reasoning. agy and claude
// both emit this vocabulary already; cursor-agent was the one executor that
// did not.
//
// The record shape is the real one, verified against the installed CLI
// (2026.08.11-e8db854): a "tool_call" record carries a "tool_call" object with
// exactly one key naming the variant, "args" on the started record and
// "result" on the completed one.
func TestCursorAgentRunEmitsToolLifecycleEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	got := runCursorStream(t, `#!/bin/sh
printf '%s\n' \
  '{"type":"tool_call","subtype":"started","tool_call":{"shellToolCall":{"args":{"command":"go test ./..."}}},"startedAtMs":"1787500729840"}' \
  '{"type":"tool_call","subtype":"completed","tool_call":{"shellToolCall":{"result":{"exitCode":0}}},"completedAtMs":"1787500731000"}' \
  '{"type":"tool_call","subtype":"started","tool_call":{"grepToolCall":{"args":{"pattern":"RegisterRun"}}},"startedAtMs":"1787500732000"}' \
  '{"type":"tool_call","subtype":"completed","tool_call":{"grepToolCall":{"result":{"matches":3}}},"completedAtMs":"1787500733000"}' \
  '{"type":"result","subtype":"success","result":"done","session_id":"s-1"}'
`)

	want := []string{
		"Tool started: shell",
		"Tool finished: shell",
		"Tool started: grep",
		"Tool finished: grep",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

// TestCursorAgentRunNamesTheEditedFile mirrors what agy's and claude's
// decoders already do: a file-mutating call says which file, because "Edit
// started" with no path is not information anyone can act on.
func TestCursorAgentRunNamesTheEditedFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	got := runCursorStream(t, `#!/bin/sh
printf '%s\n' \
  '{"type":"tool_call","subtype":"started","tool_call":{"editToolCall":{"args":{"path":"/tmp/probe.txt"}}},"startedAtMs":"1787500729840"}' \
  '{"type":"tool_call","subtype":"completed","tool_call":{"editToolCall":{"args":{"path":"/tmp/probe.txt"},"result":{"ok":true}}},"completedAtMs":"1787500729920"}' \
  '{"type":"result","subtype":"success","result":"done","session_id":"s-1"}'
`)

	want := []string{
		"Edit started: edit (/tmp/probe.txt)",
		"Edit finished: edit (/tmp/probe.txt)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

// TestCursorAgentRunReportsFailedToolCall pins the distinction that matters
// most when reading a timeline after the fact. A completed record carrying
// result.error is a tool that ran and failed; reporting it as "finished"
// would make a lane that thrashed against a broken call look like a lane that
// worked. This exact shape was observed on a real dispatch: an edit call
// rejected with "old_string and new_string are exactly the same".
func TestCursorAgentRunReportsFailedToolCall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	got := runCursorStream(t, `#!/bin/sh
printf '%s\n' \
  '{"type":"tool_call","subtype":"started","tool_call":{"editToolCall":{"args":{"path":"/tmp/probe.txt"}}},"startedAtMs":"1787500729840"}' \
  '{"type":"tool_call","subtype":"completed","tool_call":{"editToolCall":{"result":{"error":{"error":"Incorrect tool arguments","modelVisibleError":"old_string and new_string are exactly the same"}}}},"completedAtMs":"1787500729920"}' \
  '{"type":"result","subtype":"success","result":"done","session_id":"s-1"}'
`)

	want := []string{
		"Edit started: edit (/tmp/probe.txt)",
		"Edit failed: edit (/tmp/probe.txt): old_string and new_string are exactly the same",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

// TestCursorAgentRunKeepsDecodingUnknownToolVariants is a forward-compatibility
// guard. The installed CLI ships at least fifteen tool variants and will add
// more; an unrecognized one must still show up on the timeline under its own
// name rather than vanishing, and must never stop the records after it from
// decoding.
func TestCursorAgentRunKeepsDecodingUnknownToolVariants(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	got := runCursorStream(t, `#!/bin/sh
printf '%s\n' \
  '{"type":"tool_call","subtype":"started","tool_call":{"quantumToolCall":{"args":{"spooky":true}}},"startedAtMs":"1787500729840"}' \
  '{"type":"tool_call","subtype":"started","tool_call":{"readToolCall":{"args":{"path":"/tmp/probe.txt"}}},"startedAtMs":"1787500730000"}' \
  '{"type":"result","subtype":"success","result":"done","session_id":"s-1"}'
`)

	want := []string{
		"Tool started: quantum",
		"Tool started: read (/tmp/probe.txt)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

// TestCursorAgentRunStillEmitsThinkingAlongsideTools proves the new tool
// vocabulary is additive. The narrated thinking stream is the only signal the
// ledger has carried until now, and dropping it here would erase history that
// existing consumers already read.
func TestCursorAgentRunStillEmitsThinkingAlongsideTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	got := runCursorStream(t, `#!/bin/sh
printf '%s\n' \
  '{"type":"thinking","subtype":"delta","text":"checking files","timestamp_ms":42000}' \
  '{"type":"tool_call","subtype":"started","tool_call":{"lsToolCall":{"args":{"path":"/tmp"}}},"startedAtMs":"1787500729840"}' \
  '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"tests pass"}]},"timestamp_ms":43000}' \
  '{"type":"result","subtype":"success","result":"done","session_id":"s-1"}'
`)

	want := []string{
		"checking files",
		"Tool started: ls (/tmp)",
		"tests pass",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}
