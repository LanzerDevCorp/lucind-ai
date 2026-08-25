# Design Lens A — Decisions: Native Stability Campaign

## Assumed architecture

This change extends `cmd/lucind-ai/cli.go` with stability subcommands (`run`, `status`, `resume`, `abort`), extends `internal/executor/executor.go` (`Request`) and `internal/executor/agy.go` with process-group isolation, and introduces new modular subpackages under `internal/stability/` (`store`, `fixture`, `process`, `evidence`, `reconcile`). State authority is stored in SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/stability.db` independently of the primary Run ledger, while existing `internal/worktree/`, `internal/feature/`, and `internal/integrate/` packages are reused for worktree lifecycle, lease fencing, and baseline checks.

## Technical Approach

We introduce native release certification via `lucind-ai stability run|status|resume|abort` (`cmd/lucind-ai/cli.go:123-145`, `docs/adr/0001-native-stability-campaign.md:5-24`), designed directly against frozen `proposal.md` while sibling specs synthesize in parallel. The engine executes three sequential 5-dispatch Trials pinned to `gemini-3.7-flash-high` with zero retries. Ephemeral targets execute Change A and B concurrently (`internal/run/batch.go:66-78`); Change A hits an out-of-scope defect and emits a Remediation Proposal for Fix Change dispatch, while Change B is killed via `SIGKILL`, verified for zero `/proc` survivors, and recovered via 10s lease reclaim (`internal/feature/feature.go:334-398`, `internal/feature/feature.go:406-473`) and envelope adoption (`internal/run/run.go:56-60`). Campaign authority is isolated in `<git-common-dir>` SQLite/WAL, and passing runs emit an immutable RFC 8785 JSON receipt after passing a post-cleanup baseline check (`internal/integrate/integrate.go:100-120`).

## Decision 1 — Modular Subpackage Architecture under internal/stability/

**Choice**: Subpackages under `internal/stability/` (`store`, `fixture`, `process`, `evidence`, `reconcile`) coordinated by a root engine. CLI parses and formats only.
**Alternatives considered**: Monolithic `internal/stability`; scattering across top-level packages (`internal/store/`, `internal/fixture/`).
**Rationale**: Modular subpackages isolate storage, process inspection, fixtures, sanitization, and recovery (`lucind-ai-stability-run-sdd-master-plan.md:240-252`). Keeps `cmd/lucind-ai/cli.go:123-145` thin per repository conventions.
**Terminal consumer**: CLI router in `cmd/lucind-ai/cli.go:123-145` calling `internal/stability` methods.

## Decision 2 — Optional Process-Group Isolation via executor.Request

**Choice**: Add optional `Setpgid bool` to `executor.Request` (`internal/executor/executor.go:27-52`); set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` in `internal/executor/agy.go:193-205` on Linux when true.
**Alternatives considered**: Setting `Setpgid: true` unconditionally on Linux; creating a duplicate stability-only executor.
**Rationale**: Unconditional process-group changes create blast radius for general batch execution (`internal/run/batch.go:66-78`). An explicit `executor.Request` field (`internal/executor/executor.go:27-52`) confines scope while allowing `internal/stability/process` to contain and kill grandchild MCP processes (`internal/executor/agy_test.go:158-191`).
**Terminal consumer**: `internal/executor/agy.go:193-205` configuring child processes from `executor.Request` (`internal/executor/executor.go:27-52`).

## Decision 3 — Portable Common-Dir Path Resolution via worktree.GitRunner

**Choice**: Resolve common Git directory via `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`) calling `git rev-parse --git-common-dir`, normalizing relative paths against worktree root.
**Alternatives considered**: Parsing `.git` pointer files; stripping `-worktrees` suffixes; reusing `internal/ledgerpath/ledgerpath.go:34-58`.
**Rationale**: In linked worktrees (`internal/worktree/worktree.go:173-238`), `.git` points to git metadata. Git's `--git-common-dir` is authoritative. `internal/ledgerpath/ledgerpath.go:34-58` explicitly rejects paths outside `<primaryRoot>/.lucind/` and cannot resolve worktree common dirs.
**Terminal consumer**: `internal/stability/store` path resolution and `internal/worktree/worktree.go:173-238`.

## Decision 4 — Dedicated Common-Dir SQLite Authority and Single-Active Gate

