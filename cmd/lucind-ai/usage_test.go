package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/usagelog"
)

func createUsageFixture(t *testing.T, dir string) string {
	t.Helper()
	fixturePath := filepath.Join(dir, "usage.jsonl")

	baseTime := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	records := []usagelog.Record{
		{
			TS:          baseTime,
			RunID:       "run-1",
			LaneID:      "lane-1",
			Attempt:     1,
			Executor:    "agy",
			Provider:    "agy",
			Model:       "gemini-3.8-flash-high",
			LaneRole:    "apply",
			TotalTokens: 60000,
			CostUSD:     0.10,
			TokensKnown: true,
		},
		{
			TS:          baseTime.Add(time.Hour),
			RunID:       "run-1",
			LaneID:      "lane-2",
			Attempt:     1,
			Executor:    "cursor-agent",
			Provider:    "cursor",
			Model:       "claude-3-7-sonnet",
			LaneRole:    "judge",
			TotalTokens: 15000,
			CostUSD:     0.05,
			TokensKnown: true,
		},
		{
			TS:          baseTime.Add(2 * time.Hour),
			RunID:       "run-1",
			LaneID:      "lane-3",
			Attempt:     1,
			Executor:    "claude",
			Provider:    "claude",
			Model:       "claude-3-7-sonnet",
			LaneRole:    "orchestrator-lane",
			TotalTokens: 25000,
			CostUSD:     0.08,
			TokensKnown: true,
		},
	}

	for _, r := range records {
		if err := usagelog.Append(fixturePath, r); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}
	return fixturePath
}

func init() {
	// Dispatch tests in this package execute real run.Execute flows with fake
	// executors; keep their usage records out of the user's real state directory.
	if os.Getenv("LUCIND_USAGE_LOG") == "" {
		_ = os.Setenv("LUCIND_USAGE_LOG", "off")
	}
}

func TestCLIUsageReportTextAndJSON(t *testing.T) {
	fixturePath := createUsageFixture(t, t.TempDir())
	ctx := context.Background()

	t.Run("text report on fixture file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{"usage", "report", "--file", fixturePath}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "Lucind-ai Usage Report") {
			t.Errorf("stdout missing report header:\n%s", out)
		}
		if !strings.Contains(out, "agy") || !strings.Contains(out, "cursor") || !strings.Contains(out, "claude") {
			t.Errorf("stdout missing providers:\n%s", out)
		}
		if !strings.Contains(out, "Target Split") {
			t.Errorf("stdout missing Target Split:\n%s", out)
		}
		if !strings.Contains(strings.ToLower(out), "only lane calls made through lucind-ai are logged") {
			t.Errorf("stdout missing footer note:\n%s", out)
		}
	})

	t.Run("json report on fixture file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{"usage", "report", "--file", fixturePath, "--json"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
		}
		var rep usagelog.Report
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			t.Fatalf("failed to decode JSON output: %v\nOutput: %s", err, stdout.String())
		}
		if rep.GrandTotal.TotalTokens != 100000 {
			t.Errorf("GrandTotal.TotalTokens = %d, want 100000", rep.GrandTotal.TotalTokens)
		}
		if !rep.TargetComparison.HasTokenData {
			t.Error("expected HasTokenData = true")
		}
	})
}

func TestCLIUsageReportMissingFile(t *testing.T) {
	ctx := context.Background()
	missingPath := filepath.Join(t.TempDir(), "nonexistent", "usage.jsonl")

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"usage", "report", "--file", missingPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for missing file; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no usage recorded") {
		t.Errorf("stdout = %q, want 'no usage recorded'", stdout.String())
	}
}

func TestCLIUsageReportBadSince(t *testing.T) {
	fixturePath := createUsageFixture(t, t.TempDir())
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"usage", "report", "--file", fixturePath, "--since", "invalid-duration"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for bad --since", code)
	}
	if !strings.Contains(stderr.String(), "invalid --since") {
		t.Errorf("stderr = %q, want 'invalid --since'", stderr.String())
	}
}

func TestCLIUsageReportSinceDuration(t *testing.T) {
	fixturePath := createUsageFixture(t, t.TempDir())
	ctx := context.Background()

	// --since 7d
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"usage", "report", "--file", fixturePath, "--since", "7d"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for --since 7d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Lucind-ai Usage Report") {
		t.Errorf("stdout missing report for --since 7d:\n%s", stdout.String())
	}
}

func TestCLIUsageReportSinceDate(t *testing.T) {
	fixturePath := createUsageFixture(t, t.TempDir())
	ctx := context.Background()

	// --since 2026-10-01
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"usage", "report", "--file", fixturePath, "--since", "2026-10-01"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for --since 2026-10-01; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Lucind-ai Usage Report") {
		t.Errorf("stdout missing report for --since 2026-10-01:\n%s", stdout.String())
	}
}
