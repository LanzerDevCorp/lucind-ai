# Explore Lens B — Capabilities & Scenarios: Control Room UI Views

## User & Capability Impact

The Control Room UI views serve the single human operator driving parallel subscription execution, gating approvals, supervising feature integration, and debugging lane outcomes. This change introduces five core view components inside the control room shell:

1. **Batch & DAG Wave Inspector**: Visualizes concurrent batch dispatches (`BatchReport`, `internal/run/batch.go:19-27`), multi-wave DAG progression (`internal/dag/waves.go:41-70`), parallel lane execution states (`pending`, `running`, `done`, `blocked`, `deviated`, `failed` in `internal/lane/status.go:17-48`), executor allocation (`agy`, `cursor-agent`, `opencode` in `cmd/lucind-ai/cli.go:65-69`), independent per-lane timeout countdowns (`internal/run/batch.go:40-43`, `cmd/lucind-ai/cli.go:42`), worktree isolation paths (`internal/worktree/worktree.go:168-238`), and barrier synchronization status (`internal/barrier/barrier.go:18-60`).
2. **Approvals & Anti-Rubber-Stamping View**: Extends the approval interface (`internal/serve/handlers.go:120-146`, `internal/serve/static/app.js:22-69`) with strict individual item decision controls (`internal/serve/handlers.go:161-176`, `openspec/specs/approvals-web-ui/spec.md:26-48`), inline command output and `file:line` verification evidence rendering (`internal/serve/static/app.js:12-20`, `openspec/specs/approvals-web-ui/spec.md:49-66`), personal wrong-approval defect rate display (`internal/ledger/ledger.go:307-353`, `docs/prd.md:229-241`), and post-merge `opencode` RDD command presentation (`internal/serve/handlers.go:139`).
3. **Feature Lifecycle & Lease Monitor**: Surfaces feature records (`features`, `internal/ledger/schema.go:96-105`, `internal/feature/feature.go:48-93`, `internal/serve/model.go:27-35`), active lease owners and monotonic fence counters (`feature_leases`, `internal/ledger/schema.go:156-172`, `internal/feature/feature.go:293-360`, `internal/serve/model.go:52-59`), integration attempt CAS verification outcomes (`integration_attempts`, `internal/run/integrate_feature.go:28-112`, `internal/serve/model.go:37-50`), and 4-way diff overlap evidence (`overlap_evidence`, `internal/ledger/schema.go:174-190`, `internal/dag/overlap.go:52-78`, `internal/serve/model.go:62-70`).
4. **Reconciliation & Conflict Workspace**: Manages cross-feature reconciliation requests (`reconciliation_requests`, `internal/ledger/schema.go:206-230`, `internal/reconcile/reconcile.go:35-120`, `internal/serve/model.go:74-92`), AI resolution candidate diffs bounded to 400 lines (`reconciliation_candidates`, `internal/ledger/schema.go:232-260`, `internal/resolve/candidate.go:25-90`, `internal/serve/model.go:101-115`), operator action triggers (`approve`, `decline`, `cancel`, `renew`, `resolve` in `cmd/lucind-ai/cli.go:56`, `internal/reconcile/reconcile.go:122-260`), and immutable audit event timelines (`integration_events`, `internal/ledger/schema.go:262-280`, `internal/serve/model.go:117-125`).
5. **Lane Envelope & Diagnosis Inspector**: Enables deep inspection of returned `.lucind/result.json` envelopes (`internal/result/schema.go:20-65`, `internal/result/result.schema.json:1-160`), terminal consumer evidence verification, mandatory hard-stop declaration checks (`internal/run/run.go:549-574`, `docs/prd.md:181-185`), post-execution git diff boundary checks against `allowed_paths` (`internal/run/run.go:576-654`), and verbatim question relays for blocked lanes (`cmd/lucind-ai/cli.go:350-380`).

## Scenarios & Use Cases

### Scenario 1 — Real-Time Wave & Parallel Lane Progress Tracking

- **Context**: The operator dispatches a multi-wave batch with two parallel lanes assigned to `agy` and `cursor-agent` (`internal/run/batch.go:29-89`, `cmd/lucind-ai/cli.go:65-69`).
- **Action**: The operator opens the Batch Execution View in the Control Room.
- **Outcome**: The view displays active lanes in `running` status (`internal/lane/status.go:22-26`), showing each lane's isolated worktree path (`internal/worktree/worktree.go:168-238`), independent timeout countdown (`internal/run/batch.go:40-43`), and live wave synchronization state (`internal/dag/waves.go:41-70`) until the barrier releases (`internal/barrier/barrier.go:18-60`).

