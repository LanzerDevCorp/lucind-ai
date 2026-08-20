# Completion Mode Enforcement Specification

## Purpose

After `decideStatus` maps an envelope to `lane.Done`, independently verify real git state — never the executor's self-reported envelope fields — and fail a lane whose actual state violates its declared write or read-only completion mode, before persisting the final status.

## Requirements

### Requirement: Post-Status Git Verification, Not Envelope Trust

`Execute` MUST invoke a post-`decideStatus` completion-mode check (`enforceCompletionMode` or equivalent) whenever the envelope mapped to `lane.Done`, and MUST run it before the final status is persisted via `deps.Ledger.SetStatus`. `decideStatus` itself MUST remain a pure mapping of timeout / non-zero exit / unreadable envelope / `envelope.LaneStatus()`, and MUST NOT itself inspect git. The completion-mode check MUST base its verdict on independent git facts (`HasUniqueLaneCommits`, `PorcelainEmpty`) and MUST NOT trust `Envelope.Commit` or `Envelope.FilesChanged` as evidence, since those are self-reported by the executor. (Design Decision 3.)

#### Scenario: Check runs only on Done, before persist
- GIVEN `decideStatus` returned `lane.Done`
- WHEN `Execute` is about to persist the lane's final status
- THEN the completion-mode check MUST run before `SetStatus`

#### Scenario: Non-Done statuses bypass the check entirely
- GIVEN `decideStatus` returned `lane.Blocked`, `lane.Deviated`, or `lane.Failed`
- WHEN `Execute` evaluates the lane
- THEN `Execute` MUST persist that status directly, without running the completion-mode check

#### Scenario: A self-reported commit hash is not evidence
- GIVEN a write packet whose envelope `commit` field is a non-empty string, but the worktree carries no unique lane commits
- WHEN the completion-mode check runs
- THEN the lane MUST become `lane.Failed` — the self-reported field MUST NOT override the git-verified result

### Requirement: Write Packet Completion Matrix

When `Packet.ReadOnly` is `false` and the envelope mapped to `Done`, the lane MUST remain `Done` only if the worktree has at least one unique lane commit **and** porcelain is empty. Any other combination MUST become `lane.Failed` with a descriptive ledger note. A write packet that previously reached `Done` without a real commit — which nothing prevented before this change — MUST now reach `Failed`; this is an intended, disclosed behavior change, not a regression. (Design Decision 3.)

#### Scenario: Compliant write lane
- GIVEN a write packet, `decideStatus` is `Done`, unique lane commits exist, and porcelain is empty
- WHEN the completion-mode check runs
- THEN the lane MUST stay `Done`

#### Scenario: Write lane without a commit fails (disclosed behavior change)
- GIVEN a write packet, `decideStatus` is `Done`, and the worktree has zero unique lane commits
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note naming the missing commit

#### Scenario: Write lane with a dirty tree fails
- GIVEN a write packet, `decideStatus` is `Done`, unique lane commits exist, and porcelain is not empty
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note

### Requirement: Read-Only Packet Completion Matrix

When `Packet.ReadOnly` is `true` and the envelope mapped to `Done`, the lane MUST remain `Done` only if the worktree has **zero** unique lane commits **and** porcelain is empty. Any other combination MUST become `lane.Failed` with a descriptive ledger note. (Design Decision 3.)

#### Scenario: Compliant read-only lane
- GIVEN a read-only packet, `decideStatus` is `Done`, no unique lane commits, and porcelain is empty
- WHEN the completion-mode check runs
- THEN the lane MUST stay `Done`

#### Scenario: Read-only lane that committed fails
- GIVEN a read-only packet, `decideStatus` is `Done`, and the worktree has one or more unique lane commits
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note

#### Scenario: Read-only lane with a dirty tree fails
- GIVEN a read-only packet, `decideStatus` is `Done`, no unique lane commits, and porcelain is not empty
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note

### Requirement: Git Inspection Failure Resolves to Failed, Not Blocked

When the git-inspection functions themselves cannot execute (a system-command error or an inaccessible worktree), the completion-mode check MUST resolve the lane to `lane.Failed`, not `lane.Blocked` — this is a binary-wiring problem, not a case needing a human decision. (Design Decision 3.)

#### Scenario: Git command failure fails the lane
- GIVEN `decideStatus` is `Done` and either `HasUniqueLaneCommits` or `PorcelainEmpty` returns an error
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed`, and the error MUST be recorded in the ledger note

### Requirement: Combine Stays Unaware of Read-Only Lanes

`combine` MUST continue to merge every lane branch with `git merge --no-ff`, with no read-only-specific filtering added. A read-only lane that passed the completion-mode check has, by construction, zero unique commits, so merging its branch is already a correct no-op. (Design Decision 3.)

#### Scenario: A passed read-only lane is still combined, harmlessly
- GIVEN a read-only lane that passed completion-mode enforcement
- WHEN `combine` runs
- THEN that lane's branch MUST still be included in the merge step, and `git merge --no-ff` against it MUST introduce no changes
