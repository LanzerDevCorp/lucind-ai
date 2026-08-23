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

func TestApplyDAGViewConsumesServerTopologyWithoutClientRecomputation(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"function renderApplyDAG(dag)",
		"renderApplyDAG(state.apply_dag)",
		"dag.waves.forEach",
		"wave.packets.forEach",
		"dag.dependencies.forEach",
		"dag.overlap_violations.forEach",
	)

	for _, forbidden := range []string{"topologicalSort", "computeWaves", "inferDependencies", "deriveGraph"} {
		if strings.Contains(js, forbidden) {
			t.Errorf("app.js contains %q; the browser must render the API DAG without deriving topology", forbidden)
		}
	}
}

func TestApplyDAGViewSupportsMultipleWavesDependenciesTerminalStatusesAndOverlapErrors(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"data-wave-number",
		"data-packet-id",
		"dependency.from",
		"dependency.to",
		"packet.status",
		`[data-status="done"]`,
		`[data-status="blocked"]`,
		`[data-status="deviated"]`,
		`[data-status="failed"]`,
		"dag-overlap-error",
		"JSON.stringify(violation, null, 2)",
	)
}

func TestApplyDAGViewKeepsLiveStatusesAndDiagnosticsAsServerText(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		`[data-status="pending"]`,
		`[data-status="running"]`,
		"packetStatus.textContent = ` ${packet.status}`",
		"edge.textContent = `${dependency.from} → ${dependency.to}`",
		"error.textContent = JSON.stringify(violation, null, 2)",
		"region.replaceChildren(fragment)",
	)

	if strings.Contains(js, "region.innerHTML") {
		t.Error("app.js assigns region.innerHTML; server-provided DAG diagnostics must use safe DOM text")
	}
}

func TestSDDFlowViewCoversCompletePartialAndEmptyRails(t *testing.T) {
	js := readStaticAsset(t, "app.js")

	tests := []struct {
		name      string
		contracts []string
	}{
		{
			name: "complete rail separates planning and execution",
			contracts: []string{
				"const SDD_PLANNING_PHASES",
				"{ key: 'explore', label: 'Explore' }",
				"{ key: 'proposal', label: 'Proposal' }",
				"{ key: 'spec', label: 'Spec' }",
				"{ key: 'design', label: 'Design' }",
				"{ key: 'tasks', label: 'Tasks' }",
				"const SDD_EXECUTION_PHASES",
				"{ key: 'apply', label: 'Apply' }",
				"{ key: 'verify', label: 'Verify' }",
				"{ key: 'archive', label: 'Archive' }",
				"Planning rail",
				"Execution",
			},
		},
		{
			name: "planning fanout distinguishes lenses from synthesis",
			contracts: []string{
				"Planning lenses",
				"Synthesis lane",
				"data-flow-role",
				"Lens ${index + 1}",
				"flow.lane_ids",
			},
		},
		{
			name: "partial rail reports absent phases without inventing status",
			contracts: []string{
				"Not reported by server",
				"renderSDDPhase",
			},
		},
		{
			name: "empty flow has a dedicated empty state",
			contracts: []string{
				"No SDD flows reported.",
				"renderSDDFlows",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContainsAll(t, "app.js", js, tt.contracts...)
		})
	}
}

func TestSDDFlowViewUsesOnlyServerPayloadFields(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"const SDD_FLOW_FIELDS",
		"'run_id'",
		"'change'",
		"'sdd_phase'",
		"'fanout_group'",
		"'status'",
		"'lane_count'",
		"'lane_ids'",
	)

	start := strings.Index(js, "function normalizeSDDFlow")
	end := strings.Index(js, "function setupViewNavigation")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("app.js must define its SDD flow renderer before setupViewNavigation")
	}
	renderer := js[start:end]
	for _, absentField := range []string{
		"timestamp", "duration", "dependencies", "artifacts", "artifact_presence", "executor",
	} {
		if strings.Contains(renderer, absentField) {
			t.Errorf("SDD flow renderer references absent server field %q", absentField)
		}
	}
}

