# Explore Lens C — Risks, Trade-offs & Spikes: Control Room Serve

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| **SQLite WAL Lock Contention on Concurrent Reads**<br>High-frequency HTTP reads via `serve.Model` during active batch execution risk `SQLITE_BUSY` errors despite `busy_timeout=5000`. | High | Use statement-level short read transactions; configure connection pool (`db.SetMaxOpenConns(4)`); avoid open transactions across HTTP requests. | `internal/ledger/ledger.go:162-185`, `internal/serve/model.go:128-148`, `internal/run/batch.go:29-53` |
| **Real-Time Stream Connection & Goroutine Leaks**<br>Unmonitored SSE disconnects or unbounded channels risk leaking goroutines and file descriptors. | High | Bind SSE push loops directly to `r.Context().Done()`; implement 15s heartbeat frames; test client disconnect lifecycle via `net/http/httptest`. | `internal/serve/server.go:19-53`, `internal/serve/handlers.go:36-118`, `internal/ledger/schema.go:34-43` |
| **DNS Rebinding & Cross-Origin Localhost Mutation**<br>Non-loopback origins in local browsers could exploit DNS rebinding to POST to mutating endpoints. | High | Enforce Host header middleware (`localhost`/`127.0.0.1`/`[::1]`); require custom headers (e.g. `X-Lucind-Control-Room`) on mutating endpoints. | `internal/serve/server.go:20-22`, `internal/serve/server.go:55-73`, `internal/serve/handlers.go:87-115` |
| **HTTP Server Write Timeout Severing Streams**<br>Adding global `WriteTimeout` to prevent slowloris attacks would prematurely terminate long-lived SSE streams. | Medium | Use `http.ResponseController` per handler; keep global `WriteTimeout` disabled while enforcing `ReadHeaderTimeout` and `IdleTimeout`. | `internal/serve/server.go:24-28` |
| **Accidental Multi-Target Mutation Exposure**<br>New control endpoints could accidentally permit bulk mutations, violating single-item approval rules. | High | Require individual resource IDs in URL paths (`/approvals/{runID}/{laneID}`); reject JSON array payloads with HTTP 400. | `internal/serve/handlers.go:148-176`, `internal/serve/handlers.go:195-206` |
| **SPA PushState Routing Colliding with API 404s**<br>Client-side HTML5 history routing fallback could mask missing `/api/*` endpoints with `index.html` 200 OK. | Medium | Enforce strict prefix routing in `http.ServeMux`: return JSON 404 under `/api/*`; route non-API GET requests to `static/index.html`. | `internal/serve/handlers.go:39-77`, `internal/serve/static.go:8-18` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| **Real-time: Server-Sent Events (SSE)** | Unidirectional browser streaming (`EventSource`), zero external dependencies, stdlib `http.Flusher` compatible, automatic reconnect. | Unidirectional (commands need POST), 6-connection HTTP/1.1 limit per domain. | Low; minimal code footprint, low idle CPU. |
| **Real-time: Short Polling (Status Quo)** | Simple implementation (`setInterval`), completely stateless, no persistent goroutines. | 2s query churn on SQLite, high log latency, wasted idle CPU. | High SQLite I/O churn; degrades CLI batch throughput. |
| **Real-time: WebSockets** | Full duplex bidirectional communication over single socket. | Requires third-party library (`gorilla/websocket`), complex framing and handshake lifecycle. | High; violates zero-dependency rule, complex recovery. |
| **State Querying: Direct SQLite Queries** | Single source of truth in SQLite ledger, zero cross-process sync issues between `run` and `serve`. | Read lock contention during heavy dashboard refresh bursts. | Low complexity; leverages SQLite WAL mode (`internal/ledger/ledger.go:162-185`). |
| **State Querying: In-Memory Cache** | Sub-millisecond read latency, zero database I/O for polling. | Cannot synchronize across independent `run` and `serve` CLI processes without IPC. | Very high; risk of state divergence. |
| **Asset Delivery: Go `embed.FS` (Embedded)** | Single self-contained binary, no runtime path dependencies, immutable assets. | Requires Go binary rebuild to test UI changes. | Zero production deployment overhead. |
| **Asset Delivery: External Directory** | Live reloading of frontend assets during development. | Binary requires external folder on disk; path traversal risk. | Medium; breaks standalone binary model. |
| **Security: Loopback + Host Validation** | Zero-friction local UX (no tokens/passwords), stops DNS rebinding. | Relies on browser origin sandbox on localhost. | Very low; pure middleware check. |
| **Security: Session Token Auth** | Defense-in-depth against malicious local processes. | CLI workflow friction; requires token management and flag plumbing. | Medium; extra session state. |

## Potential Spikes / Proof of Concepts

- **Spike 1: SSE Event Streamer with `http.Flusher` & Disconnect Handling**
  - **Objective**: Verify `/api/events` broadcasts `ledger.Event` notifications (`internal/ledger/schema.go:34-43`) via `http.Flusher` without goroutine leaks on client disconnect (`internal/serve/server.go:19-53`, `internal/serve/handlers.go:36-118`).
  - **Seam**: `internal/serve/server.go:19-53`, `internal/serve/handlers.go:36-118`, `internal/ledger/schema.go:34-43`.

- **Spike 2: SQLite WAL Concurrency Under Multi-Process Read/Write Load**
  - **Objective**: Measure latency and verify zero `SQLITE_BUSY` errors when `serve.Model` (`internal/serve/model.go:128-344`) reads while `run.ExecuteBatch` (`internal/run/batch.go:29-53`) and `ledger.Open` (`internal/ledger/ledger.go:146-192`) commit concurrent batch updates.
  - **Seam**: `internal/ledger/ledger.go:146-214`, `internal/serve/model.go:128-250`, `internal/run/batch.go:29-53`.

- **Spike 3: SPA History Routing Fallback in `net/http.ServeMux`**
  - **Objective**: Prototype handler routing static embedded files (`internal/serve/static.go:8-18`), falling back to `index.html` for UI routes, and returning JSON 404 for missing `/api/*` endpoints (`internal/serve/handlers.go:39-77`).
  - **Seam**: `internal/serve/handlers.go:39-77`, `internal/serve/static.go:8-18`.

## Out of Scope

- **Problem space definition, candidate approaches, initial recommendations**: Owned by Lens A.
- **Capabilities, user scenarios, success criteria**: Owned by Lens B.
- **Process execution capture & pseudo-terminal multiplexing**: Owned by `control-room-capture`.
- **Database schema DDL, SQL queries, and table migrations**: Owned by `control-room-ledger`.
- **Token usage, cost accounting, subscription telemetry**: Owned by `control-room-telemetry`.
- **UI layout, CSS styling, SVG rendering, component views**: Owned by `control-room-ui-shell` / `control-room-ui-views`.
- **Multi-user authentication, RBAC, remote network exposure**: Strictly excluded; server remains a single-user localhost tool (`internal/serve/server.go:20-22`, `docs/prd.md:48-58`).

## Open Questions

- [ ] How should the HTTP server receive real-time event notifications from independent `lucind-ai run` processes: via SQLite WAL polling, an OS IPC socket, or SQLite data-change hooks?
- [ ] Should `lucind-ai serve` support an optional `--dev-static-dir <path>` flag to bypass `embed.FS` during frontend UI development?
