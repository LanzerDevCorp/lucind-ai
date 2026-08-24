# Proposal: Native Stability Campaign

## 1. Executive Summary & Problem Statement

Lucind AI coordinates work across isolated worktrees (`internal/worktree/worktree.go:173-238`), dispatches AI workers via `agy` (`internal/executor/agy.go:85-96`), and records state in SQLite at `<primaryRoot>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-148`, `internal/ledgerpath/ledgerpath.go:34-38`). CLI commands route through `cmd/lucind-ai/cli.go:123-145`.

However, the repository lacks native release certification. External harnesses place stability evidence outside product authority (`docs/adr/0001-native-stability-campaign.md:5-13`). Ordinary batch runs (`cmd/lucind-ai/cli.go:123-145`, `internal/run/batch.go:66-70`) on feature branches (`internal/feature/feature.go:98-113`, `internal/feature/feature.go:115-130`) lack multi-trial orchestration, crash recovery, or receipts.

Lucind AI introduces `lucind-ai stability run|status|resume|abort` (`cmd/lucind-ai/cli.go:123-145`, `lucind-ai-stability-run-sdd-master-plan.md:7-16`). This validates candidate commits against three consecutive deterministic Trials using real `agy` dispatches (`docs/adr/0001-native-stability-campaign.md:15-16`, `lucind-ai-stability-run-sdd-master-plan.md:59-66`), stores mutable authority under `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`, `lucind-ai-stability-run-sdd-master-plan.md:141-153`), and emits a JSON Stability Receipt (`docs/adr/0001-native-stability-campaign.md:20-24`, `lucind-ai-stability-run-sdd-master-plan.md:205-211`).

## 2. Selected Candidate & Technical Approach

Adopt **Candidate 2 (Modular Subpackage Decomposition under `internal/stability/`)** with modification to `internal/executor/agy.go:193-205` for process-group isolation (`Setpgid: true`).

1. **Preflight Admission:** `stability run` validates Linux OS, clean checkout (`internal/integrate/integrate.go:126-138`), candidate `HEAD` build match (`cmd/lucind-ai/cli.go:140-142`), zero active campaigns (`internal/ledger/ledger.go:162-185`), and baseline check (`cmd/lucind-ai/cli.go:503-509`, `internal/integrate/integrate.go:100-120`), with confirmation (`docs/adr/0001-native-stability-campaign.md:15-16`, `lucind-ai-stability-run-sdd-master-plan.md:173-176`).
2. **Three-Trial Orchestration:** Runs three sequential Trials (`lucind-ai-stability-run-sdd-master-plan.md:59-66`). Any failure resets count to zero. Each Trial executes 5 non-retryable dispatches (`internal/executor/executor.go:27-52`, `internal/executor/executor.go:80-95`) pinned to `gemini-3.7-flash-high` (`internal/executor/agy.go:85-96`, `lucind-ai-stability-run-sdd-master-plan.md:163-165`), with budgets 10m/dispatch, 45m/Trial, 135m/Campaign (`internal/executor/executor.go:57-78`).
3. **Concurrent Journey & Remediation:** Ephemeral targets (`internal/feature/feature.go:98-113`, `internal/worktree/worktree.go:173-238`) run A and B concurrently (`internal/run/batch.go:66-70`). Change A hits out-of-scope defect, persists Defect Record, and emits Remediation Proposal approved by Test Actor (`lucind-ai-stability-run-sdd-master-plan.md:69-70`, `lucind-ai-stability-run-sdd-master-plan.md:186-188`). Fix Change dispatches to A target while B proceeds.
4. **Crash Recovery & Containment:** Change B is killed via `SIGKILL` after result persistence (`lucind-ai-stability-run-sdd-master-plan.md:71-73`, `cmd/lucind-ai/cli.go:710-723`, `internal/run/run.go:48-60`). Reclaims during 10s lease TTL return `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). Replacement B reclaims after expiry, verifies zero `/proc` survivors, adopts saved envelope, and promotes B (`internal/feature/feature.go:406-473`). Fix promotes to A target; Change A resumes under original identity and promotes.
5. **Storage Authority & Receipt:** Mutable state lives in SQLite/WAL at `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`, `lucind-ai-stability-run-sdd-master-plan.md:141-153`), isolated from primary ledger (`internal/ledger/ledger.go:146-148`, `internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledgerpath/ledgerpath.go:40-58`). Passed campaigns run baseline check (`internal/integrate/integrate.go:100-112`) and emit Stability Receipt (`docs/adr/0001-native-stability-campaign.md:20-24`, `lucind-ai-stability-run-sdd-master-plan.md:205-211`).

## 3. Changes to System Concepts & Architecture Rationale

### Package Decomposition under `internal/stability/`
Modular subpackages (`lucind-ai-stability-run-sdd-master-plan.md:240-252`):
- `store`: Common-dir SQLite/WAL schema, transactions, single-active gate.
- `fixture`: Templates, synthetic packets, check scripts, tree digests.
- `process`: Process supervision, `/proc` survivor audit, test clocks.
- `evidence`: Bounded sanitization (`streamDetailCap = 4096`, `internal/run/run.go:71-90`, `internal/run/run.go:131-150`), hashing, Trial Records, canonical JSON receipts (RFC 8785).
- `reconcile`: Resume/abort inspection, idempotent cleanup (`internal/worktree/worktree.go:247-269`), `blocked_cleanup` transitions.

### Non-Additive Executor Seam Modification (R5 Requirement)
`internal/executor/agy.go:193-205` lacks `SysProcAttr`/`Setpgid`. Killing direct child leaves grandchild MCP processes alive (`internal/executor/agy.go:19-40`, `internal/executor/agy_test.go:158-191`). Setting `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on Linux introduces a reviewed blast radius on general execution (`internal/run/batch.go:66-70`).

