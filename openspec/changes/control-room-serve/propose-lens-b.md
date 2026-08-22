# Proposal Lens B — Capability Impact & Specs: Control Room Serve

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `control-room-api-state` | Added | Granular JSON system state (`/api/v1/state`), runs (`/api/v1/runs`), and lane statuses (`/api/v1/runs/{runID}/lanes`) via ledger queries. | `internal/serve/handlers.go:16-21,120-146`, `internal/ledger/ledger.go:285-330`, `internal/ledger/schema.go:18-32` |
| `control-room-api-features` | Added | HTTP REST read endpoints for features (`/api/v1/features`, `/api/v1/features/{id}`), attempts, leases, and overlap evidence via `serve.Model`. | `internal/serve/model.go:14-25,128-266`, `cmd/lucind-ai/cli.go:852-895`, `internal/ledger/schema.go:96-139` |
| `control-room-api-reconcile` | Added | HTTP REST read endpoints for reconciliation requests and candidate details (`/api/v1/reconcile/requests`, `/api/v1/reconcile/requests/{id}`) via `serve.Model`. | `internal/serve/model.go:72-115,278-323`, `internal/ledger/schema.go:141-169` |
| `control-room-events-stream` | Added | Real-time Server-Sent Events (SSE) stream (`/api/v1/events/stream`) using `http.Flusher` to push live run events and integration events. | `internal/ledger/schema.go:34-43,171-180`, `internal/ledger/ledger.go:490-525`, `internal/serve/server.go:19-53` |
| `control-room-reconcile-dispatch` | Added | Single-resource HTTP action dispatch for approving reconciliation requests (`POST /api/v1/reconcile/requests/{id}/approve`) delegating to reconciliation service. | `cmd/lucind-ai/cli.go:1160-1180`, `internal/ledger/schema.go:141-154` |
| `approvals-web-ui` | Modified | Expand from single-purpose approval interface to host Control Room shell and REST read endpoints while preserving loopback binding and anti-bulk decision invariants. | `internal/serve/handlers.go:36-118,148-189`, `internal/serve/server.go:14-22,55-73`, `cmd/lucind-ai/cli.go:683-705` |

## Delta Specifications

### Requirement: Granular REST State and Model Exposure

The server MUST expose read-only JSON endpoints under `/api/v1/` for system state (`/api/v1/state`), runs (`/api/v1/runs`), lane details (`/api/v1/runs/{runID}/lanes`), features (`/api/v1/features`, `/api/v1/features/{id}`), and reconciliation requests (`/api/v1/reconcile/requests`), backed by `serve.Model` (`internal/serve/model.go:128-343`) and `ledger.Ledger` (`internal/ledger/ledger.go:285-330`). Read responses MUST return HTTP 200 with `Content-Type: application/json`.

#### Scenario: Query feature list via REST

- GIVEN registered features in the ledger database (`internal/ledger/schema.go:96-104`)
- WHEN a client sends `GET /api/v1/features` with `Accept: application/json` (`internal/serve/handlers.go:63-66`)
- THEN the server MUST return HTTP 200 containing a JSON array of `serve.Feature` records (`internal/serve/model.go:27-35,128-149`).

#### Scenario: Query lane status collection for a run

- GIVEN an active or completed run with registered lanes in the ledger (`internal/ledger/schema.go:18-32`)
- WHEN a client sends `GET /api/v1/runs/{runID}/lanes` (`internal/ledger/ledger.go:285-330`)
- THEN the server MUST return HTTP 200 with JSON lane objects containing lane ID, executor, status, and timestamps (`internal/ledger/ledger.go:286-324`).

### Requirement: Real-Time Server-Sent Events (SSE) Stream

The server MUST expose `GET /api/v1/events/stream` streaming ledger events (`internal/ledger/schema.go:34-43`, `internal/ledger/ledger.go:490-525`) and integration events (`internal/ledger/schema.go:171-180`) using `text/event-stream` and `http.Flusher`. The server MUST bind the push loop directly to `r.Context().Done()` and MUST terminate streaming goroutines upon client disconnect without resource leaks (`internal/serve/server.go:19-53`).

#### Scenario: Client streams live execution events

- GIVEN an active `lucind-ai serve` daemon (`cmd/lucind-ai/cli.go:674-725`) and concurrent run execution (`internal/run/batch.go:29-53`)
- WHEN a client sends `GET /api/v1/events/stream` with `Accept: text/event-stream`
- THEN the server MUST respond with `Content-Type: text/event-stream` and push event records as SSE frames as events are recorded in the ledger (`internal/ledger/schema.go:38-42`).

#### Scenario: Client disconnect terminates push loop

