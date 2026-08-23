# Proposal Lens C — Risks, Rollback & Test Impact: Conflict Triage Fixture

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| **Fail-Closed Prompt Coupling** | Reusing `resolve` prompt construction causes `conflict-triage` to fail closed on business ambiguity (`ErrSemanticAmbiguity`), aborting triage instead of proposing an arbitrary resolution with ratcheted risk. | Fully decouple `internal/conflicttriage/` prompt templates and invoker from `internal/resolve/candidate.go`. Test that business ambiguity yields a prepared resolution commit, flags rationale as `ARBITRARY`, and pins risk to `High`. | `internal/resolve/candidate.go:26`, `internal/resolve/candidate.go:303-312` |
| **Residual Conflict Markers & Path Scope Escape** | Agent-generated resolution commits could leave unparsed git conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) or modify paths outside declared `allowed_paths`. | Enforce post-triage invariant checks: execute `ScanConflictMarkers` and `EnforceAllowedPaths` (4-way diff union against base) prior to registering any `CandidateSHA`. | `internal/resolve/candidate.go:48-95`, `internal/resolve/candidate.go:97-150` |
| **Missing Shared Merge Base in Fixture** | If synthetic fixture branches lack a common registered `base_sha` in the ledger, `overlap.Evaluate` returns `ErrNoMergeBase` and the gate silently continues past classification without blocking. | Fixture generator must create branches from an explicit common commit registered in `features` table (`BaseSHA`); gate evaluates `overlap.Classify` with `signals.IntersectingHunks` / `RenameDeleteCollision`. | `internal/run/attempt.go:738-747`, `internal/overlap/overlap.go:622-660` |
| **Target Tip Drift Invalidation (TOCTOU)** | Overlap gate retry clears a required block only if `matchedOtherSHA == otherSHA`. If target feature tip advances after triage, resolution is invalidated and attempt re-blocks. | Fixture test harness freezes target feature tip during blocked retry cycles until CAS promotion completes via `PromoteCAS`. | `internal/run/attempt.go:821-828`, `internal/run/attempt.go:848-856`, `internal/integrate/integrate.go:151-173` |
| **Linked Worktree Execution Rejection** | `lucind-ai reconcile resolve` explicitly rejects execution when invoked from inside linked worktrees via `IsLinkedWorktree`. | Fixture test harness and CLI scripts target the primary repo root explicitly, or invoke internal service layer (`reconcile.Service.UpdateCandidateStatus`) directly in internal test suites. | `cmd/lucind-ai/cli.go:1430-1433`, `internal/worktree/worktree.go:278-292` |
| **Mixed-Feature Batch Admission Rejection** | `FeatureTarget` rejects dispatch batches mixing feature targets, bad parent refs (`main`, `lucind/*`), or diverging parent SHAs. | Dispatch `conflict-triage-agent` and `conflict-fixture` as independent, single-feature batches satisfying `ValidateParentRef` and `DisjointAllowedPaths`. | `internal/run/integrate_feature.go:17,41,73-75`, `internal/feature/feature.go:101-113`, `internal/packet/disjoint.go:29-48` |
| **Claude Stream Telemetry Degradation** | Unexpected streaming events from `claude --output-format stream-json --verbose` during A/B judge runs could trigger telemetry degradation warnings. | Rely on fallback to raw stdout capture in `Claude.Run`; verify JSON decoder coverage across stream event types without aborting dispatch execution. | `internal/executor/claude.go:106-123`, `internal/executor/claude_stream.go:10-30` |

## Rollback & Additivity

**Rollback Plan**: Reversal is executed entirely via standard `git revert` of the commits introducing `internal/conflicttriage/` (including `fixture/`) and the judge rubrics/packets. Because this change consists of additive packages and fixtures without touching the core attempt state machine or CAS promotion engine in `internal/run/attempt.go:821-871` or `internal/integrate/integrate.go:151-173`, reverting source commits cleanly and completely restores previous behavior with zero database un-migration.

