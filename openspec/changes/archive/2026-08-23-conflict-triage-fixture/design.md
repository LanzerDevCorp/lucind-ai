# Design: Conflict Triage Fixture

## Technical Approach

Add an advisory, fail-open triage agent at `internal/conflicttriage/` and a 3-hunk fixture generator at `internal/conflicttriage/fixture/`. The generator creates two leased features that share one `base_sha` (`internal/feature/feature.go:123-124`) and edit one toy file so `evaluateOverlapGate` (`internal/run/attempt.go:687`) reaches `ClassRequired` (`internal/overlap/overlap.go:623-659`). Triage JSON lands in the existing `Candidate.Output` column (`internal/reconcile/reconcile.go:105`; `internal/ledger/schema.go:163`). A human registers the resolution SHA with `lucind-ai reconcile resolve` (`cmd/lucind-ai/cli.go:1445-1511`), which is the only writer of `Candidate.CandidateSHA` (`internal/reconcile/reconcile.go:107`). Gate, CAS, overlap thresholds, and CLI verbs stay unchanged. Offline A/B uses registered executors (`cmd/lucind-ai/cli.go:65-70`).

## Architecture Decisions

### Decision: Split Output from CandidateSHA

**Choice**: The agent writes cause, hunk decisions, 3-band risk, wall-clock verify budget, and `proposed_sha` into `Candidate.Output`. Only `runReconcileResolve` writes `CandidateSHA` via `UpdateCandidateStatus(..., CandidateStatusIntegrated, resolvedSHA, "")` (`cmd/lucind-ai/cli.go:1501-1506`; `internal/reconcile/reconcile.go:848-908`). Status stays `candidate_running` (`:60-64`, `:463-470`) until that call. Persist Output through a new output-only Service update; `UpdateCandidateStatus` SQL does not touch `output` (`:873-876`). `ledger.UpdateReconciliationCandidate` already can set `output` (`internal/ledger/ledger.go:1314-1338`).
**Alternatives considered**: Agent writes `CandidateSHA` and flips `integrated`; a bypass flag on `UpdateCandidateStatus`.
**Rationale**: The gate adopts a non-empty integrated `CandidateSHA` for CAS whenever the other tip still matches (`internal/run/attempt.go:821-828`, `:870`; `internal/integrate/integrate.go:151-173`). Agent writes to `CandidateSHA` would skip human registration and promote unverified code. The proposal's "leave a prepared SHA (`:107`)" vs "human registers the SHA" is resolved here: the prepared SHA is `proposed_sha` in Output; `:107` stays human-owned.
**Terminal consumer**: `runReconcileResolve` and `evaluateOverlapGate`.

### Decision: Packages and no public CLI

**Choice**: Agent in `internal/conflicttriage/`, generator in `internal/conflicttriage/fixture/`. No `fixture generate` or `reconcile triage` on `cmd/lucind-ai/cli.go:56`.
**Alternatives considered**: `test/fixture/`; `internal/overlap/fixture/`; public CLI; placing triage in `internal/resolve/`.
**Rationale**: `internal/overlap` owns `Classify` (`internal/overlap/overlap.go:623-659`), not reconciliation workflow. Public CLI would ship uncalibrated prompts. A `failOpen` flag on `internal/resolve` would threaten fail-closed `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`, prompt `:303-312`).
**Terminal consumer**: fixture → `evaluateOverlapGate` (`internal/run/attempt.go:687`, `:831-845`); judges → `supportedExecutors` (`cmd/lucind-ai/cli.go:65-70`).

### Decision: Fail-open invoker, reuse invariants only

**Choice**: Dedicated invoker and prompts under `internal/conflicttriage/`. After the agent proposes a commit, call `ScanConflictMarkers` (`internal/resolve/candidate.go:48-95`) and `EnforceAllowedPaths` (`:100-145`) as read-only checks. Never return `ErrSemanticAmbiguity`.
**Alternatives considered**: Parameterize `ResolveCandidateMerge`; share resolve prompts.
**Rationale**: Resolve must fail closed on business ambiguity. Triage must flag ARBITRARY, pin risk `high` (cannot lower a business hunk), and still emit JSON.
**Terminal consumer**: new `TriageInvoker` tests; invariant helpers as today (`internal/resolve/candidate_test.go:16-49,51-93`).

### Decision: Sequential disjoint fixture dispatch

