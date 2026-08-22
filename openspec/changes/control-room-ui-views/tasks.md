# Tasks: Control Room UI Views

Canonical sources: `design.md`, `specs/*/spec.md`. Threat-matrix rows (`design.md:111-121`) are all `N/A`; do not add threat-matrix RED tests. Anti-bulk 400 and no "approve all" remain product tests.

**No `apply-dag.yaml` sidecar.** Single packet, three sequential work-unit commits (Unit 1 → 2 → 3). `Integrate` runs `lucind-checks.sh` on the combined tree (`internal/run/integrate.go:50-59`); a RED-only wave is reverted. Executors below are named for a later DAG; this change is one lane.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 690–1000 (Model ~220, Handlers ~260, Static UI ~350) across 7 files |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Model) → PR 2 (HTTP GET) → PR 3 (static UI) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

Basis: `model.go` 563 lines, `handlers.go` 231, `app.js` 97, `index.html` 162. Exceeds the 400-line review budget. Change folder has no `state.yaml`; apply must ask before chaining or `size:exception`.

### Suggested Work Units

| Unit | Goal | Likely PR | Executor | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------|----------------------|-----------------|-------------------|
| 1 | Add `BatchLane` + `ListBatchLanes` on `*serve.Model` with ledger + AST tests | PR 1 (base = feature/tracker) | cursor-agent | `go test -run 'TestBatchLanesRoundTrip\|TestModelSourceDoesNotShellOut' ./internal/serve` | N/A: SQLite via `openModelLedger` (`model_test.go:22-30`); no HTTP | Revert `model.go` / `model_test.go`; existing `ListFeatures` (`model.go:128`) unchanged |
| 2 | Mount additive GET `/api/*` on `NewHandler`; keep `/api/state` and bulk POST 400 | PR 2 (base = PR 1) | cursor-agent | `go test -run 'TestGetRoutesReturnJSON\|TestBulkRequestBodyReturns400\|TestDecideAlreadyDecidedReturns409Conflict' ./internal/serve` | N/A: `httptest` against `NewHandler`; no live `serve` process | Revert `handlers.go` / `server_test.go`; `/` `/api/state` `/approvals/` remain |
| 3 | Five-panel HTML/JS: tiered poll, keyed DOM, `escapeHtml`; embed assertions | PR 3 (base = PR 2) | agy | `go test -run 'TestStaticAssetsContainFivePanels\|TestEmbedFSHasNoApproveAllControl\|TestItemsStartUnselectedInUI\|TestStaticEvidenceValidationRejectsBareMultilineProse' ./internal/serve` | N/A: embed string checks only; repo has no headless browser | Revert `static/app.js` `static/index.html` `static_test.go` |

`allowed_paths`: Unit 1 `internal/serve/model.go`, `internal/serve/model_test.go`. Unit 2 `internal/serve/handlers.go`, `internal/serve/server_test.go`. Unit 3 `internal/serve/static/app.js`, `internal/serve/static/index.html`, `internal/serve/static_test.go`. File paths, not `internal/serve/`. Same-wave pairs: none (single packet). Prefix check (`internal/packet/disjoint.go:13-21`): `model.go` is not a prefix of `static/` or `handlers.go`.

Does not prove: live 2s timers, scroll preservation, or visual layout.

## Phase 1: Data Access & DTO Foundation

- [ ] 1.1 Add `BatchLane` and `ListBatchLanes(ctx, runID) ([]BatchLane, error)` on `*serve.Model`. DTO neighborhood today is feature-parent types (`model.go:26-125`); methods start at `ListFeatures` (`:128`). SELECT `lanes` / `lane_note` events (`internal/ledger/schema.go:18-43`); map status (`internal/lane/status.go:10-16`), worktree, preserved flag (`schema.go:26-27`), `barrier.Evaluate` (`internal/barrier/barrier.go:36-60`). No `os`/`exec`/`git`.
- [ ] 1.2 Add `TestBatchLanesRoundTrip` in `model_test.go` using `openModelLedger` (`:22-30`): status mapping, worktree preservation, demotion note, barrier outcome. Keep `TestModelSourceDoesNotShellOut` (`:595-627`).

## Phase 2: HTTP Endpoints & Route Wiring

