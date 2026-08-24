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
4. Remove the blocked Lane's preserved worktree and lane branch before reusing the same ID.
5. Retry the blocked feature. When the opposing feature still matches the recorded resolution basis, Promotion uses the registered SHA.

Decline or cancel when no candidate should proceed. Renew re-anchors a stale reconciliation request; it is unrelated to feature lease renewal.

## Multi-wave sequencing

Candidate combination branches from the packet's declared `base_sha`/`parent_ref`, not from mutable primary-checkout `HEAD` (`integration-target-isolation`, merged `77f6f00`). What still requires discipline: each packet declares a static `base_sha`, not an automatically-tracked moving tip, so a stale `base_sha` on a later wave can still replace rather than accumulate an earlier wave's promoted content.

After each successful wave, advance the primary checkout to the promoted parent tip, refresh both `base_sha` and `expected_parent_sha` in every next-wave packet, and align the parent ref before dispatch. Confirm the prior wave's files with git tree evidence.

## Bisection boundary

Exclusive integration may combine completed Lanes, run checks, and bisect a red batch to find a clean subset. Bisection is post-execution recovery. Feature attempts fail whole and have no equivalent. A Lane listed in `reverted_ids` keeps its worktree and branch for inspection; do not equate reverted with lost.
