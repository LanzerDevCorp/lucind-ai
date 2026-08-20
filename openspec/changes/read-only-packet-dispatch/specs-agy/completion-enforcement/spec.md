# Completion Enforcement Specification

## Purpose

Enforce packet completion invariants in `run.Execute` through independent git inspection after `decideStatus` maps an envelope to `done`, verifying commit presence and tree cleanliness for write packets, and forbidding commits and working-tree changes for read-only packets before persisting final status.

## Requirements

### Requirement: Post-Status Git Verification Placement (Decision 3)

The execution runtime MUST invoke completion mode enforcement immediately after `decideStatus` returns `lane.Done`, before invoking `deps.Ledger.SetStatus`, and MUST NOT enforce git invariants on non-`Done` statuses.

#### Scenario: Enforcement before status persistence
- GIVEN a lane execution where `decideStatus` returns `lane.Done`
- WHEN `Execute` evaluates the lane
- THEN `Execute` MUST run completion mode enforcement on the worktree before persisting status to the ledger.

#### Scenario: Non-Done status bypasses enforcement
- GIVEN a lane execution where `decideStatus` returns `lane.Blocked`, `lane.Deviated`, or `lane.Failed`
- WHEN `Execute` evaluates the lane
- THEN `Execute` MUST bypass completion mode enforcement and persist the computed status directly.

### Requirement: Read-Only Completion Enforcement (Decision 3)

For a read-only packet (`Packet.ReadOnly == true`), completion mode enforcement MUST inspect actual worktree git state and pass `Done` ONLY when the worktree has no unique commits relative to primary HEAD and git porcelain status is clean.

#### Scenario: Compliant read-only lane succeeds
- GIVEN a read-only packet whose envelope maps to `lane.Done`
- WHEN git inspection confirms zero unique commits (`HasUniqueLaneCommits == false`) and clean working tree (`PorcelainEmpty == true`)
- THEN enforcement MUST pass and `Execute` MUST persist `lane.Done`.

#### Scenario: Read-only lane with commits fails
- GIVEN a read-only packet whose envelope maps to `lane.Done`
- WHEN git inspection detects one or more unique commits on the lane branch (`HasUniqueLaneCommits == true`)
- THEN enforcement MUST override status to `lane.Failed` and record a descriptive ledger note.

#### Scenario: Read-only lane with dirty tree fails
- GIVEN a read-only packet whose envelope maps to `lane.Done`
- WHEN git inspection detects uncommitted working-tree changes (`PorcelainEmpty == false`)
- THEN enforcement MUST override status to `lane.Failed` and record a descriptive ledger note.

### Requirement: Write Packet Completion Enforcement (Decision 3)

For a write packet (`Packet.ReadOnly == false`), completion mode enforcement MUST inspect actual worktree git state and pass `Done` ONLY when the worktree has one or more unique commits and git porcelain status is clean. Failing a write packet reaching `done` without unique commits is a mandatory disclosed behavior change.

#### Scenario: Compliant write lane succeeds
- GIVEN a write packet whose envelope maps to `lane.Done`
- WHEN git inspection confirms unique commits exist (`HasUniqueLaneCommits == true`) and working tree is clean (`PorcelainEmpty == true`)
- THEN enforcement MUST pass and `Execute` MUST persist `lane.Done`.

#### Scenario: Write lane without commits fails (disclosed behavior change)
- GIVEN a write packet whose envelope maps to `lane.Done`
- WHEN git inspection detects zero unique commits on the lane branch (`HasUniqueLaneCommits == false`)
- THEN enforcement MUST override status to `lane.Failed` and record a descriptive ledger note indicating missing commits.

#### Scenario: Write lane with dirty tree fails
- GIVEN a write packet whose envelope maps to `lane.Done`
- WHEN git inspection detects uncommitted changes (`PorcelainEmpty == false`)
- THEN enforcement MUST override status to `lane.Failed` and record a descriptive ledger note.

### Requirement: Git Inspection Error Resolution (Decision 3)

When git inspection fails to execute (e.g. system command error or inaccessible worktree), enforcement MUST resolve the lane to `lane.Failed` rather than `lane.Blocked`.

#### Scenario: Git execution error fails lane
- GIVEN a lane whose envelope maps to `lane.Done`
- WHEN `HasUniqueLaneCommits` or `PorcelainEmpty` returns an error
- THEN enforcement MUST resolve status to `lane.Failed` and record the error in the ledger note.

### Requirement: Read-Only Lane Integration No-Op (Decision 3)

The integration stage (`combine`) MUST perform standard merge processing without requiring custom lane filtering, as a compliant read-only lane branch carries zero unique commits.

#### Scenario: Read-only lane merges as clean no-op
- GIVEN a completed batch containing a compliant read-only lane
- WHEN `combine` executes `git merge --no-ff` on each lane branch
- THEN merging the read-only lane branch with zero unique commits MUST execute successfully without introducing changes.
