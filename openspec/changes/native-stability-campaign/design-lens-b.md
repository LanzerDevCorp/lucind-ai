# Design Lens B — Surface & Flow: Native Stability Campaign

## Assumed architecture

This design adopts Candidate 2 (Modular Subpackage Decomposition under `internal/stability/`), adding subpackages `store`, `fixture`, `process`, `evidence`, and `reconcile`. It routes `stability` in `cmd/lucind-ai/cli.go:123-145` and configures Linux process groups (`Setpgid: true`) in `internal/executor/agy.go:193-205`. The Run ledger (`internal/ledger/ledger.go:146-148`, `internal/ledgerpath/ledgerpath.go:34-38`) remains untouched; stability authority resides at `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`).

## Flow and Invariants

```text
Operator CLI (run|status|resume|abort)
     │
     ▼
Preflight Admission ──→ SQLite Authority (<git-common-dir>/lucind-ai/stability/v1/stability.db)
     │
     ▼
Three-Trial Engine (1..3 Sequential)
     │
     ├──► Concurrent Journey (A & B Ephemeral Targets)
     │         │
     │         ├──► A Defect Discovery ──→ Defect Record ──→ Test Actor ──→ Fix Change ──► Target A
     │         │
     │         └──► B Crash (SIGKILL) ──→ 10s Lease Wait ──→ Zero /proc Survivors ──→ Adopt Envelope ──► Target B
     │
     ▼
Ancestry & Evidence Verification (Sanitize logs <=4096B, Hash payloads)
     │
     ▼
Post-Cleanup Baseline Check ──→ Canonical JSON Stability Receipt
```

1. **Preflight Admission (`cmd/lucind-ai/cli.go:123-145` ──→ `internal/stability/store/`):**
   - *Invariant:* Validates Linux, clean tree (`internal/integrate/integrate.go:126-148`), HEAD/build match (`cmd/lucind-ai/cli.go:140-142`), baseline check pass (`cmd/lucind-ai/cli.go:503-509`, `internal/integrate/integrate.go:100-120`), `gemini-3.7-flash-high` availability (`internal/executor/agy.go:85-96`), zero active campaigns, and confirmation.
   - *Observably breaks:* Dirty tree, stale binary, or active campaign halts non-zero without state mutation or worktrees.

2. **Sequential Three-Trial Scheduling (`internal/stability/` ──→ `internal/run/batch.go:66-78`):**
   - *Invariant:* Executes 3 Trials sequentially; Trial N+1 starts only after verified cleanup of Trial N worktrees/branches (`internal/worktree/worktree.go:247-269`); any slot failure or timeout resets count to 0 and fails Campaign without retry (`internal/barrier/barrier.go:36-60`).
   - *Observably breaks:* Worktree collisions (`internal/worktree/worktree.go:173-238`) on incomplete cleanup; non-zero counts surviving failures hide instability.

3. **Concurrent Journey & Remediation (`internal/stability/fixture/` ──→ `internal/integrate/integrate.go:52-85`):**
   - *Invariant:* Runs Changes A and B concurrently on ephemeral targets with active leases (`internal/feature/feature.go:98-113`, `internal/run/batch.go:66-78`). Change A hits an out-of-scope defect, persists Defect Record, and awaits Test Actor approval; Fix Change launches to Target A while Change B proceeds (`internal/feature/feature.go:115-130`).
   - *Observably breaks:* Out-of-scope fixture edits violate Write Scope; Change B blocking on Change A destroys concurrency.

