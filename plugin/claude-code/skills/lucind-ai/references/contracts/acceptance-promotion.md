# Acceptance and Promotion

Load this module when judging Lane completion, batch integration, or final Change delivery.

## Lane and batch outcomes

- Each Lane has an independent timeout. One terminal failure does not cancel siblings.
- The barrier releases only after every Lane reaches a terminal state.
- Exit 0 requires every Lane to reach `done` and no ID in `reverted_ids`.
- `integrated_ids` and `reverted_ids` are stdout summary lines, not a separate report format.
- A `done` status does not prove Acceptance: post-execution checks may bisect or revert it.
- Completed, blocked, failed, deviated, and reverted worktrees can be preserved for evidence and recovery. Cleanup may leave the `lucind/<id>` branch.

## Acceptance gate

Accept a Lane only after independently confirming packet scope, result schema, done criteria, hard stops, changed paths, commit evidence, terminal consumers, and the applicable checks. Acceptance includes the verified result in the owning Change and does not require another human confirmation when it follows the approved strategy.

## Promotion gate

Promotion is distinct: it is the human-confirmed integration of the completed Change into its declared Integration Target. Verify the target explicitly.

Feature-targeted Promotion uses a fenced attempt, checks, overlap evaluation, and compare-and-swap on `parent_ref`. Exclusive/`legacy_main` Promotion fast-forwards the current checked-out branch and therefore depends on the operator having selected the intended target checkout.

Report accepted IDs, reverted IDs, check evidence, remaining blockers, preserved worktrees, target ref and SHA, and the human Promotion decision.
