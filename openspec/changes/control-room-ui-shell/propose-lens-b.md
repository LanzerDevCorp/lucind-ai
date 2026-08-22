# Proposal Lens B — Capability Impact & Specs: Control Room UI Shell

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `control-room-ui-shell` | Added | SPA layout shell with persistent header chrome, navigation tabs, and view outlet container. | `internal/serve/static/index.html:142-158`, `internal/serve/handlers.go:39-77` |
| `control-room-client-routing` | Added | Hash-based client router and view lifecycle manager mounting/unmounting views and clearing timers/listeners. | `internal/serve/static/app.js:96-98`, `internal/serve/handlers.go:39-55` |
| `control-room-shared-store` | Added | Client store managing shared state and periodic polling of `/api/state` across view switches. | `internal/serve/static/app.js:1-10`, `internal/serve/handlers.go:79-85` |
| `control-room-asset-embed` | Added | Zero-build asset pipeline serving modular vanilla ES modules and CSS via Go embedding. | `internal/serve/static.go:8-18`, `internal/serve/handlers.go:42-55` |
| `approvals-web-ui` | Modified | Approvals inbox transitions into a mountable view module preserving individual decision and evidence rules. | `openspec/specs/approvals-web-ui/spec.md:26-66`, `internal/serve/static/app.js:12-89` |
| `control-room-model-queries` | Added | HTTP read-only query endpoints exposing `serve.Model` ledger data to shell views. | `internal/serve/model.go:14-125`, `internal/serve/model.go:128-343` |

## Delta Specifications

### Requirement: Layout Shell and Global Chrome

The UI server MUST serve a modular SPA layout shell containing a persistent header with status indicators, connection state, approver metrics, navigation tabs, and a main view mounting outlet.

#### Scenario: SPA shell initial load
- GIVEN a running server bound to loopback at `127.0.0.1:7433` (`cmd/lucind-ai/cli.go:683-719`, `internal/serve/server.go:19-22`)
- WHEN an operator navigates to `http://127.0.0.1:7433/` (`internal/serve/handlers.go:39-77`)
- THEN the server MUST return the embedded layout shell (`internal/serve/static.go:8-18`, `internal/serve/static/index.html:142-158`) rendering header chrome, navigation tabs, and default view outlet without page reload.

#### Scenario: Global metrics and connection status display
- GIVEN a client connected to the serve HTTP backend (`internal/serve/handlers.go:79-85`, `internal/serve/static/app.js:1-10`)
- WHEN state updates are received
- THEN the persistent header MUST display connection status, approver identity, and wrong-approval rate (`internal/serve/handlers.go:130-141`, `internal/serve/static/index.html:145-148`).

### Requirement: Client-Side Routing and View Lifecycle

The UI client MUST implement hash-based client routing and a view registry lifecycle. When navigating between views, the router MUST unmount the active view, cancel active timers and listeners, and mount the target view into the main outlet.

#### Scenario: View transition and cleanup
- GIVEN the operator is viewing the approvals view with active update timers (`internal/serve/static/app.js:96-98`)
- WHEN the operator navigates to `#/features` (`internal/serve/static/index.html:142-158`)
- THEN the client router MUST unmount the approvals view, clear timers, and mount the features view into the outlet without browser reload.

#### Scenario: In-place DOM updates
- GIVEN an active view mounted in the outlet
- WHEN background state polling updates the shared client store (`internal/serve/static/app.js:1-10`, `internal/serve/handlers.go:79-85`)
- THEN the client MUST update DOM nodes in place to preserve scroll position and focus (`internal/serve/static/app.js:22-70`).

### Requirement: Centralized Reactive Client Store

The UI client MUST manage application state through a centralized store module (`store.js`). The store MUST retain cached state across view transitions and refresh state via loopback HTTP polling to `/api/state`.

