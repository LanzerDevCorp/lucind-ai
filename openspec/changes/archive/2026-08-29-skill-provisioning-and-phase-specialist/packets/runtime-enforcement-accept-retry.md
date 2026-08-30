---
id: runtime-enforcement-accept-retry
executor: agy
routed_by: Retry of runtime-enforcement-accept after the result schema was refreshed by the current installed binary.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: aa59d8e2bf8d95687df33c9e939bd2b55b84166c
expected_parent_sha: aa59d8e2bf8d95687df33c9e939bd2b55b84166c
allowed_paths: ["internal/run","internal/accept"]
read_only_paths: ["internal/result/result.go","internal/result/result.schema.json","internal/result/schema_test.go","internal/skillset","internal/packet","internal/packetauthor"]
---

# Packet runtime-enforcement-accept-retry

## Goal
Enforce required-skill declarations during run and acceptance, keeping `packetDigest` and the accept decode struct in lockstep.

## Preconditions
- Verify dependencies are integrated and the worktree is clean.
- Read `tasks.md:66-69` and `design.md:61-66,84-87`.

## Allowed paths
- `internal/run/`
- `internal/accept/`

## Read-only inputs
- `internal/result/`
- `internal/skillset/`
- `internal/packet/`
- `internal/packetauthor/`

## Out of scope
Do not modify ledger schema/authoring shape, CLI acceptance behavior, or packet compiler fields.

## Done criteria
- RED then GREEN tests prove digest stability, missing loaded skills demote run status, matching skills allow dirty-primary acceptance, and mismatch rejects without receipt.
- `packetDigest` and accept decode enumerate the same fields; run focused run/accept tests and `go test ./... -race -count=1`.
- Every enforcement path is consumed by run/accept terminal decisions; cite `file:line` and outputs.
- Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- Stop `blocked` if accept demotes instead of rejects, frozen evidence/version or ledger schema changes, or correspondence cannot be kept lockstep.
- Stop `blocked` for out-of-scope edits, impossible criteria, unresolved alternatives, or conflicting instructions.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
