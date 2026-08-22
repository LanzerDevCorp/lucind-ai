# Design: Control Room UI Views

## Technical Approach

Candidate 2: modular REST plus lazy vanilla JS panels (`openspec/changes/control-room-ui-views/explore.md:3,20-23`). `serve.NewHandler` (`internal/serve/handlers.go:36-118`) grows additive GET routes over `*serve.Model` (`internal/serve/model.go:14-24`). Loopback bind stays with `ListenAndServe` (`internal/serve/server.go:19-22`). No npm (`docs/prd.md:217-221`). No schema migration: `schemaVersion` stays 5 (`internal/ledger/schema.go:10`); no tables or columns change.

Maps to the proposal deltas: anti-bulk approvals stay on `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:161-176`; `openspec/specs/approvals-web-ui/spec.md:26-48`); feature/lease/overlap/reconcile reads use existing Model methods; batch/lane reads are new Model queries over `lanes` (`internal/ledger/schema.go:18-32`) and `events` (`internal/ledger/schema.go:34-43`). Call site is `NewHandler`, not `ExecuteBatch`.

## Architecture Decisions

### Decision: Modular GET routes over `serve.Model`

**Choice**: Add `/api/approvals`, `/api/features`, `/api/features/{id}/attempts`, `/api/leases`, `/api/overlap/{feature_id}`, `/api/reconcile/requests`, `/api/batch/lanes`. Keep `GET /api/state` (`internal/serve/handlers.go:79-85`) and `POST /approvals/...` (`internal/serve/handlers.go:87-115`).
**Alternatives considered**: Grow `ServerState` (`internal/serve/handlers.go:16-21`) into one dashboard blob; `html/template` postbacks.
**Rationale**: Existing Model methods are already granular (`ListFeatures` `internal/serve/model.go:128-149`, `ListAttempts` `:167-188`, `ListLeases` `:206-227`, `ListOverlapEvidence` `:245-266`, `ListReconciliationRequests` `:277-301`). A 2s `/api/state` poll would retransmit `evidence_json` (`internal/serve/model.go:68`) and candidate `output` (`:109`).
**Terminal consumer**: `serve.NewHandler` (`internal/serve/handlers.go:36-118`).

### Decision: Extend Model, not handler SQL

**Choice**: Add `BatchLane` plus `ListBatchLanes(ctx, runID)` on `*serve.Model`. SELECT `lanes` and `lane_note` events. Compute barrier `Outcome` in-process via `barrier.Evaluate` (`internal/barrier/barrier.go:36-60`).
**Alternatives considered**: SQL inside handler closures; shelling out to CLI or reading worktrees from handlers.
**Rationale**: Model is the shell-free boundary (`internal/serve/model.go:14-16`); `TestModelSourceDoesNotShellOut` forbids `os` / `os/exec` / `git` (`internal/serve/model_test.go:610-626`).
**Terminal consumer**: the new batch handler; AST test `internal/serve/model_test.go:595-627`.

### Decision: Tiered polling

**Choice**: Keep the 2s timer (`internal/serve/static/app.js:96-97`) on `/api/approvals`, `/api/leases`, `/api/batch/lanes`. Fetch `evidence_json`, candidate `output`/`checks`, and audit on card expand.
**Alternatives considered**: Poll every endpoint every 2s; manual refresh only.
**Rationale**: Hot path is pending approvals and lease fences; heavy fields freeze the tab if they ride the timer.
**Terminal consumer**: `internal/serve/static/app.js` timer and expand handlers.

### Decision: Keyed in-place DOM

**Choice**: Patch nodes by `card-${runID}-${laneID}` (`internal/serve/static/app.js:49`). Do not clear the list with `containerEl.innerHTML = ''` (`:45`) on every tick.
**Alternatives considered**: Full `innerHTML` replace; a virtual-DOM library.
**Rationale**: Today's wipe drops scroll, open cards, and focus. Zero npm (`docs/prd.md:217-221`).
**Terminal consumer**: panel render in `internal/serve/static/app.js:22-70`.

### Decision: Single-item POST only

**Choice**: Keep `handleDecide` 400 on array bodies and `approvals`/`decisions`/`lanes` keys (`internal/serve/handlers.go:161-176`). No "approve all" in static assets (`internal/serve/static_test.go:11-38`).
**Alternatives considered**: `/api/approvals/bulk`; client loops of single POSTs.
**Rationale**: PRD §8.3 and `openspec/specs/approvals-web-ui/spec.md:26-48`.
**Terminal consumer**: `handleDecide` (`internal/serve/handlers.go:148-176`).

### Decision: Escape every inserted diagnostic

