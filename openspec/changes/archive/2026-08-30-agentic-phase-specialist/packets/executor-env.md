---
id: executor-env
executor: agy
routed_by: Strict-TDD executor plumbing for isolated required-skill environment delivery.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: 416fb50d867236471d57ff93c0ffd105e1bf2ff1
expected_parent_sha: 416fb50d867236471d57ff93c0ffd105e1bf2ff1
allowed_paths: ["internal/executor"]
read_only_paths: ["internal/result/result.go","internal/result/result.schema.json","internal/result/schema_test.go"]
---

# Packet executor-env

## Goal
Deliver required skills to child executors through isolated JSON `LUCIND_REQUIRED_SKILLS` environment state without leaking inherited values.

## Why this is safe to dispatch now
The result-envelope foundation is integrated, and this unit is isolated to executor request construction.

## Preconditions
- Verify `result-envelope` is integrated and the worktree is clean.
- Read `tasks.md:65` and `design.md:10,44,96`.

## Allowed paths
- `internal/executor/`

## Read-only inputs
- `internal/result/`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not alter packet parsing, body rendering, acceptance, CLI routing, or external environment files.

## Done criteria
- [ ] RED then GREEN subprocess-stub tests cover JSON injection, inherited-value stripping, and empty declarations.
- [ ] Run focused executor tests and `go test ./... -race -count=1` successfully.
- [ ] `Request.RequiredSkills` is consumed by `requestEnv` and the child process; cite code and test evidence.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- [ ] Stop `blocked` if inherited skill values are retained or write paths are injected through this channel.
- [ ] Stop `blocked` for out-of-scope edits, impossible criteria, unresolved alternatives, or contradictory instructions.

## Context
`design.md:44,96` and `tasks.md:65` require the environment channel beside `LUCIND_READ_ONLY_PATHS`, with inherited values stripped.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
