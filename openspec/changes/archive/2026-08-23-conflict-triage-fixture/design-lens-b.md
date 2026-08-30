# Design Lens B — Surface & Flow: Conflict Triage Fixture

## Assumed architecture

The change introduces `internal/conflicttriage` for advisory triage and `internal/conflicttriage/fixture` for reproducible 3-hunk conflict generation with dual-judge scoring. Existing ledger schema tables (`internal/ledger/schema.go:156-169`) and reconcile types (`internal/reconcile/reconcile.go:98-111`) are extended additively by storing triage JSON in `Candidate.Output` (`internal/reconcile/reconcile.go:105`, `internal/ledger/schema.go:163`). The advisory agent runs fail-open, records its prepared resolution commit SHA inside `Candidate.Output` JSON without populating `Candidate.CandidateSHA` (`internal/reconcile/reconcile.go:107`), and leaves status as `candidate_running` (`internal/reconcile/reconcile.go:60-64`, `internal/reconcile/reconcile.go:463-470`). An operator reviews triage output and registers the resolution via `reconcile resolve --candidate --sha` (`cmd/lucind-ai/cli.go:56`) from the primary root (`internal/worktree/worktree.go:278-292`), transitioning the candidate to `integrated` (`internal/reconcile/reconcile.go:848-908`).

## Flow and Invariants

```
[Fixture Generator] ──(1) Leased Features + 3-Hunk Toy File──> [evaluateOverlapGate]
                                                                        │
                                                         (2) ClassRequired (Evidence + Request)
                                                                        ▼
[reconcile approve] ──(3) Running Candidate ─────────────────> [conflict-triage Agent]
                                                                        │
                                                         (4) Triage JSON in Candidate.Output
                                                                        ▼
[reconcile resolve] ──(5) Integrated Candidate with SHA ─────> [evaluateOverlapGate (Retry)]
                                                                        │
                                                         (6) Matched SHA + CAS Promotion
                                                                        ▼
                                                            [PromoteCAS / Target Parent]
```

1. **Fixture Generation → Gate Evaluation**:
   - *Data*: Generator creates two active features (`internal/feature/feature.go:118-124`) with valid parent refs (`internal/feature/feature.go:101-113`), disjoint `allowed_paths` (`internal/packet/disjoint.go:13-22`), and a 3-hunk toy file (1 business, 2 mechanical).
   - *Invariant*: Both features must share an identical non-empty `base_sha`.
   - *Observable Break*: Missing or divergent `base_sha` yields `overlap.ErrNoMergeBase`, causing `evaluateOverlapGate` to `continue` (`internal/run/attempt.go:743-747`), skipping `ClassRequired`.

2. **Gate Evaluation → Request Creation**:
   - *Data*: Gate calls `overlap.Classify` with `DefaultThresholds` (`internal/overlap/overlap.go:93-98`, `internal/overlap/overlap.go:623-660`), detecting intersecting hunks and rename/delete collisions (`internal/overlap/overlap.go:634-650`).
   - *Invariant*: Returns `ClassRequired` (`internal/overlap/overlap.go:659`), calls `CreateRequest` (`internal/reconcile/reconcile.go:213-335`), persists `overlap_evidence` (`internal/ledger/schema.go:131-139`, `internal/reconcile/reconcile.go:266`) and `reconciliation_requests` (`internal/ledger/schema.go:141-154`, `internal/reconcile/reconcile.go:280-305`), and blocks attempt (`internal/run/attempt.go:777-855`, `internal/run/attempt.go:848-854`).
   - *Observable Break*: Non-required classification fails to block attempt or create request.

3. **Direction Approval → Candidate Creation**:
   - *Data*: Operator runs `lucind-ai reconcile approve` (`cmd/lucind-ai/cli.go:56`), calling `Service.Approve` (`internal/reconcile/reconcile.go:406-535`).
   - *Invariant*: Transitions request to `approved` and inserts candidate with `status = 'candidate_running'` and empty `candidate_sha` (`internal/reconcile/reconcile.go:496-504`).
   - *Observable Break*: Expired request or missing actor aborts approval.

