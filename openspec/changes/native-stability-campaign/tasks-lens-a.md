# Tasks Lens A — Decomposition & Ordering: Native Stability Campaign

## Assumed decomposition

The implementation is decomposed into 5 sequential phases delivering process isolation and authority storage (Phase 1), evidence sanitization, journey fixtures, and crash reconciliation (Phase 2), campaign state machine with zero-retry reset and remediation gating (Phase 3), CLI preflight admission and status reporting (Phase 4), and simulated 3-Trial end-to-end certification (Phase 5). The critical path runs from `Request.Setpgid` and SQLite authority store (Phase 1) through the campaign engine and journey orchestration (Phase 3) to CLI preflight and end-to-end certification (Phases 4–5).

## Phase 1: Foundation (Process Group Isolation & Storage Authority)

- [ ] 1.1 Modify `internal/executor/executor.go:27-52` to add `Setpgid bool` to `executor.Request`.
- [ ] 1.2 Modify `internal/executor/agy.go:193-205` to set `SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` on Linux when `req.Setpgid` is true and signal `-pgid` on timeout.
- [ ] 1.3 Create `internal/stability/store/store.go` and `store_test.go` implementing SQLite/WAL authority at `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:146-185`, `internal/worktree/worktree.go:47-69`) with single-active campaign gate.
- [ ] 1.4 Create `internal/stability/process/process.go` and `process_test.go` implementing process group supervision, `SIGKILL` termination to `-pgid`, and `/proc` survivor verification (`internal/executor/agy.go:19-40`, `internal/executor/agy_test.go:158-191`).

## Phase 2: Evidence, Fixtures & Reconciliation

- [ ] 2.1 Create `internal/stability/evidence/evidence.go`, `receipt.go`, and `evidence_test.go` implementing 4096-byte log sanitization (`internal/run/run.go:71-90`), path stripping, payload hashing, and RFC 8785 JSON Stability Receipt generation.
- [ ] 2.2 Create `internal/stability/fixture/fixture.go` and `fixture_test.go` defining Changes A and B packets, check scripts, out-of-scope defect injection, and deterministic tree hash / commit ancestry verification (`internal/worktree/worktree.go:173-238`, `internal/overlap/overlap.go:21-23`).
- [ ] 2.3 Create `internal/stability/reconcile/reconcile.go` and `reconcile_test.go` implementing fail-closed crash reconciliation, orphan process/worktree detection, and idempotent abort cleanup to `blocked_cleanup` (`internal/worktree/worktree.go:247-269`, `internal/reconcile/reconcile.go:1-33`).

## Phase 3: Campaign Orchestration & State Machine

- [ ] 3.1 Create `internal/stability/campaign.go` and `campaign_test.go` implementing the sequential 3-Trial state machine (`internal/barrier/barrier.go:36-60`, `internal/barrier/barrier_test.go:31-60`), zero-retry reset on slot failure, and timeout budgets (10m dispatch, 45m Trial, 135m Campaign).
- [ ] 3.2 Implement concurrent journey execution in `internal/stability/campaign.go` running Changes A and B concurrently on pinned `gemini-3.7-flash-high` (`internal/executor/agy.go:85-96`, `internal/run/batch.go:66-78`), 10s lease expiration (`internal/feature/feature.go:334-398`), `ErrLeaseHeld` checks (`internal/feature/feature_test.go:278-310`), post-expiry reclaim with monotonic fence (`internal/feature/feature.go:406-473`), and envelope adoption (`internal/run/run.go:48-60`).
- [ ] 3.3 Implement remediation workflow in `internal/stability/campaign.go` managing Defect Record persistence, Test Actor approval gating, Fix Change dispatch to Target A (`internal/integrate/integrate.go:153-175`), and independent Target B fast-forward (`internal/run/attempt.go:740-750`).

## Phase 4: CLI Interface & Preflight Safety Gates

- [ ] 4.1 Create `cmd/lucind-ai/stability.go` and `stability_test.go` implementing preflight admission checks for Linux OS, clean tree (`git status --porcelain`), candidate HEAD build match, baseline check (`internal/integrate/integrate.go:100-120`), and zero active campaigns (`cmd/lucind-ai/cli_test.go:40-60`).
- [ ] 4.2 Implement interactive 15-dispatch confirmation (default no), rejection of non-interactive / release bypass flags (`--yes`, `--tag`, `--push`, `--release`), and full forensic JSON status output in `cmd/lucind-ai/stability.go`.
- [ ] 4.3 Modify `cmd/lucind-ai/cli.go:123-145` to route `stability` subcommand to `stabilityDispatch` in `cmd/lucind-ai/stability.go`.

## Phase 5: End-to-End Campaign Verification

