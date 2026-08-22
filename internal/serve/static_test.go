package serve_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/serve"
)

func readStaticAsset(t *testing.T, name string) string {
	t.Helper()
	data, err := fs.ReadFile(serve.StaticFS(), name)
	if err != nil {
		t.Fatalf("fs.ReadFile(%q): %v", name, err)
	}
	return string(data)
}

func assertContainsAll(t *testing.T, sourceName, source string, contracts ...string) {
	t.Helper()
	for _, contract := range contracts {
		if !strings.Contains(source, contract) {
			t.Errorf("%s does not contain required contract %q", sourceName, contract)
		}
	}
}

func TestEmbedFSHasNoApproveAllControl(t *testing.T) {
	staticFS := serve.StaticFS()

	files := []string{"index.html", "app.js"}
	forbiddenTerms := []string{
		"approve all",
		"approve-all",
		"approve_all",
		"approveall",
		"select all",
		"select-all",
		"select_all",
		"bulk-approve",
		"bulk_approve",
	}

	for _, filename := range files {
		data, err := fs.ReadFile(staticFS, filename)
		if err != nil {
			t.Fatalf("fs.ReadFile(%q): %v", filename, err)
		}
		lower := strings.ToLower(string(data))
		for _, term := range forbiddenTerms {
			if strings.Contains(lower, term) {
				t.Errorf("file %q contains forbidden bulk approval term %q", filename, term)
			}
		}
	}
}

func TestStaticAssetsContainOpencodeCommandAndInlineEvidence(t *testing.T) {
	staticFS := serve.StaticFS()

	htmlData, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("fs.ReadFile(index.html): %v", err)
	}
	htmlStr := string(htmlData)

	if !strings.Contains(htmlStr, "opencode") {
		t.Errorf("index.html does not contain opencode command section")
	}
	if !strings.Contains(htmlStr, "approvals-container") {
		t.Errorf("index.html does not contain approvals container")
	}

	jsData, err := fs.ReadFile(staticFS, "app.js")
	if err != nil {
		t.Fatalf("fs.ReadFile(app.js): %v", err)
	}
	jsStr := string(jsData)

	// Verify evidence validation logic is present
	if !strings.Contains(jsStr, "isValidEvidence") {
		t.Errorf("app.js does not contain isValidEvidence logic for inline evidence")
	}
}

func TestItemsStartUnselectedInUI(t *testing.T) {
	staticFS := serve.StaticFS()

	htmlData, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("fs.ReadFile(index.html): %v", err)
	}
	htmlStr := string(htmlData)

	if strings.Contains(htmlStr, "checked") || strings.Contains(htmlStr, "selected") {
		t.Errorf("index.html should not have pre-selected or pre-checked controls")
	}
}

func TestStaticEvidenceValidationRejectsBareMultilineProse(t *testing.T) {
	staticFS := serve.StaticFS()

	jsData, err := fs.ReadFile(staticFS, "app.js")
	if err != nil {
		t.Fatalf("fs.ReadFile(app.js): %v", err)
	}
	jsStr := string(jsData)

	if !strings.Contains(jsStr, "function isValidEvidence") {
		t.Fatalf("app.js does not define isValidEvidence")
	}

	// Finding 1 / Spec 61-65: Bare claim withheld -- bare multi-line prose with a newline
	// must NOT be treated as valid command output. The bare `trimmed.includes('\n')`
	// clause must be absent from isValidEvidence.
	if strings.Contains(jsStr, "trimmed.includes('\\n')") || strings.Contains(jsStr, "trimmed.includes(\"\\n\")") {
		t.Errorf("app.js contains over-permissive trimmed.includes('\\n') in isValidEvidence; bare multi-line prose must not qualify as valid evidence")
	}
}

func TestStaticAssetsRemainEmbeddedAndNoBuild(t *testing.T) {
	staticFS := serve.StaticFS()
	entries, err := fs.ReadDir(staticFS, ".")
	if err != nil {
		t.Fatalf("fs.ReadDir(static root): %v", err)
	}

	wantFiles := map[string]bool{"app.js": false, "index.html": false}
	for _, entry := range entries {
		if _, ok := wantFiles[entry.Name()]; ok && !entry.IsDir() {
			wantFiles[entry.Name()] = true
		}
	}
	for name, found := range wantFiles {
		if !found {
			t.Errorf("embedded static filesystem is missing %q", name)
		}
	}

	html := readStaticAsset(t, "index.html")
	assertContainsAll(t, "index.html", html, `<script src="/app.js"></script>`, `<style>`)
	for _, forbidden := range []string{"<script type=\"module\"", "node_modules", "https://", "http://", "<link rel=\"stylesheet\""} {
		if strings.Contains(html, forbidden) {
			t.Errorf("index.html contains build-time or external asset reference %q", forbidden)
		}
	}
}

