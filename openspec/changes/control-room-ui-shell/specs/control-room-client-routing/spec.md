# Control Room Client Routing Specification

## Purpose

Hash routing and view lifecycle so operators switch views without reloading the shell.

## Requirements

### Requirement: Client-Side Routing and View Lifecycle

The client MUST provide hash routing (`#/route`) with a view registry that mounts the target view into `#view-outlet`, tears down the previous view by unmounting DOM and cancelling active timers and listeners, and patches DOM nodes in place without replacing the outlet's entire `innerHTML`.

#### Scenario: Hash route transition clears view timers

- GIVEN the approvals view mounted with an active 2000ms timer
- WHEN the operator navigates to another registered hash route
- THEN the router MUST cancel the timer, unmount approvals, and mount the target view into #view-outlet

#### Scenario: Targeted DOM updates on poll ticks

- GIVEN an active approvals view displaying pending cards
- WHEN the store dispatches fresh state
- THEN the view MUST patch cards in place without replacing parent #view-outlet innerHTML

#### Scenario: Unregistered route shows not-found view

- GIVEN registered hash routes in the view registry
- WHEN the browser navigates to an unregistered hash
- THEN the router MUST render a not-found view in #view-outlet without page reload
