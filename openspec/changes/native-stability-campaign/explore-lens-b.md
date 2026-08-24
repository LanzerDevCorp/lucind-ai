# Explore Lens B — Capabilities & Scenarios: Native Stability Campaign

## User & Capability Impact

The Native Stability Campaign provides deterministic, native release certification for `lucind-ai` on Linux.

### Affected Personas & Operator Surfaces
- **Release Engineers & Maintainers**: Use dedicated subcommands (`stability run`, `stability status [--json]`, `stability resume`, `stability abort`) dispatched via `cmd/lucind-ai/cli.go:123-145` alongside existing CLI routes (`cmd/lucind-ai/cli.go:56-57`). Interactive preflight validates candidate `HEAD`, clean working tree (`cmd/lucind-ai/cli.go:1577-1580`), binary identity, baseline checks (`cmd/lucind-ai/cli.go:501-508`), and `agy` quotas (`cmd/lucind-ai/cli.go:355-357`), requiring explicit interactive confirmation defaulting to `no` (`cmd/lucind-ai/cli.go:158-164`).
- **Autonomous Orchestrators & Agents**: Operate within isolated linked worktrees (`internal/worktree/worktree.go:155-159`) driven by packet contracts (`internal/executor/executor.go:27-52`). Each Trial executes five non-retryable dispatches (`internal/executor/agy.go:193-205`) pinned to `gemini-3.7-flash-high` (`internal/executor/agy.go:86-88`, `internal/executor/agy.go:94-96`, `cmd/lucind-ai/cli.go:246-250`).
- **Auditors & Automation**: Read-only `stability status --json` reports campaign lifecycle, trial progress, and residue.

### Storage & Architectural Seams
- **Storage Isolation**: Mutable state resides under `<git-common-dir>/lucind-ai/stability/v1/` via SQLite/WAL (`internal/ledger/ledger.go:162-164`), separate from the primary Run ledger at `<primary-root>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-148`).
- **Receipt Boundary**: Passing campaigns emit an immutable, content-addressed Stability Receipt without tagging, pushing, branching, or releasing.

## Scenarios & Use Cases

### Scenario 1 — Preflight Rejection on Dirty Working Tree

- **Context**: Operator executes `lucind-ai stability run` with uncommitted, tracked, or untracked changes.
- **Action**: Operator invokes `lucind-ai stability run` (`cmd/lucind-ai/cli.go:123-145`).
- **Outcome**: Preflight rejects execution before creating database rows, worktrees (`internal/worktree/worktree.go:155-159`), or dispatches (`internal/run/run.go:362-369`), reporting the dirty paths and exiting non-zero.

### Scenario 2 — Preflight Confirmation and Interactive Admission

- **Context**: Clean Linux primary repository (`cmd/lucind-ai/cli.go:1577-1580`), matching candidate binary, passing `lucind-ai check` (`cmd/lucind-ai/cli.go:501-508`), and authenticated `agy` (`cmd/lucind-ai/cli.go:65-70`).
- **Action**: Operator runs `lucind-ai stability run`, inspects the preflight forecast, and types `yes` (`cmd/lucind-ai/cli.go:158-164`).
- **Outcome**: Candidate `HEAD` is re-validated, a single active Campaign is initialized in SQLite (`internal/ledger/ledger.go:162-164`), and Trial 1 begins without CI or non-interactive bypass.

### Scenario 3 — Concurrent Change Dispatch and Defect Assessment

- **Context**: Trial 1 initiates changes A and B with disjoint temporary integration targets (`internal/feature/feature.go:101-113`, `internal/worktree/worktree.go:78-81`).
- **Action**: Both Orchestrators acquire leases and dispatch lanes (`internal/run/run.go:362-369`). Change A encounters a pre-seeded defect outside its Write Scope during fixture checks.
- **Outcome**: Temporal overlap is established (`internal/barrier/barrier.go:36-47`). Change A records a Defect Record and Remediation Proposal; Test Actor approves Fix Change creation; Change A blocks on Fix while Change B continues (`internal/barrier/barrier.go:49-59`).

### Scenario 4 — Orchestrator Abrupt Crash, Lease Expiry, and Reclaim

- **Context**: Change B's lane persists its result envelope (`internal/run/run.go:56-60`) in `.lucind` (`internal/run/run.go:48-50`) before acceptance.
- **Action**: System abruptly terminates Change B's `agy` process (`internal/executor/agy.go:193-205`). Replacement Orchestrator attempts immediate acquisition.
- **Outcome**: Takeover is blocked while the 10-second lease is active (`internal/feature/feature.go:309-322`). Following expiry, replacement Orchestrator reclaims ownership, verifies the saved envelope, verifies zero surviving child processes (`internal/executor/agy.go:40-40`), and promotes B before Fix completes.

### Scenario 5 — Independent Fix Promotion and Resumption of Change A

- **Context**: Fix Change completes isolated fixture edits required by Change A.
- **Action**: Fix Change promotes to Change A's target (`internal/feature/feature.go:118-128`, `internal/integrate/integrate.go:60-64`), satisfying Change A's dependency.
- **Outcome**: Change A resumes under its original Orchestrator identity via a new `agy` dispatch (`internal/executor/executor.go:27-52`), passes fixture checks, and promotes. Git ancestry verifies target isolation without cross-contamination (`internal/run/attempt.go:743-747`, `internal/overlap/overlap.go:21-23`).

### Scenario 6 — Three-Trial Success and Failure Reset

- **Context**: Campaign requires three consecutive successful Trials for certification.
- **Action**: Trials 1 and 2 pass. Trial 3 encounters a failed slot result (`internal/executor/executor.go:57-61`).
- **Outcome**: Consecutive count resets to zero. Campaign terminates as `failed`, cleans operational state, and halts subsequent trials.

