# Proposal Lens C — Risks, Rollback & Test Impact: Native Stability Campaign

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| **Shared Executor Blast Radius**: Modifying `internal/executor/agy.go:193-205` with `Setpgid: true` risks breaking ordinary dispatches (`internal/run/run.go:205-229`, `internal/run/batch.go:66-89`). | High | Encapsulate process supervision under `internal/stability/process`, guard Linux syscalls, and test stubs (`internal/executor/agy_test.go:28-50`, `internal/executor/agy_test.go:158-191`). | `internal/executor/agy.go:193-205` |
| **Grandchild MCP Process Leaks**: Dispatched `agy` processes spawn MCP subprocesses (`internal/executor/agy.go:19-40`, `internal/executor/agy_test.go:158-191`) surviving abrupt kills. | High | Set `Setpgid: true`, kill negative PID (`-pgid`), and verify zero `/proc` survivors before Trial close. | `internal/executor/agy.go:19-40` |
| **Git Common-Dir SQLite/WAL Inconsistency**: Coordinator crash mid-Trial leaves uncheckpointed WAL files under `<git-common-dir>/lucind-ai/stability/v1/` (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:34-38`). | High | Open SQLite with WAL and busy timeout (`internal/ledger/ledger.go:162-185`), enforcing single-active constraints and idempotent recovery. | `internal/ledger/ledger.go:162-185` |
| **10s Lease Race & Premature Takeover**: Clock skew could allow premature Orchestrator B reclaim before 10s TTL expiry (`internal/feature/feature.go:331-374`, `internal/run/attempt.go:298-328`). | High | Atomic SQLite update returns `ErrLeaseHeld` on early takeover (`internal/feature/feature.go:351-373`) and increments fence (`internal/feature/feature_test.go:278-310`). | `internal/feature/feature.go:351-373` |
| **Secret & Local Path Leakage in Receipts**: Raw streams containing secrets or paths leak into receipts (`internal/run/run.go:71-90`, `cmd/lucind-ai/cli.go:708-723`). | Medium | Bounded sanitization (`streamDetailCap = 4096`) strips `/home/...` paths (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`) and stores payload SHA-256 digests. | `internal/run/run.go:71-90` |
| **Target Contamination & Deadlock**: Concurrent Changes A and B with Fix could contaminate targets or deadlock gates (`internal/barrier/barrier.go:36-60`, `internal/overlap/overlap.go:21-33`, `internal/run/attempt.go:740-760`). | Medium | Distinct targets (`internal/worktree/worktree.go:173-238`), ancestry checks via `merge-base --is-ancestor` (`internal/worktree/worktree.go:203-209`), and in-memory gates (`internal/barrier/barrier.go:36-60`). | `internal/worktree/worktree.go:203-209` |
| **High Quota Consumption & Timeouts**: 15 dispatches pinned to `gemini-3.7-flash-high` across 3 Trials without auto-retries (Q64, Q86); overruns abort campaigns. | Medium | Preflight forecasts 15 dispatches with confirmation (`cmd/lucind-ai/cli.go:246-250`) and strict 10m/45m/135m timeouts (`internal/executor/executor.go:57-78`). | `cmd/lucind-ai/cli.go:246-250` |
| **Residual Worktree/Ref Blocking**: Interrupted campaigns leave temporary worktrees, refs, or leases (`internal/worktree/worktree.go:247-269`, `internal/feature/feature.go:477-510`). | Medium | Transition to `blocked_cleanup`; `lucind-ai stability abort` idempotently purges residue without AI dispatches (`internal/worktree/worktree.go:247-269`). | `internal/worktree/worktree.go:247-269` |

## Rollback & Additivity

**Rollback Plan**: Rollback of `internal/stability` and the `lucind-ai stability` CLI routing (`cmd/lucind-ai/cli.go:123-145`) is purely code-level via `git revert`. No database down-migrations are required; historical common-dir data in `<git-common-dir>/lucind-ai/stability/v1/` remains isolated and inert. Primary repository checkout remains clean and unmodified (`internal/integrate/integrate.go:127-138`).

