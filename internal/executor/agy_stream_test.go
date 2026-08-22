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

func writeAgyStreamStub(t *testing.T, stream string) (binary, callsFile string) {
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
	cat <<'AGY_STREAM_EOF'
%s
AGY_STREAM_EOF
else
	printf '%%s\n' '{"status":"SUCCESS","response":"blocking result"}'
fi
`, callsFile, stream)
	return writeStub(t, script), callsFile
}

func progressMessages(progress <-chan executor.ProgressEvent) []string {
	var messages []string
	for {
		select {
		case event := <-progress:
			messages = append(messages, event.Message)
		default:
			return messages
		}
	}
}

func TestAgyRunStreamsNormalizedProgressAndPreservesFinalResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stream := strings.Join([]string{
		`{"event":"init","conversation_id":"conversation-1","init":{"cwd":"/tmp/probe","tools":["run_command"],"permission_mode":"always-proceed"}}`,
		`{"event":"telemetry","telemetry":{"future":"record"}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"agent_response","text_delta":"AGY_STREAM_TEXT_READY\n","usage":{"input_tokens":22322,"output_tokens":1137,"thinking_tokens":1062,"cache_read_tokens":0,"total_tokens":23459}}}`,
		`{"event":"step_update","step_update":{"step_index":3,"state":"ACTIVE","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","parameters":{"CommandLine":"pwd"}}}}`,
		`{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","parameters":{"CommandLine":"pwd"},"output":"/tmp/probe\r\n"}}}`,
		`{"event":"step_update","step_update":{"step_index":5,"state":"ACTIVE","step_type":"tool","tool_name":"write_to_file","tool_info":{"name":"write_to_file","parameters":{"TargetFile":"/tmp/probe.txt"}}}}`,
		`{"event":"step_update","step_update":{"step_index":5,"state":"DONE","step_type":"tool","tool_name":"write_to_file","tool_info":{"name":"write_to_file","parameters":{"TargetFile":"/tmp/probe.txt"}}}}`,
		`{"event":"step_update","step_update":{"step_index":9,"state":"ACTIVE","step_type":"tool","tool_name":"replace_file_content","tool_info":{"name":"replace_file_content","parameters":{"TargetFile":"/tmp/probe.txt"}}}}`,
		`{"event":"step_update","step_update":{"step_index":9,"state":"DONE","step_type":"tool","tool_name":"replace_file_content","tool_info":{"name":"replace_file_content","parameters":{"TargetFile":"/tmp/probe.txt"}}}}`,
		`{"event":"result","result":{"conversation_id":"conversation-1","status":"SUCCESS","response":"AGY_STREAM_DONE\n","duration_seconds":20.2,"num_turns":1,"usage":{"input_tokens":64264,"output_tokens":2768,"thinking_tokens":2115,"cache_read_tokens":81443,"total_tokens":67032}}}`,
	}, "\n")
	binary, callsFile := writeAgyStreamStub(t, stream)
	progress := make(chan executor.ProgressEvent, 16)

	outcome, err := (executor.Agy{Binary: binary}).Run(context.Background(), executor.Request{
		Prompt:   "probe",
		Progress: progress,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	const wantStdout = `{"conversation_id":"conversation-1","status":"SUCCESS","response":"AGY_STREAM_DONE\n","duration_seconds":20.2,"num_turns":1,"usage":{"input_tokens":64264,"output_tokens":2768,"thinking_tokens":2115,"cache_read_tokens":81443,"total_tokens":67032}}` + "\n"
	if outcome.Stdout != wantStdout || outcome.ExitCode != 0 {
		t.Errorf("Outcome = %#v, want exit 0 and final result stdout %q", outcome, wantStdout)
	}

	wantMessages := []string{
		"AGY_STREAM_TEXT_READY\n",
		"Usage: 22322 input, 1137 output, 1062 thinking, 0 cache read, 23459 total tokens",
		"Tool started: run_command",
		"Tool finished: run_command",
		"Edit started: write_to_file (/tmp/probe.txt)",
		"Edit finished: write_to_file (/tmp/probe.txt)",
		"Edit started: replace_file_content (/tmp/probe.txt)",
		"Edit finished: replace_file_content (/tmp/probe.txt)",
		"Usage: 64264 input, 2768 output, 2115 thinking, 81443 cache read, 67032 total tokens",
	}
	if got := progressMessages(progress); !reflect.DeepEqual(got, wantMessages) {
		t.Errorf("progress messages = %#v, want %#v", got, wantMessages)
	}
	calls, err := os.ReadFile(callsFile)
	if err != nil {
		t.Fatalf("ReadFile(calls) error = %v", err)
	}
	if got := string(calls); got != "stream-json\n" {
		t.Errorf("output formats invoked = %q, want one stream-json invocation", got)
	}
}

func TestAgyRunFallsBackToBlockingJSONForInvalidStream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	tests := []struct {
		name   string
		stream string
	}{
		{name: "malformed JSON", stream: `{"event":"step_update"`},
		{name: "unsupported known record shape", stream: `{"event":"step_update","step_update":{"state":"DONE"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binary, callsFile := writeAgyStreamStub(t, tt.stream)
			progress := make(chan executor.ProgressEvent, 4)
			outcome, err := (executor.Agy{Binary: binary}).Run(context.Background(), executor.Request{
				Prompt:   "probe",
				Progress: progress,
			})
			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			const wantStdout = "{\"status\":\"SUCCESS\",\"response\":\"blocking result\"}\n"
			if outcome.Stdout != wantStdout || outcome.ExitCode != 0 {
				t.Errorf("Outcome = %#v, want blocking JSON outcome", outcome)
			}
			calls, err := os.ReadFile(callsFile)
			if err != nil {
				t.Fatalf("ReadFile(calls) error = %v", err)
			}
			if got := string(calls); got != "stream-json\njson\n" {
				t.Errorf("output formats invoked = %q, want stream then blocking JSON", got)
			}
			want := []string{"Agy stream unavailable; retrying blocking JSON"}
			if got := progressMessages(progress); !reflect.DeepEqual(got, want) {
				t.Errorf("progress messages = %#v, want %#v", got, want)
			}
		})
	}
}

func TestAgyRunDoesNotReplayCompletedTurnAfterTrailingMalformedData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stream := "{\"event\":\"result\",\"result\":{\"status\":\"SUCCESS\",\"response\":\"stream result\"}}\nnot-json"
	binary, callsFile := writeAgyStreamStub(t, stream)
	outcome, err := (executor.Agy{Binary: binary}).Run(context.Background(), executor.Request{
		Prompt:   "probe",
		Progress: make(chan executor.ProgressEvent, 4),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.Stdout != "{\"status\":\"SUCCESS\",\"response\":\"stream result\"}\n" {
		t.Errorf("Stdout = %q, want completed stream result", outcome.Stdout)
	}
	calls, err := os.ReadFile(callsFile)
	if err != nil {
		t.Fatalf("ReadFile(calls) error = %v", err)
	}
	if got := string(calls); got != "stream-json\n" {
		t.Errorf("output formats invoked = %q, want no replay after terminal result", got)
	}
}
