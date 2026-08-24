# Exploration: Native Stability Campaign

## 1. Problem Statement and Background

Lucind AI coordinates repository work across isolated git worktrees (`internal/worktree/worktree.go:155-159`, `internal/worktree/worktree.go:173-238`), dispatches headless AI worker agents via `agy` (`internal/executor/agy.go:68-80`, `internal/executor/agy.go:103-109`), and records lane state and leases in SQLite at `<primaryRoot>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-168`, `internal/ledgerpath/ledgerpath.go:34-38`). CLI commands route through `cmd/lucind-ai/cli.go:56-57,123-145`.

However, the repository lacks native release certification. Per ADR-0001 (`docs/adr/0001-native-stability-campaign.md:5-13`), validating candidate stability via external harnesses places evidence and crash recovery outside product authority. Ordinary batch runs (`cmd/lucind-ai/cli.go:147-150`, `internal/run/batch.go:66`) execute tasks on feature branches (`internal/run/run.go:43-46`, `internal/feature/feature.go:98-113`, `internal/feature/feature.go:115-130`) without multi-trial orchestration, lease expiration recovery, cross-Change remediation, or immutable receipts.

To provide release confidence, Lucind AI requires a native Linux `lucind-ai stability` command group (`run`, `status [--json]`, `resume`, `abort`) (`lucind-ai-stability-run-sdd-master-plan.md:7-16`). This capability validates an immutable candidate commit against three consecutive deterministic Stability Trials using real `agy` dispatches (`docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:59-86`), isolates mutable lifecycle state under `<git-common-dir>/lucind-ai/stability/v1/` (`docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:141-143`), and emits a content-addressed Stability Receipt.

## 2. Candidate Approaches

### Candidate 1 — Monolithic Extension of Run and Ledger Subsystems
- **Approach**: Extend `lucind-ai run` (`cmd/lucind-ai/cli.go:147-150`) and primary ledger (`internal/ledger/ledger.go:146-168`) with Campaign/Trial tables under `<primaryRoot>/.lucind/`.
- **Pros**: Reuses existing migration and database execution helpers.
- **Cons**: Overloads ephemeral runs with release authority; violates ADR-0001 (`docs/adr/0001-native-stability-campaign.md:15-24`) and Decision 67 (`lucind-ai-stability-run-sdd-master-plan.md:141-143`).
- **Feasibility**: Infeasible. Ledger validator (`internal/ledgerpath/ledgerpath.go:40-58`) rejects database paths outside `<primaryRoot>/.lucind/`.

### Candidate 2 — Modular Subpackage Decomposition under `internal/stability` (Recommended)
- **Approach**: Implement `stability` CLI projection (`cmd/lucind-ai/cli.go:123-145`) backed by `internal/stability` subpackages: `store` (common-dir SQLite/WAL authority), `fixture` (embedded templates/checks), `process` (Linux process groups and survivor proof), `evidence` (sanitization and receipts), and `reconcile` (resume/abort cleanup) (`lucind-ai-stability-run-sdd-master-plan.md:240-252`). Consumes existing primitives (`internal/worktree/worktree.go:173-238`, `internal/executor/agy.go:68-80`, `internal/integrate/integrate.go:52-85`, `internal/integrate/candidate.go:48-60`, `internal/reconcile/reconcile.go:1-33`).
- **Pros**: Enforces separation of concerns; isolates SQLite schema from ordinary runs; encapsulates Linux process handling; provides disjoint Write Scopes for parallel SDD waves.
- **Cons**: Requires interface contracts between subpackages.
- **Feasibility**: High. Integrates with `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`), `executor.Agy` (`internal/executor/agy.go:68-80`), and in-memory evaluation (`internal/barrier/barrier.go:36-60`, `internal/overlap/overlap.go:21-23`).

### Candidate 3 — Consolidated Flat `internal/stability` Package
- **Approach**: Implement stability in a single flat package combining state transitions (`lucind-ai-stability-run-sdd-master-plan.md:254-273`), SQLite store, fixtures, process control, and receipts.
- **Pros**: Avoids subpackage boilerplate and import cycle risks.
- **Cons**: Mixes storage with low-level Linux process handling; creates merge conflicts across concurrent SDD lanes.
- **Feasibility**: Medium. Compiles cleanly, but hinders modular testing and contradicts repository conventions (`internal/barrier/`, `internal/integrate/`, `internal/ledger/`, `internal/reconcile/`).

