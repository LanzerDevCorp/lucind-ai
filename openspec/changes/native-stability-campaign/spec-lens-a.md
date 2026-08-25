# Spec Lens A — Capabilities & Requirements: Native Stability Campaign

## Assumed requirements

This specification defines 17 requirements across eight capabilities for the Native Stability Campaign change: seven new capabilities (`stability-command-contract` with 2 requirements, `stability-authority-store` with 2 requirements, `stability-campaign-state-machine` with 2 requirements, `stability-fixture-journey` with 3 requirements, `stability-remediation-flow` with 2 requirements, `stability-evidence-receipt` with 3 requirements, and `stability-resume-abort` with 2 requirements) and one existing capability (`lane-execution` gaining 1 requirement). All 17 requirements are classified as ADDED because they introduce new commands, storage authorities, state machines, fixture lifecycles, receipts, and process isolation without modifying existing requirements. The seven new capabilities target full specifications under `openspec/specs/<capability>/spec.md`, while `lane-execution` targets a delta specification under `openspec/changes/native-stability-campaign/specs/lane-execution/spec.md`.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `stability-command-contract` | New | `openspec/specs/stability-command-contract/spec.md` | |
| `stability-authority-store` | New | `openspec/specs/stability-authority-store/spec.md` | |
| `stability-campaign-state-machine` | New | `openspec/specs/stability-campaign-state-machine/spec.md` | |
| `stability-fixture-journey` | New | `openspec/specs/stability-fixture-journey/spec.md` | |
| `stability-remediation-flow` | New | `openspec/specs/stability-remediation-flow/spec.md` | |
| `stability-evidence-receipt` | New | `openspec/specs/stability-evidence-receipt/spec.md` | |
| `stability-resume-abort` | New | `openspec/specs/stability-resume-abort/spec.md` | |
| `lane-execution` | Existing | `openspec/changes/native-stability-campaign/specs/lane-execution/spec.md` | `openspec/specs/lane-execution/spec.md:1` |

## ADDED Requirements

### Requirement: CLI preflight admission and safety gates

The `stability run` command MUST execute a non-mutating preflight validation checking Linux OS, clean checkout (`internal/integrate/integrate.go:126-138`), candidate build matching `HEAD` (`cmd/lucind-ai/cli.go:140-142`), passing baseline check (`internal/integrate/integrate.go:100-120`), and zero active campaigns (`internal/ledger/ledger.go:162-185`) before creating state or worktrees.

**Terminal consumer**: `cmd/lucind-ai/cli.go:123-145` (routing `stability run` invoking `internal/integrate/integrate.go:100-138` and `internal/ledger/ledger.go:162-185`).

### Requirement: Interactive confirmation without non-interactive bypass

The `stability run` command MUST display the plan forecasting 15 model dispatches and require interactive confirmation defaulting to `no` before initializing state, and SHALL NOT support non-interactive bypass flags.

**Terminal consumer**: `cmd/lucind-ai/cli.go:123-145` (command preflight confirmation loop).

### Requirement: Common-directory SQLite and WAL authority

Mutable campaign and trial lifecycle state MUST be stored in SQLite/WAL at `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`), isolated from the primary run ledger at `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:34-58`).

**Terminal consumer**: `internal/ledger/ledger.go:162-185` and `internal/ledgerpath/ledgerpath.go:34-58`.

### Requirement: Single-active campaign constraint

The authority store MUST enforce a single-active campaign gate that rejects initialization of a new campaign whenever an unclosed campaign record exists.

**Terminal consumer**: `internal/ledger/ledger.go:162-185`.

### Requirement: Sequential three-Trial progression and reset-on-failure

A stability campaign MUST execute three sequential Trials without automatic retries; any dispatch failure, crash, or budget exhaustion MUST immediately fail the campaign and reset consecutive pass count to zero.

**Terminal consumer**: `internal/barrier/barrier.go:36-60`.

### Requirement: Execution timeout budgets