func TestFeatureSwimlanesViewCoversActiveExpiredBlockedPromotedAndReconciliationStates(t *testing.T) {
	js := readStaticAsset(t, "app.js")

	tests := []struct {
		name      string
		contracts []string
	}{
		{
			name: "active state is explicit for feature and live lease",
			contracts: []string{
				"featureStatus === 'active'",
				`card.dataset.status = feature.status`,
				`card.dataset.featureStatus = feature.featureStatus`,
				`[data-status="active"]`,
				`card.dataset.leaseStatus = feature.lease.status`,
				"badge-feature-status",
			},
		},
		{
			name: "expired state covers past lease TTL and expired status",
			contracts: []string{
				"formatTTL(expiresAt, now",
				"return 'Expired'",
				`card.dataset.leaseStatus = feature.lease.status`,
				`badge-expired`,
				`data-status="expired"`,
				`[data-status="expired"]`,
			},
		},
		{
			name: "blocked state renders attempt status and diagnostic failure reason",
			contracts: []string{
				`setAttribute('data-attempt-status', attempt.status)`,
				`[data-status="blocked"]`,
				"badge-blocked",
				"failure-reason",
				"Failure reason",
			},
		},
		{
			name: "promoted state renders promoted badge and candidate SHA",
			contracts: []string{
				`setAttribute('data-attempt-status', 'promoted')`,
				`[data-status="promoted"]`,
				"badge-promoted",
				"Candidate SHA",
			},
		},
		{
			name: "reconciliation-required state renders badge and request details",
			contracts: []string{
				"reconciliationRequired",
				`setAttribute('data-reconcile-badge', 'required')`,
				"Reconciliation required",
				`data-reconcile-required`,
				`[data-status="reconciliation-required"]`,
				"reconcile-card",
				"data-reconcile-status",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContainsAll(t, "app.js", js, tt.contracts...)
		})
	}
}

func TestFeatureSwimlanesRenderParentBaseRefsLeaseTTLOverlapEvidenceAndReconciliationBadges(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"function normalizeFeatureSwimlanes",
		"function renderFeatureSwimlanes",
		"function ensureFeatureSwimlanesContainer",
		"Parent / Base refs",
		"Parent ref",
		"Base SHA",
		"Expected parent SHA",
		"Lease & TTL",
		"Lease holder",
		"Lease fence",
		"Live TTL",
		"Integration attempts",
		"Candidate SHA",
		"Overlap evidence",
		"overlap-evidence-json",
		"Reconciliation",
		"Reconciliation required",
	)
}

func TestFeatureSwimlanesNormalizesGoAndSnakeCaseFieldsWithKeyedPatching(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"normalizeFeatureSwimlanes",
		"'parent_ref', 'ParentRef'",
		"'base_sha', 'BaseSHA'",
		"'expected_parent_sha', 'ExpectedParentSHA'",
		"'expires_at', 'ExpiresAt'",
		"'candidate_sha', 'CandidateSHA'",
		"'failure_reason', 'FailureReason'",
		"'evidence_class', 'EvidenceClass'",
		"'evidence_hash', 'EvidenceHash'",
		"'evidence_json', 'EvidenceJSON'",
		"data-feature-key",
		"createFeatureSwimlaneCard",
		"updateFeatureSwimlaneCard",
		"patchFeatureSwimlanes",
	)

	for _, badReplacement := range []string{
		"swimlanesContainer.innerHTML",
		"featureContainer.innerHTML",
	} {
		if strings.Contains(js, badReplacement) {
			t.Errorf("app.js replaces feature container with %q; live refreshes must patch keyed feature swimlane cards", badReplacement)
		}
	}
}

func TestFeatureSwimlanesKeepsDiagnosticsAndJsonAsSafeServerText(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"card.replaceChildren",
		"pre.className = 'overlap-evidence-json'",
		"pre.textContent = overlap.evidenceJSON",
		"pre.className = 'failure-reason'",
		"pre.textContent = attempt.failureReason",
		"data-features-empty",
		"No features reported.",
	)
}

