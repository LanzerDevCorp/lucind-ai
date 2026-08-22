# Proposal Lens C — Risks, Rollback & Test Impact: Control Room UI Views

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| **DOM Reconstruction Thrashing**: High-frequency interval polling (2s) wiping root container HTML via `innerHTML = ''` drops active element focus, card collapse/expansion state, and scroll offset across multi-view tabs. | High | Implement keyed DOM element patching and granular node updates in vanilla JS, mutating modified nodes in place while preserving user inspection state. | `internal/serve/static/app.js:45-70, 96-97` |
| **XSS and Script Injection via Model DTOs**: Unescaped rendering of agent candidate diff outputs, check logs, error notes, or failure reasons in browser views. | High | Enforce mandatory HTML entity encoding via `escapeHtml` / `textContent` for all strings, candidate outputs, and JSON payloads prior to DOM insertion. | `internal/serve/static/app.js:51-55, 91-94`, `internal/serve/model.go:109-110` |
| **Accidental Bulk / Rubber-Stamp Action Invariant Violation**: Introducing global "Approve All" or multi-selection controls in composite dashboard views violates single-decision accountability. | High | Isolate action triggers per card with individual confirmation; retain strict backend HTTP 400 Bad Request enforcement for array/bulk payloads. | `internal/serve/handlers.go:161-176`, `internal/serve/static/app.js:63-66`, `internal/serve/static_test.go:11-39` |
| **UI Freeze & Memory Bloat from Large Payloads**: Embedding heavy `evidence_json` AST blobs or 400-line candidate diffs directly in top-level periodic polling responses blocks the browser event loop. | Medium | Exclude large diagnostic payloads from routine polling; lazy-load heavy details on card expansion via dedicated sub-resource REST endpoints. | `internal/serve/model.go:68, 109, 245-275`, `internal/serve/handlers.go:79-85` |
| **Client-Server Clock Skew on Expiring Leases/TTLs**: Calculating lease expiration countdowns or reconciliation request TTLs against local browser system time causes false-expired states or premature alerts. | Medium | Emit server-calculated `remaining_seconds` or synchronize countdown calculations against server-provided timestamp headers. | `internal/serve/model.go:56, 84, 354-357`, `internal/feature/feature.go:293-315` |
| **Missing Lanes/Batch Query Abstraction in Model**: Batch and wave views querying the ledger directly without `serve.Model` DTOs risks data races with concurrent execution or shell/git leakage. | Medium | Extend `serve.Model` with read-only DTO methods (`ListBatchLanes`, `GetLaneResult`) adhering to the shell-free query contract. | `internal/serve/model.go:14-24`, `internal/lane/status.go:10-28`, `internal/run/batch.go:19-27` |

## Rollback & Additivity

**Rollback Plan**: Revert the Git commit via `git revert`. Because this change only adds read-only HTTP routes (`internal/serve/handlers.go:36-118`), UI static assets (`internal/serve/static/app.js:1-98`, `internal/serve/static/index.html:1-140`), and Model DTO methods (`internal/serve/model.go:14-125`), reverting the commit restores the prior HTTP server and asset state immediately. No SQLite schema migrations, table drops, or database mutations are performed (`internal/ledger/schema.go:10-180` remains at schemaVersion 5), so rollback requires zero data migration. The localhost-only server (`internal/serve/server.go:19-28`) leaves no external daemons, credentials, or remote state behind.

**Additivity**: Fully additive. No database tables, columns, or constraints are modified destructively (`internal/ledger/schema.go:10-180`). On the wire, existing endpoints `GET /api/state` (`internal/serve/handlers.go:79-85, 120-146`) and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115, 148-211`) remain functional for backwards compatibility. All new view routes (`/api/approvals`, `/api/features`, `/api/leases`, `/api/overlap/{id}`, `/api/reconcile/requests`, `/api/batch/lanes`) are strictly additive read endpoints (`internal/serve/handlers.go:36-118`).

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| **DTO / Model Read Layer** (`internal/serve`) | Unit tests for new batch/lane DTO queries and lease/reconcile countdown helpers. Verify `TestModelSourceDoesNotShellOut` passes with zero `os/exec` or `git` imports. | `internal/serve/model_test.go:74-347, 595-627`, `internal/serve/model.go:14-125` |
| **HTTP Dispatch & Status Codes** (`internal/serve`) | Integration tests for new REST endpoints (`/api/approvals`, `/api/features`, `/api/leases`, `/api/overlap/{id}`, `/api/reconcile/requests`, `/api/batch/lanes`). Verify 400 Bad Request on bulk decision attempts and 409 Conflict on duplicate decisions. | `internal/serve/server_test.go:42-93, 196-236`, `internal/serve/handlers.go:36-118` |
| **Static Assets & Security Invariants** (`internal/serve`) | Assert zero occurrences of forbidden bulk approval terms (`approve all`, `bulk-approve`) in embedded assets. Validate that `isValidEvidence` accepts command output and `file:line` citations while rejecting bare multiline prose. Verify cards start unselected. | `internal/serve/static_test.go:11-102`, `internal/serve/static/app.js:12-20`, `internal/serve/static.go:8-19` |
| **Batch & Barrier Observation** (`internal/run`, `internal/barrier`) | Test read-only visibility of wave progression, lane states, timeouts, and barrier release without interfering with active batch runners. Verify preserved worktrees for non-terminal lanes are surfaced accurately. | `internal/barrier/barrier_test.go:1-120`, `internal/run/batch_test.go:1-150`, `internal/run/batch.go:19-89` |
| **Result Envelope Schema Validation** (`internal/result`) | Verify lane envelope diagnostics correctly inspect `.lucind/result.json` fields (`hard_stops`, `deviations`, `done_criteria`) and display `allowed_paths` demotion details. | `internal/result/schema_test.go:1-80`, `internal/result/result.schema.json:1-98`, `internal/run/run.go:576-654` |

## Out of Scope

- Loopback listener configuration, socket binding, and TLS management (owned by `control-room-serve`, `internal/serve/server.go:1-60`).
- Global UI shell frame, sidebar navigation bar, and CSS color tokens (owned by `control-room-ui-shell`, `internal/serve/static/index.html:1-140`).
- Database migrations, schema version increments, or SQLite ledger writes (owned by `control-room-ledger`, `internal/ledger/schema.go:18-180`).
- Live log streaming, pty capture, and background process telemetry (owned by `control-room-capture` and `control-room-telemetry`).
- Third-party frontend frameworks (React, Vue, Svelte), TypeScript compilation, or npm packages (forbidden by PRD §8.3, `docs/prd.md:217-221`).
- Candidate architecture evaluation and conceptual trade-offs (owned by Lens A, `openspec/changes/control-room-ui-views/propose-lens-a.md:1-62`).
- Capability impact definitions, delta specification requirements, and user journey scenarios (owned by Lens B, `openspec/changes/control-room-ui-views/propose-lens-b.md:1-82`).

## Open Questions

- [ ] May the UI expose HTTP POST endpoints for reconciliation mutations (`approve`, `decline`, `cancel`, `renew`, `resolve`), or must it strictly present copy-paste CLI commands matching `cmd/lucind-ai/cli.go:1043-1065`? (`openspec/changes/control-room-ui-views/explore.md:84`).
- [ ] Should overlap `evidence_json` (`internal/serve/model.go:68`) be rendered as raw JSON, formatted `<pre>` text, or structured diff blocks?
- [ ] Should lease and reconciliation expiry countdowns emit pre-computed `remaining_seconds` from `serve.Model` or supply server timestamps for client offset calculations (`internal/serve/model.go:56, 84, 354-357`)?
