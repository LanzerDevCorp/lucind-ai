# Troubleshooting

Load this module for dirty roots, stale worktrees, failed or reverted Lanes, timeouts, flaky checks, or environment-specific failures.

## Dispatch and integration

| Symptom | Diagnose and recover |
|---|---|
| Primary root dirty; integration refused | Move packets to `.lucind/packets/`, preserve intended tracked edits, and restore a clean root before retry. |
| Admission failed with no worktree | Check the complete target tuple first: feature fields together, or `legacy_main` plus expected SHA. No executor ran. |
| Missing `.lucind/result.json` after executor exit 0 | The packet omitted the explicit write-and-validate instruction. Preserve any valid commit, repair the packet contract, and avoid discarding completed work. |
| `allowed_paths` mismatch | Inspect `git diff --stat HEAD^..HEAD` in the preserved Lane. Correct guessed filenames in the DAG, resplit, and order newly overlapping scopes. |
| Worktree path or branch already exists | Cleanup removes the worktree (`lucind-ai worktree cleanup --lane <id> [--force]`) but can leave `lucind/<id>`; inspect evidence, then remove both before same-ID retry. |
| Apply lane timeout or blocked mid-cycle | Execute TDD WIP-rescue: inspect the preserved worktree (`git -C <path> status` / `git -C <path> diff`), judge whether RED tests or partial GREEN work has value. If valuable, commit as WIP (`git add . && git commit -m "wip: ..."`) to retain progress, update the packet with an adjusted timeout, and re-dispatch. If work is genuinely non-recoverable, request explicit human consent before running `lucind-ai worktree cleanup --lane <id> --force`. |
| ID appears in `reverted_ids` | Reverted means excluded by integration recovery, not lost work: inspect `.lucind/result.json` and `git log -1 lucind/<id>` to confirm the Lane's own work is intact. Fix the cause first (most commonly the base itself was red — confirm with `lucind-ai check`), then run `lucind-ai integrate retry --run <run-id>` to re-integrate the preserved Lane branches with no redispatch. Only a Lane with a preserved worktree and a `"done"` envelope qualifies. |
| Wrapper died but executor may remain | Check the worktree-associated process. If alive, let it finish and inspect its envelope and commit; do not race a duplicate dispatch. |
| Synthesis timeout | Read partial branch work before rerun; use at least 30 minutes for synthesis Lanes. |
| `integrate retry` blocks again after `reconcile approve`/`resolve` | Read the exact `failure_reason` — it now names why: no request yet, approved-but-not-resolved, a resolved candidate that predates this attempt's own current content, or (previously silent) "registered against a stale tip for `<feature>`". Only that last case needs `reconcile renew` before re-approving; renew's `--source-sha`/`--target-sha` default to each feature's live `parent_ref` tip when omitted, so passing neither flag is the normal, safe path. |

## Multi-wave hazards

- Current candidate construction depends on primary-checkout `HEAD`. Between successful feature waves, advance the primary checkout, refresh both packet SHAs, align the parent ref, and verify accumulated tree content.
- Hand-authored DAG paths often predict nonexistent test filenames or miss a centralized existing file. Ground paths in repository convention and actual diffs.
- If corrected paths overlap, add DAG dependencies. The unordered-overlap error is a safety result, not a reason to bypass validation.
- A Lane body is the only actor instructed to write the result envelope. Executor schema flags do not create the file.
- If a branch was deleted before a reverted commit was inspected, the object may remain temporarily recoverable by SHA. Verify it with git, then prefer `lucind-ai integrate retry --run <run-id>` over redispatch: a Lane that already reached `"done"` with a preserved worktree does not need to re-run through an executor.

## Verification traps

- `lucind-checks.sh` is the full-tree integration gate. `lucind-lane-check.sh` checks one artifact's bookkeeping and citation existence; it does not judge quality.
- A single qualitative judgment can miss an unconsumed symbol. Cross-check task requirements and search for non-test callers before archive.
- Known full-suite timing/concurrency failures include `TestLeaseAcquisitionAndMonotonicFence`, `TestLeaseValidationAndStaleMutationRejection`, and `TestConcurrentLeaseAcquisition` under `internal/feature`; and `TestCheckingPhaseRenewsLeaseWhileChecksRun`, `TestExecuteBatchAppliesPerLaneDeadlineIndependently`, `TestExecuteApprovalWaitBlocksUntilDecideApprovePersistsDone`, and `TestExecuteBatchConcurrentLedgerWritesDoNotErrorOrLoseData` under `internal/run`. Reproduce a named failure repeatedly in isolation before classifying it as unrelated flakiness. A new failing test is not presumed flaky.
- Docker freezes environment variables at container creation; recreate the service after env changes.
- The official Postgres image may trust localhost, so an in-container localhost connection does not prove password authentication. Test through the container network address.
- `psql -c` does not interpolate `-v` variables in the safe `:'var'` form; feed SQL through stdin.

## Platform notes

On Windows, prefer PowerShell. In Git Bash, scope `MSYS_NO_PATHCONV=1` to one native command when needed; never export it globally.
