# Explore: control-room-serve

**Recommendation:** Extend `lucind-ai serve` with a granular REST read API plus a Server-Sent Events (SSE) stream. Wire existing `serve.Model` queries over HTTP, keep loopback isolation and single-item approvals, and reject snapshot-polling growth and WebSockets.

Ready for proposal: **Yes**, after the open questions below (event delivery across processes, mutation gating, optional `--dev-static-dir`).

## Problem statement and background

`serveDispatch` (`cmd/lucind-ai/cli.go:674-725`) is a single-purpose approvals web server. It binds loopback-only via `serve.ListenAndServe` (`internal/serve/server.go:19-53`), serves `embed.FS` assets (`internal/serve/static.go:8-18`), and exposes `GET /api/state` (`internal/serve/handlers.go:79-85`) which returns `ServerState` (approver, wrong-approval rate, command, pending approvals) (`internal/serve/handlers.go:16-21,120-146`). The UI polls that endpoint every 2s (`internal/serve/static/app.js:96-97`), which calls `PendingApprovals` (`internal/ledger/ledger.go:705-717`) and `ApproverRate` (`internal/ledger/ledger.go:795-814`). Decisions POST to `/approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115,148-211`).

`serve.Model` already queries features, attempts, leases, overlap evidence, and reconciliation requests (`internal/serve/model.go:14-24,128-343`). `NewHandler` does not use it (`internal/serve/handlers.go:36-118`); CLI `runFeatureStatus` does (`cmd/lucind-ai/cli.go:852`). There are no HTTP endpoints for `Lanes`, `LaneStates`, or `Events` (`internal/ledger/ledger.go:285-330,332-358,490-525`). Polling re-reads SQLite tables (`internal/ledger/schema.go:18-57`) while `ExecuteBatch` writes concurrently (`internal/run/batch.go:29-53`).

Affected areas: `cmd/lucind-ai/cli.go` (`serveDispatch`), `internal/serve/{handlers,server,static,model}.go`, `internal/serve/static/app.js`, ledger read APIs. Schema DDL stays with `control-room-ledger`.

Constraints already in code: non-loopback rejected (`internal/serve/server.go:14,19-22,55-73`; `cmd/lucind-ai/cli.go:691-694`); linked worktrees refused (`cmd/lucind-ai/cli.go:702-705`); 3s shutdown (`internal/serve/server.go:41-52`); WAL + `busy_timeout=5000` + `SetMaxOpenConns(4)` (`internal/ledger/ledger.go:162-185`); PRD single-user (`docs/prd.md:53`).

## Candidate approaches

| Approach | Pros | Cons | Feasibility |
|---|---|---|---|
| **1. Granular REST + SSE** | stdlib `net/http` + `http.Flusher`; resource GETs reuse `serve.Model` (`internal/serve/model.go:128-343`); browser `EventSource` reconnects | SSE is unidirectional (mutations stay POST, as today at `internal/serve/handlers.go:148-211`); subscriber lifecycle must track `r.Context()` | **High.** Fits `ListenAndServe` (`internal/serve/server.go:19-53`) and the existing mux (`internal/serve/handlers.go:36-118`). |
| **2. Expand `/api/state` polling** | Few routes; stateless ticks | Every 1–2s serializes growing history (`internal/ledger/schema.go:18-57`); no live stdout/progress; WAL reads contend with `AppendEvent`/`SetStatus` writes (`internal/ledger/ledger.go:366-384,448-486`) | **Medium** to implement via Model, **poor** under parallel batches. |
| **3. WebSocket gateway** | Bidirectional on one socket | Adds a third-party WS library (`go.mod` has sqlite/uuid/jsonschema only; `net/http` has no WS server); heartbeats, framing, `curl`-hostile | **Low–medium.** Viable, not justified over REST+SSE. |

