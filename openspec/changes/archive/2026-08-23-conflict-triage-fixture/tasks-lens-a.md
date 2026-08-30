# Tasks Lens A — Decomposition & Ordering: Conflict Triage Fixture

## Assumed decomposition

The implementation decomposes into four sequential phases: payload models and candidate output persistence (Phase 1), the advisory triage agent with invariant validation (Phase 2), the deterministic three-hunk fixture and disjoint packets (Phase 3), and the dual-judge evaluation rubric with retry CAS verification (Phase 4). Phase 1 establishes payload structures and ledger output persistence without modifying candidate status. Phase 2 delivers the fail-open triage agent that resolves mechanical controls, flags business hunks ARBITRARY, ratchets risk high, and enforces invariants. Phase 3 and Phase 4 construct the test fixture forcing `ClassRequired` overlap on demand and validate offline A/B grading across pinned executors without provider leakage. The critical path progresses from reconcile output persistence through agent invariant enforcement, fixture conflict generation, to dual-judge rubric grading.

## Phase 1: Foundation Models & Output Persistence

- [ ] 1.1 Create `internal/conflicttriage/types.go` defining `TriagePayload` struct, `HunkDecision` struct, risk band strings (`low`, `medium`, `high`), and verify budget formatting (`design.md:66-72,81`).
- [ ] 1.2 Create `internal/conflicttriage/invoker.go` defining `TriageInvoker` functional type for pluggable prompt execution (`design.md:83,100`).
- [ ] 1.3 Add unit test in `internal/reconcile/reconcile_test.go` asserting candidate output updates persist JSON to `reconciliation_candidates.output` without altering status or `candidate_sha` (`design.md:11-13,84`).
- [ ] 1.4 Modify `internal/reconcile/reconcile.go` adding `UpdateCandidateOutput` to `Service` to persist `Candidate.Output` via `ledger.UpdateReconciliationCandidate` without updating status (`reconcile.go:105,848-908`; `ledger.go:1314-1338`).

## Phase 2: Advisory Triage Agent & Invariants

- [ ] 2.1 Add threat-matrix regression tests in `internal/resolve/candidate_test.go` for `TestScanConflictMarkers_DocumentationAndScriptPaths`, `TestEnforceAllowedPaths_ExplicitWorktreeCwd`, and `TestEnforceAllowedPaths_StagedAndUntrackedDisjoint` (`candidate.go:48-95,100-145`; `design.md:104-110`).
- [ ] 2.2 Add unit tests in `internal/conflicttriage/triage_test.go` verifying `RunTriage` resolves mechanical hunks, flags business hunks ARBITRARY, ratchets risk `high`, emits verify budget string, and validates post-triage invariants (`specs/conflict-triage/spec.md:9-30`).
- [ ] 2.3 Create `internal/conflicttriage/triage.go` implementing `RunTriage` advisory agent using `TriageInvoker`, populating `TriagePayload` JSON in `Candidate.Output`, and executing invariant checks `ScanConflictMarkers` and `EnforceAllowedPaths` (`candidate.go:48-95,100-145`; `design.md:25-28,82`).

## Phase 3: Three-Hunk Fixture & Disjoint Packets

- [ ] 3.1 Add unit tests in `internal/conflicttriage/fixture/fixture_test.go` asserting `GenerateFixture` creates two features sharing registered `base_sha` and a 3-hunk conflicting toy file yielding `ClassRequired` (`overlap.go:93-98,623-659`; `specs/conflict-fixture/spec.md:9-31`).
- [ ] 3.2 Create `internal/conflicttriage/fixture/fixture.go` implementing `GenerateFixture` creating two active features with shared `base_sha` and a 3-hunk toy file (1 business, 2 mechanical controls: slice-literal union and rename-vs-edit) producing `ClassRequired` overlap (`feature.go:123-124`; `attempt.go:687,743-747`; `design.md:46-56,85`).
- [ ] 3.3 Create packet templates in `internal/conflicttriage/fixture/packets/` defining sequential dispatches with prefix-disjoint `allowed_paths` and valid `parent_ref` (`disjoint.go:29-48`; `feature.go:101-113`; `integrate_feature.go:17,26-78`; `design.md:32-35,87`).
- [ ] 3.4 Add integration tests in `internal/run/attempt_test.go` asserting `reconcile resolve` rejects linked worktrees (`TestReconcileResolve_RejectsLinkedWorktree`) and retry unblocks promotion via CAS on matching tip while re-blocking on tip drift (`worktree.go:278-292`; `cli.go:1478-1481`; `attempt.go:821-828,870`; `specs/reconciliation-approval/spec.md:5-20`).

