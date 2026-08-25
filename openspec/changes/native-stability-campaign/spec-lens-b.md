# Spec Lens B — Scenarios & Coverage: Native Stability Campaign

## Assumed requirements

This specification defines behavioral scenarios for ten stability capabilities from proposal (`openspec/changes/native-stability-campaign/proposal.md:43-51`) and master plan (`lucind-ai-stability-run-sdd-master-plan.md:173-211`): preflight admission, three-Trial scheduling, concurrent execution, remediation gating, process crash recovery, fixture promotion, content-bound verification, SQLite authority, evidence sanitization, and delivery boundaries.

## Scenarios

### Requirement: Campaign admission

#### Scenario: Preflight success
- GIVEN clean Linux checkout matching HEAD build with no active campaign
- WHEN `lucind-ai stability run` runs with `yes` confirmation
- THEN preflight succeeds and creates an active campaign record

#### Scenario: Dirty checkout rejected
- GIVEN uncommitted changes in primary worktree
- WHEN `lucind-ai stability run` executes
- THEN preflight exits non-zero without writing database rows or worktrees

### Requirement: Deterministic three-Trial scheduler

#### Scenario: Three passing Trials
- GIVEN active campaign at Trial 1
- WHEN Trials 1, 2, and 3 pass sequentially
- THEN consecutive count reaches 3 and triggers terminal verification

#### Scenario: Slot failure reset
- GIVEN active campaign in Trial 2
- WHEN any lane or slot fails
- THEN consecutive counter resets to 0 and campaign terminates

### Requirement: Real concurrent Changes

#### Scenario: Concurrent lane execution
- GIVEN ephemeral integration targets for Changes A and B
- WHEN scheduler dispatches lane packets concurrently
- THEN both Orchestrators hold active leases before initiating promotion

#### Scenario: Pinned model enforcement
- GIVEN dispatch requests for Changes A and B
- WHEN requests pass to executor
- THEN every dispatch enforces pinned model `gemini-3.7-flash-high`

### Requirement: Defect and remediation flow

#### Scenario: Remediation proposal approval
- GIVEN Change A encounters fixture defect outside Write Scope
- WHEN Change A emits Defect Record and Remediation Proposal
- THEN Test Actor approves proposal and dispatches Fix Change

#### Scenario: Independent Change B execution
- GIVEN Change A blocked on Fix Change dependency
- WHEN Change B executes concurrently
- THEN Change B runs to completion without blocking on Change A

### Requirement: Crash and ownership recovery

#### Scenario: Expired lease reclaim and envelope adoption
- GIVEN Orchestrator B terminated by SIGKILL after persisting envelope
- WHEN replacement Orchestrator B reclaims after 10s lease expiry
- THEN replacement adopts persisted envelope and promotes without re-dispatch

#### Scenario: Early reclaim rejected
- GIVEN Orchestrator B killed with active unexpired 10s lease
- WHEN replacement attempts acquisition before expiry
- THEN store returns `ErrLeaseHeld` and increments fence counter

### Requirement: Fix, resumption, and Promotion

#### Scenario: Fix promotion and resumption
- GIVEN completed Fix Change modifying authorized scope
- WHEN Fix Change promotes to Target A
- THEN Change A unblocks, resumes under original identity, and passes verification

#### Scenario: Independent Change B promotion
- GIVEN Change B finished while Fix Change is running
- WHEN Change B initiates promotion
- THEN Change B fast-forwards Target B independently before Fix completes

### Requirement: Content-bound verification

#### Scenario: Commit ancestry isolation
- GIVEN completed integration targets A and B
- WHEN git ancestry is verified against base commits
- THEN Target A contains Fix and A commits while Target B contains only B commits

#### Scenario: Contaminated target rejection
- GIVEN Target B containing commits originating from Change A or Fix Change
- WHEN cross-target isolation check runs
- THEN verification fails immediately and invalidates trial

### Requirement: Durable authority and recovery

#### Scenario: Idempotent abort cleanup
- GIVEN interrupted campaign in `blocked_cleanup` state
- WHEN operator executes `lucind-ai stability abort`
- THEN residual worktrees and branches are purged without AI dispatches

#### Scenario: Concurrent campaign rejection
- GIVEN active campaign in progress
- WHEN second `lucind-ai stability run` executes
- THEN transaction rejects second run and exits non-zero

### Requirement: Evidence and receipt

#### Scenario: Stability receipt generation
- GIVEN three passed trials and clean baseline check
- WHEN campaign finalizes successfully
- THEN canonical JSON Stability Receipt is written binding SHA, build, and trial records

#### Scenario: Diagnostic log sanitization
- GIVEN diagnostic streams exceeding 4096 bytes
- WHEN evidence records are persisted
- THEN output is truncated to 4096 bytes, payloads hashed, and paths stripped

### Requirement: Delivery boundary

#### Scenario: Non-mutating release exit
- GIVEN certified campaign with approved receipt
- WHEN command execution completes
- THEN process exits 0 without creating git tags or pushing remotes

