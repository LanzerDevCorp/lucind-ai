# Design Lens A — Decisions: Control Room UI Views

## Assumed architecture

The change extends `internal/serve` by refactoring `serve.NewHandler` (`internal/serve/handlers.go:36-118`) to route modular GET endpoints over `*serve.Model` (`internal/serve/model.go:14-24`) alongside existing approvals handling (`internal/serve/handlers.go:148-211`). `*serve.Model` is extended with batch, DAG wave, and lane lifecycle query methods backed by existing SQLite tables `lanes` (`internal/ledger/schema.go:18-32`) and `events` (`internal/ledger/schema.go:34-43`), without schema migrations (`schemaVersion` remains 5 at `internal/ledger/schema.go:10`). The embedded client (`internal/serve/static/app.js:1-98`, `internal/serve/static/index.html:1-140`) is refactored from a monolithic approvals polling loop into five tabbed vanilla JS panels (Batch/Wave, Approvals, Feature/Lease, Reconciliation, Lane Envelope) using keyed in-place DOM updates and on-demand diagnostic loading, with zero external npm dependencies (`docs/prd.md:217-221`).

## Technical Approach

We adopt Candidate 2: Modular REST endpoints with lazy-loaded vanilla JS panels (`openspec/changes/control-room-ui-views/explore.md:3, 20-23`). The Go backend (`internal/serve/server.go:19-28`, `internal/serve/static.go:8-19`) binds strictly to loopback HTTP and serves modular REST endpoints backed by `serve.Model` (`internal/serve/model.go:14-24`).

This approach fulfills the delta specifications:
1. **Anti-rubber-stamping in the multi-view shell** (`openspec/changes/control-room-ui-views/proposal.md:61-74`, `openspec/specs/approvals-web-ui/spec.md:26-48`): Single-card decisions and inline command/file:line evidence are preserved under `/api/approvals`, while bulk POST payloads return HTTP 400 (`internal/serve/handlers.go:161-176`).
2. **Batch and DAG wave inspection** (`proposal.md:75-88`): `GET /api/batch/lanes` exposes batch state, Kahn wave grouping (`internal/dag/waves.go:41-70`), lane statuses (`internal/lane/status.go:10-16`), deadlines (`internal/run/batch.go:40-43`), and barrier evaluation (`internal/barrier/barrier.go:21-60`).
3. **Shell-free feature and lease monitoring** (`proposal.md:89-97`): `GET /api/features`, `GET /api/features/{id}/attempts`, and `GET /api/leases` expose feature status, fences, and attempt CAS states (`internal/serve/model.go:128-227`).
4. **Reconciliation candidate inspection** (`proposal.md:98-106`): `GET /api/reconcile/requests` surfaces requests, candidates, check outcomes, and `CASResult` (`internal/serve/model.go:74-115, 278-323`).
5. **Lane demotion diagnosis** (`proposal.md:107-115`): Offending out-of-scope paths from `lane_note` events (`internal/run/run.go:423-430, 650-652`) and preserved worktrees (`internal/run/batch.go:50-52`) are surfaced for `deviated` lanes.

## Decision 1 — Modular REST Read Endpoints over `serve.Model`

**Choice**: Expose dedicated GET endpoints (`/api/approvals`, `/api/features`, `/api/features/{id}/attempts`, `/api/leases`, `/api/overlap/{feature_id}`, `/api/reconcile/requests`, `/api/batch/lanes`) served by `serve.NewHandler` and queried via `serve.Model`, while preserving backward-compatible `GET /api/state`.
**Alternatives considered**: Monolithic `/api/state` poll returning all dashboard state; multi-page `html/template` form postbacks with 303 redirects.
**Rationale**: Avoids retransmitting unbounded historical attempts, heavy `evidence_json` (`internal/ledger/schema.go:137`), and candidate output (`internal/serve/model.go:109`) on every 2-second interval, aligning with `serve.Model`'s granular read interface.
**Terminal consumer**: `serve.NewHandler` route multiplexing in `internal/serve/handlers.go:36-118`, which routes incoming GET requests to handler methods serializing `Model` DTOs.

## Decision 2 — Extend `serve.Model` with Batch and Lane Read Methods

**Choice**: Add `ListBatchLanes` and lane demotion diagnosis queries directly to `*serve.Model` (`internal/serve/model.go:14-24`), querying SQLite tables `lanes` (`internal/ledger/schema.go:18-32`) and `events` (`internal/ledger/schema.go:34-43`).
**Alternatives considered**: Ad-hoc SQL inside HTTP handler closures; invoking CLI subcommands or reading worktree filesystem directly from handlers.
**Rationale**: Enforces the architectural boundary that all UI reads route through the shell-free `serve.Model` layer (`internal/serve/model.go:14-16`), satisfying `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`) without altering SQLite schema (`schemaVersion` remains 5 at `internal/ledger/schema.go:10`).
**Terminal consumer**: `serve.NewHandler` batch handler invoking `Model.ListBatchLanes` to serve `GET /api/batch/lanes`, validated by `internal/serve/model_test.go:595-627`.

