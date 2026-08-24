# Proposal Lens B — Capability Impact & Specs: Conflict Triage Fixture

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `conflict-fixture` | Added | Deterministic test fixture generating reproducible `ClassRequired` overlap conflicts across three hunks (one business, two mechanical controls) between active features with a shared `base_sha`. | `internal/overlap/overlap.go:622-660`, `internal/run/attempt.go:687-752`, `internal/reconcile/reconcile.go:213-295` |
| `conflict-triage` | Added | Conflict triage agent explaining causes, preparing candidate commits without fail-closed aborts, flagging arbitrary business choices with high risk, and providing verification budgets. | `internal/reconcile/reconcile.go:98-111`, `internal/resolve/candidate.go:26,303-326`, `internal/ledger/schema.go:156-169` |
| `reconciliation-approval` | Modified | Reconciliation lifecycle exercised via fixture and triage candidate output, binding approved requests to triage resolution artifacts for retry CAS promotion. | `cmd/lucind-ai/cli.go:1090-1195,1398-1463`, `internal/reconcile/reconcile.go:406-535,848-910`, `internal/run/attempt.go:821-871`, `internal/integrate/integrate.go:151-173` |
| `triage-evaluation-rubric` | Added | Multi-executor evaluation rubric benchmarking triage behavior across pinned executors (`claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol`) without cross-provider leaks, scoring classification and risk. | `cmd/lucind-ai/cli.go:65-70`, `internal/executor/claude.go:33-46`, `internal/executor/opencode.go:51-60`, `internal/executor/agy.go:85-95`, `internal/executor/cursor_agent.go:35-46` |

## Delta Specifications

### Requirement: Deterministic Three-Hunk Conflict Fixture Generation

The fixture generator at `internal/conflicttriage/fixture/` MUST generate two feature branches sharing a valid `base_sha` that triggers `overlap.ClassRequired` during `evaluateOverlapGate` (`internal/run/attempt.go:687-752`, `internal/overlap/overlap.go:622-660`). The generated conflicting file MUST contain exactly three hunks: one business conflict where both versions compile and pass tests, and two mechanical controls (slice-literal union and rename colliding with an edit). The two build features MUST NOT collide with each other (`internal/run/integrate_feature.go:17,41`, `internal/packet/disjoint.go:29-48`).

#### Scenario: Fixture forces ClassRequired overlap

- GIVEN two active features share a registered `base_sha` and edit the fixture file
- WHEN `evaluateOverlapGate` invokes `overlap.Evaluate` (`internal/run/attempt.go:743`)
- THEN classification SHALL yield `ClassRequired` (`internal/overlap/overlap.go:627-660`), persist evidence in `overlap_evidence` via `CreateRequest` (`internal/reconcile/reconcile.go:266`), create an awaiting request in `reconciliation_requests` (`internal/reconcile/reconcile.go:280-300`), and block feature promotion (`internal/run/attempt.go:848-856`).

#### Scenario: Missing shared base SHA bypasses classification

- GIVEN two features have divergent or missing `base_sha` values
- WHEN `evaluateOverlapGate` evaluates overlap
- THEN evaluation MUST return `ErrNoMergeBase` and continue without recording `ClassRequired` (`internal/run/attempt.go:743-747`).

### Requirement: Semantic Conflict Triage and Risk Ratcheting

The `conflict-triage` agent under `internal/conflicttriage/` MUST analyze conflict markers and explain architectural causes rather than diffs. Triage MUST resolve mechanical hunks deterministically and MUST NOT fail closed on semantic ambiguity (`internal/resolve/candidate.go:26,303-326`). For a business conflict where choice is arbitrary, the agent MUST flag the decision as ARBITRARY, record why a side was chosen, ratchet risk to `high` across discrete low/medium/high bands, prepare a candidate commit, and state verification cost in wall clock and concrete command (`internal/reconcile/reconcile.go:98-111`).

#### Scenario: Triage classifies business conflict and ratchets risk