**Choice**: Two separate `lucind-ai run` dispatches. Each build feature gets prefix-disjoint `allowed_paths` (`internal/packet/disjoint.go:29-48`) and a valid `parent_ref` (`internal/feature/feature.go:101-113`). Agent `allowed_paths` enumerates package-root files so `PathInScope` (`internal/packet/disjoint.go:13-22`) does not also grant `internal/conflicttriage/fixture/`.
**Alternatives considered**: One multi-packet batch; skip `FeatureTarget`.
**Rationale**: `FeatureTarget` returns `ErrMixedFeatureTargets` on mixed targets (`internal/run/integrate_feature.go:17`, `:26-52`, `:41`). Overlapping prefixes fail `DisjointAllowedPaths`. Sequential dispatch lets both features register the same `base_sha`.
**Terminal consumer**: `FeatureTarget` and `DisjointAllowedPaths`.

### Decision: Offline rubric on registered executors

**Choice**: Same fixture, two judges: `claude`/`claude-opus-5` (`internal/executor/claude.go:35-52`) and `opencode`/`openai/gpt-5.6-sol` (`internal/executor/opencode.go:53-65`) via `supportedExecutors`. Win = separate the business hunk from the two mechanical controls and declare ARBITRARY on business. Uniform scoring fails. Do not grade the prepared commit or time a human.
**Alternatives considered**: Direct HTTP clients; new provider config; grading execution or operator speed.
**Rationale**: Reuses isolated CLI runners; Claude already keeps raw stdout if stream-json degrades (`internal/executor/claude.go:106-122`; `internal/executor/claude_stream.go:10-16`).
**Terminal consumer**: `Claude.Run` / `Opencode.Run`.

## Flow and Invariants

```
Fixture ─► evaluateOverlapGate ─► ClassRequired (evidence + awaiting request)
Approve ─► candidate_running ─► triage agent ─► JSON in Output (proposed_sha only)
resolve --sha (primary root) ─► integrated + CandidateSHA
retry ─► tip match ─► adopt SHA ─► PromoteCASWithRunner
```

1. **Fixture → gate.** Two active features, shared non-empty `base_sha`, 3-hunk toy file (1 business, 2 mechanical). Break: missing/divergent `base_sha` → `ErrNoMergeBase` → `continue` (`internal/run/attempt.go:743-747`).
2. **Gate → request.** `Classify` + `DefaultThresholds` (`internal/overlap/overlap.go:93-98`, `:623-659`); intersecting hunks / rename-delete / shared binary → `ClassRequired` (`:634-650`, `:659`). `CreateRequest` inserts `overlap_evidence` (`internal/ledger/schema.go:131-139`; `internal/reconcile/reconcile.go:266`) and an `awaiting` row (`schema.go:141-154`; `reconcile.go:280-305`) and blocks (`internal/run/attempt.go:777-855`).
3. **Approve → candidate.** `Approve` (`reconcile.go:406-535`) inserts `candidate_running` with empty `candidate_sha` (`:463-470`, `:496-504`).
4. **Triage → Output.** Business hunk ARBITRARY + `high`; mechanical hunks deterministic; verify budget `~N min: <cmd>`; fail-open; never write `CandidateSHA`. Break: fail-closed abort, or SHA write that skips review.
5. **Human resolve.** From primary root (`internal/worktree/worktree.go:278-292`). Linked worktree rejected (`cmd/lucind-ai/cli.go:1478-1481`).
6. **Retry → CAS.** Other tip must still equal `TargetSHA` (`attempt.go:821-828`). Else re-block (`:848-855`). N-way simultaneous resolves stay blocked (`:873-891`).
7. **A/B.** Rubric on pinned judges; uniform hunk scores fail.

## Interfaces / Contracts

JSON in `Candidate.Output` (existing TEXT NOT NULL DEFAULT `''`, not nullable):

```go
type TriagePayload struct {
    CauseSummary  string
    HunkDecisions []HunkDecision // hunk_id, kind, resolution, rationale
    RiskBand      string         // low | medium | high
    VerifyBudget  string         // "~4 min: ./lucind-checks.sh"
    ProposedSHA   string
}
```

CLI, request schema, and `CandidateSHA` contract unchanged (`cmd/lucind-ai/cli.go:56`).

## File Changes

