# Design Lens B — Surface & Flow: Control Room UI Views

## Assumed architecture

The change extends `internal/serve` by mounting modular read-only REST endpoints on `serve.NewHandler` (`internal/serve/handlers.go:36-118`) backed by `*serve.Model` (`internal/serve/model.go:14-24`), while keeping `GET /api/state` and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:148-211`). `*serve.Model` adds batch, DAG wave, and lane query methods (`ListBatchLanes`) over SQLite tables `lanes` (`internal/ledger/schema.go:18-32`) and `events` (`internal/ledger/schema.go:34-43`) without schema migration (`schemaVersion = 5` at `internal/ledger/schema.go:10`). The embedded UI (`internal/serve/static/app.js:1-98`, `internal/serve/static/index.html:1-140`) refactors into five tabbed vanilla JS panels using tiered 2s polling and keyed DOM updates with zero npm dependencies (`docs/prd.md:217-221`).

## Flow and Invariants

```
Operator Browser ──(1) HTTP GET /api/*──→ serve.NewHandler ──(2) Read DTOs──→ serve.Model ──(3) SQL Query──→ SQLite Ledger
        │                                        │
        └──────(4) POST /approvals/*─────────────┘ (Single Decision Only)
```

1. **Browser ──→ `serve.NewHandler` (`internal/serve/handlers.go:36-118`)**:
   - *Invariant*: Read-only HTTP GET on loopback (`internal/serve/server.go:19-28`) or single-item POST to `/approvals/{runID}/{laneID}`.
   - *Observably breaks*: Non-GET returns HTTP 405; non-loopback bind returns `ErrNonLoopback` (`internal/serve/server.go:14-22`); bulk POST bodies return HTTP 400 (`internal/serve/handlers.go:161-176`).

2. **`serve.NewHandler` ──→ `serve.Model` (`internal/serve/model.go:14-24`)**:
   - *Invariant*: Handlers query `serve.Model` without spawning subprocesses or importing `os`/`os/exec` (`internal/serve/model_test.go:595-627`).
   - *Observably breaks*: AST test `TestModelSourceDoesNotShellOut` fails if `model.go` imports `os` or `exec`.

3. **`serve.Model` ──→ SQLite Ledger (`internal/ledger/schema.go:18-179`)**:
   - *Invariant*: Pure SELECT queries over existing tables (`lanes`, `events`, `approvals`, `features`, `integration_attempts`, `feature_leases`, `overlap_evidence`, `reconciliation_requests`, `reconciliation_candidates`, `integration_events`) without schema change (`schemaVersion = 5` at `internal/ledger/schema.go:10`).
   - *Observably breaks*: SQLite query/scan errors return HTTP 500 JSON responses.

4. **`serve.Model` ──→ HTTP Response (`internal/serve/handlers.go:143-145`)**:
   - *Invariant*: DTOs serialize into strict JSON; nil slices serialize as `[]`; timestamps format as RFC3339Nano (`internal/serve/model.go:561-563`).
   - *Observably breaks*: Client JSON deserialization errors.

5. **HTTP Response ──→ DOM Renderer (`internal/serve/static/app.js:22-70`)**:
   - *Invariant*: 2s hot poll updates summary endpoints; heavy payloads load on card expand; text is sanitized via `escapeHtml`/`textContent` (`internal/serve/static/app.js:51-55, 91-94`); DOM updates are keyed in-place (`card-${id}`).
   - *Observably breaks*: XSS if unescaped; scroll and open cards reset if `innerHTML = ''` is used (`internal/serve/static/app.js:45`).

6. **Approval Mutation: Browser ──→ `handleDecide` (`internal/serve/handlers.go:148-211`)**:
   - *Invariant*: Single-item POST (`{"decision":"approved"|"rejected"}`); payloads with arrays or `approvals`/`decisions`/`lanes` keys are rejected (`internal/serve/handlers.go:161-176`).
   - *Observably breaks*: Bulk approval returns HTTP 400 Bad Request without mutating ledger state.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `serve.NewHandler` | `internal/serve/handlers.go:36-118` | Mounts new modular REST routes (`/api/approvals`, `/api/features`, `/api/leases`, `/api/overlap/*`, `/api/reconcile/*`, `/api/batch/*`) | Yes: preserves `/`, `/api/state`, `/approvals/...` signature and routes. |
| `GET /api/approvals` | None (in `internal/serve/handlers.go:79-85`) | Returns approvals `ServerState` (`internal/serve/handlers.go:16-21`) | Yes: additive route. |
| `GET /api/features` | None (in `internal/serve/handlers.go:36-118`) | Returns `[]serve.Feature` (`internal/serve/model.go:27-35, 128-149`) | Yes: additive route. |
| `GET /api/features/{id}/attempts` | None (in `internal/serve/handlers.go:36-118`) | Returns `[]serve.Attempt` (`internal/serve/model.go:38-49, 167-188`) | Yes: additive route. |
| `GET /api/leases` | None (in `internal/serve/handlers.go:36-118`) | Returns `[]serve.Lease` (`internal/serve/model.go:52-59, 206-227`) | Yes: additive route. |
| `GET /api/overlap/{feature_id}` | None (in `internal/serve/handlers.go:36-118`) | Returns `[]serve.OverlapEvidence` (`internal/serve/model.go:62-70, 245-266`) | Yes: additive route. |
| `GET /api/reconcile/requests` | None (in `internal/serve/handlers.go:36-118`) | Returns `[]serve.ReconciliationRequest` (`internal/serve/model.go:74-92, 277-301`) | Yes: additive route. |
| `GET /api/batch/lanes` | None (in `internal/serve/handlers.go:36-118`) | Returns `[]serve.BatchLane` with wave, status, deadline, and barrier info | Yes: additive route. |
| `type BatchLane struct` | None (in `internal/serve/model.go:26-125`) | Adds DTO struct for batch/lane status, worktree, wave, and demotion diagnosis | Yes: additive struct. |
| `Model.ListBatchLanes` | None (in `internal/serve/model.go:128-343`) | Adds `ListBatchLanes(ctx context.Context, runID string) ([]BatchLane, error)` querying `lanes` and `events` | Yes: additive method. |
| `internal/serve/static/app.js` | `internal/serve/static/app.js:1-98` | Refactors single-view poller into 5-panel tab controller with tiered 2s polling and keyed DOM updates | Yes: preserves single approval POST, validation, and escaping. |
| `internal/serve/static/index.html` | `internal/serve/static/index.html:1-140` | Adds 5-panel tab layout and containers (Batch, Approvals, Features, Reconcile, Envelope) | Yes: retains metric header element IDs. |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/serve/handlers.go` | Modify | Route new `/api/*` endpoints over `*serve.Model`; keep `/api/state` and `/approvals/...` | `serve.ListenAndServe` (`internal/serve/server.go:19-28`) serving browser client (`internal/serve/static/app.js:1-98`), verified by `internal/serve/server_test.go:42-93`. |
| `internal/serve/model.go` | Modify | Add `BatchLane` DTO and `ListBatchLanes` method over `lanes` (`internal/ledger/schema.go:18-32`) and `events` (`internal/ledger/schema.go:34-43`) | `serve.NewHandler` (`internal/serve/handlers.go:36-118`) serving `GET /api/batch/lanes`, verified by `internal/serve/model_test.go:595-627`. |
| `internal/serve/static/app.js` | Modify | Implement 5-panel UI controller, tiered 2s polling, on-demand diagnostic fetches, and keyed DOM updates | Browser runtime loaded by `serve.NewHandler` (`internal/serve/handlers.go:43-51`), verified by `internal/serve/static_test.go:11-102`. |
| `internal/serve/static/index.html` | Modify | Add 5-panel tab navigation, summary counters, and CSS styling without npm | Browser DOM loaded by `serve.NewHandler` (`internal/serve/handlers.go:68-76`), verified by `internal/serve/static_test.go:41-75`. |
| `internal/serve/server_test.go` | Modify | Add tests for `/api/*` routes, payload structure, and 400/409 error cases | `go test ./internal/serve` verifying HTTP route handling against test ledger (`internal/serve/server_test.go:42-93`). |
| `internal/serve/model_test.go` | Modify | Add unit tests for `ListBatchLanes` and demotion notes, confirming AST shell-free compliance | `go test ./internal/serve` executing `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`). |
| `internal/serve/static_test.go` | Modify | Add assertions for multi-panel markup, absence of "approve all", and evidence validation | `go test ./internal/serve` executing static asset contract tests (`internal/serve/static_test.go:11-102`). |

## Open Questions

- [ ] May the UI expose HTTP POST endpoints for reconciliation mutations (`approve`, `decline`, `cancel`, `renew`, `resolve`), or must it strictly present copy-paste CLI commands matching `cmd/lucind-ai/cli.go:1044-1065`? (Preserved from `openspec/changes/control-room-ui-views/explore.md:84` and `proposal.md:152`).
- [ ] Should lease and reconciliation countdown timers be formatted using a server-provided `remaining_seconds` field or computed on the client from `expires_at` using a server timestamp offset (`internal/serve/model.go:56, 84, 354-357`)?
- [ ] Should overlap `evidence_json` (`internal/serve/model.go:68`) and candidate checks be rendered as `<pre>` blocks with `escapeHtml` (`internal/serve/static/app.js:53, 91-94`) or via a lightweight, zero-dependency inline diff tokenizer?
- [ ] Note on sdd-design skill contract variance: This artifact adheres to the Lens B parallel decomposition packet contract (omitting architecture decision rationale and testing strategy owned by lenses A and C) per packet instructions, rather than generating a complete monolithic `design.md`.