4. **Triage Execution → Candidate Output**:
   - *Data*: Advisory agent marks business hunk `ARBITRARY` with `high` risk, assigns verify budget (`~4 min: <cmd>`), resolves mechanical hunks, validates worktree via `ScanConflictMarkers` (`internal/resolve/candidate.go:48-95`) and `EnforceAllowedPaths` (`internal/resolve/candidate.go:100-145`), and writes JSON to `Candidate.Output` (`internal/reconcile/reconcile.go:105`).
   - *Invariant*: Triage runs fail-open without `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`, `internal/resolve/candidate.go:303-312`), leaves proposed SHA in output JSON, and never writes `Candidate.CandidateSHA` directly.
   - *Observable Break*: Fail-closed abort drops advisory output; direct `CandidateSHA` write bypasses operator review.

5. **Human Review → Candidate Resolution**:
   - *Data*: Operator reviews triage JSON and registers resolution via `lucind-ai reconcile resolve --candidate <id> --sha <sha>` (`cmd/lucind-ai/cli.go:56`).
   - *Invariant*: Executed from primary root (`internal/worktree/worktree.go:278-292`), transitioning candidate to `integrated` with non-empty `CandidateSHA` (`internal/reconcile/reconcile.go:848-908`).
   - *Observable Break*: Linked worktree invocation rejected; missing resolution leaves candidate unresolved.

6. **Retry Dispatch → CAS Promotion**:
   - *Data*: Feature retry dispatches via `lucind-ai run` (`cmd/lucind-ai/cli.go:56`). Gate compares other tip with `TargetSHA` (`internal/run/attempt.go:821-828`).
   - *Invariant*: When other tip matches, gate adopts `cand.CandidateSHA` (`internal/run/attempt.go:870`) and executes `PromoteCASWithRunner` (`internal/integrate/integrate.go:151-173`).
   - *Observable Break*: Tip drift (TOCTOU) fails match check and re-blocks attempt (`internal/run/attempt.go:821-828`, `internal/run/attempt.go:848-854`).

7. **Dual-Judge A/B Evaluation**:
   - *Data*: Offline rubric evaluates fixture against `claude`/`claude-opus-5` (`internal/executor/claude.go:35`) and `opencode`/`openai/gpt-5.6-sol` (`internal/executor/opencode.go:51-53`) via registered executors (`cmd/lucind-ai/cli.go:65-70`).
   - *Invariant*: Rubric enforces 3-hunk separation and `high` business risk; stream decoding degrades cleanly (`internal/executor/claude.go:106-122`, `internal/executor/claude_stream.go:15-16`); uniform scoring fails.
   - *Observable Break*: Indistinguishable business vs mechanical scoring fails evaluation.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `conflicttriage.TriagePayload` | Not present (`internal/reconcile/reconcile.go:105`) | Triage JSON payload (`cause_summary`, `hunk_decisions`, `risk_band`, `verify_budget`, `proposed_sha`) in `Candidate.Output` | Yes: text payload in existing column |
