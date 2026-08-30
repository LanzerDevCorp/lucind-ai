# Acceptance Verifier Specification

## Purpose

Define fail-closed mechanical acceptance for a frozen lane candidate.

## Requirements

### Requirement: Exact Acceptance Binding

Every decision and receipt MUST immutably bind the lane, packet, base commit and tree, candidate commit and tree, allowed paths, check policy, and relevant environment identity.

#### Scenario: Record the complete binding

- GIVEN a candidate with every required identity value
- WHEN mechanical acceptance succeeds
- THEN the receipt contains the exact complete binding
- AND none of its bound values can be changed

#### Scenario: Reject an identity mismatch

- GIVEN the packet, commit, tree, policy, or environment differs from the requested binding
- WHEN acceptance is attempted
- THEN acceptance fails and no receipt exists

### Requirement: Fail-Closed Mechanical Criteria

The verifier MUST reject a missing or invalid result schema, packet or commit mismatch, fired hard stop, unmet done criterion, undeclared or out-of-scope change, or failed required check. A rejected attempt MUST NOT create or reuse a receipt.

#### Scenario: Reject invalid result evidence

- GIVEN result evidence is missing, schema-invalid, mismatched, has a fired hard stop, or has an unmet done criterion
- WHEN acceptance is attempted
- THEN acceptance fails and no receipt exists

#### Scenario: Reject scope or check failure

- GIVEN the candidate contains an undeclared or disallowed change, or a required check fails
- WHEN acceptance is attempted
- THEN acceptance fails and no receipt exists

### Requirement: Frozen Candidate Verification

The verifier MUST evaluate the candidate commit and tree named by the binding and MUST NOT verify a live primary checkout.

#### Scenario: Primary checkout changes concurrently

- GIVEN a frozen candidate and a primary checkout that changes afterward
- WHEN acceptance runs
- THEN only the frozen candidate is evaluated

### Requirement: Owned Isolation and Cleanup

Each attempt MUST use unique verifier-owned isolation, report cleanup success or failure, and MUST NOT alter a foreign worktree.

#### Scenario: Clean owned isolation

- GIVEN an attempt created verifier-owned isolation
- WHEN verification ends
- THEN the cleanup outcome is explicit

#### Scenario: Preserve foreign worktrees

- GIVEN another owner has a worktree
- WHEN verification or cleanup runs
- THEN that foreign worktree remains unchanged

### Requirement: Durable Receipt and Exact Cache Reuse

A successful attempt MUST atomically persist one immutable receipt. A cached receipt MAY be reused only for an exact complete-binding match; otherwise the verifier MUST verify anew.

#### Scenario: Persist successful acceptance

- GIVEN all mechanical criteria pass
- WHEN acceptance commits its outcome
- THEN exactly one complete immutable receipt becomes durable atomically

#### Scenario: Reuse only an exact receipt

- GIVEN a cached receipt exists
- WHEN every bound value matches exactly
- THEN the verifier returns that receipt idempotently
- AND any binding difference prevents reuse

### Requirement: Receipt-Gated CLI Success

The acceptance CLI MUST report success only when a valid receipt exists for that exact binding.

#### Scenario: Successful command

- GIVEN verification produces or exactly reuses a valid receipt
- WHEN the command finishes
- THEN it exits successfully and identifies that receipt

#### Scenario: Receipt absent

- GIVEN no valid exactly bound receipt exists
- WHEN the command finishes
- THEN it exits unsuccessfully

### Requirement: No Promotion Authority

Acceptance MUST NOT mutate refs or invoke human Promotion/CAS.

#### Scenario: Accepted candidate remains unpromoted

- GIVEN a candidate receives a valid acceptance receipt
- WHEN acceptance completes
- THEN repository refs remain unchanged
- AND no Promotion/CAS action occurs

### Requirement: Mechanical Evidence Is Not Semantic Approval

Hashes, bindings, checks, and receipts MUST NOT be represented as proof of semantic correctness. Qualitative review MUST remain a separate decision.

#### Scenario: Present an acceptance receipt

- GIVEN a valid mechanical acceptance receipt
- WHEN its meaning is reported
- THEN it is described only as mechanical evidence
- AND qualitative approval is not implied
