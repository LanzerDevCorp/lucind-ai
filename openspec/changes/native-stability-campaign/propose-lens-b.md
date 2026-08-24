# Proposal Lens B — Capability Impact & Specs: Native Stability Campaign

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `stability-command-contract` | Added | CLI commands (`run|status|resume|abort`), preflight, and interactive admission | `cmd/lucind-ai/cli.go:123-145` |
| `stability-authority-store` | Added | Common-dir SQLite/WAL store under `lucind-ai/stability/v1/` with single-active gate | `internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:34-38` |
| `stability-campaign-state-machine` | Added | Three-Trial sequential lifecycle, atomic counters, zero retries, and timeout budgets | `internal/barrier/barrier.go:36-60`, `internal/run/batch.go:66-78` |
| `stability-fixture-journey` | Added | Deterministic fixture templates, synthetic packets, Write Scopes, and tree hashes | `internal/worktree/worktree.go:173-238`, `internal/worktree/worktree.go:247-269` |
| `stability-remediation-flow` | Added | Defect discovery, Defect Record, Test Actor gates, Fix Change, and resumption | `internal/feature/feature.go:98-113`, `internal/feature/feature.go:115-130` |
| `stability-evidence-receipt` | Added | Sanitized logs, payload hashing, and content-addressed JSON receipt on pass | `internal/run/run.go:71-90`, `cmd/lucind-ai/cli.go:710-723` |
| `stability-resume-abort` | Added | Fail-closed reconciliation, idempotent abort, and blocked cleanup handling | `internal/reconcile/reconcile.go:1-33`, `internal/worktree/worktree.go:247-269` |
| `lane-execution` | Modified | Child execution updated with Linux process group (`Setpgid: true`) and signal kill | `internal/executor/agy.go:193-205`, `internal/executor/executor.go:57-78` |

## Delta Specifications

### Requirement: Preflight Admission, Safety Gates, and Three-Trial Scheduling

Preflight for `stability run` MUST verify Linux OS, clean checkout (`internal/integrate/integrate.go:127-138`), candidate `HEAD` build match (`cmd/lucind-ai/cli.go:140-142`), baseline check success (`internal/integrate/integrate.go:100-112`), `agy` with `gemini-3.7-flash-high` (`internal/executor/agy.go:86-96`), and no active campaign (`internal/ledger/ledger.go:162-185`), requiring interactive confirmation defaulting to `no` (`cmd/lucind-ai/cli.go:123-145`). A Campaign MUST execute three consecutive Trials sequentially (`lucind-ai-stability-run-sdd-master-plan.md:59-86`). Trial N+1 MUST NOT start until Trial N worktrees and locks are deleted (`internal/worktree/worktree.go:247-269`). Any slot failure or timeout MUST reset consecutive count to zero and fail without retry (`internal/barrier/barrier.go:36-60`). Budgets of 10m per dispatch, 45m per Trial, and 135m per Campaign MUST be enforced (`internal/executor/executor.go:57-78`).

#### Scenario: Dirty checkout rejects execution

- GIVEN an uncommitted primary checkout
- WHEN `stability run` executes
- THEN preflight MUST halt non-zero without state mutation (`internal/integrate/integrate.go:127-138`).

#### Scenario: Slot failure resets consecutive counter

- GIVEN Trial 1 passed and Trial 2 active
- WHEN Trial 2 encounters failure or timeout
- THEN consecutive count MUST reset to 0 and Campaign MUST fail (`internal/barrier/barrier.go:36-60`).

### Requirement: Concurrent Multi-Change Fixture and Remediation Flow

Each Trial MUST create ephemeral Integration Targets and dispatch Changes A and B concurrently via `agy` (`internal/feature/feature.go:98-113`, `internal/run/batch.go:66-78`). Both Orchestrators MUST hold active ownership and dispatched lanes before Promotion. Change A MUST encounter a defect outside Write Scope, persist a Defect Record, and generate a Remediation Proposal (`internal/integrate/integrate.go:52-85`). Test Actor MUST approve the proposal and launch a Fix Change, allowing Change B to proceed (`internal/feature/feature.go:115-130`).

#### Scenario: Temporal overlap verified before promotion

- GIVEN concurrent Changes A and B
- WHEN monitoring lane registration
- THEN both Orchestrators MUST hold active leases and lanes before Promotion (`internal/run/batch.go:66-78`, `internal/feature/feature.go:98-113`).

