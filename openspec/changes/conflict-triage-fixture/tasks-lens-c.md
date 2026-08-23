# Tasks Lens C — Proof & Review Burden: Conflict Triage Fixture

## Assumed decomposition

The change decomposes into 4 units across two new packages and one existing service extension: (1) payload schema and output-only reconcile update (`internal/conflicttriage/types.go`, `internal/reconcile/reconcile.go`), (2) fail-open advisory triage agent with invoker (`internal/conflicttriage/triage.go`, `internal/conflicttriage/invoker.go`), (3) deterministic 3-hunk fixture generator (`internal/conflicttriage/fixture/fixture.go`), and (4) dual-judge evaluation rubric and judge packets (`internal/conflicttriage/fixture/rubric.go`, `internal/conflicttriage/fixture/packets/`). The critical path runs from payload types and ledger output seam to the triage agent, then through the fixture generator to the offline rubric.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1100–1700 (impl ~540–940, tests ~550–930, templates/comments ~50–100) |
| 400-line budget risk | Low (evaluated against real 2000-line review budget) |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

The estimate is grounded in 7 new-or-modified files across two new packages (`internal/conflicttriage/` and `internal/conflicttriage/fixture/`) and the existing reconcile service. Compared to archived change `2026-08-20-apply-dag-dispatch-hardening` (650–1200 lines across 5 modified files with strict TDD), this change introduces two fresh packages, three test suites, and judge packet templates. Production implementation requires ~540–940 lines (`types.go` ~40–70, `triage.go` ~150–250, `invoker.go` ~40–60, `reconcile.go` output update ~30–60, `fixture.go` ~120–200, `rubric.go` ~100–180, packets ~60–120), while unit and integration tests require ~550–930 lines (`triage_test.go` ~200–350, `fixture_test.go` ~150–250, `rubric_test.go` ~150–250, `reconcile_test.go` ~50–80). The total range of 1100–1700 lines sits comfortably beneath the change's 2000-changed-line review budget, so chained PRs are not recommended.

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | Applicable | `TestScanConflictMarkers_DocumentationAndScriptPaths` | `resolve.ScanConflictMarkers` scans text/documentation/script files containing conflict markers, ignores binary files with NUL bytes (`internal/resolve/candidate.go:80`), and returns `hasMarkers == true` without executing script content (`internal/resolve/candidate_test.go:16-49`). | Triage candidate marker validation in `internal/conflicttriage/triage.go`. |
| Git repository selection | Applicable | `TestReconcileResolve_RejectsLinkedWorktree`; `TestEnforceAllowedPaths_ExplicitWorktreeCwd` | `runReconcileResolve` refuses execution from inside a linked worktree (`internal/worktree/worktree.go:278-292`, `cmd/lucind-ai/cli.go:1478-1481`), and `resolve.EnforceAllowedPaths` passes `-C worktreePath` explicitly to git diff commands (`internal/resolve/candidate.go:107-133`). | Invoker git execution and candidate validation in `internal/conflicttriage/triage.go`. |
| Commit state | Applicable | `TestEnforceAllowedPaths_StagedAndUntrackedDisjoint` | `resolve.EnforceAllowedPaths` performs 4-way diff union against `base_sha` (committed-since-base, unstaged, staged, untracked), catching out-of-scope edits and returning `resolve.ErrOutOfScopeEdits` (`internal/resolve/candidate.go:100-145`, `internal/resolve/candidate_test.go:51-93`). | Triage candidate path-scope invariant enforcement in `internal/conflicttriage/triage.go`. |
| Push state | N/A: local `git update-ref` only (`integrate.go:151-173`) | None | — | — |
| PR commands | N/A: no host PR argv | None | — | — |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Payload schema & output persistence | `go test -run ^TestUpdateCandidateOutputOnly$ ./internal/reconcile -v -count=1` | Service persists structured JSON to `Candidate.Output` in ledger without mutating `CandidateSHA` or advancing status (`internal/reconcile/reconcile.go:105`, `internal/ledger/schema.go:163`). | Does not prove the agent emits well-formed JSON or that CLI display renders the output. |
| Fail-open triage agent & semantic ratchet | `go test -run ^TestTriageAgent_BusinessHunkPinsHighRisk$ ./internal/conflicttriage -v -count=1` | Agent marks business hunk ARBITRARY, pins risk `high`, emits wall-clock verify budget without fail-closed `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`, `internal/resolve/candidate.go:303-312`). | Does not prove LLM prompts produce valid JSON across unstubbed production models. |
| Triage invariant validation | `go test -run ^TestTriageAgent_InvariantViolationsFailCandidate$ ./internal/conflicttriage -v -count=1` | Residual markers or out-of-scope edits fail candidate validation via `ScanConflictMarkers` and `EnforceAllowedPaths` (`internal/resolve/candidate.go:48-145`). | Does not prove git merge commands cannot corrupt the repository before validation runs. |
| Deterministic 3-hunk fixture generator | `go test -run ^TestFixtureGenerator_ForcesClassRequired$ ./internal/conflicttriage/fixture -v -count=1` | Generator creates 2 leased features with shared `base_sha` and 3-hunk toy file that forces `ClassRequired` in `evaluateOverlapGate` (`internal/run/attempt.go:687`, `internal/overlap/overlap.go:623-659`). | Does not prove real repositories with complex merge topologies classify identically. |
| Fixture missing base SHA bypass | `go test -run ^TestFixtureGenerator_MissingBaseSHASkipsClassRequired$ ./internal/conflicttriage/fixture -v -count=1` | Divergent or missing `base_sha` yields `ErrNoMergeBase` and skips `ClassRequired` block (`internal/run/attempt.go:743-747`). | Does not prove git merge-base resolution handles multi-parent octopus merges. |
| Dual-judge rubric scoring & isolation | `go test -run ^TestRubric_GradesDistinctThreeHunkClassification$ ./internal/conflicttriage/fixture -v -count=1` | Rubric grades distinct 3-hunk classification for `claude-opus-5` and `openai/gpt-5.6-sol` without cross-provider config leaks (`internal/executor/claude.go:35-52`, `internal/executor/opencode.go:53-65`). | Does not prove live network responses from Claude/OpenAI APIs will not timeout or drift. |
| Uniform hunk score rejection | `go test -run ^TestRubric_RejectsUniformHunkScoring$ ./internal/conflicttriage/fixture -v -count=1` | Rubric rejects judge outputs that assign uniform class or risk to all three hunks (`openspec/changes/conflict-triage-fixture/specs/triage-evaluation-rubric/spec.md:19-24`). | Does not prove fine-grained semantic quality of model rationale prose. |
| Two-step approval & CAS retry | `go test -run ^TestReconcileResolveCLI$ ./cmd/lucind-ai -v -count=1` | `reconcile approve` followed by `reconcile resolve --sha` from primary root sets `CandidateSHA` and allows compare-and-swap retry (`cmd/lucind-ai/cli.go:1478-1506`, `internal/integrate/integrate.go:151-173`). | Does not prove concurrent CLI invocations race-free without ledger file locks. |

