# Tasks: Control Room Serve

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 520–780 (production ~260–370, tests ~260–410 across 6 files) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (ledger queries & tests) → PR 2 (HTTP REST/SSE, CLI wiring & tests) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: Medium

PR 1 base = feature/tracker branch. PR 2 base = PR 1 branch.

**Dispatch: single packet, no `apply-dag.yaml` sidecar.** Two sequential work-unit commits inside one packet. Unit 1 is additive (~85–115 production lines neighboring `Lanes` / `Events` / `IntegrationEvents`) and does not pay for `lucind-ai split` or per-wave bisection. `Integrate` (`internal/run/integrate.go:50-59`) runs `lucind-checks.sh` (`CGO_ENABLED=0 go build ./...` then `go test ./... -race -count=1`).

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary | Executor |
|------|------|-----------|----------------------|-----------------|-------------------|----------|
| 1 | Additive `Runs`, `EventsSince`, `IntegrationEventsSince` on `*ledger.Ledger` | PR 1 | `go test ./internal/ledger -race -count=1` | N/A: SQLite cursor tests; no `serve` process until Unit 2 | `internal/ledger/ledger.go`, `internal/ledger/ledger_test.go` — delete the three methods; no existing caller | `agy` |
| 2 | `NewHandler(*Model)`, `/api/v1/*` REST, SSE stream, JSON 404 under `/api/`, CLI wiring, tests | PR 2 | `go test ./internal/serve ./cmd/lucind-ai -race -count=1` | N/A: `httptest` + CLI `run()` cover the mux; long-running `lucind-ai serve` is not a bounded harness | `internal/serve/handlers.go`, `cmd/lucind-ai/cli.go`, `internal/serve/server_test.go`, `cmd/lucind-ai/cli_test.go` — restore 3-arg `NewHandler` and `/api/state`-only polling | `cursor-agent` |

`allowed_paths`: Unit 1 `internal/ledger/ledger.go`, `internal/ledger/ledger_test.go`. Unit 2 `internal/serve/handlers.go`, `cmd/lucind-ai/cli.go`, `internal/serve/server_test.go`, `cmd/lucind-ai/cli_test.go`. File-level paths; sequential waves so pairwise disjointness does not apply. Splitting Unit 2 handlers vs CLI is path-disjoint under `PathInScope` (`internal/packet/disjoint.go:8-22`) but fails `go build ./...` (breaking `NewHandler` at `handlers.go:36`).

Do not edit `internal/serve/server.go`, serve flags (`cli.go:683-685`), `schemaVersion` (`schema.go:10`), `GET /api/state`, or `POST /approvals/{runID}/{laneID}`. Do not hook `ExecuteBatch` (`internal/run/batch.go:29-53`).

## Phase 1: Ledger Read & Cursor Queries (Unit 1)

- [ ] 1.1 Add `func (l *Ledger) Runs(ctx context.Context) ([]string, error)` neighboring `Lanes` (`internal/ledger/ledger.go:285-330`) as `SELECT DISTINCT run_id FROM lanes` (`schema.go:18-32`). No DDL.
- [ ] 1.2 Add `EventsSince(ctx, lastID int64) ([]Event, error)` neighboring `Events` (`ledger.go:490-525`, `WHERE run_id = ?`) and `IntegrationEventsSince(ctx, lastID int64) ([]IntegrationEvent, error)` neighboring `IntegrationEvents` (`ledger.go:892-925`, `WHERE feature_id = ?`). Both `id > lastID` ordered by `id` ASC (`schema.go:34-43,171-180`).
- [ ] 1.3 Add `TestLedgerRuns`, `TestLedgerEventsSince`, `TestLedgerIntegrationEventsSince` in `internal/ledger/ledger_test.go` proving distinct `run_id`s and cursor filter/order. Same commit as 1.1–1.2 so the wave is green.

## Phase 2: Serve HTTP Routing & REST Endpoints (Unit 2)

- [ ] 2.1 Change `NewHandler` (`handlers.go:36`) to `NewHandler(l *ledger.Ledger, m *Model, defaultApprover, opencodeCmd string)` (`model.go:17-24`). Lockstep with 4.1 and 5.1.
- [ ] 2.2 Register `GET /api/v1/runs` via `l.Runs`, `GET /api/v1/lanes?run_id=` via `Lanes` (`ledger.go:285-330`; 400 if `run_id` missing), `GET /api/v1/approvals` via `PendingApprovals` (`ledger.go:705-717`). JSON 200; nil slices as `[]` (`handlers.go:126-128`).
- [ ] 2.3 Register `GET /api/v1/features` via `ListFeatures` (`model.go:128`), `GET /api/v1/features/{id}` via `GetFeature` (`model.go:152`; 404 JSON on `sql.ErrNoRows`), `GET /api/v1/reconciliations?feature_id=` via `ListReconciliationRequests` (`model.go:278`).
- [ ] 2.4 Unmatched `/api/*` and `/api/v1/*` return JSON 404. Today the `/` catch-all uses stdlib `http.NotFound` (`handlers.go:39-77`, line 53). Keep `embed.FS` MIME (`handlers.go:44-48`; `static.go:8-18`) and `GET /` → `index.html` (`handlers.go:68-77`).