4. **Crash Recovery & Containment (`internal/stability/process/` ──→ `internal/feature/feature.go:334-473`):**
   - *Invariant:* Change B is killed via SIGKILL after envelope persistence (`cmd/lucind-ai/cli.go:710-723`, `internal/run/run.go:48-60`). Reclaims during 10s lease TTL return `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). Replacement B reclaims after expiry, verifies zero `/proc` survivors via process group (`internal/executor/agy.go:19-40`, `internal/executor/agy.go:193-205`), adopts envelope, and promotes B (`internal/feature/feature.go:406-473`).
   - *Observably breaks:* Early reclaim causes split-brain ownership; surviving MCP processes fail the Trial.

5. **Target Promotion & Receipt (`internal/integrate/integrate.go:126-175` ──→ `internal/stability/evidence/`):**
   - *Invariant:* Change B promotes to Target B before Fix completes (`internal/integrate/integrate.go:126-148`); Fix promotes to Target A (`internal/integrate/integrate.go:153-175`); Change A resumes under original identity and promotes; ancestry verifies Target A contains Fix+A and Target B contains only B (`internal/run/attempt.go:740-750`, `internal/overlap/overlap.go:21-23`); logs are sanitized to 4096B per stream (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`); post-cleanup check passes (`internal/integrate/integrate.go:100-120`); canonical JSON receipt is persisted without Git tags/pushes.
   - *Observably breaks:* Target cross-contamination invalidates isolation; post-cleanup check failure halts receipt issuance.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| CLI Command Router | `cmd/lucind-ai/cli.go:123-145` | Route `stability` subcommand to `stabilityDispatch` | Yes, additive routing |
