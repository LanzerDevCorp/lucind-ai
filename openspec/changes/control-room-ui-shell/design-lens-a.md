# Design Lens A — Decisions: Control Room UI Shell

## Assumed architecture

The backend in `internal/serve/handlers.go:36-118` extends `serve.NewHandler` to instantiate `serve.NewModel` (`internal/serve/model.go:21-24`), serve modular static ES assets via `StaticFS` (`internal/serve/static.go:8-18`), and expose read-only GET endpoints backed by `serve.Model` query methods (`internal/serve/model.go:128-343`). The frontend in `internal/serve/static/` transitions from monolithic DOM generation in `app.js` (`internal/serve/static/app.js:1-98`) to a modular vanilla ES-module SPA consisting of persistent layout chrome (`index.html:142-158`, `shell.js`), a hash router (`router.js`), a centralized reactive store (`store.js`), and view modules (`views/approvals.js`). Process lifecycle (`internal/serve/server.go:19-53`), loopback binding constraints (`internal/serve/server.go:57-73`), CLI entry points (`cmd/lucind-ai/cli.go:674-725`), ledger schema v5 (`internal/ledger/schema.go:10`), and approval mutation endpoints (`internal/serve/handlers.go:87-211`) remain intact.

## Technical Approach

The change refactors the single-page approvals inbox into an extensible multi-view control room shell while adhering to zero-build embedded delivery:

1. **Layout Shell & Chrome** (`Requirement: Layout Shell and Global Chrome`): `index.html` and `shell.js` provide persistent header chrome displaying approver identity, wrong-approval rate (`internal/serve/handlers.go:130-141`), connection/freshness status, navigation tabs, and `#view-outlet`.
2. **Client Routing & Lifecycle** (`Requirement: Client-Side Routing and View Lifecycle`): A hash router (`router.js`) handles `hashchange` events, mounting active views into `#view-outlet` and executing `unmount()` teardown to clear timers and listeners.
3. **Shared Store** (`Requirement: Centralized Reactive Client Store`): `store.js` caches `ServerState` across routes, maintains a 2000ms poll to `GET /api/state` (`internal/serve/handlers.go:79-85`), and notifies subscribed views.
4. **Embedded Asset Delivery** (`Requirement: Zero-Build Embedded ES Module Delivery`): Native ES modules are served from `StaticFS` (`internal/serve/static.go:8-18`) with `application/javascript` and `text/css` MIME types (`internal/serve/handlers.go:41-55`) without Node or bundlers.
5. **Model REST Routes** (`Requirement: Read-Only Model Inspection Endpoints`): `NewHandler` adds read-only GET routes mapping to `serve.Model` methods (`internal/serve/model.go:128-343`), returning JSON without subprocesses.
6. **Approvals Integration** (`Requirement: Approvals Inbox View Integration`): Encapsulated as `views/approvals.js`, preserving individual decisions (`internal/serve/handlers.go:87-115`), 400 bulk rejection (`internal/serve/handlers.go:161-176`), and inline evidence validation (`internal/serve/static/app.js:12-20`).

## Decision 1 — Zero-Build Vanilla ES Modules via embed.FS

**Choice**: Native browser ES modules (`<script type="module">`) split into modular files (`shell.js`, `router.js`, `store.js`, `views/approvals.js`) embedded via `embed.FS` (`internal/serve/static.go:8-18`).
**Alternatives considered**: Go `html/template` + HTMX (rejected: couples markup to Go and conflicts with JSON DTOs in `internal/serve/handlers.go:120-146`); Preact/Solid + Vite (rejected: violates no-npm/no-Node constraint in `Makefile:7-8` and `docs/prd.md:219-222`).
**Rationale**: Native ES modules provide clean component boundaries without requiring an external JavaScript toolchain.
**Terminal consumer**: `internal/serve/handlers.go:41-55` (`StaticFS` route handler with MIME types) and `internal/serve/static.go:8-18` (`StaticFS()`).

## Decision 2 — Client-Side Hash Routing (#/route)

**Choice**: Hash-based routing (`#/approvals`, `#/features`) listening on `hashchange` events.
**Alternatives considered**: HTML5 History API (rejected: requires server wildcard fallback routing in `internal/serve/handlers.go:39-77`, which currently returns 404 for unknown paths).
**Rationale**: Hash routing operates entirely client-side without modifying loopback HTTP path resolution or requiring server URL rewrites.
**Terminal consumer**: `internal/serve/handlers.go:39-55` (preserves 404 behavior) and `internal/serve/static/index.html:142-158` (nav anchors).

