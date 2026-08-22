# Tasks Lens A — Decomposition & Ordering: Control Room UI Shell

## Assumed decomposition

The implementation decomposes into four phases across 10 tasks: Phase 1 exposes read-only Model GET telemetry endpoints on the backend handler; Phase 2 introduces foundational client-side modules (`store.js`, `router.js`, `style.css`); Phase 3 delivers the modular approvals view, updates the HTML shell with persistent header chrome, bootstraps the `shell.js` entry point, and removes the legacy monolithic `app.js`; Phase 4 updates static embed and AST tests. The critical path is Phase 2 → Phase 3 (`store.js`/`router.js` → `views/approvals.js` → `shell.js`), migrating from the legacy single-file script to modular ES view lifecycles while preserving loopback and anti-bulk safety invariants.

## Phase 1: Backend Model Telemetry Endpoints

- [ ] 1.1 Construct `serve.NewModel(l)` in `serve.NewHandler` (`internal/serve/handlers.go:36-38`) and register 13 read-only Model GET endpoints (`/api/features`, `/api/features/{id}`, `/api/features/{id}/attempts`, `/api/attempts/{id}`, `/api/leases`, `/api/features/{id}/lease`, `/api/features/{id}/overlap`, `/api/features/{id}/overlap/{hash}`, `/api/features/{id}/reconciliations`, `/api/reconciliations/{id}`, `/api/reconciliations/{id}/candidates`, `/api/candidates/{id}`, `/api/features/{id}/events`) with JSON encoding, 404 handling, and 405 method rejection.
- [ ] 1.2 Add unit tests in `internal/serve/server_test.go:136` verifying 13 Model GET endpoints return HTTP 200 JSON, missing IDs return 404, non-GET methods return 405, and zero ledger mutations occur.

## Phase 2: Foundational Client Modules & Stylesheet

- [ ] 2.1 Create `internal/serve/static/store.js` (new file) implementing centralized reactive state management, polling `GET /api/state` every 2000ms, notifying subscribers, and retaining cached state on HTTP 500 errors.
- [ ] 2.2 Create `internal/serve/static/router.js` (new file) implementing client-side hash routing (`#/route`), `mount`/`unmount` view lifecycle, `hashchange` listener, timer cancellation, and 404 fallback view rendering.
- [ ] 2.3 Create `internal/serve/static/style.css` (new file) implementing CSS rules for layout shell, persistent header chrome (`#approver-name`, `#approver-rate`), disconnected indicator, navigation tabs, card outlet, and inline evidence styling.

## Phase 3: Approvals View & Shell Integration

- [ ] 3.1 Create `internal/serve/static/views/approvals.js` (new file) implementing `mount(container, store)` and `unmount()`, targeted DOM card patching by ID, inline `file:line` / command-output evidence validation, and single-item decision `POST /approvals/{runID}/{laneID}`.
- [ ] 3.2 Modify `internal/serve/static/index.html:1-163` replacing monolithic inbox markup with persistent header chrome, navigation tabs, `#view-outlet`, and ES module entry script `<script type="module" src="/shell.js">`.
- [ ] 3.3 Create `internal/serve/static/shell.js` (new file) implementing entry bootstrap module that initializes `store.js`, registers `#/approvals` with `router.js`, binds header metrics to store updates, and navigates to the default route.
- [ ] 3.4 Delete obsolete monolithic JavaScript file `internal/serve/static/app.js:1-98`.

## Phase 4: Static Embed & Invariant Verification

- [ ] 4.1 Update test assertions in `internal/serve/static_test.go:11-102` to verify zero bulk approval terms across new modules (`shell.js`, `router.js`, `store.js`, `views/approvals.js`), assert JS/CSS MIME types, and retarget inline evidence and unselected controls checks to new static assets.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Backend handler modification can start immediately; depends only on existing `serve.Model` and `ledger.Ledger`. |
| 1.2 | 1.1 | HTTP tests in `server_test.go` require Model GET route registration on the `serve.NewHandler` mux to execute assertions. |
| 2.1 | — | `store.js` is an independent ES module consuming existing `GET /api/state` (`handlers.go:79-85`). |
| 2.2 | — | `router.js` is an independent ES module defining generic hash-routing and view lifecycle contracts. |
| 2.3 | — | `style.css` is an independent stylesheet; `.css` MIME serving already exists in `handlers.go:47-48`. |
| 3.1 | 2.1 | `views/approvals.js` requires the `store.js` subscription and state retrieval interface to render and patch approval cards. |
| 3.2 | 2.3 | `index.html` structure references stylesheet classes and defines `#view-outlet` container matching `style.css`. |
| 3.3 | 2.1, 2.2, 3.1, 3.2 | `shell.js` imports `store.js`, `router.js`, and `views/approvals.js`, and binds event handlers to `#view-outlet` in `index.html`. |
| 3.4 | 3.3 | `app.js` must only be deleted once `shell.js` and modular assets are in place to prevent broken static embed lookups. |
| 4.1 | 3.2, 3.3, 3.4 | `static_test.go` uses `fs.ReadFile(StaticFS())` and fails if `app.js` is read before removal or new modules are missing. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Requirement: Layout Shell and Global Chrome | 2.3, 3.2, 3.3 |
| Requirement: Client-Side Routing and View Lifecycle | 2.2, 3.3 |
| Requirement: Centralized Reactive Client Store | 2.1, 3.3 |
| Requirement: Zero-Build Embedded ES Module Delivery | 2.1, 2.2, 2.3, 3.1, 3.2, 3.3, 3.4, 4.1 |
| Requirement: Read-Only Model Inspection Endpoints | 1.1, 1.2 |
| Requirement: Approvals Inbox View Integration | 3.1, 3.3, 4.1 |

## Open Questions

- [ ] None. Precedence note: `~/.claude/skills/sdd-tasks/SKILL.md` specifies an end-to-end single agent workflow writing the entire `tasks.md` with workload forecasting, which is superseded by the three-lens parallel task partitioning contract for this lane.
