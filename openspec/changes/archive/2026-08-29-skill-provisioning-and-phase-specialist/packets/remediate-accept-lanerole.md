---
id: remediate-accept-lanerole
executor: agy
routed_by: fix confirmed verify.md finding 6 — Design Decision 8 lockstep gap
model: gemini-3.7-flash-high
---

# Packet remediate-accept-lanerole

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-accept-lanerole  ·  **Branch:** lucind/remediate-accept-lanerole

## Goal

Fix verify.md finding 6: Design Decision 8 required the `accept.go` decode struct to gain `LaneRole` in lockstep with `packetDigest` (which already hashes `LaneRole`, per `internal/run/run.go:722-729`); only `RequiredSkills` was added to the decode struct. Add `LaneRole` to the decode struct and cross-check it in `validateVersionedEvidence`, matching how `RequiredSkills` is already handled there.

## Preconditions

- Read `internal/accept/accept.go:263-328` (the decode struct and `validateVersionedEvidence`).
- Read `internal/run/run.go:722-734` (`packetDigest`, to see exactly how `LaneRole` is folded into the digest — the accept-side check must correspond to the same semantics).
- Read `openspec/changes/skill-provisioning-and-phase-specialist/design.md` (Decision 8, describing the field-list lockstep requirement).
- Read `internal/accept/authoring_evidence_test.go:56-127` for the existing `RequiredSkills` mutation-case test pattern to follow for `LaneRole`.

## Allowed paths

- `internal/accept/`

## Read-only inputs

- `internal/run/`
- `internal/packet/`
- `internal/ledger/authoring.go` (read only — do not edit; no `AuthoringEvidence` shape or version change is in scope)

## Out of scope

Do not edit `internal/ledger/authoring.go` or change `AuthoringEvidenceVersion`. Do not touch `internal/run`, `internal/phasespec`, `cmd/lucind-ai`, or any plugin/asset files.

## Done criteria

- [ ] `accept.go`'s decode struct gains a `LaneRole` field alongside the existing `RequiredSkills` field.
- [ ] `validateVersionedEvidence` cross-checks `LaneRole` correspondence the same way it already does for `RequiredSkills` (fails closed on mismatch, matching existing precedent).
- [ ] A new mutation-case test in `authoring_evidence_test.go` (or the appropriate existing test file) proves a `LaneRole` mismatch is rejected, mirroring the existing `RequiredSkills` mismatch test.
- [ ] `go test ./internal/accept/...` passes; `go build ./...` and `go test ./... -race -count=1` remain green.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops

- Stop `blocked` if this requires any `internal/ledger/authoring.go` edit or `AuthoringEvidenceVersion` bump — that is explicitly out of scope per the original proposal.
- Stop `blocked` if `packetDigest`'s exact `LaneRole` encoding is ambiguous from the code alone.

## Return

Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
