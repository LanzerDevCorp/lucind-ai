# Tasks: Native Stability Campaign

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 2,100–3,100 lines across 16 files (3 modified, 13 new) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Executor & Store) → PR 2 (State Machine & Fixture) → PR 3 (Evidence & Reconcile) → PR 4 (CLI & E2E Wiring) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

> [!NOTE]
> Delivery is confirmed as feature-branch work-unit commits (no push, PR, or tag). Chained-PR forecast is informational sizing evidence.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Process-group isolation (`executor.Request.Setpgid`, Linux `SysProcAttr.Setpgid`) | PR 1 | `go test -v ./internal/executor -run TestAgyLinuxSetpgidSysProcAttr` | Real subprocess seam (`internal/executor/agy_test.go:158-191`) | `internal/executor/executor.go`, `internal/executor/agy.go`, `internal/executor/agy_test.go` |
| 2 | `<git-common-dir>` SQLite authority, single-active gate, stream sanitization, RFC 8785 receipts | PR 1 / PR 3 | `go test -v ./internal/stability/store ./internal/stability/evidence` | Temp SQLite DB (`internal/stability/store/store_test.go`) | `internal/stability/store/`, `internal/stability/evidence/` |
| 3 | Process supervision (`SIGKILL`, `/proc` audit) and synthetic test fixtures | PR 1 / PR 2 | `go test -v ./internal/stability/process ./internal/stability/fixture` | Test doubles in `process_test.go` and `fixture_test.go` | `internal/stability/process/`, `internal/stability/fixture/` |
| 4 | Crash reconciliation, worktree cleanup, and 3-Trial state machine with zero-retry reset | PR 2 / PR 3 | `go test -v ./internal/stability/reconcile ./internal/stability -run TestCampaign` | State doubles in `campaign_test.go` | `internal/stability/reconcile/`, `internal/stability/campaign.go` |
| 5 | CLI routing (`run\|status\|resume\|abort`), preflight checks, and status JSON emission | PR 4 | `go test -v ./cmd/lucind-ai -run TestStability` | Test harness in `cmd/lucind-ai/stability_test.go` | `cmd/lucind-ai/cli.go:123-145`, `cmd/lucind-ai/stability.go`, `cmd/lucind-ai/stability_test.go` |

## Wave Plan & Disjointness

| Wave | Units | Parallel | Green on its own |
|---|---|---|---|
| Wave 1 | Unit 1 | No | Yes: additive `Setpgid` field and Linux `SysProcAttr`; passes `lucind-checks.sh`. |
| Wave 2 | Unit 2, Unit 3 | Yes | Yes: Unit 2 (`store/`, `evidence/`) and Unit 3 (`process/`, `fixture/`) consume Wave 1; pairwise path-disjoint; passes `lucind-checks.sh`. |
| Wave 3 | Unit 4 | No | Yes: Unit 4 (`reconcile/`, `campaign.go`) consumes Waves 1–2; passes `lucind-checks.sh`. |
| Wave 4 | Unit 5 | No | Yes: Unit 5 (`cmd/lucind-ai/`) wires CLI entry points; passes `lucind-checks.sh` and E2E simulation. |

### Disjointness Check

- **Wave 1 (Unit 1)**: Single-unit wave (`internal/executor/`). Verdict: **DISJOINT (PASS)**.
- **Wave 2 (Unit 2 vs Unit 3)**:
  - Unit 2 paths: `internal/stability/store/store.go`, `store_test.go`, `internal/stability/evidence/evidence.go`, `receipt.go`, `evidence_test.go`.
  - Unit 3 paths: `internal/stability/process/process.go`, `process_test.go`, `internal/stability/fixture/fixture.go`, `fixture_test.go`.
  - Validation: Distinct subdirectories (`store/`, `evidence/` vs `process/`, `fixture/`); no component-boundary prefix overlaps (`internal/packet/disjoint.go:13-22`). Verdict: **DISJOINT (PASS)**.
- **Wave 3 (Unit 4)**: Single-unit wave (`internal/stability/reconcile/`, `internal/stability/campaign.go`). Verdict: **DISJOINT (PASS)**.
- **Wave 4 (Unit 5)**: Single-unit wave (`cmd/lucind-ai/`). Verdict: **DISJOINT (PASS)**.