| CLI `stability run` | `cmd/lucind-ai/cli.go:123-145` | Add interactive preflight admission and 3-Trial execution | Yes, additive CLI interface |
| CLI `stability status [--json]` | `cmd/lucind-ai/cli.go:123-145` | Add read-only status and JSON receipt inspection | Yes, additive CLI interface |
| CLI `stability resume` / `abort` | `cmd/lucind-ai/cli.go:123-145` | Add fail-closed resume and idempotent cleanup abort | Yes, additive CLI interface |
| `Agy.Run` Process Group | `internal/executor/agy.go:193-205` | Set `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on Linux | Yes, clean grandchild process cleanup |
| Storage Authority Path | `internal/ledgerpath/ledgerpath.go:40-58` | Resolve `<git-common-dir>/lucind-ai/stability/v1/stability.db` | Yes, isolated path outside `.lucind` |
| Storage SQLite Schema | `internal/ledger/ledger.go:162-185` | Add DDL for `campaigns`, `trials`, `defect_records`, `trial_events` | Yes, separate database schema |
| Lease TTL Parameter | `internal/feature/feature.go:334-398` | Invoke `AcquireLease(..., 10*time.Second)` | Yes, uses existing parameter signature |
| Sanitization Log Bounds | `internal/run/run.go:71-90` | Bound evidence log streams via `streamDetailCap = 4096` | Yes, reuses existing bounding standard |
| Stability Receipt JSON | `cmd/lucind-ai/cli.go:710-723` | Define RFC 8785 canonical JSON schema for terminal receipts | Yes, additive format in common-dir |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `cmd/lucind-ai/cli.go` | Modify | Route `stability` subcommand to `stabilityDispatch` | `cmd/lucind-ai/cli.go:123-145` |
| `cmd/lucind-ai/stability.go` | Create | CLI flag parsing, preflight UI, status output | `cmd/lucind-ai/cli.go:123-145` |
| `cmd/lucind-ai/stability_test.go` | Create | Test preflight, confirmation, and status JSON | `cmd/lucind-ai/cli.go:123-145` |
| `internal/executor/agy.go` | Modify | Add `SysProcAttr` with `Setpgid: true` on Linux | `internal/executor/agy.go:193-205` |
| `internal/stability/campaign.go` | Create | Campaign/Trial state machines, budgets, types | `cmd/lucind-ai/cli.go:123-145` |
| `internal/stability/campaign_test.go` | Create | Test transitions, failure resets, timeouts | `internal/barrier/barrier.go:36-60` |
| `internal/stability/store/store.go` | Create | SQLite/WAL store, single-active gate, migrations | `internal/ledger/ledger.go:162-185` |
| `internal/stability/store/store_test.go` | Create | Test single-active gate, transactions, rollback | `internal/ledger/ledger.go:162-185` |
| `internal/stability/fixture/fixture.go` | Create | Embedded templates, check scripts, packets | `internal/worktree/worktree.go:173-238` |
| `internal/stability/fixture/fixture_test.go` | Create | Test fixture generation, defect injection, digests | `internal/integrate/integrate.go:100-120` |
| `internal/stability/process/process.go` | Create | Process supervision, `SIGKILL`, `/proc` audit | `internal/executor/agy.go:19-40` |
| `internal/stability/process/process_test.go` | Create | Test process-group kill, survivor checks, lease wait | `internal/executor/agy.go:193-205` |
| `internal/stability/evidence/evidence.go` | Create | Log sanitization, SHA-256 hashing, Trial Records | `internal/run/run.go:71-90` |
| `internal/stability/evidence/receipt.go` | Create | RFC 8785 canonical JSON Stability Receipt | `cmd/lucind-ai/cli.go:710-723` |
| `internal/stability/evidence/evidence_test.go` | Create | Test log sanitization, payload hashing, receipts | `internal/run/run.go:131-150` |
| `internal/stability/reconcile/reconcile.go` | Create | Crash reconciliation, `blocked_cleanup`, cleanup | `internal/worktree/worktree.go:247-269` |
| `internal/stability/reconcile/reconcile_test.go` | Create | Test fail-closed resume and idempotent cleanup | `internal/reconcile/reconcile.go:1-33` |

## Open Questions

- [ ] Should `internal/executor/agy.go:193-205` configure `Setpgid: true` unconditionally on Linux, or should `internal/executor/executor.go:27-52` (`Request`) make it optional?
- [ ] How will `internal/stability/store` resolve `<git-common-dir>` across linked worktrees via `internal/worktree/worktree.go:47-69` (`GitRunner`)?
- [ ] Should `internal/stability/reconcile` reuse primitives from `internal/reconcile/reconcile.go:1-33` or maintain dedicated stability cleanup?
- [ ] Should `stability status --json` output full Trial Records or compact summary references?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Subcommand dispatch switch routing CLI subcommands and rejecting unknown subcommands. |
| `cmd/lucind-ai/cli.go:140-142` | CLI version output reporting binary version, runtime, and OS architecture for build validation. |
| `cmd/lucind-ai/cli.go:503-509` | Baseline check execution invoking `integrate.Check` during preflight and post-trial verification. |
| `cmd/lucind-ai/cli.go:710-723` | `PersistEnvelope` closure serializing result envelope JSON to disk. |
| `internal/barrier/barrier.go:36-60` | `Evaluate` pure function checking terminal lane status and releasing batch. |
| `internal/executor/agy.go:19-40` | `defaultWaitDelay` constant and doc explaining grandchild MCP process pipe drainage. |
| `internal/executor/agy.go:85-96` | `DefaultModel` and `KnownModels` methods pinning `gemini-3.7-flash-high` for `agy`. |
| `internal/executor/agy.go:193-205` | `Agy.Run` child process construction currently lacking `SysProcAttr` process group assignment. |
| `internal/executor/executor.go:27-52` | `Request` struct defining prompt, worktree path, model, schema path, and progress channel. |
| `internal/feature/feature.go:98-113` | `ValidateParentRef` enforcing parent branch namespace and naming rules. |
| `internal/feature/feature.go:115-130` | `Create` recording feature lifecycle transition from created to active. |
| `internal/feature/feature.go:334-398` | `AcquireLease` acquiring expiring feature lease with monotonic fence token. |
| `internal/feature/feature.go:406-473` | `RenewLease` validating active ownership and extending lease duration. |
| `internal/integrate/integrate.go:52-85` | `Combine` orchestrating multi-branch merge into an integration worktree. |
| `internal/integrate/integrate.go:100-120` | `Check` executing `lucind-checks.sh` verification script at worktree root. |
| `internal/integrate/integrate.go:126-148` | `Promote` checking `git status --porcelain` before fast-forwarding primary root. |
| `internal/integrate/integrate.go:153-175` | `PromoteCAS` atomically advancing target parent ref via compare-and-swap update-ref. |
| `internal/ledger/ledger.go:146-148` | `Open` opening ledger at `ledgerpath.Resolve(primaryRoot)`. |
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
