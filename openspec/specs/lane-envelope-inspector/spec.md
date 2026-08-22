# Lane Envelope Inspector Specification

## Purpose

Serve a dashboard view for inspecting demoted lane executions, displaying offending out-of-scope paths from diagnosis notes and preserved worktree locations without direct disk reads.

## Requirements

### Requirement: Lane Demotion Diagnosis

When a lane execution is demoted from Done to Deviated due to modifying paths
outside declared `allowed_paths`, the UI MUST display status `deviated`, the
offending-path diagnosis text recorded in the lane note, and the preserved
worktree path location.

#### Scenario: Deviated lane displays offending paths and preserved worktree

- GIVEN a lane execution demoted from Done to Deviated due to modifying paths
  outside `allowed_paths`
- WHEN the operator inspects the lane envelope view
- THEN the UI MUST display status `deviated`, offending paths from the lane
  note, and the preserved worktree path

#### Scenario: Multiple out-of-scope paths formatted in diagnosis note

- GIVEN a lane modifying multiple files outside declared `allowed_paths`
- WHEN the operator inspects the demotion diagnosis
- THEN the inspector MUST display the full comma-separated list of offending
  paths captured in the lane note

#### Scenario: Preserved worktree inspected without direct disk read

- GIVEN a demoted lane with a preserved worktree on disk
- WHEN the operator inspects the lane envelope
- THEN diagnosis data MUST come from the ledger and MUST NOT require reading
  worktree files from disk
