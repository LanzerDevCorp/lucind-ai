# Proposal Lens C — Risks, Rollback & Test Impact: Control Room Serve Subsystem

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| **SQLite lock contention (`SQLITE_BUSY`) under concurrent batch runs** | High | REST reads and SSE event polling run short SELECT queries without transactions across HTTP requests or stream intervals. WAL mode, `busy_timeout=5000`, and `SetMaxOpenConns(4)` absorb concurrency spikes. | `internal/ledger/ledger.go:162-185`, `internal/run/batch.go:29-53`, `internal/serve/model.go:128-343` |
| **SSE goroutine and socket/FD leaks on disconnect** | High | Bind push loops to `r.Context().Done()`, immediately terminating the streaming loop and releasing network file descriptors on client disconnect. | `internal/serve/server.go:19-53`, `internal/serve/handlers.go:36-118` |
| **Stream termination via HTTP server write timeouts** | Medium | Maintain server policy setting only `ReadHeaderTimeout: 10 * time.Second` and omitting global `WriteTimeout` on `http.Server`, relying on client context cancellation and heartbeat flushes. | `internal/serve/server.go:24-28` |
| **DNS rebinding and unauthorized access to unauthenticated server** | High | Enforce loopback validation in `serve.ListenAndServe` with `serve.IsLoopback` (rejecting non-loopback binds with `serve.ErrNonLoopback`) and refuse linked worktrees before opening the ledger. | `internal/serve/server.go:14,19-22,55-73`, `cmd/lucind-ai/cli.go:683-705` |
| **Bulk approval / multi-item mutation bypass** | High | Require resource-specific URL paths (`/approvals/{runID}/{laneID}`) and reject JSON arrays or composite objects (`Approvals`, `Decisions`, `Lanes`) with HTTP 400 Bad Request. | `internal/serve/handlers.go:87-115,161-189`, `internal/serve/server_test.go:42-93` |
| **SPA fallback masking unmatched API endpoints** | Medium | Isolate `/api/*` routes so unmatched API requests return structured JSON HTTP 404 responses rather than falling back to `index.html`. | `internal/serve/handlers.go:39-77` |
| **Cross-process event visibility lag between `run` and `serve`** | Medium | Because `run` and `serve` are separate OS processes, the SSE stream queries SQLite event tables by primary key cursor (`id > lastID`) with adaptive backoff rather than using in-memory Go channels. | `cmd/lucind-ai/cli.go:285,707`, `internal/ledger/schema.go:34-43,171-180`, `internal/ledger/ledger.go:490-525,892-925` |
| **Server shutdown delay during active SSE streams** | Medium | `serve.ListenAndServe` calls `http.Server.Shutdown` with a 3-second context timeout, closing listeners and cancelling active stream contexts. | `internal/serve/server.go:41-52` |

## Rollback & Additivity

**Rollback Plan**: Reversal is accomplished via standard `git revert` of serve subsystem commits (`internal/serve/`, `cmd/lucind-ai/cli.go`). Zero database schema rollback or SQLite migration reversion is required: this change introduces zero DDL migrations, schema version increments, table alterations, or index modifications (`internal/ledger/schema.go:1-308`). Reverting the binary code cleanly restores the prior single-purpose approvals server with zero database residue.

