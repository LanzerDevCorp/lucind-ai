# Verify: feature-parent-integration

**Date:** 2026-08-20/21
**Overall verdict: PASSED**

## Stage 1 -- Mechanical Check

`lucind-ai check` at commit `f121989`, exit 0, 17.87s. Full transcript:
`openspec/changes/feature-parent-integration/verify-mechanical.log`. Every package `ok`,
including the seven new/modified packages this change touches: `internal/ledger`,
`internal/feature`, `internal/overlap`, `internal/reconcile`, `internal/resolve`,
`internal/serve`, `internal/run`.

## Stage 2 -- Qualitative Judgment (phase-split, 80/20 agy/cursor-agent)

The change is large (14,185 insertions across 36 files, well over `tasks.md`'s own 2,800-4,000
line estimate) -- one holistic review packet would have diluted judgment quality. Split into
three phase-scoped `read_only: true` packets per user instruction, weighted by line volume:

| Packet | Scope | Executor | Result |
|---|---|---|---|
| phase1-2 | Tasks 1.1-1.3, 2.1-2.3 (ledger, feature, admission, worktree/CAS, attempt state machine) | agy | done, 5 findings, all positive confirmations |
| phase3 | Tasks 3.1-3.4 (overlap, reconciliation gate, reconcile domain, resolver+CAS) | agy | done, 4 findings, all positive confirmations (first attempt hit an agy OAuth timeout and was cleanly redispatched -- infra flake, not a code issue) |
| phase4 | Tasks 4.1-4.3 (serve DTOs, CLI wiring, docs) | cursor-agent | done, 7 findings, non-blocking test-quality/observability gaps |

All three lanes reached `status: done`, integrated with 0 reverted, no AI-attribution trailer.
Envelopes: `.lucind/results/verify-feature-parent-integration-{phase1-2,phase3,phase4}.json`.

## Stage 3 -- Evidence Cross-Checking

Every citation below was independently re-verified against real source on `main`, not
accepted on the lanes' word alone.

### Confirmed (phase1-2, security/correctness core)

- **CAS promotion never touches the working tree.** `internal/integrate/integrate.go:151-174`
  (`PromoteCASWithRunner`) is a pure `git update-ref <ref> <candidate> <expected>` with a
  preceding `ValidateParentRef` and no checkout, no index mutation. Confirmed by direct read.
- **Admission gate, atomic audit, and lease fencing** all cited at specific `file:line`s that
  independently re-read match the claims (`internal/run/run.go:250-264`,
  `internal/ledger/ledger.go:767-807`, `internal/feature/feature.go:279-300`).

### Confirmed (phase3, the highest-risk surface -- resolver + CAS)

- **TOCTOU defense in depth, independently re-verified line by line.**
  `internal/integrate/candidate.go:75-99` re-resolves both source and target refs
  immediately after admission and fails closed to `CandidateStatusStale` on any mismatch,
  *before* any worktree or resolver work begins. `candidate.go:216-244` re-resolves both refs
  **again**, immediately before calling `PromoteCASWithRunner` -- a second, independent
  check right at the CAS boundary, not reused from the first. Only if both pass does CAS run,
  and `PromoteCASWithRunner` itself performs a third, atomic check via `git update-ref`'s own
  expected-SHA compare-and-swap. Three independent layers, not one check reused three times.
- **Failure artifacts are preserved, not silently discarded**, confirmed at
  `candidate.go:142-149,189-197` (worktree path returned, `Remove`/`DeleteBranch` never
  called on a failure path) and its test `candidate_test.go:251-253`.
- **DAG ordering (3.2 depends_on 3.3) is real in the shipped code, not just in the dispatch
  DAG** -- confirmed at `internal/run/attempt.go:634`: the promotion gate constructs
  `reconcile.NewService(deps.Ledger)` to create the reconciliation request, the same
  dependency the DAG-authoring fork encoded as an explicit edge after `lucind-ai split`
  rejected the original unordered `internal/ledger` overlap between the two tasks.

