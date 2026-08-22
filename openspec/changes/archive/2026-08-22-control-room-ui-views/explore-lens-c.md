# Explore Lens C — Risks, Trade-offs & Spikes: Control Room UI Views

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| Full DOM reconstruction on polling interval causes scroll jumping, input loss, and UI flickering across multi-view panels | High | Implement keyed element patching / granular DOM reconciliation in vanilla JS instead of full `innerHTML` wipe on every interval tick | `internal/serve/static/app.js:45-70`, `internal/serve/static/app.js:96-97` |
| XSS and script injection from unescaped agent output, error messages, and raw evidence payloads rendered in view cards | High | Enforce mandatory text escaping via `escapeHtml` or `textContent` for all candidate outputs, notes, and failure reasons before DOM insertion | `internal/serve/static/app.js:51-55`, `internal/serve/static/app.js:91-94`, `internal/serve/model.go:109-110` |
| Accidental introduction of batch actions or "approve all" buttons in composite views violating core safety invariants | High | Isolate action triggers per card/row with individual click confirmation; preserve strict backend rejection of bulk requests | `internal/serve/handlers.go:161-176`, `internal/serve/static/app.js:63-66` |
| Performance degradation and UI thread freezing from large `evidence_json` and candidate output payloads transferred in view states | Medium | Paginate or lazy-load heavy payload details (diffs, full logs, AST evidence) on demand instead of bundling in top-level state | `internal/serve/model.go:68`, `internal/serve/model.go:109`, `internal/serve/model.go:245-275` |
| Client-server clock skew causing false expiration states or premature timeouts for leases and reconciliation requests | Medium | Base countdown badges and TTL calculations on server-emitted timestamps or remaining seconds deltas rather than local browser time | `internal/serve/model.go:56`, `internal/serve/model.go:84`, `internal/serve/model.go:354-357` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| **View Navigation**: Single Page App (hash-based tab routing) vs Multi-Page HTML routes | SPA retains client cache and polling connection across view switches; no page reload | Requires client-side view router and state container in vanilla JS | Low: Single static endpoint in `internal/serve/handlers.go:39-77` |
| **View Navigation**: Multi-Page HTML routes (`/views/approvals`, `/views/features`, `/views/reconcile`) | Clean isolation per HTML file; smaller memory footprint per view | Full reloads tear down active polling connections; duplicated layout headers | Medium: Multiple static routes and assets to serve in `internal/serve/handlers.go:39-55` |
| **Data Fetching**: Monolithic `/api/state` endpoint aggregating all view domains | Single HTTP roundtrip per poll cycle; unified snapshot of features, approvals, and reconciliations | Over-fetches data for inactive views; larger payload on every 2s tick | Low: Single handler in `internal/serve/handlers.go:120-146` backed by `internal/serve/model.go:17-24` |
| **Data Fetching**: Granular per-view endpoints (`/api/approvals`, `/api/features`, `/api/reconcile`) | Active view only fetches required domain data; reduced payload size | Multiple concurrent fetch requests; potential state inconsistency between views | Medium: Requires coordinated fetch management in `internal/serve/static/app.js:1-10` |
| **Evidence Presentation**: Raw plain text `<pre>` blocks vs Custom vanilla JS syntax/diff formatter | Zero dependencies; minimal CPU overhead; strict adherence to PRD §8.3 no-build rule | Unformatted unified diffs and dense JSON blobs are harder to read | Low: Zero external maintenance, robust plain rendering |
| **Evidence Presentation**: Custom vanilla JS diff/log formatter | High visual clarity for file:line citations and test outputs | Adds ~150 lines of custom parsing and rendering logic to maintain | Medium: Ongoing maintenance of bespoke parser without npm |
| **Action Feedback**: Authoritative polling refresh vs Optimistic local UI updates | Strict alignment with SQLite ledger ground-truth; no ghost state or rollback bugs | 0–2s latency before button action reflects in UI card removal | Low: Follows current pattern in `internal/serve/static/app.js:72-89` |
| **Action Feedback**: Optimistic UI updates (immediate card disable/spinner) | Instant user feedback on button clicks | Requires complex rollback if backend returns 409 Conflict or 500 error | Medium: Added error handling and rollback state logic in frontend |

## Potential Spikes / Proof of Concepts

- **Keyed DOM Reconciliation Spike**: Prototype a minimal, dependency-free keyed reconciliation function in `internal/serve/static/app.js:22-70` that updates modified cards in place and inserts/removes only changed nodes, proving that user scroll position and inspection state are preserved across polling ticks.
- **On-Demand Overlap Evidence Viewer Spike**: Build a prototype modal/accordion endpoint and view component fetching detailed `evidence_json` from `internal/serve/model.go:245-275` on user click, proving heavy AST/diff payloads can be excluded from the main state polling payload.
- **Reconciliation Candidate Decision & CAS Flow Spike**: Create a prototype UI component for `reconciliation_requests` (`internal/serve/model.go:74-115`) that renders direction binding, candidate status, and CAS outcome badges (`internal/serve/model.go:429-439`), verifying that actions invoke individual endpoints (`cmd/lucind-ai/cli.go:1067-1100`) without violating anti-bulk approval constraints (`internal/serve/handlers.go:161-176`).

## Out of Scope

- HTTP server configuration, loopback socket binding, and TLS management (owned by `control-room-serve`, `internal/serve/server.go:1-60`).
- Global UI shell frame, sidebar navigation layout, and base dark-theme styling (owned by `control-room-ui-shell`, `internal/serve/static/index.html:1-140`).
- SQLite database migrations, schema evolution, and query transactions (owned by `control-room-ledger`, `internal/ledger/schema.go:18-180`, `internal/ledger/ledger.go:1-1436`).
- Telemetry event emission, log streaming ingestion, and executor output capture (owned by `control-room-capture` and `control-room-telemetry`).
- Third-party JavaScript frameworks (React, Vue, Svelte), TypeScript toolchains, or npm dependencies (forbidden by PRD §8.3 constraint).
- Problem space framing, candidate architectural options, and recommendation synthesis (owned by Lens A).
- User personas, user journey scenarios, and capability acceptance criteria (owned by Lens B).

## Open Questions

- [ ] Should the Control Room UI views load all domain sub-models (`Features`, `Attempts`, `Leases`, `Reconciliations`) via a single aggregated `/api/state` JSON response or via separate lazy `/api/views/*` endpoints when each view tab is selected?
- [ ] For time-critical lease and reconciliation countdowns, should the server emit pre-calculated `remaining_seconds` in `serve.Model` structs or should the client calculate offsets against `server_time` to prevent clock drift?
