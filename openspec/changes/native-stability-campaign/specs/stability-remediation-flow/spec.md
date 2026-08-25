# Delta for Stability Remediation Flow

## ADDED Requirements

### Requirement: Out-of-scope defect detection and recording

When Change A encounters a defect outside its Write Scope during fixture checks, it MUST persist a Defect Record and halt promotion, while Change B continues execution.

#### Scenario: Remediation proposal approval

- GIVEN Change A encounters a fixture defect outside Write Scope
- WHEN Change A emits Defect Record and Remediation Proposal
- THEN Test Actor MUST approve proposal and trigger Fix Change dispatch

#### Scenario: Independent Change B execution

- GIVEN Change A blocked on Fix Change dependency
- WHEN Change B executes concurrently
- THEN Change B MUST run to completion without blocking on Change A

### Requirement: Test Actor gated remediation and resumption

A dedicated Fix Change MUST be dispatched to rectify the defect and promote to Change A target; Change A MUST resume under original identity and promote only after Fix dependency is approved by the Test Actor.

#### Scenario: Fix promotion and resumption

- GIVEN completed Fix Change modifying authorized scope
- WHEN Fix Change promotes to Target A
- THEN Change A MUST unblock, resume under original identity, and pass verification

#### Scenario: Independent Change B promotion

- GIVEN Change B finished while Fix Change runs
- WHEN Change B initiates promotion
- THEN Change B MUST fast-forward Target B independently before Fix completes
