---
id: skillset-engine-final
executor: agy
routed_by: Strict-TDD foundation work for deterministic skill derivation, budget constant, and root-independent body digesting.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: e2a3bbb5d7381d44144147f417b1094ec41a8f84
expected_parent_sha: e2a3bbb5d7381d44144147f417b1094ec41a8f84
allowed_paths: ["internal/skillset"]
---

# Packet skillset-engine-final

## Goal
Implement deterministic `skillset.Derive`, `DefaultSkillBudget = 3`, and root-independent `DigestBody` with focused tests.

## Why this is safe to dispatch now
This is a pure new package with no existing callers, explicitly selected as the remaining Wave 1 foundation unit.

## Preconditions
- Confirm a clean worktree at the declared feature target.
- Read `tasks.md:52-53` and `design.md:37-42,61-67`.

## Allowed paths
- `internal/skillset/`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not wire admission or modify packet, CLI, ledger, or external skill files.

## Done criteria
- [ ] RED then GREEN tests cover union/deduplication/sorting, mandatory derived skills, budget constant, and required-skills section elision.
- [ ] Run `go test ./internal/skillset -race -count=1` and `go test ./... -race -count=1` successfully.
- [ ] Every exported function/constant is consumed by the planned terminal consumers; cite `file:line` and test output.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- [ ] Stop `blocked` if derived skills can be trimmed, paths enter the digest, or a caller requires stateful behavior.
- [ ] Stop `blocked` for out-of-scope edits, impossible criteria, unresolved design ambiguity, or conflicting instructions.

## Context
`design.md:39-41,63-66,81-85` fixes the pure API, canonical names, and digest semantics. `tasks.md:52-53,63-64` is the checklist.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
