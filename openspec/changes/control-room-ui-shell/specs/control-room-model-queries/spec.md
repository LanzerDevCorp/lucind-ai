# Control Room Model Queries Specification

## Purpose

Read-only JSON inspection of ledger integration state over loopback HTTP.

## Requirements

### Requirement: Read-Only Model Inspection Endpoints

The HTTP handler MUST expose read-only GET endpoints returning JSON for features, a feature by id, attempts, leases, overlap evidence, reconciliation requests, and audit events, without running a shell or git subprocess and without mutating ledger state.

#### Scenario: Query features returns JSON list without writes

- GIVEN features in the ledger
- WHEN a client requests GET /api/features
- THEN the server MUST return HTTP 200 with a JSON array of features and MUST NOT mutate the ledger

#### Scenario: Query single feature by ID

- GIVEN feature feat-1 exists and feat-missing does not
- WHEN requesting GET /api/features/feat-1 and GET /api/features/feat-missing
- THEN the server MUST return HTTP 200 for feat-1 and HTTP 404 for feat-missing

#### Scenario: Non-GET methods rejected

- GIVEN model query route /api/features
- WHEN a client sends POST or DELETE
- THEN the server MUST return HTTP 405 Method Not Allowed
