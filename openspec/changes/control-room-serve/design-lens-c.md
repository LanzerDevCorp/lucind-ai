# Design Lens C — Failure, Test & Rollback: Control Room Serve Subsystem

## Assumed architecture

The serve subsystem extends `internal/serve/handlers.go` by parameterizing `NewHandler` with `*serve.Model` alongside `*ledger.Ledger`, registering read-only JSON routes (`/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconciliations`, `/api/v1/approvals`) and an SSE stream (`/api/v1/events/stream`). `internal/ledger/ledger.go` is extended with incremental cursor event queries (`EventsSince`, `IntegrationEventsSince`) and a distinct runs query. `internal/serve/server.go` retains strict loopback binding (`IsLoopback`), a 3s graceful shutdown timeout, and omitted global `WriteTimeout`, while `cmd/lucind-ai/cli.go:serveDispatch` preserves the linked-worktree refusal before opening the ledger.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit (HTTP REST) | `GET /api/v1/runs`, `GET /api/v1/lanes`, `GET /api/v1/features`, `GET /api/v1/reconciliations`, `GET /api/v1/approvals` return HTTP 200 JSON | `httptest.NewRecorder` + `httptest.NewRequest` against handler with populated ledger | `internal/serve/handlers.go:36-118`, `internal/serve/model.go:128-343` |
| Unit (Routing & 404) | Missing `/api/*` routes return JSON 404; static UI routes return `index.html` with proper MIME headers | `httptest.NewRecorder` asserting JSON on 404 and MIME types on assets | `internal/serve/handlers.go:39-77`, `internal/serve/static_test.go:11-103` |
| Unit (Security) | `serve.ListenAndServe` and `serve.IsLoopback` reject non-loopback addresses (`0.0.0.0`, remote IPs) | Table-driven `ListenAndServe` assertions expecting `serve.ErrNonLoopback` | `internal/serve/server.go:19-73`, `internal/serve/server_test.go:17-40` |
| Unit (Anti-Bulk) | `POST /approvals/{runID}/{laneID}` rejects JSON arrays, composite objects, empty decisions with 400 | `httptest.NewRequest` with malformed payloads asserting HTTP 400 and unchanged ledger | `internal/serve/handlers.go:87-115,148-211`, `internal/serve/server_test.go:42-135` |
| Unit (Concurrency) | Duplicate `POST /approvals/{runID}/{laneID}` returns HTTP 409 Conflict | `httptest.NewRequest` executing repeated decision posts | `internal/serve/handlers.go:195-206`, `internal/serve/server_test.go:196-236` |
| Unit (AST Audit) | `internal/serve/` package never imports `os/exec` or calls `git` | `go/parser` static AST inspection of `internal/serve/*.go` | `internal/serve/model_test.go:595-628` |
| Integration (SSE Lifecycle) | `GET /api/v1/events/stream` streams SSE frames (`events`, `integration_events`) and exits on context cancel | `httptest.Server` + `http.Flusher` streaming while cancelling client context | `internal/serve/server.go:19-53`, `internal/ledger/ledger.go:490-525,892-925` |
| Integration (CLI Worktree) | `lucind-ai serve` aborts with exit 1 inside linked worktrees before `ledger.Open` | CLI execution in linked worktree asserting early exit and zero ledger creation | `cmd/lucind-ai/cli.go:702-707`, `cmd/lucind-ai/cli_test.go:1908-1930` |
| Stress (WAL Concurrency) | Zero `SQLITE_BUSY` errors during concurrent `run.ExecuteBatch` writes and high-frequency REST reads | Background batch worker writing updates while concurrent clients query REST endpoints | `internal/ledger/ledger.go:162-185`, `internal/run/batch.go:29-53`, `internal/serve/model.go:128-343` |

## Test Seams

Existing seams:
- **HTTP Dispatch**: `serve.NewHandler` via `httptest.NewRequest` / `httptest.NewRecorder` (`internal/serve/server_test.go:70-77,114-120`).
- **Ledger Fixture**: `ledger.Open(ctx, t.TempDir())` initializing ephemeral SQLite databases in WAL mode (`internal/serve/server_test.go:44-49`).
- **Query DTO Surface**: `serve.NewModel(l)` methods querying features, attempts, leases, and reconciliations (`internal/serve/model.go:21-24,128-343`).
- **Loopback Validator**: `serve.IsLoopback` and `serve.ListenAndServe` enforcing local bindings (`internal/serve/server.go:19-73`).
- **Linked Worktree Guard**: `worktree.IsLinkedWorktree(primaryRoot)` check before database open (`cmd/lucind-ai/cli.go:702-705`).
- **AST Import Audit**: Static AST inspection preventing shell/exec imports (`internal/serve/model_test.go:595-628`).

