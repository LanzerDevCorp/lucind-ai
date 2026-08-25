# Design: Native Stability Campaign

## Technical Approach

We introduce native release certification via `lucind-ai stability run|status|resume|abort` (`cmd/lucind-ai/cli.go:123-145`, `docs/adr/0001-native-stability-campaign.md:5-24`), designed against `proposal.md`. The engine executes three sequential 5-dispatch Stability Trials pinned to `gemini-3.7-flash-high` (`internal/executor/agy.go:85-96`) with zero retries. Ephemeral targets run Changes A and B concurrently (`internal/run/batch.go:66-78`); Change A hits a defect and emits a Remediation Proposal for Fix Change dispatch, while Change B is killed via `SIGKILL`, verified for zero `/proc` survivors via process group (`internal/executor/agy.go:19-40`, `internal/executor/agy.go:193-205`), and recovered via 10s lease reclaim (`internal/feature/feature.go:334-398`, `internal/feature/feature.go:406-473`) and envelope adoption (`internal/run/run.go:56-60`, `cmd/lucind-ai/cli.go:710-723`). Authority is isolated in `<git-common-dir>` SQLite/WAL (`internal/ledger/ledger.go:162-185`), emitting an RFC 8785 JSON receipt after post-cleanup baseline pass (`internal/integrate/integrate.go:100-120`).

## Architecture Decisions

### Decision 1: Modular Subpackages under `internal/stability/`
**Choice**: Subpackages `store`, `fixture`, `process`, `evidence`, `reconcile` under `internal/stability/`.
**Alternatives considered**: Monolithic package; scattering across top-level packages.
**Rationale**: Isolates storage, processes, fixtures, sanitization, recovery (`lucind-ai-stability-run-sdd-master-plan.md:240-252`). Keeps `cmd/lucind-ai/cli.go:123-145` thin.
**Terminal consumer**: `cmd/lucind-ai/cli.go:123-145`.

### Decision 2: Optional Process-Group Isolation via `executor.Request.Setpgid`
**Choice**: Add optional `Setpgid bool` to `executor.Request` (`internal/executor/executor.go:27-52`); configure `SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` in `internal/executor/agy.go:193-205` on Linux when true.
**Alternatives considered**: Unconditional `Setpgid: true` on Linux; separate stability executor.
**Rationale**: Avoids altering process semantics for batch execution (`internal/run/batch.go:66-78`) while letting `internal/stability/process` contain grandchild MCP processes (`internal/executor/agy_test.go:158-191`).
**Terminal consumer**: `internal/executor/agy.go:193-205` from `executor.Request` (`internal/executor/executor.go:27-52`).

### Decision 3: Common-Dir Path Resolution via `worktree.GitRunner`
**Choice**: Resolve Git common directory using `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`) calling `git rev-parse --git-common-dir`, normalizing relative paths.
**Alternatives considered**: Parsing `.git` files; stripping `-worktrees` suffix; reusing `internal/ledgerpath/ledgerpath.go:34-58`.
**Rationale**: In linked worktrees (`internal/worktree/worktree.go:173-238`), `.git` points to git metadata files. Git `--git-common-dir` is authoritative across primary roots and worktrees. `internal/ledgerpath/ledgerpath.go:34-58` rejects non-`.lucind` paths.
**Terminal consumer**: `internal/stability/store` and `internal/worktree/worktree.go:173-238`.

### Decision 4: Dedicated Common-Dir SQLite Authority and Single-Active Gate
**Choice**: Store mutable state in SQLite/WAL at `<git-common-dir>/lucind-ai/stability/v1/stability.db` matching `internal/ledger/ledger.go:146-185`, enforcing single-active campaign via conditional transaction.
**Alternatives considered**: Storing in primary ledger at `<primaryRoot>/.lucind/lucind.db`; flat JSON files.
**Rationale**: Common-dir persistence survives checkout switches while leaving `<primaryRoot>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-185`, `internal/ledgerpath/ledgerpath.go:34-58`) untouched and read-only (`docs/adr/0001-native-stability-campaign.md:20-24`).
**Terminal consumer**: `internal/stability/store` in `lucind-ai stability run` (`cmd/lucind-ai/cli.go:123-145`).