### Recommendation
Adopt **Candidate 2 (Modular Subpackage Decomposition under `internal/stability`)**. It fulfills storage isolation under `<git-common-dir>/lucind-ai/stability/v1/` (`docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:141-143`), isolates Linux process lifecycle, preserves a thin CLI routing layer, and enables parallel SDD implementation waves.

## 3. User & Capability Impact

### Operator Surfaces & Personas
- **Release Engineers & Maintainers**: Use subcommands (`stability run|status|resume|abort`) via `cmd/lucind-ai/cli.go:123-145`. Preflight validates candidate `HEAD`, clean tree (`internal/integrate/integrate.go:127-138`), binary match, baseline checks (`cmd/lucind-ai/cli.go:501-508`, `internal/integrate/integrate.go:100-112`), and `agy` quota (`cmd/lucind-ai/cli.go:355-357`), requiring interactive confirmation defaulting to `no`.
- **Autonomous Orchestrators & Agents**: Operate within linked worktrees (`internal/worktree/worktree.go:155-159`, `internal/worktree/worktree.go:173-238`) via packet contracts (`internal/executor/executor.go:27-52`). Each Trial executes five non-retryable dispatches pinned to `gemini-3.7-flash-high` (`internal/executor/agy.go:86-96`, `cmd/lucind-ai/cli.go:246-250`).
- **Auditors & Automation**: Read-only `stability status --json` reports campaign lifecycle, trial progress, and residue.

### Storage & Architectural Boundaries
- **Storage Isolation**: Mutable state lives under `<git-common-dir>/lucind-ai/stability/v1/` in SQLite/WAL (`internal/ledger/ledger.go:162-185`), separate from `<primary-root>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-168`).
- **Receipt Boundary**: Passing campaigns emit an immutable, content-addressed JSON Stability Receipt without tagging, pushing, branching, or releasing.

## 4. Scenarios & Use Cases

### Scenario 1 — Preflight Rejection on Dirty Working Tree
- **Action**: Operator runs `lucind-ai stability run` (`cmd/lucind-ai/cli.go:123-145`) on a dirty checkout.
- **Outcome**: Preflight rejects execution before creating database rows, worktrees (`internal/worktree/worktree.go:155-159`), or dispatches, reporting dirty paths and exiting non-zero.

### Scenario 2 — Preflight Confirmation and Admission
- **Action**: Operator runs `lucind-ai stability run` with matching binary, passing check (`cmd/lucind-ai/cli.go:501-508`, `internal/integrate/integrate.go:100-112`), and available `agy` (`cmd/lucind-ai/cli.go:65-70`), confirming `yes`.
- **Outcome**: Candidate `HEAD` is re-validated, one active Campaign initializes in SQLite (`internal/ledger/ledger.go:162-185`), and Trial 1 begins without CI or bypass.

### Scenario 3 — Concurrent Change Dispatch and Defect Assessment
- **Action**: Trial 1 dispatches Change A and B concurrently to disjoint targets (`internal/feature/feature.go:98-113`, `internal/worktree/worktree.go:78-81`) under leases (`internal/feature/feature.go:283-324`). Change A encounters a defect outside Write Scope.
- **Outcome**: Temporal overlap is established (`internal/barrier/barrier.go:36-60`). Change A records Defect Record and Remediation Proposal; Test Actor approves Fix Change; Change A blocks while B continues.

### Scenario 4 — Orchestrator Crash, Lease Expiry, and Reclaim
- **Action**: Change B persists envelope (`internal/run/run.go:48-60`) before Acceptance. System abruptly kills B's `agy` process group (`internal/executor/agy.go:193-205`).
- **Outcome**: Takeover is blocked while 10s lease is active (`internal/feature/feature.go:283-324`, `internal/feature/feature.go:476-510`). After expiry, replacement reclaims ownership, verifies saved envelope and zero survivors, and promotes B.

