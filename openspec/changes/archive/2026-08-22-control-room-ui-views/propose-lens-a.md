# Proposal Lens A — Candidate & Approach: Control Room UI Views

## Selected Candidate & Approach

We select **Candidate 2: Modular REST endpoints with lazy-loaded vanilla JS panels**, confirming the frozen exploration recommendation (`openspec/changes/control-room-ui-views/explore.md:3, 20-23`).

The core approach preserves the zero-dependency embedded SPA architecture served by the Go standard library `net/http` and `embed.FS` (`internal/serve/server.go:19-28`, `internal/serve/static.go:8-19`, `docs/prd.md:217-221`), bound strictly to loopback addresses (`internal/serve/server.go:19-22, 57-73`). Instead of relying on a monolithic state endpoint or server-rendered HTML pages, the server and client are structured as follows:

1. **Modular REST Query Surface**:
   - `internal/serve/handlers.go:36-118` is refactored from serving solely `GET /api/state` (`internal/serve/handlers.go:79-85, 120-146`) and `/approvals/` decisions (`internal/serve/handlers.go:87-115, 148-211`) to exposing modular read endpoints backed by `serve.Model` (`internal/serve/model.go:14-24`):
     - `GET /api/approvals`: Serves pending approvals and approver wrong-approval rates (`internal/serve/handlers.go:16-21, 120-146`, `internal/ledger/schema.go:83-92`).
     - `GET /api/features`: Lists feature rows (`internal/serve/model.go:128-149`, `internal/ledger/schema.go:96-104`).
     - `GET /api/features/{id}/attempts`: Fetches integration attempts for a given feature (`internal/serve/model.go:167-188`, `internal/ledger/schema.go:106-120`).
     - `GET /api/leases`: Returns active feature leases with fences and expiration timestamps (`internal/serve/model.go:206-227`, `internal/ledger/schema.go:122-129`).
     - `GET /api/overlap/{feature_id}`: On-demand retrieval of overlap classifications and JSON payloads (`internal/serve/model.go:245-275`, `internal/ledger/schema.go:131-139`).
     - `GET /api/reconcile/requests`: Lists reconciliation requests with candidates and audit trails (`internal/serve/model.go:278-301, 345-350`, `internal/ledger/schema.go:141-179`).
     - `GET /api/batch/lanes`: Exposes batch execution status, per-lane lifecycle states (`internal/lane/status.go:10-16`), timeouts (`internal/run/batch.go:40-43`), barrier release state (`internal/barrier/barrier.go:21-60`), preserved worktree paths (`internal/run/batch.go:50-52`), and envelope compliance/demotion diagnostics (`internal/run/run.go:576-654`, `internal/result/result.schema.json:21-43`).

2. **Lazy-Loaded Client Architecture**:
   - `internal/serve/static/app.js:1-98` is organized into tabbed view panels (Batch/Wave Inspector, Approvals, Feature/Lease Monitor, Reconciliation Workspace, Lane Envelope Inspector).
   - High-frequency 2-second polling (`internal/serve/static/app.js:96-97`) is restricted to active, lightweight endpoints (`/api/approvals`, `/api/leases`, and `/api/batch/lanes`).
   - Heavy diagnostic data, including `evidence_json` (`internal/serve/model.go:68`), candidate check logs and outputs (`internal/serve/model.go:109-110`), and audit histories (`internal/serve/model.go:118-125`), is fetched on-demand when the operator expands a specific card.
   - All rendered content is sanitized via `escapeHtml` / `textContent` (`internal/serve/static/app.js:51-55, 91-94`). Bulk approval actions remain strictly rejected with HTTP 400 (`internal/serve/handlers.go:161-176`), ensuring single-card accountability (`internal/serve/static/app.js:63-66`, `docs/prd.md:229-241`).

This approach solves the problem by making feature lifecycles, leases, overlap evidence, and reconciliation queues visible without payload bloat or UI freezes, while preserving the single-binary zero-dependency deployment model.

## Conceptual Changes & Architecture Rationale

