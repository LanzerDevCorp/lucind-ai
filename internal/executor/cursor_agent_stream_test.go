package executor_test

import (
	"context"
	"reflect"
	"testing"
	"time"

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

// runCursorStreamEvents is runCursorStream's sibling for the assertions that
// care about when an event happened rather than what it said.
func runCursorStreamEvents(t *testing.T, script string) []executor.ProgressEvent {
	t.Helper()

	stub := writeCursorStub(t, script)
	progress := make(chan executor.ProgressEvent, 32)
	if _, err := (executor.CursorAgent{Binary: stub}).Run(context.Background(), executor.Request{
		Prompt:       "probe",
		WorktreePath: t.TempDir(),
		Progress:     progress,
	}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	var got []executor.ProgressEvent
	for len(progress) > 0 {
		got = append(got, <-progress)
	}
	return got
}

// TestCursorAgentRunTimesToolEventsFromInsideTheToolCallObject pins the second
// half of the same defect the sibling-key tests cover. cursor-agent carries
// startedAtMs and completedAtMs INSIDE the tool_call object, not at the
// record's top level, and sends no top-level timestamp_ms on a tool_call
// record at all. Reading them from the top level yields the zero value, which
// falls back to time.Now() -- so every tool call would be stamped with the
// moment the line was parsed rather than when the tool actually ran, and the
// tool-latency-versus-silence measurement this decoder exists for would be
// silently wrong instead of visibly missing.
func TestCursorAgentRunTimesToolEventsFromInsideTheToolCallObject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	got := runCursorStreamEvents(t, `#!/bin/sh
printf '%s\n' \
  '{"type":"tool_call","subtype":"started","call_id":"c-9","tool_call":{"shellToolCall":{"args":{"command":"go build ./..."}},"hookAdditionalContexts":[],"toolCallId":"c-9","startedAtMs":"1787500729840"}}' \
  '{"type":"tool_call","subtype":"completed","call_id":"c-9","tool_call":{"shellToolCall":{"args":{"command":"go build ./..."},"result":{"exitCode":0}},"hookAdditionalContexts":[],"toolCallId":"c-9","startedAtMs":"1787500729840","completedAtMs":"1787500761840"}}' \
  '{"type":"result","subtype":"success","result":"done","session_id":"s-1"}'
`)

	if len(got) != 2 {
		t.Fatalf("got %d events, want 2: %#v", len(got), got)
	}
	wantStart := time.UnixMilli(1787500729840)
	wantEnd := time.UnixMilli(1787500761840)
	if !got[0].At.Equal(wantStart) {
		t.Errorf("started At = %v, want %v", got[0].At, wantStart)
	}
	if !got[1].At.Equal(wantEnd) {
		t.Errorf("completed At = %v, want %v", got[1].At, wantEnd)
	}
	if elapsed := got[1].At.Sub(got[0].At); elapsed != 32*time.Second {
		t.Errorf("measured tool duration = %v, want 32s", elapsed)
	}
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
  '{"type":"tool_call","subtype":"started","call_id":"c-0","tool_call":{"shellToolCall":{"args":{"command":"go test ./..."}},"hookAdditionalContexts":[],"toolCallId":"c-0","startedAtMs":"1787500729840"}}' \
  '{"type":"tool_call","subtype":"completed","call_id":"c-0","tool_call":{"shellToolCall":{"args":{"command":"go test ./..."},"result":{"exitCode":0}},"hookAdditionalContexts":[],"toolCallId":"c-0","startedAtMs":"1787500729840","completedAtMs":"1787500731000"}}' \
  '{"type":"tool_call","subtype":"started","call_id":"c-1","tool_call":{"grepToolCall":{"args":{"pattern":"RegisterRun"}},"hookAdditionalContexts":[],"toolCallId":"c-1","startedAtMs":"1787500732000"}}' \
  '{"type":"tool_call","subtype":"completed","call_id":"c-1","tool_call":{"grepToolCall":{"args":{"pattern":"RegisterRun"},"result":{"matches":3}},"hookAdditionalContexts":[],"toolCallId":"c-1","startedAtMs":"1787500732000","completedAtMs":"1787500733000"}}' \
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
  '{"type":"tool_call","subtype":"started","call_id":"c-2","tool_call":{"editToolCall":{"args":{"path":"/tmp/probe.txt"}},"hookAdditionalContexts":[],"toolCallId":"c-2","startedAtMs":"1787500729840"}}' \
  '{"type":"tool_call","subtype":"completed","call_id":"c-2","tool_call":{"editToolCall":{"args":{"path":"/tmp/probe.txt"},"result":{"ok":true}},"hookAdditionalContexts":[],"toolCallId":"c-2","startedAtMs":"1787500729840","completedAtMs":"1787500729920"}}' \
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
  '{"type":"tool_call","subtype":"started","call_id":"c-2","tool_call":{"editToolCall":{"args":{"path":"/tmp/probe.txt"}},"hookAdditionalContexts":[],"toolCallId":"c-2","startedAtMs":"1787500729840"}}' \
  '{"type":"tool_call","subtype":"completed","call_id":"c-2","tool_call":{"editToolCall":{"result":{"error":{"error":"Incorrect tool arguments","modelVisibleError":"old_string and new_string are exactly the same"}}},"hookAdditionalContexts":[],"toolCallId":"c-2","startedAtMs":"1787500729840","completedAtMs":"1787500729920"}}' \
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
  '{"type":"tool_call","subtype":"started","call_id":"c-3","tool_call":{"quantumToolCall":{"args":{"spooky":true}},"hookAdditionalContexts":[],"toolCallId":"c-3","startedAtMs":"1787500729840"}}' \
  '{"type":"tool_call","subtype":"started","call_id":"c-4","tool_call":{"readToolCall":{"args":{"path":"/tmp/probe.txt"}},"hookAdditionalContexts":[],"toolCallId":"c-4","startedAtMs":"1787500730000"}}' \
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
  '{"type":"tool_call","subtype":"started","call_id":"c-5","tool_call":{"lsToolCall":{"args":{"path":"/tmp"}},"hookAdditionalContexts":[],"toolCallId":"c-5","startedAtMs":"1787500729840"}}' \
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
