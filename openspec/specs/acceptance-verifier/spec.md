# Acceptance Verifier Specification

## Purpose

Define fail-closed mechanical acceptance for a frozen lane candidate.

## Requirements

### Requirement: Exact Acceptance Binding

Every decision and receipt MUST immutably bind the lane, packet, base commit and tree, candidate commit and tree, allowed paths, check policy, relevant environment identity, authoring mode, and authored evidence identity. For a versioned contract, the binding MUST include the contract version and immutable normalized evidence sufficient to verify criteria, hard stops, execution mode, commit obligation, read-only paths, and canonical changed-path semantics.
(Previously: The binding covered packet and candidate identity, allowed paths, policy, and environment but not normalized authored-contract correspondence.)

#### Scenario: Record the complete binding
- GIVEN a candidate with every required identity and authoring value
- WHEN mechanical acceptance succeeds
- THEN the receipt MUST contain the exact complete binding
- AND none of its bound values can be changed

#### Scenario: Reject an identity mismatch
- GIVEN the packet, contract evidence, commit, tree, policy, or environment differs from the requested binding
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Stale authored evidence cannot be substituted
- GIVEN a frozen candidate whose source contract or target changes later
- WHEN acceptance is attempted
- THEN Acceptance MUST use the frozen evidence and MUST reject any substituted digest or normalized contract

### Requirement: Fail-Closed Mechanical Criteria

The verifier MUST reject a missing or invalid result schema, packet or candidate-commit mismatch, fired hard stop, unmet done criterion, undeclared or out-of-scope change, or failed required check. A required check is the repository verification suite when the lane's declared SDD phase is `apply`, when that phase is empty or missing, or when an explicit check exception is configured; for every other declared SDD phase the verifier MUST skip that suite while continuing to enforce schema validation, hard stops, done criteria, and declared-scope constraints. Lane acceptance verification and attempt execution MUST apply this gate; the shared check primitive MUST remain ungated at its own definition. For versioned contracts it MUST also reject any missing, extra, duplicate, reordered, or altered authored criterion or hard stop; mode or commit disagreement; and any path or change-classification mismatch against the canonical frozen candidate change set. A rejected attempt MUST NOT create or reuse a receipt. The status-deciding step MUST explicitly evaluate every declared hard stop's `fired` value after schema validation and demote the lane to blocked when any is true, regardless of the envelope's claimed top-level status.
(Previously: Required checks ran unconditionally on every acceptance and attempt, with no SDD-phase gate.)

#### Scenario: Reject invalid result evidence
- GIVEN result evidence is missing, schema-invalid, mismatched, has a fired hard stop, or has an unmet done criterion
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Reject scope or check failure
- GIVEN the candidate contains an undeclared or disallowed change, or a required check fails
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Reject authored-result mismatch
- GIVEN a versioned result omits or changes an authored criterion or stop
- WHEN acceptance is attempted
- THEN acceptance MUST fail even when every reported entry is green

#### Scenario: Reject commit or path-class mismatch
- GIVEN a write result names another commit or misclassifies a deletion or rename endpoint
- WHEN acceptance compares it with the frozen candidate
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Preserve explicit legacy behavior
- GIVEN an admitted manual candidate is explicitly marked legacy
- WHEN acceptance runs
- THEN universal schema, scope, commit-state, and check rules MUST apply without inventing versioned declaration correspondence

#### Scenario: Fired hard stop demotes regardless of claimed status

- GIVEN a schema-valid result envelope where at least one declared hard stop's `fired` value is true
- WHEN the verifier decides status
- THEN the lane MUST be demoted to blocked even when `envelope.Status` claims `done`

#### Scenario: Apply phase executes full verification suite

- GIVEN a candidate lane declaring SDD phase `apply`
- WHEN acceptance verification or attempt execution runs
- THEN the repository verification suite is executed and passing checks are required for acceptance

#### Scenario: Planning phase skips verification suite execution

- GIVEN a planning lane declaring a non-apply SDD phase
- WHEN acceptance verification runs
- THEN the repository verification suite is skipped and acceptance is evaluated on schema, done criteria, hard stops, and scope

#### Scenario: Unlabeled lane or explicit exception executes checks

- GIVEN a lane with an empty or missing SDD phase, or declaring an explicit check exception
- WHEN acceptance verification runs
- THEN the repository verification suite is executed

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

### Requirement: Fail-Closed Mechanical Criteria Validation

Mechanical acceptance MUST validate candidate evidence against frozen authoring evidence with zero tolerance for shortfall, tolerating extra names only when they appear in the terminal prompt or stack. Dirty candidate trees MUST NOT be accepted.

#### Scenario: Clean candidate accepted with exact skills match

- GIVEN a clean worktree matching frozen authoring evidence `RequiredSkills` exactly
- WHEN acceptance verification runs
- THEN mechanical acceptance MUST succeed.

#### Scenario: Shortfall demotes to unaccepted

- GIVEN candidate evidence declaring fewer skills loaded than required by authoring evidence
- WHEN acceptance verification evaluates the candidate
- THEN acceptance MUST fail closed without writing an acceptance receipt.

#### Scenario: Extra stack or prompt skills tolerated

- GIVEN candidate evidence declaring an extra skill present in the repository stack or prompt
- WHEN acceptance verification evaluates the candidate
- THEN acceptance MUST succeed.

