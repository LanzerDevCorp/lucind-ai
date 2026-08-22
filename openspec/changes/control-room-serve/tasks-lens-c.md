# Tasks Lens C — Proof & Review Burden: Control Room Serve

## Assumed decomposition

The work breaks into three core units: (1) Ledger read additions (`Runs`, `EventsSince`, `IntegrationEventsSince` in `internal/ledger/ledger.go`), (2) REST endpoints (`/api/v1/runs|lanes|features|reconciliations|approvals`), SSE stream (`/api/v1/events/stream`), and JSON 404 routing in `internal/serve/handlers.go`, and (3) CLI wiring (`serveDispatch` taking `*Model`) and linked-worktree refusal in `cmd/lucind-ai/cli.go`. Critical path runs sequentially from Ledger query primitives to HTTP handler routing and SSE flusher integration, concluding with CLI dispatch wiring and concurrency verification.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 520–780 lines (production: ~260–370, tests: ~260–410 across 6 files) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Ledger Queries & Tests) → PR 2 (HTTP REST/SSE Handlers, CLI Wiring & Tests) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: Medium

Basis for estimate:
- `internal/ledger/ledger.go`: ~85–115 lines (3 additive query methods `Runs`, `EventsSince`, `IntegrationEventsSince`, based on comparable `Lanes` at `internal/ledger/ledger.go:285` [45 lines] and `Events` at `internal/ledger/ledger.go:490` [35 lines]).
- `internal/serve/handlers.go`: ~170–245 lines (`NewHandler(*Model)` struct update, 6 REST handlers, JSON 404 mux handler, SSE streaming loop with `http.Flusher`).
- `cmd/lucind-ai/cli.go`: ~5–10 lines (`serveDispatch` instantiates `serve.NewModel(ledg)` and passes to `NewHandler`).
- `internal/ledger/ledger_test.go`: ~80–120 lines (query tests for run listing and cursor-based event queries, based on `internal/ledger/ledger_test.go:432`).
- `internal/serve/server_test.go`: ~150–250 lines (tests for `/api/v1/*` endpoints, JSON 404, query validation, SSE frame flushing and context cancellation, based on `internal/serve/server_test.go:42`).
- `cmd/lucind-ai/cli_test.go`: ~25–35 lines (test for linked-worktree refusal before ledger open, based on `cmd/lucind-ai/cli_test.go:1908`).

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A: HTTP + embed.FS only; no file execution | — | — | — |
| Git repository selection | N/A: SQLite reads; linked worktrees refused; internal/serve does not invoke git | — | — | — |
| Commit state | N/A: no commit/index | — | — | — |
| Push state | N/A: no remotes | — | — | — |
| PR commands | N/A: no PR argv | — | — | — |

*Note: All 5 threat matrix rows in `design.md:113-119` are marked `N/A` (no file execution, git execution, commit, or PR logic). Preserved boundary tests (loopback, anti-bulk, linked-worktree refusal, JSON 404) are tracked under Acceptance Evidence.*

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| 1. Ledger query additions (`Runs`, `EventsSince`, `IntegrationEventsSince`) | `go test -v -run 'TestLedgerRuns|TestLedgerEventsSince|TestLedgerIntegrationEventsSince' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:432,490`) | `Runs` returns distinct `run_id`s from `lanes`; `EventsSince` and `IntegrationEventsSince` query rows with `id > lastID` ordered by `id` ASC | Does not prove HTTP handler encoding or SSE streaming |
| 2. REST read endpoints & JSON 404 (`/api/v1/*`) | `go test -v -run 'TestV1RESTEndpoints|TestUnmatchedAPIJSON404' ./internal/serve` (derived from `internal/serve/server_test.go:42,136`) | All `/api/v1/` endpoints return 200 JSON payloads (empty slices as `[]`), missing feature returns 404 JSON, missing `run_id` on `/lanes` returns 400, and unmatched `/api/*` returns JSON 404 | Does not prove SSE stream flush behavior or real browser UI rendering |
| 3. SSE event stream (`GET /api/v1/events/stream`) | `go test -v -run 'TestEventsStreamSSE' ./internal/serve` (derived from `internal/serve/server_test.go:17,42`) | SSE endpoint sets `text/event-stream`, flushes frames from `events` and `integration_events`, and terminates push loop on request context cancellation without leaked goroutines | Does not prove browser EventSource reconnection semantics across network drops |
| 4. CLI wiring & linked-worktree refusal | `go test -v -run 'TestServeLinkedWorktreeRefusal|TestServeNonLoopbackAddrRejectedAtCLI' ./cmd/lucind-ai` (derived from `cmd/lucind-ai/cli_test.go:1908,1919`) | `serveDispatch` initializes `serve.NewModel`, passes to `NewHandler`, and exits 1 before opening ledger when run from linked worktrees | Does not prove interactive CLI terminal lifecycle or signal interrupt handling |
| 5. WAL non-blocking concurrency | `go test -v -race -run 'TestConcurrentServeReadsAndBatchWrite' ./internal/serve` (derived from `internal/ledger/ledger_test.go:367` and `internal/serve/server_test.go:42`) | Concurrent REST reads and SSE queries against SQLite ledger proceed without `SQLITE_BUSY` errors during active batch writes | Does not prove OS-level cross-process locking between separated binary instances |

## Verification Gaps

- **Cross-process WAL isolation**: In-process concurrent tests (`-race`) prove SQLite WAL connection pool concurrency, but true cross-process concurrency between separated `lucind-ai run` and `lucind-ai serve` processes relies on SQLite WAL locking and `busy_timeout=5000` (`internal/ledger/ledger.go:162-185`).
- **Browser EventSource consumption**: Go `httptest` verifies SSE protocol compliance (`http.Flusher`, MIME type, frame format), but browser DOM event handling is deferred to the downstream UI views capability (`control-room-ui-views`).

## Open Questions

- [ ] SSE polling interval across `run`/`serve`: fixed ticker vs adaptive backoff on `id > lastID` (`design.md:135`).
- [ ] Future mutation endpoints beyond `POST /approvals/{runID}/{laneID}` (e.g., reconcile service approvals) remain deferred (`design.md:136`).
