# Proposal: Conflict Triage Fixture

**Chosen candidate: Candidate 1 — two path-disjoint features + dual-judge rubric** (`openspec/changes/conflict-triage-fixture/explore.md:17-23`).

## Intent

Reconciliation is production-wired and has never fired in this clone: 36 `integration_attempts`, 0 `overlap_evidence`, 0 `reconciliation_requests`, 0 `reconciliation_candidates`. Two git branches that conflict never enter this path; leased features sharing a registered `base_sha` are mandatory. `evaluateOverlapGate` (`internal/run/attempt.go:687`) classifies via `overlap.Classify` (`internal/overlap/overlap.go:623`). `ClassRequired` (`:658-659`) creates an awaiting request (`internal/reconcile/reconcile.go:213-336`); evidence is inserted inside `CreateRequest` (`:266`), not on the warning branch. The attempt blocks and the lease is released (`internal/run/attempt.go:848-855`).

Operators need an on-demand `ClassRequired` they can approve, resolve, and retry against the real CLI (`cmd/lucind-ai/cli.go:56`). Humans need an advisory `conflict-triage` agent that explains cause, leaves a prepared commit, and states what accepting it unverified risks versus what verifying costs — without fail-closed `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26,303-312`).

## Scope

### In Scope
- Feature `conflict-triage-agent` (`parent_ref: feature/conflict-triage-agent`) owning `internal/conflicttriage/**`: advisory agent, JSON payload in `reconciliation_candidates.output` (`internal/reconcile/reconcile.go:105`).
- Feature `conflict-fixture` (`parent_ref: feature/conflict-fixture`) owning `internal/conflicttriage/fixture/`, judge packets, and rubric: generator that creates two leased features with one shared `base_sha` (`internal/feature/feature.go:123-124`) and a 3-hunk toy file.
- Dual-judge A/B of the same fixture on `claude`/`claude-opus-5` (`internal/executor/claude.go:35`) vs `opencode`/`openai/gpt-5.6-sol` (`internal/executor/opencode.go:53-54`). Win criterion: correct classification of the three hunks (business separated from two mechanical controls, arbitrariness declared where it belongs).

### Out of Scope
- Overlap `DefaultThresholds` (`internal/overlap/overlap.go:93-98`).
- Reconcile POST on the web surface (read-only GET remains).
- CAS (`internal/integrate/integrate.go:151-173`) and batch dispatch (`internal/run/integrate_feature.go:17,41`).
- Relaxing fail-closed `internal/resolve`.
- N-way (>2) reconciliation (`internal/run/attempt.go:873-891`).
- Unifying `conflict-triage` with `resolve`.
- Colliding the two *build* features with each other.
- Public CLI `fixture generate` / `reconcile triage` (`cmd/lucind-ai/cli.go:56`).
- Grading the prepared resolution, or timing a human, as A/B win criteria.

## Capabilities

### New Capabilities
- `conflict-triage`: advisory agent under `internal/conflicttriage/`; fail-open; 3-band risk; wall-clock verify budget; JSON in `Candidate.Output`.
- `conflict-fixture`: generator at `internal/conflicttriage/fixture/` that forces `ClassRequired` on a 3-hunk toy file between two real features.
- `triage-evaluation-rubric`: offline A/B rubric for the two pinned judges; a judge that scores all three hunks alike fails.

### Modified Capabilities
- `reconciliation-approval`: candidate `output` carries triage JSON. Approve → resolve → retry CAS is unchanged. Fail-closed resolver requirements are **not** relaxed.

## Approach

Two separate `lucind-ai run` dispatches. Mixed feature targets in one batch are refused (`internal/run/integrate_feature.go:17,41`). Packet `allowed_paths` must be prefix-disjoint (`internal/packet/disjoint.go:29-48`). `parent_ref` must not be `main`, `lucind/*`, or empty (`internal/feature/feature.go:101-113`; `internal/run/integrate_feature.go:73`).

