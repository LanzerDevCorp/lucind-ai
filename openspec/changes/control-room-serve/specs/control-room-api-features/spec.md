# Delta for control-room-api-features

## ADDED Requirements

### Requirement: Feature Listing and Inspection Endpoint

The server MUST expose read-only JSON details for all features and individual feature models at `GET /api/v1/features` returning HTTP 200 with `Content-Type: application/json` containing status, attempt history, active leases, and overlap evidence.

#### Scenario: List all features

- GIVEN features with attempts, leases, and overlap in the ledger
- WHEN `GET /api/v1/features`
- THEN return HTTP 200 with a JSON array of feature models

#### Scenario: Missing feature returns 404

- GIVEN a non-existent feature ID
- WHEN `GET /api/v1/features/nonexistent`
- THEN return HTTP 404 with a JSON error payload
