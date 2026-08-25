# Tasks Lens C — Proof & Review Burden: Native Stability Campaign

## Assumed decomposition

The change decomposes into four sequential work units across 11 files (3 modified, 8 created): Unit 1 delivers process-group execution (`internal/executor/`) and isolated `<git-common-dir>` SQLite authority (`internal/stability/store/`); Unit 2 implements the 3-Trial zero-retry state machine (`internal/stability/`) and deterministic fixture journeys (`internal/stability/fixture/`); Unit 3 adds 4096B log sanitization, RFC 8785 JSON receipts (`internal/stability/evidence/`), and fail-closed crash reconciliation (`internal/stability/reconcile/`); Unit 4 wires the `stability run|status|resume|abort` CLI commands and preflight gates (`cmd/lucind-ai/`). The critical path runs through Unit 1 (executor & store) → Unit 2 (state machine & fixture) → Unit 3 (evidence & recovery) → Unit 4 (CLI preflight & E2E certification).

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 2,100–3,100 lines across 11 files (3 modified, 8 created packages/tests) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Executor & Store) → PR 2 (State Machine & Fixture) → PR 3 (Evidence & Reconcile) → PR 4 (CLI & E2E Wiring) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

Basis for estimate:
- `cmd/lucind-ai/cli.go:123-145`, `stability.go`, `stability_test.go`: ~400–550 lines based on CLI subcommands (`cmd/lucind-ai/cli_test.go:40-68`).
- `internal/executor/` (`executor.go:27-52`, `agy.go:193-205`): ~30–50 lines for process attributes.
- `internal/stability/campaign.go` & tests: ~350–500 lines based on `internal/barrier/barrier.go:36-60` and `internal/barrier/barrier_test.go:31-60`.
- `internal/stability/store/` & tests: ~400–550 lines based on `internal/ledger/ledger.go:146-185` and `internal/ledgerpath/ledgerpath.go:34-58`.
- `internal/stability/fixture/` & tests: ~300–450 lines based on `internal/worktree/worktree.go:173-238`.
- `internal/stability/process/` & tests: ~250–350 lines based on `internal/executor/agy_test.go:158-191`.
- `internal/stability/evidence/` & tests: ~350–500 lines based on `internal/run/run.go:71-90` and `internal/integrate/integrate.go:100-120`.
- `internal/stability/reconcile/` & tests: ~300–400 lines based on `internal/worktree/worktree.go:247-269` and `internal/reconcile/reconcile.go:1-33`.
- Total diff: ~2,100–3,100 lines, exceeding 400 lines and requiring chained PRs.

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths (`requirements.txt`, `CMakeLists.txt`, Markdown, `README.sh`) | N/A | None (N/A) | N/A: No path classification boundary in synthetic templates | None (N/A) |
| Git repository selection (`git -C`, relative/absolute paths) | Applicable | `TestPreflightRejectsNonGitWorkingDir` | Non-git repo aborts preflight non-zero without state creation (`cmd/lucind-ai/cli_test.go:40-68`) | Implement `cmd/lucind-ai/stability.go` repo validation (`cmd/lucind-ai/cli.go:123-145`) |
| Git repository selection (`git -C`, relative/absolute paths) | Applicable | `TestPreflightResolvesGitCommonDirAuthority` | Resolves authority DB under `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledgerpath/ledgerpath_test.go:9-35`, `internal/worktree/worktree.go:173-238`) | Implement `internal/stability/store/store.go` common-dir resolution (`internal/ledger/ledger.go:146-185`) |
| Commit state (staged, `commit -a`, empty index) | Applicable | `TestPreflightRejectsDirtyWorkingTreeStaged` | Staged changes abort preflight with exit code 1 (`cmd/lucind-ai/cli_test.go:40-68`, `internal/integrate/integrate.go:126-138`) | Implement `cmd/lucind-ai/stability.go` preflight porcelain check (`cmd/lucind-ai/cli.go:123-145`) |
| Commit state (staged, `commit -a`, empty index) | Applicable | `TestPreflightRejectsDirtyWorkingTreeUntracked` | Untracked files abort preflight with exit code 1 (`cmd/lucind-ai/cli_test.go:40-68`, `internal/integrate/integrate.go:126-138`) | Implement `cmd/lucind-ai/stability.go` preflight porcelain check (`cmd/lucind-ai/cli.go:123-145`) |
| Commit state (staged, `commit -a`, empty index) | Applicable | `TestPreflightRejectsDirtyWorkingTreeModified` | Unstaged changes abort preflight with exit code 1 (`cmd/lucind-ai/cli_test.go:40-68`, `internal/integrate/integrate.go:126-138`) | Implement `cmd/lucind-ai/stability.go` preflight porcelain check (`cmd/lucind-ai/cli.go:123-145`) |
| Push state (tracking branch, first push, refspec) | N/A | None (N/A) | N/A: Non-goal (Q54, Q370); no git push operations | None (N/A) |
| PR commands (`--head`, env prefix, composed commands) | N/A | None (N/A) | N/A: Non-goal (Q18, Q54, Q370); no PR commands created | None (N/A) |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Process-group isolation (`internal/executor/`) | `go test -v ./internal/executor -run TestAgyLinuxSetpgidSysProcAttr` (derived from `internal/executor/agy_test.go:158-191`) | `Request.Setpgid: true` configures Linux `SysProcAttr.Setpgid` (`internal/executor/agy.go:193-205`) | Grandchild process killing under `SIGKILL` without `internal/stability/process` |
| Common-dir SQLite store (`internal/stability/store/`) | `go test -v ./internal/stability/store -run TestStoreSingleActiveGateAndCommonDirResolution` (derived from `internal/ledger/ledger_test.go:35-58`, `internal/ledgerpath/ledgerpath_test.go:9-35`) | Store resolves `<git-common-dir>` and rejects concurrent campaigns (`internal/ledger/ledger.go:146-185`) | Uncheckpointed WAL durability across OS power failure |
| Process kill & `/proc` audit (`internal/stability/process/`) | `go test -v ./internal/stability/process -run TestProcessGroupKillAndProcSurvivorAudit` (derived from `internal/executor/agy_test.go:158-191`) | `SIGKILL` to `-pgid` terminates descendants and audit finds 0 survivors (`internal/executor/agy.go:19-40`) | Non-Linux execution (scoped to Linux) |
| 3-Trial state machine (`internal/stability/`) | `go test -v ./internal/stability -run TestCampaignSequentialThreeTrialsAndZeroRetryReset` (derived from `internal/barrier/barrier_test.go:31-60`) | 3 sequential passes, 10m/45m/135m budgets, 0-retry failure reset (`internal/barrier/barrier.go:36-60`) | Live backend network model latency or throttling |
| Fixture journey & defect (`internal/stability/fixture/`) | `go test -v ./internal/stability/fixture -run TestFixtureConcurrentJourneyDefectAndAncestryIsolation` (derived from `internal/feature/feature_test.go:278-310`, `internal/integrate/integrate_test.go:49-100`) | Concurrent A/B journeys, Fix Change remediation, target ancestry isolation (`internal/worktree/worktree.go:173-238`) | Merge conflict resolution outside fixture templates |
| Evidence sanitization & receipt (`internal/stability/evidence/`) | `go test -v ./internal/stability/evidence -run TestEvidenceSanitizationAndCanonicalReceiptRFC8785` (derived from `internal/run/run.go:71-90`, `internal/integrate/integrate.go:100-120`) | 4096B stream cap, path stripping, raw payload hashing, RFC 8785 receipt (`internal/run/run.go:131-150`) | Cryptographic notary signature attestation |
| Fail-closed crash recovery (`internal/stability/reconcile/`) | `go test -v ./internal/stability/reconcile -run TestReconcileFailClosedAmbiguityAndIdempotentAbort` (derived from `internal/worktree/worktree_test.go:16-58`, `internal/reconcile/reconcile.go:1-33`) | Ambiguity triggers `blocked_cleanup`; `abort` idempotently removes residues (`internal/worktree/worktree.go:247-269`) | Cleanup when files locked by immutable root OS permissions |
| CLI preflight & E2E simulation (`cmd/lucind-ai/`) | `go test -v ./cmd/lucind-ai -run TestStabilityRunPreflightAndSimulatedThreeTrialRun` (derived from `cmd/lucind-ai/cli_test.go:40-68`, `internal/run/batch_test.go:26-59`) | Subcommand parsing, clean tree preflight, non-interactive rejection, simulated 3-Trial run (`cmd/lucind-ai/cli.go:123-145`) | Replaces required live `lucind-ai stability run` with `gemini-3.7-flash-high` |

