# Delta for acceptance-verifier

## ADDED Requirements

### Requirement: Frozen Evidence Acceptance Verification

Acceptance verification MUST evaluate immutable candidate commits, tree hashes, schema-compliant result envelopes, and clean worktree status, demoting any lane that violated hard stops or undeclared path boundaries regardless of claimed criteria.

#### Scenario: Valid frozen evidence satisfies acceptance verification

- GIVEN a clean working tree, valid commit, and schema-compliant result envelope
- WHEN acceptance verification evaluates the lane
- THEN the lane status MUST be recorded as done in the ledger

#### Scenario: Read-only packet with clean working tree and no commits passes verification

- GIVEN a packet declared read-only with no unique commits and an empty diff
- WHEN completion mode verification runs
- THEN verification MUST pass and mark the lane done

#### Scenario: Violated hard stop or unapproved scope deviation demotes verdict

- GIVEN a result envelope where a declared hard stop fired or undeclared paths were touched
- WHEN acceptance verification executes
- THEN the lane MUST be demoted to blocked or deviated regardless of green criteria claims
