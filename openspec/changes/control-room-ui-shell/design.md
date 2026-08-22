# Design: Control Room UI Shell

## Technical Approach

Candidate 1: a modular vanilla ES-module SPA shipped by `//go:embed static/*` (`internal/serve/static.go:8-18`), with hash routing, a view registry, a shared store, and read-only REST over existing `serve.Model` methods (`internal/serve/model.go:128-343`). Today's `NewHandler` (`internal/serve/handlers.go:36-118`) never constructs `NewModel`; this change wires it there. `serveDispatch` (`cmd/lucind-ai/cli.go:674-725`) and loopback listen (`internal/serve/server.go:19-53`, `57-73`) stay. Schema stays v5 (`internal/ledger/schema.go:10`); no DDL.

Maps to the proposal requirements: layout shell and chrome; hash router with `mount`/`unmount`; `store.js` polling `GET /api/state` (`internal/serve/handlers.go:79-85`); zero-build embed using the existing `.js`/`.css` MIME map (`internal/serve/handlers.go:41-55`); Model GET routes; approvals as `views/approvals.js` keeping individual `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115`) and bulk 400 (`internal/serve/handlers.go:161-176`).

Rejected: Go `html/template` + HTMX (couples markup to Go; fights JSON DTOs at `internal/serve/handlers.go:120-146`); Preact/Solid + Vite (Node; install is `go install` at `Makefile:7-8`; `docs/prd.md:219-222`).

## Architecture Decisions

### Decision: Zero-build vanilla ES modules via embed.FS

**Choice**: Native `<script type="module">` files (`shell.js`, `router.js`, `store.js`, `views/approvals.js`) in `embed.FS`.
**Alternatives considered**: html/template + HTMX; Preact/Solid + Vite.
**Rationale**: Component boundaries without a JS toolchain. `StaticFS()` (`internal/serve/static.go:8-18`) already serves `.js` / `.css` as `application/javascript` and `text/css` (`internal/serve/handlers.go:41-55`).
**Terminal consumer**: `internal/serve/handlers.go:41-55`, `internal/serve/static.go:8-18`.

### Decision: Client-side hash routing

**Choice**: Hash routes (`#/approvals` and later registered hashes) on `hashchange`.
**Alternatives considered**: HTML5 History API — needs a server wildcard; unknown paths already 404 after static lookup (`internal/serve/handlers.go:39-55`).
**Rationale**: Entirely client-side; mux path rules and loopback bind stay.
**Terminal consumer**: `internal/serve/handlers.go:39-55`.

### Decision: Explicit mount/unmount

**Choice**: Views export `mount(container, store)` and `unmount()`. `unmount()` clears listeners and intervals before the next mount into `#view-outlet`.
**Alternatives considered**: CSS-hide multiple containers (leaks timers; `setInterval(fetchState, 2000)` at `internal/serve/static/app.js:96-98` has no teardown).
**Rationale**: Stops duplicate polls and stale handlers across tab switches.
**Terminal consumer**: `internal/serve/static/app.js:96-98`.

### Decision: Centralized store with HTTP polling

**Choice**: Singleton `store.js` caches `ServerState`, polls `GET /api/state` every 2000ms, notifies subscribers.
**Alternatives considered**: SSE (`http.Flusher`) or WebSockets (neither exists on `internal/serve/handlers.go`).
**Rationale**: Reuses the proven poll; cached state avoids a blank flash on return to a view.
**Terminal consumer**: `internal/serve/handlers.go:79-85`, `internal/serve/static/app.js:1-10`.

### Decision: Targeted DOM node patching

**Choice**: Patch nodes by id (e.g. `#card-${runID}-${laneID}`) with `textContent` and safe attributes. Do not interpolate ids into `onclick` (`internal/serve/static/app.js:56-65`).
**Alternatives considered**: `containerEl.innerHTML = ''` each tick (`internal/serve/static/app.js:45-70`).
**Rationale**: Preserves scroll and focus. Untrusted `Output` (`internal/serve/model.go:109`) and evidence use `textContent` / `escapeHtml` (`internal/serve/static/app.js:91-94`).
**Terminal consumer**: `internal/serve/static/app.js:45-70`.

### Decision: Read-only tab-scoped REST over serve.Model

**Choice**: `NewHandler` constructs `NewModel` (`internal/serve/model.go:21-24`) and registers GET routes wrapping methods at `internal/serve/model.go:128-343` (paths under Interfaces). Git/lease/reconcile mutations stay on the CLI (`cmd/lucind-ai/cli.go:727-750`).
**Alternatives considered**: Dumping every ledger table through `/api/state` (payload size; SQLite load despite WAL/`busy_timeout(5000)` at `internal/ledger/ledger.go:162-185`); browser mutation endpoints.
**Rationale**: Views fetch telemetry on demand without bloating the 2s poll.
**Terminal consumer**: `internal/serve/handlers.go:36-118`.

