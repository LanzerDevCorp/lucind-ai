# Tasks: Conflict Triage Fixture

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1100–1700; below the approved 2000-line single-PR budget |
| 400-line budget risk | High (nominal skill guard; approved size exception covers the real budget) |
| Chained PRs recommended | No |
| Suggested split | One packet, four sequential work-unit commits |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Executor | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|---|
| 1 | Payload/invoker seams and output-only candidate persistence | PR 1 | Single packet; no DAG | `./lucind-checks.sh` | N/A: library seam and ledger tests use `t.TempDir()` | `types.go`, `invoker.go`, reconcile output method/tests |
| 2 | Fail-open triage, risk ratchet, and invariant contracts | PR 1 | Single packet; no DAG | `./lucind-checks.sh` | N/A: stub invoker plus temporary repository | `internal/conflicttriage/` triage files and added invariant tests |
| 3 | Three-hunk fixture, disjoint packets, and retry evidence | PR 1 | Single packet; no DAG | `./lucind-checks.sh` | N/A: deterministic temporary Git/ledger fixture | `fixture.go`, `fixture/packets/`, fixture and gate tests |
| 4 | Offline dual-judge rubric and isolation checks | PR 1 | Single packet; no DAG | `./lucind-checks.sh` | N/A: registered-executor stubs; no cloud calls | `rubric.go` and rubric tests |

No `apply-dag.yaml` sidecar: apply this as one sequential packet. Each work unit keeps its RED and GREEN together; Integrate must check the combined tree before the next unit. There are no parallel same-wave pairs, so component-boundary disjointness is vacuous; if dispatch is parallelized, use the exact file boundaries above and remember a directory path covers descendants (`internal/packet/disjoint.go:13-48`).

## Phase 1: Foundation and Output Persistence

- [ ] 1.1 Create `internal/conflicttriage/types.go` with `TriagePayload`, `HunkDecision`, `low|medium|high` bands, verify-budget text, and `proposed_sha`; keep the exact numeric risk formula open (`design.md:63-73,118-123`).
- [ ] 1.2 Create `internal/conflicttriage/invoker.go` with the stub-friendly `TriageInvoker` function boundary (`design.md:23-28,81-84`).
- [ ] 1.3 RED: add `TestUpdateCandidateOutputOnly` in `internal/reconcile/reconcile_test.go`, proving JSON output persistence leaves status and `candidate_sha` unchanged (`reconcile.go:97-107`; `schema.go:156-166`).
- [ ] 1.4 GREEN: add `Service.UpdateCandidateOutput` in `internal/reconcile/reconcile.go`, updating only `Candidate.Output` through `ledger.UpdateReconciliationCandidate` (`reconcile.go:846-876`; `ledger.go:1313-1338`).

## Phase 2: Advisory Triage and Threat Controls

- [ ] 2.1 RED: add `TestScanConflictMarkers_DocumentationAndScriptPaths` in `internal/resolve/candidate_test.go`, covering text marker detection, NUL-binary skipping, and no script execution; add `TestEnforceAllowedPaths_ExplicitWorktreeCwd` and `TestEnforceAllowedPaths_StagedAndUntrackedDisjoint` for explicit `git -C`, four-way scope checks, and out-of-scope failure (`candidate.go:46-100,107-168`).
- [ ] 2.2 RED: add `TestReconcileResolve_RejectsLinkedWorktree` in `cmd/lucind-ai/cli_test.go`, asserting linked-worktree mutation is rejected while the primary-root path remains valid (`cli.go:1472-1485`; `worktree.go:271-292`).
- [ ] 2.3 RED: add `internal/conflicttriage/triage_test.go` cases for mechanical resolution, business `ARBITRARY` plus non-decreasing `high`, verify-budget command, JSON output, and invariant failure without `ErrSemanticAmbiguity` (`specs/conflict-triage/spec.md:9-29`).
- [ ] 2.4 GREEN: implement `RunTriage` in `internal/conflicttriage/triage.go`; use `TriageInvoker`, persist JSON via `UpdateCandidateOutput`, reuse marker/path checks as validation only, never write `CandidateSHA`, and do not choose the open production executor or risk thresholds (`candidate.go:46-100`; `design.md:23-28,63-75,118-123`).

## Phase 3: Deterministic Fixture and Approval Retry

- [ ] 3.1 RED: add fixture tests for exactly one business and two mechanical hunks, shared `base_sha` forcing `ClassRequired`, missing/divergent-base bypass, and separate prefix-disjoint feature dispatch (`specs/conflict-fixture/spec.md:9-30`; `overlap.go:622-659`).
- [ ] 3.2 GREEN: create `internal/conflicttriage/fixture/fixture.go` to register two active leased features with shared base, generate the three-hunk toy file, and expose deterministic temporary-repository options (`feature.go:101-133`; `attempt.go:738-747`).
- [ ] 3.3 Create `internal/conflicttriage/fixture/packets/` templates with valid `parent_ref`, exact component-disjoint `allowed_paths`, and separate feature-targeted dispatches (`packet/disjoint.go:8-48`; `integrate_feature.go:26-77`).
- [ ] 3.4 Add integration coverage in `internal/run/attempt_test.go` and `cmd/lucind-ai/cli_test.go` for evidence/request blocking, approve → primary-root resolve, matching-tip retry promotion, and tip-drift re-blocking (`attempt.go:777-855`; `cli.go:1478-1506`; `integrate.go:150-173`).

## Phase 4: Dual-Judge Rubric and Final Verification

- [ ] 4.1 RED: add `internal/conflicttriage/fixture/rubric_test.go` cases rejecting uniform hunk class/risk, requiring business `ARBITRARY` separation, and proving each judge uses only its registered provider configuration (`specs/triage-evaluation-rubric/spec.md:9-29`).
- [ ] 4.2 GREEN: create `internal/conflicttriage/fixture/rubric.go` for offline A/B grading of the identical fixture on `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol`; do not grade prepared resolutions or human timing (`cli.go:65-70`; `claude.go:33-52`; `opencode.go:51-65`).
- [ ] 4.3 Run `./lucind-checks.sh` on the combined tree and record failures before shipping; do not add N/A Push-state or PR-command RED tests (`integrate.go:45-59`; `design.md:102-110`).

## Requirement Trace

| Requirement | Tasks |
|---|---|
| Deterministic three-hunk fixture | 3.1–3.3 |
| Semantic triage and risk ratchet | 1.1–1.4, 2.1–2.4 |
| Two-step close and retry CAS | 1.3–1.4, 3.4 |
| Dual-judge rubric isolation | 4.1–4.2 |

Dependencies are explicit: Phase 1 precedes triage; fixture tests precede its generator and packets; rubric tests follow the fixture; every later Integrate check uses the combined tree. The production triage executor/model and exact non-decreasing risk formula/thresholds remain unresolved design questions.
