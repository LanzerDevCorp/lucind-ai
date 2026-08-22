# Spec Lens B — Scenarios & Coverage: Control Room UI Shell

## Assumed requirements

This change introduces five new capabilities (`control-room-ui-shell`, `control-room-client-routing`, `control-room-shared-store`, `control-room-asset-embed`, `control-room-model-queries`) and updates `approvals-web-ui` to convert `lucind-ai serve` into a zero-build embed-only multi-view shell. It specifies six requirements: `Layout Shell and Global Chrome` (header chrome and `#view-outlet`), `Client-Side Routing and View Lifecycle` (hash routing and lifecycle cleanup), `Centralized Reactive Client Store` (state caching and polling), `Zero-Build Embedded ES Module Delivery` (embed.FS static delivery), `Approvals Inbox View Integration` (mounting approvals inbox with loopback, individual decision, and anti-bulk rules), and `Read-Only Model Inspection Endpoints` (read-only JSON telemetry queries over `serve.Model`).

## Scenarios

### Requirement: Layout Shell and Global Chrome

#### Scenario: SPA shell initial load

- GIVEN loopback server at 127.0.0.1:7433
- WHEN an HTTP client requests GET /
- THEN the server MUST return HTTP 200 HTML with header chrome, tabs, and #view-outlet

#### Scenario: State polling refreshes header metrics

- GIVEN the mounted shell and GET /api/state with approver "alice" and rate 0.05
- WHEN the store applies the update
- THEN header chrome MUST display approver "alice" and defect rate "5.0%"

#### Scenario: Network failure shows disconnected indicator

- GIVEN an active shell whose state poll fails
- WHEN polling fails
- THEN header chrome MUST display a disconnected indicator

### Requirement: Client-Side Routing and View Lifecycle

#### Scenario: Hash route transition clears view timers

- GIVEN approvals view mounted with an active 2000ms timer
- WHEN the operator navigates to #/features
- THEN the router MUST cancel the timer, unmount approvals, and mount features into #view-outlet

#### Scenario: Targeted DOM updates on poll ticks

- GIVEN an active approvals view displaying pending cards
- WHEN the store dispatches fresh state
- THEN the view MUST patch cards in place without replacing parent #view-outlet innerHTML

#### Scenario: Unregistered route shows not-found view

- GIVEN registered routes #/approvals and #/features
- WHEN the browser navigates to #/unknown
- THEN the router MUST render a not-found view in #view-outlet without page reload

### Requirement: Centralized Reactive Client Store

#### Scenario: Cached state renders on view revisit

- GIVEN approvals state cached in store.js
- WHEN navigating away and returning to #/approvals
- THEN the view MUST render cached state immediately without a blank flash

#### Scenario: Polling timer fetches state and notifies subscribers

- GIVEN store.js with registered subscribers
- WHEN the 2000ms polling timer fires
- THEN store.js MUST fetch GET /api/state and notify subscribers

#### Scenario: Polling failure retains existing cache

- GIVEN cached state in store.js
- WHEN GET /api/state returns HTTP 500
- THEN store.js MUST preserve cache and notify subscribers of the error

### Requirement: Zero-Build Embedded ES Module Delivery

#### Scenario: Static assets served with matching MIME type

- GIVEN assets embedded in staticFS
- WHEN requesting GET /shell.js or GET /style.css
- THEN the server MUST return HTTP 200 with application/javascript or text/css

#### Scenario: Module import resolution

- GIVEN modular ES modules in staticFS
- WHEN the browser fetches imported module GET /store.js
- THEN the server MUST return HTTP 200 with application/javascript

#### Scenario: Missing asset returns 404

- GIVEN a running server
- WHEN requesting GET /missing.js
- THEN the server MUST return HTTP 404 Not Found

### Requirement: Approvals Inbox View Integration

#### Scenario: Individual approval decision posted

- GIVEN a pending card in #/approvals with valid file:line evidence
- WHEN the operator submits an approval
- THEN the client MUST POST /approvals/{runID}/{laneID} with {"decision":"approved"} and receive HTTP 200

#### Scenario: Items start unselected and bare prose is withheld

- GIVEN a pending item with bare prose lacking file:line or command output
- WHEN approvals view renders the item
- THEN the item MUST start unselected and display a missing-evidence placeholder

#### Scenario: Bulk approval payload rejected

- GIVEN a multi-item approval payload posted to /approvals/run-1/lane-1
- WHEN the handler processes the request
- THEN the server MUST return HTTP 400 Bad Request

### Requirement: Read-Only Model Inspection Endpoints

#### Scenario: Query features returns JSON list without writes

- GIVEN features in the ledger
- WHEN a client requests GET /api/features
- THEN the server MUST return HTTP 200 with a JSON array matching serve.Model.ListFeatures

#### Scenario: Query single feature by ID

- GIVEN feature feat-1 exists and feat-missing does not
- WHEN requesting GET /api/features/feat-1 and GET /api/features/feat-missing
- THEN the server MUST return HTTP 200 for feat-1 and HTTP 404 for feat-missing

#### Scenario: Non-GET methods rejected

- GIVEN model query route /api/features
- WHEN a client sends POST or DELETE
- THEN the server MUST return HTTP 405 Method Not Allowed

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Layout Shell and Global Chrome | covered | covered | covered | internal/serve/handlers.go:39, internal/serve/static_test.go:41 |
| Client-Side Routing and View Lifecycle | covered | covered | covered | internal/serve/static/app.js:96, internal/serve/handlers.go:39 |
| Centralized Reactive Client Store | covered | covered | covered | internal/serve/static/app.js:1, internal/serve/handlers.go:79 |
| Zero-Build Embedded ES Module Delivery | covered | covered | covered | internal/serve/static.go:8, internal/serve/handlers.go:41 |
| Approvals Inbox View Integration | covered | covered | covered | internal/serve/server_test.go:42, internal/serve/static_test.go:11 |
| Read-Only Model Inspection Endpoints | covered | covered | covered | internal/serve/model_test.go:74, new seam required |

## Untestable Assertions

None

## Open Questions

- [ ] Scope boundary for Features view: Does control-room-ui-shell ship a stub Features view (#/features), or defer full Features UI to control-room-ui-views?
- [ ] History API fallback routing: Should client routing remain hash-only, or add server wildcard fallback for HTML5 History in serve.NewHandler?
- [ ] Real-time updates: Should future milestones introduce stdlib SSE streaming (/api/stream) or retain 2000ms polling?
- [ ] View subscription granularity: Should views subscribe to specific store slices or receive full store snapshot dispatches?
