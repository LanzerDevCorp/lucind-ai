# Tasks Synthesis Notes: Control Room UI Shell

## Unresolved Contradictions

None

## Coverage Gaps

1. **`//go:embed static/*` does not embed `views/`.** `internal/serve/static.go:8` is `//go:embed static/*`. Go's `*` does not cross path separators, so `internal/serve/static/views/approvals.js` (A 3.1, design File Changes) is not in `embed.FS`. Design claimed the glob already covers new files; no lens tasked a `static.go` change. Apply of 3.1+4.1 will 404 `GET /views/approvals.js` and fail `fs.ReadFile` of that path unless the glob is widened. Not invented into `tasks.md`.

2. **Lens C Units 2+3 merged into Unit 2.** C split UI shell (delete `app.js`, new modules) from approvals view + `static_test.go`. `static_test.go:14,57,86` `ReadFile` `app.js`. A wave that deletes `app.js` without retargeting those tests fails `lucind-checks.sh` (`integrate.go:50-59`) and is reverted. B already kept assets and tests in one unit. C's suggested PR 2 → PR 3 is the same merge: one frontend PR.

3. **Skill size-budget drift.** `~/.claude/skills/sdd-tasks/SKILL.md` requires a 530-word tasks artifact. This packet sets 1800 words and wins on execution. Forecast fields, work-unit columns, specific/actionable/verifiable/small, and threat-matrix RED-test rules follow the skill.

4. **TDD order vs lens A.** `openspec/config.yaml` `apply.tdd: true` and the skill prefer RED before GREEN. A's authoritative checklist is 1.1 production then 1.2 tests. Both stay in Unit 1, so Integrate is green either way. Not reordered.

5. **No MIME test exists today.** 4.1 adds httptest MIME assertions; there is no current `TestStaticAssetsMIMETypes`. Package command is `go test ./internal/serve`.

## Dropped Citations

1. **A 1.2 / C Model GET proving command: `server_test.go:136` as 13 Model GET tests.** Line 136 is `TestSingleApprovalAndDefectEndpoints` (POST decide/defect). File kept as the HTTP test seam; `:136` as existing Model GET coverage dropped.

2. **A `index.html:1-163`.** File ends at line 162. Modify-shell claim kept against `:141-157` inbox and `:160` script tag.

3. **A `app.js:1-98`.** File ends at line 97. Delete task kept.

4. **C `model_test.go:22` as HTTP Model endpoint tests.** Line 22 is `openModelLedger`, a ledger helper. Not an HTTP test.

5. **C `-run 'TestModelEndpointsReturnJSON|TestModelEndpointsRejectNonGET|TestModelEndpointNotFound'`.** No such tests. `go test -run` on a missing name exits 0 with zero tests.

6. **C `-run 'TestStaticAssetsMIMETypes'` derived from `static_test.go:11,41`.** `:11` is `TestEmbedFSHasNoApproveAllControl` (kept). `:41` is `TestStaticAssetsContainOpencodeCommandAndInlineEvidence`, not MIME. `TestStaticAssetsMIMETypes` does not exist.

7. **C `-run 'TestSPAShellContainsOutletAndHeader'` derived from `static_test.go:41,69`.** Those are `TestStaticAssetsContainOpencodeCommandAndInlineEvidence` and `TestItemsStartUnselectedInUI`. Invented name dropped; retarget of those tests kept in 4.1.

8. **C `-run 'TestRouterModuleStructureAndLifecycle'`, `TestStoreModulePollingAndSubscriberAPI`, `TestApprovalsViewValidationAndSafety'` derived from `static_test.go:83`.** Line 83 is `TestStaticEvidenceValidationRejectsBareMultilineProse`. Invented names dropped; retarget of the evidence test kept in 4.1.

## Decomposition Divergence

Lens A's four-phase, 10-task checklist is canonical. Spec and design choose the same cut: Model GET on `NewHandler`, then modular ES shell + approvals view, then embed-test retarget. No Features-view tasks (design out of scope).

**Lens B** independently converged on that same work as two deliverables: Unit 1 = A's Phase 1; Unit 2 = A's Phases 2–4. Sidecar declined. Cost: none. Corroboration of the backend/frontend cut.

**Lens C** independently converged on the same files and invariants, then partitioned into three sequential units (Model GET → shell modules → approvals.js + tests). Critical path Unit 1 → 2 → 3 assumed shell before approvals, which matches A's 3.3-after-3.1, but placed `static_test.go` after `app.js` deletion. That partition is not Integrate-green (see Coverage Gaps). Forecast, N/A threat-matrix rows, preserved-invariant tests, and verification gaps (no browser harness; focus/scroll) survive. C's open question (3-unit chain vs size exception) is answered for apply shape (two units, no sidecar) and left as `ask-on-risk` for review chaining.
