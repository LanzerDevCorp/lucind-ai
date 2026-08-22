# Proposal: Control Room UI Shell

## Intent

`lucind-ai serve` is a loopback HTTP process (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:19-22`) that only paints an approvals inbox (`internal/serve/static/index.html:143-158`). `app.js` rebuilds that list via `innerHTML` (`internal/serve/static/app.js:22-70`) and polls `GET /api/state` every 2000ms (`internal/serve/static/app.js:96-98`). The mux serves `/`, static files, `/api/state` (approver, rate, command, pending), and `POST /approvals/{runID}/{laneID}` including `/defect` (`internal/serve/handlers.go:36-118`, `79-85`, `120-146`). There is no nav, outlet, or view lifecycle.

`serve.Model` already returns features, attempts, leases, overlap, reconciliations, and audit events as JSON DTOs without git or shell (`internal/serve/model.go:14-24`, `26-125`, `128-343`). `NewHandler` never constructs it; those ledger rows (`internal/ledger/schema.go:18-180`) are invisible in the browser. This change makes the same loopback process an embed-only multi-view shell so operators can inspect that telemetry without npm, remote bind, or new web mutations.

## Selected Candidate and Approach

**Candidate 1 — Modular vanilla ES-module SPA**, shipped by `//go:embed static/*` (`internal/serve/static.go:8-18`), with hash routing, a view registry, a shared store, and read-only REST over `serve.Model`.

This change does **not** hook a lane-lifecycle call site (`Execute` / `b.Observe`). It extends `serve.NewHandler` (`internal/serve/handlers.go:36-118`) and the embed pipeline, invoked from `serveDispatch` (`cmd/lucind-ai/cli.go:674-725`).

1. **Layout shell.** Replace the inbox (`internal/serve/static/index.html:141-163`) with persistent header chrome (approver and wrong-approval rate from `ServerState` at `internal/serve/handlers.go:130-141`; connection/freshness is new client chrome), a tab bar, and a `#view-outlet`.
2. **Hash router and view registry.** `#/approvals` (and `#/features` as an example) with `mount(container, store)` / `unmount()`. Navigate tears down listeners and the 2000ms timer (`internal/serve/static/app.js:96-98`) and mounts the next view without reload. Hash needs no server fallback; unknown paths already 404 after static lookup (`internal/serve/handlers.go:39-77`).
3. **Shared store.** `store.js` caches `/api/state` (`internal/serve/handlers.go:79-85`, `internal/serve/static/app.js:1-10`) across switches and notifies subscribers. This change keeps HTTP polling; SSE is not required.
4. **Read-only Model HTTP.** GET routes for existing methods: `ListFeatures`/`GetFeature` (`128-164`), `ListAttempts` (`167-188`), `ListLeases` (`206-227`), `ListOverlapEvidence` (`245-266`), `ListReconciliationRequests` (`278-292`), `ListAuditEvents` (`326-343`). Prefer tab-scoped fetches over a full ledger dump on every 2s tick.
5. **Invariants.** Embed-only (`internal/serve/static.go:8-18`, `Makefile:7-8`, `lucind-checks.sh:1-4`, `docs/prd.md:219-222`). Loopback-only (`internal/serve/server.go:14-22`, `57-73`, `cmd/lucind-ai/cli.go:683-694`). Web writes stay `/approvals/` decide and defect (`internal/serve/handlers.go:87-115`); bulk still 400 (`internal/serve/handlers.go:161-176`, `docs/prd.md:229-240`). New views read-only. Evidence is command output or `file:line` (`internal/serve/static/app.js:12-20`). Patch DOM instead of wiping `innerHTML` (`internal/serve/static/app.js:45-70`). Untrusted `Output` (`internal/serve/model.go:102-115`) uses `textContent` / `escapeHtml` (`internal/serve/static/app.js:91-94`); do not interpolate IDs into `onclick` (`internal/serve/static/app.js:56-65`). Git/lease/reconcile mutations stay on the CLI (`internal/ledger/schema.go:106-170`, `internal/serve/model.go:44,55`).

**Rejected.** Candidate 2 (`html/template` + vendored HTMX) couples markup to Go and fights the JSON DTO API (`internal/serve/handlers.go:120-146`). Candidate 3 (Preact/Solid + Vite) needs Node/npm (`docs/prd.md:219-222`, `Makefile:7-8`).

## Conceptual Changes

- **Inbox → view registry.** One list is hardcoded into `#approvals-container` (`internal/serve/static/index.html:154-158`, `internal/serve/static/app.js:22-70`). Views become mountable modules. This change does not freeze a six-view catalog.
- **Model as HTTP.** GET handlers expose the existing shell-free query surface (`internal/serve/model.go:14-24`) without subprocesses.
- **Store + DOM patching.** Full-container `innerHTML` on a 2s timer (`internal/serve/static/app.js:45-70`, `96-98`) is the scroll/focus bug; a store that outlives views plus targeted patches is the fix.

## Capabilities

Contract for sdd-spec. Existing spec: `openspec/specs/approvals-web-ui/spec.md`.

### New Capabilities
- `control-room-ui-shell`, `control-room-client-routing`, `control-room-shared-store`, `control-room-asset-embed`, `control-room-model-queries` (see table).

### Modified Capabilities
- `approvals-web-ui`: inbox becomes a registered view; loopback, individual decide, evidence, and anti-bulk rules unchanged (`openspec/specs/approvals-web-ui/spec.md:10-25`, `26-66`).

| Capability | Impact | Description | Seam |
|---|---|---|---|
| `control-room-ui-shell` | Added | Header, tabs, outlet. | `internal/serve/static/index.html:142-158`, `internal/serve/handlers.go:39-77` |
| `control-room-client-routing` | Added | Hash router; unmount clears timers/listeners. | `internal/serve/static/app.js:96-98`, `internal/serve/handlers.go:39-55` |
| `control-room-shared-store` | Added | Shared cache; poll `/api/state`. | `internal/serve/static/app.js:1-10`, `internal/serve/handlers.go:79-85` |
| `control-room-asset-embed` | Added | Modular ES modules via `StaticFS`. | `internal/serve/static.go:8-18`, `internal/serve/handlers.go:41-55` |
| `approvals-web-ui` | Modified | Inbox as a view; decide/evidence/bulk kept. | `openspec/specs/approvals-web-ui/spec.md:26-66`, `internal/serve/static/app.js:12-89` |
| `control-room-model-queries` | Added | GET JSON from `Model` methods. | `internal/serve/model.go:14-125`, `internal/serve/model.go:128-343` |

Affected files: `internal/serve/static/` (shell HTML/CSS; split `app.js` into shell, router, store, views), `internal/serve/handlers.go` (static modules + Model GET routes). Unchanged: `static.go` embed API, `serveDispatch` constructing `NewHandler`, schema v5.

## Delta Specifications

### Requirement: Layout Shell and Global Chrome

The server MUST serve a modular SPA shell with a persistent header (connection/freshness, approver, wrong-approval rate), tabs, and a main view outlet.

#### Scenario: SPA shell initial load
- GIVEN `lucind-ai serve` on loopback `127.0.0.1:7433` (`cmd/lucind-ai/cli.go:683-719`, `internal/serve/server.go:19-22`)
- WHEN an operator opens `http://127.0.0.1:7433/` (`internal/serve/handlers.go:39-77`)
- THEN the server MUST return the embedded shell (`internal/serve/static.go:8-18`) with header, tabs, and outlet, without a further reload.

#### Scenario: Global metrics
- GIVEN a client polling the backend (`internal/serve/handlers.go:79-85`, `internal/serve/static/app.js:1-10`)
- WHEN state updates arrive
- THEN the header MUST show connection/freshness, approver identity, and wrong-approval rate (`internal/serve/handlers.go:130-141`, `internal/serve/static/index.html:145-148`).

### Requirement: Client-Side Routing and View Lifecycle

The client MUST implement hash routing and a view registry: unmount, cancel timers/listeners, mount into the outlet, no reload.

#### Scenario: View transition and cleanup
- GIVEN the approvals view with the 2000ms timer (`internal/serve/static/app.js:96-98`)
- WHEN the operator navigates to another registered hash route
- THEN the router MUST unmount approvals, clear those timers/listeners, and mount the target view.

#### Scenario: In-place DOM updates
- GIVEN an active view in the outlet
- WHEN polling updates the store (`internal/serve/static/app.js:1-10`, `internal/serve/handlers.go:79-85`)
- THEN the client MUST patch DOM nodes in place and MUST NOT replace the outlet's entire `innerHTML` (`internal/serve/static/app.js:22-70`).

### Requirement: Centralized Reactive Client Store

The client MUST own state in `store.js` across view transitions, refreshed by polling `/api/state`.

#### Scenario: Shared state across navigation
- GIVEN pending approvals and approver rate in the store (`internal/serve/handlers.go:120-146`, `internal/serve/static/app.js:1-10`)
- WHEN the operator leaves a view and returns
- THEN cached state MUST be available immediately (no mandatory blank render while a fetch is in flight).

#### Scenario: Polling refresh
- GIVEN the shell on loopback
- WHEN the 2000ms timer fires (`internal/serve/static/app.js:96-98`)
- THEN the store MUST `GET /api/state` (`internal/serve/handlers.go:79-85`) and notify subscribers.

### Requirement: Zero-Build Embedded ES Module Delivery

The binary MUST embed HTML, CSS, and vanilla ES modules with `embed.FS`. Assets MUST run without npm, Node, or a bundler.

#### Scenario: Embedded asset resolution
- GIVEN a compiled binary with no Node toolchain (`Makefile:7-8`, `internal/serve/static.go:8-18`)
- WHEN the browser requests `/shell.js`, `/store.js`, `/style.css`
- THEN the server MUST serve them from `StaticFS` (`internal/serve/static.go:12-18`) with `Content-Type` `application/javascript` or `text/css` (`internal/serve/handlers.go:41-55`).

### Requirement: Approvals Inbox View Integration

The approvals view MUST mount in the shell while keeping loopback-only access, inline evidence, and anti-bulk rules.

#### Scenario: Individual approval from the shell
- GIVEN a pending card with valid `file:line` or command-output evidence (`internal/serve/static/app.js:12-20,47-68`, `internal/ledger/schema.go:45-56`)
- WHEN the operator submits one decision (`internal/serve/static/app.js:72-89`)
- THEN the client MUST POST `/approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115`), the server MUST record it (`internal/serve/handlers.go:148-230`), and bulk bodies MUST return 400 (`internal/serve/handlers.go:161-176`, `openspec/specs/approvals-web-ui/spec.md:26-48`).