**Additivity**: Storage changes are strictly additive. The existing Run ledger at `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledger/ledger.go:146-168`) is NOT modified, migrated, or deleted; Trial Records reference Run IDs read-only. Stability Campaign authority is isolated in `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-58`). Terminal Stability Receipts are standalone content-addressed JSON files, preserving `.lucind/results/` (`cmd/lucind-ai/cli.go:708-723`).

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| **Domain State Machine & Transition Tests** | Strict TDD for Campaign/Trial state transitions (`preflight -> running -> failed/passed/blocked_cleanup`), counter increments, reset-on-failure, idempotency, and timeouts (`internal/lane/status_test.go:1-32`). | `internal/barrier/barrier_test.go:31-60` |
| **Storage & SQLite/WAL Tests** | Validate schema initialization, forward-only migrations, unique-active constraint, and WAL crash reopening under common-dir paths without mutating `<primaryRoot>/.lucind/lucind.db`. | `internal/ledger/ledger_test.go:35-58` |
| **Path Resolution & Boundary Tests** | Verify stability store path resolution targets `<git-common-dir>/lucind-ai/stability/v1/` and rejects invalid candidate paths. | `internal/ledgerpath/ledgerpath_test.go:9-35` |
| **Linux Process Lifecycle Tests** | Test process-group launch (`Setpgid: true`), abrupt kill (`SIGKILL`), `/proc` survivor inspection, and asserting survivor leaks fail Trial close using stubs (`writeStub`). | `internal/executor/agy_test.go:158-191` |
| **10s Lease & Fencing Tests** | Verify lease acquisition, rejection of early reclaim at t < 10s (`ErrLeaseHeld`), takeover at t >= 10s with monotonic fence increment, and clock skew safety. | `internal/feature/feature_test.go:278-310` |
| **Fixture Journey & Ancestry Tests** | Test deterministic fixture generation, seeded defect detection, Remediation Proposal, Fix promotion, and Git ancestry verification (`merge-base --is-ancestor`). | `internal/worktree/worktree.go:203-209` |
| **Worktree & Branch Cleanup Tests** | Validate idempotent creation, removal, and cleanup of linked worktrees and ephemeral branches without residue. | `internal/worktree/worktree.go:247-269` |
| **Evidence Sanitization & Receipt Tests** | Assert stripping of credentials and `/home/...` paths from logs (`streamDetailCap = 4096`), SHA-256 payload hashing, RFC 8785 canonical JSON, and digest verification. | `internal/run/run.go:71-90` |
| **CLI Preflight & Status JSON Tests** | Test `stability run\|status\|resume\|abort`, clean checkout check (`git status --porcelain`), stale binary rejection, interactive confirmation, and status JSON schema. | `cmd/lucind-ai/cli_test.go:40-60` |
| **End-to-End Fake Journey Tests** | Full 3-Trial simulated Campaign using fake executor (`writeStub`), verifying B crash, 10s lease expiry, reclaim, Fix promotion, A resumption, and receipt creation. | `internal/run/batch_test.go:26-59` |
| **Native Baseline & Real Acceptance** | Validate canonical `lucind-ai check` (`integrate.Check`) before/after Campaign (`cmd/lucind-ai/cli.go:501-508`); run manual real 3-Trial Campaign (15 `agy` dispatches with `gemini-3.7-flash-high`) outside `go test ./...`. | `internal/integrate/integrate.go:100-119` |

## Out of Scope

- Non-Linux OS for mutating execution (Linux-only in V1 per Q53).
- Non-interactive bypasses, `--yes`, NIP, or secret storage (Q29, Q33).
- External issue trackers or remote repository mutation (Q18, Q54).
- Alternative AI executors (pinned to `agy` / `gemini-3.7-flash-high` per Q80, Q86).
- Ordinary Run ledger migration at `<primary-root>/.lucind/lucind.db` (Q67).
- Automatic AI retries or dynamic timeout / lease configuration (Q50, Q64, Q65).
- Automated git tagging, version bumping, release publishing, or pushing (Q54).
- Control Room UI views for Stability Campaigns (Q77).
- Candidate selection, technical approach, and conceptual changes (Lens A).
- Capability impact table, delta specification requirements, and scenarios (Lens B).

## Open Questions