### Decision 5: Domain Separation between Stability and Branch Reconcile
**Choice**: Implement stability recovery in `internal/stability/reconcile` for interruption, residue cleanup (`internal/worktree/worktree.go:247-269`), and survivor auditing, without depending on `internal/reconcile/reconcile.go:1-33`.
**Alternatives considered**: Overloading `internal/reconcile/reconcile.go:1-33`.
**Rationale**: `internal/reconcile/reconcile.go:1-33` manages branch merge conflict approvals. Stability reconciliation handles crash inspection, fail-closed resume, and residue purges (`internal/worktree/worktree.go:247-269`).
**Terminal consumer**: `lucind-ai stability resume` and `abort` (`cmd/lucind-ai/cli.go:123-145`).

### Decision 6: Sequential Three-Trial State Machine with Strict Zero-Retry Reset
**Choice**: Sequential 3-Trial state machine with zero retries. Failure or timeout halts active Trial, resets consecutive count to 0 (`internal/barrier/barrier.go:36-60`), and transitions Campaign to `failed` (or `blocked_cleanup`).
**Alternatives considered**: Slot retries; resuming failed Trials midway.
**Rationale**: Certification requires three consecutive pass journeys (`docs/adr/0001-native-stability-campaign.md:5-24`). Retries conceal instability; reset guarantees integrity.
**Terminal consumer**: `internal/stability` engine (`internal/barrier/barrier.go:36-60`).

### Decision 7: Full Forensic Trial Record Emission in Status JSON
**Choice**: Emit full Trial Record bodies (transitions, timings, sanitized stream details, defect records, payload hashes) in `lucind-ai stability status --json`.
**Alternatives considered**: Compact summary references; unstructured text logs.
**Rationale**: Release gates require forensic inspectability without direct SQLite access (`cmd/lucind-ai/cli.go:123-145`, `docs/adr/0001-native-stability-campaign.md:20-24`).
**Terminal consumer**: `cmd/lucind-ai/cli.go:123-145` and `internal/stability/evidence`.

### Decision 8: Crash Recovery via Monotonic Lease Reclaim and Envelope Adoption
**Choice**: Simulate Change B crash via `SIGKILL` after result persistence in `.lucind/result.json` (`internal/run/run.go:56-60`, `cmd/lucind-ai/cli.go:710-723`). Reclaim during 10s TTL returns `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). After expiry, replacement B acquires lease with incremented fence (`internal/feature/feature.go:406-473`), verifies zero `/proc` survivors, and adopts envelope (`internal/run/run.go:56-60`, `cmd/lucind-ai/cli.go:710-723`) without redispatch.
**Alternatives considered**: Re-executing Change B; recovering without checking `/proc`.
**Rationale**: Proves envelopes survive crashes (`cmd/lucind-ai/cli.go:710-723`); lease fencing prevents races (`internal/feature/feature.go:334-398`, `internal/feature/feature.go:406-473`).
**Terminal consumer**: `internal/feature/feature.go:334-398`, `internal/feature/feature.go:406-473`, `cmd/lucind-ai/cli.go:710-723`, `internal/run/run.go:56-60`.

### Decision 9: Canonical RFC 8785 JSON Stability Receipt with Final Baseline Check
**Choice**: Persist immutable RFC 8785 JSON Stability Receipts at `<git-common-dir>/lucind-ai/stability/v1/receipts/<receipt_id>.json` only after three passing Trials AND post-cleanup `integrate.Check` baseline pass (`cmd/lucind-ai/cli.go:503-509`, `internal/integrate/integrate.go:100-120`). Logs use capped sanitization (`streamDetailCap = 4096`, `internal/run/run.go:71-90`, `internal/run/run.go:131-150`).
**Alternatives considered**: Emitting receipt before baseline check; Git tags/pushes; storing unsanitized logs.
**Rationale**: Post-cleanup check verifies baseline cleanliness (`internal/integrate/integrate.go:100-120`, `internal/integrate/integrate.go:126-138`). Git tags remain out of scope (`docs/adr/0001-native-stability-campaign.md:20-24`). Sanitization prevents leakage (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`).
**Terminal consumer**: `internal/stability/evidence` and `cmd/lucind-ai/cli.go:503-509`.

