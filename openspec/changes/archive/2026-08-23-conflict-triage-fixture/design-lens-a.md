# Design Lens A — Decisions: Conflict Triage Fixture

## Assumed architecture

This change extends `internal/reconcile` and `internal/ledger` to store structured triage analysis in existing candidate output columns without schema migrations, and introduces new internal packages `internal/conflicttriage` (advisory triage agent, fail-open invoker, 3-band risk ratchet) and `internal/conflicttriage/fixture` (3-hunk conflict generator establishing shared base SHAs). The reconciliation gate in `internal/run/attempt.go`, CAS promotion in `internal/integrate`, classification in `internal/overlap`, and the CLI interface in `cmd/lucind-ai/cli.go` are exercised unchanged. Offline dual-judge evaluation is conducted via existing registered executors (`claude` and `opencode`) using dedicated evaluation packets.

## Technical Approach

We introduce an on-demand conflict fixture generator and an advisory conflict triage agent to safely exercise and unblock the reconciliation subsystem (`internal/reconcile/reconcile.go:98-111` and `internal/reconcile/reconcile.go:266-276`). The fixture generator under `internal/conflicttriage/fixture/` creates two leased features with a shared `base_sha` (`internal/feature/feature.go:123-124`) that edit a 3-hunk toy file, forcing `ClassRequired` during `evaluateOverlapGate` (`internal/run/attempt.go:687-709` and `internal/run/attempt.go:831-845`) and blocking integration (`internal/run/attempt.go:848-855`).

The advisory agent under `internal/conflicttriage/` performs fail-open semantic triage on the awaiting request. It isolates business conflicts from mechanical controls, flags business ambiguity as ARBITRARY with risk pinned to `high`, defines verify cost as wall clock plus command, and records structured JSON in `Candidate.Output` (`internal/reconcile/reconcile.go:105` and `internal/ledger/schema.go:163`). To preserve human-in-the-loop safety, the human operator verifies the proposal and registers the commit SHA via `lucind-ai reconcile resolve` (`cmd/lucind-ai/cli.go:1445-1511`), which sets `Candidate.CandidateSHA` (`internal/reconcile/reconcile.go:107`) and unblocks retry promotion via CAS (`internal/run/attempt.go:821-828`, `internal/run/attempt.go:870`, and `internal/integrate/integrate.go:151-173`). An offline rubric grades judge performance across registered executors (`cmd/lucind-ai/cli.go:65-70`).

## Decision 1 — Split Ownership of Candidate Output and Candidate SHA

**Choice**: The advisory triage agent writes structured analysis and its proposed resolution commit SHA into `Candidate.Output` (`internal/reconcile/reconcile.go:105` and `internal/ledger/schema.go:163`). The human operator retains exclusive write authority over `Candidate.CandidateSHA` (`internal/reconcile/reconcile.go:107`) via `reconcile resolve --candidate <id> --sha <sha>` (`cmd/lucind-ai/cli.go:1445-1511`), transitioning status to `integrated` (`cmd/lucind-ai/cli.go:1501-1506`).
**Alternatives considered**: Allowing the triage agent to write directly to `CandidateSHA` and transition candidate status to `integrated`; adding an automated bypass flag to `UpdateCandidateStatus`.
**Rationale**: In `evaluateOverlapGate` (`internal/run/attempt.go:821-828` and `internal/run/attempt.go:870`), the gate adopts `CandidateSHA` for CAS promotion (`internal/integrate/integrate.go:151-173`) whenever a candidate has status `integrated`. If the triage agent wrote `CandidateSHA` directly, advisory triage would collapse into autonomous unverified code resolution. Storing the proposed commit in `Candidate.Output` allows the human to inspect and audit triage findings out of band before explicitly registering the SHA via CLI.
**Terminal consumer**: `runReconcileResolve` in `cmd/lucind-ai/cli.go:1501-1506` calling `UpdateCandidateStatus` (`internal/reconcile/reconcile.go:848-908`), and `evaluateOverlapGate` in `internal/run/attempt.go:821-828` and `internal/run/attempt.go:870` adopting `CandidateSHA` for promotion.

## Decision 2 — Package Placement and CLI Boundary for Agent and Fixture

