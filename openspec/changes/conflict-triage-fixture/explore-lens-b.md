# Explore Lens B — Capabilities & Scenarios: Conflict Triage Fixture

## User & Capability Impact

The `conflict-triage-fixture` change introduces capabilities for developers, operators, and automated lane orchestrators by establishing:

1. **Deterministic Overlap & Reconciliation Testbed (`conflict-fixture`)**:
   - Today, the ledger records 0 rows across `overlap_evidence`, `reconciliation_requests`, and `reconciliation_candidates` despite 36 integration attempts (`internal/ledger/schema.go:131-169`).
   - The fixture provides on-demand generation of `ClassRequired` overlap states (`internal/overlap/overlap.go:622-660`) between active features (`internal/run/attempt.go:687-714`), allowing reliable testing of the overlap gate (`internal/run/attempt.go:777-856`), CLI approval workflows (`cmd/lucind-ai/cli.go:1090-1195`), and resolution candidate registration (`cmd/lucind-ai/cli.go:1398-1460`).
   - Feature decomposition enforces isolation: two path-disjoint features (`feature/conflict-triage-agent` owning `internal/conflicttriage/**` and `feature/conflict-fixture` owning fixture assets) avoid self-collision during development (`internal/run/integrate_feature.go:17-52`).

2. **Rapid Merge Conflict Triage Agent (`conflict-triage`)**:
   - Introduces `internal/conflicttriage/` to transform merge conflicts into 30-second human decisions.
   - Unlike the autonomous merge resolver (`internal/resolve/candidate.go:303-306`) which fails closed on semantic ambiguity with `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`), `conflict-triage` explains architectural intent/causes (rather than line-level diffs), proposes resolution commits, explicitly identifies arbitrary business decisions, and ratchets risk to high.
   - Enables operator verification of prepared resolutions before committing them via `lucind-ai reconcile resolve` (`cmd/lucind-ai/cli.go:1398`).

3. **Multi-Executor Evaluation & Rubric Benchmark**:
   - Enables heterogeneous benchmarking across all four configured executors (`agy`, `claude`, `cursor-agent`, `opencode` registered in `cmd/lucind-ai/cli.go:65-70` and `internal/executor/claude.go:35-50`) using a standardized 3-hunk collision shape (1 business rule conflict, 2 mechanical controls).

## Scenarios & Use Cases

### Scenario 1 — Reproducible `ClassRequired` Overlap Generation via Conflict Fixture

- **Context**: Two features with active leases and distinct parent refs share a common base commit in the ledger (`internal/run/attempt.go:688-741`). The fixture injects a toy file with conflicting hunks matching `overlap.Classify` triggers (`internal/overlap/overlap.go:622-656`).
- **Action**: Operator or automated test harness executes batch feature integration (`lucind-ai run --packet ...`) for feature A while feature B is active (`internal/run/integrate_feature.go:80-95`).
- **Outcome**: `evaluateOverlapGate` classifies the overlap as `ClassRequired` (`internal/overlap/overlap.go:659`), writes evidence to `overlap_evidence` (`internal/run/attempt.go:768-775`), records an awaiting request in `reconciliation_requests` (`internal/run/attempt.go:830-845`), transitions attempt status to `AttemptStatusBlocked` (`internal/run/attempt.go:848`), releases the lease (`internal/run/attempt.go:854`), and preserves the lane worktree at `lucind/<laneID>` (`internal/worktree/worktree.go:79-81`).

### Scenario 2 — Triage of Semantic Ambiguity vs Mechanical Control Hunks

- **Context**: An awaiting reconciliation request exists with 3 conflicting hunks in the fixture toy file: (a) a business logic conflict where both versions compile and pass unit tests, (b) a slice-literal union, and (c) a rename colliding with an edit to the old name.
- **Action**: `conflict-triage` agent is invoked with the conflict context across a designated executor (`cmd/lucind-ai/cli.go:65-70`).
- **Outcome**: The agent resolves mechanical hunks (b and c) deterministically. For hunk (a), instead of failing closed with `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`), it explicitly flags the decision as an arbitrary business choice, ratchets risk to high, prepares a candidate resolution commit SHA, and outputs an explanatory triage summary.

### Scenario 3 — Operator Approval, Resolution Registration, and Re-Dispatch Promotion

