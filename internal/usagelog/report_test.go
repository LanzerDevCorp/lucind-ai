package usagelog_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/usagelog"
)

func TestBuildReportTargetShares(t *testing.T) {
	t.Run("exact 60/15/25 match with opencode excluded", func(t *testing.T) {
		baseTime := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
		records := []usagelog.Record{
			{
				TS:          baseTime,
				Executor:    "agy",
				Provider:    "agy",
				Model:       "gemini-3.8-flash-high",
				LaneRole:    "apply",
				TotalTokens: 60000,
				CostUSD:     0.10,
				TokensKnown: true,
			},
			{
				TS:          baseTime.Add(time.Minute),
				Executor:    "cursor-agent",
				Provider:    "cursor",
				Model:       "claude-3-7-sonnet",
				LaneRole:    "judge",
				TotalTokens: 15000,
				CostUSD:     0.05,
				TokensKnown: true,
			},
			{
				TS:          baseTime.Add(2 * time.Minute),
				Executor:    "claude",
				Provider:    "claude",
				Model:       "claude-3-7-sonnet",
				LaneRole:    "orchestrator-lane",
				TotalTokens: 25000,
				CostUSD:     0.08,
				TokensKnown: true,
			},
			{
				TS:          baseTime.Add(3 * time.Minute),
				Executor:    "opencode",
				Provider:    "opencode",
				Model:       "local-model",
				LaneRole:    "research",
				TotalTokens: 50000, // excluded from target share
				CostUSD:     0.00,
				TokensKnown: true,
			},
		}

		rep := usagelog.BuildReport(records, time.Time{}, 0)

		if rep.GrandTotal.Calls != 4 {
			t.Errorf("GrandTotal.Calls = %d; want 4", rep.GrandTotal.Calls)
		}
		if rep.GrandTotal.TotalTokens != 150000 {
			t.Errorf("GrandTotal.TotalTokens = %d; want 150000", rep.GrandTotal.TotalTokens)
		}

		tc := rep.TargetComparison
		if !tc.HasTokenData {
			t.Fatal("expected HasTokenData = true")
		}
		if tc.TargetTokens != 100000 {
			t.Errorf("TargetTokens = %d; want 100000", tc.TargetTokens)
		}

		findShare := func(provider string) usagelog.TargetShareComparison {
			for _, s := range tc.Providers {
				if s.Provider == provider {
					return s
				}
			}
			t.Fatalf("provider %q not found in target comparison", provider)
			return usagelog.TargetShareComparison{}
		}

		agy := findShare("agy")
		if agy.ActualShare != 0.60 {
			t.Errorf("agy actual share = %v; want 0.60", agy.ActualShare)
		}
		if agy.DeltaPoints != 0.0 {
			t.Errorf("agy delta points = %v; want 0.0", agy.DeltaPoints)
		}

		cursor := findShare("cursor")
		if cursor.ActualShare != 0.15 {
			t.Errorf("cursor actual share = %v; want 0.15", cursor.ActualShare)
		}
		if cursor.DeltaPoints != 0.0 {
			t.Errorf("cursor delta points = %v; want 0.0", cursor.DeltaPoints)
		}

		claude := findShare("claude")
		if claude.ActualShare != 0.25 {
			t.Errorf("claude actual share = %v; want 0.25", claude.ActualShare)
		}
		if claude.DeltaPoints != 0.0 {
			t.Errorf("claude delta points = %v; want 0.0", claude.DeltaPoints)
		}
	})

	t.Run("skewed shares compute correct deltas in percentage points", func(t *testing.T) {
		records := []usagelog.Record{
			{Provider: "agy", TotalTokens: 70000, TokensKnown: true},
			{Provider: "cursor", TotalTokens: 10000, TokensKnown: true},
			{Provider: "claude", TotalTokens: 20000, TokensKnown: true},
		}

		rep := usagelog.BuildReport(records, time.Time{}, 0)
		tc := rep.TargetComparison

		findShare := func(provider string) usagelog.TargetShareComparison {
			for _, s := range tc.Providers {
				if s.Provider == provider {
					return s
				}
			}
			t.Fatalf("provider %q not found", provider)
			return usagelog.TargetShareComparison{}
		}

		// agy: actual 70%, target 60% => delta +10.0 pp
		agy := findShare("agy")
		if diff := agy.DeltaPoints - 10.0; diff < -0.01 || diff > 0.01 {
			t.Errorf("agy delta points = %v; want +10.0", agy.DeltaPoints)
		}

		// cursor: actual 10%, target 15% => delta -5.0 pp
		cursor := findShare("cursor")
		if diff := cursor.DeltaPoints - (-5.0); diff < -0.01 || diff > 0.01 {
			t.Errorf("cursor delta points = %v; want -5.0", cursor.DeltaPoints)
		}

		// claude: actual 20%, target 25% => delta -5.0 pp
		claude := findShare("claude")
		if diff := claude.DeltaPoints - (-5.0); diff < -0.01 || diff > 0.01 {
			t.Errorf("claude delta points = %v; want -5.0", claude.DeltaPoints)
		}
	})

	t.Run("zero tokens reports no token data", func(t *testing.T) {
		records := []usagelog.Record{
			{Provider: "agy", TotalTokens: 0, TokensKnown: false},
			{Provider: "cursor", TotalTokens: 0, TokensKnown: false},
		}

		rep := usagelog.BuildReport(records, time.Time{}, 1)
		if rep.TargetComparison.HasTokenData {
			t.Error("expected HasTokenData = false for zero total tokens")
		}
		for _, p := range rep.TargetComparison.Providers {
			if p.ActualShare != 0.0 {
				t.Errorf("provider %s share = %v; want 0.0", p.Provider, p.ActualShare)
			}
		}

		text := rep.Text()
		if !strings.Contains(text, "no token data") {
			t.Errorf("expected Text() to contain 'no token data', got:\n%s", text)
		}
		if !strings.Contains(text, "Skipped lines: 1") {
			t.Errorf("expected Text() to contain 'Skipped lines: 1', got:\n%s", text)
		}
	})
}

