# Tasks Lens B — Partition & Dispatch Shape: Conflict Triage Fixture

## Assumed decomposition

The change partitions into 4 standalone work units across 3 capability boundaries: Unit 1 adds an output-only candidate update method to `reconcile.Service` (`internal/reconcile/reconcile.go`); Unit 2 implements payload types, the fail-open invoker, invariant checks, and the advisory triage agent in `internal/conflicttriage/`; Unit 3 creates the 3-hunk fixture generator with shared `base_sha` in `internal/conflicttriage/fixture/fixture.go`; and Unit 4 implements the offline A/B evaluation rubric and judge packet fixtures in `internal/conflicttriage/fixture/rubric.go` and `internal/conflicttriage/fixture/packets/`. The critical path is Unit 1 → Unit 2 for candidate output persistence, and Unit 2 + Unit 3 → Unit 4 for rubric grading against the 3-hunk fixture, while Unit 1 and Unit 3 are independent foundation deliverables.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Add output-only candidate update method to `reconcile.Service` with unit tests | `internal/reconcile/reconcile.go`<br>`internal/reconcile/reconcile_test.go` | `cursor-agent` | Restores `reconcile.Service` to status-updating candidate methods only, removing output-only updates |
| 2 | Implement `TriagePayload` types, fail-open `TriageInvoker`, invariant checks, and advisory triage agent | `internal/conflicttriage/types.go` (new file)<br>`internal/conflicttriage/types_test.go` (new file)<br>`internal/conflicttriage/invoker.go` (new file)<br>`internal/conflicttriage/invoker_test.go` (new file)<br>`internal/conflicttriage/triage.go` (new file)<br>`internal/conflicttriage/triage_test.go` (new file) | `cursor-agent` | Removes `internal/conflicttriage/` package root files (payload types, invoker, and triage agent) |
| 3 | Create 3-hunk conflict fixture generator with shared `base_sha` forcing `ClassRequired` | `internal/conflicttriage/fixture/fixture.go` (new file)<br>`internal/conflicttriage/fixture/fixture_test.go` (new file) | `cursor-agent` | Removes `internal/conflicttriage/fixture/fixture.go` generator and fixture unit tests |
| 4 | Implement offline A/B evaluation rubric rejecting uniform scores and author disjoint judge packets | `internal/conflicttriage/fixture/rubric.go` (new file)<br>`internal/conflicttriage/fixture/rubric_test.go` (new file)<br>`internal/conflicttriage/fixture/packets/claude_judge.md` (new file)<br>`internal/conflicttriage/fixture/packets/opencode_judge.md` (new file) | `agy` | Removes `internal/conflicttriage/fixture/rubric.go` and judge packet fixtures under `internal/conflicttriage/fixture/packets/` |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1, Unit 3 | Yes | Yes: Unit 1 adds additive output persistence to `reconcile.Service` with unit tests; Unit 3 creates the fixture generator against existing `feature` and `overlap` packages with unit tests; the combined tree compiles and passes `lucind-checks.sh` independently. |
| 2 | Unit 2, Unit 4 | Yes | Yes: Unit 2 implements the fail-open agent consuming Wave 1 `reconcile` updates; Unit 4 implements the rubric and judge packets consuming Wave 1 fixture primitives; both are path-disjoint and pass `lucind-checks.sh` once Wave 1 is integrated. |

## Disjointness Check

- **Wave 1 (Unit 1 vs Unit 3)**:
  - Unit 1 `allowed_paths`: `internal/reconcile/reconcile.go`, `internal/reconcile/reconcile_test.go`
  - Unit 3 `allowed_paths`: `internal/conflicttriage/fixture/fixture.go`, `internal/conflicttriage/fixture/fixture_test.go`
  - Evaluation (`internal/packet/disjoint.go:13-22`): `internal/reconcile/*` vs `internal/conflicttriage/fixture/*` share no path prefix at any component boundary. Verdict: **DISJOINT (PASS)**.