| File | Action | Terminal consumer |
|------|--------|-------------------|
| `internal/conflicttriage/types.go` | Create payload types | `Candidate.Output` unmarshal (`reconcile.go:105`) |
| `internal/conflicttriage/triage.go` | Create fail-open agent + invariant calls | Operator `reconcile resolve` (`cli.go:56`) |
| `internal/conflicttriage/invoker.go` | Create `TriageInvoker` func field | Unit tests (stub LLM) |
| `internal/reconcile/reconcile.go` | Modify: output-only update | Agent persist; not `UpdateCandidateStatus` |
| `internal/conflicttriage/fixture/fixture.go` | Create 3-hunk generator + shared `base_sha` | `evaluateOverlapGate` (`attempt.go:687`) |
| `internal/conflicttriage/fixture/rubric.go` | Create A/B grader | `Claude` / `Opencode` |
| `internal/conflicttriage/fixture/packets/` | Create disjoint judge packets | `FeatureTarget` (`integrate_feature.go:17,26-78`) |

## Testing Strategy and Test Seams

| Layer | What | Seam |
|-------|------|------|
| Unit: agent | Fail-open; ARBITRARY+`high`; `~N min: <cmd>`; no `ErrSemanticAmbiguity` | new `TriageInvoker` |
| Unit: invariants | Markers / out-of-scope diffs | `ScanConflictMarkers`, `EnforceAllowedPaths` (`candidate.go:48-95,100-145`) |
| Unit: fixture | Shared `base_sha`; `Classify` → `ClassRequired` | `DefaultThresholds`, `Classify` (`overlap.go:93-98,623-659`); `feature.Create` (`feature.go:118-133`) |
| Integration: gate | Awaiting row + `AttemptStatusBlocked`; `ErrNoMergeBase` skip | `Deps.EvaluateOverlap` (`internal/run/run.go:211`); `NewService` (`reconcile.go:157-168`); `gate_test.go:122-140` |
| Integration: CLI/CAS | approve → resolve SHA → retry CAS; tip drift re-blocks; linked worktree | `depsFactory` (`cli.go:60`); `PromoteCASWithRunner`; `IsLinkedWorktree`; `cli_test.go:2944,3126` |
| Qualitative: A/B | 3-hunk split; reject uniform scores; stream fallback | `supportedExecutors`; `Claude.Run` |

Existing seams: `WithClock`/`WithIDSource`/`WithOverlapEvaluator` (`reconcile.go:128-145`); `PromoteCASWithRunner` GitRunner; resolve `Invoker` (`internal/resolve/resolve.go:20`) as the analog, not a shared engine. New: `TriageInvoker`; `fixture.GeneratorOptions` (repo root, branch names, base commit).

## Threat Matrix

| Boundary | Applicability | Safe / failure | Planned RED |
|---|---|---|---|
| Documentation-like paths | Applicable | Scan as text; skip NUL binaries (`candidate.go:80`); never exec | `TestScanConflictMarkers_DocumentationAndScriptPaths` |
| Git repository selection | Applicable | Explicit cwd / `git -C`; `reconcile resolve` refuses linked worktrees (`worktree.go:278-292`) | `TestReconcileResolve_RejectsLinkedWorktree`; `TestEnforceAllowedPaths_ExplicitWorktreeCwd` |
| Commit state | Applicable | 4-way union vs `base_sha` (`candidate.go:100-145`) | `TestEnforceAllowedPaths_StagedAndUntrackedDisjoint` |
| Push state | N/A: local `git update-ref` only (`integrate.go:151-173`) | — | None |
| PR commands | N/A: no host PR argv | — | None |

## Rollback and Additivity

**Choice**: `git revert` of commits that add `internal/conflicttriage/` (including `fixture/`) and judge packets.
**Alternatives considered**: Schema teardown; feature flags.
**Rationale**: Additive only. `output` already exists (`schema.go:163`). Gate, CAS, CLI, and read-only GET routes are unmodified. Revert removes the agent with no ledger un-migration. No migration required.

## Open Questions and Out of Scope

Stay open (do not guess):

- [ ] Exact non-decreasing risk formula and thresholds, including mixed business+mechanical hunks.
- [ ] Which executor/model runs **production** triage (judges are pinned; production is separate).

Out of scope: overlap `DefaultThresholds`; web reconcile POST (GET stays); CAS/batch dispatch internals; relaxing `internal/resolve`; N-way merge-of-merges (`attempt.go:873-891`); unifying triage with resolve; colliding the two *build* features; public CLI verbs; grading prepared resolution or human timing as A/B criteria.
