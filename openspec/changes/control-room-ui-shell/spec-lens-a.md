# Spec Lens A — Capabilities & Requirements: Control Room UI Shell

## Assumed requirements

This change specifies six requirements across six capabilities: five new capabilities requiring full specifications (`control-room-ui-shell`, `control-room-client-routing`, `control-room-shared-store`, `control-room-asset-embed`, and `control-room-model-queries`) and one existing capability requiring a delta specification (`approvals-web-ui`). Each capability receives exactly one requirement: Layout Shell and Global Chrome for `control-room-ui-shell`, Client-Side Routing and View Lifecycle for `control-room-client-routing`, Centralized Reactive Client Store for `control-room-shared-store`, Zero-Build Embedded ES Module Delivery for `control-room-asset-embed`, Approvals Inbox View Integration for `approvals-web-ui`, and Read-Only Model Inspection Endpoints for `control-room-model-queries`. All six requirements are ADDED, introducing modular SPA architecture, client-side routing, shared state caching, embedded asset delivery, inbox view mounting, and read-only telemetry queries without modifying existing loopback, anti-bulk, or evidence requirements.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `control-room-ui-shell` | New | `openspec/specs/control-room-ui-shell/spec.md` | |
| `control-room-client-routing` | New | `openspec/specs/control-room-client-routing/spec.md` | |
| `control-room-shared-store` | New | `openspec/specs/control-room-shared-store/spec.md` | |
| `control-room-asset-embed` | New | `openspec/specs/control-room-asset-embed/spec.md` | |
| `control-room-model-queries` | New | `openspec/specs/control-room-model-queries/spec.md` | |
| `approvals-web-ui` | Existing | `openspec/changes/control-room-ui-shell/specs/approvals-web-ui/spec.md` | `openspec/specs/approvals-web-ui/spec.md:1-83` |

## ADDED Requirements

### Requirement: Layout Shell and Global Chrome

The server MUST serve a modular single-page application (SPA) layout shell featuring persistent header chrome displaying server connection/freshness status, approver identity, and wrong-approval rate metrics, navigation tabs, and a main `#view-outlet` container for mounting registered views without full page reloads.

**Terminal consumer**: `internal/serve/static/index.html:142-158`, `internal/serve/handlers.go:39-77` (browser client fetching `/`)

### Requirement: Client-Side Routing and View Lifecycle

The client application MUST provide client-side hash routing (`#/route`) with a view registry lifecycle that mounts target views into `#view-outlet`, tears down previous views by unmounting DOM and cancelling active timers and listeners, and updates DOM nodes in place without wholesale `innerHTML` replacement.

**Terminal consumer**: `internal/serve/static/app.js:96-98`, `internal/serve/handlers.go:39-55` (browser hash change listener and view lifecycle dispatcher)

### Requirement: Centralized Reactive Client Store

The client application MUST maintain centralized state in a shared `store.js` module across view transitions, caching server state, periodic polling results from `GET /api/state` on a 2000ms interval, and dispatching reactive updates to subscribed view components without full-page re-fetches.

**Terminal consumer**: `internal/serve/static/app.js:1-10`, `internal/serve/handlers.go:79-85` (client view subscriber callbacks and `GET /api/state` polling handler)

### Requirement: Zero-Build Embedded ES Module Delivery

The binary MUST embed all HTML, CSS, and vanilla ES module assets via Go `embed.FS` (`internal/serve/static.go`), serving them directly over HTTP with appropriate `Content-Type` headers (`application/javascript`, `text/css`, `text/html`) without requiring external Node.js, npm, or build bundler dependencies.

**Terminal consumer**: `internal/serve/static.go:8-18`, `internal/serve/handlers.go:41-55` (`lucind-ai serve` loopback static HTTP file server)

### Requirement: Approvals Inbox View Integration

The approvals inbox MUST mount as a registered view (`#/approvals`) within the SPA shell while preserving loopback-only enforcement (`127.0.0.1`), inline `file:line` or command-output evidence rendering, individual per-item decision submission via `POST /approvals/{runID}/{laneID}`, and rejection of bulk or array approval requests with HTTP 400.

**Terminal consumer**: `internal/serve/static/app.js:12-89`, `internal/serve/handlers.go:87-115`, `internal/serve/handlers.go:161-176` (Approvals view module mounted in `#view-outlet` and `/approvals` HTTP handler)

### Requirement: Read-Only Model Inspection Endpoints

The HTTP handler MUST expose read-only REST `GET` endpoints returning JSON serialized from `serve.Model` methods (`ListFeatures`, `GetFeature`, `ListAttempts`, `ListLeases`, `ListOverlapEvidence`, `ListReconciliationRequests`, `ListAuditEvents`) without invoking shell subprocesses, executing git commands, or mutating ledger state.

**Terminal consumer**: `internal/serve/handlers.go:36-118`, `internal/serve/model.go:14-343` (HTTP server mux router and incoming client `GET` queries)

## Open Questions

- [ ] Does this change also ship a read-only Features view, or only the shell + Approvals + Model GET routes, deferring Features UI to `control-room-ui-views`?
- [ ] Should later History API routing add a wildcard fallback on `NewHandler` (`internal/serve/handlers.go:39-77`)? This change uses hash routes.
- [ ] Keep 2s (optionally tab-scoped) polling, or add stdlib SSE (`http.Flusher`) in a follow-up? Neither `/api/stream` nor `/api/events/stream` exists.
- [ ] Should views subscribe to store slices or to full snapshots?