- **Wave 2 (Unit 2 vs Unit 4)**:
  - Unit 2 `allowed_paths`: `internal/conflicttriage/types.go`, `internal/conflicttriage/types_test.go`, `internal/conflicttriage/invoker.go`, `internal/conflicttriage/invoker_test.go`, `internal/conflicttriage/triage.go`, `internal/conflicttriage/triage_test.go`
  - Unit 4 `allowed_paths`: `internal/conflicttriage/fixture/rubric.go`, `internal/conflicttriage/fixture/rubric_test.go`, `internal/conflicttriage/fixture/packets/claude_judge.md`, `internal/conflicttriage/fixture/packets/opencode_judge.md`
  - Evaluation (`internal/packet/disjoint.go:13-22`): Unit 2 enumerates concrete package-root files (avoiding the parent directory prefix trap); Unit 4 lists distinct files under `internal/conflicttriage/fixture/`. No path in either set prefixes or matches any path in the other. Verdict: **DISJOINT (PASS)**.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**: The change totals ~700–1100 lines across 1 modified existing file (`internal/reconcile/reconcile.go`) and 6 new files in new packages (`internal/conflicttriage/` and `internal/conflicttriage/fixture/`). Under the human-approved 2000-changed-line single-PR review budget (`openspec/changes/conflict-triage-fixture/proposal.md:29-37`), this change comfortably lands in a single PR. Authoring `apply-dag.yaml`, provisioning `apply-bodies/`, and orchestrating multi-wave bisection via `lucind-ai split` introduces unneeded complexity for a change of this size. Furthermore, archived precedent `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` declined an `apply-dag.yaml` sidecar for a 650–1200 line change on identical reasoning, and `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml:8-14` demonstrates that multi-wave DAG dispatch carries risk at the `Integrate` gate (`internal/run/integrate.go:50-59`). A single packet executed sequentially with 4 work-unit commits is the optimal dispatch shape.

## Open Questions