#### Scenario: Discovered defect routes to independent fix

- GIVEN Change A discovers a defect outside Write Scope
- WHEN Change A persists a Defect Record
- THEN Change A MUST block while Change B proceeds (`internal/integrate/integrate.go:52-85`).

### Requirement: Process Group Isolation, Crash Recovery, and Lease Reclaim

The executor MUST configure child processes with `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` (`internal/executor/agy.go:193-205`). The coordinator MUST kill Change B with `SIGKILL` after result persistence (`cmd/lucind-ai/cli.go:710-723`, `internal/run/run.go:48-60`) and before Acceptance. Reclaims during the 10s lease TTL MUST be rejected with `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). After expiry, a replacement Orchestrator MUST reclaim ownership, verify zero orphan processes in `/proc`, adopt the saved envelope without re-dispatch, and promote (`internal/feature/feature.go:406-473`). Surviving processes MUST fail the Trial (`internal/executor/agy.go:193-205`).

#### Scenario: Lease reclaim blocked during 10s window

- GIVEN terminated Change B with active 10s lease
- WHEN replacement attempts early reclaim
- THEN store MUST reject acquisition with `ErrLeaseHeld` (`internal/feature/feature.go:334-398`).

#### Scenario: Reclaim reuses envelope and proves zero survivors

- GIVEN expired lease on Change B with persisted envelope
- WHEN replacement reclaims ownership after 10s
- THEN it MUST adopt the saved envelope and promote without AI re-dispatch (`internal/feature/feature.go:406-473`, `cmd/lucind-ai/cli.go:710-723`).

### Requirement: Isolated Target Promotion and Ancestry Verification

Change B MUST promote to its Integration Target before Fix completes (`internal/integrate/integrate.go:127-138`). Fix MUST modify only authorized scope and promote to Change A target (`internal/integrate/integrate.go:153-175`). Change A MUST resume under original identity via new `agy` dispatch without reclaim (`internal/feature/feature.go:115-130`). Git ancestry MUST prove Change A target contains Fix+A commits while Change B target contains only B commits (`internal/run/attempt.go:740-750`, `internal/overlap/overlap.go:21-23`).

#### Scenario: Independent change promotes before fix

- GIVEN Change B completes lane work
- WHEN Promotion executes
- THEN Change B MUST fast-forward its target before Fix integration (`internal/integrate/integrate.go:127-138`).

#### Scenario: Ancestry confirms zero target cross-contamination

- GIVEN Changes A, B, and Fix integrated into their targets
- WHEN ancestry validation inspects history
- THEN Change A target MUST contain Fix+A, and Change B target MUST contain only B (`internal/run/attempt.go:740-750`, `internal/overlap/overlap.go:21-23`).

### Requirement: Storage Authority, Sanitized Evidence, and Terminal Receipt

Campaign state MUST be stored in SQLite/WAL at `<git-common-dir>/lucind-ai/stability/v1/stability.db`, isolated from `<primary-root>/.lucind/lucind.db` (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledgerpath/ledgerpath.go:40-58`). CLI MUST provide read-only `stability status [--json]` (`cmd/lucind-ai/cli.go:123-145`). Interrupted campaigns MUST support fail-closed resume and idempotent abort without AI dispatches (`internal/reconcile/reconcile.go:1-33`, `internal/worktree/worktree.go:247-269`), entering `blocked_cleanup` on residue. Evidence MUST be sanitized before deleting infrastructure (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`, `internal/worktree/worktree.go:247-269`). When three Trials pass, post-cleanup `lucind-ai check` MUST pass before emitting canonical JSON receipt binding candidate SHA, build, and Trial Records without tagging or pushing (`internal/integrate/integrate.go:100-112`, `docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:7-16`). Latest terminal Campaign determines certification (`docs/adr/0001-native-stability-campaign.md:23-24`).

#### Scenario: Single active campaign constraint enforced

- GIVEN active Campaign in common-dir SQLite
- WHEN second `stability run` starts
- THEN transaction MUST reject creation and exit non-zero (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-58`).

#### Scenario: Passing campaign emits canonical receipt

- GIVEN three successful Trials and passing post-cleanup check
- WHEN Campaign completes
- THEN content-addressed receipt MUST be persisted without Git tags or pushes (`internal/integrate/integrate.go:100-112`, `docs/adr/0001-native-stability-campaign.md:15-24`).

## Open Questions

