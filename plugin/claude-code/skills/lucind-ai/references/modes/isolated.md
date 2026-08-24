# Isolated Mode

Load this module for an Isolated Mode Change or feature-targeted dispatch.

## Current mapping

Isolated Mode is the canonical product term. Current CLI realizes part of it through feature-targeted packets and durable integration attempts; there is no `--isolated` flag.

Every packet in one batch must name identical values for `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha`. The Orchestrator creates the git parent branch and separately records it with `lucind-ai feature create`; the command writes the ledger row and does not create the branch. Lanes do not create or move parent refs.

Feature attempts, Ownership Leases, CAS Promotion, active-feature overlap gates, durable recovery, and reconciliation are production-wired. Promotion updates the named ref without merging into the checked-out branch.

## Candidate isolation

Feature-targeted candidate construction resolves each candidate from the packet's own `base_sha`, ancestry-checked against `parent_ref`, and checks out that exact commit — it does not depend on mutable primary-checkout `HEAD`. This landed in the `integration-target-isolation` Change (`internal/worktree/worktree.go` `createWithRunner`, merged `77f6f00`); see `internal/integrate/integrate.go:60` and the ancestry-rejection test `internal/worktree/worktree_test.go:841` (`TestCreateWithParentAncestryCheck`).

The `legacy_main`/no-target path (empty `parent_ref` and `base_sha`) still branches from primary `HEAD` by design — that is Exclusive Mode's contract, not a limitation of Isolated Mode.

The packet's immutable target fields are what make this isolation possible. Between separate runs or apply waves targeting the same feature, still follow the recovery sequencing in `../coordination/recovery-reconciliation.md`: each packet's `base_sha` is a static declared value, not an automatically-tracked moving tip, so a stale `base_sha` can still replace rather than accumulate earlier work if it isn't refreshed.

## Admission and Promotion

- Refuse a batch that mixes feature targets, mixes feature and `legacy_main`, diverges on expected parent SHA, omits a target, or names `main`, an empty ref, or `lucind/` as a feature parent.
- Hold the feature lease across combine and checks. Renewal failure does not abort checks; validation after checks is authoritative.
- Treat overlap `required` as a Promotion block, `warning` as recorded non-blocking evidence, and `informational` as a no-op.
- A feature attempt fails whole. Legacy integration bisection is not available on this route.
