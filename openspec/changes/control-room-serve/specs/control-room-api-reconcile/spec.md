# Delta for control-room-api-reconcile

## ADDED Requirements

### Requirement: Reconciliation Candidates Endpoint

The server MUST expose a read-only JSON listing of reconciliation requests and candidate conflict records at `GET /api/v1/reconciliations` returning HTTP 200 with `Content-Type: application/json`.

#### Scenario: List reconciliation records

- GIVEN reconciliation requests and candidates in the ledger
- WHEN `GET /api/v1/reconciliations`
- THEN return HTTP 200 with a JSON array of reconciliation records

#### Scenario: No reconciliations returns empty array

- GIVEN zero reconciliation requests in the ledger
- WHEN `GET /api/v1/reconciliations`
- THEN return HTTP 200 with `[]`