The fixture registers one `base_sha` on both features. Without it, `Evaluate` returns `overlap.ErrNoMergeBase` and the gate `continue`s (`internal/run/attempt.go:743-747`). The toy file has three hunks: one BUSINESS (both sides compile and pass their own tests; no technical criterion to choose) plus two mechanical controls (slice-literal union; rename colliding with an edit to the old name). `Classify` already treats intersecting hunks and rename/delete as required (`internal/overlap/overlap.go:634-650`).

Triage does **not** fail closed. It labels the business hunk ARBITRARY, pins risk to `high` (agent cannot lower it), proposes a commit, and states verify cost as wall clock plus the concrete command (e.g. `~4 min: ./lucind-checks.sh`). Mechanical hunks get a deterministic proposal. Post-triage reuse `ScanConflictMarkers` and `EnforceAllowedPaths` (`internal/resolve/candidate.go:48-95,100-145`) as invariant checks only — not prompt or fail-closed policy.

Clearing a block is two steps: `reconcile approve` authorizes direction (`internal/reconcile/reconcile.go:406-535`); a human resolves out of band and registers the SHA with `reconcile resolve --candidate --sha`. Retry adopts `CandidateSHA` only if the other tip is unchanged (`internal/run/attempt.go:821-828,870`) and promotes via CAS (`internal/integrate/integrate.go:151-173`). Invoke CLI mutate verbs from the primary root (`internal/worktree/worktree.go:278-292`). Freeze the other feature's tip until CAS.