#### Scenario: Prohibited release flag rejection
- GIVEN `stability run` invoked with `--push`, `--tag`, or `--release`
- WHEN CLI argument parsing evaluates options
- THEN command rejects invalid flags and halts with exit code 1

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Campaign admission | covered | missing | covered | `cmd/lucind-ai/cli.go:123-145` |
| Deterministic three-Trial scheduler | covered | missing | covered | `internal/barrier/barrier.go:36-60` |
| Real concurrent Changes | covered | covered | missing | `internal/run/batch.go:66-78` |
| Defect and remediation flow | covered | covered | missing | `internal/feature/feature.go:98-113` |
| Crash and ownership recovery | covered | missing | covered | `internal/feature/feature.go:334-398` |
| Fix, resumption, and Promotion | covered | covered | missing | `internal/integrate/integrate.go:126-138` |
| Content-bound verification | covered | missing | covered | `internal/run/attempt.go:740-750` |
| Durable authority and recovery | covered | missing | covered | `internal/ledger/ledger.go:162-185` |
| Evidence and receipt | covered | covered | missing | `cmd/lucind-ai/cli.go:710-723` |
| Delivery boundary | covered | missing | covered | `internal/integrate/integrate.go:100-120` |

## Untestable Assertions

None

## Open Questions

- [ ] Secondary edge and error scenarios were omitted from this draft to respect the 1000-word budget and deferred to synthesis.
- [ ] Whether process-group isolation (`Setpgid: true`) in `internal/executor/agy.go:193-205` should apply unconditionally on Linux or be gated by a field in `executor.Request` (`internal/executor/executor.go:27-52`).
- [ ] Whether `stability status --json` returns full serialized Trial Records or compact status summaries (`cmd/lucind-ai/cli.go:123-145`).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Subcommand dispatch routing in main CLI entrypoint. |
| `cmd/lucind-ai/cli.go:503-509` | Baseline check execution and error handling. |
| `cmd/lucind-ai/cli.go:710-723` | JSON result envelope formatting and persistence. |
| `docs/adr/0001-native-stability-campaign.md:5-6` | Decision record defining stability campaign architecture and model pinning. |
| `docs/adr/0001-native-stability-campaign.md:15-24` | Architecture decision defining preflight gates, three-trial lifecycle, and receipt generation. |
| `internal/barrier/barrier.go:36-60` | Pure barrier evaluation over observed lane terminal states. |
| `internal/executor/agy.go:85-96` | Default and allowed model definitions for agy executor. |
| `internal/executor/agy.go:193-205` | Child command configuration and execution in target worktree. |
| `internal/executor/executor.go:27-52` | Request struct defining prompt, worktree path, model, and schema options. |
| `internal/executor/executor.go:57-78` | Outcome struct capturing exit code, timeouts, and process stream output. |
| `internal/executor/executor.go:80-95` | Executor interface declaring Run, DefaultModel, and KnownModels methods. |
| `internal/feature/feature.go:98-113` | Parent ref validation rejecting reserved namespaces and invalid names. |
| `internal/feature/feature.go:115-130` | Feature creation and immutable anchor validation. |
| `internal/feature/feature.go:334-398` | Expiring feature lease acquisition with monotonic fencing token. |
| `internal/feature/feature.go:406-473` | Lease renewal verifying current owner and fence. |
| `internal/integrate/integrate.go:100-120` | Canonical check script execution and output evaluation. |
| `internal/integrate/integrate.go:126-138` | Fast-forward promotion guarded by git porcelain cleanliness check. |
| `internal/ledger/ledger.go:162-185` | SQLite WAL connection configuration and pool parameters. |
| `internal/ledgerpath/ledgerpath.go:34-38` | Primary repository ledger file path resolution. |
| `internal/ledgerpath/ledgerpath.go:40-58` | Validation ensuring paths stay within the primary repository root. |
| `internal/overlap/overlap.go:21-23` | Merge base error definitions for ancestry and overlap checks. |
| `internal/reconcile/reconcile.go:20-33` | Sentinel error definitions for reconciliation state transitions. |
| `internal/run/attempt.go:740-750` | Feature parent ref canonicalization and commit resolution. |
| `internal/run/batch.go:66-78` | Batch execution initializing barrier across unique packet IDs. |
| `internal/run/run.go:48-60` | Result envelope path and metadata constants within worktrees. |
| `internal/run/run.go:71-90` | Per-stream detail size cap constant definition. |
| `internal/run/run.go:131-150` | Diagnosis detail formatting and stream truncation logic. |
| `internal/worktree/worktree.go:173-238` | Linked worktree creation off parent ref with commit validation. |
| `internal/worktree/worktree.go:247-269` | Worktree cleanup and branch removal helpers. |
| `lucind-ai-stability-run-sdd-master-plan.md:173-211` | Product requirements R1 through R10 defining stability run behaviors. |
| `openspec/changes/native-stability-campaign/proposal.md:43-51` | Capability impact mapping and component seam definitions. |