Stability execution MUST enforce budgets of 10 minutes per dispatch, 45 minutes per Trial, and 135 minutes per Campaign; exceeding any budget MUST terminate the active dispatch and fail the campaign (`internal/executor/executor.go:57-78`).

**Terminal consumer**: `internal/executor/executor.go:57-78`.

### Requirement: Concurrent multi-change fixture execution

Each Trial MUST create distinct ephemeral integration targets and dispatch Changes A and B concurrently (`internal/run/batch.go:66-70`), ensuring both orchestrators hold active ownership leases and dispatch lanes before promotion begins (`internal/worktree/worktree.go:173-238`).

**Terminal consumer**: `internal/worktree/worktree.go:173-238` and `internal/run/batch.go:66-70`.

### Requirement: Accelerated lease expiry and crash recovery

Abrupt termination of Change B after result persistence (`cmd/lucind-ai/cli.go:710-723`) MUST release ownership only after a 10-second lease TTL; reclaims before expiry MUST return `ErrLeaseHeld` (`internal/feature/feature.go:334-398`), and post-expiry reclaim MUST verify zero `/proc` survivors, adopt the persisted envelope, and promote without redispatch.

**Terminal consumer**: `internal/feature/feature.go:334-398` and `cmd/lucind-ai/cli.go:710-723`.

### Requirement: Deterministic fixture tree hash and ancestry isolation

Target promotions MUST verify Git commit ancestry and fixture digests: Change A target MUST contain only Fix and Change A commits, Change B target MUST contain only Change B commits, and final tree hashes MUST match deterministic fixtures (`internal/integrate/integrate.go:126-138`, `internal/worktree/worktree.go:173-238`).

**Terminal consumer**: `internal/worktree/worktree.go:173-238` and `internal/integrate/integrate.go:126-138`.

### Requirement: Out-of-scope defect detection and recording

When Change A encounters a defect outside its Write Scope during fixture checks (`internal/integrate/integrate.go:100-120`), it MUST persist a Defect Record and halt promotion (`internal/feature/feature.go:98-113`), while Change B continues execution.

**Terminal consumer**: `internal/feature/feature.go:98-113` and `internal/integrate/integrate.go:100-120`.

### Requirement: Test Actor gated remediation and resumption

A dedicated Fix Change MUST be dispatched to rectify the defect (`internal/feature/feature.go:115-130`) and promote to Change A target (`internal/integrate/integrate.go:153-175`); Change A MUST resume under original identity and promote only after Fix dependency is approved by the Test Actor.

**Terminal consumer**: `internal/feature/feature.go:115-130` and `internal/integrate/integrate.go:153-175`.

### Requirement: Bounded evidence sanitization