### Sidecar Recommendation

- **Recommendation**: Single packet, no sidecar (`apply-dag.yaml` omitted).
- **Rationale**: 16 files across `internal/executor/`, `internal/stability/`, and `cmd/lucind-ai/`. Orchestration overhead for multi-wave DAG dispatch is not justified. Single-packet execution with 5 work-unit commits maintains clean rollback boundaries without bisection risk (`openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27`).

## Phased Implementation Checklist

### Phase 1: Foundation (Process Group Isolation & Storage Authority)

- [ ] 1.1 Modify `internal/executor/executor.go:27-52` to add `Setpgid bool` to `executor.Request`.
- [ ] 1.2 Modify `internal/executor/agy.go:193-205` to configure `SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` on Linux when `req.Setpgid` is true and signal `-pgid` on timeout.
- [ ] 1.3 [RED] Add `TestPreflightResolvesGitCommonDirAuthority` in `internal/stability/store/store_test.go` asserting common-dir resolution under `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger_test.go:35-58`, `internal/ledgerpath/ledgerpath_test.go:9-35`).
- [ ] 1.4 Create `internal/stability/store/store.go` and `store_test.go` implementing SQLite/WAL authority (`internal/ledger/ledger.go:146-185`, `internal/worktree/worktree.go:47-69`) with single-active campaign gate.
- [ ] 1.5 Create `internal/stability/process/process.go` and `process_test.go` implementing process group supervision, `SIGKILL` termination to `-pgid`, and `/proc` survivor verification (`internal/executor/agy.go:19-40`, `internal/executor/agy_test.go:158-191`).

### Phase 2: Evidence, Fixtures & Reconciliation

- [ ] 2.1 Create `internal/stability/evidence/evidence.go`, `receipt.go`, and `evidence_test.go` implementing 4096-byte log sanitization (`internal/run/run.go:71-90`), path stripping, payload hashing, and RFC 8785 JSON Stability Receipt generation.
- [ ] 2.2 Create `internal/stability/fixture/fixture.go` and `fixture_test.go` defining Changes A and B packets, check scripts, defect injection, and deterministic tree hash / ancestry verification (`internal/worktree/worktree.go:173-238`, `internal/overlap/overlap.go:21-23`).
- [ ] 2.3 Create `internal/stability/reconcile/reconcile.go` and `reconcile_test.go` implementing fail-closed crash reconciliation, orphan process/worktree detection, and idempotent abort cleanup to `blocked_cleanup` (`internal/worktree/worktree.go:247-269`, `internal/reconcile/reconcile.go:1-33`).

### Phase 3: Campaign Orchestration & State Machine

- [ ] 3.1 Create `internal/stability/campaign.go` and `campaign_test.go` implementing sequential 3-Trial state machine (`internal/barrier/barrier.go:36-60`, `internal/barrier/barrier_test.go:31-60`), zero-retry reset on slot failure, and timeout budgets (10m dispatch, 45m Trial, 135m Campaign).
- [ ] 3.2 Implement concurrent journey execution in `internal/stability/campaign.go` running Changes A and B concurrently on pinned `gemini-3.7-flash-high` (`internal/executor/agy.go:85-96`, `internal/run/batch.go:66-78`), 10s lease expiration (`internal/feature/feature.go:334-398`), `ErrLeaseHeld` checks (`internal/feature/feature_test.go:278-310`), post-expiry reclaim with monotonic fence (`internal/feature/feature.go:406-473`), and envelope adoption (`internal/run/run.go:48-60`).
- [ ] 3.3 Implement remediation workflow in `internal/stability/campaign.go` managing Defect Record persistence, Test Actor approval gating, Fix Change dispatch to Target A (`internal/integrate/integrate.go:153-175`), and independent Target B fast-forward (`internal/run/attempt.go:740-750`).

### Phase 4: CLI Interface & Preflight Safety Gates

