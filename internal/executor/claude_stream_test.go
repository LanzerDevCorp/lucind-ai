package executor_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// writeClaudeStreamStub writes a stub that records every --output-format it
// was invoked with, so a test can prove how many real dispatches happened --
// the cost question, not a style one: this executor is pinned to Opus and a
// silent second dispatch would bill a whole run twice and re-apply the first
// run's side effects on top of themselves.
func writeClaudeStreamStub(t *testing.T, stream string) (binary, callsFile string) {
	t.Helper()
	callsFile = filepath.Join(t.TempDir(), "calls.txt")
	script := fmt.Sprintf(`#!/bin/sh
format=""
previous=""
for argument in "$@"; do
	if [ "$previous" = "--output-format" ]; then format="$argument"; fi
	previous="$argument"
done
echo "$format" >> %q
if [ "$format" = "stream-json" ]; then
	cat <<'CLAUDE_STREAM_EOF'
%s
CLAUDE_STREAM_EOF
else
	printf '%%s\n' '{"type":"result","subtype":"success","result":"blocking result"}'
fi
`, callsFile, stream)
	return writeClaudeStub(t, script), callsFile
}

// TestClaudeRunSendsVerboseWithStreamJSON is the load-bearing guard of this
// file, in the same family as TestClaudeRunNeverSendsPrintTimeout: claude
// refuses the combination outright rather than degrading. Verified against
// the real CLI bundle (2.1.241), which guards the flag pair with
// `if (outputFormat === "stream-json" && !verbose)`, writes `Error: When
// using --print, --output-format=stream-json requires --verbose` to stderr,
// and exits 1 without ever dispatching. Dropping --verbose here would not
// lose progress, it would break every progress dispatch.
func TestClaudeRunSendsVerboseWithStreamJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		Progress:     make(chan executor.ProgressEvent, 4),
	})

	if got, ok := flagValue(argv, "--output-format"); !ok || got != "stream-json" {
		t.Errorf("argv = %v, want --output-format stream-json, got %q (ok=%v)", argv, got, ok)
	}
	if !containsFlag(argv, "--verbose") {
		t.Errorf("argv = %v, want --verbose -- claude exits 1 on stream-json without it", argv)
	}
}

// TestClaudeRunStreamPathKeepsPromptBehindDoubleDash re-pins on the stream
// path what TestClaudeRunPassesPromptAfterDoubleDash pins on the blocking
// one. The separator is not per-format defensive style: a Markdown packet
// body that opens with a bullet dies as an unknown option either way.
func TestClaudeRunStreamPathKeepsPromptBehindDoubleDash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const prompt = "- resolve the conflict"
	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       prompt,
		WorktreePath: t.TempDir(),
		Progress:     make(chan executor.ProgressEvent, 4),
	})

	if len(argv) < 2 {
		t.Fatalf("argv = %v, want at least a separator and a prompt", argv)
	}
	if got := argv[len(argv)-1]; got != prompt {
		t.Errorf("argv[last] = %q, want the prompt %q", got, prompt)
	}
	if got := argv[len(argv)-2]; got != "--" {
		t.Errorf("argv[last-1] = %q, want %q", got, "--")
	}
}

// TestClaudeRunOmitsVerboseWithoutProgress keeps the established blocking
// path byte-identical for callers that never asked for progress. --verbose
// is only ever legal alongside stream-json here, and adding it to the JSON
// path would change that path's output for no caller's benefit.
func TestClaudeRunOmitsVerboseWithoutProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})

	if got, ok := flagValue(argv, "--output-format"); !ok || got != "json" {
		t.Errorf("argv = %v, want --output-format json when Progress is nil, got %q (ok=%v)", argv, got, ok)
	}
	if containsFlag(argv, "--verbose") {
		t.Errorf("argv = %v, want no --verbose on the blocking JSON path", argv)
	}
}

