# Verify: apply-dag-dispatch

**Overall verdict: PASSED**

## Stage 1 — Mechanical Check

- Command: `lucind-ai check --out openspec/changes/apply-dag-dispatch/verify-mechanical.log`
- Result: **passed**, exit code 0, duration 12.610472931s.
- Candidate git SHA: `b5488f08f20f3ea9b3ed78d0dbb1964769413cbb`.
- Full transcript: `openspec/changes/apply-dag-dispatch/verify-mechanical.log` (committed, commit `6aa0f08`).

## Stage 2 — Dual Qualitative Judgment Dispatch

Two `read_only: true` packets dispatched in parallel via `lucind-ai run`:

- `verify-apply-dag-dispatch-agy` (executor: `agy`) — status `done`.
- `verify-apply-dag-dispatch-cursor-agent` (executor: `cursor-agent`) — status `done`.

Both lanes satisfied the read-only completion contract and integrated cleanly. Full structured
envelopes (including `findings[]`) were persisted to `.lucind/results/verify-apply-dag-dispatch-
{agy,cursor-agent}.json` by the newly-merged `fix-persist-lane-envelope` fix — this is the first
verify pass this session with real, independently-checkable `file:line` citations surviving past
worktree removal.

## Stage 3 — Evidence Cross-Checking & Reconciliation

**Case: Unanimous Pass** — both lanes returned `done`, `status: done`, no blocking findings.

Every done-criterion in both envelopes carries concrete `file:line` evidence. Two non-blocking
findings from the cursor-agent lane were independently re-verified by the orchestrator against
the real, current codebase (not merely restated):

1. **Cross-wave overlap without a direct `depends_on` edge is not checked.**
   `internal/dag/waves.go:64-74` calls `packet.DisjointAllowedPaths` only on `currentWave` — the
   set of nodes Kahn's algorithm places in the *same* wave. A 3-node graph where packet A and
   packet C have overlapping `allowed_paths`, C `depends_on` B, and A has no edge to C at all
   would place A and C in different waves (A in wave 1, C in wave 2 behind B) and the overlap
   would never be checked. **Confirmed accurate** by reading `waves.go` directly: no pairwise,
   whole-DAG disjointness check exists anywhere in the file, only the per-wave one. Every named
   test scenario (`TestWaves_SameWaveOverlapRejected`, `TestWaves_CrossWaveOverlapAllowedWithEdge`,
   etc.) still passes, since none of them constructs this specific no-edge/different-wave case —
   this is a real gap in the *design's* stated guarantee ("an overlap with no `depends_on` edge
   between the two packets is rejected", `design.md:42`), not a contradiction of any tested
   scenario. Non-blocking: no test asserts the broader guarantee, so nothing regresses; a future
   `apply-dag.yaml` author relying on the broader claim should be aware of this.
2. **Index-only staged-but-uncommitted paths are outside the three-way diff union.**
   `internal/run/run.go:474-496` builds the union from `git diff <base> HEAD` (committed-since-
   base), plain `git diff` (unstaged, i.e. worktree-vs-index), and `git ls-files -o
   --exclude-standard` (untracked). **Confirmed accurate**: a file that is `git add`-ed
   (staged) but not committed, with no further edit after staging, produces no output from any of
   the three commands — it is not committed (misses the first), it exactly matches the index so
   plain `git diff` shows nothing (misses the second), and it is tracked in the index so
   `ls-files -o` does not list it (misses the third). The spec (`allowed-paths-enforcement/
   spec.md`) names exactly these three components, so this is a residual hole versus "everything
   the lane touched," not a contradiction of a named requirement. Non-blocking for the same
   reason: no named scenario stages-without-committing an out-of-scope file.

Both findings are genuine, verified gaps worth tracking as future hardening, but neither
contradicts a tested spec scenario or blocks this verdict.

## Result

`openspec/changes/apply-dag-dispatch/state.yaml` updated: `verify: { status: done }`.