Rejected: Candidate 2 (in-memory harness + sync LLM in the gate) — `ErrNoMergeBase` skip plus promotion coupled to LLM latency. Candidate 3 (public CLI) — premature surface before prompts and the risk formula stabilize. Continuous 0–100 risk, token/pricing verify units, and fail-closed triage are rejected product decisions.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/conflicttriage/` | New | Advisory agent, payload, fail-open invoker. |
| `internal/conflicttriage/fixture/` | New | 3-hunk generator; shared `base_sha`; judge packets; rubric. |
| `internal/reconcile/reconcile.go:105` | Additive payload | JSON in existing `output` column (`internal/ledger/schema.go:163`). |
| `cmd/lucind-ai/cli.go:65-70` | Unchanged | Judges use registered executors; no new CLI verbs. |
| `internal/run/attempt.go`, `internal/overlap/`, `internal/integrate/` | Unchanged | Gate, thresholds, and CAS are exercised, not rewritten. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Reusing `resolve` prompts inherits fail-closed | High | Isolate templates/invoker; never return `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26,303-312`). |
| Residual markers or edits outside `allowed_paths` | High | `ScanConflictMarkers`, `EnforceAllowedPaths` (`internal/resolve/candidate.go:48-95,100-145`). |
| Missing shared `base_sha` skips required | High | Fixture registers one `base_sha` on both features (`internal/run/attempt.go:743-747`). |
| Other tip moves after triage (TOCTOU) | Med | Freeze target tip until CAS (`internal/run/attempt.go:821-828`). |
| `reconcile resolve` from a linked worktree | Med | Invoke from primary root (`internal/worktree/worktree.go:278-292`). |
| Mixed-feature fixture dispatch | Low | Separate `lucind-ai run` invocations (`internal/run/integrate_feature.go:17,41`). |
| Claude `stream-json` decode during A/B | Low | Fallback already in `Claude.Run` (`internal/executor/claude.go:106-122`; `internal/executor/claude_stream.go:10-16`). |

## Rollback Plan

`git revert` of commits that add `internal/conflicttriage/` (including `fixture/`) and judge packets. No schema migration: `output` already exists (`internal/ledger/schema.go:163`). Gate, CAS, CLI verbs, and web GET routes stay untouched, so revert restores prior behavior with no ledger un-migration.

## Delta Specifications

### Requirement: Deterministic three-hunk fixture

The generator at `internal/conflicttriage/fixture/` MUST create two leased features sharing one `base_sha` that yields `ClassRequired` (`internal/overlap/overlap.go:623-659`) during `evaluateOverlapGate`. The conflicting file MUST contain exactly three hunks: one business conflict (both sides compile and pass their own tests) and two mechanical controls (slice-literal union; rename vs edit). The two *build* features MUST NOT collide (`internal/packet/disjoint.go:29-48`; `internal/run/integrate_feature.go:17,41`).

#### Scenario: Fixture forces ClassRequired
- GIVEN two active features share a registered `base_sha` and edit the toy file
- WHEN `evaluateOverlapGate` calls `Evaluate` (`internal/run/attempt.go:743`)
- THEN classification SHALL be `ClassRequired`, `CreateRequest` SHALL persist `overlap_evidence` (`internal/reconcile/reconcile.go:266`) and an awaiting `reconciliation_requests` row (`:280-300`), and the attempt SHALL block (`internal/run/attempt.go:848-855`).

#### Scenario: Missing shared base SHA bypasses classification
- GIVEN features with missing or divergent `base_sha`
- WHEN the gate evaluates overlap
- THEN `ErrNoMergeBase` SHALL cause `continue` with no `ClassRequired` (`internal/run/attempt.go:743-747`).

### Requirement: Semantic triage and risk ratchet

`conflict-triage` MUST explain cause, resolve mechanical hunks deterministically, and MUST NOT fail closed (`internal/resolve/candidate.go:26,303-312`). For a business choice with no technical criterion, it MUST flag ARBITRARY, record why that side was picked, pin risk to `high` (cannot lower it), leave a prepared SHA (`internal/reconcile/reconcile.go:107`), and state verify cost as wall clock plus command. JSON SHALL be stored in `Candidate.Output` (`:105`).

#### Scenario: Business hunk ratchets high
- GIVEN an awaiting request with the 3-hunk fixture
- WHEN triage runs
- THEN it MUST mark the business hunk ARBITRARY, pin risk `high`, declare verify budget as wall clock plus command, and write JSON to `output`.

#### Scenario: Mechanical hunks resolved deterministically
- GIVEN slice-union and rename-vs-edit controls
- WHEN triage runs
- THEN it MUST propose deterministic resolutions without `ErrSemanticAmbiguity`.

### Requirement: Two-step close and retry CAS

Clearing `ClassRequired` MUST take `reconcile approve` (`internal/reconcile/reconcile.go:406-535`) then `reconcile resolve --candidate --sha` from the primary root (`internal/worktree/worktree.go:278-292`). Retry MUST adopt `CandidateSHA` only when the other tip is unchanged (`internal/run/attempt.go:821-828,870`) and promote via CAS (`internal/integrate/integrate.go:151-173`).

#### Scenario: Valid candidate promotes on retry
- GIVEN an approved request whose registered SHA still matches the other tip
- WHEN the blocked feature is re-dispatched
- THEN the gate SHALL adopt `CandidateSHA` (`internal/run/attempt.go:870`) and CAS-promote.

#### Scenario: Tip drift re-blocks
- GIVEN the other feature's tip has moved
- WHEN retry runs
- THEN the gate MUST NOT adopt the stale SHA and MUST block (`internal/run/attempt.go:821-828,848-854`).

### Requirement: Dual-judge rubric isolation

The rubric MUST run the same fixture on registered executors (`cmd/lucind-ai/cli.go:65-70`) without cross-provider config or billing leaks (`internal/executor/claude.go:35-49`; `internal/executor/opencode.go:53-59`). Win criterion: separate the business hunk from the two mechanical controls and declare ARBITRARY on the business hunk. A judge that scores all three hunks alike SHALL fail.

#### Scenario: A/B on pinned judges
- GIVEN identical fixture evidence on `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol`
- WHEN the rubric grades
- THEN it SHALL score cause quality, `high` on the business hunk, and distinct mechanical classification, with no cross-executor leaks.

#### Scenario: Uniform hunk scoring fails
- GIVEN a judge that assigns the same class or risk to all three hunks
- WHEN the rubric runs
- THEN it MUST reject that evaluation.

## Test and Validation Impact

| Layer | Coverage |
|-------|----------|
| Unit: triage | Fail-open on semantic ambiguity; ARBITRARY + `high` on business; verify-budget `~N min: <cmd>`; never `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26,303-312`). |
| Unit: invariants | `ScanConflictMarkers` and `EnforceAllowedPaths` (`internal/resolve/candidate.go:48-95,100-145`; `internal/resolve/candidate_test.go:16-49,51-80`). |
| Unit: fixture | 3-hunk shape; `Classify` → `ClassRequired` under `DefaultThresholds` (`internal/overlap/overlap.go:93-98,623-659`). |
| Integration: gate | `CreateRequest` evidence + awaiting row; `AttemptStatusBlocked`; `ErrNoMergeBase` continue (`internal/run/attempt.go:743-747,777-855`; `internal/reconcile/reconcile.go:266-300`; `internal/reconcile/reconcile_test.go:52-100`). |
| Integration: CLI/CAS | `reconcile approve` / `resolve` / retry CAS; linked-worktree rejection (`cmd/lucind-ai/cli.go:56`; `cmd/lucind-ai/cli_test.go:2944,3126`; `internal/run/attempt.go:821-828,870`). |
| Qualitative: rubric | A/B on pinned judges; 3-hunk separation; stream decoder stability (`internal/executor/claude.go:106-122`). |

## Open Questions

These stay open until the fixture produces data. Do not guess in design.

1. Exact non-decreasing risk formula and thresholds, including mixed business+mechanical hunks.
2. Which executor/model runs **production** triage (judges are `opencode`/`openai/gpt-5.6-sol` and `claude`/`claude-opus-5`; production runtime is a separate decision).

## Accepted Deviations

Recorded 2026-08-24, after apply and before verify. These are decisions, not defects: verify must
not report them as spec violations.

### 1. Delivered as one feature, not two

**The proposal specified** two path-disjoint features — `conflict-triage-agent`
(`parent_ref: feature/conflict-triage-agent`, owning `internal/conflicttriage/**`) and
`conflict-fixture` (`parent_ref: feature/conflict-fixture`, owning `internal/conflicttriage/fixture/`,
the judge packets, and the rubric) — promoted through two separate `lucind-ai run` dispatches, per
`## Approach`.

**What was delivered** is one change on a single feature, `conflict-triage-fixture`. Every planning
phase and the apply itself dispatched in legacy mode (`--legacy-main --expected-parent-sha`), which
branches each lane from the current HEAD and ff-merges the batch back into the primary checkout —
so all of it landed on `dev`, interleaved with an unrelated concurrent change.

**Why it is accepted.** The two-feature split existed to exercise the multi-feature admission
machinery, not because the product needs the boundary: nothing in `conflict-triage`,
`conflict-fixture`, or `triage-evaluation-rubric` depends on being promoted separately. The single
`internal/conflicttriage/` tree with a `fixture/` subpackage is the same code either way, and
`tasks.md` enumerates `allowed_paths` as files precisely so the work units stay prefix-disjoint
inside one tree.

**What it costs, stated plainly.** This change no longer exercises what the split was for: the
mixed-target refusal (`internal/run/integrate_feature.go:17,41`), cross-feature prefix-disjoint
`allowed_paths`, per-feature leases, or CAS promotion onto a feature parent. That machinery remains
unverified by this change's own delivery and is now owed a separate exercise.

**What is unaffected.** The deliberate `ClassRequired` collision is produced by the fixture between
two features *it* registers at runtime (`internal/conflicttriage/fixture/fixture.go`). It never
depended on how this change itself was delivered, and the fixture's own two-feature requirement
stands unchanged.

### 2. Verify runs on a real feature target

Verify is the first phase dispatched against a registered feature
(`parent_ref: feature/conflict-triage-fixture`) rather than in legacy mode, so its lanes promote by
compare-and-swap on the feature parent instead of merging into the primary checkout. This is a
partial exercise only: judgment lanes are `read_only: true` and carry no commits to promote.
