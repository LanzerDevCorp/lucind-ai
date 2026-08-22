# Delta for Approvals Web UI

## MODIFIED Requirements

### Requirement: Individual Decisions Without Bulk Approval

Items MUST start unselected. Every decision MUST be made individually via a
single-item request. The multi-panel dashboard UI MUST NOT provide a
bulk/"approve all" control, and the server MUST reject a multi-item or array
request body with HTTP 400 Bad Request.
(Previously: Same individual-decision and bulk-rejection rules on the standalone approvals page.)

#### Scenario: Fresh load starts unselected

- GIVEN pending items
- WHEN the page loads
- THEN every item MUST be unselected.

#### Scenario: Unselected item cannot be decided

- GIVEN an unselected item
- WHEN a decision is submitted for it
- THEN the system MUST NOT record `approved` or `rejected`.

#### Scenario: Bulk request rejected

- GIVEN multiple pending items
- WHEN a multi-item approval request is posted
- THEN the server MUST reject it with HTTP 400 Bad Request and MUST NOT record
  decisions; the UI MUST NOT expose a control that could produce one.

#### Scenario: Single-item decision recorded

- GIVEN a pending lane approval
- WHEN the operator submits a decision for that single lane
- THEN the server MUST return HTTP 200 and persist the decision.