#### Scenario: Loopback binding enforcement
- GIVEN `lucind-ai serve --addr 0.0.0.0:7433` (`cmd/lucind-ai/cli.go:683-694`)
- WHEN the address is evaluated
- THEN `IsLoopback` MUST be false (`internal/serve/server.go:57-73`) and the command MUST exit with `ErrNonLoopback` (`internal/serve/server.go:14-22`, `openspec/specs/approvals-web-ui/spec.md:10-25`).

### Requirement: Read-Only Model Inspection Endpoints

The handler MUST expose read-only REST over `serve.Model` without running shell or git.

#### Scenario: Query feature and reconciliation status
- GIVEN ledger rows for features, attempts, and reconciliations (`internal/ledger/schema.go:18-180`, `internal/serve/model.go:14-125`)
- WHEN a view `GET`s a Model query route
- THEN the server MUST return JSON from `serve.Model` methods (`internal/serve/model.go:128-343`) without mutating the ledger.

## Risks

| Risk | Likelihood | Mitigation | Seam |
|------|------------|------------|------|
| SQLite `BUSY` / latency if every tab dumps the ledger on a 2s tick while batches write | Med | Tab-scoped GETs; request-scoped deadlines; WAL + `busy_timeout(5000)` already on | `internal/ledger/ledger.go:162-185`, `internal/serve/model.go:128-250`, `internal/run/batch.go:110-180` |
| XSS from agent evidence, diffs, or candidate `Output` | Med | `textContent` / `escapeHtml`; no raw `innerHTML` for untrusted fields; do not interpolate IDs into `onclick` | `internal/serve/static/app.js:56-65`, `internal/serve/static/app.js:91-94`, `internal/serve/model.go:102-115` |
| Loopback erosion / DNS rebinding | High | Keep listen-address `IsLoopback`; reject `0.0.0.0` at CLI. No HTTP `Host` check today | `internal/serve/server.go:14-22`, `internal/serve/server.go:57-73`, `cmd/lucind-ai/cli.go:691-694` |
| Browser git/lease/reconcile writes | High | New views read-only; web writes stay `/approvals/` decide and defect | `internal/serve/handlers.go:87-115` |
| Bulk / "Approve All" | High | Keep 400 on array/bulk fields; static tests forbid bulk controls; items start unselected | `internal/serve/handlers.go:161-176`, `internal/serve/static/app.js:22-70`, `internal/serve/static_test.go:11-39` |
| Poll ticks steal focus/scroll | Med | Patch per card/row; do not wipe outlet `innerHTML` | `internal/serve/static/app.js:22-70`, `internal/serve/static/app.js:96-98` |

