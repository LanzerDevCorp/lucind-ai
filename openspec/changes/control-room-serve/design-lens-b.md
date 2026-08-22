# Design Lens B — Surface & Flow: Control Room Serve

## Assumed architecture

This change extends existing packages `internal/serve` and `internal/ledger` without adding new Go packages or bumping SQLite `schemaVersion` beyond 5 (`internal/ledger/schema.go:10`). `internal/serve/handlers.go` extends `NewHandler` (`internal/serve/handlers.go:36-38`) to accept `*serve.Model` (`internal/serve/model.go:21-24`) alongside `*ledger.Ledger` (`internal/ledger/ledger.go:132-134`), exposing granular REST reads under `/api/v1/` and an SSE event stream (`/api/v1/events/stream`). `internal/ledger/ledger.go` adds cursor tailing and run listing queries (`EventsSince`, `IntegrationEventsSince`, `Runs`), while `internal/serve/server.go` (`internal/serve/server.go:19-28`) and `cmd/lucind-ai/cli.go` (`cmd/lucind-ai/cli.go:674-725`) wire these endpoints under strict loopback binding with linked-worktree refusal.

## Flow and Invariants

    Client ──(1: Request)──→ ServeMux (handlers.go:36) ──(2: Query/Tx)──→ Model / Ledger (model.go:128, ledger.go:285) ──(3: SQLite)──→ Response (4: JSON/SSE) ──→ Client