- [ ] Skill contract drift: `~/.claude/skills/sdd-tasks/SKILL.md` specifies a monolithic `tasks.md` with checklist, forecast, and Engram persistence, which is superseded by this 3-lens parallel task decomposition packet.
- [ ] Risk formula thresholds: `openspec/changes/conflict-triage-fixture/design.md:122` leaves open the exact non-decreasing risk formula and thresholds for mixed business and mechanical hunks during Unit 2 implementation.
- [ ] Production runtime model: `openspec/changes/conflict-triage-fixture/design.md:123` leaves open which executor/model runs production triage, as offline judge models are pinned to `claude-opus-5` and `openai/gpt-5.6-sol`.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | Reconcile CLI subcommands include approve, decline, cancel, renew, and resolve |
| `cmd/lucind-ai/cli.go:65-70` | Supported executors map defines agy, claude, cursor-agent, and opencode runners |
| `cmd/lucind-ai/cli.go:1445-1511` | runReconcileResolve is the human-operated caller registering CandidateSHA via UpdateCandidateStatus |
| `internal/executor/claude.go:35-52` | Claude executor pins default and known models strictly to claude-opus-5 |
| `internal/executor/claude.go:106-122` | Claude.Run falls back to retaining raw stdout stream if stream-json decoding degrades |
| `internal/executor/claude_stream.go:10-16` | claudeStreamDegradedMessage signals decoding fallback while retaining raw stream |
| `internal/executor/opencode.go:53-65` | Opencode executor pins default model to openai/gpt-5.6-sol within provider family |
| `internal/feature/feature.go:101-113` | ValidateParentRef rejects empty, main, or lucind/* parent refs |
| `internal/feature/feature.go:118-133` | Feature Service Create registers new feature with immutable parentRef and baseSHA |
| `internal/integrate/integrate.go:151-173` | PromoteCASWithRunner executes atomic update-ref for CAS promotion |
| `internal/ledger/schema.go:163` | Schema defines output column on reconciliation_candidates table as TEXT NOT NULL DEFAULT '' |
| `internal/overlap/overlap.go:93-98` | DefaultThresholds specifies default hotspot and nearby hunk classification thresholds |
| `internal/overlap/overlap.go:623-659` | Classify evaluates overlap signals to ClassRequired or ClassWarning |
| `internal/overlap/overlap.go:634-650` | Classify triggers ClassRequired on rename/delete collisions, shared binaries, and intersecting hunks |
| `internal/packet/disjoint.go:13-22` | PathInScope implements component-boundary prefix matching rule for POSIX paths |
| `internal/packet/disjoint.go:29-48` | DisjointAllowedPaths checks pairwise disjointness across packet allowed_paths |
| `internal/reconcile/reconcile.go:105` | Candidate struct declares Output field for serialized triage JSON |
| `internal/reconcile/reconcile.go:107` | Candidate struct declares CandidateSHA field for human-registered resolution SHA |
| `internal/reconcile/reconcile.go:266` | CreateRequest inserts overlap evidence into ledger |
| `internal/reconcile/reconcile.go:280-305` | CreateRequest inserts awaiting reconciliation request row into ledger |
| `internal/reconcile/reconcile.go:406-535` | Approve authorizes direction and creates candidate in candidate_running status |
| `internal/reconcile/reconcile.go:848-908` | UpdateCandidateStatus updates candidate status, candidate_sha, and failure_reason |
| `internal/reconcile/reconcile.go:873-876` | UpdateCandidateStatus SQL update explicitly modifies status, candidate_sha, and failure_reason without touching output |
| `internal/resolve/candidate.go:26` | Sentinel ErrSemanticAmbiguity returned on business ambiguity during candidate merge |
| `internal/resolve/candidate.go:48-95` | ScanConflictMarkers scans worktree files for unresolved git conflict markers |
| `internal/resolve/candidate.go:100-145` | EnforceAllowedPaths inspects actual 4-way diff union against baseSHA |
| `internal/resolve/candidate.go:303-312` | Resolve prompt instructs resolver to fail closed on semantic ambiguity |
| `internal/run/attempt.go:687` | evaluateOverlapGate entry point evaluating active feature overlap |
| `internal/run/attempt.go:743-747` | evaluateOverlapGate continues without blocking when ErrNoMergeBase is encountered |
| `internal/run/attempt.go:821-828` | evaluateOverlapGate adopts integrated CandidateSHA when target tip matches |
| `internal/run/attempt.go:848-855` | evaluateOverlapGate sets AttemptStatusBlocked and releases lease on required overlap |
| `internal/run/attempt.go:870` | evaluateOverlapGate sets att.CandidateSHA to resolvedOverrideSHA for single conflict |
| `internal/run/attempt.go:873-891` | evaluateOverlapGate blocks promotion on simultaneous N-way conflict resolutions |
| `internal/run/integrate.go:50-59` | Integrate runs verification checks on combined worktree and triggers bisection on failure |
| `internal/run/integrate_feature.go:17` | ErrMixedFeatureTargets sentinel error returned when batch mixes targets |
| `internal/run/integrate_feature.go:26-78` | FeatureTarget extracts and validates common feature target from batch packets |
| `internal/run/integrate_feature.go:41` | FeatureTarget rejects batches mixing legacy and feature-targeted packets |
| `internal/worktree/worktree.go:278-292` | IsLinkedWorktree inspects .git pointer to identify and reject linked worktrees |
| `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` | Archived precedent declining apply-dag sidecar for change fitting review budget |
| `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml:8-14` | Archived precedent documenting Integrate gate reversion of split TDD waves |
| `openspec/changes/conflict-triage-fixture/design.md:79-87` | Design file-changes table defining change deliverables and terminal consumers |
| `openspec/changes/conflict-triage-fixture/proposal.md:29-37` | Proposal capabilities defining conflict-triage, conflict-fixture, rubric, and reconciliation-approval |
