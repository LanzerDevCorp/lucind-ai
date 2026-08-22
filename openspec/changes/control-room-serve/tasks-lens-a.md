# Tasks Lens A — Decomposition & Ordering: Control Room Serve

## Assumed decomposition

The change decomposes into five sequential phases across `internal/ledger/`, `internal/serve/`, and `cmd/lucind-ai/`: (1) additive SQLite ledger cursor and run queries (`Runs`, `EventsSince`, `IntegrationEventsSince`), (2) serve HTTP routing and REST endpoints (`/api/v1/runs|lanes|features|reconciliations|approvals` and JSON 404 under `/api/`), (3) real-time SSE event streaming (`/api/v1/events/stream`), (4) CLI dispatch wiring in `serveDispatch` and linked-worktree refusal, and (5) test suite updates and WAL concurrency verification. The critical path is strictly sequential: HTTP handlers and SSE streaming require ledger query primitives to compile and run, while the breaking `NewHandler(*Model)` signature change couples `internal/serve/handlers.go`, `cmd/lucind-ai/cli.go`, and `internal/serve/server_test.go`.

## Phase 1: Ledger Read & Cursor Queries

- [ ] 1.1 Add `Ledger.Runs(ctx context.Context) ([]string, error)` in `internal/ledger/ledger.go:285` querying distinct `run_id` values from the `lanes` table.
- [ ] 1.2 Add `Ledger.EventsSince(ctx context.Context, lastID int64) ([]Event, error)` and `Ledger.IntegrationEventsSince(ctx context.Context, lastID int64) ([]IntegrationEvent, error)` in `internal/ledger/ledger.go:490,892` querying rows where `id > lastID` ordered by `id` ascending.
- [ ] 1.3 Add unit tests in `internal/ledger/ledger_test.go:432` verifying `Runs`, `EventsSince`, and `IntegrationEventsSince` cursor filter semantics against SQLite.

## Phase 2: Serve HTTP Routing & REST Endpoints

- [ ] 2.1 Update `NewHandler` signature in `internal/serve/handlers.go:36` to accept `m *Model` (`internal/serve/model.go:21`).
- [ ] 2.2 Add `GET /api/v1/runs`, `GET /api/v1/lanes`, and `GET /api/v1/approvals` route handlers in `internal/serve/handlers.go:36-118` returning JSON arrays and returning 400 for missing `run_id` on `/lanes`.
- [ ] 2.3 Add `GET /api/v1/features`, `GET /api/v1/features/{id}`, and `GET /api/v1/reconciliations` route handlers backed by `serve.Model` in `internal/serve/handlers.go:36-118` returning JSON and 404 for missing feature IDs.
- [ ] 2.4 Update mux routing in `internal/serve/handlers.go:39-77` to return JSON 404 for unmatched `/api/*` and `/api/v1/*` routes while preserving `embed.FS` MIME mappings for static assets.

## Phase 3: SSE Real-Time Event Stream

- [ ] 3.1 Add `GET /api/v1/events/stream` handler in `internal/serve/handlers.go:87` using `http.Flusher` and `text/event-stream` to poll `EventsSince` and `IntegrationEventsSince`, terminating push loop on `r.Context().Done()`.

## Phase 4: CLI Integration & Worktree Refusal

- [ ] 4.1 Update `serveDispatch` in `cmd/lucind-ai/cli.go:715` to instantiate `serve.NewModel(ledg)` and pass `model` into `serve.NewHandler`.
- [ ] 4.2 Add test `TestServeLinkedWorktreeRefusal` in `cmd/lucind-ai/cli_test.go:1908` verifying `lucind-ai serve` exits with status code 1 before ledger open when executed in a linked worktree.

## Phase 5: Verification & Concurrency Tests