## Rollback Plan

`git revert` of the shell commits. New work is embedded static files (`internal/serve/static.go:8-18`) plus additive Model GET routes (`internal/serve/model.go:128-343`). No schema bump (stays 5, `internal/ledger/schema.go:10`), no ledger cleanup. `/api/state`, `POST /approvals/{runID}/{laneID}`, and the result envelope (`internal/result/schema.go:1-68`) stay.

## Test and Validation Impact

| Layer | Coverage | Seam |
|------|----------|------|
| Model DTOs | Keep round-trip and no-shell-out tests; httptest new GET routes serialize `Model` JSON | `internal/serve/model_test.go:74-347`, `internal/serve/model_test.go:595-627` |
| HTTP | Static 200; bulk 400; empty decision 400; decide 200; already-decided 409; Model routes 200 and no writes | `internal/serve/server_test.go:42-93`, `95-134`, `136-194`, `196-236` |
| Loopback | Non-loopback still `ErrNonLoopback` | `internal/serve/server_test.go:17-40`, `internal/serve/server.go:20-22`, `cmd/lucind-ai/cli.go:691-694` |
| Static invariants | No "Approve All"; items unselected; evidence is command output or `file:line` | `internal/serve/static_test.go:11-39`, `41-67`, `69-81`, `83-102` |
| CLI | `--addr` / `--approver` / `--approval-timeout`; refuse linked worktree; shutdown on cancel | `cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:41-52` |

