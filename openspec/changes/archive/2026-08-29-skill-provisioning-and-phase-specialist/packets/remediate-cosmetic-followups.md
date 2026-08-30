---
id: remediate-cosmetic-followups
executor: agy
routed_by: fix 3 trivial non-blocking follow-ups flagged by third-pass verify
model: gemini-3.7-flash-high
---

# Packet remediate-cosmetic-followups

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-cosmetic-followups  ·  **Branch:** lucind/remediate-cosmetic-followups

## Goal

Fix `openspec/changes/skill-provisioning-and-phase-specialist/tasks.md` items 7.1, 7.2, 7.3 — three small, explicitly non-blocking cosmetic items flagged by the change's third and final verify pass (already PASSED; these were noted as nice-to-haves, not defects):

1. **7.1** — `TestPhaseSubcommandGatesPrematureSynthesis` (`cmd/lucind-ai/cli_test.go:5653-5657`) still negatively asserts that the OLD filename `propose.md` was not created. Since the canonical filename was renamed to `proposal.md` (in a prior remediation, commit `c226b6d`), this assertion should check for `proposal.md` instead, so it actually catches a regression if the specialist accidentally writes the real canonical file during the premature-synthesis gate test.
2. **7.2** — `openspec/changes/skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md`'s requirement prose (not its scenario, which already says `proposal.md`) still generically says artifacts land at `openspec/changes/<change>/<phase>.md`. Tighten this one sentence to name the actual per-phase convention implemented: `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `apply.md`, `verify.md`, `remediate.md`, `archive.md` (see `internal/phasespec/phasespec.go`'s `CanonicalArtifactFilename` for the authoritative list).
3. **7.3** — Add a one-line code comment (not a behavior change) at `cmd/lucind-ai/cli.go`'s existing-packet-reuse branch (around line 2447-2458, the `if _, err := os.Stat(cand1); err == nil { ... }`-style check) noting that a synthesis packet already on disk from before the `## Required skills` section was added will be reused as-is without that section — this is a known, accepted cosmetic gap on stale local caches (env-var delivery still applies), not something to fix behaviorally in this packet.

## Preconditions

- Read `cmd/lucind-ai/cli_test.go` around lines 5653-5657 for the exact current negative assertion in `TestPhaseSubcommandGatesPrematureSynthesis`.
- Read `openspec/changes/skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md` in full to find the exact requirement-prose sentence using the generic `<phase>.md` phrasing (distinct from the scenario text, already correct).
- Read `cmd/lucind-ai/cli.go` around lines 2440-2520 to find the existing-packet-reuse branch for the comment in 7.3.

## Allowed paths

- `cmd/lucind-ai/cli_test.go` (only the one assertion named in 7.1)
- `openspec/changes/skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md` (only the one requirement-prose sentence named in 7.2)
- `cmd/lucind-ai/cli.go` (only a comment addition, no behavior change, for 7.3)

## Read-only inputs

- `internal/phasespec/`

## Out of scope

Do not change any behavior — 7.3 is a comment-only addition. Do not touch any other test, any other file, or re-litigate any already-passed verify finding. This is pure documentation/test-assertion hygiene on an already-PASSED change.

## Done criteria

- [ ] `TestPhaseSubcommandGatesPrematureSynthesis` now asserts `proposal.md` (not `propose.md`) was not created.
- [ ] `phase-specialist-dispatch/spec.md`'s requirement prose names the actual per-phase filename convention instead of the generic `<phase>.md` placeholder.
- [ ] A one-line comment exists at the existing-packet-reuse branch in `cli.go` noting the stale-packet caveat from 7.3's description above. No functional change.
- [ ] `go build ./...` and `go test ./... -race -count=1` remain green.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops

- Stop `blocked` if any of these three items would require a behavior change beyond test assertions, spec prose, or a comment — that would mean the item was miscategorized as cosmetic and needs re-scoping, not guessed at.

## Return

Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
