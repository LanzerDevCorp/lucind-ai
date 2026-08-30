---
id: skillroots-resolution
executor: agy
routed_by: Strict-TDD foundation work for ordered machine-local skill-root resolution and fail-closed diagnostics.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: 8d0a3d13ed712c2fcbd01ca179763baa6c296727
expected_parent_sha: 0cf9c6ff95b1788a94f264f57df45425af23c4c4
allowed_paths: ["internal/skillroots"]
---

# Packet skillroots-resolution

## Goal
Implement ordered skill-root resolution with `~` expansion, data-only loading, and fail-closed missing-skill diagnostics.

## Why this is safe to dispatch now
The package is an isolated Wave 1 foundation and does not execute skill content or touch dispatch state.

## Preconditions
- Confirm a clean feature-targeted worktree.
- Read `tasks.md:54-55` and `design.md:43-48`.

## Allowed paths
- `internal/skillroots/`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not edit `.lucind/skill-roots.yaml`, execute `SKILL.md`, hash external skills as a gate, or modify admission.

## Done criteria
- [ ] RED then GREEN tests prove ordered first-match lookup, tilde expansion, missing diagnostics, and loading YAML/Markdown strictly as data.
- [ ] Run focused package tests and `go test ./... -race -count=1` successfully.
- [ ] The resolver is consumed by the later admission unit; cite terminal consumer and evidence.
- [ ] Commit conventionally, no AI attribution, clean status, and latest commit evidence recorded.

## Hard stops
- [ ] Stop `blocked` if implementation executes documentation-like files or requires unordered resolution.
- [ ] Stop `blocked` for out-of-scope edits, impossible criteria, unresolved alternatives, or contradictory instructions.

## Context
`design.md:45-47,81-84` defines ordered roots, `~`, and data-only resolution. `tasks.md:54-55` defines the tests and package.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
