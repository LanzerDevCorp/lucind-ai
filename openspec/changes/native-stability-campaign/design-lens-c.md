# Design Lens C — Failure, Test & Rollback: Native Stability Campaign

## Assumed architecture

The design adopts Candidate 2 (Modular Subpackage Decomposition under `internal/stability/`): `store`, `fixture`, `process`, `evidence`, and `reconcile`. Isolated SQLite/WAL authority lives at `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`) with a single-active gate. Extends `cmd/lucind-ai/cli.go:123-145` (`stability run|status|resume|abort`) and `internal/executor/agy.go:193-205` (`Setpgid: true`), reusing linked worktrees (`internal/worktree/worktree.go:173-238`) and baseline checks (`internal/integrate/integrate.go:100-148`). Primary Run storage at `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledger/ledger.go:146-148`) remains untouched and referenced read-only.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit | State transitions & timeouts | State machine transitions (`preflight->running->passed/failed/blocked_cleanup`), counter (0..3), reset, and timeouts. | `internal/barrier/barrier_test.go:31-60` |
| Unit | Storage schema & single-active gate | SQLite/WAL schema init, migrations, single-active gate, and crash reopening in common-dir. | `internal/ledger/ledger_test.go:35-58` |
| Unit | Path resolution & rejection | Resolve `<git-common-dir>/lucind-ai/stability/v1/stability.db` and reject paths outside common-dir. | `internal/ledgerpath/ledgerpath_test.go:9-35` |
| Unit | 10s lease & monotonic fence | 10s TTL, early reclaim rejection (`ErrLeaseHeld`), takeover at t>=10s, fence increment. | `internal/feature/feature_test.go:278-310` |
| Unit | Evidence sanitization & JSON receipt | Strip paths (`streamDetailCap = 4096`), SHA-256 hashes, RFC 8785 JSON. | `internal/run/run.go:71-90` |
| Integration | Process group & `/proc` survivor audit | Test `Setpgid: true`, `SIGKILL` to `-pgid`, and survivor detection using stubs (`writeStub`). | `internal/executor/agy_test.go:158-191` |
| Integration | Fixture defect & ancestry proof | Test seeded defect, Remediation Proposal, Fix integration, and ancestry (`merge-base --is-ancestor`). | `internal/worktree/worktree.go:203-209` |
| Integration | Worktree & branch cleanup | Test idempotent removal of worktrees/branches, asserting `blocked_cleanup` on residue. | `internal/worktree/worktree.go:247-269` |
| Integration | CLI preflight & status JSON | Test porcelain check, candidate build match, non-interactive rejection, status schema. | `cmd/lucind-ai/cli_test.go:40-60` |
| E2E | 3-Trial simulated Campaign | 3-Trial simulated Campaign verifying crash, lease, Fix, and receipt via `writeStub`. | `internal/run/batch_test.go:26-59` |
| E2E | Native baseline & real acceptance | Run baseline checks (`integrate.Check`); real 3-Trial Campaign outside `go test ./...`. | `internal/integrate/integrate.go:100-120` |

## Test Seams

### Existing Seams
- `executor.Agy.Binary` / `writeStub` (`internal/executor/agy_test.go:18-26`, `internal/executor/agy.go:193-205`): Executable script test doubles for subprocess execution.
- `worktree.GitRunner` (`internal/worktree/worktree.go:173-238`, `internal/integrate/integrate.go:153-180`): Git subprocess abstraction for faking commands and CAS promotions.
- `run.Deps` (`internal/run/run.go:205-229`): Injects `LookupExecutor`, `RunChecks`, `CombineTree`, `PromoteTarget`, `PersistEnvelope`.
- `integrate.Check` / `integrate.Promote` (`internal/integrate/integrate.go:100-138`): Baseline verification and porcelain status validation.
- `ledger.Open` / `openAtPath` (`internal/ledger/ledger.go:162-185`): SQLite WAL pragmas and connection pooling.