- [ ] Procedural precedence conflict: `~/.claude/skills/sdd-propose/SKILL.md` specifies a single subagent writing a complete `proposal.md` under a 450-word budget with Engram persistence; this packet supersedes those mechanics by splitting the proposal phase into three parallel lenses (Lens A, Lens B, Lens C) writing to `openspec/changes/native-stability-campaign/propose-lens-*.md` under a 1000-word budget with filesystem-only persistence.
- [ ] Should `internal/stability/process` provide an optional process-group supervisor wrapper around `internal/executor/agy.go:193-205` or should `executor.Agy` be extended with an optional `Setpgid` configuration field to avoid modifying the shared default execution path?
- [ ] How will `internal/stability/store` resolve `<git-common-dir>` portably across primary checkouts and linked worktrees using `worktree.GitRunner` (`internal/worktree/worktree.go:173-238`)?
- [ ] Should `stability status --json` output full Trial Record bodies or compact summary references?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | routes top-level subcommands in the CLI switch block |
| `cmd/lucind-ai/cli.go:246-250` | validates requested model against known executor models |
| `cmd/lucind-ai/cli.go:501-508` | executes integrate.Check during baseline verification |
| `cmd/lucind-ai/cli.go:708-723` | persists result envelope JSON to primary repository results directory |
| `cmd/lucind-ai/cli_test.go:40-60` | tests CLI subcommand routing and failure paths without subprocess dispatch |
| `internal/barrier/barrier.go:36-60` | evaluates lane states in memory to decide batch release and preservation |
| `internal/barrier/barrier_test.go:31-60` | tests pure barrier evaluation across terminal and running lane states |
| `internal/executor/agy.go:19-40` | documents MCP grandchild processes and WaitDelay pipe-drain tradeoffs |
| `internal/executor/agy.go:193-205` | constructs and executes child process without process-group configuration |
| `internal/executor/agy_test.go:28-50` | verifies successful zero-exit execution using executable stubs |
| `internal/executor/agy_test.go:158-191` | verifies WaitDelay pipe-drain behavior when grandchildren background and hold pipes open |
| `internal/executor/executor.go:57-78` | defines Outcome struct capturing exit codes, timeouts, and output truncation |
| `internal/feature/feature.go:331-374` | acquires expiring lease with monotonic fence token using conditional SQLite update |
| `internal/feature/feature.go:351-373` | returns ErrLeaseHeld when attempting to acquire an active unexpired lease |
| `internal/feature/feature.go:477-510` | releases active lease by setting expiration to zero |
| `internal/feature/feature_test.go:278-310` | verifies initial lease acquisition, rejection of concurrent acquisition, and monotonic fence increment after expiration |
| `internal/integrate/integrate.go:100-119` | executes lucind-checks.sh script for repository baseline verification |
| `internal/integrate/integrate.go:127-138` | verifies clean working tree via git status porcelain before promoting |
| `internal/lane/status_test.go:1-32` | verifies terminal and validity properties across lane status enum values |
| `internal/ledger/ledger.go:146-168` | opens SQLite database connection at resolved primary root path |
| `internal/ledger/ledger.go:162-185` | configures SQLite connection pool with WAL pragma and busy timeout |
| `internal/ledger/ledger_test.go:35-58` | verifies ledger database initialization under primary root .lucind directory |
| `internal/ledgerpath/ledgerpath.go:34-38` | resolves canonical database path for primary repository root |
| `internal/ledgerpath/ledgerpath.go:40-58` | validates candidate ledger paths against primary repository root boundary |
| `internal/ledgerpath/ledgerpath_test.go:9-35` | tests canonical ledger path resolution for primary root |
| `internal/overlap/overlap.go:21-33` | defines merge base error conditions and classification classes |
| `internal/run/attempt.go:298-328` | manages attempt lease acquisition and state transitions to leased |
| `internal/run/attempt.go:740-760` | evaluates feature parent refs and base SHAs for overlap gating |
| `internal/run/batch.go:66-89` | runs concurrent batch lanes bounded by sync.WaitGroup |
| `internal/run/batch_test.go:26-59` | provides helper constructors for batch packets and minimal test result envelopes |
| `internal/run/run.go:71-90` | defines stream detail cap bounding stdout and stderr storage |
| `internal/run/run.go:131-150` | formats bounded diagnostic details for stdout and stderr stream tails |
| `internal/run/run.go:205-229` | defines execution dependencies including envelope persistence and lease settings |
| `internal/worktree/worktree.go:173-238` | creates linked git worktree off specified parent ref and base SHA |
| `internal/worktree/worktree.go:203-209` | verifies base SHA is an ancestor of parent ref via git merge-base |
| `internal/worktree/worktree.go:247-269` | removes linked worktree and deletes associated branch idempotently |
