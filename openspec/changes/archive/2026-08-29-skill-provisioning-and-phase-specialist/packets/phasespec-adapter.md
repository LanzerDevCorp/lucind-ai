---
id: phasespec-adapter
executor: agy
routed_by: Strict-TDD phase-specialist adapter work after the configuration foundation is available.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: 416fb50d867236471d57ff93c0ffd105e1bf2ff1
expected_parent_sha: 416fb50d867236471d57ff93c0ffd105e1bf2ff1
allowed_paths: ["internal/phasespec"]
read_only_paths: ["internal/lucindconfig"]
---

# Packet phasespec-adapter

## Goal
Implement a fail-closed `gentle-ai sdd-status --json` adapter that sequences accepted/merged lenses before synthesis and writes the canonical phase artifact.

## Why this is safe to dispatch now
This is a new isolated package with a mockable status command and no `cmd` import; CLI admission remains a later unit.

## Preconditions
- Confirm a clean feature-targeted worktree.
- Read `tasks.md:74-75` and `design.md:28-36,106`, including the phase specialist sequencing rule.

## Allowed paths
- `internal/phasespec/`

## Read-only inputs
- `internal/lucindconfig/`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not invoke real dispatch, mutate PR state, select skills, or modify CLI/assets.

## Done criteria
- [ ] RED then GREEN tests prove malformed status JSON and CLI errors fail closed with no filesystem mutation.
- [ ] Tests prove synthesis cannot start before all required lenses are accepted and merged, then run focused tests and `go test ./... -race -count=1`.
- [ ] The adapter consumes status data and writes only the specified `openspec/changes/<change>/<phase>.md` artifact when permitted; cite evidence.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- [ ] Stop `blocked` on malformed/error status, premature synthesis, filesystem mutation before admission, or any out-of-scope edit.
- [ ] Stop `blocked` for impossible criteria, unresolved alternatives, or conflicting instructions.

## Context
`design.md:12,30-35,86` and `tasks.md:74-75` require read-only status ingestion and lens-before-synthesis sequencing.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