func TestTimelineViewCoversOrderingFiltersCursorContinuationEmptyAndLargeFeeds(t *testing.T) {
	js := readStaticAsset(t, "app.js")

	tests := []struct {
		name      string
		contracts []string
	}{
		{
			name: "ordering sorts by timestamp and preserves stable tie-breaking",
			contracts: []string{
				"function sortTimelineItems",
				"parseTimelineTimestamp(a.at)",
				"parseTimelineTimestamp(b.at)",
				"const kindPriority",
				"timeA - timeB",
				"String(a.key).localeCompare",
			},
		},
		{
			name: "filters support event kinds search query and identity criteria",
			contracts: []string{
				"function filterTimelineItems",
				"data-filter-kind",
				"timeline-search-input",
				"item.kind !== kind",
				"item.runID",
				"item.laneID",
				"item.featureID",
				"item.type",
				"haystack.includes(search)",
			},
		},
		{
			name: "cursor continuation supports cursor tracking and forward slicing",
			contracts: []string{
				"function filterAfterCursor",
				"data-cursor",
				"item.cursor === cursor",
				"nextCursor",
			},
		},
		{
			name: "empty data state covers both unpopulated feed and no filter matches",
			contracts: []string{
				"data-timeline-empty",
				"No timeline events reported.",
				"No timeline events match the selected filters.",
			},
		},
		{
			name: "large feeds enforce bounded windowing and load more pagination",
			contracts: []string{
				"const TIMELINE_WINDOW_SIZE",
				"function virtualizeTimeline",
				"data-timeline-window",
				"data-timeline-count",
				"data-timeline-more",
				"clampedLimit",
				"clampedOffset",
				"hasMore",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContainsAll(t, "app.js", js, tt.contracts...)
		})
	}
}

func TestTimelineNormalizesGoAndSnakeCaseFieldsWithKeyedPatching(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"function normalizeTimeline",
		"function renderTimeline",
		"function ensureTimelineContainer",
		"function createTimelineCard",
		"function updateTimelineCard",
		"function patchTimelineItems",
		"'run_events', 'RunEvents'",
		"'integration_events', 'IntegrationEvents'",
		"'audit_events', 'AuditEvents'",
		"'lane_progress', 'LaneProgress'",
		"'run_id', 'RunID'",
		"'lane_id', 'LaneID'",
		"'feature_id', 'FeatureID'",
		"'attempt_id', 'AttemptID'",
		"data-timeline-key",
		"data-timeline-feed",
	)

	for _, badReplacement := range []string{
		"timelineContainer.innerHTML",
		"feed.innerHTML",
		"feedContainer.innerHTML",
	} {
		if strings.Contains(js, badReplacement) {
			t.Errorf("app.js replaces timeline container with %q; live refreshes must patch keyed timeline cards", badReplacement)
		}
	}
}

func TestTimelineKeepsDetailsAsSafeServerText(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"card.append(header, meta, detailBox)",
		"pre.className = 'timeline-detail-text'",
		"pre.textContent = item.detail",
		"timeline-badge badge-",
		"Execution timeline",
	)
}

func TestApprovalsViewCoversPendingDecidedEvidenceDefectAndBulkRefusalStates(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	html := readStaticAsset(t, "index.html")

	tests := []struct {
		name      string
		contracts []string
	}{
		{
			name: "pending state renders lane packet and active decision actions",
			contracts: []string{
				"function createApprovalCard",
				"function updateApprovalCard",
				"data-approval-key",
				"data-decision",
				"badge-decision",
				"btn-approve",
				"btn-reject",
				"parts.approveButton.disabled = false",
				"parts.rejectButton.disabled = false",
				"Approve",
				"Reject",
			},
		},
		{
			name: "decided state renders decision status and disables or reflects completed decisions",
			contracts: []string{
				"badge-decision",
				"parts.decisionBadge.textContent = item.decision",
				"parts.decisionBadge.dataset.decision = item.decision",
				"parts.approveButton.disabled = true",
				"parts.rejectButton.disabled = true",
				"approval-meta",
				"approver-info",
				"decided-info",
			},
		},
		{
			name: "evidence state distinguishes valid command/file:line evidence from missing evidence note",
			contracts: []string{
				"function isValidEvidence",
				"evidence-block",
				"no-evidence",
				"No command output or file:line evidence provided.",
				"Approval evidence",
				"parts.evidence.textContent = evidenceValid ? item.evidence : ''",
			},
		},
		{
			name: "defect state exposes defect action and reflects defect flag",
			contracts: []string{
				"function submitDefect",
				"btn-defect",
				"badge-defect",
				"data-defect",
				"Defect surfaced",
				"Mark defect",
				"Unmark defect",
				"`/approvals/${encodeURIComponent(runID)}/${encodeURIComponent(laneID)}/defect`",
				"JSON.stringify({ defect })",
			},
		},
		{
			name: "bulk-action refusal enforces individual single-lane decisions and rejects bulk controls",
			contracts: []string{
				"function submitDecision",
				"`/approvals/${encodeURIComponent(runID)}/${encodeURIComponent(laneID)}`",
				"JSON.stringify({ decision })",
				"patchApprovalCards",
				"data-empty-state",
				"No pending approvals.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContainsAll(t, "app.js", js, tt.contracts...)
		})
	}

	// Verify HTML contains container and styles for approval states
	assertContainsAll(t, "index.html", html,
		`id="approvals-container"`,
		`class="approvals-list"`,
		`[data-decision="approved"]`,
		`[data-decision="rejected"]`,
		`[data-decision="timed_out"]`,
		`[data-defect="true"]`,
		`badge-decision`,
		`badge-defect`,
		`btn-defect`,
	)
}