### New Seams Required
- Storage constructor (`new seam required in internal/stability/store`): Injects base path for isolated `stability.db` testing.
- Process supervisor seam (`new seam required in internal/stability/process`): Controls `Setpgid: true`, `SIGKILL` to `-pgid`, and `/proc` audit.
- Monotonic test clock (`new seam required in internal/stability`): Injects mock clock for 10s lease and timeout testing without sleeps.
- Test Actor decision recorder (`new seam required in internal/stability/fixture`): Deterministic mock for Remediation and Promotion gates.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: Campaign executes embedded synthetic templates and invokes agy CLI directly without file-path classification. | N/A — no classification boundary | None (N/A) |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | Resolves Git common directory via `git rev-parse --git-common-dir`; rejects non-repo directories and path arguments. | `TestPreflightRejectsNonGitWorkingDir`, `TestPreflightResolvesGitCommonDirAuthority` |
| Commit state | staged, `commit -a`, empty index | Applicable | Preflight runs `git status --porcelain` before mutation; aborts on staged, unstaged, or untracked changes. | `TestPreflightRejectsDirtyWorkingTreeStaged`, `TestPreflightRejectsDirtyWorkingTreeUntracked`, `TestPreflightRejectsDirtyWorkingTreeModified` |
| Push state | tracking branch, first push, explicit refspec | N/A: V1 non-goal (Q54, Q370); stability campaigns perform no git push or remote ref updates. | N/A — no push boundary | None (N/A) |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: V1 non-goal (Q18, Q54, Q370); stability campaigns create no pull requests or forge commands. | N/A — no PR boundary | None (N/A) |

### Expected Safe & Failure Behaviors
- **Git repository selection**: Safe: resolves authority in `<git-common-dir>/lucind-ai/stability/v1/stability.db`. Failure: non-git directory aborts preflight non-zero without file creation.
- **Commit state**: Safe: executes only when `git status --porcelain` is empty. Failure: any uncommitted modifications abort preflight with exit code 1.

## Rollback and Additivity

**Choice**: `git revert` of commits adding `internal/stability/` and `cmd/lucind-ai/cli.go:123-145` routing.

**Alternatives considered**:
- *Database down-migrations on stability storage*: Rejected; common-dir storage is isolated and down-migrations risk deleting audit history.
- *Primary ledger schema migrations*: Rejected; modifying `<primaryRoot>/.lucind/lucind.db` breaks additivity.

**Rationale**: Reverting code eliminates subcommands and packages. Format changes are strictly additive:
1. Primary ledger at `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledger/ledger.go:146-148`) is untouched; Trial Records link Run IDs read-only.
2. Stability state lives exclusively in `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-58`); historical data stays inert on rollback.
3. Receipts in `.lucind/results/` (`cmd/lucind-ai/cli.go:710-723`) are standalone JSON files.
4. No existing schema, ledger, or envelope versions move.

## Out of Scope

- Non-Linux OS mutating execution (Linux-only in V1 per Q53).
- Non-interactive bypasses, `--yes` flag, NIP, or secret storage (Q29, Q33).
- External issue creation, GitHub PRs, remote push, or releases (Q18, Q54, Q370).
- Alternative AI executors or model tuning (pinned to `gemini-3.7-flash-high` per Q80, Q86).
- Primary Run ledger migration at `<primary-root>/.lucind/lucind.db` (Q67).
- Automatic AI retries or dynamic lease/timeout tuning (Q50, Q64, Q65).
- Control Room UI views (Q77).
- Technical approach & architecture decisions (Lens A).
- File-changes table & signatures (Lens B).

## Open Questions