- GIVEN an awaiting reconciliation request containing a business conflict and mechanical control hunks
- WHEN `conflict-triage` processes conflict markers
- THEN it MUST provide a prepared resolution commit SHA (`internal/reconcile/reconcile.go:107`), mark the business choice as ARBITRARY, pin risk to `high`, declare verify budget in wall clock plus command, and write JSON output to `reconciliation_candidates.output` (`internal/reconcile/reconcile.go:105,163-165`).

#### Scenario: Mechanical hunks resolved deterministically

- GIVEN conflict markers containing only mechanical hunks (slice-literal union and rename collision)
- WHEN `conflict-triage` executes
- THEN it MUST resolve all markers into a valid merge commit without semantic ambiguity errors and without setting risk to `high`.

### Requirement: Two-Step Reconciliation and Promotion Lifecycle

Clearing a `ClassRequired` block MUST require two distinct operator actions: `lucind-ai reconcile approve` to authorize direction (`cmd/lucind-ai/cli.go:1090-1195`, `internal/reconcile/reconcile.go:406-535`), followed by `lucind-ai reconcile resolve --candidate <id> --sha <sha>` from the primary root to register the candidate SHA (`cmd/lucind-ai/cli.go:1398-1463`, `internal/worktree/worktree.go:278-292`). On batch retry, `evaluateOverlapGate` MUST adopt `cand.CandidateSHA` if the target tip is unchanged (`internal/run/attempt.go:821-828,870`) and promote the feature ref via CAS (`internal/integrate/integrate.go:151-173`).

#### Scenario: Valid candidate resolution promotes via retry

- GIVEN an approved reconciliation request with a registered candidate SHA matching the target feature tip
- WHEN the batch is re-dispatched with `lucind-ai run`
- THEN `evaluateOverlapGate` SHALL adopt `cand.CandidateSHA` (`internal/run/attempt.go:870`) and CAS promote `parent_ref` (`internal/integrate/integrate.go:151-173`).

#### Scenario: Target tip drift blocks retry promotion

- GIVEN an approved reconciliation request whose registered `target_sha` no longer matches the active target feature tip
- WHEN batch integration is retried
- THEN `evaluateOverlapGate` MUST detect the stale target SHA and block promotion (`internal/run/attempt.go:811-828,848-854`).

### Requirement: Multi-Executor Rubric Calibration and Isolation

The evaluation rubric MUST benchmark triage output across registered executors (`agy`, `claude`, `cursor-agent`, `opencode` registered at `cmd/lucind-ai/cli.go:65-70`) without cross-provider configuration or billing leaks (`internal/executor/claude.go:33-46`, `internal/executor/opencode.go:51-60`). The rubric win criterion SHALL require correct separation of the business hunk from the two mechanical controls, with arbitrariness declared where it belongs. A judge evaluating all three hunks identically SHALL be failed.

#### Scenario: Dual-judge A/B benchmark execution

- GIVEN identical 3-hunk conflict fixture evidence dispatched to `claude`/`claude-opus-5` (`internal/executor/claude.go:35`) and `opencode`/`openai/gpt-5.6-sol` (`internal/executor/opencode.go:53`)
- WHEN the evaluation rubric grades triage output
- THEN the rubric SHALL score cause quality, proper `high` risk assignment for the business hunk, and valid classification of mechanical controls without cross-executor state leaks (`cmd/lucind-ai/cli.go:65-70`).

#### Scenario: Uniform hunk scoring fails the judge

- GIVEN a judge evaluation that assigns identical classification or risk to all three hunks
- WHEN rubric validation runs
- THEN the rubric MUST reject the evaluation.

## Open Questions

- [ ] **Exact non-decreasing risk formula and thresholds**: What formula and numeric thresholds govern risk escalation when combining partial business and multiple mechanical hunks? (Deliberately left open pending fixture calibration data).
- [ ] **Production triage runtime executor**: Which executor/model should run production triage once calibrated? (Judges are pinned to `opencode`/`openai/gpt-5.6-sol` and `claude`/`claude-opus-5`; production runtime selection remains open).
- [ ] **Proposal skill vs three-lens fan-out split**: `~/.claude/skills/sdd-propose/SKILL.md` describes a single subagent writing a complete `proposal.md` with Engram persistence, whereas this packet executes a 3-way fan-out (lenses A, B, C) targeting `propose-lens-b.md` as feedstock for synthesis.
