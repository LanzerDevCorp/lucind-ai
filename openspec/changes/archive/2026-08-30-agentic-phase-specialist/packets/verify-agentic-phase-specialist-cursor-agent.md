---
id: verify-agentic-phase-specialist-cursor-agent
executor: cursor-agent
routed_by: qualitative verification of spec compliance, edge cases, and test quality
model: cursor-grok-4.6-high
read_only: true
---

# Packet verify-agentic-phase-specialist-cursor-agent

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/verify-agentic-phase-specialist-cursor-agent  ·  **Branch:** lucind/verify-agentic-phase-specialist-cursor-agent

## Goal

Perform qualitative verification of the candidate implementation for change `agentic-phase-specialist` against its specifications, `design.md`'s decisions, and `tasks.md`'s done criteria, evaluating the frozen mechanical check results.

## Why this is safe to dispatch now

The candidate implementation is complete and merged to `dev` at `19d5f01c0ae5c65b12cede90d47804ec578568b7` (three commits: `6723b77`, `2080528`, `8dc595b`). Mechanical checks (`lucind-checks.sh`) have already run once on the current `HEAD` (`c271f2e9d4382121abc1f38091e551c00c04a348`, which only adds the frozen mechanical log on top of the merge) and passed deterministically. This judgment lane is read-only and does not mutate repository state or race with other lanes.

Do not just confirm each symbol is *present* or *read* somewhere — trace whether the gating actually fires on the real, production-invoked code path (`Verifier.Verify` in `internal/accept/accept.go`, `ExecuteAttempt`'s CHECKING phase in `internal/run/attempt.go`), not only inside a unit test that constructs the intermediate value directly. Actually run `git show`/`git diff` on the commits below and read the current merged code; do not pattern-match against tasks.md's claimed-done checkboxes alone.

## Specific things to verify

1. **`internal/accept/accept.go`**: `GetLaneMetadata` (`:84`) is loaded unconditionally, outside the `AuthoringEvidenceVersion` branch. `runSDDPhaseChecks` (`:120`) gates `integrate.CheckPolicySnapshot` and `v.check` on `metadata.SDDPhase == "" || metadata.SDDPhase == "apply"`. Schema, hard stops, done criteria, and `allowed_paths` (`validateResultAndScope`, `:97-99`) remain unconditional regardless of the gate. `integrate.Check` itself (`internal/integrate/integrate.go:159`) stays ungated.
2. **`internal/run/attempt.go`**: `shouldRunAttemptChecks` (`:384-396`) resolves `SDDPhase` per combined lane via `deps.Ledger.GetLaneMetadata`, fails closed (returns `true`, i.e. runs checks) on a nil ledger, empty branches, a lookup error, an empty `SDDPhase`, or an `"apply"` phase, and only returns `false` when every combined lane is a declared non-apply phase. Lease renewal (`:447-449`) and `integrate.Check` stay ungated by this logic; only `checkFunc`'s invocation in CHECKING (`:469-477`) is gated.
3. **Both skill trees** (`plugin/claude-code/skills/lucind-ai/` and `plugin/opencode/skills/lucind-ai/`) carry byte-identical edits: the Hard Rule carve-out in `SKILL.md:19` matches `design.md:39-47` verbatim; `references/strategies/fan-out.md:47` moves synthesis-note review and contradiction arbitration to the Specialist; `references/contracts/acceptance-promotion.md` adds the `sdd_phase`-conditional caveat to checklist steps 1 and 8 and upgrades subagent delegation language to decision-bearing Specialist Acceptance while keeping Dual-Judge (`:38-43`) unchanged.
4. **Test quality**: `internal/accept/accept_test.go`'s `TestVerifierSkipsChecksForDeclaredNonApplyPhase`, `TestVerifierRunsChecksForApplyEmptyOrMissingSDDPhase`, `TestVerifierNonApplyPhaseStillEnforcesScope`, and `internal/run/attempt_test.go`'s `TestExecuteAttemptSkipsChecksForDeclaredNonApplyLanes`, `TestExecuteAttemptRunsChecksForApplyEmptyOrMissingCombinedLane` assert on real terminal behavior (a failing `lucind-checks.sh` script actually causes/does not cause rejection; `checkFunc` call counts) rather than mocking away the gate itself.
5. **Task 4.1 is intentionally unchecked** in `tasks.md` (out-of-repo `~/.claude/skills/sdd-*/SKILL.md` paste, human/Orchestrator follow-up per `design.md:102-106`, `fan-out.md:43`). Confirm this is correctly scoped as out-of-Lane-authority rather than a missed requirement.

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

1. **Spec compliance**: Verify that the implementation satisfies all requirements and scenarios in each `specs/<spec-id>/spec.md` under `openspec/changes/agentic-phase-specialist/specs/` (`acceptance-verifier`, `phase-specialist-dispatch`, `phase-verdict-reporting`, `sdd-planning-fan-out`).
2. **Edge cases**: Identify any missing edge-case handling, negative scenarios, boundary conditions, or concurrency concerns (e.g. mixed combined-lane phases, ledger lookup failure, malformed `SDDPhase`).
3. **Test quality**: Evaluate whether test cases assert on real terminal behavior rather than tautologies, mocks, or internal implementation details.

## Context

### Mechanical check summary
Command: `lucind-ai check` (runs `lucind-checks.sh`: `CGO_ENABLED=0 go build ./...` then `go test ./... -race -count=1`). Candidate git SHA: `19d5f01c0ae5c65b12cede90d47804ec578568b7`. Exit code: 0. Duration: 2m36.385271238s.

### Mechanical check transcript
See `openspec/changes/agentic-phase-specialist/verify-mechanical.log` (committed to this branch at `c271f2e9d4382121abc1f38091e551c00c04a348`).

### Relevant specifications and design documents
- `openspec/changes/agentic-phase-specialist/design.md`
- `openspec/changes/agentic-phase-specialist/proposal.md`
- `openspec/changes/agentic-phase-specialist/specs/`
- `openspec/changes/agentic-phase-specialist/tasks.md`

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.

Omit the `commit` field (or leave it empty) per read-only envelope convention. Report all qualitative observations in `findings` with `finding`, `evidence` (`file:line` or command output), and `affects`.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
