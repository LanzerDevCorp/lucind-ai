# Explore Lens A — Problem & Candidates: Control Room UI Views

## Problem Space

The `lucind-ai serve` localhost server was introduced to host a blocking approval workflow for batch lane runs (`internal/serve/server.go:19-28`, `internal/serve/handlers.go:36-118`). Currently, its HTTP surface only serves `/api/state` and `/approvals/{runID}/{laneID}` (`internal/serve/handlers.go:79-115`), and its embedded web frontend is constrained to a single-purpose "lucind-ai approvals" view (`internal/serve/static/index.html:6, 144`, `internal/serve/static/app.js:22-70`).

With the introduction of feature-parent integration (schema v4 in `internal/ledger/schema.go:95-180`), the ledger tracks extensive distributed integration state:
- Registered feature branches, base commit SHAs, and expected parent commit SHAs (`internal/ledger/schema.go:96-104`).
- Integration attempts across execution lifecycles (`recorded`, `leased`, `combining`, `checking`, `cas_pending`, `promoted`, `blocked`, `failed`, `stale` in `internal/ledger/schema.go:106-120`).
- Expiring feature leases with monotonic fencing tokens (`internal/ledger/schema.go:122-129`).
- Structural and semantic overlap evidence records with classification tags (`required`, `warning`, `informational` in `internal/ledger/schema.go:131-139`).
- Reconciliation requests, direction bindings, candidate executions, and audit event logs (`internal/ledger/schema.go:141-179`).

To support querying this state without spawning shell commands or git processes, a dedicated query layer was implemented in `internal/serve/model.go:14-24` (`serve.Model`). It provides structured read operations including `ListFeatures`/`GetFeature` (`internal/serve/model.go:128-164`), `ListAttempts` (`internal/serve/model.go:167-188`), `ListLeases` (`internal/serve/model.go:206-227`), `ListOverlapEvidence` (`internal/serve/model.go:245-266`), `ListReconciliationRequests` (`internal/serve/model.go:278-292`), and `ListAuditEvents` (`internal/serve/model.go:326-343`).

However, `serve.Model` is not wired to HTTP handlers (`internal/serve/handlers.go:36`), leaving the web UI unable to display feature lifecycles, active leases, overlap evidence, or reconciliation queues. Operators are forced to rely solely on CLI commands (`cmd/lucind-ai/cli.go:820-915`, `cmd/lucind-ai/cli.go:1043-1441`) like `lucind-ai feature status` and `lucind-ai reconcile approve/decline/cancel/renew/resolve`.

The motivation for `control-room-ui-views` is to expand `lucind-ai serve` into a unified localhost Control Room dashboard, exposing feature state, lease status, overlap evidence inspection, and reconciliation direction reviews alongside lane approvals.

## Candidate Approaches

### Candidate 1 — Monolithic State Aggregation with Single-Page Tabbed UI

**Approach**: Extend `ServerState` (`internal/serve/handlers.go:16-21`) to aggregate all domain state (lane approvals, features, attempts, leases, overlap evidence, and reconciliation requests) queried via `serve.Model` (`internal/serve/model.go:14-24`) into a single `/api/state` response. In `internal/serve/static/index.html` and `app.js`, build a tabbed single-page UI switching between "Lane Approvals", "Feature Branches & Leases", "Overlap Evidence", and "Reconciliation Requests". Add action POST endpoints (`/api/reconcile/{id}/approve`, `/api/reconcile/{id}/decline`) invoking `reconcile.Service` (`internal/reconcile/reconcile.go:89-96`).
**Pros**: Reuses the existing periodic polling loop (`internal/serve/static/app.js:97`) and stdlib embedded asset pipeline (`internal/serve/static.go:8-19`) without adding client-side routing libraries or multiple timers.
**Cons**: Polling payload size grows substantially as historical attempts and verbose overlap evidence JSON payloads (`internal/ledger/schema.go:137`) accumulate in the ledger.
**Feasibility**: High. `serve.Model` (`internal/serve/model.go:128-343`) and `ledger.Ledger` (`internal/ledger/ledger.go:131-148`) already expose all required aggregation queries.

### Candidate 2 — Modular REST API with Lazy-Loaded View Panels

**Approach**: Expose resource-specific REST endpoints in `internal/serve/handlers.go:36-118` mirroring `serve.Model` methods: `/api/approvals`, `/api/features`, `/api/features/{id}/attempts`, `/api/features/{id}/leases`, `/api/overlap/{featureID}`, and `/api/reconcile/requests`. Modularize `internal/serve/static/app.js` into independent view controllers. The dashboard loads summary cards on start, while detailed attempt timelines and overlap evidence JSON are fetched on demand when an operator expands a feature or reconciliation request.
**Pros**: Efficient network utilization and payload sizing; high-frequency polling can be restricted to active leases (`internal/serve/model.go:206-227`) and pending approvals (`internal/serve/handlers.go:121`), while heavy overlap diffs are retrieved only upon user selection.
**Cons**: Requires granular handler dispatch logic in `internal/serve/handlers.go` and more state management inside `internal/serve/static/app.js`.
**Feasibility**: High. Handlers can pattern-match subpaths identically to `/approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115`), backed directly by `serve.Model` methods (`internal/serve/model.go:128-343`).

### Candidate 3 — Server-Side Rendered HTML Views with Form Postbacks

**Approach**: Replace client-side JSON rendering with Go stdlib `html/template` rendering for separate views (`/approvals`, `/features`, `/reconcile`). Embed template files via `go:embed` (`internal/serve/static.go:8-19`). User actions (such as reconciliation approval or lane decisions) submit standard HTML forms with HTTP 303 redirects, using HTTP `meta-refresh` or minimal vanilla JS for live polling.
**Pros**: Eliminates client-side DOM rendering and JSON serialization logic in `internal/serve/static/app.js`; renders strongly typed models from `serve.Model` (`internal/serve/model.go:26-125`) directly on the server.
**Cons**: Full-page reloads disrupt operator workflow during active monitoring; breaks consistency with the existing interactive approval workflow (`internal/serve/static/app.js:72-89`) and tests in `internal/serve/server_test.go:1-100`.
**Feasibility**: Medium. Requires refactoring static file serving (`internal/serve/static.go:8-19`) and rewriting existing UI test expectations in `internal/serve/handlers.go` / `internal/serve/server_test.go`.

## Initial Recommendations

Candidate 2 (Modular REST API with Lazy-Loaded View Panels) is the recommended approach. It preserves the clean, zero-dependency embedded SPA architecture (`internal/serve/static.go:8-19`, `internal/serve/server.go:19-28`) while avoiding payload bloat from large overlap evidence records (`internal/ledger/schema.go:137`). It directly utilizes the existing granular query methods on `serve.Model` (`internal/serve/model.go:128-343`) and cleanly separates fast approval/lease updates from deep-dive reconciliation views.

## Open Questions

- [ ] Should reconciliation actions (e.g., approve direction, decline, cancel) be directly executable via POST endpoints in `lucind-ai serve`, or should the UI present copy-paste CLI commands matching `cmd/lucind-ai/cli.go:1043-1441`?
- [ ] Should overlap evidence diff payloads (`internal/ledger/schema.go:137`, `internal/serve/model.go:68`) be rendered as formatted diff components or raw structured JSON in the UI?
