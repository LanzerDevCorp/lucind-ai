package usagelog_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/usagelog"
)

func TestProvider(t *testing.T) {
	tests := []struct {
		executor string
		want     string
	}{
		{executor: "agy", want: "agy"},
		{executor: "herdr-agy", want: "agy"},
		{executor: "cursor-agent", want: "cursor"},
		{executor: "claude", want: "claude"},
		{executor: "opencode", want: "opencode"},
		{executor: "custom-exec", want: "custom-exec"},
		{executor: "unknown", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.executor, func(t *testing.T) {
			got := usagelog.Provider(tt.executor)
			if got != tt.want {
				t.Fatalf("Provider(%q) = %q; want %q", tt.executor, got, tt.want)
			}
		})
	}
}

func TestExtractUsage(t *testing.T) {
	t.Run("agy payload", func(t *testing.T) {
		const payload = `{"status":"SUCCESS","response":"x","usage":{"input_tokens":331476,"output_tokens":26154,"thinking_tokens":19426,"cache_read_tokens":1377879,"total_tokens":357630}}`
		u, ok := usagelog.ExtractUsage(payload)
		if !ok {
			t.Fatal("ExtractUsage returned ok=false for agy payload")
		}
		if u.InputTokens != 331476 {
			t.Errorf("InputTokens = %d; want 331476", u.InputTokens)
		}
		if u.OutputTokens != 26154 {
			t.Errorf("OutputTokens = %d; want 26154", u.OutputTokens)
		}
		if u.ThinkingTokens != 19426 {
			t.Errorf("ThinkingTokens = %d; want 19426", u.ThinkingTokens)
		}
		if u.CacheReadTokens != 1377879 {
			t.Errorf("CacheReadTokens = %d; want 1377879", u.CacheReadTokens)
		}
		if u.TotalTokens != 357630 {
			t.Errorf("TotalTokens = %d; want 357630", u.TotalTokens)
		}
		if u.CostUSD != 0 {
			t.Errorf("CostUSD = %v; want 0", u.CostUSD)
		}
	})

	t.Run("claude payload with cost and cache creation/read", func(t *testing.T) {
		const payload = `{"type":"result","subtype":"success","is_error":false,"num_turns":2,"result":"CLAUDE_STREAM_DONE","session_id":"session-1","total_cost_usd":0.421,"usage":{"input_tokens":64264,"output_tokens":2768,"cache_creation_input_tokens":512,"cache_read_input_tokens":81443}}`
		u, ok := usagelog.ExtractUsage(payload)
		if !ok {
			t.Fatal("ExtractUsage returned ok=false for claude payload")
		}
		// cache_creation_input_tokens folded into input_tokens: 64264 + 512 = 64776
		if u.InputTokens != 64776 {
			t.Errorf("InputTokens = %d; want 64776", u.InputTokens)
		}
		if u.OutputTokens != 2768 {
			t.Errorf("OutputTokens = %d; want 2768", u.OutputTokens)
		}
		if u.CacheReadTokens != 81443 {
			t.Errorf("CacheReadTokens = %d; want 81443", u.CacheReadTokens)
		}
		// total_tokens absent: sum of parts = 64776 + 2768 + 81443 = 148987
		if u.TotalTokens != 148987 {
			t.Errorf("TotalTokens = %d; want 148987", u.TotalTokens)
		}
		if u.CostUSD != 0.421 {
			t.Errorf("CostUSD = %v; want 0.421", u.CostUSD)
		}
	})

	t.Run("cost_usd top level alternative", func(t *testing.T) {
		const payload = `{"cost_usd":1.25,"usage":{"input_tokens":10,"output_tokens":20}}`
		u, ok := usagelog.ExtractUsage(payload)
		if !ok {
			t.Fatal("ExtractUsage returned ok=false")
		}
		if u.CostUSD != 1.25 {
			t.Errorf("CostUSD = %v; want 1.25", u.CostUSD)
		}
		if u.TotalTokens != 30 {
			t.Errorf("TotalTokens = %d; want 30", u.TotalTokens)
		}
	})

	t.Run("garbage input", func(t *testing.T) {
		u, ok := usagelog.ExtractUsage("not json at all! {][}")
		if ok {
			t.Errorf("ExtractUsage returned ok=true for garbage; got %+v", u)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		u, ok := usagelog.ExtractUsage("")
		if ok {
			t.Errorf("ExtractUsage returned ok=true for empty string; got %+v", u)
		}
	})

	t.Run("JSON without usage", func(t *testing.T) {
		u, ok := usagelog.ExtractUsage(`{"status":"SUCCESS","response":"done"}`)
		if ok {
			t.Errorf("ExtractUsage returned ok=true for JSON without usage; got %+v", u)
		}
	})

	t.Run("JSON with empty usage object", func(t *testing.T) {
		u, ok := usagelog.ExtractUsage(`{"usage":{}}`)
		if ok {
			t.Errorf("ExtractUsage returned ok=true for empty usage object; got %+v", u)
		}
	})

	t.Run("NDJSON stream with final result object", func(t *testing.T) {
		const stream = `{"type":"init","model":"test"}
{"type":"step_update","step_index":1}
{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}}
`
		u, ok := usagelog.ExtractUsage(stream)
		if !ok {
			t.Fatal("ExtractUsage returned ok=false for NDJSON stream")
		}
		if u.TotalTokens != 150 {
			t.Errorf("TotalTokens = %d; want 150", u.TotalTokens)
		}
		if u.CostUSD != 0.05 {
			t.Errorf("CostUSD = %v; want 0.05", u.CostUSD)
		}
	})
}

func TestDefaultPath(t *testing.T) {
	t.Run("honors XDG_STATE_HOME", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("XDG_STATE_HOME", tmp)
		p, err := usagelog.DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath error: %v", err)
		}
		want := filepath.Join(tmp, "lucind-ai", "usage.jsonl")
		if p != want {
			t.Fatalf("DefaultPath() = %q; want %q", p, want)
		}
	})

	t.Run("fallback when XDG_STATE_HOME unset", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		p, err := usagelog.DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath error: %v", err)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("cannot resolve user home dir: %v", err)
		}
		want := filepath.Join(home, ".local", "state", "lucind-ai", "usage.jsonl")
		if p != want {
			t.Fatalf("DefaultPath() = %q; want %q", p, want)
		}
	})
}

func TestAppendAndPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "sub", "state", "usage.jsonl")

	rec := usagelog.Record{
		TS:          time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC),
		RunID:       "run-123",
		LaneID:      "lane-abc",
		Attempt:     1,
		Executor:    "herdr-agy",
		Provider:    "agy",
		Model:       "gemini-3.8-flash-high",
		LaneRole:    "apply",
		TotalTokens: 1000,
		CostUSD:     0.02,
		TokensKnown: true,
		DurationMS:  1500,
		ExitCode:    0,
	}

	if err := usagelog.Append(logPath, rec); err != nil {
		t.Fatalf("Append error: %v", err)
	}

	// Verify directory mode is 0700
	parentDir := filepath.Dir(logPath)
	dirInfo, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("stat parent dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0700 {
		t.Errorf("parent dir perm = %04o; want 0700", perm)
	}

	// Verify file mode is 0600
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0600 {
		t.Errorf("file perm = %04o; want 0600", perm)
	}

	// Append a second record
	rec2 := rec
	rec2.Attempt = 2
	rec2.TotalTokens = 2000
	if err := usagelog.Append(logPath, rec2); err != nil {
		t.Fatalf("Append 2 error: %v", err)
	}

	records, skipped, err := usagelog.ReadAll(logPath)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d; want 0", skipped)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d; want 2", len(records))
	}
	if records[0].RunID != "run-123" || records[0].Attempt != 1 {
		t.Errorf("record 0 mismatch: %+v", records[0])
	}
	if records[1].Attempt != 2 || records[1].TotalTokens != 2000 {
		t.Errorf("record 1 mismatch: %+v", records[1])
	}
}