Candidate 1 is the approach. Proposed routes: `/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconciliations`, `/api/v1/approvals`, and `/api/v1/events/stream` tailing `events` (`internal/ledger/schema.go:34-43`).

`run` and `serve` are separate processes that each `ledger.Open` (`cmd/lucind-ai/cli.go:285,707`). An in-process channel inside `serve` cannot see the other process's `AppendEvent`. SSE must follow SQLite (or IPC), not serve-local pub/sub alone.

## User and capability impact

Personas: operators watching multi-lane runs and issuing one-lane approvals; the Control Room UI replacing 2s `/api/state` polls (`internal/serve/static/app.js:96-97`); automation reading JSON instead of shelling out to `feature status`.

Today: approvals UI + `/api/state` + `/approvals/` + `/approvals/.../defect` (`internal/serve/handlers.go:36-118,213-231`). Model is CLI-only (`cmd/lucind-ai/cli.go:852`).

Proposed: keep those invariants (loopback, individual decisions, worktree refusal) and add REST reads for runs/lanes (`internal/ledger/schema.go:18-32`), features/reconcile (`internal/serve/model.go:128-343`), and SSE of run events (`internal/ledger/schema.go:38-42`) plus `integration_events` (`internal/ledger/schema.go:171-180`). Reconciliation *HTTP* dispatch does not exist; the seam to wrap is CLI `reconcile.Service.Approve` (`cmd/lucind-ai/cli.go:1166-1176`). Whether serve grows mutating dispatch at all is an open question.

## Scenarios and use cases

1. **Approvals state (current).** `GET /api/state` returns `ServerState` (`internal/serve/handlers.go:79-85,120-146`). Proposed `/api/v1/state` would add feature summaries via `ListFeatures` (`internal/serve/model.go:128-149`) without stuffing every table into one snapshot.

2. **Live events (proposed).** While `lucind-ai run` executes (`internal/run/batch.go:29-53`) and `serve` listens (`cmd/lucind-ai/cli.go:717-722`), the UI opens `GET /api/v1/events/stream`. The server pushes `events` rows (`internal/ledger/schema.go:34-43`; `Events` at `internal/ledger/ledger.go:490-525`). No SSE handler exists today.

3. **Single approval (current).** `POST /approvals/{runID}/{laneID}` with `{"decision":"approved","approver":"alice"}` (`internal/serve/handlers.go:148-211`) calls `Decide` (`internal/ledger/ledger.go:615-640`) and returns `{"ok":true}`.

4. **Anti-bulk (current).** Array or composite bodies return HTTP 400 `bulk approval rejected; decisions must be made individually` (`internal/serve/handlers.go:161-176,163`; `internal/serve/server_test.go:42-93`). Empty decision is also 400 (`internal/serve/handlers.go:178-183`).

5. **Reconcile from the UI (proposed).** An `awaiting` request (`internal/ledger/schema.go:141-154`) would POST a single-resource path that delegates to `reconcile.Service.Approve` (`cmd/lucind-ai/cli.go:1166-1176`). Not an HTTP route today.

6. **Non-loopback (current).** `--addr 0.0.0.0:7433` fails `IsLoopback` (`cmd/lucind-ai/cli.go:683-694`; `internal/serve/server.go:19-22,57-73`) with `ErrNonLoopback` (`internal/serve/server.go:14`) and exit 1.

## Technical risks and trade-offs

