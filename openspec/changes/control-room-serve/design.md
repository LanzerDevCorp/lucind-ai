# Design: Control Room Serve

## Technical Approach

Replace 2s snapshot polling of `GET /api/state` (`internal/serve/handlers.go:79-85`; UI at `internal/serve/static/app.js:96-97`) with additive `/api/v1/` resource GETs plus `GET /api/v1/events/stream` (SSE). Keep `POST /approvals/{runID}/{laneID}`. Capabilities: `control-room-api-runs`, `control-room-api-features`, `control-room-api-reconcile`, `control-room-events-stream`; `approvals-web-ui` is the host.

`run` and `serve` are separate processes, each calling `ledger.Open` (`cmd/lucind-ai/cli.go:285,707`). There is no in-process bus. Reads are short SELECTs on schema v5 (`internal/ledger/schema.go:10`); WAL, `busy_timeout=5000`, and `SetMaxOpenConns(4)` already apply (`internal/ledger/ledger.go:162-185`). Do not hold a SQLite transaction across an HTTP request or SSE interval. Do not hook `ExecuteBatch` (`internal/run/batch.go:29-53`).

1. **REST reads.** Features and reconciliations use `serve.Model` (`internal/serve/model.go:14-24,128-343`). Lanes use `Lanes(ctx, runID)` (`internal/ledger/ledger.go:285-330`). Approvals use `PendingApprovals` (`internal/ledger/ledger.go:705-717`). Run IDs: new `Runs` as `SELECT DISTINCT run_id FROM lanes` (`internal/ledger/schema.go:18-32`). Pass `*Model` into `NewHandler` (`internal/serve/handlers.go:36-38`); CLI already builds a Model at `cmd/lucind-ai/cli.go:852`.
2. **SSE.** Stdlib `http.Flusher`, `text/event-stream`. New `id > lastID` queries on `events` and `integration_events` (`internal/ledger/schema.go:34-43,171-180`). Existing `Events` / `IntegrationEvents` are per-run / per-feature (`internal/ledger/ledger.go:490-525,892-925`). Bind the loop to `r.Context().Done()`.
3. **Boundary.** Loopback (`internal/serve/server.go:14,19-22,55-73`), linked-worktree refusal before `ledger.Open` (`cmd/lucind-ai/cli.go:702-707`), 3s `Shutdown` (`internal/serve/server.go:41-52`), no global `WriteTimeout` (`internal/serve/server.go:24-28`). Anti-bulk 400 stays on path-bound POST (`internal/serve/handlers.go:87-115,161-176`). `embed.FS` stays (`internal/serve/static.go:8-18`). Unmatched `/api/*` must become JSON 404; today it is stdlib `http.NotFound` (`internal/serve/handlers.go:53`).

## Architecture Decisions

### Decision: SSE with SQLite cursor tailing

**Choice**: `text/event-stream` via `http.Flusher`; poll `events` and `integration_events` with autoincrement `id > lastID` (`internal/ledger/schema.go:35,172,43`); exit on `r.Context().Done()`.
**Alternatives considered**: WebSockets (`go.mod` has sqlite/uuid/jsonschema only; no duplex need); in-process channels (`run`/`serve` share only SQLite at `cmd/lucind-ai/cli.go:285,707`); growing `/api/state` snapshots (WAL amplification vs `ExecuteBatch` writers at `internal/run/batch.go:29-53`).
**Rationale**: Stdlib + browser `EventSource`. PK cursor is a short read that does not block writers.
**Terminal consumer**: new route on the mux in `NewHandler` (`internal/serve/handlers.go:36-38`), served by `ListenAndServe` (`internal/serve/server.go:19-28`).

### Decision: Compose `*serve.Model` and `*ledger.Ledger` in handlers

