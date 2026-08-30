# Conflict Fixture Specification

## Purpose

Generate two leased features that share one registered base SHA and a three-hunk toy file so overlap evaluation yields ClassRequired on demand.

## Requirements

### Requirement: Deterministic three-hunk fixture

The fixture generator MUST create two leased features that share one registered base SHA and MUST yield ClassRequired when overlap is evaluated. The conflicting file MUST contain exactly three hunks: one business conflict where both sides compile and pass their own tests, and two mechanical controls (a slice-literal union, and a rename versus an edit). The two build features MUST keep prefix-disjoint allowed paths and MUST be dispatched as separate feature batches. When the shared base SHA is missing or the two features' base SHAs diverge, overlap evaluation MUST NOT yield ClassRequired and MUST NOT block promotion.

#### Scenario: Fixture forces ClassRequired and an awaiting request

- GIVEN two active leased features sharing a registered base SHA and conflicting across three hunks in the toy file
- WHEN overlap is evaluated during an integration attempt
- THEN classification is ClassRequired, overlap evidence and an awaiting reconciliation request are persisted, and the attempt is blocked with its lease released

#### Scenario: Missing shared base SHA skips required classification

- GIVEN two features with missing or non-matching base SHA registrations
- WHEN overlap is evaluated
- THEN evaluation reports no merge base and the attempt continues without an awaiting request or a promotion block

#### Scenario: Combined dispatch of overlapping build scopes is refused

- GIVEN two build features whose allowed paths are not prefix-disjoint
- WHEN they are admitted in a single batch
- THEN admission MUST fail
- AND separate dispatches with disjoint scopes MAY proceed
