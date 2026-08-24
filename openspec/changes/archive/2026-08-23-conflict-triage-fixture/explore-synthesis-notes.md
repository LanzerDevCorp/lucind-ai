# Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

None

## Coverage Gaps

None

## Dropped Citations

1. **Lens B Scenario 1 — `internal/run/attempt.go:768-775` writes `overlap_evidence` for `ClassRequired`.** Those lines insert evidence only on `overlap.ClassWarning`. `ClassRequired` evidence is inserted inside `CreateRequest` at `internal/reconcile/reconcile.go:266`. The ClassRequired-creates-evidence claim is kept, retargeted to `:266`.

2. **Lens B Scenario 1 — `internal/worktree/worktree.go:79-81` preserves the blocked lane worktree at `lucind/<laneID>`.** Those lines are `BranchFor`, which returns the branch name `"lucind/" + laneID`. They do not preserve worktrees. Preservation is packet ground truth (blocked trees kept until `worktree cleanup`); the failed citation is not used.

3. **Lens B impact — `internal/run/integrate_feature.go:17-52` as path-disjoint isolation of the two build features.** `FeatureTarget` rejects mixed feature targets, mixed legacy/feature packets, and divergent expected parent SHAs in one batch. Path disjointness is `internal/packet/disjoint.go:29-48`. The mixed-batch claim is kept at `:17,:41`; the path-isolation attribution is dropped.

4. **Lens B impact — `internal/executor/claude.go:35-50` as the four-executor registry.** That range is Claude's `DefaultModel` / `KnownModels` (`claude-opus-5` only). The four executors are `cmd/lucind-ai/cli.go:65-70`. The Claude model pin is kept at `:35`; the four-executor attribution is dropped.

5. **Lens A open question — `openspec/config.yaml:31` as a test-suite execution-time budget metric.** Line 31 is `test_command: "go test ./... -race -count=1"`, a command string, not a duration. The verify-budget question is kept without this citation.

6. **Lens B Scenario 4 — `internal/worktree/worktree.go:99-110` as `FeatureTarget`'s `ErrInvalidParentRef`.** `FeatureTarget` calls `feature.ValidateParentRef` (`internal/run/integrate_feature.go:73`; `internal/feature/feature.go:101-113`). `worktree.ValidateParentRef` is a parallel git-aware check used at worktree create, not that call site.

7. **Lens B impact — `internal/ledger/schema.go:131-169` as evidence of 0 rows.** That range *defines* `overlap_evidence`, `reconciliation_requests`, and `reconciliation_candidates`. It does not report counts. The 0-row fact is packet ground truth from `.lucind/lucind.db` and is kept without this citation. Schema `:131-154` is still used as the table destination in success criteria.

## Approach Divergence

Lens A's problem statement and three candidates are primary. Lens B did not propose a competing approach; its scenarios and success criteria assume Candidate 1's operator loop (fixture → `ClassRequired` → approve → resolve → retry) and the 3-hunk business/mechanical split. That cost Lens B a process open question (explore skill vs three-lens split) that is not product scope and is not in `explore.md`.

Lens C independently made Candidate 2 unviable: `ErrNoMergeBase` causes `evaluateOverlapGate` to `continue` (`internal/run/attempt.go:743-747`), so an in-memory harness with no registered shared `base_sha` never reaches `ClassRequired`; and hooking an LLM into `CreateRequest` / the attempt loop couples promotion to external latency. Candidate 3 is adjacent to C's open question (CLI diagnostic vs `internal/conflicttriage/fixture/`) but C did not recommend shipping public CLI before calibration.

Independent convergence across all three: two path-disjoint *build* features that must not collide with each other; do not unify `conflict-triage` with fail-closed `internal/resolve`; 3-hunk fixture (one business measurement, two mechanical controls); out of scope for this change is UI reconcile POST, overlap thresholds, and production dispatch. Open questions that survived merge: verify-budget units, risk formula/thresholds, fixture packaging, triage JSON payload, production executor/model.