## Phase 4: Dual-Judge Rubric & Verification

- [ ] 4.1 Add unit tests in `internal/conflicttriage/fixture/rubric_test.go` asserting `EvaluateRubric` passes distinct 3-hunk classification, rejects uniform hunk scoring, and enforces isolated executor configuration (`specs/triage-evaluation-rubric/spec.md:9-30`).
- [ ] 4.2 Create `internal/conflicttriage/fixture/rubric.go` implementing `EvaluateRubric` running offline A/B grading across registered `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol` executors (`cli.go:65-70`; `claude.go:35-52,106-122`; `opencode.go:53-65`; `design.md:39-42,86`).

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Self-contained type definitions for payload data model |
| 1.2 | — | Self-contained invoker function signature definition |
| 1.3 | — | Unit test setup against existing reconcile service test harness |
| 1.4 | 1.3 | Reconcile service output update method satisfies test 1.3 |
| 2.1 | — | Regression tests targeting existing invariant helper functions |
| 2.2 | 1.1, 1.2, 1.4 | Agent tests require types, invoker interface, and reconcile output persistence |
| 2.3 | 2.1, 2.2 | Agent implementation must satisfy unit tests and integrate verified invariant checks |
| 3.1 | — | Fixture unit tests against existing overlap classification engine |
| 3.2 | 3.1 | Fixture generator satisfies test 3.1 and creates shared base SHA setup |
| 3.3 | 3.2 | Disjoint packet templates wrap the generated feature fixture paths |
| 3.4 | 1.4, 3.2 | Two-step CAS retry integration test requires output persistence and fixture generator |
| 4.1 | 1.1, 3.2 | Rubric unit tests require payload types and 3-hunk fixture structure |
| 4.2 | 4.1 | Rubric implementation satisfies test 4.1 and invokes registered executors |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Deterministic three-hunk fixture (`specs/conflict-fixture/spec.md`) | 3.1, 3.2, 3.3 |
| Semantic triage and risk ratchet (`specs/conflict-triage/spec.md`) | 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3 |
| Two-step close and retry CAS (`specs/reconciliation-approval/spec.md`) | 1.3, 1.4, 3.4 |
| Dual-judge rubric isolation (`specs/triage-evaluation-rubric/spec.md`) | 4.1, 4.2 |

## Open Questions