**Choice**: Add `m *Model` to `NewHandler`. Reuse Model for features/reconciliations; Ledger for lanes/approvals/events.
**Alternatives considered**: A new query service (Model already is the shell-free surface at `internal/serve/model.go:14-24`); raw SQL in handlers (bypasses `internal/serve/model_test.go`).
**Rationale**: Handlers stay HTTP. Callers: `serveDispatch` (`cmd/lucind-ai/cli.go:715`) and `internal/serve/server_test.go:70,114`.
**Terminal consumer**: `NewHandler` (`internal/serve/handlers.go:36-38`).

### Decision: Run listing without schema v6

**Choice**: Freeze `schemaVersion` at 5 (`internal/ledger/schema.go:10`). Add `Ledger.Runs(ctx) ([]string, error)` from `lanes` (`internal/ledger/schema.go:18-32`). Lane rows stay `Lanes(ctx, runID)`.
**Alternatives considered**: DDL `runs` table (violates freeze); omit listing and require client-supplied `runID` (fails `control-room-api-runs`).
**Rationale**: `PRIMARY KEY (run_id, lane_id)` (`internal/ledger/schema.go:31`) is enough for discovery. `events` is not required for the ID list.
**Terminal consumer**: `GET /api/v1/runs` registered in `NewHandler` (`internal/serve/handlers.go:36-118`).

### Decision: Isolate `/api/` JSON from `embed.FS`

**Choice**: `/api/state` and `/api/v1/*` always JSON, including JSON 404. Non-API paths keep `embed.FS` MIME mapping (`internal/serve/handlers.go:43-48`) and `GET /` → `index.html` (`internal/serve/handlers.go:68-77`).
**Alternatives considered**: Keep the `/` catch-all inspecting every path (`internal/serve/handlers.go:39-77`) — unmatched `/api/*` falls through to `http.NotFound` (`internal/serve/handlers.go:53`); external router (new dependency).
**Rationale**: API clients must not receive HTML. Stdlib mux is enough.
**Terminal consumer**: `NewHandler` mux (`internal/serve/handlers.go:36-118`).

### Decision: Keep path-bound individual mutations

**Choice**: Leave `POST /approvals/{runID}/{laneID}` and `/defect` (`internal/serve/handlers.go:87-115,213-231`). Reject JSON arrays and composite `approvals`/`decisions`/`lanes` with 400 (`internal/serve/handlers.go:161-176`). Empty decision 400; already-decided 409 (`internal/serve/handlers.go:178-198`).
**Alternatives considered**: `POST /api/v1/approvals/bulk`; move IDs into the body (breaks `internal/serve/server_test.go:42-93` and the UI).
**Rationale**: One inspected row per POST.
**Terminal consumer**: `handleDecide` (`internal/serve/handlers.go:148-211`).

## Flow and Invariants

```
Client → ServeMux → Model / Ledger → SQLite WAL → JSON or SSE
```

1. **Client → mux.** Loopback at listen (`internal/serve/server.go:55-73`); linked worktree refused before `ledger.Open` (`cmd/lucind-ai/cli.go:702-707`). `/api/` is JSON; static is `embed.FS`. POST IDs stay in the path. SSE needs `http.Flusher`. Break: public bind, HTML API 404, bulk body, hung stream.
2. **Mux → Model/Ledger.** `r.Context()` on every query. SSE loop ends on `r.Context().Done()`. No raw SQL in handlers; no tx held across a request or poll interval. Break: leaked goroutine/FD, long-held lock.
3. **Ledger → SQLite.** Short indexed SELECTs (`internal/ledger/schema.go:18-180`). Cursor tails `id > ?`. `SetStatus` pairs status UPDATE with event INSERT in one tx (`internal/ledger/ledger.go:448-486`). `Decide` is a pending-row UPDATE only (`internal/ledger/ledger.go:615-640`) — it does not write `events`. Break: `SQLITE_BUSY` vs `ExecuteBatch`.
4. **Mux → client.** REST 200 `application/json`; nil slices become `[]` (`internal/serve/handlers.go:126-128`). Mutations `{"ok":true}` (409 decided, 404 unknown). SSE frames must `Flush()`.

