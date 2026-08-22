# Design Lens A — Decisions: Control Room Serve

## Assumed architecture

This change extends existing packages `internal/serve` and `internal/ledger` without adding new Go packages or bumping SQLite `schemaVersion` beyond 5 (`internal/ledger/schema.go:10`). `internal/serve/handlers.go` is extended with granular `/api/v1/*` read handlers and an SSE event stream, receiving `*serve.Model` (`internal/serve/model.go:17-24`) alongside `*ledger.Ledger` (`internal/ledger/ledger.go:191-192`). `internal/serve/server.go` maintains loopback binding with no global write timeouts (`internal/serve/server.go:14-28`), while `cmd/lucind-ai/cli.go` (`cmd/lucind-ai/cli.go:674-725`) wires the extended handler and preserves linked-worktree refusal.

## Technical Approach

The design replaces monolithic polling (`GET /api/state` at `internal/serve/handlers.go:79-85`) with targeted REST reads under `/api/v1/` and a real-time Server-Sent Events stream (`GET /api/v1/events/stream`), while preserving existing approval mutations (`POST /approvals/{runID}/{laneID}`).

1. **REST Reads Layer** (`Granular REST reads`, capabilities `control-room-api-runs`, `control-room-api-features`, `control-room-api-reconcile`):
   - Features, attempts, leases, overlap evidence, and reconciliation queries delegate directly to existing `serve.Model` methods (`internal/serve/model.go:128-343`).
   - Run listing derives distinct runs from `lanes` and `events` tables (`internal/ledger/schema.go:18-43`), while lane inspection calls `ledger.Lanes(ctx, runID)` (`internal/ledger/ledger.go:285-330`).
   - Approvals querying reuses `ledger.PendingApprovals` (`internal/ledger/ledger.go:705-717`).

2. **Event Push Layer** (`SSE stream`, capability `control-room-events-stream`):
   - Streams ledger mutations from `events` (`internal/ledger/schema.go:34-43`) and `integration_events` (`internal/ledger/schema.go:171-180`) using standard library `http.Flusher`.
   - Cursor tailing (`id > lastID`) polls SQLite across independent processes (`run` and `serve` at `cmd/lucind-ai/cli.go:285,707`), exiting cleanly on `r.Context().Done()`.

3. **Routing and Security Boundary** (`Loopback and worktree guard`, `Individual decisions`, `Embedded assets and API 404s`):
   - Enforces loopback binding via `serve.IsLoopback` (`internal/serve/server.go:55-73`) and linked-worktree refusal before `ledger.Open` (`cmd/lucind-ai/cli.go:702-707`).
   - Rejects bulk mutation arrays or composite payloads with HTTP 400 in `handleDecide` (`internal/serve/handlers.go:148-211`).
   - Routes `/api/*` requests to JSON handlers with dedicated JSON 404s while serving embedded assets from `embed.FS` (`internal/serve/static.go:8-18`).

## Decision 1 — Server-Sent Events with SQLite Cursor Tailing

**Choice**: Use Server-Sent Events (`text/event-stream` via standard library `http.Flusher`) backed by an autoincrement integer cursor (`id > lastID`) querying `events` (`internal/ledger/schema.go:34-43`) and `integration_events` (`internal/ledger/schema.go:171-180`), bound to `r.Context().Done()`.
**Alternatives considered**: WebSockets (rejected: adds third-party dependencies to minimal `go.mod` with no duplex requirement); In-process Go channels (rejected: `run` and `serve` are separate OS processes sharing only SQLite at `cmd/lucind-ai/cli.go:285,707`); Full-state polling via `/api/state` (rejected: causes WAL contention and read amplification with concurrent `ExecuteBatch` writers at `internal/run/batch.go:29-53`).
**Rationale**: SSE uses Go stdlib `net/http`, integrates with browser `EventSource`, and an `id > lastID` query leverages indexed primary keys (`internal/ledger/schema.go:35,172`) with short read transactions that do not block concurrent writers.
**Terminal consumer**: `GET /api/v1/events/stream` route registered in `NewHandler` (`internal/serve/handlers.go:36-38`) and served via `serve.ListenAndServe` (`internal/serve/server.go:19-28`).

## Decision 2 — Composition of `serve.Model` and `ledger.Ledger` in HTTP Handlers