### Confirmed real, non-blocking (phase4)

1. **`deps.EvaluateOverlap` is dead code in production.** Independently re-checked:
   `productionDeps` (`cmd/lucind-ai/cli.go:525` onward) never assigns `EvaluateOverlap` in its
   `lucindrun.Deps{...}` literal, so the injection branch at `cli.go:1183-1184` is never true
   in production. `reconcile renew` still recomputes evidence correctly via the service's own
   default evaluator (`overlap.Evaluate`), so this is an unused seam, not a functional defect.
2. **`deriveCASResult` under-covers non-terminal candidate states.** Independently re-checked
   at `internal/serve/model.go:429-438`: only `"integrated"` and `"failed"` are matched;
   `candidate_running` and `stale` fall through to `not_attempted` with no dedicated test for
   either path.
3. **`feature status` has no reconciliation/audit output yet** -- confirmed the DTOs
   (`ReconciliationRequest`) carry every spec-required field, but the CLI command simply
   doesn't print them; this matches what `docs/feature-parent-integration.md` documents as
   the current limitation, not a contradiction between docs and code.
4. **CLI edge-case coverage for unknown IDs is thin** (`recover --attempt`, `reconcile
   --request` with a nonexistent ID) -- missing-flag coverage exists, unknown-ID coverage
   mostly doesn't.
5. **`feature status` swallows `GetLease`/`ListLeases` errors** as an absent lease rather than
   surfacing the read failure (`cmd/lucind-ai/cli.go:783-788,808`).

None of the five phase4 findings dispute a spec requirement or represent a shipped defect;
all are test-coverage or CLI-observability gaps.

## Verdict

**PASSED.** All requirements in both delta specs
(`specs/parent-feature-integration/spec.md`, `specs/reconciliation-approval/spec.md`) are
implemented in production and independently confirmed, not merely checked off in `tasks.md`.
The highest-risk surface -- the bounded Sonnet resolver and its CAS promotion path -- has
three independent layers of ref-staleness defense, verified line by line, and preserves
failure evidence rather than discarding it. 16 total findings across three phases, zero
blocking; the five phase4 findings are recorded as follow-ups.

## Follow-ups (not blockers, not tracked as separate SDD changes unless requested)

- Wire `productionDeps.EvaluateOverlap` (or remove the unused injection seam at
  `cli.go:1183-1184` if it will never be needed).
- Add a `deriveCASResult` test for `candidate_running`/`stale` candidates.
- Add `feature status`/`reconcile` output for reconciliation/audit state (tracked as a known
  limitation in `docs/feature-parent-integration.md` already).
- Add CLI tests asserting stderr content and exit behavior for unknown attempt/request IDs.
- Surface (not swallow) `GetLease`/`ListLeases` read errors distinctly from an absent lease.

## Real incidents this cycle (recorded for the record, none are code defects)

Full detail in `state.yaml`'s per-phase apply notes. Summary: (1) an auto-integrate revert
caused by this session's own uncommitted primary-root edits before Unit 1's dispatch,
recovered by manual merge after independent re-verification; (2) a forked agent leaving two
orphan worktrees against the primary repo during DAG-authoring, despite an explicit
instruction not to dispatch -- no data lost, cleaned up, recorded as a process-trust finding;
(3) task 2.1's own "Explicit Feature Target" admission gate rejecting every subsequent
dispatch for this same change once it shipped, requiring `--legacy-main
--expected-parent-sha` on every later `lucind-ai run` -- the feature working exactly as
specced, discovered by dogfooding it against its own remaining implementation; (4) Wave 7
integrating cleanly despite main having advanced one commit past its dispatch base mid-flight
(a self-inflicted primary-root edit while the lane was still running) -- `Combine`'s merge
step absorbed the non-conflicting divergence before `Promote`'s fast-forward ran; (5) one agy
OAuth timeout on the first phase3 verify attempt, cleanly redispatched.