- **Context**: A feature attempt is blocked on a `ClassRequired` overlap (`internal/run/attempt.go:848-850`). A prepared resolution commit SHA exists from triage or manual intervention.
- **Action**: Operator executes `lucind-ai reconcile approve --request <id> --source <f1> --target <f2>` (`cmd/lucind-ai/cli.go:1090-1195`, `internal/reconcile/reconcile.go:406-535`), registers the candidate via `lucind-ai reconcile resolve --candidate <id> --sha <sha>` (`cmd/lucind-ai/cli.go:1398-1460`, `internal/reconcile/reconcile.go:848-910`), cleans up the worktree via `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:56`), and re-dispatches the packet batch.
- **Outcome**: On retry, `evaluateOverlapGate` detects the approved request and matching target tip (`internal/run/attempt.go:821-828`), adopts `cand.CandidateSHA` as `resolvedOverrideSHA` (`internal/run/attempt.go:870`), clears the overlap block, and completes CAS promotion on the feature `parent_ref` (`internal/run/integrate_feature.go:80-96`).

### Scenario 4 — Multi-Feature Admission Gate Enforcement

- **Context**: Disjoint features `feature/conflict-triage-agent` and `feature/conflict-fixture` are managed independently in the ledger (`internal/run/integrate_feature.go:17-53`).
- **Action**: An operator or script attempts to dispatch a single batch mixing packets for both features or naming invalid parent refs (`main` or `lucind/*`).
- **Outcome**: `FeatureTarget` rejects the batch with `ErrMixedFeatureTargets` (`internal/run/integrate_feature.go:17`, `:41`) or `ErrInvalidParentRef` (`internal/run/integrate_feature.go:73`, `internal/worktree/worktree.go:99-110`), failing fast before worktrees or ledger leases are allocated.

### Scenario 5 — Heterogeneous Judge Model A/B Benchmarking

- **Context**: The conflict fixture provides identical, reproducible conflict evidence across evaluation runs.
- **Action**: The benchmark rubric executes `conflict-triage` judge packets against `claude`/`claude-opus-5` (`internal/executor/claude.go:35`) and `opencode`/`openai/gpt-5.6-sol` (`cmd/lucind-ai/cli.go:69`).
- **Outcome**: The rubric compares both models on cause explanation clarity, proper business risk ratcheting, and prepared commit validity while enforcing strict single-provider billing isolation per executor (`cmd/lucind-ai/cli.go:65-70`).

## Success Criteria

- [ ] Conflict fixture deterministically triggers `ClassRequired` overlap classification in `overlap.Classify` (`internal/overlap/overlap.go:622-660`) and inserts valid `overlap_evidence` rows (`internal/ledger/schema.go:131-139`).
- [ ] Overlap gate creates awaiting `reconciliation_requests` rows (`internal/ledger/schema.go:141-154`) and blocks feature promotion (`internal/run/attempt.go:848-856`) upon encountering fixture conflicts.
- [ ] `conflict-triage` agent resolves mechanical hunks and surfaces business conflicts with mandatory high-risk ratcheting and a prepared commit SHA without failing closed (`internal/resolve/candidate.go:26`, `:305`).
- [ ] End-to-end reconciliation lifecycle completes via CLI commands `reconcile approve` (`cmd/lucind-ai/cli.go:1090`) and `reconcile resolve` (`cmd/lucind-ai/cli.go:1398`), allowing retried feature attempts to promote using `cand.CandidateSHA` (`internal/run/attempt.go:821-871`).
- [ ] Benchmark rubric evaluates judge performance across supported executors (`agy`, `claude`, `cursor-agent`, `opencode` in `cmd/lucind-ai/cli.go:65-70`) without cross-executor configuration leaks.

## Open Questions

- [ ] **Explore Skill vs Lane Split Contract Drift**: `~/.claude/skills/sdd-explore/SKILL.md` expects a single subagent to author an entire `explore.md` and persist to Engram, whereas this packet splits exploration into three parallel lenses (A, B, C) targeting `explore-lens-b.md` for upstream synthesis.
- [ ] **Risk Ratchet Thresholds**: What exact mathematical formula or discrete rubric determines risk escalation when multiple partial-business and partial-mechanical hunks co-exist?
- [ ] **Verification Budget Estimation**: How should the triage agent calculate the verification cost (token spend, test suite execution time) when recommending whether a human should accept a prepared resolution unverified?