**Choice**: Store mutable Campaign state in SQLite/WAL at `<git-common-dir>/lucind-ai/stability/v1/stability.db` with connection pool matching `internal/ledger/ledger.go:146-185`, enforcing single-active campaign via conditional transaction.
**Alternatives considered**: Storing in primary Run ledger at `<primaryRoot>/.lucind/lucind.db`; using flat JSON files.
**Rationale**: Campaigns span multiple Trials, worktrees, and targets. Common-dir persistence survives checkout changes while leaving `<primaryRoot>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-185`, `internal/ledgerpath/ledgerpath.go:34-58`) untouched and read-only (`docs/adr/0001-native-stability-campaign.md:20-24`).
**Terminal consumer**: `internal/stability/store` single-active gate in `lucind-ai stability run` (`cmd/lucind-ai/cli.go:123-145`).

## Decision 5 — Domain Separation between Stability Reconcile and Branch Reconcile

**Choice**: Implement stability recovery in `internal/stability/reconcile` for campaign interruption, residue cleanup (`internal/worktree/worktree.go:247-269`), and survivor processes, without depending on `internal/reconcile/reconcile.go:1-33`.
**Alternatives considered**: Overloading `internal/reconcile/reconcile.go:1-33` with campaign recovery logic.
**Rationale**: `internal/reconcile/reconcile.go:1-33` manages branch merge conflict approvals and candidate models. Stability reconciliation handles crash inspection, fail-closed resume, and residue purges (`internal/worktree/worktree.go:247-269`).
**Terminal consumer**: `lucind-ai stability resume` and `abort` in `cmd/lucind-ai/cli.go:123-145`.

## Decision 6 — Sequential Three-Trial State Machine with Strict Zero-Retry Reset

**Choice**: Sequential 3-Trial state machine with zero retries. Any failure halts active Trial, resets consecutive trial count to 0 (`internal/barrier/barrier.go:36-60`), and transitions Campaign to `failed` (or `blocked_cleanup`).
**Alternatives considered**: Allowing slot retries; resuming failed Trials midway.
**Rationale**: Release certification requires three consecutive complete pass journeys (`docs/adr/0001-native-stability-campaign.md:5-24`). Retries hide instability. Zero-retry reset guarantees integrity.
**Terminal consumer**: `internal/stability` engine coordinating 3-Trial execution (`internal/barrier/barrier.go:36-60`).

## Decision 7 — Full Forensic Trial Record Emission in Status JSON

**Choice**: Emit full Trial Record bodies (transitions, timings, sanitized stream details, defect records, payload hashes) in `lucind-ai stability status --json`.
**Alternatives considered**: Compact summary references only; unstructured text logs.
**Rationale**: Audit tools and CI release gates require complete forensic inspectability without direct SQLite access (`cmd/lucind-ai/cli.go:123-145`, `docs/adr/0001-native-stability-campaign.md:20-24`).
**Terminal consumer**: CLI `stability status --json` output in `cmd/lucind-ai/cli.go:123-145` and `internal/stability/evidence`.

## Decision 8 — Crash Recovery via Monotonic Lease Reclaim and Envelope Adoption

