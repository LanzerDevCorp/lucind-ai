# Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

None

## Coverage Gaps

None. All nine spine items are present in `proposal.md`. The sdd-propose skill's 450-word cap and Intent/Scope template are superseded by this packet's 1800-word budget and spine; that is packet precedence, not a missing spine item. Dedicated "Dependencies" and "Success Criteria" headings from the skill template are folded into Approach and Delta Specifications rather than omitted.

## Dropped Citations

1. **Lens B — `internal/reconcile/reconcile.go:163-165` writes triage JSON to candidate output.** Those lines are `NewService` assigning the default evaluator to `overlap.Evaluate`. The storage slot is `Candidate.Output` at `:105`. The JSON-in-output claim is kept at `:105`; the `:163-165` attribution is dropped.

2. **Lens A — `internal/ledger/schema.go:131-169` as ledger-backed features with a registered common `base_sha`.** That range defines `overlap_evidence`, `reconciliation_requests`, and `reconciliation_candidates`. Feature rows and leases sit earlier in the file. `Create` requires a non-empty `baseSHA` at `internal/feature/feature.go:123-124`. Schema `:163` is still used for the existing `output` column; `:156-169` for the candidates table. This is not a resurrection of explore drop #7 (0-row counts); it is a separate misattribution of the same line range.

3. **Lens A — `internal/feature/feature.go:101-113` as registered common `base_sha`.** Those lines are `ValidateParentRef` (reject empty, `main`, `lucind/`). Base SHA is required at `:123-124`. The parent-ref claim is kept at `:101-113`; the base-SHA attribution is dropped.

4. **Lens C / B — `internal/serve/handlers.go:95-109` as read-only GET reconcile routes.** Those lines are `/api/state` pagination constants (`stateMaxRuns`, `stateMaxLanes`). The web-stays-read-only out-of-scope item is packet ground truth and is kept **without** this citation (packet forbids retargeting a failed citation).

5. **Lens A — `cmd/lucind-ai/cli.go:1090-1120` as `reconcile approve`.** `:1090-1111` is `IsLinkedWorktree` on **feature renew**; `:1113-1136` is `reconcileDispatch`. `runReconcileApprove` starts at `:1138`. Approve behavior is kept via `internal/reconcile/reconcile.go:406-535` and usage at `cmd/lucind-ai/cli.go:56`.

6. **Lens A/B/C — `cmd/lucind-ai/cli.go:1398-1463` as `reconcile resolve`.** `:1397-1432` is still **renew** (`Renew` at `:1416`). `runReconcileResolve` starts at `:1445`. The resolve linked-worktree check is at `:1478-1481` (uncited; not retargeted). Resolve is kept via usage `:56` and `internal/worktree/worktree.go:278-292`.

7. **Lens C — `cmd/lucind-ai/cli.go:1430-1433` as resolve linked-worktree rejection.** Those lines print renew success. Dropped.

8. **Lens C — `internal/run/gate_test.go:41-100` as overlap-gate tests.** That range is `newGateTestDeps`. First `Test*` is at `:122`. Dropped.

9. **Lens C — `internal/overlap/overlap_test.go:37-60` as `Classify` → `ClassRequired` tests.** That range is `TestCaptureRaw_PredictableDiff`. Dropped.

10. **Lens C — `cmd/lucind-ai/cli_test.go:2825-2900` as reconcile CLI tests.** That range asserts feature-renew lease expiry. `TestReconcileApproveCLI` is at `:2944`; `TestReconcileResolveCLI` at `:3126`. Those latter symbols are kept; `:2825-2900` is dropped.

11. **Lens C — `internal/executor/claude_stream_test.go:1-30` as stream-decoder tests.** That range is package preamble plus `writeClaudeStreamStub`. Dropped as decoder-coverage evidence. `Claude.Run` fallback is kept at `internal/executor/claude.go:106-122` and `claude_stream.go:10-16`.

12. **Lens B — `internal/run/attempt.go:687-752` as the `ClassRequired` persist/block path.** Function starts at `:687`; `Evaluate` at `:743`; `ClassRequired` handling at `:777`; `CreateRequest` at `:831`; block at `:848`. `:687-752` includes the skip on `ErrNoMergeBase` but not evidence insert or block. Kept: `:687` (function), `:743-747` (skip), `:777-855` (required path). Dropped: `:687-752` as persist/block.

Explore dropped citations were not resurrected: `attempt.go:768-775` as ClassRequired evidence; `worktree.go:79-81` as worktree preservation; `integrate_feature.go:17-52` as path-disjoint isolation; `claude.go:35-50` as four-executor registry; `openspec/config.yaml:31` as a duration budget; `worktree.go:99-110` as `FeatureTarget`'s parent-ref error; `schema.go:131-169` as 0-row proof.

## Scope Divergence

Lens A's Candidate 1 (two disjoint features + dual-judge rubric) is authoritative. Lens B did not propose a competing candidate; its impact table and delta specs assume the operator loop (fixture → `ClassRequired` → approve → resolve → retry) and the 3-hunk business/mechanical split. That cost Lens B a process open question (sdd-propose skill vs three-lens fan-out) that is not product scope and is not in `proposal.md`.

Lens C independently rejected Candidate 2: `ErrNoMergeBase` makes `evaluateOverlapGate` `continue` (`internal/run/attempt.go:743-747`), so an in-memory harness with no registered shared `base_sha` never reaches `ClassRequired`; a synchronous LLM in the gate couples promotion to external latency. Candidate 3 is adjacent to C's "no public CLI until calibration" stance; C did not recommend shipping `fixture generate` / `reconcile triage`.

Independent convergence across all three: two separately dispatched *build* features that must not collide with each other; do not unify `conflict-triage` with fail-closed `internal/resolve`; 3-hunk fixture (one business measurement, two mechanical controls); out of scope is UI reconcile POST, overlap thresholds, and production dispatch.

Content from B or C that did **not** enter `proposal.md` because it contradicts Lens A / decided product questions:

- Lens B scenario "mechanical-only hunks must not set risk to `high`" answers the still-open non-decreasing formula (mixed business+mechanical thresholds). The mechanical-resolution behavior is kept; the risk-band claim for mechanical-only is not.
- Lens C qualitative layer grading prepared-commit validity. A/B win criterion is 3-hunk classification plus ARBITRARY where it belongs; grading the prepared resolution was rejected.
- Lens A "~30 second" operator review as a measurable outcome. Timing a human was rejected as an A/B criterion; it is not a success metric here.
- Lens A/B process open questions about skill template vs fan-out. Packet sets the 1800-word spine; not a product question.

`internal/conflicttriage/**` (agent) prefix-includes `internal/conflicttriage/fixture/` under `PathInScope` (`internal/packet/disjoint.go:17-18`). Packet ownership still names those paths and requires separate dispatches so mixed-feature admission (`integrate_feature.go:17,41`) never sees both in one batch. Exact `allowed_paths` encoding so the two *build* features also do not overlap-gate each other is a design detail, not a candidate fork.