func TestClaudeRunStreamsNormalizedProgressAndPreservesFinalResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","cwd":"/tmp/probe","session_id":"session-1","model":"claude-opus-5"}`,
		`{"type":"stream_event","event":{"type":"content_block_delta"},"session_id":"session-1"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Reading the conflict"}]},"session_id":"session-1"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"private reasoning"}]},"session_id":"session-1"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"git status"}}]},"session_id":"session-1"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"clean"}]},"session_id":"session-1"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_2","name":"Write","input":{"file_path":"/tmp/probe.txt"}}]},"session_id":"session-1"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"written"}]},"session_id":"session-1"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_3","name":"Edit","input":{"file_path":"/tmp/probe.txt"}}]},"session_id":"session-1"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_3","content":"boom","is_error":true}]},"session_id":"session-1"}`,
		`{"type":"result","subtype":"success","is_error":false,"num_turns":4,"result":"CLAUDE_STREAM_DONE","session_id":"session-1","total_cost_usd":0.421,"usage":{"input_tokens":64264,"output_tokens":2768,"cache_creation_input_tokens":512,"cache_read_input_tokens":81443}}`,
	}, "\n")
	binary, callsFile := writeClaudeStreamStub(t, stream)
	progress := make(chan executor.ProgressEvent, 32)

	outcome, err := (executor.Claude{Binary: binary}).Run(context.Background(), executor.Request{
		Prompt:   "probe",
		Progress: progress,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	const wantStdout = `{"type":"result","subtype":"success","is_error":false,"num_turns":4,"result":"CLAUDE_STREAM_DONE","session_id":"session-1","total_cost_usd":0.421,"usage":{"input_tokens":64264,"output_tokens":2768,"cache_creation_input_tokens":512,"cache_read_input_tokens":81443}}` + "\n"
	if outcome.Stdout != wantStdout || outcome.ExitCode != 0 {
		t.Errorf("Outcome = %#v, want exit 0 and the terminal result record as stdout %q", outcome, wantStdout)
	}

	wantMessages := []string{
		"Reading the conflict",
		"Tool started: Bash",
		"Tool finished: Bash",
		"Edit started: Write (/tmp/probe.txt)",
		"Edit finished: Write (/tmp/probe.txt)",
		"Edit started: Edit (/tmp/probe.txt)",
		"Edit failed: Edit (/tmp/probe.txt)",
		"Usage: 64264 input, 2768 output, 81443 cache read, 512 cache write tokens, $0.4210",
	}
	if got := progressMessages(progress); !reflect.DeepEqual(got, wantMessages) {
		t.Errorf("progress messages = %#v, want %#v", got, wantMessages)
	}

	calls, err := os.ReadFile(callsFile)
	if err != nil {
		t.Fatalf("ReadFile(calls) error = %v", err)
	}
	if got := string(calls); got != "stream-json\n" {
		t.Errorf("output formats invoked = %q, want exactly one stream-json invocation", got)
	}
}

// TestClaudeRunReportsNonSuccessResultSubtype surfaces the one thing a
// timeline reader cannot infer from an exit code: claude exits 0 on
// error_max_turns, so a run that stopped short of finishing its work looks
// identical to a completed one unless the subtype is said out loud.
func TestClaudeRunReportsNonSuccessResultSubtype(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stream := `{"type":"result","subtype":"error_max_turns","is_error":true,"result":"","session_id":"session-1","usage":{"input_tokens":10,"output_tokens":20}}`
	binary, _ := writeClaudeStreamStub(t, stream)
	progress := make(chan executor.ProgressEvent, 8)

	if _, err := (executor.Claude{Binary: binary}).Run(context.Background(), executor.Request{
		Prompt:   "probe",
		Progress: progress,
	}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	want := []string{
		"Result: error_max_turns",
		"Usage: 10 input, 20 output, 0 cache read, 0 cache write tokens",
	}
	if got := progressMessages(progress); !reflect.DeepEqual(got, want) {
		t.Errorf("progress messages = %#v, want %#v", got, want)
	}
}

// TestClaudeRunNeverRedispatchesWithoutTerminalResult is a cost guard, and
// the one place this decoder deliberately departs from Agy.Run. Agy replays
// the turn as blocking JSON when the stream carried no terminal record;
// claude must not. Outcome.Stdout is diagnosis-only here (the real result
// envelope is read from the worktree's .lucind/result.json), the whole raw
// stream is still captured, and a replay would bill a second full Opus run
// while re-applying the first run's edits on top of themselves.
func TestClaudeRunNeverRedispatchesWithoutTerminalResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stream := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"cut off mid-turn"}]},"session_id":"session-1"}`
	binary, callsFile := writeClaudeStreamStub(t, stream)
	progress := make(chan executor.ProgressEvent, 8)

	outcome, err := (executor.Claude{Binary: binary}).Run(context.Background(), executor.Request{
		Prompt:   "probe",
		Progress: progress,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	calls, err := os.ReadFile(callsFile)
	if err != nil {
		t.Fatalf("ReadFile(calls) error = %v", err)
	}
	if got := string(calls); got != "stream-json\n" {
		t.Errorf("output formats invoked = %q, want exactly one -- a replay bills a second Opus run", got)
	}
	if !strings.Contains(outcome.Stdout, "cut off mid-turn") {
		t.Errorf("Stdout = %q, want the raw stream retained when no terminal record arrived", outcome.Stdout)
	}
	if got := progressMessages(progress); !reflect.DeepEqual(got, []string{"cut off mid-turn"}) {
		t.Errorf("progress messages = %#v, want the decoded text", got)
	}
}

// TestClaudeRunNotesDegradedStreamOnceAndKeepsDecoding covers a line that
// does not parse. Unlike opencode's decoder, decoding is not switched off:
// one unreadable line (a truncated write, or non-JSON chatter --verbose
// might add) must not blind the rest of a dispatch's timeline. It is
// announced exactly once so the degradation is visible rather than silent.
func TestClaudeRunNotesDegradedStreamOnceAndKeepsDecoding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stream := strings.Join([]string{
		`not json at all`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"still decoding"}]}}`,
		`{"type":"assistant","message":{"role":"assistant",`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"and still"}]}}`,
	}, "\n")
	binary, _ := writeClaudeStreamStub(t, stream)
	progress := make(chan executor.ProgressEvent, 8)

	if _, err := (executor.Claude{Binary: binary}).Run(context.Background(), executor.Request{
		Prompt:   "probe",
		Progress: progress,
	}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	want := []string{
		"Claude progress decoding degraded; raw stream retained as stdout",
		"still decoding",
		"and still",
	}
	if got := progressMessages(progress); !reflect.DeepEqual(got, want) {
		t.Errorf("progress messages = %#v, want %#v", got, want)
	}
}

// TestClaudeRunIgnoresRecordsAfterTerminalResult mirrors the same rule the
// agy decoder enforces: once the terminal record has been seen the turn is
// over, and trailing bytes must not overwrite the captured result.
func TestClaudeRunIgnoresRecordsAfterTerminalResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stream := strings.Join([]string{
		`{"type":"result","subtype":"success","result":"first","usage":{"input_tokens":1,"output_tokens":2}}`,
		`trailing garbage`,
	}, "\n")
	binary, callsFile := writeClaudeStreamStub(t, stream)

	outcome, err := (executor.Claude{Binary: binary}).Run(context.Background(), executor.Request{
		Prompt:   "probe",
		Progress: make(chan executor.ProgressEvent, 8),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	const wantStdout = `{"type":"result","subtype":"success","result":"first","usage":{"input_tokens":1,"output_tokens":2}}` + "\n"
	if outcome.Stdout != wantStdout {
		t.Errorf("Stdout = %q, want the terminal result %q", outcome.Stdout, wantStdout)
	}
	calls, err := os.ReadFile(callsFile)
	if err != nil {
		t.Fatalf("ReadFile(calls) error = %v", err)
	}
	if got := string(calls); got != "stream-json\n" {
		t.Errorf("output formats invoked = %q, want no replay after a terminal result", got)
	}
}
