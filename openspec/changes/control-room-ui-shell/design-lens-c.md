# Design Lens C — Failure, Test & Rollback: Control Room UI Shell

## Assumed architecture

The backend extends `serve.NewHandler` (`internal/serve/handlers.go:36-118`) to instantiate `serve.NewModel` (`internal/serve/model.go:21-24`), serve modular static ES assets via `StaticFS` (`internal/serve/static.go:8-18`), and expose read-only GET endpoints backed by `serve.Model` methods (`internal/serve/model.go:128-343`). The frontend in `internal/serve/static/` transitions from monolithic DOM generation in `app.js` (`internal/serve/static/app.js:1-98`) to a modular vanilla ES-module SPA consisting of persistent layout chrome (`index.html:142-158`, `shell.js`), a hash router (`router.js`), a centralized reactive store (`store.js`), and view modules (`views/approvals.js`). Process lifecycle (`internal/serve/server.go:19-53`), loopback binding constraints (`internal/serve/server.go:57-73`), CLI entry points (`cmd/lucind-ai/cli.go:674-725`), ledger schema v5 (`internal/ledger/schema.go:10`), and approval mutation endpoints (`internal/serve/handlers.go:87-211`) remain intact.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit (Static & AST) | Embed delivery, MIME types, forbidden bulk terms, AST no-shell-out | `fs.ReadFile` on `StaticFS()`, substring checks, Go AST parser on `model.go` | `internal/serve/static.go:12-18`, `internal/serve/static_test.go:11-39`, `internal/serve/model_test.go:595-627` |
| Unit (Model DTO) | Model query methods return ledger state without git/shell mutations | Seed temporary SQLite database via write APIs, invoke `serve.Model` methods, assert DTO round-trip | `internal/serve/model.go:21-24`, `internal/serve/model_test.go:22-30`, `internal/ledger/ledger.go:42-58` |
| Integration (HTTP) | Static file routing, `/api/state` polling, Model GET routes, POST approval/defect, 400 on bulk/unselected, 409 conflict | `httptest.NewRecorder` and `httptest.NewServer` against `serve.NewHandler` | `internal/serve/handlers.go:36-118`, `internal/serve/server_test.go:42-93`, `internal/serve/server_test.go:136-236` |
| Integration (Process) | Strict loopback listen enforcement, rejection of `0.0.0.0`, graceful shutdown | `serve.ListenAndServe` with cancelled contexts, `serve.IsLoopback` validation matrix | `internal/serve/server.go:19-53`, `internal/serve/server.go:57-73`, `internal/serve/server_test.go:17-40` |
| Unit (Frontend Invariants) | Inline evidence validation, initial unselected controls, DOM textContent escaping | Static pattern tests in Go over `StaticFS()`; new seam required for browser runtime execution | `internal/serve/static.go:12-18`, `internal/serve/static_test.go:41-67`, `internal/serve/static_test.go:83-102` |

## Test Seams

### Existing Seams
- **HTTP Handler (`internal/serve/handlers.go:36-118`)**: `serve.NewHandler(l, approver, opencodeCmd)` is the top-level HTTP injection seam tested with `httptest.NewRecorder` and `httptest.NewRequest`.
- **Model Query Layer (`internal/serve/model.go:21-24`)**: `serve.NewModel(l)` injects any open `ledger.Ledger` to test shell-free DTO queries directly.
- **Isolated SQLite Ledger (`internal/ledger/ledger.go:42-58`, `internal/serve/model_test.go:22-30`)**: `ledger.Open(ctx, t.TempDir())` provides ephemeral schema v5 databases (`internal/ledger/schema.go:10`) for hermetic testing.
- **Embedded Asset System (`internal/serve/static.go:12-18`)**: `serve.StaticFS()` returns `fs.FS` for testing embedded scripts and markup without disk dependencies.
- **Loopback Enforcement (`internal/serve/server.go:14`, `19-53`, `57-73`)**: `serve.IsLoopback`, `serve.ListenAndServe`, and `serve.ErrNonLoopback` allow testing network binding and graceful cancellation.

