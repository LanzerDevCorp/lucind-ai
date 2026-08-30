---
id: verify-skill-provisioning-and-phase-specialist-cursor-agent
executor: cursor-agent
routed_by: qualitative verification of spec compliance, edge cases, and test quality
model: cursor-grok-4.6-high
read_only: true
---

# Packet verify-skill-provisioning-and-phase-specialist-cursor-agent

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/verify-skill-provisioning-and-phase-specialist-cursor-agent  ·  **Branch:** lucind/verify-skill-provisioning-and-phase-specialist-cursor-agent

## Goal

Perform qualitative verification of the candidate implementation for change `skill-provisioning-and-phase-specialist` against its specifications, edge cases, and test quality, evaluating the frozen mechanical check results.

## Why this is safe to dispatch now

This is the THIRD verify pass after two rounds of remediation. Round 1 (commit `07b359c`): BLOCKED, `cursor-agent` found 8 issues, `agy` falsely reported unconditional pass. Round 2 re-verify (commit `a266179`): both judges confirmed all 7 original findings fixed — but `cursor-agent` (again, not `agy`) surfaced 2 NEW residual findings while re-tracing production paths, both independently confirmed by direct code inspection. Round 2 remediation for those 2 findings has now landed at commit `d89604753adb3a5a52e8473b64b79685c29c141d`, and mechanical checks pass deterministically. Full history in `openspec/changes/skill-provisioning-and-phase-specialist/verify.md`.

**`agy`'s pattern across both prior rounds has been to report an unconditional pass and miss real, confirmable defects that `cursor-agent` catches.** Do not repeat this: do not just confirm each seam is *read* somewhere (e.g. "X reads Y") — trace whether the value is actually *produced* on the real, production-invoked code path (CLI entry point → dispatch → execution), not only in a unit test that constructs the intermediate value directly. Actually run `git show`/`git diff` on the specific commits below and read the actual current code, don't pattern-match against the summary text.

## Findings to specifically re-check (round 2's residual findings — the 7 round-1 findings are already confirmed fixed twice over, no need to re-trace those in depth)

1. **(round 2, CONFIRMED)** `internal/phasespec/phasespec.go`'s `CanonicalArtifactFilename("propose")` must now return `"proposal.md"` (not `"propose.md"`), matching this repo's real convention (this very change's own `proposal.md` file, `fan-out.md:12`, existing packet templates).
2. **(round 2, CONFIRMED)** The specialist's dynamically-generated synthesis packet body (`cmd/lucind-ai/cli.go`'s `packetContent := fmt.Sprintf(...)` construction) must now include a `## Required skills` section with resolved skill paths, not just env-var delivery.
3. **(round 2, flagged risk, not required to fix)** Lens eligibility depends on status-JSON keys (`lenses`/`lensStates`/`phaseLenses`) with no checked-in live `gentle-ai sdd-status` contract sample. Note whether this is still true; it does not block this verify pass per `verify.md`'s own disposition (design.md's Testing Strategy names mock `sdd-status` as the intended seam).

## Preconditions

- Mechanical checks have already executed deterministically and passed.
- Frozen mechanical check log is committed to the candidate branch and embedded in `## Context`.
- Worktree is created from the candidate branch `HEAD`.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** Verification citations trace to concrete symbols, spec requirements, or tests.
- [ ] **The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`).**
- [ ] **Qualitative evaluation completed** (`.lucind/result.json` populated with `status`, `summary`, and structured `findings`).

## Allowed paths

None. This is a read-only judgment lane. Do NOT create, modify, or delete any tracked or untracked files in the worktree, other than `.lucind/result.json`.

## Allowed paths outside the repository

None.

## Out of scope

Do NOT execute `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suite. Deterministic mechanical checks have already run once; their frozen output is in `## Context`. Re-running them wastes quota and adds no new signal. Do NOT modify any source files or commit any changes.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired. An undeclared hard stop invalidates the result.

- Executing mechanical test/build commands when mechanical results are already provided.
- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not anticipate.
- Two reasonable interpretations exist for a spec requirement and the specification does not say which.
- Satisfying one instruction in this packet would require violating another.

## Tool selection guidance

Perform your qualitative evaluation using read/navigation tools (`Read`, `Glob`, `Grep`, `codegraph`) and read-only git queries (`git diff`, `git log`, `git show`). Do NOT use shell execution for build or test runners.

## Evaluation areas

1. **Spec compliance**: Verify that the implementation satisfies all requirements and scenarios in each `specs/<spec-id>/spec.md` under `openspec/changes/skill-provisioning-and-phase-specialist/specs/` (acceptance-verifier, lane-execution, packet-authoring-contract, phase-specialist-dispatch, read-only-packet-schema, skill-derivation, skill-load-correspondence, skill-root-resolution).
2. **Edge cases**: Identify any missing edge-case handling, negative scenarios, boundary conditions, or concurrency concerns.
3. **Test quality**: Evaluate whether test cases assert on real terminal behavior rather than tautologies, mocks, or internal implementation details.

## Context

### Mechanical check summary
Command: `lucind-ai check` (runs `lucind-checks.sh`: `CGO_ENABLED=0 go build ./...` then `go test ./... -race -count=1`). Candidate git SHA: `d89604753adb3a5a52e8473b64b79685c29c141d`. Exit code: 0.

### Mechanical check transcript
See `openspec/changes/skill-provisioning-and-phase-specialist/verify-mechanical.log` (committed to this branch).

### Relevant specifications and design documents
- `openspec/changes/skill-provisioning-and-phase-specialist/design.md`
- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md`
- `openspec/changes/skill-provisioning-and-phase-specialist/specs/`
- `openspec/changes/skill-provisioning-and-phase-specialist/tasks.md`

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.

Omit the `commit` field (or leave it empty) per read-only envelope convention. Report all qualitative observations in `findings` with `finding`, `evidence` (`file:line` or command output), and `affects`.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