### Concept Additions & Modifications
- **Stability Campaign & Trial:** Release certification and sequential 5-dispatch journeys (`CONTEXT.md:27-33`, `lucind-ai-stability-run-sdd-master-plan.md:48-56`).
- **Common-Dir Storage Authority:** SQLite store under `<git-common-dir>/lucind-ai/stability/v1/`, bypassing `.lucind` path validation (`internal/ledgerpath/ledgerpath.go:40-58`).
- **Accelerated Lease:** 10s expiry and reclaim with monotonic fencing (`internal/feature/feature.go:334-398`, `internal/feature/feature_test.go:278-310`).
- **In-Memory Barrier:** In-memory evaluation (`internal/barrier/barrier.go:36-60`) tracking slot progression independently of primary ledger.

## 4. User & Capability Impact

| Capability | Impact | Description | Seam (file:line) |
|---|---|---|---|
| `stability-command-contract` | Added | CLI commands (`run|status|resume|abort`), preflight, confirmation | `cmd/lucind-ai/cli.go:123-145` |
| `stability-authority-store` | Added | Common-dir SQLite/WAL store with single-active gate | `internal/ledger/ledger.go:162-185` |
| `stability-campaign-state-machine` | Added | Three-Trial sequential lifecycle, zero retries, timeout budgets | `internal/barrier/barrier.go:36-60` |
| `stability-fixture-journey` | Added | Fixture templates, synthetic packets, Write Scopes, tree hashes | `internal/worktree/worktree.go:173-238` |
| `stability-remediation-flow` | Added | Defect discovery, Defect Record, Test Actor gates, Fix Change | `internal/feature/feature.go:98-113` |
| `stability-evidence-receipt` | Added | Sanitized logs, payload hashing, JSON receipt on pass | `cmd/lucind-ai/cli.go:710-723` |
| `stability-resume-abort` | Added | Fail-closed resume, idempotent abort, blocked cleanup | `internal/reconcile/reconcile.go:1-33` |
| `lane-execution` | Modified | Child execution updated with process group (`Setpgid: true`) and kill | `internal/executor/agy.go:193-205` |

## 5. Delta Specifications

