# Tasks: Conflict Triage Fixture

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1100–1700 (impl ~540–940, tests ~550–930, packets ~50–100) |
| 400-line budget risk | Low (judged against the human 2000-line review budget) |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

**Dispatch:** No `apply-dag.yaml` sidecar. Single packet, four sequential work-unit commits. Each unit contains its RED tests and GREEN production so Integrate (`internal/run/integrate.go:50-59`) sees a green combined tree. A two-wave DAG is possible after remapping types into Unit 1, but sidecar overhead is not justified at this size.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary | allowed_paths | Executor |
|------|------|-----------|----------------------|-----------------|-------------------|---------------|----------|
| 1 | Payload types, `TriageInvoker`, output-only persist | PR 1 | `go test ./internal/reconcile -run '^TestUpdateCandidateOutputOnly$' -count=1` | N/A — ledger unit tests; no new listen path | revert `internal/reconcile/reconcile.go` persist method; delete `internal/conflicttriage/types.go`, `invoker.go` | `internal/conflicttriage/types.go`, `internal/conflicttriage/invoker.go`, `internal/reconcile/reconcile.go`, `internal/reconcile/reconcile_test.go` | `cursor-agent` |
| 2 | Threat-matrix RED plus fail-open `RunTriage` | PR 1 | `go test ./internal/conflicttriage -run '^TestTriageAgent_BusinessHunkPinsHighRisk$' -count=1` | N/A — stub `TriageInvoker`; live LLM is out of CI | delete `internal/conflicttriage/triage.go`, `triage_test.go`; revert `internal/resolve/candidate_test.go` | `internal/resolve/candidate_test.go`, `internal/conflicttriage/triage.go`, `internal/conflicttriage/triage_test.go` | `cursor-agent` |
| 3 | 3-hunk fixture, disjoint packets, CAS/linked-worktree tests | PR 1 | `go test ./internal/conflicttriage/fixture -run '^TestFixtureGenerator_ForcesClassRequired$' -count=1` | N/A — fixture is in-process; CLI linked-worktree is `go test ./cmd/lucind-ai` | delete `internal/conflicttriage/fixture/fixture.go`, `fixture_test.go`, `packets/`; revert `internal/run/attempt_test.go`, `cmd/lucind-ai/cli_test.go` | `internal/conflicttriage/fixture/fixture.go`, `internal/conflicttriage/fixture/fixture_test.go`, `internal/conflicttriage/fixture/packets/`, `internal/run/attempt_test.go`, `cmd/lucind-ai/cli_test.go` | `cursor-agent` |
| 4 | Offline A/B rubric on pinned judges | PR 1 | `go test ./internal/conflicttriage/fixture -run '^TestRubric_GradesDistinctThreeHunkClassification$' -count=1` | N/A — isolated executor stubs (`internal/executor/claude_test.go:18-26`, `opencode_test.go:19-27`); no live network | delete `internal/conflicttriage/fixture/rubric.go`, `rubric_test.go` | `internal/conflicttriage/fixture/rubric.go`, `internal/conflicttriage/fixture/rubric_test.go` | `agy` |

Same-wave pairs: none (sequential). File-level `PathInScope` (`internal/packet/disjoint.go:13-22`): Unit 2 package-root files do not prefix `fixture/`. Do not list `internal/conflicttriage` as a directory.

## Dependency Order

| Task | Depends on | Why |
|------|------------|-----|
| 1.1, 1.2, 1.3, 2.1, 3.1 | — | Types, invoker, persist RED, threat RED, and fixture RED are independent |
| 1.4 | 1.3 | GREEN persist satisfies 1.3 |
| 2.2 | 1.1, 1.2, 1.4 | Agent tests need payload, invoker, and output persist |
| 2.3 | 2.1, 2.2 | Agent GREEN must satisfy 2.2 and call verified invariants |
| 3.2 | 3.1 | Generator satisfies fixture RED |
| 3.3 | 3.2 | Packets wrap generated feature paths |
| 3.4 | 3.2 | CAS retry needs a `ClassRequired` fixture; linked-worktree RED is CLI-only |
| 4.1 | 1.1, 3.2 | Rubric grades `TriagePayload` shape against the 3-hunk fixture |
| 4.2 | 4.1 | Rubric GREEN on pinned judges |

Unit commit order: 1 → 2 → 3 → 4. Units 1 and 3 are path-disjoint and could run in parallel; this packet does not.

## Phase 1: Foundation Models & Output Persistence

- [ ] 1.1 Create `internal/conflicttriage/types.go` with `TriagePayload`, `HunkDecision`, risk bands `low`/`medium`/`high`, and verify-budget string `~N min: <cmd>` (`design.md:66-72,81`). Do not encode a numeric risk formula.
- [ ] 1.2 Create `internal/conflicttriage/invoker.go` with `TriageInvoker` func type for stubbed prompt execution (`design.md:83,100`).
- [ ] 1.3 RED: `TestUpdateCandidateOutputOnly` in `internal/reconcile/reconcile_test.go` — persist JSON to `Candidate.Output` without changing status or `CandidateSHA` (`reconcile.go:105`; `schema.go:163`; `UpdateCandidateStatus` SQL at `reconcile.go:873-876` does not touch `output`).
- [ ] 1.4 GREEN: add output-only `Service` update via `ledger.UpdateReconciliationCandidate` (`ledger.go:1314-1338`). Do not route through `UpdateCandidateStatus` (`reconcile.go:848-908`).