- [ ] 4.1 [RED] Add `TestPreflightRejectsNonGitWorkingDir`, `TestPreflightRejectsDirtyWorkingTreeStaged`, `TestPreflightRejectsDirtyWorkingTreeUntracked`, and `TestPreflightRejectsDirtyWorkingTreeModified` in `cmd/lucind-ai/stability_test.go` asserting preflight rejection of non-git repos or dirty working trees (`cmd/lucind-ai/cli_test.go:40-68`, `internal/integrate/integrate.go:126-138`).
- [ ] 4.2 Create `cmd/lucind-ai/stability.go` and `stability_test.go` implementing preflight admission checks for Linux OS, clean tree (`git status --porcelain`), candidate HEAD build match, baseline check (`internal/integrate/integrate.go:100-120`), and zero active campaigns (`cmd/lucind-ai/cli_test.go:40-60`).
- [ ] 4.3 Implement interactive 15-dispatch confirmation (default no), rejection of non-interactive / release bypass flags (`--yes`, `--tag`, `--push`, `--release`), and full forensic JSON status output in `cmd/lucind-ai/stability.go`.
- [ ] 4.4 Modify `cmd/lucind-ai/cli.go:123-145` to route `stability` subcommand to `stabilityDispatch` in `cmd/lucind-ai/stability.go`.

### Phase 5: End-to-End Campaign Verification

- [ ] 5.1 Implement and execute simulated 3-Trial Campaign verification tests (`internal/run/batch_test.go:26-59`, `internal/integrate/integrate.go:126-138`) verifying defect remediation, Change B crash recovery, zero `/proc` survivors, post-cleanup baseline pass, and immutable RFC 8785 receipt persistence.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Standalone struct field on `executor.Request`. |
| 1.2 | 1.1 | Needs `Request.Setpgid` to configure Linux `SysProcAttr` and signal process groups. |
| 1.3 | — | Standalone RED test asserting common-dir store path resolution. |
| 1.4 | 1.3 | SQLite/WAL authority satisfying storage contract. |
| 1.5 | 1.2 | Depends on `Setpgid` process group execution in `executor.Agy`. |
| 2.1 | — | Standalone evidence sanitization and RFC 8785 JSON receipt logic. |
| 2.2 | — | Standalone synthetic journey templates, check scripts, and ancestry fixtures. |
| 2.3 | 1.4, 1.5 | Requires store persistence and process supervisor to audit survivors and reconcile state. |
| 3.1 | 1.4, 1.5, 2.1 | Requires store authority, process supervisor, and evidence recorder to manage Trial transitions. |
| 3.2 | 3.1, 2.2 | Requires campaign state machine and journey fixtures to orchestrate concurrent runs and 10s lease recovery. |
| 3.3 | 3.2 | Requires concurrent journey harness to inject defects, gate Fix Change, and promote targets. |
| 4.1 | — | Standalone RED tests asserting preflight repository and working tree safety guards. |
| 4.2 | 1.4, 3.1, 4.1 | Requires store and campaign engine to validate preflight invariants and initialize campaign. |
| 4.3 | 4.2, 2.1 | Requires preflight CLI structure and evidence types to format status JSON and prompt operator. |
| 4.4 | 4.2, 4.3 | Requires stability CLI implementation to wire subcommand routing into main dispatcher. |
| 5.1 | 3.3, 4.4 | Requires complete campaign orchestration, CLI entry points, and post-cleanup verification. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `lane-execution: Process group isolation and termination` | 1.1, 1.2, 1.5 |
| `stability-authority-store: Common-directory SQLite and WAL authority` | 1.3, 1.4 |
| `stability-authority-store: Single-active campaign constraint` | 1.4 |
| `stability-campaign-state-machine: Sequential three-Trial progression and reset-on-failure` | 3.1 |
| `stability-campaign-state-machine: Execution timeout budgets` | 3.1 |
| `stability-command-contract: CLI preflight admission and safety gates` | 4.1, 4.2, 4.4 |
| `stability-command-contract: Interactive confirmation without non-interactive bypass` | 4.3 |
| `stability-evidence-receipt: Bounded evidence sanitization` | 2.1 |
| `stability-evidence-receipt: Terminal stability receipt generation` | 2.1, 5.1 |
| `stability-evidence-receipt: Non-mutating delivery boundary` | 4.3, 5.1 |
| `stability-fixture-journey: Concurrent multi-change fixture execution` | 2.2, 3.2 |
| `stability-fixture-journey: Accelerated lease expiry and crash recovery` | 1.5, 3.2 |
| `stability-fixture-journey: Deterministic fixture tree hash and ancestry isolation` | 2.2, 3.3, 5.1 |
| `stability-remediation-flow: Out-of-scope defect detection and recording` | 2.2, 3.3 |
| `stability-remediation-flow: Test Actor gated remediation and resumption` | 3.3 |
| `stability-resume-abort: Fail-closed resume reconciliation` | 2.3 |
| `stability-resume-abort: Idempotent abort and blocked cleanup` | 2.3 |