**Choice**: Simulate Change B crash via `SIGKILL` after result persistence in `.lucind/result.json` (`internal/run/run.go:56-60`). Reclaim during 10s TTL returns `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). After expiry, replacement B acquires lease with incremented fence (`internal/feature/feature.go:406-473`), verifies zero `/proc` survivors, and adopts envelope (`internal/run/run.go:56-60`, `cmd/lucind-ai/cli.go:710-723`) without redispatch.
**Alternatives considered**: Re-executing Change B from scratch; recovering without checking `/proc`.
**Rationale**: Proves result envelopes survive crashes (`cmd/lucind-ai/cli.go:710-723`), and lease fencing prevents dual-writer races before promotion (`internal/feature/feature.go:334-398`, `internal/feature/feature.go:406-473`).
**Terminal consumer**: `internal/feature/feature.go:334-398`, `internal/feature/feature.go:406-473`, `cmd/lucind-ai/cli.go:710-723`, and `internal/run/run.go:56-60`.

## Decision 9 — Canonical RFC 8785 JSON Stability Receipt with Final Baseline Check

**Choice**: Persist immutable RFC 8785 JSON Stability Receipts at `<git-common-dir>/lucind-ai/stability/v1/receipts/<receipt_id>.json` only after three passing Trials AND a post-cleanup `integrate.Check` baseline pass (`cmd/lucind-ai/cli.go:503-509`, `internal/integrate/integrate.go:100-120`). Capped sanitization applies (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`).
**Alternatives considered**: Emitting receipt before post-cleanup check; creating Git release tags; storing unsanitized logs.
**Rationale**: Post-cleanup check ensures a clean repository (`internal/integrate/integrate.go:100-120`, `internal/integrate/integrate.go:126-138`). Git tags remain out of scope (`docs/adr/0001-native-stability-campaign.md:20-24`). Sanitization prevents secret leaks (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`).
**Terminal consumer**: `internal/stability/evidence` and `cmd/lucind-ai/cli.go:503-509` completing `lucind-ai stability run`.

## Open Questions

- [ ] None. (The 4 open questions from `proposal.md` Section 9 are resolved above: Decision 2 resolves process-group configuration via `executor.Request`, Decision 3 resolves common-dir resolution via `worktree.GitRunner`, Decision 5 resolves separation from `internal/reconcile/`, and Decision 7 resolves full forensic JSON emission in `stability status`).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Root CLI subcommand routing table dispatching commands and validating subcommands |
| `cmd/lucind-ai/cli.go:503-509` | Baseline check execution measuring duration and delegating to `integrate.Check` |
| `cmd/lucind-ai/cli.go:710-723` | `PersistEnvelope` serializing and saving result envelopes to `.lucind/results/` |
| `docs/adr/0001-native-stability-campaign.md:5-24` | ADR establishing 3-Trial stability campaign lifecycle, common-dir authority, and JSON receipts |
| `internal/barrier/barrier.go:36-60` | `Evaluate` pure batch barrier function determining terminal status and lane outcome |
| `internal/executor/agy.go:193-205` | `exec.CommandContext` child process construction and execution without `SysProcAttr` or process-group isolation |
| `internal/executor/agy_test.go:158-191` | `TestRunGrandchildHoldingPipesExitZeroReportsOutputTruncated` verifying grandchild subprocess pipe inheritance behavior |
| `internal/executor/executor.go:27-52` | `Request` struct defining dispatch parameters (Prompt, WorktreePath, Model, SchemaPath, Progress) |
| `internal/feature/feature.go:334-398` | `AcquireLease` implementing conditional lease acquisition with monotonic fencing and `ErrLeaseHeld` on active hold |
| `internal/feature/feature.go:406-473` | `RenewLease` validating active ownership and fencing token before updating lease expiration |
| `internal/integrate/integrate.go:100-120` | `Check` running `lucind-checks.sh` in the specified worktree path to verify repository baseline |
| `internal/integrate/integrate.go:126-138` | `Promote` checking `git status --porcelain` cleanliness before fast-forward merge |
| `internal/ledger/ledger.go:146-185` | `Open` and `openAtPath` configuring SQLite connection pool and WAL pragmas |
| `internal/ledgerpath/ledgerpath.go:34-58` | `Resolve` and `Validate` restricting ledger paths strictly to `<primaryRoot>/.lucind/` |
| `internal/reconcile/reconcile.go:1-33` | Package declaration and sentinel errors for feature branch overlap reconciliation requests |
| `internal/run/batch.go:66-78` | `ExecuteBatch` constructing barrier and dispatching parallel lane workers |
| `internal/run/run.go:56-60` | `resultEnvelopePath` constant locating `.lucind/result.json` in the worktree |
| `internal/run/run.go:71-90` | `streamDetailCap` constant (4096 bytes) and bounded log rationale |
| `internal/run/run.go:131-150` | `diagnosisDetail` and `formatStreamDetail` formatting truncated stderr/stdout diagnostics |
| `internal/worktree/worktree.go:47-69` | `GitRunner` interface and `execGitRunner` execution wrapper |
| `internal/worktree/worktree.go:173-238` | `CreateWithParent` and `createWithRunner` adding linked git worktrees with ancestor checks |
| `internal/worktree/worktree.go:247-269` | `Cleanup`, `Remove`, and `DeleteBranch` for idempotent worktree and branch deletion |
| `lucind-ai-stability-run-sdd-master-plan.md:240-252` | Suggested package seams and architecture decomposition for native stability campaigns |
