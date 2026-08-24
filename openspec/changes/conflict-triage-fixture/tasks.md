# Tasks: Conflict Triage Fixture

Single packet, sequential apply, four work-unit commits. No `apply-dag.yaml` sidecar: the change is one PR under the human 2000-line review budget, and splitting RED from GREEN across Integrate waves is reverted (`internal/run/integrate.go:50-59`).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1100–1700 (impl ~540–940, tests ~550–930, templates ~50–100) |
| 400-line budget risk | Low (judged against the human 2000-line budget, not the skill’s 400-line default) |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

Seven new-or-modified files (`design.md:79-87`) plus tests. Range sits under 2000 lines, so no chain.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary | Executor |
|------|------|-----------|----------------------|-----------------|-------------------|----------|
| 1 | Payload types, `TriageInvoker`, output-only persist | PR 1 | `go test ./internal/reconcile -run '^TestUpdateCandidateOutputOnly$' -count=1` | N/A — ledger unit tests; no new listen path | revert `UpdateCandidateOutput` in `internal/reconcile/reconcile.go`; delete `internal/conflicttriage/types.go`, `invoker.go` | `cursor-agent` |
| 2 | Threat-matrix RED plus fail-open `RunTriage` | PR 1 | `go test ./internal/conflicttriage -run '^TestTriageAgent_BusinessHunkPinsHighRisk$' -count=1` | N/A — stub `TriageInvoker`; live LLM out of CI | delete `triage.go`, `triage_test.go`; revert `internal/resolve/candidate_test.go` | `cursor-agent` |
| 3 | 3-hunk fixture, disjoint packets, two-step CAS tests | PR 1 | `go test ./internal/conflicttriage/fixture -run '^TestFixtureGenerator_ForcesClassRequired$' -count=1` | N/A — generator is not a CLI; gate/CLI already have stub seams | delete `internal/conflicttriage/fixture/fixture.go` and `packets/`; revert CAS/linked-worktree tests | `cursor-agent` |
| 4 | Offline A/B rubric, reject uniform scores | PR 1 | `go test ./internal/conflicttriage/fixture -run '^TestRubric_GradesDistinctThreeHunkClassification$' -count=1` | N/A — `writeClaudeStub` / `writeOpencodeStub` (`claude_test.go:18-26`, `opencode_test.go:19-27`) | delete `rubric.go`, `rubric_test.go` | `agy` |

`allowed_paths` (enumerate files; a directory prefix would collide):

- Unit 1: `internal/conflicttriage/types.go`, `invoker.go`; `internal/reconcile/reconcile.go`, `reconcile_test.go`
- Unit 2: `internal/conflicttriage/triage.go`, `triage_test.go`; `internal/resolve/candidate_test.go`
- Unit 3: `internal/conflicttriage/fixture/fixture.go`, `fixture_test.go`, `packets/`; `internal/run/attempt_test.go`; `cmd/lucind-ai/cli_test.go`
- Unit 4: `internal/conflicttriage/fixture/rubric.go`, `rubric_test.go`

Same-wave pairs: none dispatched. If a DAG were added later, Unit 1 files vs Unit 3 files are prefix-disjoint (`internal/packet/disjoint.go:13-22,29-48`). Unit 2 package-root files vs Unit 4 `fixture/` files are prefix-disjoint only when enumerated as files — `internal/conflicttriage/` as a directory would also grant `fixture/`. Unit 4 still cannot run in parallel with Unit 2: rubric consumes `TriagePayload` (task 4.1 depends on 1.1).

## Phase 1: Foundation Models & Output Persistence

- [ ] 1.1 Create `internal/conflicttriage/types.go` with `TriagePayload`, `HunkDecision`, bands `low`/`medium`/`high`, verify-budget `~N min: <cmd>` (`design.md:66-72,81`). Do not encode a numeric risk formula.
- [ ] 1.2 Create `internal/conflicttriage/invoker.go` with `TriageInvoker` func type (`design.md:83,100`).
- [ ] 1.3 RED: `TestUpdateCandidateOutputOnly` in `internal/reconcile/reconcile_test.go` — persist JSON to `Candidate.Output` without changing status or `CandidateSHA` (`reconcile.go:105`; `schema.go:163`; `UpdateCandidateStatus` SQL at `reconcile.go:873-876` does not touch `output`).
- [ ] 1.4 GREEN: `Service.UpdateCandidateOutput` via `ledger.UpdateReconciliationCandidate` (`ledger.go:1314-1338`). Do not reuse `UpdateCandidateStatus` (`reconcile.go:848-908`).

## Phase 2: Advisory Triage Agent & Invariants

Threat-matrix RED before agent GREEN. Existing `TestScanConflictMarkers` / `TestEnforceAllowedPaths` (`candidate_test.go:16-49,51-93`) do not cover these cases. Push state and PR commands are N/A (`design.md:104-110`) — no RED tasks.

