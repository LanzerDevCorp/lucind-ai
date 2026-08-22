# Delta for approvals-web-ui

## ADDED Requirements

### Requirement: Approval Queue Read Endpoint

The server MUST expose a read-only JSON listing of all pending lane approvals at `GET /api/v1/approvals` returning HTTP 200 with `Content-Type: application/json`.

#### Scenario: List pending approvals

- GIVEN pending lane approvals in the ledger
- WHEN `GET /api/v1/approvals`
- THEN return HTTP 200 with a JSON array of pending approvals

#### Scenario: Zero pending approvals returns empty array

- GIVEN zero pending lane approvals
- WHEN `GET /api/v1/approvals`
- THEN return HTTP 200 with `[]`

### Requirement: Embedded Static Assets and JSON API 404 Routing

The server MUST serve embedded static assets with JavaScript and CSS MIME types for UI routes and MUST return a JSON 404 payload for unmatched `/api/*` and `/api/v1/*` routes rather than falling back to HTML.

#### Scenario: Serve static assets with correct MIME

- GIVEN embedded `app.js` and `app.css`
- WHEN `GET /app.js` or `GET /app.css`
- THEN return HTTP 200 with `application/javascript` or `text/css`

#### Scenario: Unmatched API route returns JSON 404

- GIVEN an unmatched path under `/api/*`
- WHEN `GET /api/v1/nonexistent`
- THEN return HTTP 404 with JSON rather than HTML

## MODIFIED Requirements

### Requirement: Loopback Binding

The server MUST bind only to loopback addresses, MUST reject a non-loopback `--addr` with an error, and MUST refuse execution in linked worktrees before opening the ledger.
(Previously: Bound only to 127.0.0.1 and rejected non-loopback addresses without checking for linked worktrees before opening the ledger.)

#### Scenario: Loopback listen

- GIVEN loopback address `127.0.0.1:7433`
- WHEN starting `lucind-ai serve`
- THEN the server MUST listen on loopback.

#### Scenario: Non-loopback rejected

- GIVEN non-loopback address `0.0.0.0:7433`
- WHEN starting `lucind-ai serve --addr 0.0.0.0:7433`
- THEN the server MUST reject binding and exit with an error.

#### Scenario: Linked worktree execution refused before ledger open

- GIVEN execution inside a linked git worktree
- WHEN starting `lucind-ai serve`
- THEN exit 1 with an error on stderr before opening the ledger