### Scenario 5 — Fix Promotion and Resumption of Change A
- **Action**: Fix completes isolated edits and promotes to A's target (`internal/feature/feature.go:115-130`, `internal/integrate/integrate.go:52-85`, `internal/integrate/integrate.go:153-170`).
- **Outcome**: Change A resumes under original identity via new `agy` dispatch (`internal/executor/executor.go:27-52`), passes checks, and promotes. Git ancestry verifies target isolation (`internal/run/attempt.go:743-747`, `internal/overlap/overlap.go:21-23`).

### Scenario 6 — Three-Trial Certification and Failure Reset
- **Action**: Trials 1 and 2 pass; Trial 3 encounters failed slot outcome (`internal/executor/executor.go:57-78`).
- **Outcome**: Consecutive count resets to zero. Campaign terminates as `failed`, cleans operational state, and halts.

### Scenario 7 — Abort and Blocked Cleanup Recovery
- **Action**: Operator runs `lucind-ai stability abort` on interrupted campaign.
- **Outcome**: System enters `blocked_cleanup` if residue remains, idempotently removes temporary worktrees/leases (`internal/worktree/worktree.go:247-269`) without AI dispatches, and closes campaign.

### Scenario 8 — Receipt Generation and Status Observation
- **Action**: All three Trials pass and post-cleanup `lucind-ai check` succeeds (`cmd/lucind-ai/cli.go:501-508`, `internal/integrate/integrate.go:100-112`). Operator queries `stability status --json`.
- **Outcome**: A canonical content-addressed JSON Stability Receipt is persisted under common-dir authority, binding commit SHA, build, versions, environment, and trial evidence.

## 5. Technical Risks & Trade-offs Matrix

### Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing Seam |
|---|---|---|---|
| Orphaned process leakage during crash injection | High | Assign process group (`Setpgid: true`), kill negative PID (`-pgid`), inspect `/proc` for survivors. | `internal/executor/agy.go:12-40`, `internal/executor/agy.go:193-205` |
| Common-dir SQLite WAL contention & recovery | High | Dedicated SQLite database with WAL and `busy_timeout=5000`; transactional single-campaign gate. | `internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledgerpath/ledgerpath.go:40-58` |
| 10s lease race condition & premature takeover | High | Atomic monotonic fencing (`expires_at <= now`); reject early reclaim with `ErrLeaseHeld`. | `internal/feature/feature.go:283-324`, `internal/feature/feature.go:476-510`, `internal/run/attempt.go:298-328` |
| Secret/environment leakage in receipts | Medium | Redact home paths, usernames, secrets; persist bounded logs and SHA-256 payload hashes. | `internal/run/run.go:71-90`, `internal/run/run.go:131-150` |
| Overlap gating deadlock between lanes | Medium | Configure disjoint paths and distinct targets so `evaluateOverlapGate` does not block. | `internal/integrate/integrate.go:52-85`, `internal/overlap/overlap.go:26-42`, `internal/run/attempt.go:743-747` |
| Residual worktree/ref state blocking trials | Medium | Verify worktree/ref deletion before advancement; transition to `blocked_cleanup` on residue. | `internal/barrier/barrier.go:36-60`, `internal/worktree/worktree.go:155-159`, `internal/worktree/worktree.go:247-269` |

### Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| Process group (`Setpgid`) vs Cgroups v2 | Standard POSIX API; requires no root or systemd. | Does not contain processes creating new sessions (`setsid`). | Low: standard library `syscall` and `/proc` inspection. |
| Common-dir vs Root `.lucind/` directory | Shared authority across worktrees; survives deletion; isolates store. | Requires `git rev-parse --git-common-dir` resolution. | Low: resolved once during store initialization. |
| Host abrupt kill vs In-process signaling | Tests crash recovery deterministically without race. | Requires coordinator-level process supervision. | Low: executed by coordinator after envelope write. |
| Fixed 10s lease TTL vs Dynamic configurable | Fast deterministic execution; strictly adheres to Q50. | Requires monotonic time checks against clock skew. | Minimal: 10s deterministic delay during crash recovery. |
| Content-addressed JSON receipt vs SQLite blob | Verifiable via `sha256sum`, immutable, human-inspectable. | Requires canonical JSON formatting (RFC 8785). | Low: emitted once per passing Campaign. |

## 6. Potential Spikes / Proof of Concepts

