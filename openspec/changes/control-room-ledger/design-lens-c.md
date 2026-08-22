# Design Lens C — Failure, Test & Rollback: Control Room Ledger

## Assumed architecture

We assume Candidate 1 (relational schema expansion with modular domain files). Schema version advances from 5 (`internal/ledger/schema.go:10`) to 6 via transactional `migrate` (`internal/ledger/schema.go:221-307`), adding `runs` and `lane_progress` tables, rebuilding `lanes` with nullable `model`, `agent`, `feature` columns, and rebuilding `events` to admit `run_status_changed`. `internal/ledger/ledger.go` methods split into domain files (`runs.go`, `lanes_meta.go`, `progress.go`, `events.go`) sharing `*Ledger` for disjoint apply paths. `serve.Model` (`internal/serve/model.go:14-25`) adds shell-free DTOs querying SQLite directly, while `cmd/lucind-ai/cli.go:282-290` registers runs at dispatch while preserving WAL mode, `busy_timeout=5000`, `MaxOpenConns=4`, and primary-root isolation.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit | Schema v6 migration and idempotency | Run `migrate` against clean and v1–v5 DB fixtures; assert rows preserved, new columns default null/empty, repeat `migrate` is no-op. | `internal/ledger/schema.go:221-307`, `internal/ledger/ledger_test.go:579-620,733-745,934-970` |
| Unit | Ordered progress cursor queries | Append chunks; verify `GetProgressAfter` returns strictly ascending `seq > afterSeq`, empty on current cursor, handles offsets. | New seam required: `AppendProgress` and `GetProgressAfter` on `*ledger.Ledger` |
| Unit | Progress retention pruning | Seed `lane_progress` with governance rows; run `PruneProgress(cutoff)` and assert only expired progress deleted. | New seam required: `PruneProgress` on `*ledger.Ledger` (analog: `internal/ledger/ledger.go:877-890`, `internal/ledger/ledger_test.go:1584`) |
| Integration | Concurrency under progress and lease renewals | Concurrent `AppendProgress`, `ValidateLease`, `SetStatus`, and approval polling under pool of 4; assert 0 `SQLITE_BUSY`. | `internal/ledger/ledger.go:162-185`, `internal/ledger/ledger_test.go:368`, `internal/run/attempt.go:434-441,482-488` |
| Unit | Shell-free read model DTO queries | Validate `serve.Model` methods (`GetRun`, `ListRuns`, `GetLaneProgress`) query SQLite; AST test enforces 0 `os/exec` / `git` imports. | `internal/serve/model.go:14-25`, `internal/serve/model_test.go:595` |
| Integration | Primary root resolution and worktree refusal | Verify `ledger.Open` uses `ledgerpath.Resolve` and CLI commands exit 1 inside linked worktrees. | `internal/ledgerpath/ledgerpath.go:36-38`, `internal/ledgerpath/ledgerpath_test.go:9,37`, `cmd/lucind-ai/cli.go:277-280,702-705` |
| Unit | Governance lifecycle invariance | Assert approval decisions (`Decide`), bulk rejections, defect marking, and `ApproverRate` work identically under v6. | `internal/ledger/ledger.go:614-640,643-661,797-814`, `internal/serve/handlers.go:161-177`, `internal/ledger/ledger_test.go:1047,1153` |

## Test Seams

Existing injectable and fakeable test seams:
- **Test DB Fixtures**: `openTestLedger(t)` (`internal/ledger/ledger_test.go:24`) and `openAtPath` (`internal/ledger/ledger.go:155`) spin up isolated temp SQLite instances with pragmas and pool.
- **Migration Runner**: `migrate(ctx, db)` (`internal/ledger/schema.go:221-307`) is callable against `*sql.DB` fixtures.
- **Path Resolution**: `ledgerpath.Resolve` and `ledgerpath.Validate` (`internal/ledgerpath/ledgerpath.go:36-44`) are pure functions (`internal/ledgerpath/ledgerpath_test.go:9,37`).
- **AST Conformance**: `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595`) parses `model.go` AST to enforce zero shell dependencies.
- **CLI Dependency Injection**: `depsFactory` in `cmd/lucind-ai/cli.go:292` (`cmd/lucind-ai/cli_test.go:1078`) injects mock execution with real ledger handles.

