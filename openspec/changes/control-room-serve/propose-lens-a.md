# Proposal Lens A — Candidate & Approach: Control Room Serve Subsystem

## Selected Candidate & Approach

**Chosen Candidate**: Candidate 1 — Granular REST API with Server-Sent Events (SSE) Stream.

**Core Approach**:
1. **Granular REST Read Endpoints**: Extend `internal/serve/handlers.go:36-118` with resource-oriented HTTP GET endpoints (`/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconciliations`, `/api/v1/approvals`). Wire them directly to existing query routines in `serve.Model` (`internal/serve/model.go:14-24,128-343`) and `ledger.Ledger` (`internal/ledger/ledger.go:285-358,705-717`), surfacing queries previously restricted to CLI subcommands (`cmd/lucind-ai/cli.go:852`).
2. **Server-Sent Events (SSE) Telemetry Stream**: Implement a `GET /api/v1/events/stream` handler using standard library `net/http` and `http.Flusher`. The handler tails new rows from `events` (`internal/ledger/schema.go:34-43`; `internal/ledger/ledger.go:490-525`) and `integration_events` (`internal/ledger/schema.go:171-180`), streaming `text/event-stream` payloads to browser `EventSource` clients while strictly tying stream goroutine lifecycles to request context cancellation via `r.Context().Done()`.
3. **Loopback Isolation & Operational Constraints**: Retain strict loopback network binding in `serve.ListenAndServe` and `serve.IsLoopback` (`internal/serve/server.go:14,19-22,55-73`; `cmd/lucind-ai/cli.go:683-694`), refuse execution in linked worktrees (`cmd/lucind-ai/cli.go:702-705`), maintain graceful 3-second shutdown (`internal/serve/server.go:41-52`), and avoid global write timeouts that would disrupt SSE (`internal/serve/server.go:24-28`).
4. **Anti-Bulk Mutation Preservation**: Preserve existing single-lane decision semantics on `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115,148-211`), strictly enforcing HTTP 400 Bad Request on JSON arrays, composite objects, or empty decisions (`internal/serve/handlers.go:161-183`; `internal/serve/server_test.go:42-93`).

**Why This Solves the Problem**:
- Eliminates the database overhead and polling latency of 2-second `/api/state` polling (`internal/serve/static/app.js:96-97`) against SQLite tables (`internal/ledger/schema.go:18-57`), avoiding WAL read lock contention with concurrent batch writers during `ExecuteBatch` (`internal/run/batch.go:29-53`; `internal/ledger/ledger.go:162-185,366-384`).
- Unlocks granular caching and real-time event delivery with zero new external dependencies in `go.mod`, aligning with the single-user subscription architecture (`docs/prd.md:53`).

## Conceptual Changes & Architecture Rationale

- **Decoupling Read Resources from Event Streaming**: Moves the daemon from a monolithic snapshot polling model (`ServerState` at `internal/serve/handlers.go:15-21`) to a clean separation of granular REST query endpoints and a continuous SSE push channel.
- **Cross-Process Event Streaming Seam**: `lucind-ai run` (`cmd/lucind-ai/cli.go:285`) and `lucind-ai serve` (`cmd/lucind-ai/cli.go:707`) execute as distinct OS processes independently opening the WAL SQLite database (`internal/ledger/ledger.go:162-185`). SSE streaming tails persisted SQLite event IDs (`internal/ledger/schema.go:34-43`) across processes rather than relying on in-process Go channels.
- **Handler Model Composition**: Updates `serve.NewHandler` (`internal/serve/handlers.go:36-38`) to accept `*serve.Model` (`internal/serve/model.go:21-24`) alongside `*ledger.Ledger`, cleanly separating HTTP routing from underlying SQL queries.
- **Client Polling Deprecation**: Replaces frontend polling (`internal/serve/static/app.js:96-97`) with native browser `EventSource` subscriptions and targeted REST fetches, while preserving backward compatibility for existing `/api/state` (`internal/serve/handlers.go:79-85`) and embedded static files (`internal/serve/static.go:8-18`).
- **Database Concurrency Isolation**: All HTTP read handlers execute brief, unnested SELECT queries within SQLite WAL concurrency bounds (`SetMaxOpenConns(4)` at `internal/ledger/ledger.go:182-184`; `busy_timeout=5000` at `internal/ledger/ledger.go:163`), ensuring long-lived SSE connections never hold open SQLite read transactions.

## Alternatives Considered & Rejected

- **Candidate 2 — Monolithic Hierarchical Snapshot Polling (Extended `/api/state`)**: Rejected because serializing growing execution history (`internal/ledger/schema.go:18-57`) every 1–2 seconds creates substantial SQLite read load, introduces unacceptable latency for live stdout/telemetry, and causes WAL lock contention against concurrent lane execution writes (`internal/run/batch.go:29-53`; `internal/ledger/ledger.go:366-384,448-486`).
- **Candidate 3 — Full-Duplex WebSocket Gateway (`/ws`)**: Rejected because WebSockets require introducing third-party dependencies to `go.mod`, violating the stdlib zero-dependency standard library pattern. Full-duplex connection framing, heartbeat protocols, and complex reconnect logic provide no architectural benefit over stdlib `http.Flusher` SSE combined with standard HTTP POST, and break simple observability with `curl`.
- **In-Memory Cache in `serve` Process**: Rejected because `lucind-ai run` and `lucind-ai serve` run as separate processes opening the ledger independently (`cmd/lucind-ai/cli.go:285,707`). An in-memory cache in `serve` would not observe database writes from `run` without polling SQLite anyway.
- **Multi-User Network Binding with Token Authentication**: Rejected because single-user localhost isolation is a fundamental design invariant (`docs/prd.md:53`; `internal/serve/server.go:14,20-22`). Adding token infrastructure introduces unnecessary CLI and setup friction for a local developer tool.

## Open Questions

- [ ] Cross-process event tailing: Should the SSE stream (`/api/v1/events/stream`) tail `events` (`internal/ledger/schema.go:34-43`) via ID cursor queries with adaptive backoff, or should SQLite commit hooks / OS-level IPC be introduced for lower event latency between `run` and `serve` processes?
- [ ] Mutation dispatch surface: Should future workflow dispatch actions (e.g., triggering reconciliation runs via `reconcile.Service.Approve` at `cmd/lucind-ai/cli.go:1166-1176`) be exposed over HTTP, gated behind a CLI flag (e.g., `--enable-dispatch`), or remain exclusively CLI-driven to preserve the minimal attack surface of unauthenticated loopback (`internal/serve/server.go:19-22`)?
- [ ] Development static asset serving: Should `lucind-ai serve` accept an optional `--dev-static-dir` flag in `cmd/lucind-ai/cli.go:675-689` to bypass `embed.FS` (`internal/serve/static.go:8-18`) during UI development without requiring binary recompilation?
- [ ] SDD propose fan-out topology precedence: As authorized by this packet, the three-lens parallel proposal fan-out and skeleton take precedence over `~/.claude/skills/sdd-propose/SKILL.md:92-158` monolithic single-agent proposal layout.
