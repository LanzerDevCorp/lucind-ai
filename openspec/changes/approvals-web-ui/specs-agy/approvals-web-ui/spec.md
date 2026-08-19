# Approvals Web UI Specification

## Purpose

Serve a localhost interface for inspecting lane approvals with inline evidence, making individual decisions, and tracking approver accuracy.

## Requirements

### Requirement: Localhost Binding

The server MUST bind only to `127.0.0.1` and MUST reject non-loopback addresses.

#### Scenario: Loopback listen

- GIVEN loopback address `127.0.0.1:7433`
- WHEN starting `lucind-ai serve`
- THEN the server MUST listen on loopback.

#### Scenario: Non-loopback rejected

- GIVEN non-loopback address `0.0.0.0:7433`
- WHEN starting `lucind-ai serve --addr 0.0.0.0:7433`
- THEN the server MUST reject binding and exit with error.

### Requirement: Individual Decisions Without Bulk Approval

The UI MUST enforce individual decisions starting unselected and MUST NOT provide bulk approval controls.

#### Scenario: Decide single item

- GIVEN pending items with none selected
- WHEN the user approves one item
- THEN the system MUST record the decision for that item only.

#### Scenario: Bulk approval omitted

- GIVEN multiple pending items
- WHEN viewing the UI or posting requests
- THEN the UI MUST omit bulk actions and the server MUST reject multi-item requests.

### Requirement: Inline Evidence and Accuracy Tracking

The UI MUST display inline command output or `file:line` evidence and approver wrong-approval rates.

#### Scenario: Display evidence and rate

- GIVEN a pending item with command evidence
- WHEN the user inspects the item
- THEN the UI MUST render evidence inline and display the approver's wrong-approval rate.

#### Scenario: Zero defect history

- GIVEN an approver with zero flagged defects
- WHEN viewing the approver rate
- THEN the UI MUST display 0% wrong-approval rate.