## Decision 3 — Explicit mount/unmount Lifecycle Contract

**Choice**: Views export `mount(container, store)` and `unmount()` methods. `unmount()` removes DOM listeners and clears intervals before next mount into `#view-outlet`.
**Alternatives considered**: Multi-container CSS toggling (`display: none`) (rejected: leaks memory and keeps dormant timers polling in background).
**Rationale**: Explicit teardown prevents memory leaks, duplicate network requests, and timer collisions across navigation.
**Terminal consumer**: `internal/serve/static/app.js:96-98` (clears orphaned `setInterval(fetchState, 2000)`).

## Decision 4 — Centralized Store with HTTP Polling

**Choice**: Singleton `store.js` caching `ServerState`, polling `GET /api/state` every 2000ms, and notifying subscribers.
**Alternatives considered**: Server-Sent Events (`/api/stream` via `http.Flusher`) or WebSockets (rejected: adds connection lifecycle overhead; neither exists in `internal/serve/handlers.go` today).
**Rationale**: Centralizing polling in `store.js` prevents blank flashes on tab switches and reuses the proven polling model.
**Terminal consumer**: `internal/serve/handlers.go:79-85` (`GET /api/state`) and `internal/serve/static/app.js:1-10` (`fetchState`).

## Decision 5 — Targeted DOM Node Patching

**Choice**: Granular DOM updates patching elements by ID (`#card-${runID}-${laneID}`) using `textContent` and safe attribute updates.
**Alternatives considered**: Container-wide `innerHTML = ''` replacement on every poll tick (rejected: wiping `#approvals-container` in `internal/serve/static/app.js:45-70` causes focus loss and scroll jumping).
**Rationale**: Targeted patching preserves scroll and focus states during background refresh ticks without a virtual DOM library.
**Terminal consumer**: `internal/serve/static/app.js:45-70` (replaces destructive `containerEl.innerHTML = ''` wipe).

## Decision 6 — Read-Only Tab-Scoped REST Routes over serve.Model

**Choice**: Register granular HTTP GET endpoints in `internal/serve/handlers.go` (`/api/features`, `/api/leases`, `/api/overlap`, `/api/reconciliations`, `/api/audit`) wrapping `serve.Model` methods (`internal/serve/model.go:128-343`).
**Alternatives considered**: Monolithic `/api/state` payload expansion (rejected: dumping all ledger tables increases payload size and SQLite contention in `internal/ledger/ledger.go:162-185`); browser mutation endpoints (rejected: mutations stay on CLI in `cmd/lucind-ai/cli.go:727-750`).
**Rationale**: Granular endpoints allow views to fetch ledger telemetry on demand without bloating global state polling.
**Terminal consumer**: `internal/serve/handlers.go:36-118` (`NewHandler` constructing `NewModel` at `internal/serve/model.go:21-24` and invoking `ListFeatures` at `internal/serve/model.go:128-149`).

## Decision 7 — Approvals Inbox Encapsulation with Unchanged Safety Invariants

**Choice**: Encapsulate approvals inbox in `views/approvals.js` while maintaining individual decision endpoints (`POST /approvals/{runID}/{laneID}`), 400 rejection for bulk bodies, loopback address binding, and inline evidence checks.
**Alternatives considered**: Batch decision endpoints or multi-select UI controls (rejected: strictly prohibited by `internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`, and `openspec/specs/approvals-web-ui/spec.md:26-48`).
**Rationale**: Preserves compliance with `approvals-web-ui` specifications and static security tests (`internal/serve/static_test.go:11-39`).
**Terminal consumer**: `internal/serve/handlers.go:87-115` (`/approvals/` POST handlers) and `internal/serve/static_test.go:11-39` (`TestEmbedFSHasNoApproveAllControl`).

## Open Questions

- [ ] Should future changes add Server-Sent Events (`http.Flusher`) under `/api/stream` for push updates, or retain tab-scoped HTTP polling indefinitely?
- [ ] Should `control-room-ui-shell` ship a placeholder read-only Features view, or defer all non-approvals UI components entirely to `control-room-ui-views`?
- [ ] Note: The phase execution contract in this packet overrides the single-agent `sdd-design` SKILL.md (800-word limit, full document creation, and Engram persistence) by scoping Lens A specifically to architecture decisions under a 1000-word budget.