**Choice**: Pass `*ledger.Ledger` (`internal/ledger/ledger.go:191-192`) and `*serve.Model` (`internal/serve/model.go:17-24`) to `NewHandler`, reusing shell-free query methods on `serve.Model` for features and reconciliations.
**Alternatives considered**: Introducing a new unified query service layer (rejected: `serve.Model` already implements shell-free status and audit queries at `internal/serve/model.go:14-24`); Executing raw SQL directly inside HTTP handlers (rejected: violates encapsulation and bypasses existing unit test coverage in `internal/serve/model_test.go:1-628`).
**Rationale**: `serve.Model` was built as the shell-free query surface for feature and reconciliation state (`internal/serve/model.go:14-19`). Passing it directly to `NewHandler` avoids code duplication and keeps handlers focused on HTTP transport.
**Terminal consumer**: `NewHandler` constructor signature in `internal/serve/handlers.go:36-38` invoked by `serveDispatch` in `cmd/lucind-ai/cli.go:715`.

## Decision 3 — Deriving Run Listings without Schema Migrations

**Choice**: Query distinct runs and metadata from existing `lanes` (`internal/ledger/schema.go:18-32`) and `events` (`internal/ledger/schema.go:34-43`) tables, keeping SQLite `schemaVersion` frozen at 5 (`internal/ledger/schema.go:10`).
**Alternatives considered**: Schema migration v6 creating a dedicated `runs` table (rejected: violates constraint freezing DDL migrations and risks backward compatibility); Omitting run listing and requiring client-supplied `runID` (rejected: fails capability `control-room-api-runs` and prevents dashboard run discovery).
**Rationale**: The `lanes` table indexes `(run_id, lane_id)` (`internal/ledger/schema.go:31`) and `events` indexes `(run_id, id)` (`internal/ledger/schema.go:43`). Querying `SELECT DISTINCT run_id FROM lanes` provides run discovery on SQLite WAL without DDL alterations.
**Terminal consumer**: `/api/v1/runs` handler registered in `internal/serve/handlers.go:36-85`.

## Decision 4 — Strict Routing Separation and JSON 404 Isolation

**Choice**: Segment `http.ServeMux` routes so that `/api/` prefix routes (both `/api/state` and `/api/v1/*`) emit JSON responses with dedicated JSON 404 handlers, while non-API paths serve embedded files from `embed.FS` (`internal/serve/static.go:8-18`).
**Alternatives considered**: Catch-all root handler `/` with path inspections (rejected: pattern in `internal/serve/handlers.go:39-77` conflates static asset lookup with root GET and risks returning `index.html` or plain text 404 for malformed API routes); External HTTP router framework (rejected: violates minimal dependency policy).
**Rationale**: Isolating API endpoints prevents HTML fallback leakage to API clients, guarantees HTTP 404 JSON errors for missing API endpoints, and preserves static asset MIME mapping (`internal/serve/handlers.go:43-48`).
**Terminal consumer**: Mux dispatch and route registration in `NewHandler` (`internal/serve/handlers.go:36-118`).

## Decision 5 — Preserving Path-Bound Mutation and Anti-Bulk Gatekeeping

**Choice**: Retain path-parameterized approval mutations (`POST /approvals/{runID}/{laneID}` and `POST /approvals/{runID}/{laneID}/defect`) in `internal/serve/handlers.go:87-115`, enforcing anti-bulk JSON array and composite object rejection with HTTP 400 (`internal/serve/handlers.go:161-176`).
**Alternatives considered**: Introducing bulk approval endpoints `POST /api/v1/approvals/bulk` (rejected: violates operator oversight rules requiring individual manual inspection per `internal/serve/handlers.go:161-165`); Moving `runID`/`laneID` from URL path to request body (rejected: breaks existing UI contract and test suite at `internal/serve/server_test.go:42-93`).
**Rationale**: Path-based mutation guarantees explicit targeting of single approval records, and immediate 400 rejection of array or composite JSON prevents mass automated approval accidents.
**Terminal consumer**: `handleDecide` (`internal/serve/handlers.go:148-211`) and `handleDefect` (`internal/serve/handlers.go:213-231`).

## Open Questions

- [ ] Whether cross-process SSE event tailing between `run` and `serve` should rely on a fixed polling ticker (e.g. 500ms) with SQLite `id > lastID` queries or an adaptive backoff when idle.
- [ ] Whether a development CLI flag (`--dev-static-dir` in `cmd/lucind-ai/cli.go:676-690`) should be admitted in a future change to allow live UI development without re-embedding assets.