## Interfaces / Contracts

| Surface | Today | Delta | Compatible? |
|---|---|---|---|
| `NewHandler` | `(l *ledger.Ledger, defaultApprover, opencodeCmd string)` at `internal/serve/handlers.go:36` | Add `m *Model` (`internal/serve/model.go:21-24`) | No — update `cmd/lucind-ai/cli.go:715` and tests |
| `GET /api/v1/runs` | — | `[]string` via new `Ledger.Runs` | Yes |
| `GET /api/v1/lanes?run_id=` | — | `[]ledger.Lane` via `Lanes` (`internal/ledger/ledger.go:285`) | Yes |
| `GET /api/v1/features`, `/api/v1/features/{id}` | — | `ListFeatures` / `GetFeature` (`internal/serve/model.go:128,152`) | Yes |
| `GET /api/v1/reconciliations?feature_id=` | — | `ListReconciliationRequests` (`internal/serve/model.go:278`) | Yes |
| `GET /api/v1/approvals` | — | `PendingApprovals` (`internal/ledger/ledger.go:705`) | Yes |
| `GET /api/v1/events/stream` | — | SSE of `events` + `integration_events` | Yes |
| `GET /api/state` | `ServerState` (`internal/serve/handlers.go:16-21,79-85`) | Unchanged | Yes |
| `POST /approvals/{runID}/{laneID}` `/defect` | path + anti-bulk 400 | Unchanged | Yes |
| Unmatched `/api/*` | `http.NotFound` (`internal/serve/handlers.go:53`) | JSON 404 | Yes (error shape only) |
| `Ledger.Runs` / `EventsSince` / `IntegrationEventsSince` | absent; neighbors are `Lanes` / `Events` / `IntegrationEvents` | Additive queries | Yes |
| `lucind-ai serve` flags | `-addr`, `-approver`, `-approval-timeout` (`cmd/lucind-ai/cli.go:683-685`) | Unchanged | Yes |
| SQLite | `schemaVersion` 5 (`internal/ledger/schema.go:10`) | No DDL | Yes |

