# Approvals Web UI Specification

## Purpose

Serve a localhost interface for reviewing lane approvals — inline evidence, per-item decisions,
the merged batch's `opencode` review command, and approver accuracy.

## Requirements

### Requirement: Loopback Binding

The server MUST bind only to `127.0.0.1` and MUST reject a non-loopback `--addr`.

#### Scenario: Loopback listen

- GIVEN loopback address `127.0.0.1:7433`
- WHEN starting `lucind-ai serve`
- THEN the server MUST listen on loopback.

#### Scenario: Non-loopback rejected

- GIVEN non-loopback address `0.0.0.0:7433`
- WHEN starting `lucind-ai serve --addr 0.0.0.0:7433`
- THEN the server MUST reject binding and exit with an error.

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

### Requirement: Inline Evidence and Batch Review Command

Evidence MUST be command output or `file:line`, never a bare claim. The merged-batch view MUST
show the exact `opencode` RDD command to run.

#### Scenario: Evidence and command visible

- GIVEN pending items with command output or `file:line` evidence, and a merged batch ready for
  review
- WHEN the user opens the UI
- THEN evidence MUST render inline and the exact `opencode` command MUST be shown.

#### Scenario: Bare claim withheld

- GIVEN an item with neither command output nor a `file:line` reference
- WHEN the UI renders it
- THEN the system MUST NOT present an unsupported claim as evidence.

### Requirement: Approver Wrong-Approval Rate

The UI MUST show the signed-in user's own rate of approvals that later surfaced a defect in the
same packet — never another approver's rate.

#### Scenario: Zero defect history

- GIVEN an approver with zero flagged defects
- WHEN viewing their rate
- THEN the UI MUST display 0%.

#### Scenario: Own rate only

- GIVEN another approver has marked defects
- WHEN the current user opens the UI
- THEN those rows MUST NOT count toward the current user's displayed rate.