### Scenario 2 — Individual Item Approval with Inline Evidence and Defect Scorecard

- **Context**: Two lanes in a batch request operator approval with attached test output and `file:line` citations (`internal/ledger/ledger.go:230-264`, `internal/serve/handlers.go:120-146`).
- **Action**: The operator navigates to the Approvals View and reviews the pending items.
- **Outcome**: All items initialize unselected with bulk approval prohibited (`internal/serve/handlers.go:161-176`, `openspec/specs/approvals-web-ui/spec.md:26-48`). The view renders verified command output inline (`internal/serve/static/app.js:12-20`), displays the operator's personal wrong-approval defect rate (`internal/ledger/ledger.go:307-353`), and upon individual approval reveals the exact `opencode` RDD command (`internal/serve/handlers.go:139`, `openspec/specs/approvals-web-ui/spec.md:51-60`).

### Scenario 3 — Supervise Feature Integration, Leases, and Overlap Evidence

- **Context**: A feature branch integration is initiated with active lease locking and 4-way overlap checks (`internal/ledger/schema.go:96-190`, `internal/feature/feature.go:293-360`).
- **Action**: The operator selects a feature in the Feature Lifecycle View.
- **Outcome**: The view displays current feature status (`internal/serve/model.go:27-35`), active lease owner and monotonic fence counter (`internal/serve/model.go:52-59`), latest integration attempt CAS result (`internal/run/integrate_feature.go:28-112`, `internal/serve/model.go:37-50`), and detailed overlap evidence diffs (`internal/serve/model.go:62-70`, `internal/dag/overlap.go:52-78`).

### Scenario 4 — Review and Resolve AI Reconciliation Candidates

- **Context**: Concurrent feature branches produce an overlap conflict requiring reconciliation (`internal/reconcile/reconcile.go:35-120`, `internal/ledger/schema.go:206-230`).
- **Action**: The operator opens the Reconciliation Workspace to inspect AI-generated candidate resolutions (`internal/resolve/candidate.go:25-90`, `internal/serve/model.go:101-115`).
- **Outcome**: The view presents candidate diffs bounded to 400 lines alongside verification check results (`internal/serve/model.go:101-115`), providing action buttons to approve, decline, cancel, or resolve the candidate SHA (`cmd/lucind-ai/cli.go:56`, `internal/reconcile/reconcile.go:122-260`) while logging integration audit events (`internal/serve/model.go:117-125`).

### Scenario 5 — Diagnosing a Demoted Lane via Envelope & Allowed-Paths Inspector

- **Context**: An executor reports `done` in `.lucind/result.json`, but git diff inspection reveals modified files outside declared `allowed_paths` (`internal/run/run.go:576-654`).
- **Action**: The operator inspects the completed lane in the Lane Detail View.
- **Outcome**: The view highlights the demotion from `done` to `deviated` (`internal/run/run.go:651-653`), displays the offending path diffs (`internal/run/run.go:620-650`), verifies whether mandatory hard stops were declared (`internal/result/schema.go:20-65`, `docs/prd.md:181-185`), and links to the preserved worktree for audit (`internal/run/batch.go:50-52`).

## Success Criteria

- [ ] Every view component (Batch/Wave Inspector, Approvals View, Feature/Lease Monitor, Reconciliation Workspace, Lane Envelope Inspector) maps to existing ledger models (`internal/serve/model.go:27-125`, `internal/ledger/schema.go:18-280`).
- [ ] Approvals View strictly enforces individual selection, rejection of bulk actions, and rendering of inline evidence (`internal/serve/handlers.go:161-176`, `openspec/specs/approvals-web-ui/spec.md:26-66`).
- [ ] Batch & Wave Inspector reflects live lane status transitions (`pending`, `running`, `done`, `blocked`, `deviated`, `failed`) and barrier releases (`internal/lane/status.go:17-48`, `internal/barrier/barrier.go:18-60`).
- [ ] Feature & Reconciliation Views surface lease fences, CAS outcomes, overlap evidence, candidate diffs, and audit trails without executing git commands (`internal/serve/model.go:14-125`).
- [ ] Lane Envelope Inspector validates result envelopes against `internal/result/result.schema.json` and flags `allowed_paths` scope violations (`internal/run/run.go:549-654`).

## Open Questions

- [ ] None