**Choice**: `escapeHtml` / `textContent` on all dynamic strings (`internal/serve/static/app.js:51-55,91-94`), including candidate `output` (`internal/serve/model.go:109`) and `lane_note` diagnosis (`internal/run/run.go:423-430`).
**Alternatives considered**: Raw `.innerHTML`; server HTML templates.
**Rationale**: Agent output is untrusted.
**Terminal consumer**: `escapeHtml` in `internal/serve/static/app.js:91-94`.

## Flow and Invariants

```
Browser --GET /api/*--> NewHandler --DTO--> Model --SELECT--> SQLite
   |                         |
   +---- POST /approvals/* --+  (one decision)
```

1. **Browser → handler** (`internal/serve/handlers.go:36-118`): GET on loopback, or one-item POST. Break: non-GET on `/` and `/api/state` → 405 (`:57-59,80-82`); non-loopback listen → `ErrNonLoopback` (`internal/serve/server.go:14-22`); bulk POST → 400 (`internal/serve/handlers.go:161-176`).
2. **Handler → Model** (`internal/serve/model.go:14-24`): no `os`/`exec`. Break: `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`).
3. **Model → ledger**: SELECT only on existing tables. Query/scan errors → HTTP 500.
4. **Handler → JSON**: `json.Encoder` as today (`internal/serve/handlers.go:143-145`); nil slices materialize as `[]` (`internal/serve/model.go:137`).
5. **JSON → DOM** (`internal/serve/static/app.js:22-70`): 2s hot poll; lazy heavy payloads; `escapeHtml`; keyed patch. Break: XSS if unescaped; scroll reset if `:45` wipe remains.
6. **Approve**: body `{"decision":"approved"|"rejected"}`. Arrays or bulk keys → 400, no `Decide`.

## Interfaces / Contracts

| Surface | Today | Delta | Compatible? |
|---|---|---|---|
| `NewHandler` | `internal/serve/handlers.go:36-118` | Mounts the GET routes below | Yes: `/`, `/api/state`, `/approvals/` stay |
| `GET /api/approvals` | none (`:79-85` is `/api/state`) | `ServerState` (`:16-21`) | Additive |
| `GET /api/features` | none | `[]Feature` (`internal/serve/model.go:27-35,128-149`) | Additive |
| `GET /api/features/{id}/attempts` | none | `[]Attempt` (`:38-49,167-188`) | Additive |
| `GET /api/leases` | none | `[]Lease` (`:52-59,206-227`) | Additive |
| `GET /api/overlap/{feature_id}` | none | `[]OverlapEvidence` (`:62-70,245-266`) | Additive |
| `GET /api/reconcile/requests` | none | Compose per-feature `ListReconciliationRequests` (`:74-92,277-301`) | Additive |
| `GET /api/batch/lanes` | none | `[]BatchLane`: status (`internal/lane/status.go:10-16`), worktree / preserved (`internal/ledger/schema.go:26-27`), barrier `Outcome` (`internal/barrier/barrier.go:21-29`), `lane_note` (`internal/run/run.go:423-430`) | Additive |
| `BatchLane` / `ListBatchLanes` | none (`internal/serve/model.go:26-125` is feature-parent DTOs only) | New DTO + method | Additive |
| `app.js` / `index.html` | poller `internal/serve/static/app.js:1-97`; body `internal/serve/static/index.html:141-162` | Five panels: Batch/Wave, Approvals, Feature/Lease, Reconciliation, Lane envelope | Yes: keep per-card POST, `isValidEvidence` (`app.js:12-20`), `escapeHtml` |

`ListReconciliationRequests` is per-feature today (`internal/serve/model.go:277-278`); the collection URI composes those calls.

## File Changes

| File | Action | Terminal consumer |
|---|---|---|
| `internal/serve/handlers.go` | Modify: mount GET `/api/*`; keep `/api/state` and `/approvals/` | `ListenAndServe` (`internal/serve/server.go:19-22`) serving `app.js`; `internal/serve/server_test.go:42-93` |
| `internal/serve/model.go` | Modify: `BatchLane`, `ListBatchLanes` | NewHandler batch route; `internal/serve/model_test.go:595-627` |
| `internal/serve/static/app.js` | Modify: five-panel controller, tiered poll, keyed DOM | Browser via `internal/serve/handlers.go:43-51`; `internal/serve/static_test.go:11-102` |
| `internal/serve/static/index.html` | Modify: five-panel layout; keep metric element IDs | Browser via `internal/serve/handlers.go:68-76`; `internal/serve/static_test.go:41-75` |
| `internal/serve/server_test.go` | Modify: new GET routes; keep 400/409 | `go test ./internal/serve` |
| `internal/serve/model_test.go` | Modify: `ListBatchLanes`, demotion notes, AST | `TestModelSourceDoesNotShellOut` (`:595-627`) via `openModelLedger` (`:22-30`) |
| `internal/serve/static_test.go` | Modify: multi-panel markup; no "approve all"; `isValidEvidence` | `StaticFS()` (`internal/serve/static.go:12-18`) |

