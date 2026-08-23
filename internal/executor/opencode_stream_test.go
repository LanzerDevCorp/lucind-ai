package executor

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// TestDecodeOpencodeRecord pins the record shapes verified against the
// installed CLI (1.18.21) by reading its bundle: `opencode run --format
// json` writes one line per Z(type, extra) call as
// `{"type":<type>,"timestamp":<ms>,"sessionID":<id>,...extra}`, where extra
// is `{part: J}` for "step_start"/"step_finish"/"text"/"tool_use" and
// `{error: J.error}` for the unrelated top-level "error" event (out of
// scope here; it carries no "part" and falls through the default case).
//
// The "tool_use" shape is the one surprise relative to cursor-agent: the
// CLI's own print loop only calls Z("tool_use", {part:J}) when
// J.state.status is "completed" or "error" -- never "pending" or
// "running" -- so no separate "started" record ever reaches stdout. The
// tool part itself still carries both timestamps (state.time.start and
// state.time.end), so decodeOpencodeRecord expands one such line into two
// events: a started event backdated to the real start time, and a
// finished/failed event at the real end time. Losing the start event (or
// reporting it "now", at record-arrival time) would erase exactly the
// silence this decoder exists to measure.
func TestDecodeOpencodeRecord(t *testing.T) {
	t.Parallel()

	const timestamp = int64(1787412178422)
	wantAt := time.UnixMilli(timestamp)
	tests := []struct {
		name       string
		record     string
		wantEvents []ProgressEvent
		wantErr    bool
	}{
		{
			name:       "step start",
			record:     `{"type":"step_start","timestamp":1787412178422,"part":{"type":"step-start"}}`,
			wantEvents: []ProgressEvent{{Message: "Step started", At: wantAt}},
		},
		{
			name:       "text",
			record:     `{"type":"text","timestamp":1787412178422,"part":{"type":"text","text":"PROBE_OK"}}`,
			wantEvents: []ProgressEvent{{Message: "PROBE_OK", At: wantAt}},
		},
		{
			name:       "step finish without usage",
			record:     `{"type":"step_finish","timestamp":1787412178422,"part":{"type":"step-finish","reason":"stop"}}`,
			wantEvents: []ProgressEvent{{Message: "Step finished: stop", At: wantAt}},
		},
		{
			name:   "step finish reports token usage",
			record: `{"type":"step_finish","timestamp":1787412178422,"part":{"type":"step-finish","reason":"stop","cost":0.0123,"tokens":{"input":120,"output":320,"reasoning":80,"cache":{"read":10,"write":5}}}}`,
			wantEvents: []ProgressEvent{
				{Message: "Step finished: stop", At: wantAt},
				{Message: "Usage: 120 input, 320 output, 80 reasoning, 10 cache read, 5 cache write tokens, $0.0123", At: wantAt},
			},
		},
		{
			name:   "tool use completed is reported as Tool kind",
			record: `{"type":"tool_use","timestamp":1787412178422,"sessionID":"ses_1","part":{"id":"prt_1","tool":"bash","callID":"call_1","state":{"status":"completed","input":{"command":"go test ./..."},"time":{"start":1787500729840,"end":1787500731000}}}}`,
			wantEvents: []ProgressEvent{
				{Message: "Tool started: bash", At: time.UnixMilli(1787500729840)},
				{Message: "Tool finished: bash", At: time.UnixMilli(1787500731000)},
			},
		},
		{
			name:   "tool use completed edit names the file under Edit kind",
			record: `{"type":"tool_use","timestamp":1787412178422,"sessionID":"ses_1","part":{"id":"prt_2","tool":"edit","callID":"call_2","state":{"status":"completed","input":{"filePath":"/tmp/probe.txt"},"time":{"start":1787500729840,"end":1787500729920}}}}`,
			wantEvents: []ProgressEvent{
				{Message: "Edit started: edit (/tmp/probe.txt)", At: time.UnixMilli(1787500729840)},
				{Message: "Edit finished: edit (/tmp/probe.txt)", At: time.UnixMilli(1787500729920)},
			},
		},
		{
			name:   "tool use error reports the failure reason",
			record: `{"type":"tool_use","timestamp":1787412178422,"sessionID":"ses_1","part":{"id":"prt_3","tool":"bash","callID":"call_3","state":{"status":"error","input":{"command":"exit 1"},"error":"Tool execution failed: exit status 1","time":{"start":1787500732000,"end":1787500732500}}}}`,
			wantEvents: []ProgressEvent{
				{Message: "Tool started: bash", At: time.UnixMilli(1787500732000)},
				{Message: "Tool failed: bash: Tool execution failed: exit status 1", At: time.UnixMilli(1787500732500)},
			},
		},
		{
			name:       "tool use missing a tool name is skipped",
			record:     `{"type":"tool_use","timestamp":1787412178422,"sessionID":"ses_1","part":{"id":"prt_4","callID":"call_4","state":{"status":"completed","time":{"start":1787500732000,"end":1787500732500}}}}`,
			wantEvents: nil,
		},
		{
			name:       "unknown record",
			record:     `{"type":"future_record","timestamp":1787412178422,"part":{"new_field":true}}`,
			wantEvents: nil,
		},
		{
			name:    "malformed record",
			record:  `{"type":"text","part":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := decodeOpencodeRecord([]byte(tt.record))
			if (err != nil) != tt.wantErr {
				t.Fatalf("decodeOpencodeRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(events, tt.wantEvents) {
				t.Errorf("decodeOpencodeRecord() events = %#v, want %#v", events, tt.wantEvents)
			}
		})
	}
}

func TestOpencodeRunStreamsProgressAndPreservesRawOutcome(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = "{\"type\":\"step_start\",\"timestamp\":1787412178297,\"part\":{\"type\":\"step-start\"}}\n" +
		"{\"type\":\"future_record\",\"timestamp\":1787412178300}\n" +
		"{\"type\":\"text\",\"timestamp\":1787412178422,\"part\":{\"type\":\"text\",\"text\":\"PROBE_OK\"}}\n"
	stub := writeOpencodeStreamStub(t, output)
	progress := make(chan ProgressEvent, 4)

	outcome, err := (Opencode{Binary: stub}).Run(context.Background(), Request{
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
	if outcome.Stdout != output {
		t.Errorf("Stdout = %q, want the complete raw JSON stream %q", outcome.Stdout, output)
	}

	wantMessages := []string{"Step started", "PROBE_OK"}
	for _, want := range wantMessages {
		select {
		case got := <-progress:
			if got.Message != want {
				t.Errorf("progress Message = %q, want %q", got.Message, want)
			}
		default:
			t.Fatalf("missing progress event %q", want)
		}
	}
	select {
	case got := <-progress:
		t.Fatalf("unexpected progress event after unknown record: %+v", got)
	default:
	}
}

// TestOpencodeRunMalformedLineIsNotedOnceAndDecodingContinues pins a
// deliberate policy change from the decoder's original behavior (which
// disabled decoding entirely after the first bad line). This timeline is
// now load-bearing for profiling tool latency versus reasoning silence
// (see opencodeStreamDegradedMessage's doc comment), so one unreadable
// line must not blind the rest of a lane's timeline -- matching
// claude_stream.go's claudeStreamDecoder.note(), which made the identical
// choice for the identical reason. cursor-agent's decoder never disables
// either: cursorStreamCapture.decode() sets a malformed flag but keeps
// decoding every subsequent line regardless.
func TestOpencodeRunMalformedLineIsNotedOnceAndDecodingContinues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = "{\"type\":\"text\",\"timestamp\":1787412178422,\"part\":{\"type\":\"text\",\"text\":\"before\"}}\n" +
		"not-json\n" +
		"not-json-either\n" +
		"{\"type\":\"text\",\"timestamp\":1787412178423,\"part\":{\"type\":\"text\",\"text\":\"after\"}}\n"
	stub := writeOpencodeStreamStub(t, output)
	progress := make(chan ProgressEvent, 8)

	outcome, err := (Opencode{Binary: stub}).Run(context.Background(), Request{
		Prompt:       "probe",
		WorktreePath: t.TempDir(),
		Progress:     progress,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil: telemetry decode failure must not fail the dispatch", err)
	}
	if outcome.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", outcome.ExitCode)
	}
	if outcome.Stdout != output {
		t.Errorf("Stdout = %q, want the complete raw output preserved regardless of decoder state %q", outcome.Stdout, output)
	}

	select {
	case got := <-progress:
		if got.Message != "before" {
			t.Errorf("progress Message = %q, want %q", got.Message, "before")
		}
	default:
		t.Fatal("missing progress event emitted before the malformed input")
	}
	select {
	case got := <-progress:
		if got.Message != opencodeStreamDegradedMessage {
			t.Errorf("degraded-note Message = %q, want %q", got.Message, opencodeStreamDegradedMessage)
		}
		if got.At.IsZero() {
			t.Error("degraded-note At is zero, want the decode-failure time")
		}
	default:
		t.Fatal("missing degraded note after the first malformed line")
	}
	select {
	case got := <-progress:
		if got.Message != "after" {
			t.Errorf("progress Message = %q, want %q -- decoding must resume after the note, and the second malformed line must not note again", got.Message, "after")
		}
	default:
		t.Fatal("missing progress event decoded after the malformed lines -- decoding must not stay disabled")
	}
	select {
	case got := <-progress:
		t.Fatalf("unexpected extra progress event: %+v -- the degraded note must fire at most once per dispatch", got)
	default:
	}
}

// TestOpencodeRunEmitsToolLifecycleEvents mirrors
// TestCursorAgentRunEmitsToolLifecycleEvents: opencode is the second
// executor real fan-out synthesis lanes could run on once the synthesis
// prompt is tuned, and it needs the same tool-latency-versus-silence
// instrumentation cursor-agent just got in 08e3442. Verified against the
// installed CLI (1.18.21): a "tool_use" record only ever arrives once a
// call has reached "completed" or "error", carrying both state.time.start
// and state.time.end -- so one record yields both the started and the
// finished/failed line, at their real respective timestamps.
func TestOpencodeRunEmitsToolLifecycleEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = `{"type":"tool_use","timestamp":1787500731100,"sessionID":"ses_1","part":{"id":"prt_1","tool":"bash","callID":"call_1","state":{"status":"completed","input":{"command":"go test ./..."},"time":{"start":1787500729840,"end":1787500731000}}}}` + "\n" +
		`{"type":"tool_use","timestamp":1787500733100,"sessionID":"ses_1","part":{"id":"prt_2","tool":"grep","callID":"call_2","state":{"status":"completed","input":{"pattern":"RegisterRun"},"time":{"start":1787500732000,"end":1787500733000}}}}` + "\n"
	stub := writeOpencodeStreamStub(t, output)
	progress := make(chan ProgressEvent, 8)

	outcome, err := (Opencode{Binary: stub}).Run(context.Background(), Request{
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

	want := []string{
		"Tool started: bash",
		"Tool finished: bash",
		"Tool started: grep",
		"Tool finished: grep",
	}
	var got []string
	for len(progress) > 0 {
		got = append(got, (<-progress).Message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

// TestOpencodeRunNamesTheEditedFile mirrors TestCursorAgentRunNamesTheEditedFile:
// a file-mutating call says which file, under the "Edit" kind. "edit" is one
// of the three tool names opencode's own permission-config normalizer folds
// into a single "edit" bucket (verified in the installed CLI, 1.18.21:
// `if (name === "write" || name === "edit" || name === "patch") ...`).
func TestOpencodeRunNamesTheEditedFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = `{"type":"tool_use","timestamp":1787500729950,"sessionID":"ses_1","part":{"id":"prt_1","tool":"edit","callID":"call_1","state":{"status":"completed","input":{"filePath":"/tmp/probe.txt"},"time":{"start":1787500729840,"end":1787500729920}}}}` + "\n"
	stub := writeOpencodeStreamStub(t, output)
	progress := make(chan ProgressEvent, 4)

	outcome, err := (Opencode{Binary: stub}).Run(context.Background(), Request{
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

	want := []string{
		"Edit started: edit (/tmp/probe.txt)",
		"Edit finished: edit (/tmp/probe.txt)",
	}
	var got []string
	for len(progress) > 0 {
		got = append(got, (<-progress).Message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

// TestOpencodeRunReportsFailedToolCall mirrors
// TestCursorAgentRunReportsFailedToolCall: a call that reached "error"
// must read as failed, never as finished, or a lane that thrashed against
// a broken call would look like a lane that worked.
func TestOpencodeRunReportsFailedToolCall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = `{"type":"tool_use","timestamp":1787500732600,"sessionID":"ses_1","part":{"id":"prt_1","tool":"edit","callID":"call_1","state":{"status":"error","input":{"filePath":"/tmp/probe.txt"},"error":"old_string and new_string are exactly the same","time":{"start":1787500732000,"end":1787500732500}}}}` + "\n"
	stub := writeOpencodeStreamStub(t, output)
	progress := make(chan ProgressEvent, 4)

	outcome, err := (Opencode{Binary: stub}).Run(context.Background(), Request{
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

	want := []string{
		"Edit started: edit (/tmp/probe.txt)",
		"Edit failed: edit (/tmp/probe.txt): old_string and new_string are exactly the same",
	}
	var got []string
	for len(progress) > 0 {
		got = append(got, (<-progress).Message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

// TestOpencodeRunEmitsUsageLineOnStepFinish exercises the third vocabulary
// item shared across all four executor decoders: a usage line where the
// executor reports token usage. Verified against the installed CLI
// (1.18.21): a "step-finish" part always carries `tokens` (input, output,
// reasoning, cache.read, cache.write) and `cost`.
func TestOpencodeRunEmitsUsageLineOnStepFinish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = `{"type":"step_finish","timestamp":1787500734000,"sessionID":"ses_1","part":{"type":"step-finish","reason":"stop","cost":0.0123,"tokens":{"input":120,"output":320,"reasoning":80,"cache":{"read":10,"write":5}}}}` + "\n"
	stub := writeOpencodeStreamStub(t, output)
	progress := make(chan ProgressEvent, 4)

	outcome, err := (Opencode{Binary: stub}).Run(context.Background(), Request{
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

	want := []string{
		"Step finished: stop",
		"Usage: 120 input, 320 output, 80 reasoning, 10 cache read, 5 cache write tokens, $0.0123",
	}
	var got []string
	for len(progress) > 0 {
		got = append(got, (<-progress).Message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}
}

func TestOpencodeRunWithoutProgressPreservesBlockingJSONPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = "{\"type\":\"text\",\"timestamp\":1787412178422,\"part\":{\"type\":\"text\",\"text\":\"PROBE_OK\"}}\n"
	stub := writeOpencodeStreamStub(t, output)

	outcome, err := (Opencode{Binary: stub}).Run(context.Background(), Request{
		Prompt:       "probe",
		WorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.Stdout != output {
		t.Errorf("Stdout = %q, want unchanged blocking JSON output %q", outcome.Stdout, output)
	}
}

func writeOpencodeStreamStub(t *testing.T, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode-stream-stub.sh")
	script := "#!/bin/sh\nprintf '%s' '" + output + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(stub) error = %v", err)
	}
	return path
}