func TestBuildReportSinceFiltering(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)

	records := []usagelog.Record{
		{TS: t0, Provider: "agy", TotalTokens: 100, TokensKnown: true},
		{TS: t1, Provider: "agy", TotalTokens: 200, TokensKnown: true},
		{TS: t2, Provider: "agy", TotalTokens: 300, TokensKnown: true},
	}

	// Filter since t1: t0 should be excluded, t1 and t2 included
	rep := usagelog.BuildReport(records, t1, 0)
	if rep.GrandTotal.Calls != 2 {
		t.Errorf("Calls = %d; want 2", rep.GrandTotal.Calls)
	}
	if rep.GrandTotal.TotalTokens != 500 {
		t.Errorf("TotalTokens = %d; want 500", rep.GrandTotal.TotalTokens)
	}
}

func TestReportTextAndJSON(t *testing.T) {
	baseTime := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	records := []usagelog.Record{
		{
			TS:          baseTime,
			Executor:    "agy",
			Provider:    "agy",
			Model:       "gemini-3.8-flash-high",
			LaneRole:    "apply",
			TotalTokens: 60000,
			CostUSD:     0.12,
			TokensKnown: true,
		},
		{
			TS:          baseTime.Add(time.Minute),
			Executor:    "cursor-agent",
			Provider:    "cursor",
			Model:       "claude-3-7-sonnet",
			LaneRole:    "judge",
			TotalTokens: 15000,
			CostUSD:     0.05,
			TokensKnown: true,
		},
		{
			TS:          baseTime.Add(2 * time.Minute),
			Executor:    "claude",
			Provider:    "claude",
			Model:       "claude-3-7-sonnet",
			LaneRole:    "orchestrator-lane",
			TotalTokens: 25000,
			CostUSD:     0.08,
			TokensKnown: true,
		},
	}

	rep := usagelog.BuildReport(records, time.Time{}, 0)

	text := rep.Text()
	// Must contain footer limitation notice
	wantFooter := "only lane calls made through lucind-ai are logged, so orchestrator Claude usage outside claude lanes is not visible"
	if !strings.Contains(strings.ToLower(text), strings.ToLower(wantFooter)) {
		t.Errorf("text output missing footer limitation notice:\n%s", text)
	}

	// Must contain target split table
	if !strings.Contains(text, "agy") || !strings.Contains(text, "cursor") || !strings.Contains(text, "claude") {
		t.Errorf("text output missing provider names:\n%s", text)
	}

	// Test JSON serialization
	jsonBytes, err := rep.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	var roundtrip usagelog.Report
	if err := json.Unmarshal(jsonBytes, &roundtrip); err != nil {
		t.Fatalf("unmarshal JSON() output: %v", err)
	}

	if roundtrip.GrandTotal.TotalTokens != rep.GrandTotal.TotalTokens {
		t.Errorf("roundtrip TotalTokens = %d; want %d", roundtrip.GrandTotal.TotalTokens, rep.GrandTotal.TotalTokens)
	}
	if len(roundtrip.Providers) != len(rep.Providers) {
		t.Errorf("roundtrip Providers count = %d; want %d", len(roundtrip.Providers), len(rep.Providers))
	}
}
