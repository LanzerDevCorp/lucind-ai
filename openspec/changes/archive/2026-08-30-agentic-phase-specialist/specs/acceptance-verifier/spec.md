# Delta for acceptance-verifier

## MODIFIED Requirements

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
