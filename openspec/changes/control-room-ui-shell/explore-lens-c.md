# Explore Lens C — Risks, Trade-offs & Spikes: control-room-ui-shell

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| SQLite lock contention from frequent multi-table UI polling during active lane runs | Medium | Limit UI queries to short timeouts with bounded row limits; rely on WAL mode and busy timeouts; query only active tab data rather than full ledger dumps on every tick. | `internal/ledger/ledger.go:162-185`, `internal/serve/model.go:128-250`, `internal/run/batch.go:110-180` |
| UI state synchronization thrashing and polling performance degradation without build tools | Medium | Implement lightweight vanilla JS DOM reconciliation per section/tab instead of full page rebuilds; preserve scroll/focus states; consider ETag/version polling headers. | `internal/serve/static/app.js:22-70`, `internal/serve/static/app.js:96-98`, `internal/serve/handlers.go:120-146` |
| Premature exposure of write/mutation actions in UI bypassing CLI concurrency & lease guards | High | Restrict Control Room UI Shell to read-only views for features, leases, reconciliations, and runs; keep mutations strictly confined to individual approval decisions and CLI flows. | `internal/serve/handlers.go:87-115`, `internal/feature/feature.go:89-135`, `internal/reconcile/reconcile.go:116-168` |
| Loopback security boundary erosion or DNS rebinding exposing sensitive codebase telemetry | High | Maintain strict loopback binding (`127.0.0.1`/`localhost`), validate request Host headers, and reject any non-loopback interface binding at startup. | `internal/serve/server.go:16-22`, `internal/serve/server.go:57-73`, `cmd/lucind-ai/cli.go:691-694` |
| XSS and injection vulnerabilities when rendering untrusted agent stdout/stderr and diffs | Medium | Strictly sanitize ANSI escape codes and HTML entities using `textContent` and pre-escaped DOM elements for all evidence and candidate outputs before insertion. | `internal/serve/static/app.js:12-20`, `internal/serve/static/app.js:91-94`, `internal/serve/model.go:102-115` |
| Lack of test harness for HTTP endpoint serialization and query model routing | Low | Add `httptest.Server` end-to-end tests validating JSON payloads across all `internal/serve/model.go` query types and static asset resolution. | `internal/serve/server_test.go:1-74`, `internal/serve/model_test.go:1-250`, `internal/serve/handlers.go:36-118` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| **Live Telemetry: Polling vs. Server-Sent Events (SSE)** | Short-polling uses zero extra dependencies, simple stateless reconnects (`app.js:97`); SSE provides instant push updates for audit events without query spam. | Polling causes continuous SQLite read load; SSE requires in-memory event fanout channel in Go server. | Low for granular polling; Low-to-Medium for stdlib SSE. |
| **API Architecture: Monolithic `/api/state` vs. Granular Endpoints** | Monolithic endpoint provides one round-trip; Granular endpoints (`/api/approvals`, `/api/features`, `/api/reconcile`, `/api/audit`) query only active tabs and scale cleanly. | Monolithic payload balloons with candidate logs and audit events; Granular requires multiple initial fetches. | Granular is significantly lower SQLite load and better modularity. |
| **Frontend Packaging: Single Monolithic HTML vs. Modular Tabbed Shell** | Monolithic HTML keeps single-file view; Modular Tabbed Shell cleanly partitions Approvals, Feature Leases, Reconciliation, and Run DAG. | Monolithic becomes cluttered and unmaintainable; Tabbed shell requires structured CSS/JS organization in embedded assets. | Low; zero build toolchain required for either approach (`static.go:12`). |
| **Action Surface: Read-Only Dashboard vs. Interactive Control Center** | Read-Only preserves CLI ownership of git/lease invariants and minimizes security blast radius; Interactive allows one-click lease renewal/reconcile. | Read-Only requires running CLI commands in terminal for actions; Interactive duplicates validation and risks race conditions. | Read-Only has near-zero risk; Interactive adds high state-machine complexity. |

## Potential Spikes / Proof of Concepts

- **Spike 1: Multi-tab Vanilla Shell & Sanitized Output Rendering** (`internal/serve/static/app.js:22-70`, `internal/serve/static/index.html:141-163`, `internal/serve/static.go:12`): Prototype a zero-dependency tabbed navigation shell (Approvals, Features & Leases, Reconciliation, DAG Runs, Audit Stream) with safe ANSI/HTML escaping and hash routing to verify DOM performance without external libraries.
- **Spike 2: Granular REST API vs SQLite Contention Benchmark** (`internal/serve/model.go:128-250`, `internal/serve/handlers.go:120-146`, `internal/ledger/ledger.go:182`): Benchmark concurrent reads on `features`, `integration_attempts`, `reconciliation_requests`, and `approvals` tables against active write transactions from simulated batch runs (`internal/run/batch.go:110-180`) to verify busy-timeout headroom.
- **Spike 3: Interactive CLI Command Snippet Generator** (`internal/serve/static/index.html:151-153`, `internal/serve/model.go:72-100`, `cmd/lucind-ai/cli.go:727-750`): Prototype contextual copyable CLI snippets (e.g., `lucind-ai feature renew`, `lucind-ai reconcile`) within the read-only feature/reconcile views, mirroring the existing `opencode` batch review command box.

## Out of Scope

- Modifying the SQLite schema version or adding new database migrations (remains at schemaVersion 5 in `internal/ledger/schema.go:10`).
- Adding Node.js, npm, bundlers, frontend frameworks (React, Vue, Svelte), or CDN-dependent stylesheets (`internal/serve/static.go:12`, `docs/prd.md:219-222`).
- Remote network binding, multi-tenant authentication, or TLS termination (remains loopback-only on `127.0.0.1` in `internal/serve/server.go:16-22`).
- Bulk approval buttons or bypassing per-item unselected approval enforcement (`internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`).
- Direct git mutations or automated code rebasing triggered from the browser UI.
- Terminal emulation or replacing terminal multiplexers.

## Open Questions

- [ ] Should live telemetry remain timer-based HTTP polling (e.g. 2s) across granular tab endpoints, or is a stdlib SSE endpoint (`/api/events/stream`) preferred for real-time audit stream updates?
- [ ] Should the Control Room UI render the full historical DAG run tree and wave progression, or focus primarily on active runs and pending approvals?