New seams required:
- **Run Persistence**: `RegisterRun` and `UpdateRunStatus` on `*ledger.Ledger` (`internal/ledger/runs.go`).
- **Progress Streaming**: `AppendProgress` and `GetProgressAfter` on `*ledger.Ledger` (`internal/ledger/progress.go`).
- **Progress Pruning**: `PruneProgress` on `*ledger.Ledger` (`internal/ledger/progress.go`).
- **Model DTO Accessors**: `GetRun`, `ListRuns`, `GetLaneProgress` on `serve.Model` (`internal/serve/model.go`).

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: change does not classify or execute documentation/script paths | N/A | N/A |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | DB path resolves to `<primaryRoot>/.lucind/lucind.db` via `ledgerpath.Resolve` (`internal/ledgerpath/ledgerpath.go:36-38`); CLI gates reject linked worktrees with exit code 1 before `Open` (`cmd/lucind-ai/cli.go:277-280,702-705`). Safe: primary DB used. Failure: worktree execution refused. | RED tests: `TestResolve` with relative/absolute paths (`internal/ledgerpath/ledgerpath_test.go:9`); CLI test asserting exit code 1 on linked worktree root. |
| Commit state | staged, `commit -a`, empty index | N/A: change does not manipulate git index or commit state | N/A | N/A |
| Push state | tracking branch, first push, explicit refspec | N/A: change does not manage git push or remote refs | N/A | N/A |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: change does not compose or run PR automation commands | N/A | N/A |

## Rollback and Additivity

**Choice**: `git revert` of the Go commits in `internal/ledger/`, `internal/serve/`, and `cmd/lucind-ai/`.
**Alternatives considered**: Destructive schema downgrade script (`DROP TABLE runs`, `DROP TABLE lane_progress`, rebuild `lanes`/`events` back to v5). Rejected because SQLite ignores unused tables and nullable columns; v5 binaries operate safely on a v6 database.
**Rationale**: Schema version advances from 5 (`internal/ledger/schema.go:10`) to 6. `migrate` only applies steps where `currentVersion < N` (`internal/ledger/schema.go:240-304`) and ignores higher versions in `schema_migrations`. v5 readers explicitly select named columns (`internal/ledger/ledger.go:287-289`), ignoring new nullable columns (`model`, `agent`, `feature`) and tables (`runs`, `lane_progress`). Reverting Go commits restores v5 behavior with zero data loss.

Format deltas are additive:
- Schema v6 adds `runs`, `lane_progress`, and nullable `lanes` metadata columns without altering existing columns or primary keys (`internal/ledger/schema.go:18-57`).
- `events.type` CHECK widening preserves existing enum literals (`internal/ledger/schema.go:38-39`).
- `serve.Model` adds DTO structs without modifying existing feature/lease structs (`internal/serve/model.go:14-25`).
- Packet parsing (`internal/packet/packet.go:33-75`) and result schema (`.lucind/result.schema.json:1-160`) are unchanged.

## Out of Scope

- Control Room frontend views and templates (`control-room-ui-shell`, `control-room-ui-views`).
- HTTP/SSE/WebSocket listener lifecycle (`control-room-serve`).
- Child process stdout/stderr stream piping (`control-room-capture`).
- External telemetry exporters (`control-room-telemetry`).
- High-level architecture decisions and data flow (owned by Lens A and Lens B).
- Modifying `gentle-ai` review policies, RDD delivery gates, or packet schemas.

## Open Questions

- [ ] Should `lane_progress` retention pruning be triggered periodically by `lucind-ai serve` or exposed strictly as an on-demand CLI maintenance command?
- [ ] Should `lane_progress.message` store raw text chunks or structured JSON distinguishing stdout, stderr, and system events?
- [ ] Execution-topology note: Proposal and design phases fan out across three parallel lenses (Lens A, Lens B, Lens C) feeding a synthesis lane, per packet authorization, superseding single-subagent monolithic `design.md` generation in `~/.claude/skills/sdd-design/SKILL.md`.
