---
id: apply-skill-anchoring-guardrails
executor: agy
routed_by: single apply packet per tasks.md's own recommendation (lens B partition + synthesis re-check both confirm one packet, no apply-dag.yaml sidecar — cmd/lucind-ai/cli.go is touched by multiple concerns and internal callers must land atomically with the new worktree.Cleanup/Remove signature)
allowed_paths: ["internal/worktree/worktree.go", "internal/worktree/worktree_test.go", "internal/integrate/integrate.go", "internal/integrate/candidate.go", "internal/run/integrate.go", "cmd/lucind-ai/cli.go", "cmd/lucind-ai/cli_test.go", "internal/dag/split.go", "internal/dag/split_test.go", "plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md", "plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md", "plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md", ".agents/skills/lucind-apply/SKILL.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: eb02a8aab103194aa5d2aa9f3d572d5d9b2127bc
expected_parent_sha: eb02a8aab103194aa5d2aa9f3d572d5d9b2127bc
---

# Packet apply-skill-anchoring-guardrails

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/apply-skill-anchoring-guardrails · **Branch:** lucind/apply-skill-anchoring-guardrails

## Goal

Implement the full `skill-anchoring-guardrails` change per `openspec/changes/skill-anchoring-guardrails/tasks.md`: a fail-closed dirty-worktree guardrail on `worktree.Cleanup`/`Remove` (new `force bool` parameter, new `ErrWorktreeDirty` sentinel), a `--force`/`-f` flag on `lucind-ai worktree cleanup`, `force: true` passed by every automated internal teardown caller, four CLI guidance banners anchoring failure/milestone output to the existing skill reference docs, and a documented TDD WIP-rescue protocol. When finished, every task in `tasks.md` (1.1 through 3.7) is checked off with real evidence.

## Why this is safe to dispatch now

Proposal, design, and specs are accepted and frozen with zero unresolved contradictions (design: 58 citations verified; specs: 81 citations verified across 5 ADDED capabilities; tasks: fully traced, zero dropped citations). `tasks.md` and its own independent wave-viability re-check both confirm this is a single, atomically-dispatchable unit — do not split it.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/tasks.md`, `design.md`, and `specs/*/spec.md` exist and are accepted.
- Strict TDD Mode is active for this repository (project convention). Every production change in this packet MUST be preceded by a RED test that fails for the right reason before the GREEN implementation makes it pass. Do not write GREEN code first and backfill a test.
- Read `.agents/skills/lucind-apply/SKILL.md` in full before starting — it is this repository's actual RED → GREEN → SWEEP contract for apply lanes, including what to do if you run low on time mid-cycle. It governs *how* you execute; this packet governs *what* you build, in what order, and this packet's `allowed_paths` and done criteria are authoritative over any conflicting instruction in the skill about file scope.

## Task checklist (verbatim from `tasks.md` — treat every citation as a lead to re-verify against real code in this worktree, not as ground truth to copy blind)

### Phase 1: Core Worktree Guardrail & Automated Callers

- [ ] 1.1 RED: In `internal/worktree/worktree_test.go` add unit tests for a trailing `force bool` parameter on `Remove` and `Cleanup`: unforced dirty worktree (staged, unstaged, AND untracked cases separately) returns `ErrWorktreeDirty` and preserves files; forced dirty removal succeeds; clean removal succeeds; invalid path fails cleanly without panic; nonexistent lane cleanup returns nil idempotently. Run the new tests and confirm they fail for the right reason (compile error or assertion failure against current unconditional-delete behavior) before touching production code.
- [ ] 1.2 GREEN: In `internal/worktree/worktree.go` export `ErrWorktreeDirty = errors.New("worktree: linked worktree has uncommitted changes")` near the existing sentinel errors. Update `worktree.Cleanup` and `worktree.Remove` to accept a trailing `force bool`; when `force == false`, call the existing `PorcelainEmpty` helper and return `ErrWorktreeDirty` without deleting anything if it reports dirty. Prove: `go test ./internal/worktree -run 'TestRemove|TestCleanup|TestPorcelainEmpty' -v`.
- [ ] 1.3 GREEN: Update every internal automated call site of `worktree.Remove`/`worktree.Cleanup` to pass `force: true` (these are machine-managed scratch trees, not operator-facing) — the merge-conflict abort path in `internal/integrate/integrate.go`, the promotion teardown in `internal/integrate/candidate.go`, the lane teardown in `internal/run/integrate.go`, and `DiscardCombined`/`RemoveLaneWorktree` in `cmd/lucind-ai/cli.go`. Prove: `go test ./internal/integrate ./internal/run ./cmd/lucind-ai -count=1`.

### Phase 2: CLI Worktree Cleanup Command

- [ ] 2.1 RED: In `cmd/lucind-ai/cli_test.go`, extend `TestWorktreeCleanupCLI` (or the current equivalent): unforced dirty cleanup exits 1, prints porcelain status plus a diagnostic pointer to `troubleshooting.md`, and preserves files; dirty cleanup with `--force`/`-f` exits 0 and removes the worktree; clean cleanup exits 0; nonexistent lane cleanup exits 0 idempotently; missing `--lane` exits nonzero. Confirm these fail against current behavior first.
- [ ] 2.2 GREEN: In `cmd/lucind-ai/cli.go`, update `runWorktreeCleanup` (and its usage string) to parse `--force`/`-f`, call `worktree.Cleanup(ctx, primaryRoot, laneID, force)`, and on `errors.Is(err, worktree.ErrWorktreeDirty)` print the porcelain diff, exact `git status`/`git diff` inspection commands, a reference to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md`, and the exact `--force` re-run command, then exit 1. Prove: `go test ./cmd/lucind-ai -run TestWorktreeCleanupCLI -v`.

### Phase 3: CLI Failure Guidance Banners & Operator Documentation

- [ ] 3.1 RED: Add test cases in `cmd/lucind-ai/cli_test.go` and `internal/dag/split_test.go` asserting: (a) the non-done lane report function emits a banner (worktree path, diff-inspection steps, `troubleshooting.md` reference) only when lane status is not done; (b) the integrate-report function emits an `integrate retry --run <run-id>` banner referencing `recovery-reconciliation.md` only when `reverted_ids` is non-empty; (c) the acceptance-receipt renderer emits a reminder of qualitative review steps 2–10 referencing `acceptance-promotion.md`; (d) `runSplit`/`dag.Split` emits a multi-wave `base_sha`/`expected_parent_sha` refresh warning to **stderr** (never stdout — must not break scripts parsing wave commands) only when there is more than one wave. Confirm these fail first.
- [ ] 3.2 GREEN: Implement the non-done diagnostic banner in the lane-report printing function.
- [ ] 3.3 GREEN: Implement the reverted-IDs retry banner in the integrate-report printing function.
- [ ] 3.4 GREEN: Implement the qualitative-checklist-reminder banner in the acceptance-receipt renderer.
- [ ] 3.5 GREEN: Implement the multi-wave stderr warning in `runSplit`/`dag.Split`. Prove 3.1–3.5: `go test ./cmd/lucind-ai ./internal/dag -run 'TestPrint|TestAccept|TestSplit' -v`.
- [ ] 3.6 DOC: Update `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md` with the prescriptive TDD WIP-rescue protocol: inspect the preserved worktree (`git status`/`git diff`), judge whether RED tests or partial GREEN work has value, commit as WIP if so, re-dispatch with an adjusted timeout, and only request explicit human consent for `worktree cleanup --force` when work is genuinely non-recoverable. Also add/confirm the `integrate retry` (not redispatch) guidance for reverted batches and the multi-wave `base_sha` refresh reminder belong here or in `recovery-reconciliation.md` — follow that file's existing structure rather than duplicating content across both.
- [ ] 3.7 DOC: Update `.agents/skills/lucind-apply/SKILL.md` to reference the TDD WIP-rescue protocol (pointing at `troubleshooting.md`) for a lane that times out or gets blocked mid-cycle.

## Strict TDD discipline (MANDATORY)

For every RED/GREEN pair above: write the test first, run it, and confirm it fails for the reason the task states (not a compile error caused by an unrelated typo) before writing the corresponding production code. Do not batch all RED tests before any GREEN, and do not skip running a RED test before its GREEN — that ordering is what makes the commit history itself proof the tests are real.

## Out of scope

Everything the proposal's Out of Scope section names: daemon/cron worktree garbage collection, `.lucind/result.json` schema changes, automatic commits by the binary on timeout, feature lease/fencing changes, acceptance ledger storage/hashing changes, and orchestrator-adapter changes outside `plugin/claude-code/skills/lucind-ai/`. Do not touch any file not listed in `allowed_paths`.

## Allowed paths

Exactly the frontmatter `allowed_paths` list above. Touching anything else is a deviation — stop and report it.

## Allowed paths outside the repository

None. Write nothing outside this repository.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason `tasks.md` did not anticipate (e.g. a cited function no longer exists or has a materially different signature than described — re-verify against the real worktree first, then report the actual state).
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and neither `design.md` nor `tasks.md` says which.
- Satisfying one instruction in this packet would require violating another.
- A RED test cannot be made to fail for the intended reason before its GREEN counterpart.

## Done criteria

- [ ] **Every task 1.1 through 3.7 in `tasks.md` is complete**, with each RED test shown to have failed before its GREEN counterpart made it pass (describe how you confirmed this — a `go test` run against pre-GREEN code, or equivalent evidence).
- [ ] **`go test ./internal/worktree ./internal/integrate ./internal/run ./internal/dag ./cmd/lucind-ai -count=1` passes.**
- [ ] **`./lucind-checks.sh` passes on the final combined tree** (full-suite integration gate — run it yourself before considering this done; do not rely solely on the binary's own post-dispatch check).
- [ ] **Every introduced indirection names and proves a terminal consumer**: `ErrWorktreeDirty` is consumed by `errors.Is` checks in both `worktree.Remove` and `runWorktreeCleanup`; the `force bool` parameter is consumed by every listed call site (cite each with `file:line` in your envelope); the `--force`/`-f` flag is consumed by `runWorktreeCleanup`'s flag parsing; each of the four banners is consumed by its own printing function and demonstrably reachable (cite the calling code path).
- [ ] **Manual verification**: demonstrate (describe the exact commands and observed output) — (1) a worktree with an uncommitted modified file, `worktree cleanup` without `--force` fails with the file preserved; (2) the same with `--force` succeeds and removes it; (3) a `blocked`/non-done `lucind-ai run` output shows the troubleshooting banner; (4) a `split` on a 2+ wave DAG prints the multi-wave warning to stderr and NOT stdout.
- [ ] **The work is committed with a conventional commit and no AI attribution**, `git status --porcelain` empty, `git log --oneline -1` evidence. Check `git log -1 --format=%B` and strip any injected `Co-authored-by:` trailer.

## Context

Change: **skill-anchoring-guardrails**. Full grounding is in `openspec/changes/skill-anchoring-guardrails/{proposal.md,design.md,tasks.md,specs/}` in this worktree — read them; this packet is the dispatch contract, they are the technical source of truth. Delivery strategy: single-pr, review budget 2000 lines (forecast: ~250–450 lines, medium risk on the 400-line default but the human-confirmed budget is 2000, so no chaining is needed). Execution: Isolated Mode, `agy` executor (already decided). Strict TDD Mode is a standing project convention (see `CLAUDE.md`), not optional for this packet.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