- [ ] 5.1 Implement and execute simulated 3-Trial Campaign verification tests (`internal/run/batch_test.go:26-59`, `internal/integrate/integrate.go:126-138`) verifying defect remediation, Change B crash recovery, zero `/proc` survivors, post-cleanup baseline pass, and immutable RFC 8785 receipt persistence.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Standalone struct field modification on `executor.Request`. |
| 1.2 | 1.1 | Needs `Request.Setpgid` field to configure Linux `SysProcAttr` and signal process groups. |
| 1.3 | — | Standalone storage package establishing SQLite/WAL authority at `<git-common-dir>`. |
| 1.4 | 1.2 | Depends on `Setpgid` process group execution semantics in `executor.Agy`. |
| 2.1 | — | Standalone evidence sanitization and canonical RFC 8785 JSON receipt encoding logic. |
| 2.2 | — | Standalone synthetic journey templates, check scripts, and ancestry verification fixtures. |
| 2.3 | 1.3, 1.4 | Requires store persistence and process supervisor to audit survivors and reconcile state. |
| 3.1 | 1.3, 1.4, 2.1 | Requires store authority, process supervisor, and evidence recorder to manage Trial transitions. |
| 3.2 | 3.1, 2.2 | Requires campaign state machine and journey fixture definitions to orchestrate concurrent runs and 10s lease recovery. |
| 3.3 | 3.2 | Requires concurrent journey harness to inject out-of-scope defects, gate Fix Change, and promote targets. |
| 4.1 | 1.3, 3.1 | Requires store and campaign engine to validate preflight invariants and initialize campaign. |
| 4.2 | 4.1, 2.1 | Requires preflight CLI structure and evidence types to format status JSON and prompt operator. |
| 4.3 | 4.1, 4.2 | Requires stability CLI implementation to wire subcommand routing into main CLI dispatcher. |
| 5.1 | 3.3, 4.3 | Requires complete campaign orchestration, CLI entry points, and post-cleanup verification. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `lane-execution: Process group isolation and termination` | 1.1, 1.2, 1.4 |
| `stability-authority-store: Common-directory SQLite and WAL authority` | 1.3 |
| `stability-authority-store: Single-active campaign constraint` | 1.3 |
| `stability-campaign-state-machine: Sequential three-Trial progression and reset-on-failure` | 3.1 |
| `stability-campaign-state-machine: Execution timeout budgets` | 3.1 |
| `stability-command-contract: CLI preflight admission and safety gates` | 4.1, 4.3 |
| `stability-command-contract: Interactive confirmation without non-interactive bypass` | 4.2 |
| `stability-evidence-receipt: Bounded evidence sanitization` | 2.1 |
| `stability-evidence-receipt: Terminal stability receipt generation` | 2.1, 5.1 |
| `stability-evidence-receipt: Non-mutating delivery boundary` | 4.2, 5.1 |
| `stability-fixture-journey: Concurrent multi-change fixture execution` | 2.2, 3.2 |
| `stability-fixture-journey: Accelerated lease expiry and crash recovery` | 1.4, 3.2 |
| `stability-fixture-journey: Deterministic fixture tree hash and ancestry isolation` | 2.2, 3.3, 5.1 |
| `stability-remediation-flow: Out-of-scope defect detection and recording` | 2.2, 3.3 |
| `stability-remediation-flow: Test Actor gated remediation and resumption` | 3.3 |
| `stability-resume-abort: Fail-closed resume reconciliation` | 2.3 |
| `stability-resume-abort: Idempotent abort and blocked cleanup` | 2.3 |

## Open Questions

- [ ] None.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Subcommand router handling CLI dispatch across subcommands. |
| `cmd/lucind-ai/cli_test.go:40-60` | CLI argument validation and unknown subcommand tests. |
| `internal/barrier/barrier.go:36-60` | Barrier state evaluation across concurrent dispatch lanes. |
| `internal/barrier/barrier_test.go:31-60` | Unit tests for lane state evaluation and release logic. |
| `internal/executor/agy.go:19-40` | Executor constants and type definitions for headless CLI dispatch. |
| `internal/executor/agy.go:85-96` | Model validation enforcing supported executor models. |
| `internal/executor/agy.go:193-205` | Command execution context configuration for subprocess dispatch. |
| `internal/executor/agy_test.go:158-191` | Grandchild process pipe-holding and process group behavior tests. |
| `internal/executor/executor.go:27-52` | Request struct defining per-dispatch configuration parameters. |
| `internal/feature/feature.go:334-398` | Per-feature lease acquisition with monotonic fencing and TTL checks. |
| `internal/feature/feature.go:406-473` | Monotonic lease renewal, fence validation, and expiry handling. |
| `internal/feature/feature_test.go:278-310` | Lease acquisition, ErrLeaseHeld contention, and monotonic fence tests. |
| `internal/integrate/integrate.go:100-120` | Project baseline verification execution via lucind-checks.sh. |
| `internal/integrate/integrate.go:126-138` | Fast-forward promotion and clean working tree verification. |
| `internal/integrate/integrate.go:153-175` | Atomic compare-and-swap ref advancement with git update-ref. |
| `internal/ledger/ledger.go:146-185` | SQLite WAL connection configuration and schema initialization. |
| `internal/ledger/ledger_test.go:35-58` | SQLite database resolution under primary root ledger directory. |
| `internal/ledgerpath/ledgerpath.go:34-58` | Ledger path resolution and validation against repository roots. |
| `internal/ledgerpath/ledgerpath_test.go:9-35` | Ledger path resolution and validation test suite. |
| `internal/overlap/overlap.go:21-23` | Sentinel errors for missing or multiple merge bases. |
| `internal/reconcile/reconcile.go:1-33` | Branch reconciliation sentinel errors and request status types. |
| `internal/run/attempt.go:740-750` | Ref SHA resolution and commit verification routines. |
| `internal/run/batch.go:66-78` | Batch execution initialization and lane barrier construction. |
| `internal/run/batch_test.go:26-59` | Batch test fixtures, packet generation, and envelope stubs. |
| `internal/run/run.go:48-60` | Result directory and envelope schema path constants. |
| `internal/run/run.go:71-90` | Dispatch stream output sanitization cap of 4096 bytes. |
| `internal/worktree/worktree.go:47-69` | DefaultGitRunner interface and implementation for git commands. |
| `internal/worktree/worktree.go:173-238` | Linked worktree creation with parent ref and base SHA validation. |
| `internal/worktree/worktree.go:247-269` | Idempotent worktree cleanup, force removal, and branch deletion. |