## Verification Gaps

- **Real-world live acceptance**: In-process test doubles cannot execute live `agy` dispatches with `gemini-3.7-flash-high` across real network endpoints; final acceptance requires running `lucind-ai stability run` manually outside `go test ./...` in an authenticated Linux environment (`docs/adr/0001-native-stability-campaign.md:5-24`).
- **Non-Linux execution**: Cross-platform execution on macOS/Windows is not proven because v1 is Linux-only (`docs/adr/0001-native-stability-campaign.md:5-24`).

## Open Questions

- [ ] None.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Subcommand router dispatching CLI commands in run() |
| `cmd/lucind-ai/cli.go:503-509` | Check dispatch invoking integrate.Check baseline validation |
| `cmd/lucind-ai/cli.go:710-723` | PersistEnvelope writing result envelopes under .lucind/results/ |
| `cmd/lucind-ai/cli_test.go:40-68` | Test seams verifying CLI argument parsing and failure exits |
| `docs/adr/0001-native-stability-campaign.md:5-24` | Accepted ADR defining native 3-Trial stability campaign contract |
| `internal/barrier/barrier.go:36-60` | Evaluate function assessing lane statuses for barrier outcome |
| `internal/barrier/barrier_test.go:31-60` | Unit test verifying lane state evaluation and release transitions |
| `internal/executor/agy.go:19-40` | Default wait delay bounding stdio pipe draining for child and grandchild processes |
| `internal/executor/agy.go:85-96` | Pinned default model gemini-3.7-flash-high and known models allow-list |
| `internal/executor/agy.go:193-205` | Exec command creation and process attribute configuration |
| `internal/executor/agy_test.go:158-191` | Test seam verifying grandchild processes holding pipes open past wait delay |
| `internal/executor/executor.go:27-52` | Request struct definition configuring execution parameters |
| `internal/feature/feature.go:98-113` | ValidateParentRef helper enforcing parent branch constraints |
| `internal/feature/feature.go:115-130` | Create method registering feature records and immutability checks |
| `internal/feature/feature.go:334-398` | AcquireLease acquiring expiring ownership leases with monotonic fencing |
| `internal/feature/feature.go:406-473` | RenewLease extending active unexpired leases with fence validation |
| `internal/feature/feature_test.go:278-310` | Unit test verifying lease acquisition, expiry, and monotonic fence increments |
| `internal/integrate/integrate.go:100-120` | Check executing lucind-checks.sh for baseline verification |
| `internal/integrate/integrate.go:126-138` | Promote verifying clean working tree via git status --porcelain |
| `internal/integrate/integrate_test.go:49-100` | Integration test combining lane branches into a linked worktree |
| `internal/ledger/ledger.go:146-185` | Open and openAtPath configuring SQLite WAL connection pool |
| `internal/ledger/ledger_test.go:35-58` | Test proving database creation under primary repo .lucind directory |
| `internal/ledgerpath/ledgerpath.go:34-58` | Resolve and Validate path helpers enforcing primary root storage |
| `internal/ledgerpath/ledgerpath_test.go:9-35` | Unit test verifying primary root ledger path resolution |
| `internal/ledgerpath/ledgerpath_test.go:37-58` | Unit test verifying validation and rejection of paths outside primary repo |
| `internal/reconcile/reconcile.go:1-33` | Reconcile package definition and sentinel errors |
| `internal/run/batch.go:66-78` | ExecuteBatch building barriers and dispatching lane batches |
| `internal/run/batch_test.go:26-59` | Batch packet and result envelope double fixtures |
| `internal/run/run.go:56-60` | Constant defining resultEnvelopePath as .lucind/result.json |
| `internal/run/run.go:71-90` | Constant streamDetailCap bounding diagnostic stream logs to 4096 bytes |
| `internal/run/run.go:131-150` | Stream truncation and formatting functions for diagnostic details |
| `internal/worktree/worktree.go:173-238` | Linked worktree creation and commit SHA ancestry verification |
| `internal/worktree/worktree.go:247-269` | Cleanup, Remove, and DeleteBranch functions for worktree management |
| `internal/worktree/worktree_test.go:16-58` | Unit test detecting linked worktrees via .git pointer verification |
| `lucind-ai-stability-run-sdd-master-plan.md:240-252` | Architecture master plan defining suggested stability subpackages |
| `openspec/changes/native-stability-campaign/design.md:121-127` | Threat matrix defining boundary applicability and planned RED tests |