## Acceptance Evidence & Proving Commands

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Process-group isolation (`internal/executor/`) | `go test -v ./internal/executor -run TestAgyLinuxSetpgidSysProcAttr` (derived from `internal/executor/agy_test.go:158-191`) | `Request.Setpgid: true` configures Linux `SysProcAttr.Setpgid` (`internal/executor/agy.go:193-205`) | Grandchild process killing under `SIGKILL` without `internal/stability/process` |
| Common-dir SQLite store (`internal/stability/store/`) | `go test -v ./internal/stability/store -run TestStoreSingleActiveGateAndCommonDirResolution` (derived from `internal/ledger/ledger_test.go:35-58`, `internal/ledgerpath/ledgerpath_test.go:9-35`) | Store resolves `<git-common-dir>` and rejects concurrent campaigns (`internal/ledger/ledger.go:146-185`) | Uncheckpointed WAL durability across OS power failure |
| Process kill & `/proc` audit (`internal/stability/process/`) | `go test -v ./internal/stability/process -run TestProcessGroupKillAndProcSurvivorAudit` (derived from `internal/executor/agy_test.go:158-191`) | `SIGKILL` to `-pgid` terminates descendants; audit finds 0 survivors (`internal/executor/agy.go:19-40`) | Non-Linux execution (scoped to Linux) |
| 3-Trial state machine (`internal/stability/`) | `go test -v ./internal/stability -run TestCampaignSequentialThreeTrialsAndZeroRetryReset` (derived from `internal/barrier/barrier_test.go:31-60`) | 3 sequential passes, 10m/45m/135m budgets, 0-retry failure reset (`internal/barrier/barrier.go:36-60`) | Live backend network model latency or throttling |
| Fixture journey & defect (`internal/stability/fixture/`) | `go test -v ./internal/stability/fixture -run TestFixtureConcurrentJourneyDefectAndAncestryIsolation` (derived from `internal/feature/feature_test.go:278-310`, `internal/integrate/integrate_test.go:49-100`) | Concurrent A/B journeys, Fix Change remediation, target ancestry isolation (`internal/worktree/worktree.go:173-238`) | Merge conflict resolution outside fixture templates |
| Evidence sanitization & receipt (`internal/stability/evidence/`) | `go test -v ./internal/stability/evidence -run TestEvidenceSanitizationAndCanonicalReceiptRFC8785` (derived from `internal/run/run.go:71-90`, `internal/integrate/integrate.go:100-120`) | 4096B stream cap, path stripping, raw payload hashing, RFC 8785 receipt (`internal/run/run.go:131-150`) | Cryptographic notary signature attestation |
| Fail-closed crash recovery (`internal/stability/reconcile/`) | `go test -v ./internal/stability/reconcile -run TestReconcileFailClosedAmbiguityAndIdempotentAbort` (derived from `internal/worktree/worktree_test.go:16-58`, `internal/reconcile/reconcile.go:1-33`) | Ambiguity triggers `blocked_cleanup`; `abort` idempotently removes residues (`internal/worktree/worktree.go:247-269`) | Cleanup when files locked by immutable root OS permissions |
| CLI preflight & E2E simulation (`cmd/lucind-ai/`) | `go test -v ./cmd/lucind-ai -run TestStabilityRunPreflightAndSimulatedThreeTrialRun` (derived from `cmd/lucind-ai/cli_test.go:40-68`, `internal/run/batch_test.go:26-59`) | Subcommand parsing, clean tree preflight, non-interactive rejection, simulated 3-Trial run (`cmd/lucind-ai/cli.go:123-145`) | Replaces required live `lucind-ai stability run` with `gemini-3.7-flash-high` |

## Open Questions

- [ ] None.
