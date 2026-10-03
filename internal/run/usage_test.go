package run_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/usagelog"
)

func init() {
	// Guard tests in internal/run against touching the real user state directory.
	if os.Getenv("XDG_STATE_HOME") == "" {
		if tmp, err := os.MkdirTemp("", "lucind-run-test-state-*"); err == nil {
			_ = os.Setenv("XDG_STATE_HOME", tmp)
		}
	}
}

func TestExecuteRecordsUsagePerAttempt(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	exec := &scriptedExecutor{defaultModel: "gemini-3.7-flash-high"}
	exec.runFunc = func(ctx context.Context, req executor.Request, callCount int) (executor.Outcome, error) {
		if err := os.WriteFile(filepath.Join(wtDir, fmt.Sprintf("work_%d.txt", callCount)), []byte("work\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeEnvelope(t, wtDir, "done")
		// Attempt 1: output JSON with usage
		if callCount == 1 {
			stdout := `{"status":"SUCCESS","usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}}`
			return executor.Outcome{ExitCode: 0, Stdout: stdout}, nil
		}
		// Attempt 2: output JSON with different tokens
		stdout := `{"status":"SUCCESS","usage":{"input_tokens":200,"output_tokens":80,"total_tokens":280}}`
		return executor.Outcome{ExitCode: 0, Stdout: stdout}, nil
	}

	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return os.DirFS(wtDir)
	}, exec, baseSHA)

	verifCalls := 0
	deps.RunAttested = func(ctx context.Context, dir, cmd string) (int, string, error) {
		verifCalls++
		if verifCalls == 1 {
			return 1, "FAIL", nil // fail attempt 1 to trigger attempt 2
		}
		return 0, "PASS", nil
	}
	deps.HasValidAttestation = func(ctx context.Context, repoRoot, cmd, tree string) (bool, error) {
		return true, nil
	}

	var mu sync.Mutex
	var recorded []usagelog.Record
	deps.RecordUsage = func(rec usagelog.Record) {
		mu.Lock()
		defer mu.Unlock()
		recorded = append(recorded, rec)
	}

	p := testPacket()
	p.Model = "gemini-3.7-flash-high"
	p.MaxIterations = 2
	p.Verification = []string{"go test ./..."}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Done {
		t.Fatalf("report.Status = %v, want done; diagnosis: %s", report.Status, report.Diagnosis)
	}
	if report.Attempts != 2 {
		t.Fatalf("report.Attempts = %d, want 2", report.Attempts)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(recorded) != 2 {
		t.Fatalf("recorded %d usage records; want 2", len(recorded))
	}

	// Verify attempt 1 record
	r1 := recorded[0]
	if r1.Attempt != 1 {
		t.Errorf("r1.Attempt = %d; want 1", r1.Attempt)
	}
	if r1.LaneID != p.ID {
		t.Errorf("r1.LaneID = %q; want %q", r1.LaneID, p.ID)
	}
	if r1.Executor != p.Executor {
		t.Errorf("r1.Executor = %q; want %q", r1.Executor, p.Executor)
	}
	if r1.Provider != "agy" {
		t.Errorf("r1.Provider = %q; want agy", r1.Provider)
	}
	if r1.Model != "gemini-3.7-flash-high" {
		t.Errorf("r1.Model = %q; want gemini-3.7-flash-high", r1.Model)
	}
	if r1.LaneRole != p.LaneRole {
		t.Errorf("r1.LaneRole = %q; want %q", r1.LaneRole, p.LaneRole)
	}
	if !r1.TokensKnown || r1.TotalTokens != 150 {
		t.Errorf("r1 tokens: known=%v, total=%d; want true, 150", r1.TokensKnown, r1.TotalTokens)
	}
	if r1.Status != "" {
		t.Errorf("r1.Status at call time must be empty, got %q", r1.Status)
	}

	// Verify attempt 2 record
	r2 := recorded[1]
	if r2.Attempt != 2 {
		t.Errorf("r2.Attempt = %d; want 2", r2.Attempt)
	}
	if !r2.TokensKnown || r2.TotalTokens != 280 {
		t.Errorf("r2 tokens: known=%v, total=%d; want true, 280", r2.TokensKnown, r2.TotalTokens)
	}
	if r2.Status != "" {
		t.Errorf("r2.Status at call time must be empty, got %q", r2.Status)
	}
}

func TestExecuteUsageExtractionFallbackToProgress(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	exec := progressExecutor{run: func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		req.Progress <- executor.ProgressEvent{
			Message:     "step 1",
			TotalTokens: 1200,
			CostUSD:     0.03,
		}
		req.Progress <- executor.ProgressEvent{
			Message:     "step 2",
			TotalTokens: 3400,
			CostUSD:     0.08,
		}
		writeEnvelope(t, wtDir, "done")
		// Stdout does NOT contain parseable usage JSON
		return executor.Outcome{ExitCode: 0, Stdout: "raw unparseable output"}, nil
	}}

	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return os.DirFS(wtDir)
	}, exec, baseSHA)

	var recorded []usagelog.Record
	deps.RecordUsage = func(rec usagelog.Record) {
		recorded = append(recorded, rec)
	}

	p := testPacket()
	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Done {
		t.Fatalf("report.Status = %v, want done", report.Status)
	}

	if len(recorded) != 1 {
		t.Fatalf("recorded %d records; want 1", len(recorded))
	}
	r := recorded[0]
	if !r.TokensKnown {
		t.Errorf("TokensKnown = false, want true from progress fallback")
	}
	if r.TotalTokens != 3400 {
		t.Errorf("TotalTokens = %d, want 3400 (largest seen)", r.TotalTokens)
	}
	if r.CostUSD != 0.08 {
		t.Errorf("CostUSD = %v, want 0.08", r.CostUSD)
	}
}

