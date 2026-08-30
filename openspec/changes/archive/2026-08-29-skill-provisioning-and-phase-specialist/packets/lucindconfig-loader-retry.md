---
id: lucindconfig-loader-retry
executor: agy
routed_by: Strict-TDD foundation work for tracked lucind.yaml role stacks and closed YAML configuration loading.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: 0cf9c6ff95b1788a94f264f57df45425af23c4c4
expected_parent_sha: 0cf9c6ff95b1788a94f264f57df45425af23c4c4
allowed_paths: ["internal/lucindconfig","lucind.yaml"]
---

# Packet lucindconfig-loader-retry

## Goal
Create the tracked `lucind.yaml` configuration loader with strict `KnownFields(true)`, role stacks, and optional skill budget.

## Why this is safe to dispatch now
It is an independent Wave 1 configuration unit; strict parsing and its tests establish the only accepted config shape before admission wiring.

## Preconditions
- Confirm a clean feature-targeted worktree and inspect existing root files.
- Read `tasks.md:56` and `design.md:43-48,105`.

## Allowed paths
- `internal/lucindconfig/`
- `lucind.yaml`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not read `openspec/config.yaml` in Go, modify `.lucind/skill-roots.yaml`, or add unrelated configuration keys.

## Done criteria
- [ ] RED then GREEN tests cover valid role stacks, optional budget, malformed YAML, and unknown-key rejection.
- [ ] Run `go test ./internal/lucindconfig -race -count=1` and `go test ./... -race -count=1` successfully.
- [ ] The loader is consumed by admission; cite terminal consumer and command evidence.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- [ ] Stop `blocked` if permissive YAML parsing or unrelated keys are required.
- [ ] Stop `blocked` for edits outside the whitelist, impossible criteria, unresolved design choices, or conflicting instructions.

## Context
`design.md:45-47` requires tracked `lucind.yaml`, role stacks, optional budget, and `KnownFields(true)`. `tasks.md:56` defines the unit.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
