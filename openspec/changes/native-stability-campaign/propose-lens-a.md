# Proposal Lens A — Candidate & Approach: Native Stability Campaign

## Selected Candidate & Approach

Adopt **Candidate 2 (Modular Subpackage Decomposition under `internal/stability/`)** with an explicit cross-cutting modification to `internal/executor/agy.go` for Linux process-group isolation.

The native Stability Campaign provides product-level release certification (`docs/adr/0001-native-stability-campaign.md:5-13`) by introducing the CLI command group `lucind-ai stability run|status|resume|abort` (`cmd/lucind-ai/cli.go:123-145`, `lucind-ai-stability-run-sdd-master-plan.md:7-16`).

### Core Approach

1. **Preflight Gating & Admission:**
   `stability run` executes a read-only preflight validating Linux OS, clean primary working tree via porcelain status (`internal/integrate/integrate.go:126-138`), candidate `HEAD` identity matching the running binary build, zero active campaigns, and passing native verification baseline (`cmd/lucind-ai/cli.go:503-509`, `internal/integrate/integrate.go:100-120`). Mutating execution requires explicit interactive confirmation defaulting to `no` with no non-interactive bypass (`docs/adr/0001-native-stability-campaign.md:15-16`, `lucind-ai-stability-run-sdd-master-plan.md:173-176`).

2. **Deterministic Three-Trial Orchestration:**
   The Campaign coordinates three sequential Stability Trials from clean operational state (`lucind-ai-stability-run-sdd-master-plan.md:59-66`). Any single failure halts the Campaign, triggers idempotent cleanup, and resets consecutive successes to zero. Each Trial executes exactly 5 non-retryable `agy` dispatches (`internal/executor/executor.go:27-52`, `internal/executor/executor.go:80-95`) pinned to `gemini-3.7-flash-high` (`internal/executor/agy.go:85-96`, `lucind-ai-stability-run-sdd-master-plan.md:163-165`).

3. **Concurrent Journey, Defect Flow, & Resumption:**
   Each Trial creates distinct ephemeral Integration Targets off feature parents (`internal/feature/feature.go:98-113`, `internal/worktree/worktree.go:173-238`) and dispatches Changes A and B concurrently (`internal/run/batch.go:66-70`). Change A hits a pre-seeded defect outside its Write Scope, persists a Defect Record, and emits a Remediation Proposal. The Test Actor approves the fix without bypassing human-gate semantics (`lucind-ai-stability-run-sdd-master-plan.md:69-70`, `lucind-ai-stability-run-sdd-master-plan.md:186-188`). A separate Fix Change is dispatched to A's target while B continues unaffected.

4. **Crash Recovery & Process Containment:**
   Change B's `agy` process group is abruptly killed via `SIGKILL` after result persistence and before Acceptance (`lucind-ai-stability-run-sdd-master-plan.md:71-73`, `lucind-ai-stability-run-sdd-master-plan.md:189-192`). Replacement Orchestrator takeover is blocked until the 10-second Ownership Lease expires (`internal/feature/feature.go:477-510`). Replacement B explicitly reclaims ownership, verifies zero surviving descendant processes (including inherited MCP server grandchildren), and promotes B before Fix completes (`internal/run/attempt.go:737-755`, `internal/overlap/overlap.go:20-23`). Fix promotes to A's target; Change A resumes under its original Orchestrator identity and promotes.