### Spike 1: Linux Process-Group Termination & Descendant Survivor Detection
- **Objective**: Validate setting `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on `agy` commands (`internal/executor/agy.go:193-205`) allows clean `syscall.Kill(-pgid, syscall.SIGKILL)` termination without leaving orphaned MCP server subprocesses (`internal/executor/agy.go:12-40`).
- **Seam**: `internal/executor/agy.go:193-205`.

### Spike 2: Git Common-Dir SQLite Authority & Crash Recovery
- **Objective**: Validate creating `<git-common-dir>/lucind-ai/stability/v1/stability.db` with WAL mode and connection pool (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:34-38`), verifying single-active-campaign constraints and clean WAL replay.
- **Seam**: `internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-58`.

### Spike 3: Ten-Second Lease Expiry & Explicit Reclaim Protocol
- **Objective**: Implement unit tests modeling 10s lease lifecycle using SQLite conditional updates (`internal/feature/feature.go:283-324`, `internal/run/attempt.go:298-328`), proving early reclaim at t=5s fails with `ErrLeaseHeld` while reclaim at t=11s increments fence.
- **Seam**: `internal/feature/feature.go:283-324`, `internal/feature/feature.go:476-510`.

### Spike 4: Bounded Evidence Sanitization & Canonical Receipt Hashing
- **Objective**: Benchmark redaction pipelines stripping absolute paths (`/home/...`) and secrets from logs (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`), and verify SHA-256 canonical JSON digest generation for terminal receipts.
- **Seam**: `internal/run/run.go:71-90`, `internal/run/run.go:131-150`, `cmd/lucind-ai/cli.go:708-721`.

## 7. Success Criteria

- [ ] `lucind-ai stability` provides `run`, `status [--json]`, `resume`, `abort` (`cmd/lucind-ai/cli.go:123-145`, `lucind-ai-stability-run-sdd-master-plan.md:7-16`).
- [ ] Preflight halts on non-Linux OS, dirty checkout, mismatched binary, failed `lucind-ai check`, or unavailable `agy`/model.
- [ ] Mutating execution requires interactive confirmation defaulting to `no`.
- [ ] Campaign executes up to three sequential Trials, resetting consecutive count to zero on any single failure (`lucind-ai-stability-run-sdd-master-plan.md:59-86`).
- [ ] Each Trial executes concurrent Changes A and B, records Defect Records, and creates independent Fix Changes.
- [ ] Abrupt crash of Orchestrator B preserves envelope and requires 10s lease expiry before reclaim (`internal/feature/feature.go:283-324`).
- [ ] Target isolation is proven by Git ancestry (Fix+A on A's target, B on B's target) (`internal/run/attempt.go:743-747`, `internal/overlap/overlap.go:21-23`).
- [ ] Surviving descendant processes fail Trial validation.
- [ ] Campaign authority is isolated in SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/` without mutating ordinary run ledger (`docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:141-143`).
- [ ] Passed campaign emits canonical content-addressed JSON Stability Receipt without tagging, pushing, or releasing.

## 8. Out of Scope and Open Questions

### Out of Scope
- Non-Linux platforms for mutating Campaign execution (Linux-only in V1 per Q53).
- Non-interactive bypasses, `--yes` flags, NIP, or secret storage (Q29, Q33).
- External issue trackers (GitHub issues, Jira) or remote repository mutation (Q18, Q54).
- Multi-model or alternative executors (pinned to `agy` / `gemini-3.7-flash-high` per Q80, Q86).
- Migration of Run ledger at `<primary-root>/.lucind/lucind.db` (Q67).
- Automatic AI retries or dynamic timeout / lease configuration (Q50, Q64, Q65).
- Automated git tagging, version bumping, release publishing, or pushing (Q54).
- Control Room UI views or web integration for Campaign management (Q77).

### Open Questions
- [ ] Should `internal/stability/reconcile` share abstractions with `internal/reconcile/reconcile.go:1-33` or remain separate given that stability reconciliation handles crashed-trial worktree/ref cleanup rather than feature merge overlap?
- [ ] How will `internal/stability/store` resolve `<git-common-dir>` portably across primary checkouts and linked worktrees using `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`)?
- [ ] Should `stability status --json` output full Trial Record bodies or compact summary references?