- [ ] 2.1 RED: `TestScanConflictMarkers_DocumentationAndScriptPaths` (scan docs/scripts as text, skip NUL binaries, never exec — `candidate.go:48-95,80`); `TestEnforceAllowedPaths_ExplicitWorktreeCwd` (`git -C worktreePath`, `candidate.go:107-133`); `TestEnforceAllowedPaths_StagedAndUntrackedDisjoint` (4-way union vs `base_sha`, `ErrOutOfScopeEdits` — `candidate.go:100-145`).
- [ ] 2.2 RED: `internal/conflicttriage/triage_test.go` — `TestTriageAgent_BusinessHunkPinsHighRisk` (ARBITRARY + `high` + verify budget, no `ErrSemanticAmbiguity`); `TestTriageAgent_InvariantViolationsFailCandidate` (`specs/conflict-triage/spec.md:9-30`; `candidate.go:26,303-312`).
- [ ] 2.3 GREEN: `internal/conflicttriage/triage.go` `RunTriage` using `TriageInvoker`, JSON in `Candidate.Output` only (`proposed_sha`, never `CandidateSHA`), fail-open, then the two invariant helpers. Do not name a production executor/model.

## Phase 3: Three-Hunk Fixture & Disjoint Packets

- [ ] 3.1 RED: `TestFixtureGenerator_ForcesClassRequired` and `TestFixtureGenerator_MissingBaseSHASkipsClassRequired` in `internal/conflicttriage/fixture/fixture_test.go` — shared `base_sha` + 3-hunk toy → `Classify` `ClassRequired` (`overlap.go:93-98,623-659`); missing/divergent `base_sha` does not (`specs/conflict-fixture/spec.md:9-31`). These prove `Classify`, not `evaluateOverlapGate`.
- [ ] 3.2 GREEN: `GenerateFixture` — two leased features, one `base_sha` (`feature.go:123-124`), 3-hunk toy (1 business, slice-literal union, rename-vs-edit) (`design.md:46-56,85`).
- [ ] 3.3 Packet templates under `internal/conflicttriage/fixture/packets/` (including `claude_judge.md`, `opencode_judge.md`) with prefix-disjoint `allowed_paths` and valid `parent_ref` (`disjoint.go:29-48`; `feature.go:101-113`; `integrate_feature.go:17,26-78`).
- [ ] 3.4 RED then GREEN in one unit: `TestReconcileResolve_RejectsLinkedWorktree` in `cmd/lucind-ai/cli_test.go` (`worktree.go:278-292`; `cli.go:1478-1481`); fixture-driven retry in `internal/run/attempt_test.go` — matching tip adopts SHA and CAS (`attempt.go:821-828,870`; `integrate.go:151-173`); tip drift re-blocks (`attempt.go:848-855`; `specs/reconciliation-approval/spec.md:5-20`). Stub-driven unblock already exists (`gate_test.go` `TestApprovedIntegratedCandidateUnblocksPromotion`).

## Phase 4: Dual-Judge Rubric & Verification

- [ ] 4.1 RED: `TestRubric_GradesDistinctThreeHunkClassification` and `TestRubric_RejectsUniformHunkScoring` in `internal/conflicttriage/fixture/rubric_test.go` (`specs/triage-evaluation-rubric/spec.md:9-30`).
- [ ] 4.2 GREEN: `EvaluateRubric` offline A/B on registered `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol` (`cli.go:65-70`; `claude.go:35-52`; `opencode.go:53-65`). Isolation via stubs, not live APIs. Do not grade `proposed_sha` or human timing. Production triage runtime stays open.

## Dependency Order

| Task | Depends on | Why |
|------|------------|-----|
| 1.1, 1.2 | — | Types and invoker |
| 1.3 | — | Persist test vs existing service |
| 1.4 | 1.3 | GREEN for 1.3 |
| 2.1 | — | Regression on existing helpers |
| 2.2 | 1.1, 1.2, 1.4 | Agent tests need types, invoker, persist |
| 2.3 | 2.1, 2.2 | GREEN for agent + invariants |
| 3.1 | — | Fixture tests vs `Classify` |
| 3.2 | 3.1 | GREEN for 3.1 |
| 3.3 | 3.2 | Packets wrap fixture paths |
| 3.4 | 1.4, 3.2 | CAS path needs persist + generator |
| 4.1 | 1.1, 3.2 | Rubric grades `TriagePayload` on the 3-hunk shape |
| 4.2 | 4.1 | GREEN for 4.1 |

Order: 1 → 2 → 3 → 4. Strict-TDD RED/GREEN pairs stay in one unit.

## Requirement Traceability

| Requirement | Tasks |
|-------------|-------|
| Deterministic three-hunk fixture (`specs/conflict-fixture/spec.md`) | 3.1, 3.2, 3.3 |
| Semantic triage and risk ratchet (`specs/conflict-triage/spec.md`) | 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3 |
| Two-step close and retry CAS (`specs/reconciliation-approval/spec.md`) | 1.3, 1.4, 3.4 |
| Dual-judge rubric isolation (`specs/triage-evaluation-rubric/spec.md`) | 4.1, 4.2 |

## Open Questions

Stay open (`design.md:118-123`). No task may close them.

- Exact non-decreasing risk formula and thresholds (including mixed business+mechanical hunks).
- Which executor/model runs production triage (judges are pinned; production is separate).