## Flow and Invariants

```text
CLI (run|status|resume|abort) ──► Preflight ──► SQLite Authority (<git-common-dir>/.../stability.db)
                                      │
  ┌───────────────────────────────────┴────────────────────────────────────┐
  ▼                                                                        ▼
Trials 1..3 Sequential (Change A defect -> Fix; Change B crash -> Reclaim) ──► Receipt
```

1. **Preflight Admission (`cmd/lucind-ai/cli.go:123-145` ──→ `internal/stability/store/`):** Validates Linux, clean tree (`internal/integrate/integrate.go:126-138`), candidate `HEAD` build match (`cmd/lucind-ai/cli.go:140-142`), baseline check pass (`cmd/lucind-ai/cli.go:503-509`, `internal/integrate/integrate.go:100-120`), `gemini-3.7-flash-high` availability (`internal/executor/agy.go:85-96`), zero active campaigns (`internal/ledger/ledger.go:162-185`), and confirmation. Dirty tree, stale binary, or active campaign halts non-zero.
2. **Sequential Three-Trial Scheduling (`internal/stability/` ──→ `internal/run/batch.go:66-78`):** Executes 3 Trials sequentially; Trial N+1 starts only after verified cleanup of Trial N worktrees/branches (`internal/worktree/worktree.go:247-269`); failure/timeout resets count to 0 without retry (`internal/barrier/barrier.go:36-60`). Incomplete cleanup halts on worktree collision (`internal/worktree/worktree.go:173-238`).
3. **Concurrent Journey & Remediation (`internal/stability/fixture/` ──→ `internal/integrate/integrate.go:52-85`):** Runs Changes A and B concurrently on ephemeral targets with leases (`internal/feature/feature.go:98-113`, `internal/run/batch.go:66-78`). Change A hits defect, persists Defect Record, awaits Test Actor approval; Fix Change launches to Target A while Change B proceeds (`internal/feature/feature.go:115-130`).
4. **Crash Recovery & Containment (`internal/stability/process/` ──→ `internal/feature/feature.go:334-473`):** Change B killed via `SIGKILL` after envelope write (`cmd/lucind-ai/cli.go:710-723`, `internal/run/run.go:48-60`). Reclaim during 10s TTL returns `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). Replacement B reclaims after expiry, verifies zero `/proc` survivors via process group (`internal/executor/agy.go:19-40`, `internal/executor/agy.go:193-205`), adopts envelope, and promotes B (`internal/feature/feature.go:406-473`). Surviving processes fail Trial.
5. **Target Promotion & Receipt (`internal/integrate/integrate.go:126-175` ──→ `internal/stability/evidence/`):** Change B promotes to Target B before Fix (`internal/integrate/integrate.go:126-148`); Fix promotes to Target A (`internal/integrate/integrate.go:153-175`); Change A resumes and promotes. Ancestry verifies Target A contains Fix+A and Target B contains only B (`internal/run/attempt.go:740-750`, `internal/overlap/overlap.go:21-23`). Logs sanitized to 4096B (`internal/run/run.go:71-90`, `internal/run/run.go:131-150`); post-cleanup check passes (`internal/integrate/integrate.go:100-120`); JSON receipt persisted.

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `cmd/lucind-ai/cli.go` | Modify | Route `stability` subcommand | `cmd/lucind-ai/cli.go:123-145` |
| `cmd/lucind-ai/stability.go`, `stability_test.go` | Create | Flag parsing, preflight UI, status output | `cmd/lucind-ai/cli.go:123-145` |
| `internal/executor/executor.go` | Modify | Add `Setpgid bool` to `Request` | `internal/executor/executor.go:27-52` |
| `internal/executor/agy.go` | Modify | Set `SysProcAttr` with `Setpgid: true` on Linux | `internal/executor/agy.go:193-205` |
| `internal/stability/campaign.go`, `campaign_test.go` | Create | State machines, budgets, transitions | `internal/barrier/barrier.go:36-60` |
| `internal/stability/store/store.go`, `store_test.go` | Create | SQLite/WAL store, single-active gate | `internal/ledger/ledger.go:162-185` |
| `internal/stability/fixture/fixture.go`, `fixture_test.go` | Create | Templates, check scripts, packets | `internal/worktree/worktree.go:173-238` |
| `internal/stability/process/process.go`, `process_test.go` | Create | Process supervision, `SIGKILL`, `/proc` audit | `internal/executor/agy.go:19-40` |
| `internal/stability/evidence/evidence.go`, `receipt.go`, `evidence_test.go` | Create | Sanitization, hashing, Trial Records, receipts | `internal/run/run.go:71-90` |
| `internal/stability/reconcile/reconcile.go`, `reconcile_test.go` | Create | Crash reconciliation, `blocked_cleanup` | `internal/worktree/worktree.go:247-269` |

## Testing Strategy and Test Seams

### Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit | State transitions | Transitions (`preflight->running->passed/failed/blocked_cleanup`), counter (0..3), reset, timeouts. | `internal/barrier/barrier_test.go:31-60` |
| Unit | Storage schema | SQLite schema init, single-active gate, crash recovery. | `internal/ledger/ledger_test.go:35-58` |
| Unit | Path resolution | Resolve `<git-common-dir>` path, reject external paths. | `internal/ledgerpath/ledgerpath_test.go:9-35` |
| Unit | 10s lease & fence | 10s TTL, early reclaim rejection, takeover at t>=10s, fence increment. | `internal/feature/feature_test.go:278-310` |
| Unit | Sanitization & receipt | Strip paths (`streamDetailCap = 4096`), hashes, RFC 8785 JSON. | `internal/run/run.go:71-90` |
| Integration | Process group & `/proc` | Test `Setpgid: true`, `SIGKILL` to `-pgid`, survivor detection with stubs. | `internal/executor/agy_test.go:158-191` |
| Integration | Fixture defect | Seeded defect, Remediation, Fix integration, ancestry (`merge-base --is-ancestor`). | `internal/worktree/worktree.go:203-209` |
| Integration | Worktree cleanup | Idempotent removal, assert `blocked_cleanup` on residue. | `internal/worktree/worktree.go:247-269` |
| Integration | CLI preflight | Porcelain check, candidate build match, non-interactive rejection. | `cmd/lucind-ai/cli_test.go:40-60` |
| E2E | 3-Trial simulated run | 3-Trial simulated run verifying crash, lease, Fix, and receipt via stubs. | `internal/run/batch_test.go:26-59` |
| E2E | Native acceptance | Baseline checks (`integrate.Check`); real 3-Trial Campaign outside `go test ./...`. | `internal/integrate/integrate.go:100-120` |

### Test Seams
- **Existing Seams**: `executor.Agy.Binary` / `writeStub` (`internal/executor/agy_test.go:18-26`, `internal/executor/agy.go:193-205`); `worktree.GitRunner` (`internal/worktree/worktree.go:173-238`, `internal/integrate/integrate.go:153-180`); `run.Deps` (`internal/run/run.go:205-229`); `integrate.Check` / `integrate.Promote` (`internal/integrate/integrate.go:100-138`); `ledger.Open` / `openAtPath` (`internal/ledger/ledger.go:162-185`).
- **New Seams**: Storage constructor (`internal/stability/store`); process supervisor (`internal/stability/process`); monotonic clock (`internal/stability`); Test Actor recorder (`internal/stability/fixture`).

## Threat Matrix

> [!NOTE]
> The reference matrix (`references/threat-matrix.md`) covers VCS/PR automation. The primary threat surface here is process supervision (`Setpgid: true`, `SIGKILL`, `/proc` survivor audit, lease races), handled via the dedicated process package and test seams above. The reference matrix is included per specification rules:

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, Markdown, `README.sh` | N/A: Synthetic templates and agy CLI without path classification. | N/A — no classification boundary | None (N/A) |
| Git repository selection | `git -C`, relative/absolute paths | Applicable | Resolves Git common-dir via `git rev-parse --git-common-dir`; rejects non-repo dirs. | `TestPreflightRejectsNonGitWorkingDir`, `TestPreflightResolvesGitCommonDirAuthority` |
| Commit state | staged, `commit -a`, empty index | Applicable | Preflight runs `git status --porcelain`; aborts on uncommitted changes. | `TestPreflightRejectsDirtyWorkingTreeStaged`, `TestPreflightRejectsDirtyWorkingTreeUntracked`, `TestPreflightRejectsDirtyWorkingTreeModified` |
| Push state | tracking branch, first push, refspec | N/A: V1 non-goal (Q54, Q370); no git push or remote ref updates. | N/A — no push boundary | None (N/A) |
| PR commands | `--head`, env prefix, composed commands | N/A: V1 non-goal (Q18, Q54, Q370); creates no PRs or forge commands. | N/A — no PR boundary | None (N/A) |

### Expected Safe & Failure Behaviors
- **Git repository selection**: Safe: resolves authority in `<git-common-dir>/lucind-ai/stability/v1/stability.db`. Failure: non-git directory aborts preflight non-zero without file creation.
- **Commit state**: Safe: executes only when `git status --porcelain` is empty. Failure: uncommitted modifications abort preflight with exit code 1.

## Rollback and Additivity

**Choice**: `git revert` of commits adding `internal/stability/` and `cmd/lucind-ai/cli.go:123-145` routing.
**Alternatives considered**: Database down-migrations (rejected: common-dir storage is isolated); primary ledger schema migrations (rejected: breaks additivity).
**Rationale**: Reverting code eliminates subcommands and packages. Format changes are strictly additive:
1. Primary ledger at `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:34-38`, `internal/ledger/ledger.go:146-148`) is untouched; Trial Records link Run IDs read-only.
2. Stability state lives exclusively in `<git-common-dir>/lucind-ai/stability/v1/stability.db` (`internal/ledger/ledger.go:162-185`, `internal/ledgerpath/ledgerpath.go:40-58`); historical data stays inert on rollback.
3. Receipts in `.lucind/results/` (`cmd/lucind-ai/cli.go:710-723`) are standalone JSON files.
4. No existing schema, ledger, or envelope versions move.

## Open Questions and Out of Scope

### Open Questions
- None. (All four open questions from `proposal.md` Section 9 are resolved: Decision 2 resolves process-group configuration via `executor.Request.Setpgid`, Decision 3 resolves common-dir resolution via `worktree.GitRunner`, Decision 5 resolves domain separation from `internal/reconcile/`, and Decision 7 resolves full forensic JSON emission in `stability status --json`).

### Out of Scope
- Non-Linux OS mutating execution; non-interactive bypasses, `--yes` flag, NIP, or secret storage; external issues, PRs, pushes, or releases; alternative AI executors/models (pinned to `gemini-3.7-flash-high`); primary Run ledger migrations; automatic AI retries or dynamic tuning; Control Room UI views.
