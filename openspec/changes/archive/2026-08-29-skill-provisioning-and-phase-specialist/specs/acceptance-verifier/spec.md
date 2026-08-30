# Delta for acceptance-verifier

## MODIFIED Requirements

### Requirement: Fail-Closed Mechanical Criteria

The verifier MUST reject a missing or invalid result schema, packet or candidate-commit mismatch, fired hard stop, unmet done criterion, undeclared or out-of-scope change, failed required check, or missing required skill declaration. For versioned contracts it MUST also reject any missing, extra, duplicate, reordered, or altered authored criterion, hard stop, or required skill; mode or commit disagreement; and any path or change-classification mismatch against the canonical frozen candidate change set. A rejected attempt MUST NOT create or reuse a receipt.
(Previously: Mechanical criteria verified criteria, hard stops, checks, and changed paths against frozen evidence without validating required skills correspondence.)

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

#### Scenario: Complete skills loaded accepted

- GIVEN required skills `["lucind-executor", "lucind-fan-out-lens", "sdd-propose"]` and an envelope declaring matching `skills_loaded` with `status: done`
- WHEN mechanical acceptance runs
- THEN acceptance MUST succeed.

#### Scenario: Superfluous declared skills tolerated

- GIVEN required skill `["lucind-executor"]` and an envelope declaring `skills_loaded: ["lucind-executor", "extra-skill"]`
- WHEN mechanical acceptance runs
- THEN acceptance MUST succeed.

#### Scenario: Reject missing required skill

- GIVEN a versioned result whose `skills_loaded` omits a frozen required skill
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists.