## Scope

### In Scope
- SPA shell, hash router, view registry, shared store, embed-only vanilla modules.
- Approvals as the first registered view, preserving `approvals-web-ui`.
- Read-only HTTP over existing `serve.Model` methods.
- In-place DOM updates and XSS-safe rendering of untrusted fields.

### Out of Scope
- Schema bump or DDL (`internal/ledger/schema.go:10`).
- Node, npm, bundlers, React/Vue/Svelte, CDN (`internal/serve/static.go:8-18`, `docs/prd.md:219-222`).
- Non-loopback bind, multi-tenant auth, TLS (`internal/serve/server.go:14-22`, `cmd/lucind-ai/cli.go:691-694`).
- Bulk approve (`internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`).
- Browser git mutation, rebase, lease acquire/renew, or reconcile approve/decline.
- Terminal emulation; SSE / `/api/stream`; HTML5 History catch-all.
- Specialized Reconciliations, Leases, Timeline, Fleet, DAG Canvas, or SDD Flows views (`control-room-ui-views`).

## Open Questions

- [ ] Does this change also ship a read-only Features view, or only the shell + Approvals + Model GET routes, deferring Features UI to `control-room-ui-views`?
- [ ] Should later History API routing add a wildcard fallback on `NewHandler` (`internal/serve/handlers.go:39-77`)? This change uses hash routes.
- [ ] Keep 2s (optionally tab-scoped) polling, or add stdlib SSE (`http.Flusher`) in a follow-up? Neither `/api/stream` nor `/api/events/stream` exists.
- [ ] Should views subscribe to store slices or to full snapshots?

## Dependencies

Existing `internal/serve`, `internal/ledger` (schema v5), `approvals-web-ui`; Go stdlib `net/http` and `embed` only.

## Success Criteria

- [ ] Assets stay in `embed.FS`; `make install` is the only build; no npm.
- [ ] Hash router mounts and unmounts registered views; chrome persists.
- [ ] Store survives switches and refreshes from `/api/state`. SSE is not required.
- [ ] Chrome shows connection/freshness and the signed-in approver's wrong-approval rate.
- [ ] Individual-decision, evidence, and loopback rules still hold.
- [ ] New Model GET routes are read-only and covered by httptest.
- [ ] Schema version remains 5; result envelope unchanged.