## Testing Strategy and Test Seams

| Layer | What | Approach | Seam |
|---|---|---|---|
| Unit Model | Existing list methods plus `ListBatchLanes` / demotion notes | Seeded in-memory ledger | `NewModel` (`internal/serve/model.go:22-24`), `openModelLedger` (`internal/serve/model_test.go:22-30`) |
| Unit AST | `model.go` never imports `os`/`os/exec` or invokes git | Import + body scan | `internal/serve/model_test.go:595-627` |
| HTTP | New GET 200 + JSON; `/api/state` unchanged; bulk POST 400 | `httptest` against `NewHandler` | `internal/serve/handlers.go:36-118`; bulk cases `internal/serve/server_test.go:42-93` |
| Static | No approve-all; unselected cards; `isValidEvidence` rejects bare prose | `fs.ReadFile` on embed | `StaticFS()` (`internal/serve/static.go:12-18`); `internal/serve/static_test.go:11-102` |
| Observation | Status, worktree, and barrier DTO mapping without `ExecuteBatch` | Ledger lane rows + `barrier.Evaluate` | `internal/barrier/barrier.go:21-60`; `internal/lane/status.go:10-28` |

Existing injectables: `NewHandler(l, approver, cmd)` (`internal/serve/handlers.go:36`); `NewModel(l)` (`internal/serve/model.go:22`); `StaticFS()` (`internal/serve/static.go:12`); `barrier.Evaluate` (`internal/barrier/barrier.go:36`); `feature.NewService(l)` (`internal/feature/feature.go:94`); `reconcile.NewService(l, WithClock)` (`internal/reconcile/reconcile.go:128,157`).

New: `ListBatchLanes`. Whether `NewHandler` grows a `*Model` argument or calls `NewModel(l)` internally is open.

## Threat Matrix

This change adds HTTP GET routes and static panels. It does not classify files, select git repos, commit, push, or compose PR argv. Loopback bind and TLS stay with `control-room-serve`.

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: handlers and Model do not classify or execute paths | — | None |
| Git repository selection | N/A: Model reads SQLite only (`internal/serve/model_test.go:610-626`) | — | None |
| Commit state | N/A: no commit construction | — | None |
| Push state | N/A: no push | — | None |
| PR commands | N/A: no PR argv | — | None |

Anti-bulk POST 400 and no "approve all" remain product tests (`internal/serve/server_test.go:42-93`; `internal/serve/static_test.go:11-38`), not threat-matrix rows.

## Rollback and Additivity

**Choice**: `git revert` the commit.
**Alternatives considered**: Feature flags; migration down-scripts.
**Rationale**: Additive GET routes, Model reads, and static JS/HTML. `schemaVersion` remains 5 (`internal/ledger/schema.go:10`). `GET /api/state` (`internal/serve/handlers.go:79-85`) and `POST /approvals/{runID}/{laneID}` (`:87-115,148-211`) keep their wire formats. Revert restores the single-panel UI; no orphan ledger rows; CLI reconcile (`cmd/lucind-ai/cli.go:1044-1065`) is untouched.

## Open Questions

- [ ] May the UI POST reconcile `approve`/`decline`/`cancel`/`renew`/`resolve`, or only show copy-paste CLI matching `cmd/lucind-ai/cli.go:1044-1065`? `NewHandler` has no reconcile routes (`internal/serve/handlers.go:36-118`). Escalated from explore/proposal. This change's reconcile surface is read-only until answered.
- [ ] Countdown: Model `remaining_seconds`, or `expires_at` plus server timestamp (`internal/serve/model.go:56,84,354-357`)?
- [ ] Overlap `evidence_json` (`internal/serve/model.go:68`): `<pre>` + `escapeHtml` (`internal/serve/static/app.js:53,91-94`), or a zero-dependency tokenizer?
- [ ] `NewHandler(*ledger.Ledger, ...)` (`internal/serve/handlers.go:36`; tests at `internal/serve/server_test.go:70`) vs constructing `NewModel(l)` internally vs a new `*Model` parameter.

## Out of Scope

- Schema migrations and ledger writes — `control-room-ledger` (`internal/ledger/schema.go:10,18-180`).
- Listen address, TLS, process lifecycle — `control-room-serve` (`internal/serve/server.go:16-73`).
- Shell chrome and CSS tokens — `control-room-ui-shell` (`internal/serve/static/index.html:8-17`).
- Live logs / pty / telemetry — `control-room-capture`, `control-room-telemetry`.
- npm, TypeScript, React/Vue/Svelte (`docs/prd.md:217-221`).
- Reading `.lucind/result.json` from a worktree (`internal/serve/model_test.go:610-618`).
- Reconciliation mutation (transport is the first open question).