- [ ] Precedence conflict: `~/.claude/skills/sdd-design/SKILL.md` (single subagent, 800 words, Engram) is superseded by three parallel lenses writing `design-lens-*.md` (1000 words, filesystem persistence).
- [ ] Process Boundary Gap: Threat matrix reference (`references/threat-matrix.md`) covers VCS/PR/classification; process supervision (`Setpgid: true`, `SIGKILL`, `/proc` audit, lease race) requires dedicated tasks.
- [ ] Common-Dir Seam: Should `internal/stability/store` resolve common directory via `worktree.GitRunner` (`internal/worktree/worktree.go:179-191`) or path helper?
- [ ] Process Isolation Scope: Should `internal/executor/agy.go:193-205` configure `Setpgid: true` unconditionally on Linux or via `executor.Request` field?
- [ ] Status JSON Detail: Should `stability status --json` output full Trial Record bodies or compact summaries?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | routes top-level subcommands in the CLI switch block |
| `cmd/lucind-ai/cli.go:246-258` | validates requested model against known executor models |
| `cmd/lucind-ai/cli.go:503-509` | executes integrate.Check during baseline verification |
| `cmd/lucind-ai/cli.go:710-723` | persists result envelope JSON to primary repository results directory |
| `cmd/lucind-ai/cli_test.go:40-60` | tests CLI subcommand routing and failure paths without subprocess dispatch |
| `docs/adr/0001-native-stability-campaign.md:5-24` | records architectural decision for native stability campaigns and 3-trial validation |
| `internal/barrier/barrier.go:36-60` | evaluates lane states in memory to decide batch release and preservation |
| `internal/barrier/barrier_test.go:31-60` | tests pure barrier evaluation across terminal and running lane states |
| `internal/executor/agy.go:19-40` | documents MCP grandchild processes and WaitDelay pipe-drain tradeoffs |
| `internal/executor/agy.go:85-96` | defines default and known models for agy executor |
| `internal/executor/agy.go:193-205` | constructs and executes child process without process-group configuration |
| `internal/executor/agy_test.go:18-26` | implements writeStub helper for executable script test doubles |
| `internal/executor/agy_test.go:28-50` | verifies successful zero-exit execution using executable stubs |
| `internal/executor/agy_test.go:158-191` | verifies WaitDelay pipe-drain behavior when grandchildren background and hold pipes open |
| `internal/executor/executor.go:57-78` | defines Outcome struct capturing exit codes, timeouts, and output truncation |
| `internal/feature/feature.go:98-113` | validates feature parent ref format and namespace restrictions |
| `internal/feature/feature.go:334-398` | acquires expiring lease with monotonic fence token using conditional SQLite update |
| `internal/feature/feature.go:406-473` | renews and releases active feature leases in SQLite ledger |
| `internal/feature/feature_test.go:278-310` | verifies initial lease acquisition, rejection of concurrent acquisition, and monotonic fence increment after expiration |
| `internal/integrate/integrate.go:100-120` | executes lucind-checks.sh script for repository baseline verification |
| `internal/integrate/integrate.go:126-138` | verifies clean working tree via git status porcelain before promoting |
| `internal/integrate/integrate.go:153-180` | promotes ref using compare-and-swap update-ref with GitRunner |
| `internal/lane/status_test.go:1-31` | verifies terminal and validity properties across lane status enum values |
| `internal/ledger/ledger.go:146-148` | opens SQLite database connection at resolved primary root path |
| `internal/ledger/ledger.go:162-185` | configures SQLite connection pool with WAL pragma and busy timeout |
| `internal/ledger/ledger_test.go:35-58` | verifies ledger database initialization under primary root .lucind directory |
| `internal/ledgerpath/ledgerpath.go:34-38` | resolves canonical database path for primary repository root |
| `internal/ledgerpath/ledgerpath.go:40-58` | validates candidate ledger paths against primary repository root boundary |
| `internal/ledgerpath/ledgerpath_test.go:9-35` | tests canonical ledger path resolution for primary root |
| `internal/overlap/overlap.go:21-23` | defines merge base error conditions for git overlap evaluation |
| `internal/reconcile/reconcile.go:1-33` | manages reconciliation records and sentinel error definitions |
| `internal/run/attempt.go:298-328` | manages attempt lease acquisition and state transitions to leased |
| `internal/run/attempt.go:735-765` | evaluates feature parent refs and base SHAs for overlap gating |
| `internal/run/batch.go:66-78` | runs concurrent batch lanes bounded by sync.WaitGroup |
| `internal/run/batch_test.go:26-59` | provides helper constructors for batch packets and minimal test result envelopes |
| `internal/run/run.go:48-60` | defines result envelope and schema paths relative to lucind directory |
| `internal/run/run.go:71-90` | defines stream detail cap bounding stdout and stderr storage |
| `internal/run/run.go:131-150` | formats bounded diagnostic details for stdout and stderr stream tails |
| `internal/run/run.go:205-229` | defines execution dependencies including envelope persistence and lease settings |
| `internal/worktree/worktree.go:173-238` | creates linked git worktree off specified parent ref and base SHA |
| `internal/worktree/worktree.go:203-209` | verifies base SHA is an ancestor of parent ref via git merge-base |
| `internal/worktree/worktree.go:247-269` | removes linked worktree and deletes associated branch idempotently |
| `lucind-ai-stability-run-sdd-master-plan.md:7-16` | specifies public CLI subcommands and core stability outcomes |
| `lucind-ai-stability-run-sdd-master-plan.md:59-73` | defines 3-trial canonical journey, crash injection, and recovery rules |
| `lucind-ai-stability-run-sdd-master-plan.md:141-153` | defines common-dir storage authority and lifecycle invariants |
| `lucind-ai-stability-run-sdd-master-plan.md:358-371` | lists explicit non-goals for stability campaign V1 |