```go
func NewHandler(l *ledger.Ledger, m *Model, defaultApprover, opencodeCmd string) http.Handler
func (l *Ledger) Runs(ctx context.Context) ([]string, error)
func (l *Ledger) EventsSince(ctx context.Context, lastID int64) ([]Event, error)
func (l *Ledger) IntegrationEventsSince(ctx context.Context, lastID int64) ([]IntegrationEvent, error)
```

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/serve/handlers.go` | Modify | `NewHandler(*Model)`; `/api/v1/*`; SSE; JSON 404 under `/api/` | `cmd/lucind-ai/cli.go:715`; `ListenAndServe` (`internal/serve/server.go:24-28`) |
| `internal/ledger/ledger.go` | Modify | `Runs`, `EventsSince`, `IntegrationEventsSince` | those handlers |
| `cmd/lucind-ai/cli.go` | Modify | `serve.NewModel(ledg)` into `NewHandler` in `serveDispatch` (`cmd/lucind-ai/cli.go:674-725`) | `lucind-ai serve` |
| `internal/serve/server_test.go` | Modify | Pass `*Model`; REST, JSON 404, SSE cancel tests | `go test ./internal/serve` |
| `internal/serve/model_test.go` | Keep AST | `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`) stays on `model.go` | `go test ./internal/serve` |

`internal/serve/server.go` and CLI flags are unchanged.

## Testing Strategy and Test Seams

| Layer | What | Approach | Seam |
|---|---|---|---|
| REST | `/api/v1/runs\|lanes\|features\|reconciliations\|approvals` → 200 JSON | `httptest` + populated ledger | `NewHandler` (`internal/serve/server_test.go:70-77,114-120`); Model (`internal/serve/model.go:128-343`); `ledger.Open` (`internal/serve/server_test.go:44-49`) |
| Routing | missing `/api/*` → JSON 404; JS/CSS MIME | new `httptest` | mux (`internal/serve/handlers.go:36-118`); keep embed tests (`internal/serve/static_test.go:11-103`) as non-MIME coverage |
| Loopback | `0.0.0.0` / remote → `ErrNonLoopback` | keep table | `internal/serve/server.go:19-73`; `internal/serve/server_test.go:17-40`; `cmd/lucind-ai/cli_test.go:1908-1917` |
| Anti-bulk / 409 | array, composite, empty → 400; second POST → 409 | keep | `internal/serve/handlers.go:148-211`; `internal/serve/server_test.go:42-135,196-236` |
| AST | `model.go` must not `os/exec` or `git` | keep | `internal/serve/model_test.go:595-627` (parses `model.go` only) |
| SSE | frames from both event tables; cancel on `r.Context().Done()` | `httptest.Server` + `Flusher` | new; `ListenAndServe` shutdown at `internal/serve/server.go:41-52` is process-level, not per-client |
| Worktree | `serve` exit 1 before `ledger.Open` | new CLI test | production check `cmd/lucind-ai/cli.go:702-707` (no test today) |
| WAL | REST+SSE during `ExecuteBatch`; no `SQLITE_BUSY` | concurrent | pool `internal/ledger/ledger.go:162-185`; `internal/run/batch.go:29-53` |

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: HTTP + `embed.FS` only; no file execution | — | None |
| Git repository selection | `git -C`, relative/absolute paths | N/A: SQLite reads; linked worktrees refused; `internal/serve` does not invoke git | — | None |
| Commit state | staged, `commit -a`, empty index | N/A: no commit/index | — | None |
| Push state | tracking branch, first push, refspec | N/A: no remotes | — | None |
| PR commands | `--head`, env prefix, composed argv | N/A: no PR argv | — | None |

Unauthenticated loopback POST remains a preserved boundary (loopback + anti-bulk), not a new skill-matrix row. RED coverage stays the existing listen/CLI/anti-bulk tests plus JSON 404.

## Rollback and Additivity

**Choice**: `git revert` of `internal/serve/`, `cmd/lucind-ai/cli.go`, and the three ledger query methods.
**Alternatives considered**: Feature flags or a second binary. Rejected — no DDL, no format shift.
**Rationale**: `schemaVersion` stays 5 (`internal/ledger/schema.go:10`). Wire: keep `GET /api/state` and `POST /approvals/{runID}/{laneID}`; add `/api/v1/*` on new paths. `Packet` (`internal/packet/packet.go:32-47`) and `internal/result/result.schema.json` unchanged. Revert restores prior HTTP and CLI; existing DBs remain valid.

No migration required.

## Open Questions and Out of Scope

Open questions:

- [ ] SSE cadence across `run`/`serve`: fixed ticker vs adaptive backoff on `id > lastID`. In-process pub/sub is not enough.
- [ ] Mutations beyond `POST /approvals/{runID}/{laneID}` (e.g. `reconcile.Service.Approve` at `cmd/lucind-ai/cli.go:1166-1176`): HTTP by default, flag-gated, or CLI-only on unauthenticated loopback?
- [ ] Optional `--dev-static-dir` on the serve flag set (`cmd/lucind-ai/cli.go:675-689`) to bypass `embed.FS`?
- [ ] HTTP shape for attempts, leases, and overlap evidence (`internal/serve/model.go:166-275`): nested in feature JSON, extra GETs, or deferred? Proposal capability `control-room-api-features` names them; no lens specified routes.

Out of scope: PTY capture (`control-room-capture`); schema/DDL (`control-room-ledger`); telemetry/cost (`control-room-telemetry`); UI/CSS (`control-room-ui-shell`, `control-room-ui-views`); SPA history fallback; auth, RBAC, remote bind (`internal/serve/server.go:14,20-22`); changing `ExecuteBatch`; HTTP reconcile/workflow dispatch until the question above is answered.