### Preflight Admission, Safety Gates, and Three-Trial Scheduling
Preflight validates Linux OS, clean checkout (`internal/integrate/integrate.go:126-138`), `HEAD` build match (`cmd/lucind-ai/cli.go:140-142`), baseline check success (`cmd/lucind-ai/cli.go:503-509`, `internal/integrate/integrate.go:100-120`), `gemini-3.7-flash-high` availability (`internal/executor/agy.go:85-96`), and zero active campaigns (`internal/ledger/ledger.go:162-185`), with confirmation (`cmd/lucind-ai/cli.go:123-145`). Executes three sequential Trials (`lucind-ai-stability-run-sdd-master-plan.md:59-66`), deleting prior worktrees (`internal/worktree/worktree.go:247-269`). Failures reset count to zero (`internal/barrier/barrier.go:36-60`). Budgets: 10m/dispatch, 45m/Trial, 135m/Campaign (`internal/executor/executor.go:57-78`).
- **Scenario (Dirty checkout):** GIVEN dirty checkout, WHEN `stability run` executes, THEN preflight halts non-zero without mutation (`internal/integrate/integrate.go:126-138`).
- **Scenario (Failure reset):** GIVEN active Trial 2, WHEN slot fails, THEN consecutive count resets to 0 and Campaign fails (`internal/barrier/barrier.go:36-60`).

### Concurrent Multi-Change Fixture and Remediation Flow
Trials create ephemeral targets and dispatch Changes A and B concurrently (`internal/feature/feature.go:98-113`, `internal/run/batch.go:66-78`). Both Orchestrators hold active leases before Promotion. Change A hits out-of-scope defect, persists Defect Record, and emits Remediation Proposal (`internal/integrate/integrate.go:52-85`). Test Actor approves fix and launches Fix Change; Change B proceeds (`internal/feature/feature.go:115-130`).
- **Scenario (Temporal overlap):** GIVEN Changes A and B, WHEN registering lanes, THEN both hold active leases before Promotion (`internal/run/batch.go:66-78`, `internal/feature/feature.go:98-113`).
- **Scenario (Independent fix):** GIVEN Change A defect, WHEN Defect Record persists, THEN A blocks while B proceeds (`internal/integrate/integrate.go:52-85`).

