# Delegated Packet Author Shadow Specification

## Purpose

Measure delegated typed authoring without granting the specialist canonical packet, target, or dispatch authority.

## Requirements

### Requirement: Permission-Bounded Typed Output

The shadow specialist MUST receive only authoring facts permitted for comparison and MUST return typed target-free contract data. It MUST NOT select or emit live targets, render canonical Markdown, write project files, allocate worktrees or quota, invoke dispatch, or perform other side effects. Invalid or unauthorized output MUST be rejected as shadow evidence.

#### Scenario: Valid typed output
- GIVEN bounded authoring facts without live targets
- WHEN the specialist returns schema-valid target-free contract data
- THEN only the trusted compiler MAY bind and render that data for comparison

#### Scenario: Specialist attempts authority
- GIVEN specialist output contains a target, dispatch instruction, or unauthorized field
- WHEN output validation runs
- THEN the output MUST be classified invalid and MUST NOT affect canonical dispatch

### Requirement: Comparable Shadow Evidence

The manual and specialist paths MUST receive the same source facts and, when compilation is possible, the same late target binding. Each attempt MUST record specialist validity, normalized semantic differences, rendered digest equality and replay stability, latency, operator review cost, and a typed failure or fallback class. Evidence MUST distinguish equivalence from mere schema validity.

#### Scenario: Equivalent shadow artifact
- GIVEN valid specialist output semantically equivalent to the manual contract
- WHEN both paths compile with the same binding
- THEN evidence MUST record field equivalence, digest comparison, stability, latency, and review cost

#### Scenario: Semantic mismatch
- GIVEN valid specialist output that changes a criterion, stop, mode, or path
- WHEN comparison runs
- THEN evidence MUST identify the differing normalized fields and classify non-equivalence

#### Scenario: Deterministic replay mismatch
- GIVEN repeated compilation of the same specialist output and binding yields different bytes or digests
- WHEN stability is evaluated
- THEN evidence MUST classify deterministic instability

### Requirement: Non-Blocking Failure and Fallback

Timeout, unavailable routing, invalid typed data, schema failure, compiler rejection, and detected executor fallback MUST be distinct shadow failure classes. Each MUST remain non-blocking when the canonical manual packet passes universal admission. Executor fallback MUST NOT be reported as successful specialist execution.

#### Scenario: Invalid specialist output
- GIVEN the specialist returns malformed or schema-invalid data
- WHEN shadow processing completes
- THEN the failure class MUST be recorded and the admitted manual packet MUST dispatch unchanged

#### Scenario: Executor fallback detected
- GIVEN routing silently executes a fallback agent instead of the named specialist
- WHEN executor identity is checked
- THEN evidence MUST record fallback detection and MUST NOT attribute output to the specialist

#### Scenario: Timeout or route unavailable
- GIVEN the shadow invocation times out or cannot be routed
- WHEN the manual packet remains safe
- THEN the observation MUST be recorded and manual dispatch MUST proceed

### Requirement: Manual Canonicality and Explicit Disable

For this Change, only the admitted manual artifact SHALL be canonical and dispatchable. No metric, success rate, or operator action MUST automatically cut over to specialist output. Disabling or rolling back shadow invocation MUST leave manual admission and dispatch behavior available and MUST NOT require conversion of stored manual packets.

#### Scenario: Shadow outperforms manual comparison
- GIVEN shadow evidence satisfies every measured quality dimension
- WHEN dispatch selects its packet
- THEN the admitted manual artifact MUST remain canonical

#### Scenario: Shadow disabled
- GIVEN shadow invocation is disabled or rolled back
- WHEN a safe manual packet is dispatched
- THEN dispatch MUST proceed without shadow output or automatic cutover
