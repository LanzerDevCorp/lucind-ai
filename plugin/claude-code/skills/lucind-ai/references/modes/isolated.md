# Isolated Mode

Load this module for an Isolated Mode Change or feature-targeted dispatch.

## Current mapping

Isolated Mode is the canonical product term. Current CLI realizes part of it through feature-targeted packets and durable integration attempts; there is no `--isolated` flag.

Every packet in one batch must name identical values for `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha`. The Orchestrator creates the git parent branch and separately records it with `lucind-ai feature create`; the command writes the ledger row and does not create the branch. Lanes do not create or move parent refs.

A feature's `parent_ref`/`base_sha` stay immutable while it is active: re-running `feature create` against an existing active feature with a different anchor is rejected, because attempts may already be recorded against the current one. `lucind-ai feature disable --id <id>` is the explicit signal that an anchor is abandoned — most commonly because its base turned out to be red. Disabling transitions the feature to `disabled` and removes it from the active-feature set the overlap gate iterates. Once disabled, the same ID is no longer immutable: `lucind-ai feature create --id <id>` against that ID re-anchors and reactivates it under a corrected `parent_ref`/`base_sha`, instead of returning the immutability error. This is retire-and-recreate, not in-place re-anchoring of a live feature.

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