func TestApprovalsNormalizesGoAndSnakeCaseFieldsWithKeyedPatching(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"function normalizeApproval",
		"function normalizeApprovals",
		"'run_id', 'RunID'",
		"'lane_id', 'LaneID'",
		"'packet_id', 'PacketID'",
		"'approver', 'Approver'",
		"'evidence', 'Evidence'",
		"'decision', 'Decision'",
		"'defect_surfaced_later', 'DefectSurfacedLater'",
		"'requested_at', 'RequestedAt'",
		"'decided_at', 'DecidedAt'",
		"data-approval-key",
		"if (card) {",
		"updateApprovalCard(card, item)",
	)

	for _, badReplacement := range []string{
		"approvalsContainer.innerHTML",
		"containerEl.innerHTML",
	} {
		if strings.Contains(js, badReplacement) {
			t.Errorf("app.js replaces approvals container with %q; live refreshes must patch keyed approval cards", badReplacement)
		}
	}
}

func TestApprovalsKeepsEvidenceAndNotesAsSafeServerText(t *testing.T) {
	js := readStaticAsset(t, "app.js")
	assertContainsAll(t, "app.js", js,
		"parts.evidence.textContent = evidenceValid ? item.evidence : ''",
		"parts.lane.textContent = `Lane: ${item.laneID || '-'}`",
		"parts.packet.textContent = `Packet: ${item.packetID || '-'}`",
		"parts.decisionBadge.textContent = item.decision",
		"parts.approverInfo.textContent = item.approver ? `Approver: ${item.approver}` : ''",
		"parts.decidedInfo.textContent = item.decidedAt ? `Decided: ${item.decidedAt}` : ''",
	)
}

func TestStaticWiringAllSixViewsLoadAndWireState(t *testing.T) {
	js := readStaticAsset(t, "app.js")

	t.Run("all six view modules are defined", func(t *testing.T) {
		assertContainsAll(t, "app.js", js,
			// View 1: Approvals
			"function normalizeApprovals",
			"function patchApprovalCards",
			"function createApprovalCard",
			"function updateApprovalCard",
			"function submitDecision",
			"function submitDefect",
			// View 2: Fleet
			"function normalizeFleetState",
			"function patchFleetCards",
			"function createFleetCard",
			"function updateFleetCard",
			// View 3: Apply DAG
			"function renderApplyDAG",
			"function createApplyDAGRegion",
			// View 4: SDD Flows
			"function normalizeSDDFlow",
			"function groupSDDFlowsByChange",
			"function renderSDDFlows",
			"function ensureSDDFlowsContainer",
			// View 5: Feature Swimlanes
			"function normalizeFeatureSwimlanes",
			"function renderFeatureSwimlanes",
			"function patchFeatureSwimlanes",
			"function ensureFeatureSwimlanesContainer",
			// View 6: Timeline
			"function normalizeTimeline",
			"function renderTimeline",
			"function patchTimelineItems",
			"function ensureTimelineContainer",
		)
	})

	t.Run("renderState wires all six views into the embedded application", func(t *testing.T) {
		assertContainsAll(t, "app.js", js,
			"function renderState(state)",
			"patchApprovalCards(approvalsContainer, state.approvals || [])",
			"patchFleetCards(fleetContainer, normalizeFleetState(state, serverNow()))",
			"renderApplyDAG(state.apply_dag)",
			"renderSDDFlows(ensureSDDFlowsContainer(), sddFlows)",
			"renderFeatureSwimlanes(ensureFeatureSwimlanesContainer(), features)",
			"renderTimeline(ensureTimelineContainer(), timeline)",
		)
	})

	t.Run("live store retains /api/state polling fallback", func(t *testing.T) {
		assertContainsAll(t, "app.js", js,
			"function createLiveStore",
			"const POLL_INTERVAL_MS = 2000",
			"fetchStateImpl('/api/state'",
			"startPolling('Stream unavailable')",
			"cachedState = nextState",
		)
	})
}

