# Approvals Web UI Specification

## Purpose

Localhost per-item approval UI with inline evidence, `opencode`, and wrong-approval rate.

## Requirements

### Requirement: Loopback Bind

Serve MUST bind `127.0.0.1`, reject non-loopback `--addr`, and omit approve-all.

#### Scenario: Localhost review
- GIVEN default address and pending items
- WHEN serve starts and the UI is shown
- THEN it MUST listen on `127.0.0.1` with no approve-all

#### Scenario: Rejected bind or bulk
- GIVEN non-loopback `--addr` or a multi-item request
- WHEN serve starts or the request is submitted
- THEN it MUST reject it

### Requirement: Per-Item Decisions

Items MUST start unselected and MUST be decided individually.

#### Scenario: Fresh load
- GIVEN pending items
- WHEN the page loads
- THEN every item MUST be unselected

#### Scenario: Unselected submit
- GIVEN an unselected item
- WHEN a decision is submitted
- THEN it MUST NOT record `approved` or `rejected`

### Requirement: Inline Evidence

Evidence MUST be command output or `file:line`, never a bare claim. The batch view MUST show the `opencode` command.

#### Scenario: Evidence visible
- GIVEN pending items with output or `file:line`
- WHEN the user opens the UI
- THEN evidence MUST be inline and `opencode` MUST be shown

#### Scenario: Bare claim withheld
- GIVEN an item with neither
- WHEN the UI renders it
- THEN it MUST NOT present a claim as evidence

### Requirement: Wrong-Approval Rate

The UI MUST show this user's marked-defect / approved rate.

#### Scenario: Own rate shown
- GIVEN this user has marked-defect approvals
- WHEN they open the UI
- THEN it MUST show that user's rate

#### Scenario: Other users excluded
- GIVEN another approver has marked defects
- WHEN the user opens the UI
- THEN those rows MUST NOT count
