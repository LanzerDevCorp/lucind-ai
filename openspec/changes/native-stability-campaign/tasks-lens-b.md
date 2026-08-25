# Tasks Lens B — Partition & Dispatch Shape: Native Stability Campaign

## Assumed decomposition

The change partitions into 5 standalone work units across 3 capability boundaries: Unit 1 adds process-group isolation (`Setpgid bool`) to `executor.Request` (`internal/executor/executor.go:27-52`) and `internal/executor/agy.go:193-205`; Unit 2 implements dedicated `<git-common-dir>` SQLite storage authority and RFC 8785 JSON evidence receipts in `internal/stability/store/` (`internal/ledger/ledger.go:146-185`) and `internal/stability/evidence/` (`internal/run/run.go:71-90`); Unit 3 implements process supervision (`SIGKILL`, `/proc` audit) and synthetic test fixtures in `internal/stability/process/` (`internal/executor/agy.go:19-40`) and `internal/stability/fixture/` (`internal/worktree/worktree.go:173-238`); Unit 4 implements crash reconciliation, worktree residue cleanup (`internal/worktree/worktree.go:247-269`), and the sequential 3-Trial state machine with zero-retry reset (`internal/barrier/barrier.go:36-60`) in `internal/stability/reconcile/` and `internal/stability/campaign.go`; and Unit 5 routes `stability run|status|resume|abort`, preflight checks (`cmd/lucind-ai/cli.go:140-142`, `cmd/lucind-ai/cli.go:503-509`), and status JSON emission in `cmd/lucind-ai/cli.go:123-145` and `cmd/lucind-ai/stability.go`. The critical path is Unit 1 → Unit 3 for process supervision, Unit 2 + Unit 3 → Unit 4 for campaign orchestration, and Unit 4 → Unit 5 for CLI invocation, while Units 1, 2, and 3 provide orthogonal foundation deliverables.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| Unit 1: Process-Group Isolation | Add `Setpgid bool` to `executor.Request` and configure `SysProcAttr` with `Setpgid: true` on Linux | `internal/executor/executor.go`<br>`internal/executor/agy.go`<br>`internal/executor/agy_test.go` | `cursor-agent` | Reverts `executor.Request.Setpgid` and Linux `SysProcAttr` configuration, restoring default executor process spawning without affecting downstream packages. |
| Unit 2: Storage Authority & Evidence Receipts | Implement dedicated `<git-common-dir>` SQLite authority, single-active gate, stream sanitization, and RFC 8785 JSON receipts | `internal/stability/store/store.go` (new file)<br>`internal/stability/store/store_test.go` (new file)<br>`internal/stability/evidence/evidence.go` (new file)<br>`internal/stability/evidence/receipt.go` (new file)<br>`internal/stability/evidence/evidence_test.go` (new file) | `agy` | Removes `internal/stability/store/` and `internal/stability/evidence/` subpackages; `<git-common-dir>` SQLite DB stays inert without affecting primary ledger. |
| Unit 3: Process Supervision & Fixture Journey | Implement process-group supervision, `SIGKILL` termination, `/proc` survivor audit, and synthetic defect/crash test fixtures | `internal/stability/process/process.go` (new file)<br>`internal/stability/process/process_test.go` (new file)<br>`internal/stability/fixture/fixture.go` (new file)<br>`internal/stability/fixture/fixture_test.go` (new file) | `agy` | Removes `internal/stability/process/` and `internal/stability/fixture/` subpackages. |
| Unit 4: Stability Reconcile & Campaign State Machine | Implement stability crash reconciliation, residue worktree/branch purges, and sequential 3-Trial state machine with zero retries | `internal/stability/reconcile/reconcile.go` (new file)<br>`internal/stability/reconcile/reconcile_test.go` (new file)<br>`internal/stability/campaign.go` (new file)<br>`internal/stability/campaign_test.go` (new file) | `cursor-agent` | Removes `internal/stability/reconcile/` and `internal/stability/campaign.go` state machine. |
| Unit 5: CLI Subcommands & Preflight Admission | Route `stability` subcommand, implement `run|status|resume|abort`, preflight checks, and status JSON emission | `cmd/lucind-ai/cli.go`<br>`cmd/lucind-ai/stability.go` (new file)<br>`cmd/lucind-ai/stability_test.go` (new file) | `agy` | Reverts `cmd/lucind-ai/cli.go:123-145` routing and removes `cmd/lucind-ai/stability.go`, restoring prior CLI subcommand surface. |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| Wave 1 | Unit 1 | No | Yes: Unit 1 adds additive `Setpgid` field and Linux `SysProcAttr` configuration with unit tests; compiles and passes `lucind-checks.sh` independently. |
| Wave 2 | Unit 2, Unit 3 | Yes | Yes: Unit 2 provides standalone storage/evidence subpackages; Unit 3 provides standalone process/fixture subpackages consuming Wave 1 executor (`internal/run/batch.go:66-78`); pairwise path-disjoint; combined tree passes `lucind-checks.sh` independently. |
| Wave 3 | Unit 4 | No | Yes: Unit 4 implements reconciliation and campaign state machines consuming Wave 1–2 primitives; compiles and passes `lucind-checks.sh` independently. |
| Wave 4 | Unit 5 | No | Yes: Unit 5 mounts CLI routing and preflight checks consuming Wave 2 and Wave 4; compiles and passes all checks (`lucind-checks.sh`) including E2E CLI test suites. |

## Disjointness Check

