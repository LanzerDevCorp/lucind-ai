---
name: lucind-apply
description: >-
  Trigger: apply lane, writer lane, code changes, bug fixes, implementation packet.
  Step-by-step TDD and implementation guide for writer lanes with strict scope enforcement, tool safety, and result envelope generation.
---

# Lucind Apply Lane Guide

## Exact Allowed Edit Surfaces

- **Authoritative Boundary**: The packet's `## Allowed edit surfaces` (or `allowed_paths`) is the strict, authoritative list. Refuse to write outside them; report deviations or interaction needed instead.
- **Allowed Writes**: Writes may include pre-existing untracked targets explicitly listed by the orchestrator and new files required by the task, but only inside the exact allowed edit surfaces.
- **Preserve Unrelated Files**: Preserve every unrelated tracked or untracked file. Never edit, move, delete, stage, or alter anything outside the allowed edit surfaces.

## Tool Safety

- **Sensitive Locations**: Never read sensitive files or locations, including secrets, credentials, tokens, private keys, personal data, `.env` files, credential stores, or unrelated user-home content.
- **Write Boundaries**: Never write outside the exact allowed edit surfaces, including through generated output, shell redirection, temporary copies, formatters, or scripts.
- **No Destructive Operations**: Never run destructive commands or deletion operations (`rm`, filesystem replacement, destructive migrations, or destructive git commands such as `git reset`, `git clean`, `git checkout`, `git restore`, `git rebase`).
- **No Git Lifecycle Mutations**: Never stage, commit, push, publish, release, or delegate. Do not run `git add`, `git commit`, `git push`, or package publish/release commands. The worker never commits; the dispatcher commits after green attestation and review.
- **No Unauthorized Modifications**: Do not run installers, dependency mutations, network-changing commands, migrations, or arbitrary repository scripts unless the orchestrator explicitly authorized the exact non-destructive command.
- **Safe Tool Substitutions**: Use the shell only for safe working-tree inspection and authorized verification commands. Prefer `bat`, `rg`, `fd`, `eza` instead of `cat`, `grep`, `find`, `ls`.

## Strict TDD Lifecycle

Consume the orchestrator's effective TDD mode, its source, and the exact runner; the mere existence of tests does not activate it. When strict TDD is active, follow the disciplined lifecycle:

1. **RED Phase**:
   - Write the smallest behavior-level unit or integration test reproducing the issue or asserting the new requirement.
   - Run the focused test runner specified in the packet (e.g. `go test ./<pkg>/... -count=1`).
   - Confirm the test fails for the EXPECTED behavioral reason (not a compilation error or panic). Capture observed failure evidence.
2. **GREEN Phase**:
   - Implement the minimal required logic to turn the test green.
   - Run the focused test runner and confirm it passes. Capture observed passing evidence.
3. **TRIANGULATE Phase**:
   - Exercise relevant negative, edge, or alternate cases that materially protect the contract.
4. **REFACTOR / SWEEP Phase**:
   - Improve code clarity and maintainability while focused tests remain green.
   - Run broad project validation (e.g. `go build ./...`, `go test ./... -race -count=1`, `go vet ./...`) only when authorized.

### Evidence Discipline and Exceptions
- **Truthful Evidence**: Never invent lifecycle evidence. If strict TDD was not activated, report `RED: not active — strict TDD was not activated` and `GREEN: not active — validation is reported separately`.
- **Documented Exceptions**: If strict TDD is active but the change cannot have a meaningful pre-implementation behavior test (e.g. passive documentation, skill documents, or no runner available), report a narrowly justified exception and still run every affected validation.
- **Mutation Testing for New Assertions**: When adding new assertions, temporarily alter production logic to confirm the test fails naming the specific condition, then restore production logic and verify green.

## Verification and Known Environmental Failures

When the packet carries a `## Verification` heading, it represents the delegated verification contract:

- **Foreground Execution**: Run every command listed under `## Verification` exactly as written, one at a time, in the foreground. Never launch verification commands in the background, and never end the task with a listed command unreported.
- **Evidence Reporting**: Report each command as `<exact command>: <observed result>` in the result.
- **Known Environmental Failures**: `## Known environmental failures` lists exact test names or command lines that already fail on the base before this task's changes. Report those named failures as evidence, not as a blocker.
- **Failure Boundaries**: Any unlisted verification command failure prevents reporting a successful completion and forces status `deviated` (partial completion).
- **Honest Reporting**: State failed or skipped checks plainly without hiding or rewriting around them.

## Escalating Ambiguity: Interaction Contract

If scope, ownership, allowed edit surfaces, acceptance criteria, or another human choice is ambiguous, stop immediately with `status: "interaction_required"`. Do not guess.

- **Non-Human Blockers vs. Human Decisions**: Use `status: "blocked"` only for a non-human technical blocker (such as a missing required tool, denied filesystem access, or an impossible repository invariant) or when a hard stop fires. Every decision requiring human or orchestrator guidance uses `interaction_required`.
- **Closed-Set Options**: Every interaction must be answerable from the payload alone. Present concrete choices in `options` as a closed set the human can approve, decline, or select from; never ask the human to author paths, globs, or commands as free text.
- **Derived Edit Surfaces**: When the missing input is the allowed edit surface, derive the candidate repository-relative paths the task would touch and present them in `options`.
- **Interaction Payload**: When `status` is `interaction_required`, populate the `interaction` object matching `internal/result/result.schema.json`:
  - `question`: Deterministic question for the orchestrator or human.
  - `reason`: Deterministic blocking reason explaining why execution cannot proceed.
  - `options`: Closed set of concrete choices (optional).
  - `unblock_response`: Exact context needed to continue.

## Mid-Cycle Timeout and Blocker Recovery (WIP-Rescue)

If a lane runs low on time, times out, or gets blocked mid-cycle:
1. **Preserve Worktree**: Do NOT delete or force-clean the worktree. The worktree is preserved on disk for inspection.
2. **Inspect State**: Inspect uncommitted changes with `git status` and `git diff`.
3. **No WIP Commits**: Workers never run `git commit`. Leave clean uncommitted changes in the worktree.
4. **Document Handoff**: Record partial work, remaining risks, and unblock requirements in `.lucind/result.json`.

## Result Envelope Mapping

Map worker execution states to the lucind-ai result envelope written to `.lucind/result.json` conforming to `internal/result/result.schema.json`:

- `completed` -> `status: "done"`: Every done-criterion met AND every hard stop declared not fired.
- `partial` -> `status: "deviated"`: Work completed but required departing from stated approach or touching out-of-scope; `deviations` array populated.
- `blocked` -> `status: "blocked"`: Non-human technical blocker or hard stop fired; `hard_stops[].fired=true` or `questions` array populated.
- `interaction_required` -> `status: "interaction_required"`: Human or orchestrator decision needed; `interaction` object populated with `question`, `reason`, `options`, `unblock_response`.

### Envelope Structure (`internal/result/result.schema.json`)
- `packet_id`: Echo of the packet ID from the task.
- `status`: One of `"done"`, `"blocked"`, `"deviated"`, `"failed"`, `"interaction_required"`.
- `summary`: Two or three sentences describing what was actually done.
- `hard_stops`: Array of `{ "hard_stop": string, "fired": boolean, "note"?: string }` for every hard stop in the packet.
- `files_changed`: Array of `{ "path": string, "change": "created"|"modified"|"deleted"|"copied", "why"?: string }`.
- `done_criteria`: Array of `{ "criterion": string, "met": boolean, "evidence": string }`.
- `interaction`: Required when `status == "interaction_required"` (`question`, `reason`, `options`, `unblock_response`).

## Pre-Completion Verification Checklist

- [ ] `git status --porcelain` shows only paths within `## Allowed edit surfaces`.
- [ ] Verification commands under `## Verification` pass cleanly (or match `## Known environmental failures`).
- [ ] No git operations (`git add`, `git commit`, `git push`) were executed.
- [ ] `.lucind/result.json` is written conforming to `internal/result/result.schema.json`.

## Key Learnings

Close the lane handoff with 1 to 5 numbered standalone factual sentences summarizing discoveries, pitfalls, or invariants learned during the lane.