### New Seams Required
- **Model GET Routes in `NewHandler` (`internal/serve/handlers.go:39`)**: Route registration mapping `/api/features`, `/api/leases`, `/api/overlap`, `/api/reconciliations`, and `/api/audit` to `serve.Model` query methods, testable via `httptest.NewRecorder`.
- **Modular Static MIME Dispatch (`internal/serve/handlers.go:41-55`)**: Header dispatch mapping `.js` to `application/javascript` and `.css` to `text/css` for modular files in `StaticFS`.
- **Optional Headless JS Runner**: `node --test` or headless browser harness if end-to-end DOM rendering is tested outside Go unit tests.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: UI shell renders evidence strings as plain text and performs no file classification or execution | N/A | None (no execution boundary) |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: `serveDispatch` resolves root once at startup via `resolvePrimaryRoot` (`cmd/lucind-ai/cli.go:696-705`) and issues no git subprocesses | N/A | None (no git subprocess boundary) |
| Commit state | staged, `commit -a`, empty index | N/A: Change performs zero git commits; web mutations only update SQLite approval rows (`internal/serve/handlers.go:87-115`) | N/A | None (no commit boundary) |
| Push state | tracking branch, first push, explicit refspec | N/A: Change performs zero git push operations | N/A | None (no push boundary) |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: Change performs zero PR automation or CLI composition | N/A | None (no PR boundary) |

*Note*: `N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary exists in control-room-ui-shell; AST tests enforce that the model layer does not shell out (`internal/serve/model_test.go:595-627`).*

## Rollback and Additivity

**Choice**: Clean `git revert` of commits introducing the UI shell and Model GET endpoints.
**Alternatives considered**: Database rollback scripts or schema downgrades (rejected: schema remains v5); runtime feature flags (rejected: unnecessary complexity for an embedded SPA).
**Rationale**: The change is purely additive and fully backward-compatible across all interfaces:
1. **Ledger Schema**: Remains at version 5 (`internal/ledger/schema.go:10`); no SQLite DDL migrations or schema bumps occur.
2. **Result Envelope**: Result schema contract (`internal/result/schema.go:1-63`, `result.schema.json`) remains untouched.
3. **Go APIs**: `serve.NewHandler` (`internal/serve/handlers.go:36`) and `serve.StaticFS()` (`internal/serve/static.go:12-18`) retain their existing signatures.
4. **HTTP Endpoints**: Wire format `ServerState` (`internal/serve/handlers.go:16-21`), polling route `/api/state` (`internal/serve/handlers.go:79-85`), and POST `/approvals/{runID}/{laneID}` contracts (`internal/serve/handlers.go:87-115`) remain unchanged.
5. **Additive Routes**: New `/api/features`, `/api/leases`, etc., endpoints are read-only HTTP GET handlers backed by existing `serve.Model` methods (`internal/serve/model.go:128-343`).
6. **Embedded Bundle**: Reverting the commits restores the monolithic `app.js` and `index.html` inbox in `embed.FS` with zero orphaned database state.

## Out of Scope

- Schema migrations or DDL changes (`internal/ledger/schema.go:10`); ledger remains v5.
- Node.js, npm, bundlers (Vite/Webpack), or third-party frontend frameworks (React/Vue/Svelte) (`internal/serve/static.go:8-18`, `docs/prd.md:219-222`).
- Non-loopback network binding, TLS, or remote authentication (`internal/serve/server.go:14-22`, `cmd/lucind-ai/cli.go:691-694`).
- Bulk approval actions or multi-item decision controls (`internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`).
- Browser-initiated git mutations (rebase, commit, push), lease acquire/renew, or reconciliation approve/decline mutations from the UI; mutations stay on the CLI (`cmd/lucind-ai/cli.go:727-750`).
- Specialized rich UI views (Reconciliations, Leases, Timeline, Fleet, DAG Canvas, SDD Flows) owned by downstream change `control-room-ui-views`.
- Real-time Server-Sent Events (`/api/stream` via `http.Flusher`) or WebSockets.

## Open Questions

- [ ] Does this change ship a read-only placeholder Features view (`views/features.js`), or only the shell + Approvals view + Model GET routes, deferring all additional views to `control-room-ui-views`?
- [ ] Should future changes add Server-Sent Events (`http.Flusher`) under `/api/stream` for push updates, or retain tab-scoped HTTP polling indefinitely?
- [ ] Note: The phase execution contract in this packet overrides the single-agent `sdd-design` SKILL.md (800-word limit, full document creation, and Engram persistence) by scoping Lens C specifically to failure, test, threat matrix, and rollback under a 1000-word budget.
