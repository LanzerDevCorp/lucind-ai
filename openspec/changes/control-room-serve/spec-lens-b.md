# Spec Lens B — Scenarios & Coverage: Control Room Serve

## Assumed requirements

This change touches five capabilities: four new (`control-room-api-runs`, `control-room-api-features`, `control-room-api-reconcile`, `control-room-events-stream`) and one modified (`approvals-web-ui`). New capabilities add read-only JSON REST endpoints for runs, lanes, features, and reconciliations, plus an SSE stream of ledger events. The `approvals-web-ui` capability adds linked-worktree refusal before ledger open, approval queue reads (`/api/v1/approvals`), JSON 404s under `/api/*`, and preserves loopback binding, static asset MIME types, and single-item decisions.

## Scenarios

### Requirement: Run Listing Endpoint

#### Scenario: List existing runs
- GIVEN recorded runs in the ledger
- WHEN `GET /api/v1/runs`
- THEN return HTTP 200 with a JSON array of run summaries

#### Scenario: Empty ledger returns empty array
- GIVEN an empty ledger with no runs
- WHEN `GET /api/v1/runs`
- THEN return HTTP 200 with `[]`

### Requirement: Lane Listing by Run Endpoint

#### Scenario: List lanes for a run
- GIVEN a run with recorded lanes in the ledger
- WHEN `GET /api/v1/lanes?run_id=<runID>`
- THEN return HTTP 200 with a JSON array of lane objects with ID, executor, status, and timestamps

#### Scenario: Missing run ID returns 400
- GIVEN a request omitting the run ID
- WHEN `GET /api/v1/lanes`
- THEN return HTTP 400 Bad Request

### Requirement: Feature Listing and Inspection Endpoint

#### Scenario: List all features
- GIVEN features with attempts, leases, and overlap in the ledger
- WHEN `GET /api/v1/features`
- THEN return HTTP 200 with a JSON array of feature models

#### Scenario: Missing feature returns 404
- GIVEN a non-existent feature ID
- WHEN `GET /api/v1/features/nonexistent`
- THEN return HTTP 404 with a JSON error payload

### Requirement: Reconciliation Candidates Endpoint

#### Scenario: List reconciliation records
- GIVEN reconciliation requests and candidates in the ledger
- WHEN `GET /api/v1/reconciliations`
- THEN return HTTP 200 with a JSON array of reconciliation records

#### Scenario: No reconciliations returns empty array
- GIVEN zero reconciliation requests in the ledger
- WHEN `GET /api/v1/reconciliations`
- THEN return HTTP 200 with `[]`

### Requirement: Real-Time Event Streaming via SSE

#### Scenario: Live event stream flushes rows
- GIVEN `serve` running and events being appended to the ledger
- WHEN `GET /api/v1/events/stream` with `Accept: text/event-stream`
- THEN stream event rows using `text/event-stream` and flush frames as appended

#### Scenario: Client disconnect terminates loop
- GIVEN an active SSE client stream
- WHEN the client disconnects closing request context
- THEN terminate the push loop with no leaked goroutines

### Requirement: Approval Queue Read Endpoint

#### Scenario: List pending approvals
- GIVEN pending lane approvals in the ledger
- WHEN `GET /api/v1/approvals`
- THEN return HTTP 200 with a JSON array of pending approvals

#### Scenario: Zero pending approvals returns empty array
- GIVEN zero pending lane approvals
- WHEN `GET /api/v1/approvals`
- THEN return HTTP 200 with `[]`

### Requirement: Embedded Static Assets and JSON API 404 Routing

#### Scenario: Serve static assets with correct MIME
- GIVEN embedded `app.js` and `app.css`
- WHEN `GET /app.js` or `GET /app.css`
- THEN return HTTP 200 with `application/javascript` or `text/css`

#### Scenario: Unmatched API route returns JSON 404
- GIVEN an unmatched path under `/api/*`
- WHEN `GET /api/v1/nonexistent`
- THEN return HTTP 404 with JSON rather than HTML

### Requirement: Loopback Binding

#### Scenario: Loopback listen succeeds
- GIVEN loopback address `127.0.0.1:7433` in primary repository
- WHEN starting `lucind-ai serve --addr 127.0.0.1:7433`
- THEN listen on loopback and serve HTTP requests

#### Scenario: Linked worktree execution refused before ledger open
- GIVEN execution inside a linked git worktree
- WHEN starting `lucind-ai serve`
- THEN exit 1 with an error on stderr before opening the ledger

#### Scenario: Non-loopback address rejected
- GIVEN non-loopback address `0.0.0.0:7433`
- WHEN starting `lucind-ai serve --addr 0.0.0.0:7433`
- THEN return `ErrNonLoopback` and exit 1 with an error on stderr

### Requirement: Individual Decisions Without Bulk Approval

#### Scenario: Single decision recorded
- GIVEN a pending approval for a run and lane
- WHEN `POST /approvals/{runID}/{laneID}` with `{"decision":"approved","approver":"alice"}`
- THEN record the decision in the ledger and return HTTP 200 `{"ok":true}`

#### Scenario: Bulk array or composite payload rejected
- GIVEN a payload of `[{"decision":"approved"}]` or composite `{approvals: ...}`
- WHEN `POST /approvals/{runID}/{laneID}`
- THEN return HTTP 400 Bad Request rejecting bulk approval

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Run Listing Endpoint | covered | covered | missing | `internal/serve/handlers.go:36-118` (new seam required) |
| Lane Listing by Run Endpoint | covered | missing | covered | `internal/serve/handlers.go:36-118`, `internal/ledger/ledger.go:285-330` |
| Feature Listing and Inspection Endpoint | covered | missing | covered | `internal/serve/handlers.go:36-118`, `internal/serve/model.go:128-266` |
| Reconciliation Candidates Endpoint | covered | covered | missing | `internal/serve/handlers.go:36-118`, `internal/serve/model.go:278-323` |
| Real-Time Event Streaming via SSE | covered | covered | missing | `internal/serve/handlers.go:36-118`, `internal/ledger/schema.go:34-43,171-180` |
| Approval Queue Read Endpoint | covered | covered | missing | `internal/serve/handlers.go:36-118`, `internal/ledger/ledger.go:705-717` |
| Embedded Static Assets and JSON API 404 Routing | covered | missing | covered | `internal/serve/handlers.go:39-77`, `internal/serve/static.go:8-18` |
| Loopback Binding | covered | covered | covered | `cmd/lucind-ai/cli.go:691-707`, `internal/serve/server.go:14,20-22,55-73` |
| Individual Decisions Without Bulk Approval | covered | missing | covered | `internal/serve/handlers.go:87-115,148-211`, `internal/serve/server_test.go:42-135` |

## Untestable Assertions

None

## Open Questions

- [ ] Omitted secondary scenarios under budget: error states for GET endpoints (`Run Listing`, `Reconciliations`, `Approvals`, `SSE Stream`) and edge cases (`Lane Listing`, `Features`, `Decisions`, `Static Assets`) were omitted to prioritize happy paths and core safety guards.
- [ ] Whether cross-process SSE event streaming between `run` and `serve` uses SQLite `id` cursor polling with backoff or OS IPC notifications.
- [ ] Whether an optional `--dev-static-dir` flag should be supported on `lucind-ai serve` during local UI development.
- [ ] Execution divergence from `sdd-spec` skill: writing only `spec-lens-b.md` per packet precedence.