Evidence captured during campaign execution MUST be sanitized before worktree cleanup (`internal/worktree/worktree.go:247-269`) by capping stream captures to 4096 bytes (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`), stripping absolute paths/credentials, and hashing raw stream payloads.

**Terminal consumer**: `internal/run/run.go:71-90` and `internal/run/run.go:131-150`.

### Requirement: Terminal stability receipt generation

A campaign passing all three sequential Trials and post-cleanup baseline check (`internal/integrate/integrate.go:100-120`) MUST persist an immutable canonical JSON Stability Receipt (RFC 8785) binding candidate commit SHA, build version, fixture digests, Trial records, and pass verdict (`cmd/lucind-ai/cli.go:710-723`).

**Terminal consumer**: `cmd/lucind-ai/cli.go:710-723` and `internal/integrate/integrate.go:100-120`.

### Requirement: Non-mutating delivery boundary

Completion and certification of a Stability Campaign MUST NOT create Git tags, push commits to remotes, mutate release branches, bump semantic versions, or create issue tracker records.

**Terminal consumer**: `cmd/lucind-ai/cli.go:123-145` and `internal/integrate/integrate.go:126-138`.

### Requirement: Fail-closed resume reconciliation

The `stability resume` command MUST reconcile active processes, leases, worktrees, and refs before continuing; any ambiguous or non-deterministic state discrepancy MUST fail closed and prohibit resumption.

**Terminal consumer**: `internal/reconcile/reconcile.go:1-33`.

### Requirement: Idempotent abort and blocked cleanup

The `stability abort` command MUST idempotently terminate processes, release leases, and remove ephemeral worktrees (`internal/worktree/worktree.go:247-269`); unremovable residue MUST transition the campaign to `blocked_cleanup` without redispatching tasks.

**Terminal consumer**: `internal/reconcile/reconcile.go:1-33` and `internal/worktree/worktree.go:247-269`.

### Requirement: Process group isolation and termination

Lane execution on Linux MUST configure child processes with a dedicated process group (`Setpgid: true`), and termination on timeout or cancellation MUST signal the entire process group (`-pgid`) to ensure no surviving child or grandchild processes remain (`internal/executor/agy.go:193-205`).

**Terminal consumer**: `internal/executor/agy.go:193-205`.

## Open Questions

- [ ] Should `internal/executor/agy.go` configure Linux process-group isolation (`Setpgid: true`) unconditionally on Linux, or should `executor.Request` introduce an optional configuration field?
- [ ] How to resolve `<git-common-dir>` portably across checkouts using `worktree.GitRunner`?
- [ ] SDD phase process precedence: this draft follows packet instructions for three-lens capability decomposition; full spec generation under `specs/` is deferred to the synthesizer.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | CLI subcommand switch routing commands `run`, `check`, `feature`, and `reconcile` |
| `cmd/lucind-ai/cli.go:140-142` | Build version and runtime platform information emitted by version flags |
| `cmd/lucind-ai/cli.go:710-723` | `PersistEnvelope` writes result envelope JSON files to `.lucind/results/` |
| `internal/barrier/barrier.go:36-60` | `Evaluate` computes batch release outcome from observed lane states |
| `internal/executor/agy.go:193-205` | `exec.CommandContext` configures and executes the `agy` child process |
| `internal/executor/executor.go:57-78` | `Outcome` struct definition capturing exit codes, timeouts, and stream truncation |
| `internal/feature/feature.go:98-113` | `ValidateParentRef` enforces parent branch ref validation rules |
| `internal/feature/feature.go:115-130` | `Create` registers a new feature record and transitions status to active |
| `internal/feature/feature.go:334-398` | `AcquireLease` enforces expiring feature leases and monotonic fencing |
| `internal/integrate/integrate.go:100-120` | `Check` executes `lucind-checks.sh` to determine repository verification status |
| `internal/integrate/integrate.go:126-138` | `Promote` checks working tree cleanliness via `git status --porcelain` before merging |
| `internal/integrate/integrate.go:153-175` | `PromoteCAS` validates parent ref and atomically updates target ref |
| `internal/ledger/ledger.go:162-185` | `openPath` initializes SQLite database connection with WAL pragma and connection pool limits |
| `internal/ledgerpath/ledgerpath.go:34-58` | `Resolve` and `Validate` compute and verify SQLite database file paths |
| `internal/reconcile/reconcile.go:1-33` | Package comment and sentinel error definitions for reconciliation lifecycle recovery |
| `internal/run/batch.go:66-70` | `ExecuteBatch` coordinates parallel lane execution and barrier synchronization |
| `internal/run/run.go:71-90` | `streamDetailCap` defines the 4096-byte boundary for captured output streams |
| `internal/run/run.go:131-150` | `diagnosisDetail` and `formatStreamDetail` truncate and format captured stream output |
| `internal/worktree/worktree.go:173-238` | `CreateWithParent` creates linked git worktrees with branch and base commit verification |
| `internal/worktree/worktree.go:247-269` | `Cleanup`, `Remove`, and `DeleteBranch` worktree and branch deletion routines |
| `openspec/specs/lane-execution/spec.md:1` | Live capability specification for `lane-execution` exists |
