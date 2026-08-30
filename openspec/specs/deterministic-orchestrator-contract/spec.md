# Deterministic Orchestrator Contract Specification

## Purpose

Define the deterministic preflight verification, phase routing, wave sequencing, workspace isolation, and fail-closed halt contract across Claude Code and OpenCode runtimes.

## Requirements

### Requirement: Cross-Runtime Orchestrator Preflight and Sequencing

The orchestrator MUST execute deterministic preflight verification across Claude Code and OpenCode, enforce fail-closed phase routing and wave planning, preserve workspace isolation for concurrent sibling worktrees, and halt execution before allocating worktrees if skill parity or schema checks fail.

#### Scenario: Preflight verification succeeds across runtimes

- GIVEN identical canonical skill trees in Claude Code and OpenCode and a clean repository root
- WHEN orchestrator preflight executes before dispatch
- THEN preflight MUST exit 0 and allow phase routing and wave planning to proceed

#### Scenario: Concurrent execution in sibling worktree preserves workspace isolation

- GIVEN an active lane execution in a sibling worktree
- WHEN preflight runs in the primary workspace
- THEN fork-local roots, ledgers, and worktrees MUST remain isolated without cross-talk

#### Scenario: Stale skill copy or schema mismatch halts before allocation

- GIVEN the OpenCode skill copy differs from the canonical Claude skill or the binary schema is outdated
- WHEN orchestrator preflight executes
- THEN preflight MUST exit non-zero and halt execution before any worktree allocation

#### Scenario: Target repository without its own skill tree or schema file skips preflight cleanly

- GIVEN the repository root has neither the canonical Claude skill tree nor an on-disk result schema file (any project other than the orchestrator's own source tree, since a real plugin install never places either artifact inside a consumer project's own working directory)
- WHEN orchestrator preflight executes
- THEN preflight MUST exit 0 without treating the absence as drift, and MUST still enforce the full fail-closed comparison for whichever artifact is present