### Process Group Isolation, Crash Recovery, and Lease Reclaim
Executor sets `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` (`internal/executor/agy.go:193-205`). Coordinator kills Change B with `SIGKILL` after result persistence (`cmd/lucind-ai/cli.go:710-723`, `internal/run/run.go:48-60`) before Acceptance. Reclaims during 10s lease TTL return `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). After expiry, replacement B reclaims ownership, verifies zero `/proc` survivors, adopts saved envelope, and promotes B (`internal/feature/feature.go:406-473`). Surviving processes fail Trial (`internal/executor/agy.go:193-205`).
- **Scenario (Early reclaim blocked):** GIVEN killed B with active 10s lease, WHEN replacement attempts reclaim, THEN store returns `ErrLeaseHeld` (`internal/feature/feature.go:334-398`).
- **Scenario (Envelope adoption):** GIVEN expired B lease, WHEN replacement reclaims at t>=10s, THEN it adopts saved envelope and promotes without AI dispatch (`internal/feature/feature.go:406-473`, `cmd/lucind-ai/cli.go:710-723`).

### Isolated Target Promotion and Ancestry Verification
Change B promotes to its target before Fix completes (`internal/integrate/integrate.go:126-138`). Fix modifies authorized scope and promotes to Change A target (`internal/integrate/integrate.go:153-175`). Change A resumes under original identity via new `agy` dispatch without reclaim (`internal/feature/feature.go:115-130`). Git ancestry proves Change A target contains Fix+A commits while Change B target contains only B commits (`internal/run/attempt.go:740-750`, `internal/overlap/overlap.go:21-23`).
- **Scenario (Independent promotion):** GIVEN Change B completed, WHEN promoting, THEN B fast-forwards target before Fix integration (`internal/integrate/integrate.go:126-138`).
- **Scenario (Ancestry proof):** GIVEN integrated targets, WHEN inspecting history, THEN A target contains Fix+A, B target contains only B (`internal/run/attempt.go:740-750`, `internal/overlap/overlap.go:21-23`).

### Storage Authority, Sanitized Evidence, and Terminal Receipt
State lives in SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/stability.db`, isolated from primary ledger (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledgerpath/ledgerpath.go:40-58`). CLI provides `stability status [--json]` (`cmd/lucind-ai/cli.go:123-145`). Interrupted campaigns support fail-closed resume and idempotent abort (`internal/reconcile/reconcile.go:1-33`, `internal/worktree/worktree.go:247-269`), entering `blocked_cleanup` on residue. Evidence is sanitized before cleanup (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`, `internal/worktree/worktree.go:247-269`). Passing campaigns run baseline check before emitting JSON receipt binding candidate SHA, build, and Trial Records (`internal/integrate/integrate.go:100-120`, `docs/adr/0001-native-stability-campaign.md:20-24`, `lucind-ai-stability-run-sdd-master-plan.md:7-16`). Terminal status certifies release (`docs/adr/0001-native-stability-campaign.md:20-24`).
- **Scenario (Single active constraint):** GIVEN active Campaign, WHEN second `run` starts, THEN transaction rejects with non-zero exit (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-58`).
- **Scenario (Receipt generation):** GIVEN three passed Trials and clean check, WHEN completing, THEN JSON receipt is persisted without Git tags/pushes (`internal/integrate/integrate.go:100-120`, `docs/adr/0001-native-stability-campaign.md:20-24`).

## 6. Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Seam (file:line) |
|---|---|---|---|
| **Executor Blast Radius**: `Setpgid: true` risks breaking ordinary dispatches (`internal/run/run.go:205-229`). | High | Encapsulate supervision, test stubs (`internal/executor/agy_test.go:28-50`). | `internal/executor/agy.go:193-205` |
| **Grandchild MCP Leaks**: Dispatched `agy` MCP subprocesses survive kills (`internal/executor/agy_test.go:158-191`). | High | Set `Setpgid: true`, kill `-pgid`, audit zero `/proc` survivors. | `internal/executor/agy.go:19-40` |
| **SQLite/WAL Inconsistency**: Mid-Trial crash leaves uncheckpointed WAL (`internal/ledgerpath/ledgerpath.go:34-38`). | High | Open SQLite with WAL, enforcing single active gate. | `internal/ledger/ledger.go:162-185` |
| **10s Lease Race**: Premature Orchestrator B reclaim before 10s expiry (`internal/run/attempt.go:298-328`). | High | Update returns `ErrLeaseHeld` and increments fence (`internal/feature/feature_test.go:278-310`). | `internal/feature/feature.go:334-398` |
| **Secret/Path Leakage**: Raw streams leak into receipts (`cmd/lucind-ai/cli.go:710-723`). | Medium | Sanitization (`streamDetailCap = 4096`, `internal/run/run.go:131-150`) strips paths and hashes payloads. | `internal/run/run.go:71-90` |
| **Target Deadlock**: Concurrent Changes A and B with Fix deadlock gates (`internal/overlap/overlap.go:21-23`). | Medium | Distinct targets, ancestry checks (`internal/worktree/worktree.go:203-209`), in-memory gates (`internal/barrier/barrier.go:36-60`). | `internal/worktree/worktree.go:173-238` |
| **Quota & Timeouts**: 15 dispatches pinned to `gemini-3.7-flash-high` (`lucind-ai-stability-run-sdd-master-plan.md:136-137`). | Medium | Preflight forecasts 15 dispatches with confirmation and timeouts (`internal/executor/executor.go:57-78`). | `cmd/lucind-ai/cli.go:246-250` |
| **Cleanup Residue**: Interrupted campaigns leave temporary residue (`internal/feature/feature.go:477-510`). | Medium | Transition to `blocked_cleanup`; `stability abort` purges residue. | `internal/worktree/worktree.go:247-269` |

## 7. Rollback Plan & Additivity

**Rollback Plan**: Rollback of `internal/stability` and CLI routing (`cmd/lucind-ai/cli.go:123-145`) is via `git revert`. No down-migrations are required; historical data in `<git-common-dir>/lucind-ai/stability/v1/` remains inert. Primary checkout stays clean (`internal/integrate/integrate.go:126-138`).

**Additivity**: Storage changes are strictly additive. The Run ledger at `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledger/ledger.go:146-148`) is untouched; Trial Records reference Run IDs read-only. Stability Campaign authority is isolated in `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-58`). Stability Receipts are standalone JSON files preserving `.lucind/results/` (`cmd/lucind-ai/cli.go:710-723`).

## 8. Test & Validation Impact

| Test Layer | Impact / Required Coverage | Seam (file:line) |
|---|---|---|
| **Domain State Machine** | TDD for transitions, counters, reset-on-failure, timeouts (`internal/lane/status_test.go:1-31`). | `internal/barrier/barrier_test.go:31-60` |
| **Storage & SQLite/WAL** | Schema init, migrations, unique-active gate, WAL crash reopening. | `internal/ledger/ledger_test.go:35-58` |
| **Path Resolution** | Resolution targeting `<git-common-dir>/lucind-ai/stability/v1/` and rejection. | `internal/ledgerpath/ledgerpath_test.go:9-35` |
| **Process Lifecycle** | Process-group launch (`Setpgid: true`), kill, `/proc` survivor audit (`writeStub`). | `internal/executor/agy_test.go:158-191` |
| **10s Lease & Fencing** | Lease acquisition, early reclaim rejection (`ErrLeaseHeld`), takeover at t>=10s. | `internal/feature/feature_test.go:278-310` |
| **Fixture & Ancestry** | Fixture generation, defect detection, Remediation, Fix promotion, ancestry proof. | `internal/worktree/worktree.go:203-209` |
| **Cleanup Tests** | Idempotent creation, removal, and cleanup of worktrees/branches. | `internal/worktree/worktree.go:247-269` |
| **Evidence Sanitization** | Stripping paths (`streamDetailCap = 4096`), hashing, RFC 8785 JSON. | `internal/run/run.go:71-90` |
| **CLI Preflight & Status** | `stability` CLI suite, porcelain check, stale binary rejection, schema tests. | `cmd/lucind-ai/cli_test.go:40-60` |
| **Fake Journey E2E** | 3-Trial simulated Campaign verifying crash, reclaim, Fix, receipt. | `internal/run/batch_test.go:26-59` |
| **Native Baseline** | Check baseline (`cmd/lucind-ai/cli.go:503-509`); real 3-Trial Campaign outside `go test`. | `internal/integrate/integrate.go:100-120` |

## 9. Out of Scope & Open Questions

### Out of Scope
- Non-Linux OS for mutating execution (Linux-only in V1 per Q53).
- Non-interactive bypasses, `--yes`, NIP, or secret storage (Q29, Q33).
- External issue trackers or remote repository mutation (Q18, Q54).
- Alternative AI executors (pinned to `gemini-3.7-flash-high` per Q80, Q86).
- Ordinary Run ledger migration at `<primary-root>/.lucind/lucind.db` (Q67).
- Automatic AI retries or dynamic timeout / lease configuration (Q50, Q64, Q65).
- Git tagging, version bumping, release publishing, or pushing (Q54).
- Control Room UI views for Stability Campaigns (Q77).

### Open Questions
- [ ] Should `internal/executor/agy.go` configure Linux process-group isolation (`Setpgid: true`) unconditionally on Linux, or should `executor.Request` introduce an optional configuration field?
- [ ] How to resolve `<git-common-dir>` portably across checkouts using `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`)?
- [ ] Should `internal/stability/reconcile` reuse primitives from `internal/reconcile/` (`internal/reconcile/reconcile.go:1-33`) or keep distinct cleanup semantics?
- [ ] Should `stability status --json` output full Trial Record bodies or compact summary references?