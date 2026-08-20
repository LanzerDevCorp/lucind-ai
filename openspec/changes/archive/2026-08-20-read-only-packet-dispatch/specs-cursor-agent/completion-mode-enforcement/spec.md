# Completion Mode Enforcement Specification

## Purpose

After `decideStatus` returns `Done`, inspect real git state and fail lanes that violate their write or read-only completion mode.

## Requirements

### Requirement: Git Inspection After DecideStatus

`Execute` MUST invoke a post-`decideStatus` check (`enforceCompletionMode` or equivalent) when the envelope mapped to `lane.Done`, and MUST do so before the final status is persisted with `SetStatus`. `decideStatus` itself MUST stay a pure mapping of timeout / non-zero exit / unreadable envelope / `envelope.LaneStatus()` and MUST NOT inspect git. The check MUST use independent git facts via `HasUniqueLaneCommits` and `PorcelainEmpty`, and MUST NOT trust `Envelope.Commit` or `Envelope.FilesChanged`. (Design Decision 3.)

#### Scenario: Check runs only on Done, before persist
- GIVEN `decideStatus` returned `lane.Done`
- WHEN Execute is about to persist
- THEN the completion-mode check MUST run before `SetStatus`

#### Scenario: Envelope commit is ignored
- GIVEN a write packet whose envelope `commit` is a non-empty string but the worktree has no unique lane commits
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed`

### Requirement: Write Packet Done Matrix

When `Packet.ReadOnly` is `false` and the envelope mapped to `Done`, the lane MUST remain `Done` only if it has unique lane commits AND porcelain is empty. Otherwise the lane MUST become `Failed` with a ledger note. A write packet that previously reached `Done` without a commit MUST now reach `Failed`; this is an intended, disclosed behavior change. (Design Decision 3.)

#### Scenario: Write compliant
- GIVEN a write packet, `decideStatus` is `Done`, unique lane commits exist, and porcelain is empty
- WHEN the completion-mode check runs
- THEN the lane MUST stay `Done`

#### Scenario: Write violating — no commit
- GIVEN a write packet, `decideStatus` is `Done`, and the worktree has no unique lane commits
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note

#### Scenario: Write violating — dirty tree
- GIVEN a write packet, `decideStatus` is `Done`, unique lane commits exist, and porcelain is not empty
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note

### Requirement: Read-Only Packet Done Matrix

When `Packet.ReadOnly` is `true` and the envelope mapped to `Done`, the lane MUST remain `Done` only if it has no unique lane commits AND porcelain is empty. Otherwise the lane MUST become `Failed` with a ledger note. (Design Decision 3.)

#### Scenario: Read-only compliant
- GIVEN a read-only packet, `decideStatus` is `Done`, no unique lane commits, and porcelain empty
- WHEN the completion-mode check runs
- THEN the lane MUST stay `Done`

#### Scenario: Read-only violating — committed
- GIVEN a read-only packet, `decideStatus` is `Done`, and unique lane commits exist
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note

#### Scenario: Read-only violating — dirty tree
- GIVEN a read-only packet, `decideStatus` is `Done`, no unique lane commits, and porcelain is not empty
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed` with a ledger note

### Requirement: Non-Done Statuses Unchanged

The completion-mode check MUST NOT override `blocked`, `deviated`, or `failed`. A git-inspection error MUST resolve to `Failed`, not `blocked`. (Design Decision 3.)

#### Scenario: Blocked is not rewritten
- GIVEN a read-only packet whose envelope mapped to `blocked` and whose worktree is dirty
- WHEN Execute persists
- THEN the lane MUST stay `blocked`

#### Scenario: Git cannot run
- GIVEN `decideStatus` is `Done` and `HasUniqueLaneCommits` or `PorcelainEmpty` returns an error
- WHEN the completion-mode check runs
- THEN the lane MUST become `Failed`, not `blocked`

### Requirement: Combine Stays Unaware

`combine` MUST keep merging every lane branch with `git merge --no-ff` and MUST NOT filter read-only lanes. A read-only lane that passed completion-mode enforcement has zero unique commits, so that merge is a no-op. (Design Decision 3.)

#### Scenario: Read-only lane still combined
- GIVEN a read-only lane that passed completion-mode enforcement
- WHEN `combine` runs
- THEN that lane's branch MUST still be included and `git merge --no-ff` MUST be a no-op against it