**Additivity**: Formats, schemas, and endpoints change strictly additively:
- **HTTP Wire API**: Fully additive. Existing endpoints `GET /api/state` (`internal/serve/handlers.go:79-85`) and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115,148-211`) retain their exact signature and JSON schema. New REST endpoints (`/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconcile/requests`) and SSE stream (`/api/v1/events/stream`) are introduced on distinct additive paths (`internal/serve/handlers.go:36-118`).
- **Database Schema & Persisted Formats**: Zero modifications. The serve subsystem reads exclusively from existing SQLite tables (`internal/ledger/schema.go:18-180`). Schema DDL version remains at 5 (`internal/ledger/schema.go:10`).
- **Envelope & Result Schemas**: Zero modifications. Result envelope schema (`.lucind/result.schema.json:1-160`) and packet structures (`internal/packet/packet.go:29-47`) remain unchanged.

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| **Loopback & Security (Unit/Integration)** | Verify `serve.ListenAndServe` and `serve.IsLoopback` reject non-loopback addresses (`0.0.0.0`, public IPs, empty hosts) and accept loopback (`127.0.0.1`, `localhost`, `::1`); verify linked worktree refusal before ledger open. | `internal/serve/server_test.go:17-40`, `cmd/lucind-ai/cli_test.go:1908-1917`, `internal/serve/server.go:14-22,55-73`, `cmd/lucind-ai/cli.go:683-705` |
| **Anti-Bulk Mutation Validation (HTTP Unit)** | Verify `POST /approvals/{runID}/{laneID}` and new action endpoints reject JSON arrays, composite objects, empty decisions, and invalid IDs with HTTP 400; verify already-decided items return HTTP 409. | `internal/serve/server_test.go:42-135,196-236`, `internal/serve/handlers.go:148-211` |
| **Granular REST Query (HTTP Unit)** | Verify `GET /api/v1/runs`, `GET /api/v1/lanes`, `GET /api/v1/features`, `GET /api/v1/reconcile/requests` return HTTP 200 with JSON payloads matching ledger state and return JSON HTTP 404 for nonexistent resources. | `internal/serve/server_test.go:136-194`, `internal/serve/handlers.go:36-118`, `internal/serve/model.go:128-343` |
| **SSE Event Stream Lifecycle (Integration)** | Verify `GET /api/v1/events/stream` returns `text/event-stream`, frames `events` and `integration_events` rows, and terminates push loops upon `r.Context().Done()` without leaking goroutines or connections. | `internal/serve/server.go:19-53`, `internal/ledger/ledger.go:490-525,892-925` |
| **Concurrency & WAL Stress (Stress/Load)** | Concurrently run batch runs appending events while querying REST endpoints and streaming SSE; assert zero `SQLITE_BUSY` errors within WAL concurrency bounds (`SetMaxOpenConns(4)`). | `internal/ledger/ledger.go:162-185`, `internal/run/batch.go:29-53`, `internal/serve/model.go:128-343` |
| **Model Shell-Free Audit (Source AST)** | Verify `internal/serve/model.go` and handlers never import `os/exec` or invoke `git`, reading exclusively from ledger SQL. | `internal/serve/model_test.go:595-628`, `internal/serve/model.go:14-24` |
| **Static Asset Delivery & Route Fallback (HTTP Unit)** | Verify embedded assets (`index.html`, `app.js`) serve correct MIME types, UI routes fall back to `index.html`, and missing `/api/*` endpoints return JSON HTTP 404 instead of HTML. | `internal/serve/static_test.go:11-103`, `internal/serve/handlers.go:39-77` |

## Out of Scope

- PTY terminal process capture and streaming (owned by `control-room-capture`).
- SQLite schema migrations, DDL evolution, and database indexing (owned by `control-room-ledger`).
- Telemetry metrics, token accounting, and cost tracking (owned by `control-room-telemetry`).
- Frontend UI components, styling, CSS layout, and client rendering logic (owned by `control-room-ui-shell` and `control-room-ui-views`).
- Multi-user authentication, remote network binding, and token-based RBAC (`internal/serve/server.go:14,20-22`; `docs/prd.md:53`).
- Batch execution scheduling or DAG dispatch modifications (`internal/run/batch.go:29-53`).
- Complete proposal authoring (Candidate selection / approach owned by Lens A; capability delta specifications owned by Lens B).

## Open Questions

- [ ] Precedence note: As authorized by this packet, the three-lens parallel proposal fan-out and skeleton take precedence over `~/.claude/skills/sdd-propose/SKILL.md:92-158` monolithic single-agent proposal layout.
- [ ] SSE cross-process polling interval: Should the SSE stream (`/api/v1/events/stream`) query SQLite event tables with a fixed polling ticker (e.g., 250ms) or an adaptive backoff loop to balance event latency against database query load?
- [ ] HTTP mutation surface scope: Should workflow actions beyond approvals (such as reconciliation approval via `cmd/lucind-ai/cli.go:1166-1176`) be enabled over HTTP by default or gated behind an opt-in CLI flag?
- [ ] Static asset development reload: Should `lucind-ai serve` accept an optional `--dev-static-dir` flag (`cmd/lucind-ai/cli.go:675-689`) to bypass `embed.FS` (`internal/serve/static.go:8-18`) during UI development?