- **Wave 1 (Unit 1)**: Single-unit wave (`internal/executor/executor.go`, `internal/executor/agy.go`, `internal/executor/agy_test.go`). Single-unit wave requires no pair check. Verdict: **DISJOINT (PASS)**.
- **Wave 2 (Unit 2 vs Unit 3)**:
  - Unit 2 `allowed_paths`: `internal/stability/store/store.go`, `internal/stability/store/store_test.go`, `internal/stability/evidence/evidence.go`, `internal/stability/evidence/receipt.go`, `internal/stability/evidence/evidence_test.go`
  - Unit 3 `allowed_paths`: `internal/stability/process/process.go`, `internal/stability/process/process_test.go`, `internal/stability/fixture/fixture.go`, `internal/stability/fixture/fixture_test.go`
  - Evaluation (`internal/packet/disjoint.go:13-22`, `internal/packet/disjoint.go:29-48`): All paths are concrete file paths within distinct subdirectories (`store/`, `evidence/` vs `process/`, `fixture/`); no path in Unit 2 is a component-boundary prefix of any path in Unit 3 and vice versa. Verdict: **DISJOINT (PASS)**.
- **Wave 3 (Unit 4)**: Single-unit wave (`internal/stability/reconcile/reconcile.go`, `internal/stability/reconcile/reconcile_test.go`, `internal/stability/campaign.go`, `internal/stability/campaign_test.go`). Single-unit wave requires no pair check. Verdict: **DISJOINT (PASS)**.
- **Wave 4 (Unit 5)**: Single-unit wave (`cmd/lucind-ai/cli.go`, `cmd/lucind-ai/stability.go`, `cmd/lucind-ai/stability_test.go`). Single-unit wave requires no pair check. Verdict: **DISJOINT (PASS)**.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**: The change spans 16 files (3 modified, 13 new) across `internal/executor/`, `internal/stability/`, and `cmd/lucind-ai/` (`openspec/changes/native-stability-campaign/design.md:81-93`), totaling ~1500–2200 lines. While the units partition into 4 waves with Wave 2 parallelizable, authoring `apply-dag.yaml`, provisioning `apply-bodies/`, running `lucind-ai split`, and managing multi-wave bisection overhead is not justified for a cohesive feature. Archived precedent `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` declined an `apply-dag.yaml` sidecar on identical reasoning, and `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml:8-14` demonstrates the risk of multi-wave DAG dispatch at the `Integrate` gate (`internal/run/integrate.go:50-59`). A single packet executed sequentially with 5 work-unit commits maintains modular rollback boundaries without sidecar orchestration overhead.

## Open Questions

- [ ] Contract divergence: `~/.claude/skills/sdd-tasks/SKILL.md` specifies a monolithic `tasks.md` with checklist, forecast, and Engram persistence, which is superseded by this 3-lens parallel task decomposition packet.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:123-145` | Subcommand router handling run, split, check, serve, feature, reconcile, worktree, integrate, and version |
| `cmd/lucind-ai/cli.go:140-142` | CLI version output format and runtime metadata matching candidate HEAD build |
| `cmd/lucind-ai/cli.go:503-509` | CLI preflight baseline integrate check execution |
| `cmd/lucind-ai/cli.go:710-723` | PersistEnvelope writing JSON formatted result envelopes into .lucind/results directory |
| `internal/barrier/barrier.go:36-60` | Evaluate function calculating batch barrier outcome without shared state or retries |
| `internal/executor/agy.go:19-40` | Agy executor struct, default wait delay, and subprocess runner definitions |
| `internal/executor/agy.go:85-96` | Agy known models registry defining gemini-3.7-flash-high provider binding |
| `internal/executor/agy.go:193-205` | CommandContext subprocess execution and stream tap wiring for agy dispatches |
| `internal/executor/executor.go:27-52` | Request struct defining prompt, worktree path, model, agent, and schema fields |
| `internal/integrate/integrate.go:52-85` | Combine function creating integration worktree and merging lane branches with conflict resolution |
| `internal/integrate/integrate.go:100-120` | Check function running project verification script in worktree and returning combined output |
| `internal/integrate/integrate.go:126-138` | Promote function verifying clean working tree via porcelain status before fast-forward |
| `internal/ledger/ledger.go:146-185` | Ledger Open and openAtPath configuring SQLite WAL mode, foreign keys, and connection pool |
| `internal/packet/disjoint.go:13-22` | PathInScope component-boundary prefix matching rule for POSIX repo paths |
| `internal/packet/disjoint.go:29-48` | DisjointAllowedPaths validating pairwise disjointness across packet path declarations |
| `internal/run/batch.go:66-78` | ExecuteBatch initializing barrier and validating lane IDs before dispatch |
| `internal/run/integrate.go:50-59` | Integrate gate running checks on combined tree and triggering bisection on failure |
| `internal/run/run.go:71-90` | streamDetailCap bounding captured stdout and stderr in ledger diagnostic notes |
| `internal/worktree/worktree.go:47-69` | GitRunner interface and default exec implementation for git operations |
| `internal/worktree/worktree.go:173-238` | CreateWithParent creating linked worktrees with recorded base SHA |
| `internal/worktree/worktree.go:247-269` | Cleanup and Remove deleting linked worktrees and branches idempotently |
| `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` | Precedent declining apply-dag sidecar for change fitting review budget |
| `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml:8-14` | Precedent recording Integrate gate failure on split TDD waves |
| `openspec/changes/native-stability-campaign/design.md:81-93` | Design file-changes table defining native-stability-campaign deliverables and terminal consumers |
