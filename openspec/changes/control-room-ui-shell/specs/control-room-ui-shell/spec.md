# Control Room UI Shell Specification

## Purpose

Persistent header chrome, navigation tabs, and a view outlet for the loopback serve UI.

## Requirements

### Requirement: Layout Shell and Global Chrome

The server MUST serve a modular single-page application layout with persistent header chrome showing connection or freshness status, signed-in approver identity, and the signed-in approver's wrong-approval rate, plus navigation tabs and a main `#view-outlet`. Registered views MUST mount in that outlet without a full page reload.

#### Scenario: SPA shell initial load

- GIVEN loopback server at 127.0.0.1:7433
- WHEN an HTTP client requests GET /
- THEN the server MUST return HTTP 200 HTML with header chrome, tabs, and #view-outlet

#### Scenario: State polling refreshes header metrics

- GIVEN the mounted shell and GET /api/state with approver "alice" and rate 0.05
- WHEN the store applies the update
- THEN header chrome MUST display approver "alice" and defect rate "5.0%"

#### Scenario: Network failure shows disconnected indicator

- GIVEN an active shell whose state poll fails
- WHEN polling fails
- THEN header chrome MUST display a disconnected indicator