## Phase 2: Advisory Triage Agent & Invariants

Threat-matrix RED (Applicable rows only; Push state and PR commands are N/A):

- [ ] 2.1 RED in `internal/resolve/candidate_test.go` before 2.3: `TestScanConflictMarkers_DocumentationAndScriptPaths` (scan docs/scripts, skip NUL binaries, never exec — `candidate.go:80`); `TestEnforceAllowedPaths_ExplicitWorktreeCwd` (`git -C` at `candidate.go:107-133`); `TestEnforceAllowedPaths_StagedAndUntrackedDisjoint` (4-way union, `ErrOutOfScopeEdits` — `candidate.go:100-145`). These lock existing helpers; they must stay green without editing `candidate.go` production.

- [ ] 2.2 RED in `internal/conflicttriage/triage_test.go`: `RunTriage` resolves mechanical hunks, flags business hunk ARBITRARY, pins risk `high`, emits verify budget, never returns `ErrSemanticAmbiguity` (`candidate.go:26,303-312`; `specs/conflict-triage/spec.md:9-30`). Also `TestTriageAgent_InvariantViolationsFailCandidate` via `ScanConflictMarkers` / `EnforceAllowedPaths` (`candidate.go:48-145`).
- [ ] 2.3 GREEN: `internal/conflicttriage/triage.go` `RunTriage` using `TriageInvoker`, JSON in `Candidate.Output` only (`proposed_sha`, never `CandidateSHA`), fail-open, then the two invariant helpers.

## Phase 3: Three-Hunk Fixture & Disjoint Packets

- [ ] 3.1 RED in `internal/conflicttriage/fixture/fixture_test.go`: `GenerateFixture` yields two features sharing `base_sha` and a 3-hunk toy file that `Classify` maps to `ClassRequired` (`overlap.go:93-98,623-659`; `specs/conflict-fixture/spec.md:9-31`). Also `TestFixtureGenerator_MissingBaseSHASkipsClassRequired` — missing/divergent `base_sha` → `ErrNoMergeBase` continue (`attempt.go:743-747`).
- [ ] 3.2 GREEN: `internal/conflicttriage/fixture/fixture.go` — two leased features, one `base_sha` (`feature.go:123-124`), 3-hunk file (1 business, slice-literal union, rename-vs-edit) for `evaluateOverlapGate` (`attempt.go:687`).
- [ ] 3.3 Create `internal/conflicttriage/fixture/packets/` with two sequential dispatches, prefix-disjoint `allowed_paths` (`disjoint.go:29-48`), valid `parent_ref` (`feature.go:101-113`; `integrate_feature.go:17,26-78`).
- [ ] 3.4 RED (no production edits): `TestReconcileResolve_RejectsLinkedWorktree` in `cmd/lucind-ai/cli_test.go` (`worktree.go:278-292`; `cli.go:1478-1481`). CAS: in `internal/run/attempt_test.go`, matching tip adopts `CandidateSHA` and `PromoteCASWithRunner` (`attempt.go:821-828,870`; `integrate.go:151-173`); tip drift re-blocks (`attempt.go:848-855`); N-way stays blocked (`attempt.go:873-891`). Existing `TestReconcileResolveCLI` (`cli_test.go:3126`) proves SHA registration only.

## Phase 4: Dual-Judge Rubric

- [ ] 4.1 RED in `internal/conflicttriage/fixture/rubric_test.go`: `EvaluateRubric` passes distinct 3-hunk classification, rejects uniform hunk scores (`specs/triage-evaluation-rubric/spec.md:9-30`).
- [ ] 4.2 GREEN: `internal/conflicttriage/fixture/rubric.go` grades offline A/B on registered `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol` (`cli.go:65-70`; `claude.go:35-52`; `opencode.go:53-65`). Judges only — do not name a production triage executor.

Full tree: `./lucind-checks.sh`.

## Requirement Traceability

| Requirement | Tasks |
|-------------|-------|
| Deterministic three-hunk fixture (`specs/conflict-fixture/spec.md`) | 3.1, 3.2, 3.3 |
| Semantic triage and risk ratchet (`specs/conflict-triage/spec.md`) | 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3 |
| Two-step close and retry CAS (`specs/reconciliation-approval/spec.md`) | 3.4 |
| Dual-judge rubric isolation (`specs/triage-evaluation-rubric/spec.md`) | 4.1, 4.2 |

## Open Questions

Remain open (`design.md:118-123`). Do not resolve in apply:

- Exact non-decreasing risk formula and thresholds for mixed business+mechanical hunks.
- Which executor/model runs **production** triage (judges are pinned; production is separate).
