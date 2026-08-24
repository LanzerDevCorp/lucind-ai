# Explore Lens A — Problem & Candidates: Conflict Triage Fixture

## Problem Space

The Lucind multi-feature orchestration framework coordinates concurrent feature development across isolated git worktrees (`internal/worktree/worktree.go:150-185`) and enforces deterministic overlap gating prior to promotion (`internal/run/attempt.go:684-856`). During attempt integration, `evaluateOverlapGate` compares candidate commits against all active features via `overlap.Classify` (`internal/overlap/overlap.go:622-660`). When `ClassRequired` overlap is detected (e.g., merge-tree conflicts, rename/delete collisions, intersecting hunks, or hotspot weights >= 0.50 per `internal/overlap/overlap.go:93-98`), promotion is blocked, a reconciliation request is recorded (`internal/reconcile/reconcile.go:213-336`), and the attempt enters `AttemptStatusBlocked` (`internal/run/attempt.go:848-854`).

However, two major operational gaps exist in the current system:

1. **Unexercised Reconciliation Pipeline in Production**: Across 36 integration attempts in the ledger, zero `ClassRequired` overlaps have been recorded — `overlap_evidence`, `reconciliation_requests`, and `reconciliation_candidates` all contain 0 rows. The entire reconciliation lifecycle (`internal/reconcile/reconcile.go:1-934`) and CLI resolution bridge (`cmd/lucind-ai/cli.go:1386-1463`) remain completely unexercised in production.
2. **Missing Reproducible Conflict Fixture**: Batch admission strictly rejects mixed feature targets and invalid parents (`internal/run/integrate_feature.go:26-78`). Two plain git branches that collide never enter the feature ledger. Without an automated fixture generating multi-feature collisions on demand, there is no harness to benchmark resolution models, calibrate risk formulas, or validate judge prompts.
3. **Semantic Ambiguity and Triage Deficit**: The existing resolver (`internal/resolve/candidate.go:1-378`) is explicitly fail-closed (`ErrSemanticAmbiguity` at `candidate.go:26`, prompt at `candidate.go:303-305`), forbidding directional choice. When human intervention is required, the operator must execute a manual two-step process (`reconcile approve` followed by `reconcile resolve --candidate <id> --sha <sha>` in `cmd/lucind-ai/cli.go:1386-1463`) without automated triage to explain root causes, estimate verification budgets, or prepare candidate commits. The `conflict-triage` agent (`internal/conflicttriage/`) is designed to fill this gap but has not yet been implemented.

## Candidate Approaches

### Candidate 1 — Two Disjoint Feature Workflows with Heterogeneous Dual-Judge Rubric

**Approach**: Execute the change as two path-disjoint features (`feature/conflict-triage-agent` owning `internal/conflicttriage/**` and `feature/conflict-fixture` owning the fixture generator, judge packets, and rubric evaluation), respecting admission rules in `internal/run/integrate_feature.go:26-78` and disjointness checks in `internal/packet/disjoint.go:29-48`. The fixture synthesizes a 3-hunk collision file (1 business decision hunk where conflicting product rules both compile and pass tests, plus 2 mechanical control hunks: a slice-literal union and a rename/edit collision). It feeds the conflict into a dual-judge pipeline executed by heterogeneous models across `supportedExecutors` (`cmd/lucind-ai/cli.go:65-70` `agy`, `opencode`, `claude`, `cursor-agent`), scoring triage outputs against a deterministic rubric.
**Pros**: Preserves strict feature isolation without circular dependencies; enables empirical calibration of the risk ratchet and verify budgets across multiple model families; tests both semantic and mechanical triage reasoning with clear controls.
**Cons**: Requires authoring and managing two separate feature batches and an offline rubric evaluation harness.
**Feasibility**: High. Directly leverages existing multi-executor dispatch (`cmd/lucind-ai/cli.go:65-70`), worktree management (`internal/worktree/worktree.go:150-185`), and candidate status tracking (`internal/reconcile/reconcile.go:848-890`).

### Candidate 2 — In-Memory Synthetic Overlap Harness with Synchronous Attempt Hooks

**Approach**: Implement the 3-hunk conflict fixture as an in-memory/temporary directory test helper in Go unit tests (`openspec/config.yaml:8-13`). Hook `internal/conflicttriage/` directly into `internal/reconcile/reconcile.go:213-336` and `internal/run/attempt.go:830-856` so that creating a reconciliation request automatically and synchronously invokes LLM triage, populating candidate proposals directly during the attempt evaluation loop.
**Pros**: Keeps code self-contained within unit test suites; avoids coordinating multiple feature branches and batch runs.
**Cons**: Couples synchronous ledger transactions and attempt promotion loops to external, high-latency LLM API calls; risks promotion timeouts in `evaluateOverlapGate` (`internal/run/attempt.go:684-856`); eliminates realistic multi-worktree git behavior (`internal/worktree/worktree.go:1-14`).
**Feasibility**: Medium. Technically possible within SQLite transaction boundaries (`internal/ledger/ledger.go`), but introduces unacceptable latency and failure coupling into core promotion mechanics.

### Candidate 3 — CLI-Driven Fixture Generator with CLI Triage Assistant Subcommand

**Approach**: Add dedicated CLI subcommands to `cmd/lucind-ai/cli.go:56` (`lucind-ai fixture generate` and `lucind-ai reconcile triage --request <id>`). The fixture command populates real on-disk worktrees and ledger entries with `ClassRequired` overlaps (`internal/overlap/overlap.go:622-660`), while the triage command invokes `internal/conflicttriage` to output human-readable cause analysis and register a prepared resolution commit with `runReconcileResolve` (`cmd/lucind-ai/cli.go:1386-1463`).
**Pros**: Provides immediate terminal-level inspection and operational ergonomics matching existing `reconcile` verbs (`cmd/lucind-ai/cli.go:56, 1386-1463`).
**Cons**: Exposes new public CLI surface area before judge scoring and prompt calibration have stabilized; mixes exploratory fixture tooling into production binary commands.
**Feasibility**: High. Can follow established flag parsing patterns (`cmd/lucind-ai/cli.go:1398-1440`) and ledger interfaces (`internal/ledger/ledger.go`).

## Initial Recommendations

We recommend **Candidate 1** (Two Disjoint Feature Workflows with Heterogeneous Dual-Judge Rubric). This approach adheres to the established feature decomposition (`feature/conflict-triage-agent` and `feature/conflict-fixture`), satisfies batch admission constraints (`internal/run/integrate_feature.go:26-78`), and maintains path disjointness (`internal/packet/disjoint.go:29-48`). Most importantly, it creates the empirical foundation needed to validate the 0-row reconciliation tables (`internal/run/attempt.go:684-856`) and calibrate the triage prompt's 30-second decision objective against heterogeneous models (`cmd/lucind-ai/cli.go:65-70`) before modifying CLI surfaces or UI handlers (`internal/serve/handlers.go:95-109`).

## Open Questions

- [ ] How should verify budget estimates be standardized in triage outputs (e.g., token usage, test suite execution time via `openspec/config.yaml:31`, or model pricing)?
- [ ] What exact numeric threshold and formula will govern the non-decreasing risk ratchet when semantic product differences are detected?
- [ ] Should the fixture generator live under `test/fixture/` or as an internal package under `internal/fixture/`?