### Decision: Approvals inbox encapsulation with unchanged safety invariants

**Choice**: Move the inbox into `views/approvals.js`. Keep decide and defect POSTs, loopback bind, evidence checks (`internal/serve/static/app.js:12-20`), and 400 on array bodies or `Approvals`/`Decisions`/`Lanes` (`internal/serve/handlers.go:161-176`).
**Alternatives considered**: Batch decide or multi-select (forbidden by `docs/prd.md:232-234` and `openspec/specs/approvals-web-ui/spec.md:26-48`).
**Rationale**: `approvals-web-ui` still holds; `TestEmbedFSHasNoApproveAllControl` (`internal/serve/static_test.go:11-39`) stays.
**Terminal consumer**: `internal/serve/handlers.go:87-115`, `internal/serve/static_test.go:11-39`.

## Flow and Invariants

```
Browser -- GET / --> mux --> StaticFS (index.html, shell.js, …)
   |
   +-- hashchange --> router -- unmount/mount --> view in #view-outlet
   +-- 2s poll --> store --> GET /api/state
   +-- tab-scoped GET --> view --> Model JSON routes
   +-- one POST --> approvals view --> /approvals/{runID}/{laneID} --> SQLite
```

1. **Listen + embed.** `ListenAndServe` rejects non-loopback (`internal/serve/server.go:14`, `19-22`, `57-73`; `cmd/lucind-ai/cli.go:691-694`). Assets from `embed.FS` (`internal/serve/static.go:8-18`). *Breaks:* `ErrNonLoopback`.
2. **Router lifecycle.** Unmount before mount. *Breaks:* leaked 2000ms timer (`internal/serve/static/app.js:96-98`).
3. **Store + patch.** Poll `/api/state` (`internal/serve/handlers.go:79-85`). Do not wipe outlet `innerHTML` (`internal/serve/static/app.js:45-70`). Escape untrusted strings. *Breaks:* focus loss; XSS.
4. **Model GET.** Read-only JSON from `serve.Model` (`internal/serve/model.go:128-343`). *Breaks:* shell/git from the handler.
5. **Write gate.** Only decide and defect (`internal/serve/handlers.go:87-115`, `104-107`). Bulk/empty → 400 (`internal/serve/handlers.go:161-176`). Header shows approver and wrong-approval rate from `ServerState` (`internal/serve/handlers.go:16-21`, `130-141`); connection/freshness is new client chrome, not a `ServerState` field.

## Interfaces / Contracts

`NewHandler(l, defaultApprover, opencodeCmd)` signature unchanged (`internal/serve/handlers.go:36`). `ServerState` unchanged. `StaticFS()` unchanged. CLI flags `--addr`, `--approver`, `--approval-timeout` unchanged (`cmd/lucind-ai/cli.go:683-686`).