New seams required:
- `serve.NewHandler(l *ledger.Ledger, m *serve.Model, defaultApprover, opencodeCmd string)` in `internal/serve/handlers.go:36-38`: Injects `*serve.Model` into the HTTP router.
- `Ledger.EventsSince(ctx context.Context, lastID int64) ([]Event, error)` and `Ledger.IntegrationEventsSince(ctx context.Context, lastID int64) ([]IntegrationEvent, error)` in `internal/ledger/ledger.go:490-525,892-925`: Cursor queries for SSE flusher loop.
- `Ledger.Runs(ctx context.Context) ([]string, error)` in `internal/ledger/ledger.go:285-330`: Exposes distinct run IDs from `lanes` for `GET /api/v1/runs`.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: control-room-serve only serves HTTP endpoints and static web assets; no file execution boundary | Classification and execution boundary | None |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: control-room-serve reads state from SQLite and rejects linked worktrees; no git invocations | Repository/cwd authority | None |
| Commit state | staged, `commit -a`, empty index | N/A: control-room-serve is a read-mostly HTTP server; no commit creation or index inspection | Index/worktree semantics | None |
| Push state | tracking branch, first push, explicit refspec | N/A: control-room-serve does not push refs or interact with remotes | Destination/ref resolution | None |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: control-room-serve does not compose or run PR commands | Argument composition and ownership | None |

*Summary*: N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary exists in `control-room-serve`.

## Rollback and Additivity

**Choice**: Standard `git revert` of commits modifying `internal/serve/`, `cmd/lucind-ai/cli.go`, and helper queries in `internal/ledger/ledger.go`.
**Alternatives considered**: Feature flags or side-by-side binaries. Rejected because changes introduce zero persistent SQLite schema migrations or format shifts; `git revert` immediately restores prior behavior.
**Rationale**: All changes are strictly additive:
- **Database Schema**: Zero DDL migrations. `schemaVersion` remains 5 (`internal/ledger/schema.go:10`). All endpoints query existing tables (`lanes`, `events`, `approvals`, `features`, `reconciliation_requests`, `integration_events` at `internal/ledger/schema.go:18-180`).
- **HTTP Wire API**: `GET /api/state` (`internal/serve/handlers.go:79-85`) and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115`) maintain exact schemas. New endpoints (`/api/v1/*`, `/api/v1/events/stream`) use new additive paths.
- **Envelope & Packets**: Packet definitions (`internal/packet/packet.go:32-47`) and result schema (`.lucind/result.schema.json`) remain unchanged.

## Out of Scope

- PTY process capture and terminal multiplexing (owned by `control-room-capture`).
- SQLite schema migrations, DDL evolution, and index modifications (owned by `control-room-ledger`).
- Telemetry metrics, token accounting, and cost tracking (owned by `control-room-telemetry`).
- Frontend UI components, CSS styling, and dashboard rendering (owned by `control-room-ui-shell` and `control-room-ui-views`).
- Multi-user authentication, remote network binding, and RBAC (`internal/serve/server.go:14,20-22`).
- Modifying batch DAG execution or scheduling logic (`internal/run/batch.go:29-53`).
- Candidate selection (owned by Lens A) and data-flow diagrams/signature deltas (owned by Lens B).

## Open Questions

- [ ] Precedence note: Three-lens parallel design fan-out takes precedence over `~/.claude/skills/sdd-design/SKILL.md` single-agent layout.
- [ ] SSE polling cadence: Should `GET /api/v1/events/stream` query SQLite event tables via a fixed timer (e.g. 250ms) or adaptive backoff across independent `run` and `serve` processes?
- [ ] HTTP mutation surface scope: Should workflow actions beyond approvals (e.g. reconciliation approval via `cmd/lucind-ai/cli.go:1166-1176`) be exposed over HTTP by default or gated behind a CLI flag?
- [ ] Dev asset reloading: Should `lucind-ai serve` accept an optional `--dev-static-dir` flag (`cmd/lucind-ai/cli.go:675-689`) to bypass `embed.FS` during UI development?
