# Tasks Synthesis Notes: Native Stability Campaign

## Unresolved Contradictions

None.

All three lens drafts converged on the core implementation architecture, package boundaries, and dependency ordering. The apparent divergence between Lens B's recommendation of a single packet (no `apply-dag.yaml` sidecar, 5 sequential work-unit commits) and Lens C's recommendation of a 4-PR feature-branch chain is fully resolved by the delivery strategy already confirmed by the human (work-unit commits on the feature branch only, with no pull requests, git pushes, or git tags). Lens C's review forecast functions as informational sizing evidence rather than an unresolved delivery conflict.

## Coverage Gaps

None.

All eight spine items required by the repository and `sdd-tasks` contract are substantively covered in `tasks.md`:
1. Review Workload Forecast table with all fields populated and plain-text guard contract lines.
2. Suggested Work Units table with standalone deliverables, focused test commands, runtime harness references, and rollback boundaries.
3. Phased checklist with specific, actionable, verifiable, and small tasks using hierarchical numbering.
4. Threat-matrix RED tests preceding production tasks for every Applicable row (`TestPreflightResolvesGitCommonDirAuthority`, `TestPreflightRejectsNonGitWorkingDir`, `TestPreflightRejectsDirtyWorkingTreeStaged`, `TestPreflightRejectsDirtyWorkingTreeUntracked`, `TestPreflightRejectsDirtyWorkingTreeModified`), and none for N/A rows.
5. Explicit dependency order table with rationales.
6. Wave plan verified green-on-its-own under `Integrate` and pairwise disjoint under the component-boundary prefix rule (`internal/packet/disjoint.go:13-22`). No waves required merging.
7. Explicit executor recommendations and single-packet shape rationale.
8. Complete requirement traceability matrix mapping all 17 requirements across the 8 accepted capability delta specs in `openspec/changes/native-stability-campaign/specs/` to checklist tasks.

## Dropped Citations

None.

Every `file:line` citation in the manifests and prose of Lens A, Lens B, and Lens C was verified against the repository worktree and confirmed to support its stated claim:
- `cmd/lucind-ai/cli.go:123-145`, `140-142`, `503-509`, `710-723`: verified subcommand routing, version output format, preflight check execution, and result envelope persistence.
- `cmd/lucind-ai/cli_test.go:40-68`: verified CLI argument validation, missing/unknown subcommand tests, and version parsing seams.
- `internal/barrier/barrier.go:36-60`, `internal/barrier/barrier_test.go:31-60`: verified pure barrier evaluation, release transitions, and unit tests.
- `internal/executor/agy.go:19-40`, `85-96`, `193-205`: verified `defaultWaitDelay`, model allow-listing (`gemini-3.7-flash-high`), and `exec.CommandContext` setup.
- `internal/executor/agy_test.go:158-191`: verified grandchild pipe-holding behavior and process group testing seams.
- `internal/executor/executor.go:27-52`: verified `Request` struct definition.
- `internal/feature/feature.go:98-130`, `334-398`, `406-473`: verified `ValidateParentRef`, `Create`, `AcquireLease` with monotonic fencing, and `RenewLease`.
- `internal/feature/feature_test.go:278-310`: verified unit tests for lease acquisition, expiration, and fence increments.
- `internal/integrate/integrate.go:52-85`, `100-120`, `126-138`, `153-175`: verified `Combine`, `Check`, `Promote`, and `PromoteCAS` / `PromoteRef`.
- `internal/integrate/integrate_test.go:49-100`: verified integration test combining lane branches into linked worktrees.
- `internal/ledger/ledger.go:146-185`, `internal/ledger/ledger_test.go:35-58`: verified SQLite WAL connection configuration and DB placement.
- `internal/ledgerpath/ledgerpath.go:34-58`, `internal/ledgerpath/ledgerpath_test.go:9-58`: verified ledger path resolution and validation.
- `internal/overlap/overlap.go:21-23`: verified merge base sentinel errors.
- `internal/packet/disjoint.go:13-22`, `29-48`: verified `PathInScope` prefix rule and `DisjointAllowedPaths`.
- `internal/reconcile/reconcile.go:1-33`: verified reconcile sentinel errors and request status types.
- `internal/run/attempt.go:740-750`: verified ref SHA resolution and commit verification.
- `internal/run/batch.go:66-78`, `internal/run/batch_test.go:26-59`: verified batch execution initialization and test fixtures.
- `internal/run/integrate.go:50-59`: verified integrate gate check execution and bisection on failure.
- `internal/run/run.go:48-60`, `71-90`, `131-150`: verified envelope constants, `streamDetailCap = 4096`, and diagnostic formatting.
- `internal/worktree/worktree.go:47-69`, `173-238`, `247-269`: verified `GitRunner`, `CreateWithParent`, `Cleanup`, and `Remove`.
- `internal/worktree/worktree_test.go:16-58`: verified linked worktree detection tests.
- `docs/adr/0001-native-stability-campaign.md:5-24`: verified accepted ADR contract.
- `lucind-ai-stability-run-sdd-master-plan.md:240-252`: verified suggested stability subpackages.
- `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27`: verified sidecar precedent.
- `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml:8-14`: verified Strict-TDD bisection precedent.
- `openspec/changes/native-stability-campaign/design.md:81-93`, `121-127`: verified file-changes table and threat matrix.
- Derivation sources for proposed test names (`TestAgyLinuxSetpgidSysProcAttr`, `TestStoreSingleActiveGateAndCommonDirResolution`, `TestProcessGroupKillAndProcSurvivorAudit`, `TestCampaignSequentialThreeTrialsAndZeroRetryReset`, `TestFixtureConcurrentJourneyDefectAndAncestryIsolation`, `TestEvidenceSanitizationAndCanonicalReceiptRFC8785`, `TestReconcileFailClosedAmbiguityAndIdempotentAbort`, `TestStabilityRunPreflightAndSimulatedThreeTrialRun`) verified against existing test patterns.

