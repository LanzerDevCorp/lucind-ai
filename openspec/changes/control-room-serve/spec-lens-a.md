# Spec Lens A — Capabilities & Requirements: Control Room Serve

## Assumed requirements

This change touches five capabilities: four new capabilities (`control-room-api-runs`, `control-room-api-features`, `control-room-api-reconcile`, and `control-room-events-stream`) and one existing capability (`approvals-web-ui`). New full specs define run and lane listing endpoints (2 requirements), feature inspection endpoints (1 requirement), reconciliation candidate endpoints (1 requirement), and real-time SSE event streaming (1 requirement). The existing `approvals-web-ui` delta spec modifies loopback binding to enforce linked worktree guards (1 modified requirement) and adds approval queue reads along with embedded static asset serving and JSON 404 routing (2 added requirements).

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `control-room-api-runs` | New | `openspec/specs/control-room-api-runs/spec.md` | |
| `control-room-api-features` | New | `openspec/specs/control-room-api-features/spec.md` | |
| `control-room-api-reconcile` | New | `openspec/specs/control-room-api-reconcile/spec.md` | |
| `control-room-events-stream` | New | `openspec/specs/control-room-events-stream/spec.md` | |
| `approvals-web-ui` | Existing | `openspec/changes/control-room-serve/specs/approvals-web-ui/spec.md` | `openspec/specs/approvals-web-ui/spec.md:1-83` |

## ADDED Requirements

### Requirement: Run Listing Endpoint

The server MUST expose a read-only JSON listing of all known runs at `GET /api/v1/runs` returning HTTP 200 with `Content-Type: application/json`.

**Terminal consumer**: `internal/serve/handlers.go:36-118` (new endpoint registered on serve HTTP mux).

### Requirement: Lane Listing by Run Endpoint

The server MUST expose a read-only JSON listing of lanes for a specified run ID at `GET /api/v1/lanes` returning HTTP 200 with `Content-Type: application/json` containing lane ID, executor, status, and start/finish timestamps.

**Terminal consumer**: `internal/serve/handlers.go:36-118` querying `internal/ledger/ledger.go:285-330`.

### Requirement: Feature Listing and Inspection Endpoint

The server MUST expose read-only JSON details for all features and individual feature models at `GET /api/v1/features` returning HTTP 200 with `Content-Type: application/json` containing status, attempt history, active leases, and overlap evidence.

**Terminal consumer**: `internal/serve/handlers.go:36-118` querying `internal/serve/model.go:128-266`.

### Requirement: Reconciliation Candidates Endpoint

The server MUST expose a read-only JSON listing of reconciliation requests and candidate conflict records at `GET /api/v1/reconciliations` returning HTTP 200 with `Content-Type: application/json`.

**Terminal consumer**: `internal/serve/handlers.go:36-118` querying `internal/serve/model.go:278-323`.

### Requirement: Real-Time Event Streaming via SSE

The server MUST stream ledger `events` and `integration_events` as Server-Sent Events with `Content-Type: text/event-stream` at `GET /api/v1/events/stream` using `http.Flusher`, and MUST terminate the push loop immediately when the client disconnects or request context is cancelled.

**Terminal consumer**: `internal/serve/handlers.go:36-118` tailing `internal/ledger/schema.go:34-43,171-180`.

### Requirement: Approval Queue Read Endpoint

The server MUST expose a read-only JSON listing of all pending lane approvals at `GET /api/v1/approvals` returning HTTP 200 with `Content-Type: application/json`.

**Terminal consumer**: `internal/serve/handlers.go:36-118` calling `internal/ledger/ledger.go:705-717`.

### Requirement: Embedded Static Assets and JSON API 404 Routing

The server MUST serve embedded static assets with JavaScript and CSS MIME types for UI routes and MUST return a JSON 404 payload for unmatched `/api/*` and `/api/v1/*` routes rather than falling back to HTML.

**Terminal consumer**: `internal/serve/handlers.go:39-77`, `internal/serve/static.go:8-18`.

## MODIFIED Requirements

### Requirement: Loopback Binding

The server MUST bind only to loopback addresses, MUST reject a non-loopback `--addr` with an error, and MUST refuse execution in linked worktrees before opening the ledger.
(Previously: Bound only to 127.0.0.1 and rejected non-loopback addresses without checking for linked worktrees before opening the ledger.)

**Live block**: `openspec/specs/approvals-web-ui/spec.md:10-25` (2 scenarios)

## Open Questions

- [ ] Whether cross-process SSE event streaming between `run` and `serve` relies on SQLite ID polling with backoff or direct OS IPC notifications (deferred to design).
- [ ] Whether an optional `--dev-static-dir` flag should be supported on `lucind-ai serve` to bypass embedded assets during local frontend development.