## Verification Gaps

Live multi-model LLM execution against real cloud endpoints (`claude-opus-5` and `openai/gpt-5.6-sol`) is not proven in unit CI, as tests use isolated runner stubs (`internal/executor/claude_test.go:18-26`, `internal/executor/opencode_test.go:19-27`); proving live invocation requires manual end-to-end evaluation runs with configured credentials. Production triage executor selection and continuous numeric risk formulas remain open in design (`openspec/changes/conflict-triage-fixture/design.md:118-123`) and are intentionally not asserted by unit tests.

## Open Questions

- [ ] Exact non-decreasing risk formula and thresholds for mixed business+mechanical hunks (`openspec/changes/conflict-triage-fixture/design.md:118-123`).
- [ ] Which executor/model runs production triage (`openspec/changes/conflict-triage-fixture/design.md:118-123`).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:1478-1481` | `runReconcileResolve` refuses execution from inside a linked worktree |
| `cmd/lucind-ai/cli.go:1478-1506` | `runReconcileResolve` validates primary worktree and updates candidate status to integrated with candidate SHA |
| `cmd/lucind-ai/cli.go:1501-1506` | `runReconcileResolve` updates candidate status to integrated with candidate SHA |
| `cmd/lucind-ai/cli_test.go:3126-3150` | `TestReconcileResolveCLI` proves human reconciliation SHA registration CLI flow |
| `internal/executor/claude.go:35-52` | `Claude` executor default and known model is pinned to `claude-opus-5` |
| `internal/executor/claude_test.go:18-26` | `writeClaudeStub` provides isolated subprocess test double without network access |
| `internal/executor/opencode.go:53-65` | `Opencode` executor default model is pinned to `openai/gpt-5.6-sol` |
| `internal/executor/opencode_test.go:19-27` | `writeOpencodeStub` provides isolated subprocess test double without network access |
| `internal/integrate/integrate.go:151-173` | `PromoteCASWithRunner` updates branch ref via atomic compare-and-swap |
| `internal/ledger/schema.go:163` | `reconciliation_candidates.output` column stores structured text |
| `internal/overlap/overlap.go:623-659` | `Classify` evaluates conflict signals against thresholds to return `ClassRequired` |
| `internal/reconcile/reconcile.go:105` | `Candidate.Output` struct field holds unstructured or JSON string |
| `internal/reconcile/reconcile_test.go:56-100` | `TestCreateRequestFromRequiredOverlapDisplaysExactFields` proves exact request creation fields |
| `internal/resolve/candidate.go:26` | `ErrSemanticAmbiguity` sentinel error indicates unresolved ambiguity |
| `internal/resolve/candidate.go:48-145` | Invariant checks `ScanConflictMarkers` and `EnforceAllowedPaths` |
| `internal/resolve/candidate.go:80` | `ScanConflictMarkers` skips binary files containing NUL bytes |
| `internal/resolve/candidate.go:100-145` | `EnforceAllowedPaths` validates 4-way diff union against `base_sha` |
| `internal/resolve/candidate.go:107-133` | `EnforceAllowedPaths` passes `-C worktreePath` explicitly to git diff commands |
| `internal/resolve/candidate.go:303-312` | `ResolveCandidateMerge` prompt instructs resolver to fail closed on ambiguity |
| `internal/resolve/candidate_test.go:16-49` | `TestScanConflictMarkers` tests conflict marker detection in files |
| `internal/resolve/candidate_test.go:51-93` | `TestEnforceAllowedPaths` tests in-scope vs out-of-scope diff detection |
| `internal/run/attempt.go:687` | `evaluateOverlapGate` evaluates overlap during promotion attempt |
| `internal/run/attempt.go:743-747` | `evaluateOverlapGate` skips blocking on `overlap.ErrNoMergeBase` |
| `internal/run/gate_test.go:122-160` | `TestRequiredOverlapGateBlocksBothParentsWhileAdmissionSucceeds` proves overlap gate blocking |
| `internal/worktree/worktree.go:278-292` | `IsLinkedWorktree` inspects `.git` file pointer to detect linked worktrees |
| `openspec/changes/conflict-triage-fixture/design.md:104-110` | Threat matrix reference table embedded in design document |
| `openspec/changes/conflict-triage-fixture/design.md:118-123` | Open questions on risk formula and production triage runtime |
| `openspec/changes/conflict-triage-fixture/specs/triage-evaluation-rubric/spec.md:19-24` | Spec scenario requiring rejection of uniform hunk scoring |