**Choice**: House the advisory triage agent in `internal/conflicttriage/` and the fixture generator in `internal/conflicttriage/fixture/`. No public CLI verbs (e.g. `fixture generate` or `reconcile triage`) are added to `cmd/lucind-ai/cli.go:56`.
**Alternatives considered**: Placing the fixture under `test/fixture/` or `internal/overlap/fixture/`; adding public CLI subcommands `lucind-ai fixture generate` and `lucind-ai reconcile triage`; placing triage in `internal/resolve/`.
**Rationale**: `test/` lacks domain packaging; `internal/overlap/` owns geometric and AST classification (`internal/overlap/overlap.go:623-659`), not reconciliation workflows. Public CLI commands expose uncalibrated prompts prematurely. Sub-packaging `internal/conflicttriage/fixture/` maintains clean module boundaries while keeping generator primitives accessible to tests without leaking CLI surface.
**Terminal consumer**: `evaluateOverlapGate` in `internal/run/attempt.go:687-709` and `internal/run/attempt.go:831-845` consuming `ClassRequired` evidence from the fixture, satisfying proposal `## Capabilities` (`openspec/changes/conflict-triage-fixture/proposal.md:31-35`).

## Decision 3 — Independent Fail-Open Invoker vs Reusing Resolve Engine

**Choice**: Implement a dedicated fail-open triage invoker and prompt templates under `internal/conflicttriage/`, strictly separated from `internal/resolve/candidate.go`. Post-triage verification reuses `ScanConflictMarkers` (`internal/resolve/candidate.go:48-95`) and `EnforceAllowedPaths` (`internal/resolve/candidate.go:100-145`) solely as read-only invariant assertions.
**Alternatives considered**: Parameterizing `internal/resolve.ResolveCandidateMerge` with a `failOpen` flag; sharing prompt templates between `resolve` and `conflicttriage`.
**Rationale**: `internal/resolve` is deliberately fail-closed on semantic ambiguity via `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`, prompt at `internal/resolve/candidate.go:303-312`). Triage has the opposite requirement: when facing business ambiguity, it must never fail closed, but instead flag ARBITRARY, pin risk to `high`, and propose an actionable path. Modifying `internal/resolve` risks introducing regressions into the resolver's fail-closed safety invariants.
**Terminal consumer**: `internal/conflicttriage` agent invoker generating triage JSON without returning `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`), verified against invariant helpers in `internal/resolve/candidate.go:48-95` and `internal/resolve/candidate.go:100-145`.

## Decision 4 — Multi-Packet Dispatch Isolation for Fixture Build Features

**Choice**: Dispatch the two colliding fixture build features via two separate sequential `lucind-ai run` dispatches rather than a single batch. Each feature declares prefix-disjoint `allowed_paths` (`internal/packet/disjoint.go:29-48`) and a valid `parent_ref` (`internal/feature/feature.go:101-113`).
**Alternatives considered**: Dispatching both fixture features in a single multi-lane batch (`lucind-ai run --packet a.md --packet b.md`); bypassing `FeatureTarget` validation in `internal/run/integrate_feature.go`.
**Rationale**: `FeatureTarget` rejects batches mixing feature targets with `ErrMixedFeatureTargets` (`internal/run/integrate_feature.go:17` and `internal/run/integrate_feature.go:26-52`). Furthermore, batch admission rejects overlapping `allowed_paths` (`internal/packet/disjoint.go:29-48`). Sequential dispatch allows both features to register the same `base_sha` (`internal/feature/feature.go:123-124`) and establish active feature records in the ledger without violating batch admission invariants.
**Terminal consumer**: `FeatureTarget` in `internal/run/integrate_feature.go:26-52` and `DisjointAllowedPaths` in `internal/packet/disjoint.go:29-48`.

## Decision 5 — Offline Rubric Evaluation Architecture on Registered Executors

**Choice**: Execute the A/B evaluation rubric as offline test packets executed via registered executors (`claude`/`claude-opus-5` at `internal/executor/claude.go:35-52` and `opencode`/`openai/gpt-5.6-sol` at `internal/executor/opencode.go:53-65`) through `supportedExecutors` (`cmd/lucind-ai/cli.go:65-70`), scoring 3-hunk classification quality without cross-provider state leakage.
**Alternatives considered**: Hardcoding direct HTTP LLM client calls inside the test suite; introducing dynamic provider configuration in `cmd/lucind-ai`; grading runtime execution of prepared commits or operator speed.
**Rationale**: `cmd/lucind-ai/cli.go:65-70` maps canonical executor names to isolated CLI runners. Reusing existing executor registrations avoids adding external API dependencies or leaking credentials. Grading 3-hunk classification (separating business from mechanical) provides a deterministic, repeatable benchmark without introducing flaky runtime timing or grading unverified code execution.
**Terminal consumer**: `supportedExecutors` in `cmd/lucind-ai/cli.go:65-70` dispatching `Claude.Run` (`internal/executor/claude.go:106-122`) and `Opencode.Run` (`internal/executor/opencode.go:53-65`).

