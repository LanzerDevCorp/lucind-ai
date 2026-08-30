---
id: cli-packet-authoring
executor: agy
routed_by: Terminal CLI wiring after specialist, contract, roots, and configuration subsystems are green.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: ed28806c7745bccbac01de86296ac79522b68ebe
expected_parent_sha: ed28806c7745bccbac01de86296ac79522b68ebe
allowed_paths: ["cmd/lucind-ai"]
read_only_paths: ["internal/packet","internal/packetauthor","internal/skillset","internal/skillroots","internal/lucindconfig","internal/phasespec"]
---

# Packet cli-packet-authoring

## Goal
Wire `lucind-ai phase <name>` and fail-closed skill derivation/root resolution/budget admission through the existing packet authoring path.

## Why this is safe to dispatch now
All required subsystems are declared dependencies, making this the single terminal CLI unit; it owns only `cmd/lucind-ai`.

## Preconditions
- Verify packet-contract, skillroots, lucindconfig, phasespec, and runtime-enforcement-accept are integrated and green.
- Read `tasks.md:73-76` and `design.md:22-36,81-87,107`.

## Allowed paths
- `cmd/lucind-ai/`

## Read-only inputs
- `internal/packet/`
- `internal/packetauthor/`
- `internal/skillset/`
- `internal/skillroots/`
- `internal/lucindconfig/`
- `internal/phasespec/`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not dispatch any lane, modify non-CLI source, change feature target identity, add failure banners, or alter acceptance demotion semantics.

## Done criteria
- [ ] RED then GREEN tests prove whole-batch pre-worktree rejection for missing skills/over-budget sets and successful valid admission.
- [ ] Tests prove `lucind-ai phase <name>` invokes the specialist path and preserve existing command behavior; run focused tests and `go test ./... -race -count=1`.
- [ ] Admission consumes config, derivation, and root resolution before allocation; phase output is consumed by the existing packet workflow. Cite terminal consumers and outputs.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- [ ] Stop `blocked` if admission allocates before validation, silently trims skills, changes target identity, or requires dispatch from this lane.
- [ ] Stop `blocked` for out-of-scope edits, impossible criteria, unresolved alternatives, or conflicting instructions.

## Context
`design.md:24-27,30-35,81-87` and `tasks.md:73-76,95-99` define fail-closed admission and CLI wiring. The feature target is immutable.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
