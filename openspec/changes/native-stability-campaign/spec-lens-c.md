# Spec Lens C — Live-Spec Conflicts & Migration: Native Stability Campaign

## Assumed requirements

The Native Stability Campaign change touches 8 capabilities: 7 new capability domains (`stability-command-contract`, `stability-authority-store`, `stability-campaign-state-machine`, `stability-fixture-journey`, `stability-remediation-flow`, `stability-evidence-receipt`, and `stability-resume-abort`) and 1 modified capability domain (`lane-execution`). The new capabilities introduce preflight admission, single-active common-dir SQLite/WAL persistence, sequential three-Trial execution with non-retryable 5-dispatch journeys, synthetic fixtures with remediation gates, sanitized evidence receipts, and fail-closed resume/abort reconciliation. The modified capability `lane-execution` is expected to assert Linux process-group isolation (`Setpgid: true`), negative-PID signal delivery (`-pgid`), and zero `/proc` survivor verification without altering existing approval-wait gates, barrier observation ordering, schema additivity, or metadata persistence.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `lane-execution` | `openspec/specs/lane-execution/spec.md:1-84` | 4 | 9 | Yes (process group isolation in `internal/executor/agy.go:193-205`) |

The other 7 capabilities listed in the proposal (`stability-command-contract`, `stability-authority-store`, `stability-campaign-state-machine`, `stability-fixture-journey`, `stability-remediation-flow`, `stability-evidence-receipt`, and `stability-resume-abort`) are newly introduced capability domains with no pre-existing live specifications under `openspec/specs/`.

## Conflicts

None. The Native Stability Campaign does not contradict or invalidate any guarantees established by live specifications. Specifically:

1. `openspec/specs/lane-execution/spec.md:10-26` (`Gate Placement in the Lifecycle`): Requires approval wait to run after status computation and resolve before ledger persistence. The stability campaign does not alter approval wait timing or ledger persistence order.
2. `openspec/specs/lane-execution/spec.md:27-43` (`Resolve Before Barrier Observation`): Requires approval wait to resolve before batch barrier observation. Stability campaigns use independent in-memory evaluation (`internal/barrier/barrier.go:36-60`) and do not modify batch barrier observation semantics for standard lane execution.
3. `openspec/specs/lane-execution/spec.md:44-62` (`Additive Schema, Unchanged Enum`): Requires approval records to use additive storage without adding a 7th enum value to `lane.Status`. Stability campaigns maintain isolated SQLite storage under `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`) and preserve the 6-value `lane.Status` enum unmodified.
4. `openspec/specs/lane-execution/spec.md:63-84` (`Lane metadata dispatch persistence`): Requires lane registration to persist packet and routing metadata. Stability trial dispatches continue persisting metadata.

The executor modification setting `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on Linux (`internal/executor/agy.go:193-205`) introduces new subprocess group isolation and lifecycle supervision behavior. This is an ADDED requirement within the `lane-execution` delta specification rather than a modification of existing live requirements.

## MODIFIED Full Blocks

None. No existing live requirements in `openspec/specs/lane-execution/spec.md` (or any other live specification) are modified or superseded by this change. All four live requirements remain intact. The process-group isolation and child supervision capability represents an ADDED requirement in the `lane-execution` delta specification.

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| None | N/A | N/A | N/A | N/A |

No requirements are removed or renamed by this change. All existing live requirements across `openspec/specs/` remain intact and active with zero migration required.

## Open Questions

- [ ] Procedural precedence conflict: `~/.claude/skills/sdd-spec/SKILL.md` describes a single sub-agent writing full delta specs under `openspec/changes/{change-name}/specs/` with Engram persistence; this packet supersedes those mechanics by splitting the spec phase into three parallel lenses (Lens A, Lens B, Lens C) writing to `openspec/changes/native-stability-campaign/spec-lens-*.md` under a 1000-word budget with filesystem-only persistence.
- [ ] Should `internal/executor/agy.go:193-205` configure Linux process-group isolation (`Setpgid: true`) unconditionally on Linux, or should `executor.Request` introduce an optional configuration field to limit blast radius on ordinary dispatches?
- [ ] How will `internal/stability/store` resolve `<git-common-dir>` portably across primary checkouts and linked worktrees using `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`, `internal/worktree/worktree.go:173-238`)?
- [ ] Should `internal/stability/reconcile` reuse primitives from `internal/reconcile/` (`internal/reconcile/reconcile.go:1-33`) or maintain dedicated cleanup semantics?
- [ ] Should `stability status --json` output full Trial Record bodies or compact summary references?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | routes top-level subcommands in the CLI switch block |
| `cmd/lucind-ai/cli.go:710-723` | persists result envelope JSON to primary repository results directory |
| `internal/barrier/barrier.go:36-60` | evaluates lane states in memory to decide batch release and progression |
| `internal/executor/agy.go:19-40` | documents MCP grandchild processes and WaitDelay pipe-drain tradeoffs |
| `internal/executor/agy.go:193-205` | constructs and executes child process without process-group configuration |
| `internal/executor/executor.go:57-78` | defines Outcome struct capturing exit codes, timeouts, and output truncation |
| `internal/feature/feature.go:98-113` | registers feature branches and initializes worktree structures |
| `internal/feature/feature.go:334-398` | acquires expiring lease with monotonic fence token using conditional SQLite update |
| `internal/feature/feature.go:406-473` | reclaims expired lease and verifies state before promoting |
| `internal/integrate/integrate.go:100-120` | executes lucind-checks.sh script for repository baseline verification |
| `internal/integrate/integrate.go:126-138` | verifies clean working tree via git status porcelain before promoting |
| `internal/ledger/ledger.go:146-148` | opens SQLite database connection at resolved primary root path |
| `internal/ledger/ledger.go:162-185` | configures SQLite connection pool with WAL pragma and busy timeout |
| `internal/ledgerpath/ledgerpath.go:34-38` | resolves canonical database path for primary repository root |
| `internal/ledgerpath/ledgerpath.go:40-58` | validates candidate ledger paths against primary repository root boundary |
| `internal/reconcile/reconcile.go:1-33` | defines reconciliation state types and inspect logic |
| `internal/run/batch.go:66-70` | runs concurrent batch lanes bounded by sync.WaitGroup |
| `internal/run/run.go:48-60` | handles result persistence and exit codes |
| `internal/run/run.go:71-90` | defines stream detail cap bounding stdout and stderr storage |
| `internal/run/run.go:131-150` | formats bounded diagnostic details for stdout and stderr stream tails |
| `internal/worktree/worktree.go:47-69` | executes git commands via GitRunner helper methods |
| `internal/worktree/worktree.go:173-238` | creates linked git worktree off specified parent ref and base SHA |
| `internal/worktree/worktree.go:247-269` | removes linked worktree and deletes associated branch idempotently |
| `openspec/specs/lane-execution/spec.md:1-84` | complete live lane-execution specification |
| `openspec/specs/lane-execution/spec.md:10-26` | specifies lifecycle placement requirement for approval wait gate |
| `openspec/specs/lane-execution/spec.md:27-43` | specifies barrier observation ordering requirement for approval wait gate |
| `openspec/specs/lane-execution/spec.md:44-62` | specifies additive schema and unchanged six-value lane.Status enum requirement |
| `openspec/specs/lane-execution/spec.md:63-84` | specifies lane metadata dispatch persistence requirement |
