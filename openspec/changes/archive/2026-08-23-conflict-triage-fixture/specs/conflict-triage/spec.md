# Conflict Triage Specification

## Purpose

An advisory agent that explains ClassRequired overlap, proposes resolutions, and records risk without the fail-closed contract of the existing resolver.

## Requirements

### Requirement: Semantic triage and risk ratchet

Conflict triage MUST explain the cause of a pending reconciliation, resolve mechanical hunks deterministically, and MUST NOT fail closed on semantic ambiguity. For a business conflict with no technical selection criterion, triage MUST flag the hunk ARBITRARY, record why that side was chosen, pin risk to high (and MUST NOT lower it), and state verify cost as wall-clock duration plus a concrete command. Structured JSON MUST be stored in the candidate's output. Residual conflict markers or edits outside allowed paths MUST fail candidate validation. Triage MUST NOT relax the resolver's fail-closed authority.

#### Scenario: Business conflict ratchets risk to high

- GIVEN an awaiting reconciliation request containing a business hunk with no technical selection criterion
- WHEN conflict-triage evaluates the conflict
- THEN it flags the hunk ARBITRARY, pins risk to high, records a wall-clock verify budget with a concrete command, and writes structured JSON to candidate output

#### Scenario: Mechanical controls resolve without fail-closed error

- GIVEN slice-literal union and rename-versus-edit control hunks
- WHEN conflict-triage processes the hunks
- THEN it emits deterministic resolution proposals and completes without a semantic-ambiguity failure

#### Scenario: Invariant violations fail candidate validation

- GIVEN a triage candidate that leaves conflict markers or edits outside allowed paths
- WHEN invariant checks run
- THEN validation fails and the candidate is marked failed without erasing ledger auditability