Additive GET JSON (absent on today's mux at `internal/serve/handlers.go:39`):

| Route | Model |
|---|---|
| `GET /api/features` | `[]Feature` (`model.go:27,128`) |
| `GET /api/features/{id}` | `Feature` (`:152`) |
| `GET /api/features/{id}/attempts` | `[]Attempt` (`:38,167`) |
| `GET /api/attempts/{id}` | `Attempt` (`:191`) |
| `GET /api/leases` | `[]Lease` (`:52,206`) |
| `GET /api/features/{id}/lease` | `Lease` (`:230`) |
| `GET /api/features/{id}/overlap` | `[]OverlapEvidence` (`:62,245`) |
| `GET /api/features/{id}/overlap/{hash}` | `OverlapEvidence` (`:269`) |
| `GET /api/features/{id}/reconciliations` | `[]ReconciliationRequest` (`:74,278`) |
| `GET /api/reconciliations/{id}` | `ReconciliationRequest` (`:295`) |
| `GET /api/reconciliations/{id}/candidates` | `[]ReconciliationCandidate` (`:102,304`) |
| `GET /api/candidates/{id}` | `ReconciliationCandidate` (`:316`) |
| `GET /api/features/{id}/events` | `[]AuditEvent` (`:118,326`) |

Writes unchanged: `decideRequest` (`handlers.go:23`), `defectRequest` (`handlers.go:31`).

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/serve/handlers.go` | Modify | `NewModel(l)`; register Model GET routes. MIME for `.js`/`.css` already set (`:41-55`). | `cmd/lucind-ai/cli.go:715` (`serveDispatch`) |
| `internal/serve/static/index.html` | Modify | Replace inbox (`:141-163`) with header (approver, rate), tabs, `#view-outlet`, module entry. | GET `/` (`handlers.go:69-76`) |
| `internal/serve/static/style.css` | Create | Shell, tabs, cards, evidence. | `.css` MIME (`handlers.go:47-48`) |
| `internal/serve/static/shell.js` | Create | Bootstrap store/router; header metrics; default route. | Script site today `index.html:160`; JS MIME (`handlers.go:44-45`) |
| `internal/serve/static/router.js` | Create | Hash registry; unmount/mount. | `shell.js` |
| `internal/serve/static/store.js` | Create | Poll `/api/state` 2000ms; notify. | `shell.js`, `views/approvals.js` |
| `internal/serve/static/views/approvals.js` | Create | Card patch; evidence; POST decide. | `router.js` on `#/approvals` |
| `internal/serve/static/app.js` | Delete | Monolithic script (`:1-98`). | Replaced by `shell.js` |

`static.go` embed glob `static/*` already covers new files. No schema files.

## Testing Strategy

| Layer | What | Approach | Seam |
|---|---|---|---|
| Unit (static/AST) | Embed, MIME, no bulk terms, `model.go` no shell-out | `fs.ReadFile(StaticFS())`; AST on `model.go` | `static.go:12-18`, `static_test.go:11-39`, `model_test.go:595-627` |
| Unit (Model DTO) | Query methods round-trip ledger rows | `ledger.Open(ctx, t.TempDir())` then `NewModel` | `model.go:21-24`, `model_test.go:22-30`, `ledger.go:146` |
| HTTP | Static 200; Model GET JSON; POST decide/defect; bulk/empty 400; 409 | `httptest.NewRequest` + `NewRecorder` vs `NewHandler` | `handlers.go:36-118`, `server_test.go:42-93`, `136-236` |
| Process | Non-loopback fail; cancel shutdown | `ListenAndServe` + `IsLoopback` | `server.go:19-53`, `57-73`, `server_test.go:17-40` |
| Frontend invariants | Evidence, unselected, no `trimmed.includes('\\n')` | Go substring tests over `StaticFS()`; retarget `app.js` reads (`static_test.go:14,57,86`) to new modules | `static_test.go:41-67`, `83-102` |

### Test seams

Existing: `NewHandler`; `NewModel`; `ledger.Open` (`ledger.go:146`); `StaticFS()`; `IsLoopback` / `ListenAndServe` / `ErrNonLoopback` (`server.go:14`).

New: Model GET registration on the mux (`handlers.go:39`), covered by `httptest.NewRecorder`. JS/CSS MIME already exists (`handlers.go:41-55`).

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: UI renders evidence as text; no file classification or execution | N/A | None |
| Git repository selection | `git -C`, relative/absolute paths | N/A: `serveDispatch` resolves root once (`cli.go:696-705`); no git subprocesses | N/A | None |
| Commit state | staged, `commit -a`, empty index | N/A: no git commits; web writes are SQLite approval rows (`handlers.go:87-115`) | N/A | None |
| Push state | tracking branch, first push, refspec | N/A: no push | N/A | None |
| PR commands | `--head`, env prefix, composed commands | N/A: no PR automation | N/A | None |

Existing loopback and anti-bulk checks stay (`server_test.go:17-40`, `42-93`); they are preserved invariants, not new matrix rows. `model_test.go:595-627` keeps the model layer from shelling out.

## Rollback and Additivity

**Choice**: `git revert` of the shell and Model GET commits.
**Alternatives considered**: Schema downgrade (none: still v5 at `internal/ledger/schema.go:10`); runtime feature flags (unneeded for an embed SPA).
**Rationale**: Additive and revertible with no orphaned DB state.

No migration required. Schema version 5 unchanged; no DDL. Result envelope (`internal/result/schema.go:1-62`) unchanged. `NewHandler` and `StaticFS()` signatures unchanged. `ServerState`, `/api/state`, and POST decide/defect unchanged. New GET routes are read-only wrappers of existing Model methods. Revert restores `app.js` and the inbox `index.html` in `embed.FS`.

## Open Questions

- [ ] Ship a read-only Features view in this change, or only shell + Approvals + Model GET routes, deferring Features UI to `control-room-ui-views`?
- [ ] Later History API routing: add a wildcard fallback on `NewHandler` (`internal/serve/handlers.go:39-77`), or keep hash-only?
- [ ] Keep 2s (optionally tab-scoped) polling, or add stdlib SSE (`http.Flusher`) in a follow-up? No `/api/stream` today.
- [ ] Should views subscribe to store slices or to full snapshots?

## Out of Scope

- Schema bump / DDL (`internal/ledger/schema.go:10`).
- Node, npm, bundlers, React/Vue/Svelte (`docs/prd.md:219-222`).
- Non-loopback bind, TLS, remote auth (`internal/serve/server.go:14-22`, `cli.go:691-694`).
- Bulk approve (`internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`).
- Browser git / lease acquire-renew / reconcile approve-decline; those stay on CLI (`cli.go:727-750`).
- Specialized Reconciliations, Leases, Timeline, Fleet, DAG Canvas, SDD Flows views (`control-room-ui-views`).
- SSE / WebSockets / HTML5 History catch-all in this change.
