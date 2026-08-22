# Design Lens C — Failure, Test & Rollback: Control Room Capture

## Assumed architecture

This design assumes Candidate 4 (Hybrid File-Backed Stream Spooling with Ledger Milestones and Model Query Routing). `executor.Request` (`internal/executor/executor.go:14-37`) adds destination writers; `Agy` (`internal/executor/agy.go:169-175`), `CursorAgent` (`internal/executor/cursor_agent.go:91-97`), and `Opencode` (`internal/executor/opencode.go:130-136`) tee process stdio to `<primaryRoot>/.lucind/runs/<run_id>/lanes/<lane_id>.log` via `io.MultiWriter`, leaving `Outcome` (`internal/executor/executor.go:42-63`) diagnosis-only. `Execute` (`internal/run/run.go:368-375,416-435`) manages primary-root log lifecycle across `completeIntegration` (`internal/run/integrate.go:159-165`) and keeps `events.detail` capped at 4096 bytes/stream (`internal/run/run.go:89,132-144`). `internal/serve` (`internal/serve/server.go:19-53`, `internal/serve/handlers.go:36-118`) adds loopback SSE tailing and transcript download via `serve.NewModel` (`internal/serve/model.go:21-24`).

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit | `Agy`, `CursorAgent`, `Opencode` tee stdio to dest writers | Subprocess stubs (`writeStub`) with `bytes.Buffer` dests | `internal/executor/agy_test.go:18-26,28-65`, `internal/executor/cursor_agent_test.go:28-60`, `internal/executor/opencode_test.go:28-80` |
| Unit | Grandchild pipe leak past `WaitDelay` yields `Outcome.OutputTruncated = true`, sets `Report.OutputCaptureIncomplete`, keeps exit code | Stub spawning grandchild (`sleep 10 &`) with small `WaitDelay` | `internal/executor/agy_test.go:158-218`, `internal/run/run_test.go:645-670` |
| Unit | `events.detail` capped at 4096 bytes/stream (`streamDetailCap`) with truncation marker on non-success, omitted on `Done` | Temp ledger (`ledger.Open`), fake executor returning >4096-byte streams | `internal/run/run.go:89,132-144,422-435`, `internal/run/run_test.go:25-48,675-710` |
| Unit | Log paths resolve under `<primaryRoot>/.lucind/` and `ledgerpath.Validate` rejects worktree paths | Table-driven path tests asserting valid primary subpaths and `ErrLedgerOutsidePrimaryRepo` | `internal/ledgerpath/ledgerpath.go:23-58`, `internal/ledgerpath/ledgerpath_test.go:9-35,37-60` |
| Unit / Integration | Loopback HTTP exposes SSE live tail (`text/event-stream`) and transcript download; disconnect ends tail | `httptest.NewServer`/`Recorder` with active log append and context cancellation | `internal/serve/server_test.go:17-40`, `internal/serve/handlers.go:36-118` (new seam required: log stream handler and file tail pump) |
| Integration | `Execute` writes primary log at spawn, preserving full streams across `Done`, `Blocked`, `Failed`, `Deviated` | Injected `Deps`, temp primary root, fake executor generating stream payloads | `internal/run/run.go:149-212,368-435`, `internal/run/run_test.go:25-56,645-730` |
| Integration | Concurrent lanes write isolated logs; `completeIntegration` deletes worktrees while retaining primary logs | Multi-lane `ExecuteBatch` with concurrent stubs; `completeIntegration` asserting `.lucind/runs/...` logs persist | `internal/run/batch_test.go:66-100,530-580`, `internal/run/integrate_test.go:20-80,392-440` |
| E2E | `lucind-ai run` dispatches stub child, writes durable primary root log, and surfaces terminal report | CLI execution in temp repo with stub script, verifying log creation under `<primaryRoot>/.lucind/runs/` | `cmd/lucind-ai/cli.go:99-173,640-662`, `cmd/lucind-ai/cli_test.go:37-80,1058-1150` |

## Test Seams

**Existing seams**:
- **Subprocess stubbing (`writeStub`)**: `internal/executor/agy_test.go:18-26` writes scripts to `t.TempDir()`, overriding `Binary` and `WaitDelay` to test process lifecycle and `ErrWaitDelay`.
- **Workflow injection (`run.Deps`)**: `internal/run/run.go:149-212` injects `PrimaryRoot`, `Ledger`, `LookupExecutor`, `CreateWorktree`, `WorktreeFS`, `Now`, `LaneTimeout`, `ApprovalTimeout`, `PersistEnvelope`, `RemoveLaneWorktree`.
- **Executor double (`fakeExecutor`)**: `internal/run/run_test.go:25-56` captures `executor.Request`, returning programmed `Outcome`, errors, and `beforeReturn` hooks.
- **Path validation (`ledgerpath.Validate`)**: `internal/ledgerpath/ledgerpath.go:40-58` validates candidate paths against `<primaryRoot>/.lucind/` in memory.
- **HTTP harness (`httptest`)**: `internal/serve/server_test.go:17-40` and `internal/serve/handlers.go:36-118` test endpoints, SSE streams, and disconnects.

