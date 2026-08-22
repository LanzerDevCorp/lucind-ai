# Reconciliation Workspace Specification

## Purpose

Serve a dashboard workspace for inspecting pending reconciliation requests, resolution candidate records, automated check outcomes, CAS promotion evaluation results, and audit event timelines.

## Requirements

### Requirement: Reconciliation Candidate Inspection

The reconciliation workspace UI MUST display pending reconciliation requests,
resolution candidate records, automated check outcomes, CAS promotion
evaluation results, and immutable audit event timelines.

#### Scenario: Read-only reconciliation request and check inspection

- GIVEN a reconciliation request with candidate diffs, check outcomes, and CAS
  status
- WHEN the operator opens the reconciliation workspace
- THEN the UI MUST render request direction, allowed paths, model, candidate
  SHA, checks output, CAS result, and audit log

#### Scenario: Failed CAS promotion displays failure reason

- GIVEN a reconciliation request where compare-and-swap failed due to SHA mismatch
- WHEN the operator inspects the candidate
- THEN the UI MUST display CAS outcome `failed` alongside the recorded failure
  reason and candidate output