### Scenario 7 — Abort Handling and Blocked Cleanup Recovery

- **Context**: A running Campaign is interrupted or leaves residual worktrees (`cmd/lucind-ai/cli.go:470-479`).
- **Action**: Operator inspects state via `lucind-ai stability status` and executes `lucind-ai stability abort`.
- **Outcome**: System enters `blocked_cleanup` if residue remains, idempotently removes temporary worktrees/leases with zero AI dispatches, saves logs, and closes the campaign.

### Scenario 8 — Receipt Generation and Status Observation

- **Context**: All three Trials pass and post-cleanup `lucind-ai check` succeeds (`cmd/lucind-ai/cli.go:501-508`).
- **Action**: Operator queries certification via `lucind-ai stability status --json`.
- **Outcome**: A canonical content-addressed JSON Stability Receipt is persisted under common-dir authority, binding commit SHA, build, versions, environment, and trial evidence. Status command outputs JSON confirmation.

## Success Criteria

- [ ] `lucind-ai stability` CLI provides `run`, `status [--json]`, `resume`, and `abort` subcommands.
- [ ] Preflight halts on non-Linux OS, dirty checkout, mismatched binary, failed `lucind-ai check`, or unavailable `agy`/model.
- [ ] Mutating execution requires explicit interactive confirmation defaulting to `no`.
- [ ] Campaign executes up to three sequential Trials, resetting consecutive count to zero on any single failure.
- [ ] Each Trial executes concurrent changes A and B, records Defect Records upon pre-seeded defect detection, and creates independent Fix Changes.
- [ ] Abrupt crash of Orchestrator B preserves envelope without duplicate work and requires ten-second lease expiration before reclaim.
- [ ] Target isolation is proven by Git ancestry (Fix+A on A's target, B on B's target).
- [ ] Surviving descendant processes fail Trial validation.
- [ ] Campaign authority is isolated in SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/` without mutating ordinary run ledger.
- [ ] Passed campaign emits a canonical content-addressed JSON Stability Receipt without tagging, pushing, or releasing.

## Open Questions

- [ ] Should `stability status --json` output full Trial Record bodies or compact summary references?
- [ ] SDD Fan-out partitioning: This Lens B artifact focuses strictly on capabilities and scenarios; synthesis wave will consolidate Lens A (approaches) and Lens C (risks) into canonical `explore.md`.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56-57` | Defines CLI usage string listing available top-level subcommands |
| `cmd/lucind-ai/cli.go:65-70` | Maps supported executor names including agy and reject unknown executors |
| `cmd/lucind-ai/cli.go:123-145` | Routes CLI subcommands in main dispatch switch |
| `cmd/lucind-ai/cli.go:158-164` | Parses run flags and options for lane dispatch |
| `cmd/lucind-ai/cli.go:246-250` | Validates requested model against executor's KnownModels |
| `cmd/lucind-ai/cli.go:355-357` | Prints multi-lane concurrent quota burn forecast |
| `cmd/lucind-ai/cli.go:470-479` | Defines check subcommand interface and flag parsing |
| `cmd/lucind-ai/cli.go:501-508` | Executes integrate.Check to run mechanical repository validation |
| `cmd/lucind-ai/cli.go:1577-1580` | Guards commands by refusing execution from inside linked worktrees |
| `internal/barrier/barrier.go:36-47` | Evaluates lane batch releasing only when all expected lanes reach terminal state |
| `internal/barrier/barrier.go:49-59` | Partitions completed lanes into integrate and preserve sets |
| `internal/executor/agy.go:40-40` | Sets default 5-second wait delay for stdio pipe drain |
| `internal/executor/agy.go:86-88` | Returns gemini-3.7-flash-high as default model for agy |
| `internal/executor/agy.go:94-96` | Defines known models list for agy executor |
| `internal/executor/agy.go:103-109` | Configures non-interactive flags accept-edits and dangerously-skip-permissions |
| `internal/executor/agy.go:193-205` | Spawns and executes child agy process with captured output |
| `internal/executor/executor.go:27-52` | Defines Request structure for dispatching prompts to executor |
| `internal/executor/executor.go:57-61` | Defines Outcome structure capturing process exit code and timeout |
| `internal/feature/feature.go:101-113` | Validates parent ref rejecting empty, main, or temp branch names |
| `internal/feature/feature.go:118-128` | Creates feature record enforcing immutable parent ref and base SHA |
| `internal/feature/feature.go:309-322` | Enforces lease expiration before granting new feature lease |
| `internal/integrate/integrate.go:60-64` | Creates integration worktree branched from declared parent ref |
| `internal/ledger/ledger.go:146-148` | Opens SQLite database in primary repository .lucind directory |
| `internal/ledger/ledger.go:162-164` | Configures WAL journal mode and busy timeout on ledger connection |
| `internal/overlap/overlap.go:21-23` | Defines error returned when no common merge base is found |
| `internal/run/attempt.go:743-747` | Ignores ErrNoMergeBase during overlap gate evaluation |
| `internal/run/run.go:48-50` | Defines .lucind directory constant for worktree coordination |
| `internal/run/run.go:56-60` | Defines expected path for agent result envelope json |
| `internal/run/run.go:362-369` | Executes batch of lane dispatches concurrently |
| `internal/worktree/worktree.go:78-81` | Generates standardized branch name for lane worktree |
| `internal/worktree/worktree.go:155-159` | Computes target sibling path for linked worktree directory |
| `internal/worktree/worktree.go:175-177` | Creates linked worktree initialized at specified parent and base SHA |