- [ ] 2.1 In `NewHandler` (`handlers.go:36-118`) mount GET `/api/approvals`, `/api/features`, `/api/features/{id}/attempts`, `/api/leases`, `/api/overlap/{feature_id}`, `/api/reconcile/requests`, `/api/batch/lanes`. Keep `GET /api/state` (`:79-85`) and bulk POST 400 (`:161-176`). Call `ListBatchLanes` plus existing `ListFeatures` `:128`, `ListAttempts` `:167`, `ListLeases` `:206`, `ListOverlapEvidence` `:245`, composed `ListReconciliationRequests` `:278`. Nil slices encode as `[]`.
- [ ] 2.2 Add `TestGetRoutesReturnJSON` in `server_test.go`: 200 JSON, empty `[]`, error paths. Keep `TestBulkRequestBodyReturns400` (`:42-93`) and `TestDecideAlreadyDecidedReturns409Conflict` (`:196-236`).

## Phase 3: Dashboard Layout & Panel Structure

- [ ] 3.1 In `index.html:141-162` add containers for Batch/Wave, Approvals, Feature/Lease, Reconciliation, Lane Envelope. Keep `approver-name`, `approver-rate`, `opencode-cmd` (`:146-152`).

## Phase 4: Client Controller & Dynamic Rendering

- [ ] 4.1 Replace the single `/api/state` poller (`app.js:1-97`) with five-panel fetch. Hot 2s poll (`:96-97`) only `/api/approvals`, `/api/leases`, `/api/batch/lanes`; lazy-fetch overlap `evidence_json` and candidate `output`/`checks` on expand. Patch by `card-${runID}-${laneID}` (`:49`); do not wipe with `innerHTML = ''` (`:45`) every tick. Keep `escapeHtml` (`:91-94`), `isValidEvidence` (`:12-20`), per-card POST (`:72-78`).

## Phase 5: Static Asset & UI Contract Verification

- [ ] 5.1 Add `TestStaticAssetsContainFivePanels` in `static_test.go` via `StaticFS()` (`static.go:12-18`): five panel IDs, metric IDs present. Keep no-approve-all (`:11-38`), unselected cards (`:69-80`), `isValidEvidence` rejects bare prose (`:83-101`).

## Dependency Order

| Task | Depends on | Why |
|------|------------|-----|
| 1.1 | — | New Model API; ledger + barrier only |
| 1.2 | 1.1 | Exercises `ListBatchLanes` |
| 2.1 | 1.1 | Handlers call `ListBatchLanes` (will not compile without 1.1) |
| 2.2 | 2.1 | HTTP tests need mounted routes |
| 3.1 | — | Markup only |
| 4.1 | 2.1, 3.1 | Fetches 2.1 routes; binds 3.1 containers |
| 5.1 | 3.1, 4.1 | Embed tests assert HTML/JS |

Commit order = Unit 1 (1.1–1.2), Unit 2 (2.1–2.2), Unit 3 (3.1, 4.1, 5.1). 1.2 and 2.2 stay in the same lane as their production task so a RED wave is never what `Integrate` sees.

## Threat-matrix RED tests

None. `design.md:111-121`: Documentation-like paths, Git repository selection, Commit state, Push state, PR commands — all `N/A`. Product tests 2.2 and 5.1 cover anti-bulk / no approve-all; those are not threat rows.

## Requirement Traceability

| Requirement | Tasks |
|-------------|-------|
| Individual Decisions Without Bulk Approval (`specs/approvals-web-ui/spec.md:5-37`) | 2.1, 2.2, 3.1, 4.1, 5.1 |
| Batch and DAG Wave Inspection (`specs/batch-wave-view/spec.md:5-26`) | 1.1, 1.2, 2.1, 2.2, 3.1, 4.1, 5.1 |
| Shell-Free Feature and Lease Monitoring (`specs/feature-lease-monitor/spec.md:5-32`) | 1.2, 2.1, 2.2, 3.1, 4.1, 5.1 |
| Lane Demotion Diagnosis (`specs/lane-envelope-inspector/spec.md:5-33`) | 1.1, 1.2, 2.1, 2.2, 3.1, 4.1, 5.1 |
| Reconciliation Candidate Inspection (`specs/reconciliation-workspace/spec.md:5-25`) | 2.1, 2.2, 3.1, 4.1, 5.1 |

## Open Questions

- [ ] Reconcile UI: copy-paste CLI (`cmd/lucind-ai/cli.go:1044-1065`) vs future POST? Surface is read-only (`design.md:133`).
- [ ] Lease countdown: Model `remaining_seconds` vs `expires_at` plus server time (`design.md:134`).
- [ ] Overlap `evidence_json`: `<pre>` + `escapeHtml` vs tokenizer (`design.md:135`).
- [ ] `NewHandler` builds `NewModel(l)` internally vs a `*Model` parameter (`design.md:136`; `handlers.go:36`).
