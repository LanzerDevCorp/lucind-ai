package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeOpencodeRecord(t *testing.T) {
	t.Parallel()

	const timestamp = int64(1787412178422)
	wantAt := time.UnixMilli(timestamp)
	tests := []struct {
		name        string
		record      string
		wantMessage string
		wantAt      time.Time
		wantEvent   bool
		wantErr     bool
	}{
		{
			name:        "step start",
			record:      `{"type":"step_start","timestamp":1787412178422,"part":{"type":"step-start"}}`,
			wantMessage: "Step started",
			wantAt:      wantAt,
			wantEvent:   true,
		},
		{
			name:        "text",
			record:      `{"type":"text","timestamp":1787412178422,"part":{"type":"text","text":"PROBE_OK"}}`,
			wantMessage: "PROBE_OK",
			wantAt:      wantAt,
			wantEvent:   true,
		},
		{
			name:        "step finish",
			record:      `{"type":"step_finish","timestamp":1787412178422,"part":{"type":"step-finish","reason":"stop"}}`,
			wantMessage: "Step finished: stop",
			wantAt:      wantAt,
			wantEvent:   true,
		},
		{
			name:      "unknown record",
			record:    `{"type":"future_record","timestamp":1787412178422,"part":{"new_field":true}}`,
			wantEvent: false,
		},
		{
			name:    "malformed record",
			record:  `{"type":"text","part":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok, err := decodeOpencodeRecord([]byte(tt.record))
			if (err != nil) != tt.wantErr {
				t.Fatalf("decodeOpencodeRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.wantEvent {
				t.Fatalf("decodeOpencodeRecord() event = %v, want %v", ok, tt.wantEvent)
			}
			if !tt.wantEvent {
				return
			}
			if event.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", event.Message, tt.wantMessage)
			}
			if !event.At.Equal(tt.wantAt) {
				t.Errorf("At = %v, want %v", event.At, tt.wantAt)
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

func TestOpencodeRunMalformedStreamDegradesToRawOutcome(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const output = "{\"type\":\"text\",\"timestamp\":1787412178422,\"part\":{\"type\":\"text\",\"text\":\"before\"}}\n" +
		"not-json\n" +
		"{\"type\":\"text\",\"timestamp\":1787412178423,\"part\":{\"type\":\"text\",\"text\":\"after\"}}\n"
	stub := writeOpencodeStreamStub(t, output)
	progress := make(chan ProgressEvent, 4)

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
		t.Errorf("Stdout = %q, want the complete raw output after decoder fallback %q", outcome.Stdout, output)
	}

	select {
	case got := <-progress:
		if got.Message != "before" {
			t.Errorf("progress Message = %q, want %q", got.Message, "before")
		}
	default:
		t.Fatal("missing progress event emitted before malformed input")
	}
	select {
	case got := <-progress:
		const want = "Opencode progress decoding disabled; continuing with raw JSON result"
		if got.Message != want {
			t.Errorf("fallback progress Message = %q, want %q", got.Message, want)
		}
		if got.At.IsZero() {
			t.Error("fallback progress At is zero, want the decode-failure time")
		}
	default:
		t.Fatal("missing fallback note after malformed input disabled decoding")
	}
	select {
	case got := <-progress:
		t.Fatalf("unexpected decoded progress after fallback: %+v", got)
	default:
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
