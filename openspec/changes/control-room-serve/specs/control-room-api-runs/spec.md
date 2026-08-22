# Delta for control-room-api-runs

## ADDED Requirements

### Requirement: Run Listing Endpoint

The server MUST expose a read-only JSON listing of all known runs at `GET /api/v1/runs` returning HTTP 200 with `Content-Type: application/json`.

#### Scenario: List existing runs

- GIVEN recorded runs in the ledger
- WHEN `GET /api/v1/runs`
- THEN return HTTP 200 with a JSON array of run summaries

#### Scenario: Empty ledger returns empty array

- GIVEN an empty ledger with no runs
- WHEN `GET /api/v1/runs`
- THEN return HTTP 200 with `[]`

### Requirement: Lane Listing by Run Endpoint

The server MUST expose a read-only JSON listing of lanes for a specified run ID at `GET /api/v1/lanes` returning HTTP 200 with `Content-Type: application/json` containing lane ID, executor, status, and start/finish timestamps.

#### Scenario: List lanes for a run

- GIVEN a run with recorded lanes in the ledger
- WHEN `GET /api/v1/lanes?run_id=<runID>`
- THEN return HTTP 200 with a JSON array of lane objects with ID, executor, status, and timestamps

#### Scenario: Missing run ID returns 400

- GIVEN a request omitting the run ID
- WHEN `GET /api/v1/lanes`
- THEN return HTTP 400 Bad Request