## Decision 3 — Tiered Client Polling with On-Demand Diagnostic Inspection

**Choice**: Restrict high-frequency 2-second polling in `internal/serve/static/app.js:96-97` to lightweight summary endpoints (`/api/approvals`, `/api/leases`, `/api/batch/lanes`), while fetching large payload fields (`evidence_json`, candidate `output`/`checks`, audit logs) on demand upon card expansion.
**Alternatives considered**: Polling all endpoints concurrently every 2s; manual-only refresh.
**Rationale**: Eliminates browser memory bloat and DOM rendering latency from re-parsing large text diffs and audit logs, ensuring real-time responsiveness for pending approvals and lease fences without degrading UI responsiveness.
**Terminal consumer**: `internal/serve/static/app.js:1-98` timer loop and card click handler executing asynchronous `fetch()` for detailed child endpoints.

## Decision 4 — Keyed In-Place DOM Updates for Live Supervision

**Choice**: Update panel DOM nodes in-place using deterministic ID keys (`card-${runID}-${laneID}`) rather than clearing container HTML via `containerEl.innerHTML = ''` (`internal/serve/static/app.js:45-46`).
**Alternatives considered**: Full container `innerHTML` replacement on every poll tick; importing external virtual-DOM or reactive UI libraries.
**Rationale**: Full `innerHTML` wipes destroy operator scroll position, collapse expanded diagnostic cards, and reset input focus during live supervision; vanilla keyed patching solves UI stability without external npm dependencies (`docs/prd.md:217-221`).
**Terminal consumer**: `renderState` and panel render routines in `internal/serve/static/app.js:22-70` mutating existing DOM elements by identifier.

## Decision 5 — Strict Single-Decision HTTP Mutations and Bulk Prohibition

**Choice**: Retain single-item POST endpoints (`/approvals/{runID}/{laneID}`) returning HTTP 400 for array/bulk bodies (`internal/serve/handlers.go:161-176`), and forbid "select all" or "approve all" controls in static assets (`internal/serve/static_test.go:11-39`).
**Alternatives considered**: Introducing a `/api/approvals/bulk` POST endpoint; client-side loop firing concurrent single-decision POSTs.
**Rationale**: Preserves the core anti-rubber-stamping governance rule (`openspec/specs/approvals-web-ui/spec.md:26-48`, `docs/prd.md:229-241`) requiring deliberate operator engagement for every approval decision.
**Terminal consumer**: `handleDecide` in `internal/serve/handlers.go:148-176` rejecting non-single requests with `http.StatusBadRequest`.

## Decision 6 — Client-Side Sanitization for Untrusted Diagnostics and Raw Outputs

**Choice**: Enforce `escapeHtml` / `textContent` sanitization on all dynamic text insertions (`internal/serve/static/app.js:51-55, 91-94`), specifically for candidate `output` (`internal/serve/model.go:109`), `evidence_json` (`68`), and `lane_note` demotion diagnosis (`internal/run/run.go:423-430`).
**Alternatives considered**: Rendering raw HTML directly via `.innerHTML`; server-side HTML template escaping.
**Rationale**: Candidate execution output and agent envelopes contain arbitrary strings and compiler logs from external LLMs/processes that could include malicious scripts or unescaped HTML tags.
**Terminal consumer**: `escapeHtml` and DOM construction in `internal/serve/static/app.js:51-55, 91-94`.

## Open Questions

- [ ] May the UI expose HTTP POST endpoints for reconciliation mutations (`approve`, `decline`, `cancel`, `renew`, `resolve`), or must it strictly present copy-paste CLI commands matching `cmd/lucind-ai/cli.go:1043-1065`? (Preserved from `openspec/changes/control-room-ui-views/explore.md:84` and `proposal.md:152`).
- [ ] Should lease and reconciliation countdown timers be formatted using a server-provided `remaining_seconds` field or computed on the client from `expires_at` using a server timestamp offset (`internal/serve/model.go:56, 84, 354-357`)?
- [ ] Should overlap `evidence_json` (`internal/serve/model.go:68`) and candidate checks be rendered as `<pre>` blocks with `escapeHtml` (`internal/serve/static/app.js:53, 91-94`) or via a lightweight, zero-dependency inline diff tokenizer?
- [ ] Note on sdd-design skill contract variance: This artifact adheres to the Lens A parallel decomposition packet contract (omitting file change tables and test strategies owned by lenses B and C) per packet instructions, rather than generating a complete monolithic `design.md`.