- **Hop 1 (Client → ServeMux)**: Prefix `/api/` maps to JSON; unmatched `/api/*` returns JSON 404 (`internal/serve/handlers.go:39-77`). SSE requires `text/event-stream` and `http.Flusher`. Path-bound mutations require `{runID}/{laneID}` (`internal/serve/handlers.go:87-115`) and reject bulk JSON (`internal/serve/handlers.go:161-176`). Startup checks `serve.IsLoopback` (`internal/serve/server.go:55`) and `worktree.IsLinkedWorktree` (`cmd/lucind-ai/cli.go:702`). Observably breaks: HTML 404s, public exposure, bulk mutation, or hanging streams.
- **Hop 2 (ServeMux → Model/Ledger)**: Handlers invoke shell-free query methods on `*serve.Model` (`internal/serve/model.go:128-343`) and `*ledger.Ledger` (`internal/ledger/ledger.go:285-330,705-717`) using `r.Context()`. SSE polling loop binds to `r.Context().Done()`. Observably breaks: raw SQL execution, leaked goroutines, or database locks across HTTP requests.
- **Hop 3 (Model/Ledger → SQLite WAL)**: Read queries execute short indexed SELECTs (`internal/ledger/schema.go:18-180`). SSE queries tail `events` and `integration_events` via monotonic `id > ?` cursors. Mutations execute atomic UPDATE + `AppendEvent` (`internal/ledger/ledger.go:448-486,615-640`). Observably breaks: `SQLITE_BUSY` contention with `ExecuteBatch` (`internal/run/batch.go:29-53`), missed events, or desynchronized state.
- **Hop 4 (ServeMux → Client)**: REST emits HTTP 200 JSON with non-nil arrays (`internal/serve/handlers.go:126-128`). SSE flushes frames via `http.Flusher.Flush()`. Mutations return `{"ok":true}` (409 if decided, 404 if unknown). Observably breaks: client JSON parse errors, buffered SSE frames, or duplicate approval writes.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `serve.NewHandler` | `internal/serve/handlers.go:36` | Signature adds `m *Model` parameter | No; signature change updated across callers. |
| `GET /api/v1/runs` | `internal/serve/handlers.go:36-118` | Adds route returning `[]string` of run IDs via `ledger.Runs(ctx)` | Yes; additive REST endpoint. |
| `GET /api/v1/lanes` | `internal/serve/handlers.go:36-118` | Adds route with `?run_id=<id>` returning `[]ledger.Lane` via `ledger.Lanes` (`internal/ledger/ledger.go:285`) | Yes; additive REST endpoint. |
| `GET /api/v1/features` | `internal/serve/handlers.go:36-118` | Adds route returning `[]serve.Feature` via `m.ListFeatures` (`internal/serve/model.go:128`) | Yes; additive REST endpoint. |
| `GET /api/v1/features/{id}` | `internal/serve/handlers.go:36-118` | Adds route returning `serve.Feature` via `m.GetFeature` (`internal/serve/model.go:152`) | Yes; additive REST endpoint. |
| `GET /api/v1/reconciliations` | `internal/serve/handlers.go:36-118` | Adds route with `?feature_id=<id>` returning `[]serve.ReconciliationRequest` (`internal/serve/model.go:278`) | Yes; additive REST endpoint. |
| `GET /api/v1/approvals` | `internal/serve/handlers.go:36-118` | Adds route returning `[]ledger.Approval` via `ledger.PendingApprovals` (`internal/ledger/ledger.go:705`) | Yes; additive REST endpoint. |
| `GET /api/v1/events/stream` | `internal/serve/handlers.go:36-118` | Adds SSE route streaming `events` and `integration_events` via `http.Flusher` | Yes; additive SSE endpoint. |
| `GET /api/state` | `internal/serve/handlers.go:79-85` | Unchanged; serves `ServerState` (`internal/serve/handlers.go:16-21`) for UI (`internal/serve/static/app.js:96`) | Yes; preserves existing schema. |
| `POST /approvals/{runID}/{laneID}` | `internal/serve/handlers.go:87-115` | Unchanged; preserves path mutation, anti-bulk 400 (`internal/serve/handlers.go:161-176`), and `{"ok":true}` | Yes; preserves mutation route. |
| `POST /approvals/{runID}/{laneID}/defect` | `internal/serve/handlers.go:103-107` | Unchanged; preserves path defect marking and `{"ok":true}` | Yes; preserves mutation route. |
| Router `/api/*` 404 handler | `internal/serve/handlers.go:39-55` | Unmatched `/api/*` routes return JSON 404; non-API paths serve `embed.FS` (`internal/serve/static.go:8-18`) | Yes; isolates API errors. |
| `Ledger.Runs` | `internal/ledger/ledger.go:284-330` | Adds `func (l *Ledger) Runs(ctx context.Context) ([]string, error)` from `lanes` (`internal/ledger/schema.go:18-32`) | Yes; additive query method. |
| `Ledger.EventsSince` | `internal/ledger/ledger.go:488-525` | Adds `func (l *Ledger) EventsSince(ctx context.Context, lastID int64) ([]Event, error)` (`internal/ledger/schema.go:34-43`) | Yes; additive query method. |
| `Ledger.IntegrationEventsSince` | `internal/ledger/ledger.go:892-928` | Adds `func (l *Ledger) IntegrationEventsSince(ctx context.Context, lastID int64) ([]IntegrationEvent, error)` (`internal/ledger/schema.go:171-180`) | Yes; additive query method. |
| CLI flags `lucind-ai serve` | `cmd/lucind-ai/cli.go:683-685` | Unchanged; `-addr`, `-approver`, `-approval-timeout` flag definitions and defaults preserved | Yes; CLI surface unchanged. |
| SQLite database schema | `internal/ledger/schema.go:10-180` | Unchanged; `schemaVersion` remains 5 (`internal/ledger/schema.go:10`) with zero DDL migrations | Yes; database format unchanged. |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/serve/handlers.go` | Modify | Update `NewHandler` signature to receive `*Model`, register `/api/v1/*` routes and SSE stream, and add JSON 404 routing for `/api/*`. | `cmd/lucind-ai/cli.go:715` (`serveDispatch`), `internal/serve/server.go:26` (`ListenAndServe`), and proposal delta `Granular REST reads` (`openspec/changes/control-room-serve/proposal.md:42-48`). |
| `internal/ledger/ledger.go` | Modify | Add `Runs(ctx context.Context) ([]string, error)`, `EventsSince(ctx context.Context, lastID int64) ([]Event, error)`, and `IntegrationEventsSince(ctx context.Context, lastID int64) ([]IntegrationEvent, error)` methods. | `GET /api/v1/runs` and `GET /api/v1/events/stream` HTTP handlers in `internal/serve/handlers.go:36-118` and proposal delta `SSE stream` (`openspec/changes/control-room-serve/proposal.md:49-55`). |
| `cmd/lucind-ai/cli.go` | Modify | Initialize `serve.NewModel(ledg)` and pass `model` to `serve.NewHandler` in `serveDispatch` (`cmd/lucind-ai/cli.go:715`). | CLI operator running `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`). |
| `internal/serve/server_test.go` | Modify | Update `NewHandler` calls to supply `*serve.Model` (`internal/serve/server_test.go:70,114`), and add tests for `/api/v1/*` REST endpoints, JSON 404s, and SSE streaming with cancellation. | `go test ./internal/serve` test runner validating `control-room-api-runs`, `control-room-events-stream`, and anti-bulk invariants (`openspec/changes/control-room-serve/proposal.md:96-107`). |
| `internal/serve/model_test.go` | Modify | Add tests for feature and reconciliation queries against populated ledger fixtures while preserving shell-free AST isolation (`internal/serve/model_test.go:595-628`). | `TestModelSourceDoesNotShellOut` and Go test suite in `internal/serve/model_test.go:1-628`. |

## Open Questions

- [ ] Whether cross-process SSE event streaming (`GET /api/v1/events/stream`) across `run` and `serve` should poll SQLite using a fixed interval (e.g. 250ms–500ms) or adaptive backoff when idle.
- [ ] Whether future mutation endpoints (such as reconciliation approval via `cmd/lucind-ai/cli.go:1166-1176`) should be exposed over HTTP under `/api/v1/` or remain restricted to CLI subcommands.
- [ ] Precedence note: The 3-lens parallel execution model takes precedence over the single-agent full-design instructions in `~/.claude/skills/sdd-design/SKILL.md`.