#### Scenario: Shared state across navigation
- GIVEN pending approval items and approver rates loaded into the store (`internal/serve/handlers.go:120-146`, `internal/serve/static/app.js:1-10`)
- WHEN the operator navigates between views and returns
- THEN cached state MUST remain immediately available in the store without a blank render.

#### Scenario: Polling refresh
- GIVEN the UI shell running on loopback
- WHEN the 2000ms polling timer fires (`internal/serve/static/app.js:96-98`)
- THEN the client store MUST fetch `GET /api/state` (`internal/serve/handlers.go:79-85`) and notify view subscribers.

### Requirement: Zero-Build Embedded ES Module Delivery

The Go binary MUST embed all HTML, CSS, and vanilla ES module JavaScript files using `embed.FS`. Web assets MUST execute in modern browsers without npm, Node.js, or bundlers.

#### Scenario: Embedded asset resolution
- GIVEN a compiled binary without Node.js dependencies (`Makefile:7-8`, `internal/serve/static.go:8-18`)
- WHEN a browser requests static module files (`/shell.js`, `/store.js`, `/style.css`)
- THEN the server MUST serve files from embedded `StaticFS` (`internal/serve/static.go:12-18`) with valid `Content-Type` headers (`internal/serve/handlers.go:41-55`).

### Requirement: Approvals Inbox View Integration

The approvals inbox view MUST integrate into the shell while preserving loopback-only access, inline evidence rules, approver accuracy metrics, and rejection of bulk approval requests.

#### Scenario: Individual approval submission from shell
- GIVEN a pending approval card with valid `file:line` or command output evidence in the shell view (`internal/serve/static/app.js:12-20,47-68`, `internal/ledger/schema.go:45-56`)
- WHEN the operator submits an individual decision (`internal/serve/static/app.js:72-89`)
- THEN the client MUST POST to `/approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115`), the server MUST record the decision (`internal/serve/handlers.go:148-230`), and bulk request bodies MUST be rejected with HTTP 400 (`internal/serve/handlers.go:161-176`, `openspec/specs/approvals-web-ui/spec.md:26-48`).

#### Scenario: Loopback binding enforcement
- GIVEN `lucind-ai serve` invoked with `--addr 0.0.0.0:7433` (`cmd/lucind-ai/cli.go:683-694`)
- WHEN the server evaluates the listen address
- THEN `IsLoopback` MUST return false (`internal/serve/server.go:57-73`), and the command MUST terminate with `ErrNonLoopback` (`internal/serve/server.go:14-22`, `openspec/specs/approvals-web-ui/spec.md:10-25`).

### Requirement: Read-Only Model Inspection Endpoints

The HTTP handler MUST provide read-only REST endpoints exposing the `serve.Model` query surface for ledger entities without executing shell or git commands.

#### Scenario: Querying feature and reconciliation status
- GIVEN ledger entries for features, integration attempts, and reconciliations (`internal/ledger/schema.go:18-180`, `internal/serve/model.go:14-125`)
- WHEN a client or view component issues a `GET` request to a model query endpoint
- THEN the server MUST return JSON representations via `serve.Model` methods (`internal/serve/model.go:128-343`) without mutating ledger state.

## Open Questions

- [ ] Route format: Hash routing (`#/approvals`, `#/features`) is selected for zero-config static serving; should wildcard fallback routing in Go handler (`internal/serve/handlers.go:39-55`) be added if HTML5 History API routing is needed?
- [ ] View Ingestion Contract: Should future view modules register in a central registry and subscribe to discrete store slices or full store snapshots?
- [ ] Transport upgrade: While HTTP polling (`/api/state`) provides immediate store sync (`internal/serve/static/app.js:96-98`), should Server-Sent Events (`/api/stream`) be added in this change or deferred?
- [ ] Drift note: The canonical `sdd-propose` skill (`~/.claude/skills/sdd-propose/SKILL.md:90-158`) defines a monolithic proposal (`proposal.md`), whereas this change packet executes a 3-lens parallel proposal model (Lens B: Capability Impact & Specs).
