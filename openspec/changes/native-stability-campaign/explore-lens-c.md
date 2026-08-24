# Explore Lens C — Risks, Trade-offs & Spikes: Native Stability Campaign

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| Orphaned process leakage during abrupt crash injection | High | Assign isolated process group (`Setpgid: true`), terminate via negative PID (`-pgid`), and inspect `/proc` for child survivors before trial cleanup. | `internal/executor/agy.go:20-40`, `internal/executor/agy.go:193-205` |
| Common-dir SQLite WAL lock contention and crash recovery | High | Deploy dedicated SQLite database with WAL and `busy_timeout=5000` under git common-dir; enforce single-active-campaign constraint via transactional gate. | `internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:36-38`, `internal/ledgerpath/ledgerpath.go:40-59` |
| Ten-second lease race condition and premature takeover | High | Enforce atomic monotonic fencing updates (`expires_at <= now`) and reject early reclaim before expiry with `ErrLeaseHeld`. | `internal/feature/feature.go:283-324`, `internal/feature/feature.go:476-510`, `internal/run/attempt.go:298-328` |
| Environment or credential leakage in long-lived receipts | Medium | Sanitize output streams by redacting home directories, usernames, and secret tokens; persist bounded logs and SHA-256 payload hashes. | `internal/run/run.go:71-90`, `internal/run/run.go:131-150` |
| Accidental overlap gating deadlock between concurrent fixture lanes | Medium | Configure disjoint fixture paths and distinct integration targets so `evaluateOverlapGate` does not trigger blocking reconciliation. | `internal/integrate/integrate.go:60-91`, `internal/overlap/overlap.go:26-42`, `internal/run/attempt.go:743-747` |
| Residual worktree or branch state blocking consecutive trials | Medium | Verify worktree and ref destruction before trial advancement; transition to `blocked_cleanup` if residue remains. | `internal/barrier/barrier.go:36-60`, `internal/worktree/worktree.go:155-159`, `internal/worktree/worktree.go:247-269` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| Process group (`Setpgid`) vs Cgroups v2 for kill isolation | Standard POSIX API across Linux distros; requires no root privileges or systemd integration. | Does not contain processes that explicitly create new sessions (`setsid`). | Low: purely standard library `syscall` and `/proc` tree inspection. |
| Common-dir storage vs Root `.lucind/` directory | Shared authority across linked worktrees; survives worktree deletion; isolates stability from run ledger. | Requires `git rev-parse --git-common-dir` resolution instead of local path joining. | Low: resolved once per command invocation during store initialization. |
| Host-side abrupt kill vs In-process agent signaling | Deterministically tests crash recovery after result persistence and before acceptance without agent race. | Requires coordinator-level process supervision and PID lifecycle tracking. | Low: executed directly by launcher coordinator after envelope write. |
| Fixed 10s lease TTL vs Dynamic configurable lease | Fast, deterministic test execution; strict adherence to approved decision Q50 without configuration drift. | Requires monotonic time checks to prevent premature takeover on loaded test hosts. | Minimal: 10s deterministic delay during crashed Orchestrator recovery. |
| Content-addressed JSON receipt vs SQLite blob storage | Fully transparent, verifiable via external tools (`sha256sum`), immutable, human-inspectable. | Requires strict canonical JSON formatting (RFC 8785) and dual file/database write paths. | Low: emitted once per passing Campaign upon terminal verification. |

## Potential Spikes / Proof of Concepts

