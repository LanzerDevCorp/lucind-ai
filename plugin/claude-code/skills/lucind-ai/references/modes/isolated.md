# Isolated Mode

Load this module for an Isolated Mode Change or feature-targeted dispatch.

## Current mapping

Isolated Mode is the canonical product term. Current CLI realizes part of it through feature-targeted packets and durable integration attempts; there is no `--isolated` flag.

Every packet in one batch must name identical values for `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha`. The Orchestrator creates the git parent branch and separately records it with `lucind-ai feature create`; the command writes the ledger row and does not create the branch. Lanes do not create or move parent refs.

Feature attempts, Ownership Leases, CAS Promotion, active-feature overlap gates, durable recovery, and reconciliation are production-wired. Promotion updates the named ref without merging into the checked-out branch.

## Verified limitation

Feature-targeted candidate construction still depends on mutable primary-checkout `HEAD`. This means current behavior does not yet satisfy the canonical promise that arbitrary Isolated Mode Changes can safely coexist. Do not claim general N-Orchestrator safety until the separate `integration-target-isolation` Change lands.

The packet's immutable target fields still matter, but they do not remove that candidate-base dependency. Between separate runs or apply waves targeting the same feature, follow the recovery sequencing in `../coordination/recovery-reconciliation.md`; otherwise a later candidate can replace rather than accumulate earlier work.

## Admission and Promotion

- Refuse a batch that mixes feature targets, mixes feature and `legacy_main`, diverges on expected parent SHA, omits a target, or names `main`, an empty ref, or `lucind/` as a feature parent.
- Hold the feature lease across combine and checks. Renewal failure does not abort checks; validation after checks is authoritative.
- Treat overlap `required` as a Promotion block, `warning` as recorded non-blocking evidence, and `informational` as a no-op.
- A feature attempt fails whole. Legacy integration bisection is not available on this route.
