# Tasks Synthesis Notes: Control Room UI Views

## Unresolved Contradictions

**Wave order vs task 4.1.** Lens A (authoritative checklist) states task 4.1 depends on 2.1: the frontend fetches the GET routes mounted in `NewHandler` (`handlers.go:36-118`) and binds containers from 3.1. Lens B’s Wave 1 runs Unit 3 (HTML + `app.js` + `static_test.go`, including 4.1) in parallel with Unit 1 and **before** Unit 2. Lens C’s assumed critical path is Unit 1 → Unit 2 → Unit 3, which matches A’s 4.1→2.1 order and not B’s wave.

Code does not settle dispatch order. Unit 3’s proving commands are embed string checks (`static_test.go:11-102`); they do not call HTTP. Unit 3 therefore compiles and passes `lucind-checks.sh` without the new routes. Unit 2 calling `ListBatchLanes` does **not** compile without Unit 1. Both A’s logical/contract order and B’s compile-green parallel (1+3) are viable. Not picked. Canonical `tasks.md` follows A’s dependency table inside lens B’s recommended **single packet** (no sidecar), so B’s two-wave DAG is not the dispatch shape. Apply must not author `apply-dag.yaml` that places 4.1 before 2.1 without a human decision.

## Coverage Gaps

- **Waves merged for Integrate viability: none.** Re-check of lens B’s Wave 1 (Units 1+3): combined tree adds unused exported Model methods plus static files; `handlers.go` unchanged, so no missing-symbol compile. Wave 2 (Unit 2) is green only after Wave 1 supplies `ListBatchLanes`. Neither wave required merging. Not dispatched (no sidecar).
- **Skill 530-word tasks budget vs this packet’s 1800-word ceiling.** `~/.claude/skills/sdd-tasks/SKILL.md` sizes the artifact at 530 words. Packet execution rules win. Forecast fields, work-unit columns, specific/actionable/verifiable/small, and threat-matrix RED rule followed from the skill. Not a missing spine item.
- **No `state.yaml` / human `delivery_strategy` in the change folder or packet Context.** Forecast uses lens C’s `ask-on-risk` + `feature-branch-chain` (C owns that table). Decision needed before apply remains Yes.
- **Runtime harness column.** Neither B nor C populated the skill’s Runtime harness field. Values in `tasks.md` are N/A with reasons taken from lens C Verification Gaps (no Chromedp/Playwright; HTTP is `httptest`; Model is `openModelLedger`). Not invented live-serve scripts.
- **Browser / visual proof.** Lens C: no headless DOM or visual regression in-repo. Unfilled; not invented.
- **Kahn waves and per-lane deadlines on `ListBatchLanes`.** Design-synthesis already dropped SQL-backed waves (`internal/dag/waves.go:41-70` is in-memory Kahn) and deadlines (`internal/run/batch.go:40-43` is `LaneTimeout` on `ExecuteBatch`). Task 1.1 follows design: `lanes` columns + `lane_note` + `barrier.Evaluate` only. Spec still names wave grouping and deadlines; reconstruction from SQLite is still unnamed.
- **`openspec/config.yaml` `apply.tdd: true` vs lens A’s test-after-production order.** A sequences 1.1 then 1.2, 2.1 then 2.2. Not reordered (A owns decomposition). Tests stay in the same unit/lane as production so `Integrate` never sees a RED-only wave.

## Dropped Citations

Claims removed from `tasks.md` because the cited range did not support them. Neighboring true facts kept with a verified line.

### Lens A

- **`internal/serve/model_test.go:595-627` as the insertion site for `ListBatchLanes` tests.** Those lines are `TestModelSourceDoesNotShellOut` (AST import/body scan). Task 1.2 still adds `TestBatchLanesRoundTrip` in `model_test.go` via `openModelLedger` (`:22-30`) and keeps `:595-627` passing. The “modify :595-627 to add ListBatchLanes tests” pin is dropped.
- **`internal/serve/server_test.go:42-93` as the insertion site for new GET route tests.** Those lines are `TestBulkRequestBodyReturns400`. Task 2.2 adds `TestGetRoutesReturnJSON` elsewhere in the file and keeps `:42-93` and `:196-236`.
- **`internal/serve/static/app.js:1-98` as the whole client.** File ends at line 97 (`setInterval(fetchState, 2000)`). Kept as `:1-97`.

### Lens B

- No `file:line` claim dropped. `disjoint.go:13-21` is `PathInScope` / component-boundary prefix. Archive paths exist (`openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md`, `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml`) and support the no-sidecar precedent.

### Lens C

- **`internal/serve/model.go:85-115` as `BatchLane` / `ListBatchLanes`.** Those lines are `ReconciliationRequest`, `CASResult`, and `ReconciliationCandidate`. `BatchLane` does not exist. Neighborhood kept via design’s `model.go:26-125` (feature-parent DTOs only).
- **`internal/serve/handlers.go:80-145` as the seven new GET routes.** `:79-85` is `GET /api/state`; `:120-145` is `serveStateJSON`. New routes mount in `NewHandler` (`:36-118`), which today has `/`, `/api/state`, `/approvals/` only (no `/api/approvals` etc.).
- **`internal/serve/static/index.html:140-220` as five-panel markup.** File ends at 162. Body/metric IDs are `:141-162` (single-panel today).
- **`internal/serve/static/app.js:20-130` as the multi-panel poller.** File ends at 97. Kept `:1-97` / `:12-20` / `:45` / `:49` / `:91-94` / `:96-97`.
- **Proving-command names `TestBatchLanesRoundTrip`, `TestGetRoutesReturnJSON`, `TestStaticAssetsContainFivePanels`.** They do not exist yet (`model_test.go` has `TestStatusRoundTripFromWriteAPIs` at `:74`, `TestReconciliationObservableStatus` at `:370`, AST at `:595`; `server_test.go` has bulk/unselected/defect/409; `static_test.go` has four embed tests). Kept as names of tests the checklist must add, not as existing symbols.

## Decomposition Divergence

**Not irreconcilable on what to build.** All three name the same seven files and the same three slices: Model `BatchLane`/`ListBatchLanes`, `NewHandler` GET routes, five-panel static UI. Lens A’s five phases nest into that: Phase 1 = Unit 1, Phase 2 = Unit 2, Phases 3–5 = Unit 3.

**Independent convergence.** B and C, without seeing A, both grouped HTML + JS + `static_test.go` as one UI unit and Model vs HTTP as two Go units. Treat as corroboration of A’s file cut.

**B vs A.** B parallelizes Unit 3 with Unit 1 (Wave 1) and delays HTTP to Wave 2. A’s 4.1 depends on 2.1; 3.1 does not. B’s grouping of 3.1+4.1+5.1 into one unit makes the UI unit inherit 4.1’s HTTP dependency. Content B lost as dispatch: the two-wave DAG (see Unresolved Contradictions). Sidecar recommendation (single packet, three commits) survives.

**C vs A.** C’s opening block is three sequential units 1→2→3, matching A’s 4.1→2.1 and not B’s waves. C did not emit a phase-3-only HTML task; 3.1 is inside Unit 3. Forecast, N/A threat rows, proving commands (after citation repair), and verification gaps survive. C did not name executors or `allowed_paths`; those come from B.

**Content lost to A’s checklist gate.** None. Every B unit and C acceptance row maps to A tasks 1.1–5.1.
