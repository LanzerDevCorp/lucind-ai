# Synthesis Notes: Native Stability Campaign

## Unresolved Contradictions

None.

All three drafts agree on the problem boundary, candidate architecture (modular subpackages under `internal/stability`), storage under `<git-common-dir>/lucind-ai/stability/v1/`, 3 sequential Stability Trials, real `agy` dispatches with `gemini-3.7-flash-high`, 10-second lease expiry and reclaim, abrupt process group termination, content-addressed JSON receipt, and V1 non-goals. The minor divergence in status JSON output formatting (full records vs summary references) was surfaced as an open question rather than a contradictory conflict.

## Coverage Gaps

None.

None of the packet's eight exploration-spine items were missing from the drafts. No draft sized exact SQLite table schemas, column types, or Go API signatures; those belong to the specification and design phases. Control Room UI and external issue trackers were properly excluded per V1 non-goals.

## Dropped Citations

Every citation below was opened and checked in this worktree against real code. The claim was removed from `explore.md` or rewritten with the correct citation:

1. **`cmd/lucind-ai/cli.go:158-164` (B)** — Cited in prose for "requiring explicit interactive confirmation defaulting to no". Those lines parse flags (`-packet`, `-timeout`, `-approval-timeout`, `-legacy-main`, `-expected-parent-sha`) for `lucind-ai run`. Interactive confirmation defaulting to "no" is not yet implemented in `cli.go` (it is a master plan Decision 32 requirement for `stability run`). The confirmation claim was dropped from this citation.

2. **`cmd/lucind-ai/cli.go:1577-1580` (B)** — Cited in prose for "Clean Linux primary repository" preflight checks. Those lines check `if worktree.IsLinkedWorktree(primaryRoot)` inside `worktree cleanup` to refuse execution from linked worktrees; they do not perform working tree cleanliness checks (`git status --porcelain`). Clean checkout validation was retargeted to `internal/integrate/integrate.go:127-138` (`Promote` porcelain check).

3. **`internal/executor/agy.go:40-40` (B)** — Cited in prose for "verifies zero surviving child processes". Line 40 is `const defaultWaitDelay = 5 * time.Second` bounding stdio pipe drain for grandchild processes; it does not perform process survivor verification. Process tree inspection and orphan termination belong to `internal/stability/process`. The survivor check claim on line 40 was dropped.

4. **`internal/run/run.go:362-369` (B)** — Cited for "Executes batch of lane dispatches concurrently". Those lines are inside `executeLane` updating lane metadata and appending `EventLaneRegistered` to the ledger. Concurrent batch dispatch is `ExecuteBatch` at `internal/run/batch.go:66`.

5. **`internal/run/run.go:708-721` (C)** — Cited for "PersistEnvelope writes result envelope JSON to primary repository results directory". Those lines are `git diff` plumbing invocations inside `enforceAllowedPaths`. The `PersistEnvelope` closure is defined in `cmd/lucind-ai/cli.go:708-721` and the `run.Deps` field is at `internal/run/run.go:212`. The citation in Spike 4 was retargeted to `cmd/lucind-ai/cli.go:708-721`.

## Approach Divergence

**Lens B** treated several future `stability` CLI behaviors (interactive confirmation defaulting to "no", zero-survivor process checks, and clean working tree checks) as if they were already implemented at existing line numbers in `cli.go` and `agy.go`. Those claims were dropped or retargeted, but Lens B's underlying scenario requirements (clean tree preflight, interactive admission, 8 observable acceptance scenarios, and success criteria) aligned completely with Master Plan requirements R1–R10.

**Lens C** focused on operating system process boundaries (Linux `Setpgid` vs Cgroups v2), storage trade-offs (common-dir vs root `.lucind/`), 10s monotonic fencing, and privacy sanitization. It corroborated Lens A's Candidate 2 decomposition by isolating process management in `internal/stability/process` and common-dir authority in `internal/stability/store`.

**Convergence**: All three lenses independently converged on:
- CLI command group `lucind-ai stability run|status|resume|abort` under product authority (ADR-0001).
- Modular subpackage architecture under `internal/stability/` (`store`, `fixture`, `process`, `evidence`, `reconcile`).
- Storage under `<git-common-dir>/lucind-ai/stability/v1/` using SQLite/WAL with single-active-campaign constraints.
- Three sequential Stability Trials with 5 non-retryable `agy` dispatches pinned to `gemini-3.7-flash-high`.
- Canonical crash recovery testing (abrupt kill of B, 10s lease expiration, explicit reclaim, ancestry isolation).
- Content-addressed JSON Stability Receipt emitted upon passing.
- Strict V1 non-goals (no NIP, no CI bypass, no external issue creation, no tagging/pushing/releasing).