**New seams required**:
- `executor.Request` stream destinations (`internal/executor/executor.go:14-37`): Add writer fields (`StdoutWriter`, `StderrWriter io.Writer`) to `Request`.
- `run.Deps` log configuration (`internal/run/run.go:149-212`): Add log path resolver or base directory to `Deps`.
- `serve.NewHandler` log filesystem provider (`internal/serve/handlers.go:36-118`): Add parameter accepting primary root or log reader interface.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: does not classify, evaluate, or execute documentation files | Pass-through non-boundary | None |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | Log paths resolve under `<primaryRoot>/.lucind/runs/` via `ledgerpath.Resolve` and `ledgerpath.Validate`. Candidate paths targeting worktrees (`../<repo>-worktrees/<lane>/.lucind/`) or traversals (`../`) are rejected with `ErrLedgerOutsidePrimaryRepo`. Safe behavior: logs write to primary `.lucind/`. Failure behavior: non-primary candidates return error and abort. | 1. Unit test in `internal/ledgerpath/ledgerpath_test.go` verifying `Validate` rejects worktree candidates and traversals.<br>2. Integration test in `internal/run/run_test.go` verifying `Execute` creates logs under `<primaryRoot>/.lucind/runs/` and never in worktrees. |
| Commit state | staged, `commit -a`, empty index | N/A: does not create git commits or modify git staging index | Pass-through non-boundary | None |
| Push state | tracking branch, first push, explicit refspec | N/A: does not execute git push or interact with remote repositories | Pass-through non-boundary | None |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: does not compose or execute PR commands or VCS automation | Pass-through non-boundary | None |

## Rollback and Additivity

**Choice**: Revert code changes via `git revert <sha>`, restoring in-memory `bytes.Buffer` capture in `internal/executor/` and diagnosis-only note recording in `internal/run/run.go:416-435`.
**Alternatives considered**: Database migration rollback or log cleanup utilities. Rejected because no SQLite migrations are applied (`schemaVersion = 5` at `internal/ledger/schema.go:9-10`), and disk logs in gitignored `<primaryRoot>/.lucind/` (`.gitignore:2`) are ephemeral working state.
**Rationale**: Purely additive:
- **Filesystem**: Logs live under `<primaryRoot>/.lucind/runs/`, coexisting beside `.lucind/results/` (`cmd/lucind-ai/cli.go:655-660`) and `.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:30-38`).
- **SQLite Ledger**: `schemaVersion = 5` (`internal/ledger/schema.go:9-10`) is untouched; `events.type` retains existing lifecycle values (`internal/ledger/schema.go:38-39`); notes in `events.detail` stay capped at 4096 bytes (`internal/run/run.go:89`).
- **Result Envelopes**: `result.schema.json` (`internal/result/schema.go:10-28`) and envelope parser (`internal/result/result.go:43-135`) are unchanged.
- **HTTP APIs**: Existing routes (`/`, `/api/state`, `/approvals/` at `internal/serve/handlers.go:36-118`) remain backward compatible; log streaming endpoints are additive.

## Out of Scope

- Web UI shell layout, terminal emulation components, and xterm rendering (owned by `control-room-ui-shell`, `control-room-ui-views`).
- Global server multiplexer architecture, listener lifecycle, and route ownership (owned by `control-room-serve`).
- SQLite schema migrations, new database tables, and ledger event indexing (owned by `control-room-ledger`).
- Real-time token analytics, metric collectors, and timeline aggregations (owned by `control-room-telemetry`).
- Candidate evaluation and technical approach selection (owned by Lens A).
- File changes table, data-flow diagrams, and exact type/schema deltas (owned by Lens B).

## Open Questions

- [ ] Directory layout: Standardize on `.lucind/runs/<run_id>/lanes/<lane_id>.log` or `.lucind/logs/<run_id>/<lane_id>.log`? Recommendation: `.lucind/runs/<run_id>/lanes/<lane_id>.log`.
- [ ] Stream multiplexing: Interleaved `.log` file or separate `.stdout.log`/`.stderr.log` files? Recommendation: Single interleaved file with stream markers.
- [ ] Log retention: Copy run logs in `lucind-ai archive` or prune via `lucind-ai worktree cleanup`? Recommendation: Keep gitignored by default and add retention cleanup flags.
- [ ] Skill contract precedence: `~/.claude/skills/sdd-design/SKILL.md` describes a monolithic `design.md` with Engram persistence and an 800-word budget, superseded here by the parallel three-lens workflow writing `design-lens-c.md` under a 1000-word budget.
