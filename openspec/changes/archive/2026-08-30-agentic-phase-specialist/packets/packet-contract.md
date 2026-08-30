---
id: packet-contract
executor: agy
routed_by: Strict-TDD contract plumbing after the skill derivation foundation is green.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: 416fb50d867236471d57ff93c0ffd105e1bf2ff1
expected_parent_sha: 416fb50d867236471d57ff93c0ffd105e1bf2ff1
allowed_paths: ["internal/packet","internal/packetauthor"]
read_only_paths: ["internal/skillset"]
---

# Packet packet-contract

## Goal
Extend packet and authoring contracts with lane role, ad-hoc skills, and derived required skills; render required skill paths and hash canonical body content.

## Why this is safe to dispatch now
The skillset foundation is a declared dependency and this unit owns the packet/compiler correspondence required by the design.

## Preconditions
- Verify `skillset-engine` is integrated and its tests are green.
- Read `tasks.md:57-64` and `design.md:16-21,49-66`.

## Allowed paths
- `internal/packet/`
- `internal/packetauthor/`

## Read-only inputs
- `internal/skillset/`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not modify executor, run/accept enforcement, ledger, admission, or CLI files.

## Done criteria
- [ ] RED then GREEN tests cover closed lane roles/phases, legacy omission, ad-hoc parsing, compile rendering, and digest stability across resolved roots.
- [ ] Run focused packet/packetauthor tests and `go test ./... -race -count=1` successfully.
- [ ] `Compile` consumes `skillset.Derive` and its rendered contract is consumed by executor/runtime; cite terminal consumers.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- [ ] Stop `blocked` if `required_skills` becomes an authored frontmatter field, paths enter canonical digests, or ledger fields must change.
- [ ] Stop `blocked` for out-of-scope edits, impossible criteria, unresolved alternatives, or conflicting instructions.

## Context
`design.md:18-20,39-41,61-66` fixes both authoring surfaces and digest behavior. `tasks.md:57-64` defines the coupled edits.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