func TestExecuteUsageTokensUnknownWhenNeitherStdoutNorProgress(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	exec := progressExecutor{run: func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		req.Progress <- executor.ProgressEvent{
			Message:     "progress without tokens",
			TotalTokens: 0,
			CostUSD:     0,
		}
		writeEnvelope(t, wtDir, "done")
		return executor.Outcome{ExitCode: 0, Stdout: "no usage json here"}, nil
	}}

	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return os.DirFS(wtDir)
	}, exec, baseSHA)

	var recorded []usagelog.Record
	deps.RecordUsage = func(rec usagelog.Record) {
		recorded = append(recorded, rec)
	}

	p := testPacket()
	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Done {
		t.Fatalf("report.Status = %v, want done", report.Status)
	}

	if len(recorded) != 1 {
		t.Fatalf("recorded %d records; want 1", len(recorded))
	}
	r := recorded[0]
	if r.TokensKnown {
		t.Errorf("TokensKnown = true, want false")
	}
	if r.TotalTokens != 0 || r.CostUSD != 0 {
		t.Errorf("TotalTokens=%d CostUSD=%v, want zeros", r.TotalTokens, r.CostUSD)
	}
}

func TestFailingDefaultRecordUsageNeverFailsLane(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	exec := &scriptedExecutor{}
	exec.runFunc = func(ctx context.Context, req executor.Request, callCount int) (executor.Outcome, error) {
		writeEnvelope(t, wtDir, "done")
		return executor.Outcome{ExitCode: 0}, nil
	}

	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return os.DirFS(wtDir)
	}, exec, baseSHA)
	// Deps.RecordUsage is nil => invokes default path.
	// Point XDG_STATE_HOME to a read-only directory to force usagelog.Append failure.
	roDir := t.TempDir()
	if err := os.Chmod(roDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(roDir, 0o700) }()
	t.Setenv("XDG_STATE_HOME", roDir)

	p := testPacket()
	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute failed when RecordUsage default failed: %v", err)
	}
	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want done despite usage log append error", report.Status)
	}
}

func TestUsageLogDoesNotTouchHomeDuringTests(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve user home dir: %v", err)
	}
	statePath := filepath.Join(home, ".local", "state", "lucind-ai", "usage.jsonl")

	var modBefore time.Time
	if fi, err := os.Stat(statePath); err == nil {
		modBefore = fi.ModTime()
	}

	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)
	exec := &scriptedExecutor{}
	exec.runFunc = func(ctx context.Context, req executor.Request, callCount int) (executor.Outcome, error) {
		writeEnvelope(t, wtDir, "done")
		return executor.Outcome{ExitCode: 0}, nil
	}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return os.DirFS(wtDir)
	}, exec, baseSHA)

	tempState := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempState)

	p := testPacket()
	if _, err := run.Execute(context.Background(), deps, p); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Verify tempState received the write
	tempUsageFile := filepath.Join(tempState, "lucind-ai", "usage.jsonl")
	if _, err := os.Stat(tempUsageFile); err != nil {
		t.Errorf("expected usage file in temp state dir %q: %v", tempUsageFile, err)
	}

	// Verify real state path in home was NOT modified
	if fi, err := os.Stat(statePath); err == nil {
		if !fi.ModTime().Equal(modBefore) {
			t.Errorf("real home statePath %q was modified!", statePath)
		}
	}
}
