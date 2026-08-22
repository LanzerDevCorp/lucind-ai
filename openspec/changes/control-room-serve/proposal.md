# Proposal: Control Room Serve

## Executive summary and problem

`lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) is a loopback approvals server. `GET /api/state` (`internal/serve/handlers.go:79-85`) returns `ServerState` — approver, wrong-approval rate, opencode command, pending approvals (`internal/serve/handlers.go:15-21,120-146`). The UI polls it every 2s (`internal/serve/static/app.js:96-97`), re-reading SQLite (`internal/ledger/schema.go:18-57`) while `ExecuteBatch` writes concurrently (`internal/run/batch.go:29-53`).

`serve.Model` already queries features, attempts, leases, overlap evidence, and reconciliation requests (`internal/serve/model.go:14-24,128-343`). `NewHandler` does not take it (`internal/serve/handlers.go:36-38`); CLI `runFeatureStatus` does (`cmd/lucind-ai/cli.go:852`). There is no HTTP for `Lanes` / `LaneStates` / `Events` (`internal/ledger/ledger.go:285-358,490-525`).

This change does not hook `ExecuteBatch`. It reads rows already written by `AppendEvent` and `SetStatus` (`internal/ledger/ledger.go:366-384,448-486`).

## Selected candidate and approach

**Candidate 1 — granular REST reads + SSE.** Rejected: growing `/api/state` snapshots (WAL contention with `AppendEvent`/`SetStatus`); WebSockets (third-party library; `go.mod` has sqlite/uuid/jsonschema only); in-memory cache in `serve` (`run` and `serve` each `ledger.Open` at `cmd/lucind-ai/cli.go:285,707`); multi-user bind (`docs/prd.md:53`; `internal/serve/server.go:14,20-22`).

1. Resource GETs on the existing mux (`internal/serve/handlers.go:36-118`): `/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconciliations`, `/api/v1/approvals`. Features and reconciliations reuse `serve.Model`. Lanes reuse `Lanes` (requires `runID`; `internal/ledger/ledger.go:285-330`). Approvals reuse `PendingApprovals` (`internal/ledger/ledger.go:705-717`). There is no `Ledger.Runs()`; `/api/v1/runs` needs a new listing or a derivation from `lanes` (`internal/ledger/schema.go:18-32`). Pass `*serve.Model` into `NewHandler`.
2. `GET /api/v1/events/stream` via stdlib `http.Flusher`, `text/event-stream`. Tail `events` (`internal/ledger/schema.go:34-43`) and `integration_events` (`internal/ledger/schema.go:171-180`). Existing `Events` / `IntegrationEvents` are per-run / per-feature (`internal/ledger/ledger.go:490-525,892-925`), not an `id > lastID` cursor — that query is new. Bind the push loop to `r.Context().Done()`.
3. Keep loopback (`internal/serve/server.go:14,19-22,55-73`; `cmd/lucind-ai/cli.go:683-694`), linked-worktree refusal before `ledger.Open` (`cmd/lucind-ai/cli.go:702-707`), 3s `Shutdown` (`internal/serve/server.go:41-52`), and no global `WriteTimeout` (`internal/serve/server.go:24-28`).
4. Keep `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115,148-211`): HTTP 400 on JSON arrays, composite `approvals`/`decisions`/`lanes`, or empty decision (`internal/serve/handlers.go:161-183`; `internal/serve/server_test.go:42-93`). Preserve `GET /api/state` and `embed.FS` (`internal/serve/static.go:8-18`).

Short SELECTs only; WAL + `busy_timeout=5000` + `SetMaxOpenConns(4)` already apply (`internal/ledger/ledger.go:162-185`). Do not hold a SQLite transaction across an HTTP request or SSE interval.

## Concepts and architecture

- Split snapshot polling (`ServerState`) from resource GETs plus a push stream.
- `run` and `serve` are separate processes sharing SQLite, not an in-process channel.
- Client: `EventSource` plus targeted GETs; `/api/state` stays for the current UI.

## User and capability impact

| Capability | Impact | Description |
|---|---|---|
| `control-room-api-runs` | Added | JSON reads for runs and lanes (`Lanes` at `internal/ledger/ledger.go:285-330`; `lanes` at `internal/ledger/schema.go:18-32`). |
| `control-room-api-features` | Added | JSON reads for features (and Model-backed attempts/leases/overlap) (`internal/serve/model.go:14-24,128-266`; CLI seam `cmd/lucind-ai/cli.go:852-895`). |
| `control-room-api-reconcile` | Added | JSON reads for reconciliation requests/candidates (`internal/serve/model.go:72-115,278-323`; `internal/ledger/schema.go:141-169`). Read-only in this change. |
| `control-room-events-stream` | Added | SSE of run `events` and `integration_events`. |
| `approvals-web-ui` | Modified | Host the new reads and stream; keep loopback, worktree guard, individual decisions, `/api/state`, and `POST /approvals/{runID}/{laneID}`. |

Personas: operators watching multi-lane runs; Control Room UI replacing 2s polls; automation reading JSON instead of `feature status`.

## Delta specifications

### Granular REST reads

The server MUST expose read-only JSON under `/api/v1/` for runs, lanes, features, reconciliations, and approvals. Responses MUST be HTTP 200 with `Content-Type: application/json`. Lanes MUST use `Lanes(ctx, runID)`. Features MUST use `ListFeatures` / `GetFeature` (`internal/serve/model.go:27-35,128-149`; `internal/ledger/schema.go:96-104`).

- GIVEN features in the ledger, WHEN `GET /api/v1/features`, THEN HTTP 200 with a JSON array of `serve.Feature`.
- GIVEN lanes for a run, WHEN `GET /api/v1/lanes` with that run ID, THEN HTTP 200 with lane ID, executor, status, timestamps (`internal/ledger/ledger.go:286-324`).

### SSE stream

`GET /api/v1/events/stream` MUST use `text/event-stream` and `http.Flusher`, frame `events` (`internal/ledger/schema.go:38-42`) and `integration_events`, and exit the push loop on `r.Context().Done()`.

- GIVEN `serve` (`cmd/lucind-ai/cli.go:674-725`) and a concurrent `ExecuteBatch`, WHEN a client sends `Accept: text/event-stream`, THEN the server streams event rows as they appear in the ledger.
- GIVEN an open stream, WHEN the client disconnects, THEN `r.Context().Done()` ends the loop with no leaked goroutine or FD.

### Loopback and worktree guard

MUST bind only loopback via `IsLoopback` (`internal/serve/server.go:55-73`). Non-loopback MUST return `ErrNonLoopback` and exit 1 (`internal/serve/server.go:14,20-22`; `cmd/lucind-ai/cli.go:691-694`). Linked worktrees MUST be refused before `ledger.Open` (`cmd/lucind-ai/cli.go:702-707`).

- GIVEN `0.0.0.0:7433`, WHEN `lucind-ai serve --addr 0.0.0.0:7433`, THEN `ErrNonLoopback` and exit 1 (`internal/serve/server_test.go:17-40`; `cmd/lucind-ai/cli_test.go:1908-1917`).
- GIVEN a linked worktree, WHEN `lucind-ai serve`, THEN error and exit 1 before opening the ledger.

### Individual decisions

`POST /approvals/{runID}/{laneID}` MUST keep IDs in the path (`internal/serve/handlers.go:87-115`). Arrays and composite objects MUST be HTTP 400 (`internal/serve/handlers.go:161-176`). Empty decisions MUST be HTTP 400 (`internal/serve/handlers.go:178-183`).

- GIVEN a pending approval (`internal/ledger/schema.go:45-56`), WHEN `POST` `{"decision":"approved","approver":"alice"}`, THEN `Decide` (`internal/ledger/ledger.go:615-640`) and HTTP 200 `{"ok":true}` (`internal/serve/handlers.go:208-211`).
- WHEN the body is `[{"decision":"approved"}]` or a composite object, THEN HTTP 400 `bulk approval rejected; decisions must be made individually` (`internal/serve/handlers.go:163`).

### Embedded assets and API 404s

MUST serve `embed.FS` with JS/CSS MIME types (`internal/serve/static.go:8-18`; `internal/serve/handlers.go:43-48`). `GET /` still serves `index.html` (`internal/serve/handlers.go:68-77`). Unmatched `/api/*` MUST return JSON 404, not `index.html`. Today unmatched paths use stdlib `http.NotFound` (`internal/serve/handlers.go:53`).

- WHEN `GET /app.js`, THEN HTTP 200 `Content-Type: application/javascript` (`internal/serve/handlers.go:44-46`).
- WHEN `GET /api/v1/nonexistent`, THEN HTTP 404 JSON, not `index.html`.

## Technical risks and failure modes

| Risk | Impact | Mitigation |
|---|---|---|
| `SQLITE_BUSY` vs batch writers | High | Short SELECTs; no tx across HTTP/SSE; existing WAL/`busy_timeout`/pool of 4 (`internal/ledger/ledger.go:162-185`). |
| SSE goroutine/FD leaks | High | Bind the push loop to `r.Context().Done()`. Mux has no stream today (`internal/serve/handlers.go:36-118`). |
| Global `WriteTimeout` killing SSE | Medium | Keep `ReadHeaderTimeout` only (`internal/serve/server.go:24-28`). |
| DNS rebinding to unauthenticated POSTs | High | Loopback + worktree guard (`internal/serve/server.go:14,19-22,55-73`; `cmd/lucind-ai/cli.go:683-705`). Bind-address check only; no Host gate on `/approvals/`. |
| Bulk mutation on new routes | High | IDs in the path; keep array/composite 400 (`internal/serve/handlers.go:87-115,161-189`). |
| HTML 404 / SPA fallback masking `/api/*` | Medium | JSON 404 under `/api/*` (`internal/serve/handlers.go:39-77`). |
| Cross-process event lag | Medium | Tail SQLite, not a `serve`-local channel (`cmd/lucind-ai/cli.go:285,707`). Cursor vs IPC is an open question. |
| Shutdown with live streams | Medium | Existing 3s `Shutdown` (`internal/serve/server.go:41-52`). |

## Rollback and additivity

`git revert` of `internal/serve/` and `cmd/lucind-ai/cli.go`. No DDL, no schema version bump (`schemaVersion` is 5 at `internal/ledger/schema.go:10`; file `internal/ledger/schema.go:1-308`). Reads only existing tables (`internal/ledger/schema.go:18-180`).

Additive HTTP: keep `GET /api/state` and `POST /approvals/{runID}/{laneID}`; add `/api/v1/*` and `/api/v1/events/stream` on new paths. Packet struct unchanged (`internal/packet/packet.go:32-47`). Result envelope remains `internal/result/result.schema.json`.

## Test and validation impact

| Layer | Coverage |
|---|---|
| Loopback / worktree | Keep `TestNonLoopbackListenFails` and CLI `0.0.0.0` (`internal/serve/server_test.go:17-40`; `cmd/lucind-ai/cli_test.go:1908-1917`). Add linked-worktree refusal before ledger open (`cmd/lucind-ai/cli.go:702-707`). |
| Anti-bulk | Keep array/composite 400, empty 400, 409 already-decided (`internal/serve/server_test.go:42-135,196-236`; `internal/serve/handlers.go:148-211`). |
| REST reads | New HTTP tests for `/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconciliations` vs ledger/Model (`internal/serve/model.go:128-343`). |
| SSE | New: `text/event-stream`, frames from `events` / `integration_events`, cancel on `r.Context().Done()`. |
| WAL stress | REST + SSE during `ExecuteBatch`; no `SQLITE_BUSY` within pool of 4. |
| Model AST | Keep `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-628`). |
| Static / API 404 | Keep embed tests (`internal/serve/static_test.go:11-103`); add JS/CSS MIME and JSON 404 for missing `/api/*` (`internal/serve/handlers.go:39-48`). |

## Out of scope and open questions

Out of scope: PTY capture (`control-room-capture`); schema/DDL (`control-room-ledger`); telemetry/cost (`control-room-telemetry`); UI layout/CSS (`control-room-ui-shell`, `control-room-ui-views`); SPA history fallback; auth, RBAC, remote bind; changing `ExecuteBatch`; HTTP reconcile/workflow dispatch.

Open questions:

- SSE across `run`/`serve`: SQLite `id` cursor with adaptive backoff, fixed ticker, commit hooks, or OS IPC? In-process pub/sub is not enough.
- Mutations beyond `POST /approvals/{runID}/{laneID}` (e.g. wrapping `reconcile.Service.Approve` at `cmd/lucind-ai/cli.go:1166-1176`): HTTP by default, `--enable-dispatch`, or CLI-only for unauthenticated loopback?
- Optional `--dev-static-dir` on the serve flag set (`cmd/lucind-ai/cli.go:675-689`) to bypass `embed.FS` during UI work?