1. **Integration of `serve.Model` into HTTP Dispatch**:
   - *Current State*: `serve.NewHandler` (`internal/serve/handlers.go:36`) accepts only `*ledger.Ledger`, and `serveStateJSON` (`internal/serve/handlers.go:120-146`) directly invokes ledger query methods. Although `serve.Model` (`internal/serve/model.go:14-24`) provides a shell-free status query abstraction over feature-parent integration tables (`internal/ledger/schema.go:96-179`), it is currently unreferenced by `internal/serve/handlers.go`.
   - *Change*: `serve.NewHandler` accepts `*Model` (or instantiates `Model` from `*ledger.Ledger`), routing all feature-parent queries through `Model`'s typed query interface.

2. **Expansion of Model Read Surface for Batches and Lanes**:
   - *Current State*: `serve.Model` (`internal/serve/model.go:26-125`) only models feature-parent schema tables (`features`, `integration_attempts`, `feature_leases`, `overlap_evidence`, `reconciliation_requests`, `reconciliation_candidates`, `integration_events`). It lacks query methods for `runs`/`lanes` and `approvals` (`internal/ledger/schema.go:76-92`).
   - *Change*: Add read methods to `serve.Model` for batch/lane execution status and envelope results, enabling the UI to inspect DAG waves (`internal/dag/waves.go:41-70`), barrier evaluation outcomes (`internal/barrier/barrier.go:21-60`), lane deadlines (`internal/run/batch.go:40-43`), preserved worktrees (`internal/run/batch.go:50-52`), and path demotions (`internal/run/run.go:576-654`).

3. **Separation of Hot-State Polling from On-Demand Inspection**:
   - *Rationale*: Combining all dashboard state into `ServerState` (`internal/serve/handlers.go:16-21`) would serialize large historical diffs and candidate checks on every poll. Granular REST endpoints decouple dynamic status updates from heavy diagnostic inspection.

4. **Strict Localhost Read-Only Boundary**:
   - *Rationale*: Per PRD §8.3 (`docs/prd.md:217-221`) and `serve.Model` design (`internal/serve/model.go:14-16`), the web UI does not execute git or shell processes. Interactive actions are limited to single-decision ledger transactions (`internal/serve/handlers.go:148-211`), keeping browser operations safe and auditable.

## Alternatives Considered & Rejected

1. **Monolithic `/api/state` with Aggregated Model Data**:
   - *Concept*: Expand `ServerState` (`internal/serve/handlers.go:16-21`) to include all features, attempts, leases, overlap evidence, and reconciliation records in a single payload polled every 2s (`internal/serve/static/app.js:96-97`).
   - *Reason for Rejection*: Heavy overlap payloads (`evidence_json` in `internal/serve/model.go:68`) and candidate outputs (`internal/serve/model.go:109-110`) would be retransmitted continuously, causing severe network overhead, browser memory bloat, and UI stutter.

2. **Server-Rendered HTML (`html/template`) with Form Postbacks**:
   - *Concept*: Server-render multi-page HTML views (`/approvals`, `/features`, `/reconcile`) using `html/template` with 303 redirects and `<meta http-equiv="refresh">`.
   - *Reason for Rejection*: Full-page reloads destroy client-side UI focus, tear down timer state, break existing single-card POST-then-poll workflows (`internal/serve/static/app.js:72-89`), and disrupt automated server integration tests (`internal/serve/server_test.go:1-100`).

3. **External Frontend Framework and Build Pipeline (React/Vue/Svelte + npm)**:
   - *Concept*: Build a rich single-page application using modern frontend frameworks and npm build tooling.
   - *Reason for Rejection*: Violates core architectural requirement PRD §8.3 (`docs/prd.md:217-221`), which mandates zero external dependencies, no npm/bundlers, and pure Go `embed.FS` distribution (`internal/serve/static.go:8-19`).

## Open Questions

- [ ] May the UI expose HTTP POST endpoints for reconciliation mutations (`approve`, `decline`, `cancel`, `renew`, `resolve`), or must it strictly present copy-paste CLI commands matching `cmd/lucind-ai/cli.go:1043-1065`? (Escalated in `openspec/changes/control-room-ui-views/explore.md:84`).
- [ ] Should lease and reconciliation expiry timers be rendered from a server-computed `remaining_seconds` field or derived on the client from `expires_at` using a server timestamp offset (`internal/serve/model.go:56, 84, 354-357`)?
- [ ] Should overlap `evidence_json` (`internal/serve/model.go:68`) and candidate checks be formatted in simple `<pre class="evidence-block">` elements with `escapeHtml` (`internal/serve/static/app.js:53, 91-94`) or via a lightweight, zero-dependency inline diff tokenizer?