5. **Common-Dir Storage Authority & Stability Receipt:**
   Mutable Campaign/Trial state is persisted in SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`, `lucind-ai-stability-run-sdd-master-plan.md:141-153`), isolating release authority from ordinary run ledgers (`internal/ledger/ledger.go:146-148`, `internal/ledgerpath/ledgerpath.go:34-38`). Passing campaigns emit a canonical content-addressed JSON Stability Receipt binding candidate commit, build metadata, environment, and sanitized Trial evidence without creating Git tags, branches, or external releases (`docs/adr/0001-native-stability-campaign.md:20-24`, `lucind-ai-stability-run-sdd-master-plan.md:205-211`).

## Conceptual Changes & Architecture Rationale

### Architecture Seams & Package Decomposition

The modular subpackage decomposition under `internal/stability/` (`lucind-ai-stability-run-sdd-master-plan.md:240-252`) remains sound for domain orchestration and state evaluation:
- `internal/stability/store`: Manages Git common-dir resolution and SQLite/WAL storage schemas, transactions, and the single-active-campaign constraint.
- `internal/stability/fixture`: Houses versioned templates, packets, deterministic verification scripts, and expected Git tree digests.
- `internal/stability/process`: Coordinates Linux process monitoring, `/proc` inspection, test clocks, and survivor validation.
- `internal/stability/evidence`: Handles privacy log sanitization (redacting home paths, environment variables, credentials), SHA-256 payload hashing, Trial Records, and canonical JSON receipt formatting.
- `internal/stability/reconcile`: Drives resume/abort state inspection, idempotent worktree/ref cleanup planning (`internal/worktree/worktree.go:247-269`), and `blocked_cleanup` lifecycle transitions.

### Non-Additive Executor Seam Modification (R5 Finding)

The proposal **cannot be purely additive to `internal/stability/`**. `internal/executor/agy.go:193-205` builds child processes with `exec.CommandContext`, `cmd.Dir`, and `cmd.WaitDelay` without `SysProcAttr` or `Setpgid`. Under this configuration:
1. An abrupt kill on child PID leaves grandchild processes (e.g., background MCP servers spawned by `agy`) alive.
2. Descendant-survivor detection and process-group `SIGKILL` (`-pgid`) cannot function without explicit process-group assignment.

**Architectural Decision:** Modify `internal/executor/agy.go` to set `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on Linux (or configure process-group options on `executor.Request`). Because `internal/executor/agy.go` is shared with ordinary runs (`internal/run/batch.go:66-70`), this introduces a deliberate, reviewed blast radius on general lane execution.

### Concept Additions & Modifications

- **Stability Campaign & Stability Trial:** New first-class lifecycle concepts representing release certification and sequential 5-dispatch journey executions (`lucind-ai-stability-run-sdd-master-plan.md:48-56`).
- **Common-Dir Storage Authority:** Dedicated SQLite database under `<git-common-dir>/lucind-ai/stability/v1/`, bypassing `<primaryRoot>/.lucind/` validation rules (`internal/ledgerpath/ledgerpath.go:40-58`).
- **Accelerated Ownership Lease:** Enforces deterministic 10-second expiry and explicit reclaim without altering standard feature lease configurations.
- **In-Memory Barrier & State Transitions:** Reuses pure in-memory batch evaluation patterns (`internal/barrier/barrier.go:36-60`) to track Trial slot progression without coupling to the primary ledger.

## Alternatives Considered & Rejected

1. **Monolithic Extension of Run and Ledger Subsystems:**
   Extending `lucind-ai run` (`cmd/lucind-ai/cli.go:123-145`) and `<primaryRoot>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-148`) was rejected because it overloads ephemeral lane execution with release certification authority, violates ADR-0001, and fails path validation in `internal/ledgerpath/ledgerpath.go:40-58`.

2. **Consolidated Flat `internal/stability` Package:**
   Placing all stability logic in a single flat package was rejected because it combines low-level OS process handling, SQLite persistence, and fixtures into an unmaintainable unit and creates merge bottlenecks during parallel SDD implementation waves.

3. **Purely Additive `internal/stability` Package (Without Modifying `internal/executor`):**
   Attempting to implement crash recovery without modifying `internal/executor/agy.go:193-205` was rejected because uncontained MCP grandchildren cannot be terminated or audited without process-group control at process launch.

4. **External Acceptance Harness:**
   Using an external test harness was rejected per ADR-0001 (`docs/adr/0001-native-stability-campaign.md:5-13`) because release evidence, crash recovery, and verification semantics must remain authoritative within the Lucind AI product.

## Open Questions