### Spike 1: Linux Process-Group Termination & Descendant Survivor Detection
- **Objective**: Validate that setting `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on headless `agy` commands (extending `internal/executor/agy.go:193-205`) allows clean `syscall.Kill(-pgid, syscall.SIGKILL)` termination without leaving orphaned MCP server subprocesses (referenced in `internal/executor/agy.go:20-40`).
- **Seam**: `internal/executor/agy.go:193-205`.

### Spike 2: Git Common-Dir SQLite Authority & Crash Recovery
- **Objective**: Validate creating `<git-common-dir>/lucind-ai/stability/v1/stability.db` using WAL mode and connection pooling (extending `internal/ledger/ledger.go:162-185` and `internal/ledgerpath/ledgerpath.go:36-38`), verifying single-active-campaign constraints and clean WAL replay following unexpected process termination.
- **Seam**: `internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-59`.

### Spike 3: Ten-Second Lease Expiry & Explicit Reclaim Protocol
- **Objective**: Implement isolated unit tests modeling the 10-second lease lifecycle using SQLite conditional updates (`internal/feature/feature.go:283-324`, `internal/run/attempt.go:298-328`), proving early reclaim at t=5s fails with `ErrLeaseHeld` while reclaim at t=11s succeeds and increments fence.
- **Seam**: `internal/feature/feature.go:283-324`, `internal/feature/feature.go:476-510`.

### Spike 4: Bounded Evidence Sanitization & Canonical Receipt Hashing
- **Objective**: Benchmark redaction pipelines for stripping absolute paths (`/home/...`) and environment variables from captured logs (extending `internal/run/run.go:71-90`, `internal/run/run.go:131-150`), and verify SHA-256 canonical JSON digest generation for terminal receipts.
- **Seam**: `internal/run/run.go:71-90`, `internal/run/run.go:131-150`, `internal/run/run.go:708-721`.

## Out of Scope

- Non-Linux platforms for mutating Campaign execution (Linux-only in V1 per Q53).
- Non-interactive bypasses, `--yes` flags, NIP, or password/secret storage (Q29, Q33).
- External issue trackers (GitHub issues, Jira) or remote repository mutation (Q18, Q54).
- Multi-model or alternative executor support (strictly pinned to `agy` with `gemini-3.7-flash-high` per Q80, Q86).
- Migration of the existing Run ledger at `<primary-root>/.lucind/lucind.db` (Q67).
- Automatic AI dispatch retries or dynamic timeout / lease configuration (Q50, Q64, Q65).
- Automated git tagging, version bumping, release publishing, or branch pushing (Q54).
- Control Room UI views or web server integration for Campaign management (Q77).

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:124-137` | CLI subcommand switch routes run, split, check, serve, feature, reconcile, and worktree commands |
| `internal/barrier/barrier.go:36-60` | Evaluate evaluates expected versus observed lane states to release integration or preservation |
| `internal/executor/agy.go:20-40` | defaultWaitDelay bounds stdio drain time for MCP grandchild processes |
| `internal/executor/agy.go:193-205` | runFormat constructs exec.CommandContext without process group assignment or descendant tracking |
| `internal/feature/feature.go:98-113` | ValidateParentRef rejects empty, main, or lucind temporary namespace refs |
| `internal/feature/feature.go:283-324` | AcquireLease inserts or updates feature lease with atomic timestamp comparison |
| `internal/feature/feature.go:476-510` | ValidateLease checks whether owner and fence hold an unexpired lease |
| `internal/integrate/integrate.go:60-91` | combine creates temporary worktree with parent and merges lane branches |
| `internal/integrate/integrate.go:100-120` | Check executes lucind-checks.sh at worktree root to verify baseline |
| `internal/integrate/integrate.go:153-183` | PromoteCAS performs atomic compare-and-swap update of parent ref via git update-ref |
| `internal/ledger/ledger.go:162-185` | openAtPath configures SQLite with WAL journal mode, busy timeout, and connection limits |
| `internal/ledgerpath/ledgerpath.go:36-38` | Resolve returns primary root database path under .lucind/lucind.db |
| `internal/ledgerpath/ledgerpath.go:40-59` | Validate rejects database candidate paths outside the primary repository .lucind directory |
| `internal/overlap/overlap.go:26-42` | Class represents overlap classification outcomes including required, warning, and informational |
| `internal/run/attempt.go:298-328` | ExecuteAttempt acquires feature lease and transitions attempt to leased or blocked |
| `internal/run/attempt.go:743-747` | ErrNoMergeBase makes evaluateOverlapGate continue without blocking |
| `internal/run/run.go:48-60` | lucindDir defines .lucind directory and resultEnvelopePath defines .lucind/result.json |
| `internal/run/run.go:71-90` | streamDetailCap bounds captured output per stream to 4096 bytes in ledger notes |
| `internal/run/run.go:131-150` | diagnosisDetail and formatStreamDetail format bounded stream tails for diagnostic events |
| `internal/run/run.go:447-460` | decideStatus determines terminal status from outcome before checking allowed paths and completion mode |
| `internal/run/run.go:708-721` | PersistEnvelope writes result envelope JSON to primary repository results directory |
| `internal/worktree/worktree.go:155-159` | pathFor computes sibling worktree directory path using worktreesDirSuffix |
| `internal/worktree/worktree.go:173-238` | CreateWithParent creates linked git worktree branching from resolved base SHA |
| `internal/worktree/worktree.go:247-269` | Cleanup, Remove, and DeleteBranch provide idempotent worktree and branch deletion |
| `internal/worktree/worktree.go:278-292` | IsLinkedWorktree checks gitdir prefix to identify linked worktrees |