func TestStaticAssetsContainAllViewContainers(t *testing.T) {
	html := readStaticAsset(t, "index.html")
	js := readStaticAsset(t, "app.js")

	t.Run("static HTML contains shell and baseline view containers", func(t *testing.T) {
		assertContainsAll(t, "index.html", html,
			`id="control-room-shell"`,
			`id="approvals-view"`,
			`id="approvals-container"`,
			`id="activity-view"`,
			`data-view-outlet="approvals"`,
			`data-view-outlet="activity"`,
			`id="pending-approvals-count"`,
			`id="approver-name"`,
			`id="approver-rate"`,
			`id="connection-status"`,
			`id="connection-status-text"`,
			`id="opencode-cmd"`,
		)
	})

	t.Run("embedded JavaScript defines dynamic view containers rendered by the shell", func(t *testing.T) {
		assertContainsAll(t, "app.js", js,
			// Apply DAG view container
			"apply-dag-view",
			"createApplyDAGRegion",
			"renderApplyDAG",
			// SDD Flows view container
			"sdd-flows-container",
			"ensureSDDFlowsContainer",
			"renderSDDFlows",
			// Feature Swimlanes view container
			"feature-swimlanes-container",
			"ensureFeatureSwimlanesContainer",
			"renderFeatureSwimlanes",
			// Timeline view container
			"timeline-container",
			"ensureTimelineContainer",
			"renderTimeline",
		)
	})
}

func TestStaticAssetsContainFivePanels(t *testing.T) {
	TestStaticAssetsContainAllViewContainers(t)
}




func TestLeaseCountdownIsAnchoredToServerClockNotBrowserClock(t *testing.T) {
	js := readStaticAsset(t, "app.js")

	// The point of the anchor is that no countdown is computed from the raw
	// browser clock. Assert both halves: the offset is measured from the
	// server's own timestamp, and the normalizers that feed every TTL and
	// lease-status badge are handed serverNow() rather than Date.now().
	assertContainsAll(t, "app.js", js,
		"function syncServerClock(state",
		"state.server_time",
		"serverClockOffsetMs = parsed - now",
		"function serverNow(",
		"return now + serverClockOffsetMs",
		"syncServerClock(state);",
		"normalizeFleetState(state, serverNow())",
		"normalizeFeatureSwimlanes(state, serverNow())",
	)

	// A stale anchor is as bad as no anchor, so the sync must happen on the
	// render path that consumes it, ahead of the first normalizer call.
	syncAt := strings.Index(js, "syncServerClock(state);")
	fleetAt := strings.Index(js, "normalizeFleetState(state, serverNow())")
	featuresAt := strings.Index(js, "normalizeFeatureSwimlanes(state, serverNow())")
	if syncAt < 0 || fleetAt < 0 || featuresAt < 0 {
		t.Fatalf("anchor call sites missing: sync=%d fleet=%d features=%d", syncAt, fleetAt, featuresAt)
	}
	if syncAt > fleetAt || syncAt > featuresAt {
		t.Errorf("syncServerClock at %d runs after a normalizer (fleet=%d, features=%d) -- the offset would be one payload stale", syncAt, fleetAt, featuresAt)
	}

	// Nothing that renders a countdown may reach for Date.now() directly.
	for _, forbidden := range []string{
		"normalizeFleetState(state, Date.now())",
		"normalizeFeatureSwimlanes(state, Date.now())",
	} {
		if strings.Contains(js, forbidden) {
			t.Errorf("app.js still contains %q -- that countdown is differenced against the viewer's own clock", forbidden)
		}
	}
}