- [ ] Should `internal/executor/agy.go` enable `Setpgid: true` unconditionally on Linux for all dispatches, or should `executor.Request` introduce an optional process-group configuration field?
- [ ] Should `internal/stability/reconcile` reuse reconciliation primitives from `internal/reconcile/` or maintain distinct cleanup semantics dedicated to worktree and temporary branch removal?
- [ ] SDD Contract Scope: This draft satisfies the packet's parallel Lens A candidate/approach slice under a 1000-word budget rather than authoring a complete `proposal.md` as described in `~/.claude/skills/sdd-propose/SKILL.md`.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Subcommand routing switch for root CLI entry points |
| `cmd/lucind-ai/cli.go:503-509` | Baseline check execution via `integrate.Check` in CLI runner |
| `docs/adr/0001-native-stability-campaign.md:5-13` | Architecture decision establishing native Stability Campaign over external harness |
| `docs/adr/0001-native-stability-campaign.md:15-16` | Preflight requirements, clean checkout check, and interactive confirmation defaulting to no |
| `docs/adr/0001-native-stability-campaign.md:20-24` | Storage under git-common-dir, receipt generation, and delivery boundaries |
| `internal/barrier/barrier.go:36-60` | In-memory evaluation and terminal status verification for lane batches |
| `internal/executor/agy.go:85-96` | Model pinning methods returning `gemini-3.7-flash-high` |
| `internal/executor/agy.go:193-205` | Process execution via `exec.CommandContext` lacking `SysProcAttr`/`Setpgid` |
| `internal/executor/executor.go:27-52` | Request struct definition for agent dispatches |
| `internal/executor/executor.go:80-95` | Executor interface contract defining `Run`, `DefaultModel`, and `KnownModels` |
| `internal/feature/feature.go:98-113` | Validation of parent ref names rejecting main and temporary refs |
| `internal/feature/feature.go:477-510` | Lease release implementation expiring feature ownership |
| `internal/integrate/integrate.go:100-120` | Check implementation executing `lucind-checks.sh` |
| `internal/integrate/integrate.go:126-138` | Clean working tree verification via `git status --porcelain` |
| `internal/ledger/ledger.go:146-148` | Primary ledger initialization bound to `<primaryRoot>/.lucind/` |
| `internal/ledger/ledger.go:162-185` | SQLite WAL connection configuration and pragmas |
| `internal/ledgerpath/ledgerpath.go:34-38` | Ledger path resolution under `.lucind/lucind.db` |
| `internal/ledgerpath/ledgerpath.go:40-58` | Ledger path validator rejecting locations outside `.lucind` |
| `internal/overlap/overlap.go:20-23` | Error definitions for missing or multiple merge bases |
| `internal/run/attempt.go:737-755` | Verification of commit SHA ancestry across active features |
| `internal/run/batch.go:66-70` | Concurrent batch dispatch function `ExecuteBatch` |
| `internal/worktree/worktree.go:173-238` | Worktree creation with parent ref and base SHA validation |
| `internal/worktree/worktree.go:247-269` | Idempotent cleanup and removal of worktrees and branches |
| `lucind-ai-stability-run-sdd-master-plan.md:7-16` | Public CLI command specification for `stability run/status/resume/abort` |
| `lucind-ai-stability-run-sdd-master-plan.md:48-56` | Canonical vocabulary definitions for Stability Campaign and Trial |
| `lucind-ai-stability-run-sdd-master-plan.md:59-66` | Requirement for three consecutive successful trials and operational state reset |
| `lucind-ai-stability-run-sdd-master-plan.md:69-70` | Behavior cleanup requirement and deterministic Test Actor gate preservation |
| `lucind-ai-stability-run-sdd-master-plan.md:71-73` | Canonical crash point after result persistence and recovery via native authority |
| `lucind-ai-stability-run-sdd-master-plan.md:141-153` | Native storage specification under git-common-dir SQLite/WAL |
| `lucind-ai-stability-run-sdd-master-plan.md:163-165` | Journey embedding, digest verification, and model pinning |
| `lucind-ai-stability-run-sdd-master-plan.md:173-176` | Product requirement R1 for Campaign admission and interactive confirmation |
| `lucind-ai-stability-run-sdd-master-plan.md:186-188` | Product requirement R4 for defect assessment and separate Fix Change |
| `lucind-ai-stability-run-sdd-master-plan.md:189-192` | Product requirement R5 for crash recovery, 10s lease, and descendant survivor detection |
| `lucind-ai-stability-run-sdd-master-plan.md:205-211` | Product requirements R9 and R10 for evidence receipts and delivery boundaries |
| `lucind-ai-stability-run-sdd-master-plan.md:240-252` | Architecture package decomposition across internal/stability subpackages |