## Phase 3: SSE Real-Time Event Stream (Unit 2)

- [ ] 3.1 Register `GET /api/v1/events/stream` on the `NewHandler` mux (`handlers.go:36-118`). `text/event-stream`, `http.Flusher`, poll 1.2, exit on `r.Context().Done()`. Do not attach to `/approvals/` (`handlers.go:87-115`). Cadence stays an open question (`design.md:135`).

## Phase 4: CLI Integration & Worktree Refusal (Unit 2)

- [ ] 4.1 In `serveDispatch` (`cli.go:715`) build `serve.NewModel(ledg)` (precedent `cli.go:852`) and pass it into `NewHandler`.
- [ ] 4.2 Add `TestServeLinkedWorktreeRefusal` in `cmd/lucind-ai/cli_test.go` proving exit 1 before `ledger.Open`. Production already refuses at `cli.go:702-707`; no test exists. Keep `TestServeNonLoopbackAddrRejectedAtCLI` (`cli_test.go:1908-1917`) and `TestNonLoopbackListenFails` (`server_test.go:17-40`).

## Phase 5: Verification & Concurrency Tests (Unit 2)

- [ ] 5.1 Pass `serve.NewModel(l)` at every `NewHandler` site (`server_test.go:70,114,155,215`).
- [ ] 5.2 `httptest` for `/api/v1/runs|lanes|features|reconciliations|approvals`: 200 JSON, `[]` when empty, lanes 400 without `run_id`, missing feature 404 JSON, unmatched `/api/*` JSON 404. Keep anti-bulk/409 (`server_test.go:42-236`).
- [ ] 5.3 `httptest` for `/api/v1/events/stream`: SSE content-type, frames from both event tables, cancel on request context done.
- [ ] 5.4 Concurrent REST+SSE reads during ledger writes (pool `ledger.go:162-185`) with no `SQLITE_BUSY`. Do not modify `ExecuteBatch`.

## Dependency Order

Unit 1 (1.1, 1.2, 1.3) then Unit 2 (2.1 with 4.1 and 5.1; then 2.2–2.4, 3.1, 4.2, 5.2–5.4). 1.1 and 1.2 are independent. 2.2 needs 1.1 and 2.1. 2.3 and 2.4 need 2.1. 3.1 needs 1.2 and 2.1. 4.2 has no code prerequisite. 5.2 needs 2.2–2.4 and 5.1. 5.3 needs 3.1 and 5.1. 5.4 needs 2.2, 3.1, and 5.1.

Phases 2–5 are one Integrate unit: changing `NewHandler` without `cli.go:715` and `server_test.go:70,114,155,215` fails `go build ./...`.

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Approval Queue Read Endpoint (`approvals-web-ui`) | 2.2, 5.2 |
| Embedded Static Assets and JSON API 404 Routing (`approvals-web-ui`) | 2.4, 5.2 |
| Loopback Binding (`approvals-web-ui`) | keep `server_test.go:17-40` and `cli_test.go:1908-1917`; new 4.2 for linked worktrees |
| Feature Listing and Inspection Endpoint (`control-room-api-features`) | 2.3, 5.2 |
| Reconciliation Candidates Endpoint (`control-room-api-reconcile`) | 2.3, 5.2 |
| Run Listing Endpoint (`control-room-api-runs`) | 1.1, 1.3, 2.2, 5.2 |
| Lane Listing by Run Endpoint (`control-room-api-runs`) | 2.2, 5.2 |
| Real-Time Event Streaming via SSE (`control-room-events-stream`) | 1.2, 1.3, 3.1, 5.3 |

## Threat Matrix

All five rows in `design.md:113-119` are `N/A`. No new RED-test tasks. Preserved boundaries stay the existing loopback, anti-bulk, and AST tests (`model_test.go:595-627` parses `model.go` only).

## Open Questions

- [ ] SSE cadence across `run`/`serve`: fixed ticker vs adaptive backoff on `id > lastID` (`design.md:135`).
- [ ] Mutations beyond `POST /approvals/{runID}/{laneID}` (e.g. `reconcile.Service.Approve`) deferred (`design.md:136`).
- [ ] Optional `--dev-static-dir` on serve (`cli.go:683-685`; `design.md:137`).
