# Tasks Lens A — Decomposition & Ordering: Control Room UI Views

## Assumed decomposition

The implementation is broken down into five sequential phases spanning backend data modeling, HTTP route exposure, static HTML structuring, client-side controller logic, and static asset regression testing. Phase 1 extends the shell-free query model with batch and DAG lane retrieval, Phase 2 exposes modular REST GET endpoints, Phase 3 updates the HTML skeleton for five distinct panels, Phase 4 adds the frontend poller with keyed DOM patching and XSS escaping, and Phase 5 verifies embed assets and security invariants. The critical path follows Model data querying (`internal/serve/model.go`) -> HTTP route handlers (`internal/serve/handlers.go`) -> client-side controller (`internal/serve/static/app.js`) -> static asset test verification (`internal/serve/static_test.go`).

## Phase 1: Data Access & DTO Foundation

- [ ] 1.1 Modify `internal/serve/model.go:26-125` to define the `BatchLane` DTO and implement `ListBatchLanes(ctx context.Context, runID string) ([]BatchLane, error)` querying `lanes` and `events` (`lane_note`) with `barrier.Evaluate`.
- [ ] 1.2 Modify `internal/serve/model_test.go:595-627` to add unit tests for `ListBatchLanes` (status mapping, worktree path preservation, demotion note extraction, barrier outcomes) and verify `TestModelSourceDoesNotShellOut` passes.

## Phase 2: HTTP Endpoints & Route Wiring

- [ ] 2.1 Modify `internal/serve/handlers.go:36-118` to mount GET routes `/api/approvals`, `/api/features`, `/api/features/{id}/attempts`, `/api/leases`, `/api/overlap/{feature_id}`, `/api/reconcile/requests`, and `/api/batch/lanes` on `NewHandler` while retaining `/api/state` and anti-bulk POST rules.
- [ ] 2.2 Modify `internal/serve/server_test.go:42-93` to add HTTP tests for all new GET routes verifying 200 OK JSON responses, empty slice handling, error handling, and retention of anti-bulk 400 Bad Request behavior.

## Phase 3: Dashboard Layout & Panel Structure

- [ ] 3.1 Modify `internal/serve/static/index.html:141-162` to create DOM container sections for the five panels (Batch/Wave, Approvals, Feature/Lease, Reconciliation, Lane Envelope) while preserving existing metric element IDs (`approver-name`, `approver-rate`, `opencode-cmd`).

## Phase 4: Client Controller & Dynamic Rendering

- [ ] 4.1 Modify `internal/serve/static/app.js:1-98` to implement multi-panel fetching with tiered polling (2s interval for approvals, leases, batch lanes; on-demand expansion for overlap evidence and candidate checks), keyed DOM updates (`card-${runID}-${laneID}`), XSS sanitization via `escapeHtml`, and single-item approval POST submissions.

## Phase 5: Static Asset & UI Contract Verification

- [ ] 5.1 Modify `internal/serve/static_test.go:11-102` to add embed tests verifying multi-panel container IDs, absence of bulk approval controls, initial unselected card state, and evidence validation with `isValidEvidence` rejecting bare multiline prose.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Foundational data access layer; depends only on existing ledger schema and barrier packages. |
| 1.2 | 1.1 | Unit tests exercise `ListBatchLanes` and `BatchLane` types introduced in task 1.1. |
| 2.1 | 1.1 | HTTP handlers call `ListBatchLanes` and existing `Model` query methods. |
| 2.2 | 2.1 | HTTP tests require routes to be mounted in `NewHandler` before asserting status codes and JSON payloads. |
| 3.1 | — | Static HTML markup structure is independent of backend Go implementation and can proceed in parallel. |
| 4.1 | 2.1, 3.1 | Frontend script fetches backend endpoints added in 2.1 and binds dynamic content to DOM containers created in 3.1. |
| 5.1 | 3.1, 4.1 | Static asset embed tests assert contents and invariants of `index.html` and `app.js`. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Approvals Web UI: Individual Decisions Without Bulk Approval (`specs/approvals-web-ui/spec.md:5-37`) | 2.1, 2.2, 3.1, 4.1, 5.1 |
| Batch Wave View: Batch and DAG Wave Inspection (`specs/batch-wave-view/spec.md:5-26`) | 1.1, 1.2, 2.1, 2.2, 3.1, 4.1, 5.1 |
| Feature Lease Monitor: Shell-Free Feature and Lease Monitoring (`specs/feature-lease-monitor/spec.md:5-32`) | 1.2, 2.1, 2.2, 3.1, 4.1, 5.1 |
| Lane Envelope Inspector: Lane Demotion Diagnosis (`specs/lane-envelope-inspector/spec.md:5-33`) | 1.1, 1.2, 2.1, 2.2, 3.1, 4.1, 5.1 |
| Reconciliation Workspace: Reconciliation Candidate Inspection (`specs/reconciliation-workspace/spec.md:5-25`) | 2.1, 2.2, 3.1, 4.1, 5.1 |

## Open Questions

- [ ] Reconcile action transport: Should the reconciliation workspace UI display copy-paste CLI commands or prepare for future mutation POST routes (`design.md:133`)?
- [ ] Countdown computation: Should remaining lease duration be computed on the server (`remaining_seconds`) or calculated client-side from `expires_at` against server clock (`design.md:134`)?
- [ ] Overlap evidence payload rendering: Should `evidence_json` be rendered via `<pre>` with `escapeHtml` or use a dedicated client-side JSON tokenizer (`design.md:135`)?
- [ ] Handler dependency injection: Should `NewHandler` construct `*Model` internally via `NewModel(l)` or accept `*Model` as an explicit parameter (`design.md:136`)?
- [ ] Parallel lens partitioning: As specified in the packet, Work Unit breakdown is delegated to Lens B and Review Workload Forecast / RED tests are delegated to Lens C.