## Open Questions

- [ ] Exact non-decreasing risk formula and numeric thresholds, including mixed business and mechanical hunks (deferred until fixture outputs are calibrated).
- [ ] Selection of production executor/model for live conflict triage (judges are pinned to `claude-opus-5` and `openai/gpt-5.6-sol`; production runtime remains a distinct operational decision).
- [ ] Asymmetric precedence reconciliation: `~/.claude/skills/sdd-design/SKILL.md` defines monolithic single-agent document schemas, superseded here by multi-lens fan-out packet execution topology and 1000-word draft limits.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | Usage string defines CLI commands including reconcile approve and resolve |
| `cmd/lucind-ai/cli.go:65-70` | supportedExecutors registry maps claude and opencode executor constructors |
| `cmd/lucind-ai/cli.go:1445-1511` | runReconcileResolve validates candidate and sha arguments and executes resolution |
| `cmd/lucind-ai/cli.go:1501-1506` | runReconcileResolve calls UpdateCandidateStatus with CandidateStatusIntegrated |
| `internal/executor/claude.go:35-52` | Claude DefaultModel returns claude-opus-5 and KnownModels restricts models |
| `internal/executor/claude.go:106-122` | Claude Run handles stream-json decoding with terminal stdout fallback |
| `internal/executor/opencode.go:53-65` | Opencode DefaultModel returns openai/gpt-5.6-sol and KnownModels restricts models |
| `internal/feature/feature.go:101-113` | ValidateParentRef rejects empty, main, and lucind/ namespace refs |
| `internal/feature/feature.go:123-124` | Create requires non-empty baseSHA on feature creation |
| `internal/integrate/integrate.go:151-173` | PromoteCASWithRunner executes compare-and-swap ref update |
| `internal/ledger/schema.go:163` | reconciliation_candidates schema includes output TEXT column |
| `internal/overlap/overlap.go:623-659` | Classify evaluates signals against thresholds and returns ClassRequired |
| `internal/packet/disjoint.go:29-48` | DisjointAllowedPaths verifies pairwise disjoint allowed paths in a batch |
| `internal/reconcile/reconcile.go:98-111` | Candidate struct definition contains Output and CandidateSHA fields |
| `internal/reconcile/reconcile.go:105` | Candidate struct Output field carries triage JSON |
| `internal/reconcile/reconcile.go:107` | Candidate struct CandidateSHA field carries human-resolved commit SHA |
| `internal/reconcile/reconcile.go:266-276` | CreateRequest inserts ClassRequired overlap evidence into ledger |
| `internal/reconcile/reconcile.go:848-908` | UpdateCandidateStatus transitions candidate status and records candidate_sha |
| `internal/resolve/candidate.go:26` | ErrSemanticAmbiguity sentinel error for fail-closed resolution |
| `internal/resolve/candidate.go:48-95` | ScanConflictMarkers scans worktree for git conflict markers |
| `internal/resolve/candidate.go:100-145` | EnforceAllowedPaths verifies modified files stay within allowed_paths |
| `internal/resolve/candidate.go:303-312` | Resolve prompt requires resolver to fail closed on semantic ambiguity |
| `internal/run/attempt.go:687-709` | evaluateOverlapGate initializes overlap evaluation against active features |
| `internal/run/attempt.go:821-828` | evaluateOverlapGate matches approved integrated candidate on other tip SHA |
| `internal/run/attempt.go:831-845` | evaluateOverlapGate calls CreateRequest on unhandled ClassRequired overlap |
| `internal/run/attempt.go:848-855` | evaluateOverlapGate blocks attempt and releases feature lease |
| `internal/run/attempt.go:870` | evaluateOverlapGate adopts candidate SHA when exactly one conflict resolved |
| `internal/run/integrate_feature.go:17` | ErrMixedFeatureTargets sentinel error for mixed batch targets |
| `internal/run/integrate_feature.go:26-52` | FeatureTarget validates uniform feature targets across all batch packets |
| `openspec/changes/conflict-triage-fixture/proposal.md:31-35` | Capabilities defines conflict-triage and conflict-fixture capabilities |