- [ ] Should `internal/stability/reconcile` share abstractions with `internal/reconcile/reconcile.go:1-33` or remain separate for crashed-trial cleanup?
- [ ] How will `internal/stability/store` resolve `<git-common-dir>` across checkouts and linked worktrees using `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`)?
- [ ] Should `stability status --json` output full Trial Record bodies or compact summary references?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Subcommand dispatch switch routing CLI commands and rejecting non-interactive stability invocations. |
| `cmd/lucind-ai/cli.go:140-142` | CLI version output reporting binary version, runtime, and OS architecture for build validation. |
| `cmd/lucind-ai/cli.go:710-723` | `PersistEnvelope` closure writing result envelope JSON to disk before worktree removal. |
| `docs/adr/0001-native-stability-campaign.md:15-24` | Accepted architectural decisions and operational boundaries for native stability campaigns. |
| `docs/adr/0001-native-stability-campaign.md:23-24` | Policy defining latest terminal campaign as current certification without repository release actions. |
| `internal/barrier/barrier.go:36-60` | `Evaluate` batch barrier checking terminal lane status and releasing completed lanes. |
| `internal/executor/agy.go:86-96` | Default and known model declarations pinning `gemini-3.7-flash-high` for `agy` execution. |
| `internal/executor/agy.go:193-205` | `exec.CommandContext` child process construction currently lacking `SysProcAttr` process group assignment. |
| `internal/executor/executor.go:57-78` | `Outcome` struct capturing exit code, timeout state, stderr/stdout, and truncation flags. |
| `internal/feature/feature.go:98-113` | `ValidateParentRef` enforcing parent branch namespace and naming rules. |
| `internal/feature/feature.go:115-130` | `Create` recording feature lifecycle transition from created to active. |
| `internal/feature/feature.go:334-398` | `AcquireLease` creating or re-acquiring expiring feature lease with monotonic fence token. |
| `internal/feature/feature.go:406-473` | `RenewLease` validating active ownership and extending lease duration. |
| `internal/integrate/integrate.go:52-85` | `Combine` orchestrating multi-branch merge into an integration worktree. |
| `internal/integrate/integrate.go:100-112` | `Check` executing `lucind-checks.sh` verification script at worktree root. |
| `internal/integrate/integrate.go:127-138` | `Promote` checking `git status --porcelain` before fast-forwarding primary root. |
| `internal/integrate/integrate.go:153-175` | `PromoteCAS` atomically advancing target parent ref via compare-and-swap update-ref. |
| `internal/ledger/ledger.go:162-185` | `openAtPath` configuring SQLite connection pool with WAL pragma and busy timeout. |
| `internal/ledgerpath/ledgerpath.go:34-38` | `Resolve` constructing ledger database path under primary root's `.lucind` directory. |
| `internal/ledgerpath/ledgerpath.go:40-58` | `Validate` enforcing database paths remain contained within `.lucind` directory. |
| `internal/overlap/overlap.go:21-23` | Sentinel errors `ErrNoMergeBase` and `ErrMultipleMergeBases` for git divergence classification. |
| `internal/reconcile/reconcile.go:1-33` | Package documentation and sentinel errors for reconciliation requests and lifecycle decisions. |
| `internal/run/attempt.go:740-750` | `evaluateOverlapGate` verifying commit SHA ancestry against target parent ref. |
| `internal/run/batch.go:66-78` | `ExecuteBatch` constructing batch barrier and orchestrating concurrent lane goroutines. |
| `internal/run/run.go:48-60` | Result schema and envelope constants defining `.lucind/result.json` path. |
| `internal/run/run.go:71-90` | `streamDetailCap` constant bounding per-stream output size in ledger notes. |
| `internal/run/run.go:131-150` | `diagnosisDetail` and `formatStreamDetail` truncating and formatting captured output streams. |
| `internal/worktree/worktree.go:47-69` | `GitRunner` interface and `DefaultGitRunner` executing git commands. |
| `internal/worktree/worktree.go:173-238` | `CreateWithParent` adding linked git worktree anchored at base commit SHA. |
| `internal/worktree/worktree.go:247-269` | `Cleanup`, `Remove`, and `DeleteBranch` safely removing worktree directories and branches. |
| `lucind-ai-stability-run-sdd-master-plan.md:7-16` | Product command definition and outcomes for native stability campaigns. |
| `lucind-ai-stability-run-sdd-master-plan.md:59-86` | Decision ledger A defining stability evidence rules and 3-trial canonical journey. |
