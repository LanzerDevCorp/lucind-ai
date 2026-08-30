---
id: result-envelope-retry
executor: agy
routed_by: Strict-TDD foundation work for the optional skills-loaded result contract and schema reflection pin.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: 0cf9c6ff95b1788a94f264f57df45425af23c4c4
expected_parent_sha: 0cf9c6ff95b1788a94f264f57df45425af23c4c4
allowed_paths: ["internal/result/result.go","internal/result/result.schema.json","internal/result/schema_test.go"]
---

# Packet result-envelope-retry

## Goal
Add optional `skills_loaded` support to the result envelope and JSON schema, with a reflection test pin and no schema-version or ledger migration.

## Why this is safe to dispatch now
The design fixes this as an additive foundation unit and keeps frozen evidence unchanged. It is isolated to `internal/result/` and has no runtime dependency.

## Preconditions
- Start at the declared feature target and confirm the worktree is clean.
- Read `tasks.md` and `design.md` citations before editing.

## Allowed paths
- `internal/result/result.go`
- `internal/result/result.schema.json`
- `internal/result/schema_test.go`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not modify ledger schemas, packet parsing, runtime enforcement, or any path outside the whitelist.

## Done criteria
- [ ] Strict TDD: add failing schema/reflection assertions, implement the minimal additive field, then run `go test ./internal/result -race -count=1`.
- [ ] `skills_loaded` is optional, a string array, and reflected by `Envelope.SkillsLoaded`; existing schema tests and `go test ./... -race -count=1` pass.
- [ ] Every indirection introduced is demonstrably consumed by `result.Read`/runtime envelope consumers; cite `file:line` and command output.
- [ ] Commit conventionally with no AI attribution; `git status --porcelain` is empty and `git log --oneline -1` is recorded.

## Hard stops
- [ ] Stop `blocked` if a schema version or ledger migration is required.
- [ ] Stop `blocked` if a change outside allowed paths is needed or any criterion is impossible.
- [ ] Stop `blocked` if two reasonable implementations are not resolved by `design.md`.

## Context
`design.md:9-12,61-66,97-99` requires an optional field and reflection pin. `tasks.md:26,59` defines this unit. The schema uses `additionalProperties: false`.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