| `conflicttriage.HunkDecision` | Not present (`internal/overlap/overlap.go:102-107`) | Per-hunk classification struct (`hunk_id`, `type`, `resolution`, `rationale`) | Yes: internal to new package |
| `conflicttriage.RiskBand` | Not present (`internal/overlap/overlap.go:86-90`) | 3-band risk enum (`low`, `medium`, `high`) | Yes: additive internal type |
| `conflicttriage.VerifyBudget` | Not present (`internal/run/integrate_feature.go:26-78`) | Verification budget struct (wall-clock duration, command) | Yes: additive internal type |
| `reconciliation_candidates.output` | `internal/ledger/schema.go:163` | Stores serialized `TriagePayload` JSON | Yes: column already exists in schema |
| `reconcile.Candidate.Output` | `internal/reconcile/reconcile.go:105` | Holds structured triage JSON string | Yes: existing Go struct field |
| `reconcile.Candidate.CandidateSHA` | `internal/reconcile/reconcile.go:107` | Retains human-registered resolution SHA | Yes: existing resolution contract preserved |
| `fixture.Generator` / `fixture.Config` | Not present (`internal/feature/feature.go:118-124`) | Programmatic generator for two leased features with shared `base_sha` and 3-hunk toy file | Yes: internal test fixture package |
| `fixture.Rubric` / `fixture.EvaluationResult` | Not present (`cmd/lucind-ai/cli.go:65-70`) | Harness scoring judge outputs across `claude` and `opencode` | Yes: internal offline evaluation utility |
| CLI commands and flags | `cmd/lucind-ai/cli.go:56` | Unchanged; uses existing `reconcile approve` and `reconcile resolve` | Yes: no public CLI surface modified |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/conflicttriage/types.go` | Create | Defines `TriagePayload`, `HunkDecision`, `RiskBand`, and `VerifyBudget` structures | `internal/reconcile/reconcile.go:105` (`Candidate.Output` unmarshaling) |
| `internal/conflicttriage/triage.go` | Create | Advisory triage logic, fail-open prompts, and post-triage invariant checks | `cmd/lucind-ai/cli.go:56` (`reconcile resolve` operator review flow) |
| `internal/conflicttriage/fixture/fixture.go` | Create | Generates two active features with shared `base_sha` and 3-hunk toy file | `internal/run/attempt.go:687` (`evaluateOverlapGate` triggering `ClassRequired`) |
| `internal/conflicttriage/fixture/rubric.go` | Create | Offline A/B rubric grading 3-hunk separation and risk ratcheting | `internal/executor/claude.go:35` and `internal/executor/opencode.go:51-53` (offline A/B runner) |
| `internal/conflicttriage/fixture/packets/` | Create | Test packet definitions with disjoint `allowed_paths` (`internal/packet/disjoint.go:29-48`) | `internal/run/integrate_feature.go:17`, `internal/run/integrate_feature.go:26-78` (`FeatureTarget` batch admission) |
| `openspec/changes/conflict-triage-fixture/design-lens-b.md` | Create | Surface & Flow design document | Design synthesizer lane producing `design.md` |

## Open Questions

- [ ] Exact non-decreasing risk formula and thresholds for mixed business and mechanical hunks (intentionally kept open pending fixture calibration data).
- [ ] Production runtime executor/model selection for automated triage dispatches (offline A/B evaluates `claude`/`claude-opus-5` vs `opencode`/`openai/gpt-5.6-sol`).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | CLI usage string defines `reconcile approve` and `reconcile resolve` commands. |
| `cmd/lucind-ai/cli.go:65-70` | Supported executor registry maps executor keys to constructors including `claude` and `opencode`. |
| `internal/executor/claude.go:35` | `Claude.DefaultModel` returns `claude-opus-5` for Claude executor dispatches. |
| `internal/executor/claude.go:106-122` | `Claude.Run` implements fallback handling when streaming progress decoding degrades. |
| `internal/executor/claude_stream.go:15-16` | `claudeStreamDegradedMessage` defines the warning message emitted on stream decode degradation. |
| `internal/executor/opencode.go:51-53` | `Opencode.DefaultModel` returns `openai/gpt-5.6-sol` for Opencode executor dispatches. |
| `internal/feature/feature.go:101-113` | `ValidateParentRef` enforces that feature parent refs are non-empty and not `main` or `lucind/*`. |
| `internal/feature/feature.go:118-124` | `Service.Create` initializes active features and requires a non-empty `baseSHA`. |
| `internal/integrate/integrate.go:151-173` | `PromoteCASWithRunner` executes atomic compare-and-swap promotion on the parent ref. |
| `internal/ledger/schema.go:131-139` | Ledger schema defines `overlap_evidence` table for recording overlap evaluations. |
| `internal/ledger/schema.go:141-154` | Ledger schema defines `reconciliation_requests` table and allowed request statuses. |
| `internal/ledger/schema.go:156-169` | Ledger schema defines `reconciliation_candidates` table with `output` and `candidate_sha` columns. |
| `internal/ledger/schema.go:163` | `reconciliation_candidates.output` column is defined as `TEXT NOT NULL DEFAULT ''`. |
| `internal/overlap/overlap.go:86-90` | `Thresholds` struct defines configurable numeric thresholds for overlap classification. |
| `internal/overlap/overlap.go:93-98` | `DefaultThresholds` defines default classification thresholds including required hotspot weight 0.50. |
| `internal/overlap/overlap.go:102-107` | `Hunk` struct defines line ranges affected in base and new files. |
| `internal/overlap/overlap.go:623-660` | `Classify` evaluates deterministic diff signals against thresholds and returns classification class. |
| `internal/overlap/overlap.go:634-650` | `Classify` triggers `ClassRequired` on rename/delete collisions, shared binaries, and intersecting hunks. |
| `internal/overlap/overlap.go:659` | `Classify` returns `ClassRequired` when required rationales are present. |
| `internal/packet/disjoint.go:13-22` | `PathInScope` tests path prefix containment using component-boundary matching. |
| `internal/packet/disjoint.go:29-48` | `DisjointAllowedPaths` validates pairwise path disjointness across packet allowed paths. |
| `internal/reconcile/reconcile.go:60-64` | `CandidateStatus` enum defines candidate lifecycle statuses including `candidate_running` and `integrated`. |
| `internal/reconcile/reconcile.go:98-111` | `Candidate` struct represents reconciliation candidate state in Go. |
| `internal/reconcile/reconcile.go:105` | `Candidate.Output` string field stores freeform candidate execution output. |
| `internal/reconcile/reconcile.go:107` | `Candidate.CandidateSHA` field holds the human-registered resolution commit SHA. |
| `internal/reconcile/reconcile.go:213-335` | `Service.CreateRequest` validates required overlap and persists request and evidence records. |
| `internal/reconcile/reconcile.go:266` | `Service.CreateRequest` inserts evidence into `overlap_evidence` for required overlaps. |
| `internal/reconcile/reconcile.go:280-305` | `Service.CreateRequest` inserts an `awaiting` row into `reconciliation_requests`. |
| `internal/reconcile/reconcile.go:406-535` | `Service.Approve` transitions request to approved and inserts initial candidate record. |
| `internal/reconcile/reconcile.go:463-470` | `Service.Approve` initializes candidate with status `candidate_running` and empty `candidate_sha`. |
| `internal/reconcile/reconcile.go:496-504` | `Service.Approve` executes atomic SQL insert of new candidate record. |
| `internal/reconcile/reconcile.go:848-908` | `Service.UpdateCandidateStatus` transitions candidate to terminal status with candidate SHA. |
| `internal/resolve/candidate.go:26` | `ErrSemanticAmbiguity` sentinel error indicates unresolved semantic ambiguity in resolver. |
| `internal/resolve/candidate.go:48-95` | `ScanConflictMarkers` scans directory files for unresolved git conflict markers. |
| `internal/resolve/candidate.go:100-145` | `EnforceAllowedPaths` validates 4-way diff union against declared allowed paths. |
| `internal/resolve/candidate.go:303-312` | Candidate resolution prompt instructs resolver to fail closed on semantic ambiguity. |
| `internal/run/attempt.go:687` | `evaluateOverlapGate` evaluates cross-feature overlap signals for active feature attempts. |
| `internal/run/attempt.go:743-747` | `evaluateOverlapGate` continues without blocking when `overlap.Evaluate` returns `ErrNoMergeBase`. |
| `internal/run/attempt.go:777-855` | `evaluateOverlapGate` handles `ClassRequired` overlap by creating requests and blocking attempt. |
| `internal/run/attempt.go:821-828` | `evaluateOverlapGate` checks if other feature's tip matches registered `TargetSHA` before adopting resolution. |
| `internal/run/attempt.go:848-854` | `evaluateOverlapGate` sets attempt status to `AttemptStatusBlocked` and releases feature lease. |
| `internal/run/attempt.go:870` | `evaluateOverlapGate` adopts `resolvedOverrideSHA` onto `CandidateSHA` for retry promotion. |
| `internal/run/integrate_feature.go:17` | `ErrMixedFeatureTargets` indicates batch packets specify conflicting feature targets. |
| `internal/run/integrate_feature.go:26-78` | `FeatureTarget` resolves single integration target and enforces non-main parent ref rules. |
| `internal/worktree/worktree.go:278-292` | `IsLinkedWorktree` determines if a path is a linked git worktree via `.git` file inspection. |