**Additivity**: All formats, ledger tables, and schemas change strictly additively. The SQLite database schema (`internal/ledger/schema.go:131-169`) already defines `reconciliation_candidates` with an `output TEXT NOT NULL DEFAULT ''` column (`internal/ledger/schema.go:163`), mirrored by `Candidate.Output` (`internal/reconcile/reconcile.go:105`). The JSON payload stored within `output` introduces additive fields (e.g. wall-clock verify budget `~X min: <command>`, 3-band risk `high|medium|low`, `ARBITRARY` rationale flag) without breaking existing consumers. Reconcile web routes remain strictly read-only GET endpoints (`internal/serve/handlers.go:95-109`). No tables, columns, constraints, or wire endpoints are modified or deleted.

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| **Unit: Triage Agent & Prompt Invariants** | Verify prompt construction, fail-open handling on semantic ambiguity, `ARBITRARY` rationale marking, `High` risk band pinning on business conflicts, and verify-budget formatting (`~X min: <cmd>`). Ensure `ErrSemanticAmbiguity` is never returned. | `internal/resolve/candidate.go:26,303-326`, `internal/resolve/candidate_test.go:25-80` |
| **Unit: Post-Triage Worktree Validation** | Verify `ScanConflictMarkers` catches remaining conflict markers in resolution outputs and `EnforceAllowedPaths` rejects 4-way diff union edits outside declared paths. | `internal/resolve/candidate.go:48-95,97-150`, `internal/resolve/candidate_test.go:16-49,51-80` |
| **Unit: Fixture Collision Synthesis** | Verify deterministic generation of 3-hunk collision (1 business conflict + 2 mechanical controls: slice union, rename-vs-edit). Verify `overlap.Evaluate` and `overlap.Classify` trigger `ClassRequired` under `DefaultThresholds`. | `internal/overlap/overlap.go:93-98,622-660`, `internal/overlap/overlap_test.go:37-60` |
| **Integration: Gate Overlap & Ledger Persistence** | Verify `evaluateOverlapGate` calls `reconcile.Service.CreateRequest` to persist `overlap_evidence` and `reconciliation_requests` (`awaiting`) rows, transition attempt to `AttemptStatusBlocked`, and release lease. Verify `ErrNoMergeBase` continue behavior. | `internal/run/attempt.go:738-747,777-856`, `internal/reconcile/reconcile.go:266-290`, `internal/run/gate_test.go:41-100`, `internal/reconcile/reconcile_test.go:52-100` |
| **Integration: Reconcile CLI & CAS Promotion** | End-to-end lifecycle: `reconcile approve` (`cmd/lucind-ai/cli.go:1090-1195`) authorizes request, `reconcile resolve` (`cmd/lucind-ai/cli.go:1398-1463`) sets candidate status `integrated` with `CandidateSHA`, and retry pass clears block when `matchedOtherSHA == otherSHA` to promote via `PromoteCAS`. Verify rejection in linked worktree. | `internal/run/attempt.go:821-828,868-872`, `cmd/lucind-ai/cli.go:1430-1433`, `cmd/lucind-ai/cli_test.go:2825-2900,3013-3115` |
| **Qualitative / E2E: Multi-Model Judge Rubric** | A/B evaluation comparing `opencode` (`openai/gpt-5.6-sol`) and `claude` (`claude-opus-5`). Rubric verifies 3-hunk separation, `ARBITRARY` annotation, `High` risk pinning, and prepared resolution commit validity. Verify stream decoder stability. | `cmd/lucind-ai/cli.go:65-70`, `internal/executor/claude.go:35,106-123`, `internal/executor/opencode.go:53-54`, `internal/executor/claude_stream_test.go:1-30` |

## Out of Scope

- Modifying default overlap thresholds (`DefaultThresholds` in `internal/overlap/overlap.go:93-98`).
- Wiring reconcile POST endpoints into Web UI (`internal/serve/handlers.go:95-109` remain read-only GET routes).
- Modifying core CAS promotion or batch dispatch mechanics (`internal/integrate/integrate.go:151-173`, `internal/run/attempt.go:821-871`, `internal/run/integrate_feature.go:17-52`).
- Relaxing fail-closed rules in the autonomous merge resolver (`internal/resolve/candidate.go:26,303-312`).
- Supporting simultaneous N-way (>2) feature reconciliation (`internal/run/attempt.go:873-893`).
- Unifying or merging `conflict-triage` with `internal/resolve`.
- Allowing collision between the two build features (`feature/conflict-triage-agent` and `feature/conflict-fixture`).

## Open Questions

- [ ] What is the exact non-decreasing risk formula and threshold distribution across multi-file and mixed business+mechanical conflicts? (The fixture exists to generate data to calibrate this formula).
- [ ] Which executor/model should run production triage dispatch once calibration across the judges (`opencode`/`openai/gpt-5.6-sol` vs `claude`/`claude-opus-5`) completes?