- [ ] None. (The exact risk formula/thresholds and production executor selection remain open by design and are not resolved by any task.)

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | Reconcile CLI commands definition and subcommand routing |
| `cmd/lucind-ai/cli.go:65-70` | Pinned executor registry for Claude and Opencode models |
| `cmd/lucind-ai/cli.go:1445-1511` | `runReconcileResolve` implementation and linked worktree validation |
| `cmd/lucind-ai/cli.go:1478-1481` | `runReconcileResolve` refuses execution from inside a linked worktree |
| `cmd/lucind-ai/cli.go:1501-1506` | `runReconcileResolve` calls `UpdateCandidateStatus` with `CandidateStatusIntegrated` |
| `cmd/lucind-ai/cli_test.go:2944` | `TestReconcileApproveCLI` integration test |
| `cmd/lucind-ai/cli_test.go:3126` | `TestReconcileResolveCLI` integration test |
| `internal/executor/claude.go:35-52` | Claude executor registration and model mapping |
| `internal/executor/claude.go:106-122` | Fallback logic for raw stdout if Claude streaming JSON decode degrades |
| `internal/executor/claude_stream.go:10-16` | Claude streaming response parser types |
| `internal/executor/opencode.go:53-65` | Opencode executor model options and command runner |
| `internal/feature/feature.go:101-113` | `ValidateParentRef` refusing main or lucind branch references |
| `internal/feature/feature.go:123-124` | `feature.Service.Create` requiring non-empty `base_sha` |
| `internal/integrate/integrate.go:151-173` | `PromoteCASWithRunner` atomic compare-and-swap update |
| `internal/ledger/ledger.go:1314-1338` | `UpdateReconciliationCandidate` updating mutable candidate fields including output |
| `internal/ledger/schema.go:131-139` | `overlap_evidence` table schema definition |
| `internal/ledger/schema.go:141-154` | `reconciliation_requests` table schema definition |
| `internal/ledger/schema.go:163` | `reconciliation_candidates` `output` column schema definition |
| `internal/overlap/overlap.go:93-98` | `DefaultThresholds` default classification thresholds |
| `internal/overlap/overlap.go:623-659` | `Classify` evaluating signals and returning `ClassRequired` |
| `internal/overlap/overlap.go:634-650` | `Classify` triggers on intersecting hunks and rename/delete collisions |
| `internal/packet/disjoint.go:13-22` | `PathInScope` component-boundary path matching rule |
| `internal/packet/disjoint.go:29-48` | `DisjointAllowedPaths` verifying pairwise disjoint path scopes |
| `internal/reconcile/reconcile.go:60-64` | `CandidateStatusRunning` constant definition |
| `internal/reconcile/reconcile.go:105` | `Candidate.Output` JSON payload field |
| `internal/reconcile/reconcile.go:107` | `Candidate.CandidateSHA` commit SHA field |
| `internal/reconcile/reconcile.go:213-336` | `CreateRequest` persisting request and evidence |
| `internal/reconcile/reconcile.go:266` | `CreateRequest` inserting `overlap_evidence` record |
| `internal/reconcile/reconcile.go:280-305` | `CreateRequest` creating awaiting `reconciliation_requests` row |
| `internal/reconcile/reconcile.go:406-535` | `Approve` creating running candidate |
| `internal/reconcile/reconcile.go:463-470` | `Approve` inserting candidate with `candidate_running` status |
| `internal/reconcile/reconcile.go:496-504` | `Approve` audit event logging |
| `internal/reconcile/reconcile.go:848-908` | `UpdateCandidateStatus` transition logic |
| `internal/reconcile/reconcile.go:873-876` | `UpdateCandidateStatus` SQL updating status and candidate_sha |
| `internal/resolve/candidate.go:26` | `ErrSemanticAmbiguity` fail-closed error declaration |
| `internal/resolve/candidate.go:48-95` | `ScanConflictMarkers` scanning worktree for conflict markers |
| `internal/resolve/candidate.go:80` | `ScanConflictMarkers` checking null byte to skip binary files |
| `internal/resolve/candidate.go:100-145` | `EnforceAllowedPaths` 4-way git diff check |
| `internal/resolve/candidate.go:303-312` | Resolve prompt instructions enforcing fail-closed on ambiguity |
| `internal/resolve/candidate_test.go:16-49` | `TestScanConflictMarkers` unit test |
| `internal/resolve/candidate_test.go:51-93` | `TestEnforceAllowedPaths` unit test |
| `internal/run/attempt.go:687` | `evaluateOverlapGate` evaluating candidate overlaps |
| `internal/run/attempt.go:743-747` | `evaluateOverlapGate` skipping evaluation on `ErrNoMergeBase` |
| `internal/run/attempt.go:777-855` | `evaluateOverlapGate` creating request and blocking attempt on `ClassRequired` |
| `internal/run/attempt.go:821-828` | `evaluateOverlapGate` verifying target feature SHA match before adopting candidate |
| `internal/run/attempt.go:848-855` | `evaluateOverlapGate` setting `AttemptStatusBlocked` and releasing feature lease |
| `internal/run/attempt.go:870` | `evaluateOverlapGate` adopting `CandidateSHA` for CAS promotion |
| `internal/run/attempt.go:873-891` | `evaluateOverlapGate` handling multi-candidate conflicts |
| `internal/run/gate_test.go:122-140` | `gate_test.go` overlap gate integration test fixtures |
| `internal/run/integrate_feature.go:17` | `ErrMixedFeatureTargets` error definition |
| `internal/run/integrate_feature.go:26-78` | `FeatureTarget` extracting single target and validating parent ref |
| `internal/run/integrate_feature.go:41` | `FeatureTarget` rejecting mixed batch targets |
| `internal/run/integrate_feature.go:73` | `FeatureTarget` calling `ValidateParentRef` |
| `internal/worktree/worktree.go:278-292` | `IsLinkedWorktree` verifying gitdir pointer |