- GIVEN an active SSE connection to `/api/v1/events/stream`
- WHEN the client closes the HTTP connection
- THEN the server MUST detect context cancellation via `r.Context().Done()` and cleanly terminate the streaming loop without leaking goroutines or file descriptors (`internal/serve/server.go:41-52`).

### Requirement: Strict Loopback Isolation and Worktree Guard

The server MUST bind strictly to loopback addresses (`127.0.0.1`, `localhost`, `::1`) via `serve.IsLoopback` (`internal/serve/server.go:55-73`) and MUST return `serve.ErrNonLoopback` (`internal/serve/server.go:14,20-22`) and exit with status 1 when non-loopback addresses are provided (`cmd/lucind-ai/cli.go:691-694`). The server MUST refuse startup when executed from inside a linked worktree (`cmd/lucind-ai/cli.go:702-705`).

#### Scenario: Reject non-loopback bind address

- GIVEN a non-loopback address `0.0.0.0:7433`
- WHEN starting `lucind-ai serve --addr 0.0.0.0:7433` (`cmd/lucind-ai/cli.go:683-694`)
- THEN the server MUST reject the bind with `ErrNonLoopback` and exit with status 1 (`internal/serve/server.go:14,20-22`).

#### Scenario: Refuse startup inside linked worktree

- GIVEN the current working directory is inside a linked git worktree (`cmd/lucind-ai/cli.go:702-705`)
- WHEN running `lucind-ai serve`
- THEN the process MUST print an error message and exit with status 1 before opening the ledger database (`cmd/lucind-ai/cli.go:703-705`).

### Requirement: Individual Decision Enforcement and Anti-Bulk Rejection

All mutating action endpoints (`POST /approvals/{runID}/{laneID}`, `POST /api/v1/reconcile/requests/{id}/approve`) MUST require specific resource IDs in the URL path (`internal/serve/handlers.go:87-115`). The server MUST reject JSON array payloads or composite multi-item objects with HTTP 400 Bad Request (`internal/serve/handlers.go:161-176`). Empty or unselected decisions MUST be rejected with HTTP 400 (`internal/serve/handlers.go:178-189`).

#### Scenario: Single lane decision succeeds

- GIVEN a pending lane approval in the ledger (`internal/ledger/schema.go:45-56`, `internal/ledger/ledger.go:615-640`)
- WHEN a client sends `POST /approvals/{runID}/{laneID}` with `{"decision":"approved","approver":"alice"}` (`internal/serve/handlers.go:148-195`)
- THEN the server MUST record the approval in the ledger and return HTTP 200 `{"ok":true}` (`internal/serve/handlers.go:208-211`).

#### Scenario: Bulk approval payload rejected

- GIVEN multiple pending lane approvals
- WHEN a client sends a JSON array `[{"decision":"approved"}]` or composite body (`internal/serve/handlers.go:161-176`)
- THEN the server MUST return HTTP 400 with message `bulk approval rejected; decisions must be made individually` (`internal/serve/handlers.go:163`).

### Requirement: Embedded Asset Delivery and JSON API 404 Isolation

The server MUST serve embedded static UI assets from `embed.FS` (`internal/serve/static.go:8-18`) with accurate MIME headers for CSS and JavaScript (`internal/serve/handlers.go:43-48`). Non-file UI navigation requests MUST fall back to `index.html` (`internal/serve/handlers.go:68-77`), while unrecognized requests under `/api/*` MUST return a JSON-formatted HTTP 404 Not Found error (`internal/serve/handlers.go:39-55`).

#### Scenario: Serve embedded static asset

- GIVEN embedded static JavaScript and CSS assets (`internal/serve/static.go:8-18`)
- WHEN a client sends `GET /app.js`
- THEN the server MUST return HTTP 200 with `Content-Type: application/javascript` (`internal/serve/handlers.go:44-46`).

#### Scenario: Unmatched API route returns JSON 404

- GIVEN an unrecognized endpoint path `/api/v1/nonexistent`
- WHEN a client sends `GET /api/v1/nonexistent`
- THEN the server MUST return HTTP 404 Not Found with a JSON error payload rather than `index.html` (`internal/serve/handlers.go:53`).

## Open Questions

- [ ] Contract Precedence Note: `~/.claude/skills/sdd-propose/SKILL.md` defines a monolithic `proposal.md` written by a single agent, which is intentionally superseded by this packet's 3-lens parallel execution model writing `propose-lens-b.md`.
- [ ] Event Delivery Mechanism: Should SSE event streaming rely on SQLite WAL polling on `events` by sequence ID (`internal/ledger/schema.go:34-43`) or an IPC notification mechanism across independent `run` and `serve` processes?
- [ ] Development Asset Flag: Should `lucind-ai serve` introduce an optional `--dev-static-dir` flag (`cmd/lucind-ai/cli.go:676-690`) to allow hot-reloading frontend assets without recompilation during UI development?
