# Control Room Shared Store Specification

## Purpose

Client state that survives view switches and refreshes from the loopback server.

## Requirements

### Requirement: Centralized Reactive Client Store

The client MUST keep centralized state across view transitions, cache GET /api/state results polled every 2000ms, and dispatch updates to subscribed views without a full-page re-fetch.

#### Scenario: Cached state renders on view revisit

- GIVEN approvals state cached in the shared store
- WHEN navigating away and returning to #/approvals
- THEN the view MUST render cached state immediately without a blank flash

#### Scenario: Polling timer fetches state and notifies subscribers

- GIVEN the shared store with registered subscribers
- WHEN the 2000ms polling timer fires
- THEN the store MUST fetch GET /api/state and notify subscribers

#### Scenario: Polling failure retains existing cache

- GIVEN cached state in the shared store
- WHEN GET /api/state returns HTTP 500
- THEN the store MUST preserve cache and notify subscribers of the error