| Risk | Severity | Notes |
|---|---|---|
| SQLITE_BUSY under dashboard + batch writes | High | WAL/`busy_timeout=5000`/pool of 4 already set (`internal/ledger/ledger.go:162-185`). Keep reads short; do not hold tx across HTTP. Spike 2. |
| SSE goroutine/FD leaks | High | Bind push loops to `r.Context().Done()`; heartbeats. Mux today has no stream (`internal/serve/handlers.go:36-118`). |
| DNS rebinding to mutating POSTs | High | Bind-address check only (`internal/serve/server.go:20-22,55-73`). No `Host` or custom-header gate on `/approvals/` (`internal/serve/handlers.go:87-115`). |
| Global `WriteTimeout` killing SSE | Medium | Server sets only `ReadHeaderTimeout` (`internal/serve/server.go:24-28`). Do not add a global write deadline on the stream. |
| New endpoints allowing bulk mutate | High | Keep IDs in the path; keep array rejection (`internal/serve/handlers.go:148-176`). |
| SPA fallback masking `/api/*` 404s | Medium | Unknown paths already `http.NotFound` (`internal/serve/handlers.go:39-53`). If UI history-fallback is added, JSON 404 under `/api/*`. |

| Choice | Advantages | Disadvantages |
|---|---|---|
| SSE | stdlib flusher; `EventSource`; HTTP/1.1 ~6 connections per origin | Commands still POST |
| Short polling | Stateless | 2s SQLite churn (`internal/serve/static/app.js:96-97`) |
| WebSockets | Duplex | Third-party WS stack |
| Direct SQLite reads | One ledger for `run` and `serve` | Read bursts vs WAL writers |
| In-memory cache in serve | Fast | Diverges from the `run` process |
| `embed.FS` | Single binary | Rebuild to see UI edits |
| Loopback + (proposed) Host check | No tokens | Relies on localhost browser rules |
| Session tokens | Extra local-process defense | CLI friction; out of scope for v1 |

## Potential spikes

1. **SSE + disconnect.** `http.Flusher` stream of `events` (`internal/ledger/schema.go:34-43`) cancelled on client drop (`internal/serve/server.go:19-53`). Path: `/api/v1/events/stream`.
2. **WAL concurrency.** `serve.Model` reads (`internal/serve/model.go:128-343`) during `ExecuteBatch` (`internal/run/batch.go:29-53`) and `ledger.Open` (`internal/ledger/ledger.go:146-192`); measure latency and `SQLITE_BUSY`.
3. **SPA vs API 404.** Embed static (`internal/serve/static.go:8-18`); `index.html` fallback for UI GETs; JSON 404 for missing `/api/*` (`internal/serve/handlers.go:39-77`).

## Success criteria

- Loopback-only bind; `ErrNonLoopback` otherwise (`internal/serve/server.go:14-22,57-73`).
- JSON REST for state, runs, features, reconcile requests via `serve.Model` (`internal/serve/model.go:128-343`).
- SSE `text/event-stream` for `events` and `integration_events` (`internal/ledger/schema.go:38-42,171-180`).
- Bulk and unselected decisions still HTTP 400 (`internal/serve/handlers.go:161-183`; `internal/serve/server_test.go:42-93`).
- Embedded assets with JS/CSS MIME types (`internal/serve/static.go:8-18`; `internal/serve/handlers.go:39-55`).
- Shutdown within 3s on cancel (`internal/serve/server.go:41-52`).
- Refuse linked worktrees (`cmd/lucind-ai/cli.go:702-705`).

## Out of scope and open questions

Out of scope: PTY/process capture (`control-room-capture`); schema migrations/SQL (`control-room-ledger`); token/cost telemetry (`control-room-telemetry`); UI layout/CSS (`control-room-ui-shell` / `control-room-ui-views`); multi-user auth, RBAC, and remote bind (`internal/serve/server.go:20-22`; `docs/prd.md:53`).

Open questions:

- How should `serve` learn about `run` events: SQLite `events` tail by id (`internal/ledger/schema.go:34-43`), OS IPC, or SQLite change hooks? In-process pub/sub alone is not sufficient (separate processes).
- Add mutation/dispatch HTTP (beyond existing single-lane decide/defect), or gate with a flag, or leave dispatch on the CLI to protect unauthenticated loopback (`cmd/lucind-ai/cli.go:683-694`)?
- Optional `--dev-static-dir` to bypass `embed.FS` during UI work?