func TestConcurrentAppend(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "concurrent", "usage.jsonl")

	const goroutines = 40
	const writesPerGoroutine = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(g int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				rec := usagelog.Record{
					TS:          time.Now().UTC(),
					RunID:       fmt.Sprintf("run-%d", g),
					LaneID:      fmt.Sprintf("lane-%d-%d", g, j),
					Attempt:     j + 1,
					Executor:    "agy",
					Provider:    "agy",
					Model:       "gemini-3.8-flash-high",
					TotalTokens: int64((g + 1) * 100),
					TokensKnown: true,
				}
				if err := usagelog.Append(logPath, rec); err != nil {
					t.Errorf("concurrent Append error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	records, skipped, err := usagelog.ReadAll(logPath)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("skipped = %d; want 0 (corrupted concurrent write detected)", skipped)
	}
	wantTotal := goroutines * writesPerGoroutine
	if len(records) != wantTotal {
		t.Fatalf("len(records) = %d; want %d", len(records), wantTotal)
	}
}

func TestReadAll(t *testing.T) {
	t.Run("missing file is empty not error", func(t *testing.T) {
		tmpDir := t.TempDir()
		records, skipped, err := usagelog.ReadAll(filepath.Join(tmpDir, "missing.jsonl"))
		if err != nil {
			t.Fatalf("expected nil err for missing file, got %v", err)
		}
		if skipped != 0 {
			t.Errorf("skipped = %d; want 0", skipped)
		}
		if len(records) != 0 {
			t.Errorf("records len = %d; want 0", len(records))
		}
	})

	t.Run("skips blank lines and malformed lines", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		content := `{"ts":"2026-10-03T10:00:00Z","run_id":"r1","lane_id":"l1","attempt":1,"executor":"agy","provider":"agy","total_tokens":100,"tokens_known":true}

not a valid json line
   
{"ts":"2026-10-03T10:01:00Z","run_id":"r2","lane_id":"l2","attempt":1,"executor":"claude","provider":"claude","total_tokens":200,"tokens_known":true}
{"corrupt json
{"ts":"2026-10-03T10:02:00Z","run_id":"r3","lane_id":"l3","attempt":1,"executor":"cursor-agent","provider":"cursor","total_tokens":300,"tokens_known":true}
`
		if err := os.WriteFile(logPath, []byte(content), 0600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		records, skipped, err := usagelog.ReadAll(logPath)
		if err != nil {
			t.Fatalf("ReadAll error: %v", err)
		}
		if skipped != 2 {
			t.Errorf("skipped = %d; want 2 malformed lines", skipped)
		}
		if len(records) != 3 {
			t.Fatalf("len(records) = %d; want 3", len(records))
		}
		if records[0].LaneID != "l1" || records[1].LaneID != "l2" || records[2].LaneID != "l3" {
			t.Errorf("unexpected records: %+v", records)
		}
	})
}

func TestAppendRouterEvent(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "events", "usage.jsonl")

	ev := usagelog.RouterEvent{
		TS:             time.Date(2026, 10, 3, 11, 0, 0, 0, time.UTC),
		Kind:           "router_disagreement",
		PrimaryRoute:   "inline",
		CandidateRoute: "worker",
		Confidence:     0.85,
		Signals: &usagelog.RouterSignals{
			AllowedPathCount: 2,
			NewFile:          false,
			RiskTierLevel:    1,
			ReadOnly:         false,
		},
	}

	if err := usagelog.AppendRouterEvent(logPath, ev); err != nil {
		t.Fatalf("AppendRouterEvent error: %v", err)
	}

	// Verify permissions
	dirInfo, err := os.Stat(filepath.Dir(logPath))
	if err != nil {
		t.Fatalf("stat parent dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0700 {
		t.Errorf("parent dir perm = %04o; want 0700", perm)
	}
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0600 {
		t.Errorf("file perm = %04o; want 0600", perm)
	}

	// Append an error event
	errEv := usagelog.RouterEvent{
		TS:        time.Date(2026, 10, 3, 11, 1, 0, 0, time.UTC),
		Kind:      "router_error",
		ErrorKind: "unauthorized",
	}
	if err := usagelog.AppendRouterEvent(logPath, errEv); err != nil {
		t.Fatalf("AppendRouterEvent error: %v", err)
	}

	records, skipped, err := usagelog.ReadAll(logPath)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d; want 0", skipped)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d; want 2", len(records))
	}
	if records[0].Kind != "router_disagreement" || records[0].PrimaryRoute != "inline" || records[0].CandidateRoute != "worker" {
		t.Errorf("unexpected record 0: %+v", records[0])
	}
	if records[0].Signals == nil || records[0].Signals.AllowedPathCount != 2 {
		t.Errorf("unexpected record 0 signals: %+v", records[0].Signals)
	}
	if records[1].Kind != "router_error" || records[1].ErrorKind != "unauthorized" {
		t.Errorf("unexpected record 1: %+v", records[1])
	}
}

func TestConcurrentAppendRouterEvent(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "concurrent_events", "usage.jsonl")

	const goroutines = 40
	const writesPerGoroutine = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(g int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				ev := usagelog.RouterEvent{
					TS:             time.Now().UTC(),
					Kind:           "router_disagreement",
					PrimaryRoute:   "inline",
					CandidateRoute: "worker",
					Confidence:     0.8,
					Signals: &usagelog.RouterSignals{
						AllowedPathCount: g + 1,
						NewFile:          j%2 == 0,
					},
				}
				if err := usagelog.AppendRouterEvent(logPath, ev); err != nil {
					t.Errorf("concurrent AppendRouterEvent error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	records, skipped, err := usagelog.ReadAll(logPath)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("skipped = %d; want 0 (corrupted concurrent write detected)", skipped)
	}
	wantTotal := goroutines * writesPerGoroutine
	if len(records) != wantTotal {
		t.Fatalf("len(records) = %d; want %d", len(records), wantTotal)
	}
}
