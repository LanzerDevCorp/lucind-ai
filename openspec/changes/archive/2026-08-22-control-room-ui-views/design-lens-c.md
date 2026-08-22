# Design Lens C — Failure, Test & Rollback: Control Room UI Views

## Assumed architecture

`internal/serve/handlers.go` is extended to route read-only modular REST endpoints (`/api/approvals`, `/api/features`, `/api/features/{id}/attempts`, `/api/leases`, `/api/overlap/{feature_id}`, `/api/reconcile/requests`, `/api/batch/lanes`) backed by `serve.Model`, while keeping `GET /api/state` and `POST /approvals/{runID}/{laneID}`. `internal/serve/model.go` is extended with batch/lane DTOs (`ListBatchLanes`) and demotion diagnosis readers without adding `os`, `exec`, or `git` imports. `internal/serve/static/` is modularized into vanilla JS panels with 2s polling for hot status and lazy fetch for large payloads (evidence, candidate outputs), using zero npm dependencies.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit (Model DTOs) | Query methods (`ListFeatures`, `ListAttempts`, `ListLeases`, `ListOverlapEvidence`, `ListReconciliationRequests`, `ListBatchLanes`) return typed JSON structs from SQLite | Round-trip tests using seeded in-memory ledger | `internal/serve/model.go:21-24` via `openModelLedger` (`internal/serve/model_test.go:22-30`) |
| Unit (AST Safety) | `internal/serve/model.go` never imports `os`, `os/exec`, or executes `git` subprocesses | AST parser and import scan in unit test | `internal/serve/model_test.go:595-627` (`TestModelSourceDoesNotShellOut`) |
| Integration (HTTP REST) | New `/api/*` GET endpoints return 200 OK and expected JSON DTOs; `/api/state` remains backward-compatible | `httptest.NewRecorder` and `httptest.NewRequest` against `serve.NewHandler` | `internal/serve/handlers.go:36-118` (`NewHandler`) |
| Integration (Anti-Bulk Security) | Multi-item or array payloads to `/approvals/...` return HTTP 400 and reject batch decisions | POST payload assertion with JSON arrays/objects | `internal/serve/handlers.go:161-176`, tested in `internal/serve/server_test.go:42-93` |
| Integration (Static Safety) | Embedded assets contain no "approve all" controls, items start unselected, and `isValidEvidence` rejects bare prose | `io/fs.ReadFile` assertions on embedded assets | `internal/serve/static.go:12-14` (`StaticFS`), tested in `internal/serve/static_test.go:11-102` |
| Integration (Barrier / Batch View) | DAG wave grouping, lane statuses, preserved worktrees, and barrier outcomes render correctly without running batches | Construct ledger lane states and verify DTO mapping against barrier logic | `internal/barrier/barrier.go:21-60` (`Outcome`, `Evaluate`), `internal/lane/status.go:10-28` |

## Test Seams

- **Existing Injectable / Fakeable Seams**:
  - `serve.NewHandler(l *ledger.Ledger, defaultApprover string, opencodeCmd string)` (`internal/serve/handlers.go:36`): Mounts all HTTP routes against an isolated SQLite ledger instance.
  - `serve.NewModel(l *ledger.Ledger)` (`internal/serve/model.go:23`): Encapsulates read-only ledger queries, testable without network or subprocess infrastructure.
  - `serve.StaticFS()` (`internal/serve/static.go:13`): Exposes embedded filesystem assets (`embed.FS`) for unit assertions on HTML/JS safety rules.
  - `barrier.Evaluate(expected []string, observed []lane.State) Outcome` (`internal/barrier/barrier.go:36`): Pure in-memory join over lane states, testable without ledger, clock, or I/O.
  - `feature.NewService(l)` (`internal/feature/feature.go:48`) and `reconcile.NewService(l, WithClock)` (`internal/reconcile/service.go:26`): Seed features, attempts, leases, and reconciliation requests into test ledgers.
- **New Seams Required**:
  - `serve.Model.ListBatchLanes(ctx context.Context, runID string) ([]BatchLane, error)` (new method in `internal/serve/model.go:26`): Shell-free batch wave and lane queries over `lanes` and `events` tables.
  - `serve.NewHandler` extension or `serve.NewHandlerWithModel(m *Model, ...)` (new seam in `internal/serve/handlers.go:36`): Injects `*Model` into HTTP handler routing.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: UI dashboard and HTTP handlers perform no file classification or execution | N/A — no path classification boundary | None |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: UI layer reads SQLite ledger only; AST test guarantees shell-free queries | N/A — no git subprocess boundary | None |
| Commit state | staged, `commit -a`, empty index | N/A: UI layer does not construct or manipulate git commits | N/A — no commit boundary | None |
| Push state | tracking branch, first push, explicit refspec | N/A: UI layer performs no git push or remote refspec resolution | N/A — no push boundary | None |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: UI layer does not compose or execute VCS/PR automation commands | N/A — no PR automation boundary | None |

`N/A — no such boundary`: `control-room-ui-views` is a localhost-only read dashboard and approvals HTTP handler over SQLite `*ledger.Ledger`. It performs no routing, shell, subprocess, VCS/PR automation, or executable-file classification (verified by AST guard in `internal/serve/model_test.go:595-627`).

## Rollback and Additivity

**Choice**: Single git commit revert (`git revert <commit-sha>`).
**Alternatives considered**: Feature flags or database migration down-scripts. Rejected because this change introduces zero schema modifications and is entirely additive.
**Rationale**:
- **Schema & Ledger Additivity**: Schema version remains unchanged at `schemaVersion = 5` (`internal/ledger/schema.go:10`). No tables, columns, or indexes are modified or added.
- **Wire Protocol Compatibility**: Existing routes `GET /api/state` (`internal/serve/handlers.go:79-85`) and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115`) remain functional with unchanged wire formats. All new `/api/*` endpoints are additive.
- **Reversion Safety**: Reverting the commit removes the new HTTP routes, Model read methods, and multi-panel static assets, restoring the single-panel approvals UI without leaving orphan ledger data or breaking CLI operations.

## Out of Scope

- Schema migrations and ledger write operations (`internal/ledger/schema.go:10, 18-180`, owned by `control-room-ledger`).
- Server lifecycle, loopback listener binding, and TLS configuration (`internal/serve/server.go:16-73`, owned by `control-room-serve`).
- Global UI shell chrome, navigation framework, and design tokens (`internal/serve/static/index.html:8-17`, owned by `control-room-ui-shell`).
- Shelling out to Git or reading `.lucind/result.json` from the filesystem (prohibited by `internal/serve/model_test.go:595-627`).
- Reconciliation mutation via web UI (deferred; UI renders copy-paste CLI commands per `cmd/lucind-ai/cli.go:1044-1065`).
- Frontend build pipelines, npm packages, TypeScript, and SPA frameworks (`docs/prd.md:217-221`).

## Open Questions

- [ ] Render overlap `evidence_json` (`internal/serve/model.go:68`) as a sanitized `<pre>` block or an inline zero-dependency visual diff tokenizer?
- [ ] Format countdown timers using server-computed `remaining_seconds` or client-side calculation against `expires_at` and `server_time` (`internal/serve/model.go:56, 84, 354-357`)?
- [ ] Should `serve.NewHandler` take `*Model` directly (breaking signature) or construct `NewModel(l)` internally to preserve backward compatibility with existing tests (`internal/serve/server_test.go:70`)?