func TestControlRoomShellHasPersistentChromeOutletsAndAccessibleStatus(t *testing.T) {
	html := readStaticAsset(t, "index.html")
	assertContainsAll(t, "index.html", html,
		"Control Room",
		`id="control-room-shell"`,
		`id="pending-approvals-count"`,
		`id="approver-name"`,
		`id="approver-rate"`,
		`id="connection-status"`,
		`role="status"`,
		`aria-live="polite"`,
		`id="connection-status-text"`,
		`data-view-outlet="approvals"`,
		`data-view-outlet="activity"`,
		`id="approvals-container"`,
	)
}

func TestLiveStoreUsesSSEFirstAndDeterministicPollingFallback(t *testing.T) {
	js := readStaticAsset(t, "app.js")

	t.Run("SSE open and named events refresh state", func(t *testing.T) {
		assertContainsAll(t, "app.js", js,
			"function createLiveStore",
			"new EventSourceImpl('/api/stream')",
			"eventSource.addEventListener('open'",
			"eventSource.addEventListener('event'",
			"eventSource.addEventListener('progress'",
			"eventSource.addEventListener('resync'",
			"eventSource.addEventListener('error'",
		)
	})

	t.Run("stream error starts one two-second polling loop", func(t *testing.T) {
		assertContainsAll(t, "app.js", js,
			"const POLL_INTERVAL_MS = 2000",
			"if (pollingTimer !== null) return",
			"pollingTimer = setIntervalImpl",
			"startPolling('Stream unavailable')",
			"fetchStateImpl('/api/state'",
		)
	})

	t.Run("HTTP failure retains cached state and publishes status", func(t *testing.T) {
		assertContainsAll(t, "app.js", js,
			"let cachedState = null",
			"cachedState = nextState",
			"showing cached data",
			"notify()",
			"getSnapshot",
			"teardown",
		)
		if count := strings.Count(js, "cachedState = null"); count > 1 {
			t.Errorf("app.js assigns null to cached state %d times; only initialization is allowed so failed refreshes retain the last state", count)
		}
	})
}

func TestApprovalsRemainIndividuallyPatchedAndSubmitted(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"function patchApprovalCards",
		"data-approval-key",
		"function submitDecision(runID, laneID, decision)",
		"`/approvals/${encodeURIComponent(runID)}/${encodeURIComponent(laneID)}`",
		"JSON.stringify({ decision })",
		"isValidEvidence",
	)
	for _, replacement := range []string{"containerEl.innerHTML", "approvalsContainer.innerHTML"} {
		if strings.Contains(js, replacement) {
			t.Errorf("app.js contains %q; live refreshes must patch approval cards without replacing the view outlet", replacement)
		}
	}
}

func TestFleetViewCoversEmptyRunningBlockedAndProgressRichStates(t *testing.T) {
	js := readStaticAsset(t, "app.js")

	tests := []struct {
		name      string
		contracts []string
	}{
		{
			name: "empty state is explicit",
			contracts: []string{
				"function patchFleetCards",
				"data-fleet-empty",
				"No lanes are reporting yet.",
			},
		},
		{
			name: "running state has text and a non-color symbol",
			contracts: []string{
				"running: ['▶', 'Running']",
				"fleet-status-symbol",
				"fleet-status-text",
				"formatElapsed(startedAt, endedAt, now)",
			},
		},
		{
			name: "blocked state has text and a distinct shape",
			contracts: []string{
				"blocked: ['■', 'Blocked']",
				"card.dataset.status = lane.status",
				"statusSymbol.setAttribute('aria-hidden', 'true')",
			},
		},
		{
			name: "progress rich state exposes activity and supplied telemetry",
			contracts: []string{
				"Executor",
				"Model",
				"SDD phase",
				"Fanout group",
				"Feature",
				"Worktree",
				"Attempt",
				"Elapsed",
				"latest_progress",
				"Latest activity",
				"total_tokens",
				"cost_usd",
				"tools_per_minute",
				"Tool rate",
				"Unavailable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContainsAll(t, "app.js", js, tt.contracts...)
		})
	}
}

func TestFleetViewNormalizesGoAndSnakeCaseFieldsWithKeyedPatching(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"function normalizeFleetState",
		"'run_id', 'RunID'",
		"'lane_id', 'LaneID'",
		"'sdd_phase', 'SDDPhase'",
		"'fanout_group', 'FanoutGroup'",
		"'worktree_path', 'WorktreePath'",
		"data-fleet-key",
		"if (card) updateFleetCard(card, lane)",
	)
	if strings.Contains(js, "fleetContainer.innerHTML") {
		t.Error("app.js replaces the Fleet outlet; live refreshes must patch keyed lane cards")
	}
}
