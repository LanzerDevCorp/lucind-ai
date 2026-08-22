# Delta for approvals-web-ui

## ADDED Requirements

### Requirement: Approvals Inbox View Integration

The approvals inbox MUST mount as a registered view at `#/approvals` inside the layout shell while still enforcing loopback-only access, inline `file:line` or command-output evidence, individual POST `/approvals/{runID}/{laneID}` decisions, and HTTP 400 rejection of bulk or array approval bodies.

#### Scenario: Individual approval decision posted

- GIVEN a pending card in #/approvals with valid file:line evidence
- WHEN the operator submits an approval
- THEN the client MUST POST /approvals/{runID}/{laneID} with {"decision":"approved"} and receive HTTP 200

#### Scenario: Items start unselected and bare prose is withheld

- GIVEN a pending item with bare prose lacking file:line or command output
- WHEN approvals view renders the item
- THEN the item MUST start unselected and display a missing-evidence placeholder

#### Scenario: Bulk approval payload rejected

- GIVEN a multi-item approval payload posted to /approvals/run-1/lane-1
- WHEN the handler processes the request
- THEN the server MUST return HTTP 400 Bad Request
