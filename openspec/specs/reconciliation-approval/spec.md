# Delta for Reconciliation Approval

## ADDED Requirements

### Requirement: Visible Balanced Evidence

Lucind SHALL classify overlap as informational, warning, or reconciliation-required using deterministic, balanced signals including predicted Git conflicts, nearby hunks, and weighted hotspots. Evidence MUST expose base/tip SHAs, relevant paths and changes, signal rationale, and checks. Optional structural evidence MAY supplement but MUST NOT solely determine a class.

#### Scenario: Evidence is classified
- GIVEN two active parent ranges share a merge base
- WHEN overlap is evaluated
- THEN the class and supporting deterministic signals SHALL be recorded and visible

#### Scenario: Structural evidence is unavailable
- GIVEN optional structural evidence is stale or unavailable
- WHEN classification runs
- THEN classification MUST use other evidence and disclose the omission

### Requirement: Required Reconciliation Gate

Reconciliation-required evidence MUST block promotion of both affected parents while allowing lane admission and dispatch. Informational and warning evidence MUST remain visible but SHALL NOT require direction approval.

#### Scenario: Required overlap is pending
- GIVEN reconciliation-required evidence affects parents A and B
- WHEN either parent attempts promotion
- THEN promotion MUST be blocked while new lanes MAY dispatch

#### Scenario: Warning is present
- GIVEN overlap is classified as warning
- WHEN a parent otherwise qualifies for promotion
- THEN the warning SHALL remain visible without blocking promotion

### Requirement: Exact Expiring Direction Approval

The future localhost web request SHALL offer only an exact source-to-target decision for one record, plus decline or cancellation. It MUST NOT infer direction or offer approve-all. Requests SHALL expire; renewal MUST recompute fresh evidence. Approval SHALL bind the evidence snapshot and exact expected source and target SHAs.

#### Scenario: Direction is approved
- GIVEN a current request displays source A, target B, evidence, and expected SHAs
- WHEN an actor approves A to B before expiry
- THEN only that direction and snapshot SHALL be authorized and audited

#### Scenario: Request expires or state changes
- GIVEN a request expired or either expected SHA changed
- WHEN approval or renewal is attempted
- THEN old authority MUST be rejected and renewal SHALL use fresh evidence

### Requirement: One Bounded Candidate

One valid approval SHALL authorize exactly one bounded Sonnet candidate in the approved target context. Automatic CAS promotion SHALL occur only when mandatory checks pass, limits are honored, no conflict markers remain, and both expected refs are unchanged. No second approval is required.

#### Scenario: Authorized candidate passes
- GIVEN one direction-bound candidate satisfies every gate
- WHEN promotion validates both expected refs
- THEN the target parent SHALL advance by CAS and the source SHALL remain unchanged

#### Scenario: Candidate is unsafe
- GIVEN checks fail, time expires, limits are exceeded, refs are stale, or markers remain
- WHEN promotion is evaluated
- THEN promotion MUST fail closed and preserve candidate and evidence

### Requirement: Resolver Authority and Observable Audit

The resolver MUST NOT choose direction or invent business semantics and SHALL fail closed on semantic ambiguity. Durable status and audit history SHALL expose actor, timestamps, evidence/version, exact direction and SHAs, candidate/check outcomes, failures, and CAS result for the future localhost UI. That UI does not yet exist. Lucind MUST NOT promote to `main` or assume review or delivery ownership.

#### Scenario: Meaning is ambiguous
- GIVEN a direction-bound conflict requires an unproven business decision
- WHEN the resolver cannot establish a check-backed result
- THEN it MUST block without inventing semantics

#### Scenario: Status is inspected before UI exists
- GIVEN any reconciliation transition occurs
- WHEN durable state is queried without a web UI
- THEN the complete observable status and audit history SHALL be available