- [ ] 5.1 Update existing `NewHandler` call sites in `internal/serve/server_test.go:70,114,155,215` to pass `serve.NewModel(l)`.
- [ ] 5.2 Add tests in `internal/serve/server_test.go:42` covering `/api/v1/*` REST endpoints (`runs`, `lanes`, `features`, `reconciliations`, `approvals`), empty array fallbacks (`[]`), query validation, and unmatched `/api/*` JSON 404.
- [ ] 5.3 Add tests in `internal/serve/server_test.go:42` for `GET /api/v1/events/stream` verifying SSE header setup, event frame flushing from both tables, and push loop termination on context cancellation.
- [ ] 5.4 Add concurrency test in `internal/serve/server_test.go:42` verifying non-blocking SQLite WAL reads during active batch execution without `SQLITE_BUSY`.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Standalone additive query on `*ledger.Ledger` reading `lanes` table. |
| 1.2 | — | Standalone additive cursor queries on `*ledger.Ledger` reading `events` and `integration_events`. |
| 1.3 | 1.1, 1.2 | Cannot test `Runs`, `EventsSince`, and `IntegrationEventsSince` until query methods are declared. |
| 2.1 | — | Extends `NewHandler` parameter list to accept existing `*Model` type (`internal/serve/model.go:18-24`). |
| 2.2 | 1.1, 2.1 | Handler methods call `l.Runs` (1.1) and register on the mux instantiated in `NewHandler` (2.1). |
| 2.3 | 2.1 | Handler methods invoke `m.ListFeatures`, `m.GetFeature`, and `m.ListReconciliationRequests` on `*Model` (2.1). |
| 2.4 | 2.1 | Modifies mux routing logic inside `NewHandler` (2.1) to isolate `/api/` error responses. |
| 3.1 | 1.2, 2.1 | Stream loop calls `l.EventsSince` and `l.IntegrationEventsSince` (1.2) and registers on `NewHandler` mux (2.1). |
| 4.1 | 2.1 | `serveDispatch` cannot compile against the updated `NewHandler` signature without passing `*Model`. |
| 4.2 | — | CLI linked-worktree refusal logic already exists at `cmd/lucind-ai/cli.go:702-707`; test has no code prerequisites. |
| 5.1 | 2.1 | Existing `server_test.go` tests cannot compile until `NewHandler` call sites pass `*Model`. |
| 5.2 | 2.2, 2.3, 2.4, 5.1 | REST and JSON 404 integration tests require route handlers and compiled test suite. |
| 5.3 | 3.1, 5.1 | SSE integration tests require `GET /api/v1/events/stream` handler and compiled test suite. |
| 5.4 | 2.2, 3.1, 5.1 | Concurrency verification requires REST/SSE endpoints active against concurrent ledger writes. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Approval Queue Read Endpoint (`approvals-web-ui`) | 2.2, 5.2 |
| Embedded Static Assets and JSON API 404 Routing (`approvals-web-ui`) | 2.4, 5.2 |
| Loopback Binding (`approvals-web-ui`) | 4.2 |
| Feature Listing and Inspection Endpoint (`control-room-api-features`) | 2.3, 5.2 |
| Reconciliation Candidates Endpoint (`control-room-api-reconcile`) | 2.3, 5.2 |
| Run Listing Endpoint (`control-room-api-runs`) | 1.1, 1.3, 2.2, 5.2 |
| Lane Listing by Run Endpoint (`control-room-api-runs`) | 2.2, 5.2 |
| Real-Time Event Streaming via SSE (`control-room-events-stream`) | 1.2, 1.3, 3.1, 5.3 |

## Open Questions

- [ ] SSE polling interval across `run` and `serve` processes: fixed ticker interval vs adaptive backoff on `id > lastID` cursor queries (`design.md:135`).
- [ ] Mutation endpoints beyond `POST /approvals/{runID}/{laneID}` (e.g., reconcile service approvals) remain deferred until authentication / authorization model is defined (`design.md:136,140`).
- [ ] Optional `--dev-static-dir` flag on `serve` subcommand (`cmd/lucind-ai/cli.go:683-685`) to allow UI development without rebuilds (`design.md:137`).
