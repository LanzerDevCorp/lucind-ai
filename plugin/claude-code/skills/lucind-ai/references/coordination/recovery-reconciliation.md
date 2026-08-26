# Recovery and reconciliation

Load this module for failed attempts, overlap blocks, lease loss, retries, or multiple apply waves.

## Durable feature attempts

Feature-targeted dispatch creates an integration-attempt row and prints its ID. A non-terminal attempt can be resumed with feature recovery. The lease is exclusive, fenced, and renewed while checks run; another dispatch on the same feature is blocked rather than raced.

The overlap gate evaluates the original candidate against every other active feature. `required` creates a deduplicated awaiting-reconciliation request, releases the lease, demotes Lanes, and preserves worktrees. Two or more simultaneously resolved conflicts block explicitly because N-way resolution composition is unsupported.

## Reconciliation sequence

Use the current `lucind-ai reconcile` operations in the order below; inspect CLI usage for exact flags.

1. Approve a request to authorize a candidate. Approval alone does not resolve textual conflict or unblock Promotion.
2. Resolve the conflict out of band within bounded paths and create a real commit.
3. Register that commit against the approved candidate; the SHA is checked against the repository graph.
4. Remove the blocked Lane's preserved worktree and lane branch before reusing the same ID. `lucind-ai worktree cleanup --lane <id>` only removes the worktree; it does not delete the `lucind/<id>` branch, so deleting the branch is a separate manual `git branch -D lucind/<id>` step.
5. Retry the blocked feature. When the opposing feature still matches the recorded resolution basis, Promotion uses the registered SHA.

Decline or cancel when no candidate should proceed. Renew re-anchors a stale reconciliation request; it is unrelated to feature lease renewal. `--source-sha`/`--target-sha` on `reconcile renew` are optional: an omitted flag now defaults to that feature's own current real `parent_ref` tip (falling back to `expected_parent_sha`, then `base_sha`) instead of silently carrying forward whatever SHA the request being renewed already had stored. Pass one or both explicitly only to pin a specific historical SHA on purpose — do not rely on partially-supplied flags to "refresh" the other side; an omitted flag is always recomputed live, never copied from the old request.

If a retry blocks again after approve/resolve, read the failure reason before re-running the sequence: `evaluateOverlapGate` names the exact reason a resolution did not clear the block — a request that exists but is not yet approved, one that is approved but not yet resolved, a resolved candidate that predates this attempt's own current content, or (the previously silent trap) a resolution *approved and registered but against a stale tip for the opposing feature* ("registered against a stale tip for `<feature>` -- recorded `<sha>`, current `<sha>`"). Only that last case, and only when unresolved, requires `reconcile renew` before re-approving; a generic "reconciliation-required overlap with feature `<id>`" with none of that detail means no request has ever existed for this pair yet.

## Multi-wave sequencing

Candidate combination branches from the packet's declared `base_sha`/`parent_ref`, not from mutable primary-checkout `HEAD` (`integration-target-isolation`, merged `77f6f00`). What still requires discipline: each packet declares a static `base_sha`, not an automatically-tracked moving tip, so a stale `base_sha` on a later wave can still replace rather than accumulate an earlier wave's promoted content.

After each successful wave, advance the primary checkout to the promoted parent tip, refresh both `base_sha` and `expected_parent_sha` in every next-wave packet, and align the parent ref before dispatch. Confirm the prior wave's files with git tree evidence.

## Bisection boundary

Exclusive integration may combine completed Lanes, run checks, and bisect a red batch to find a clean subset. Bisection is post-execution recovery. Feature attempts fail whole instead: a failing combined tree fails the whole attempt, and every Lane in that attempt's batch is reverted together, with no bisection. A Lane listed in `reverted_ids` keeps its worktree, its `lucind/<id>` branch, and its own `.lucind/result.json` for inspection; do not equate reverted with lost.

The supported recovery for a feature-targeted revert is `lucind-ai integrate retry --run <run-id>`, not a redispatch. Fix the cause first — most commonly the base itself was red, unrelated to the Lanes' own work (see `references/core/safety.md` for confirming a base is green before `feature create`). Then retry: it rebuilds the batch directly from the ledger's lane rows and each Lane's preserved worktree, with no AI dispatch involved. Only a Lane with a preserved worktree AND a `"done"` status in its own `.lucind/result.json` qualifies; pass `--lane <id>` (repeatable) to hand-pick specific Lanes instead of every qualifying one, and an explicitly named Lane that does not qualify fails the whole rebuild closed rather than silently shrinking the batch. A feature-targeted retry re-reads the feature's *current* `parent_ref`/`base_sha` from the ledger at retry time, so re-anchoring first — `lucind-ai feature disable --id <id>`, then `lucind-ai feature create` against the corrected base — composes directly with it.
