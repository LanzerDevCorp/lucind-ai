# Delta for Acceptance Verifier

## MODIFIED Requirements

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

The verifier MUST reject a missing or invalid result schema, packet or candidate-commit mismatch, fired hard stop, unmet done criterion, undeclared or out-of-scope change, or failed required check. For versioned contracts it MUST also reject any missing, extra, duplicate, reordered, or altered authored criterion or hard stop; mode or commit disagreement; and any path or change-classification mismatch against the canonical frozen candidate change set. A rejected attempt MUST NOT create or reuse a receipt.
(Previously: Result validity checked reported criteria, stops, and changed paths but could not prove exact correspondence to frozen authored declarations or commit and classification semantics.)

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