## Decomposition Divergence

The three lens drafts proposed overlapping but distinct decomposition shapes (5 phases / 14 tasks in Lens A, 5 work units / 4 waves in Lens B, and 4 sequential PR units in Lens C):

### Shape Mapping
- **Lens A (Authoritative decomposition: 5 sequential phases, 14 numbered tasks)**:
  - Phase 1: Foundation (Tasks 1.1, 1.2, 1.3, 1.4, 1.5)
  - Phase 2: Evidence, Fixtures & Reconciliation (Tasks 2.1, 2.2, 2.3)
  - Phase 3: Campaign Orchestration & State Machine (Tasks 3.1, 3.2, 3.3)
  - Phase 4: CLI Interface & Preflight Safety Gates (Tasks 4.1, 4.2, 4.3, 4.4)
  - Phase 5: End-to-End Campaign Verification (Task 5.1)
- **Lens B (5 Suggested Work Units across 4 Waves)**:
  - Unit 1 (Wave 1): Process-Group Isolation (`internal/executor/`) → Maps to Lens A Tasks 1.1, 1.2.
  - Unit 2 (Wave 2, parallel): Storage Authority & Evidence Receipts (`store/`, `evidence/`) → Maps to Lens A Tasks 1.3, 1.4, 2.1.
  - Unit 3 (Wave 2, parallel): Process Supervision & Fixture Journey (`process/`, `fixture/`) → Maps to Lens A Tasks 1.5, 2.2.
  - Unit 4 (Wave 3): Stability Reconcile & Campaign State Machine (`reconcile/`, `campaign.go`) → Maps to Lens A Tasks 2.3, 3.1, 3.2, 3.3.
  - Unit 5 (Wave 4): CLI Subcommands & Preflight Admission (`cmd/lucind-ai/`) → Maps to Lens A Tasks 4.1, 4.2, 4.3, 4.4, 5.1.
- **Lens C (4 Sequential PR Work Units)**:
  - PR Unit 1: Executor & Store (`internal/executor/`, `store/`, `process/`) → Maps to Lens A Phase 1 (Tasks 1.1–1.5).
  - PR Unit 2: State Machine & Fixture (`campaign.go`, `fixture/`) → Maps to Lens A Tasks 2.2, 3.1, 3.2, 3.3.
  - PR Unit 3: Evidence & Reconcile (`evidence/`, `reconcile/`) → Maps to Lens A Tasks 2.1, 2.3.
  - PR Unit 4: CLI & E2E Wiring (`cmd/lucind-ai/`) → Maps to Lens A Phases 4 and 5 (Tasks 4.1–4.4, 5.1).

### Divergence Analysis
- Lens A decomposed chronologically by architectural layer (Foundation → Artifacts/Fixtures → Engine → CLI → E2E).
- Lens B partitioned by package encapsulation and parallelization potential (pairing `store/` with `evidence/` and `process/` with `fixture/` to achieve path-disjoint Wave 2 concurrency).
- Lens C partitioned by code review diff budget (~500–800 lines per PR unit).
- All three converged independently on the 16 delivery files (3 modified, 13 new) and the primary dependency chain.
- No tasks or deliverables were dropped; the synthesized `tasks.md` preserves Lens A's 14 tasks (with explicit RED test tasks 1.3 and 4.1), maps them directly onto Lens B's 5 standalone work units, and reflects Lens C's review sizing.
