# Proposal Lens C — Risks, Rollback & Test Impact: Control Room Capture

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| Grandchild process pipe inheritance causing WaitDelay stall and capture truncation | Dispatched sub-agents (e.g. MCP servers) inherit stdout/stderr pipes; `cmd.Wait()` blocks until `WaitDelay` fires. Captured stdio in `Outcome` is marked truncated (`OutputTruncated = true`). | Maintain non-zero `cmd.WaitDelay` timeout (5s default); ensure file-backed stream tee closes child handles on exit and marks `OutputCaptureIncomplete` without failing the lane. | `internal/executor/agy.go:15-39`, `internal/executor/agy.go:165-197`, `internal/executor/cursor_agent.go:87-118`, `internal/executor/opencode.go:126-160`, `internal/run/run.go:62-70,488-499,506` |
| Unbounded in-memory stream buffering during concurrent lane execution | Accumulating raw stdout/stderr in memory (`bytes.Buffer`) across N concurrent lanes exhausts heap memory on multi-megabyte dispatches. | Stream child stdio directly to disk files via `io.MultiWriter` / file tees, bounding any circular replay buffer (~1MB/lane) for in-flight UI tailing. | `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, `internal/run/batch.go:80-89` |
| Ephemeral worktree destruction deleting execution logs upon integration | Logs written to a lane worktree directory are wiped when `completeIntegration` removes the worktree and deletes its branch upon batch merge. | Write execution logs strictly to primary repository root (`<primaryRoot>/.lucind/runs/<run_id>/lanes/<lane_id>.log`), validated via `ledgerpath.Validate`. | `internal/run/integrate.go:159-165`, `cmd/lucind-ai/cli.go:641-660`, `internal/ledgerpath/ledgerpath.go:23-58` |
| Slow or hung loopback HTTP/SSE consumer blocking child process execution | A slow or disconnected browser client reading SSE stream endpoints creates backpressure on stdio reader pipes, stalling child CLI execution. | Decouple child stdio collection pumps from HTTP handlers via file-backed disk reads or non-blocking buffered channels monitoring `r.Context().Done()`. | `internal/serve/server.go:19-53`, `internal/serve/handlers.go:36-118`, `cmd/lucind-ai/cli.go:674-725` |
| ANSI terminal escape sequences and spinner codes corrupting plaintext SQLite notes | Rich CLI outputs containing ANSI cursor controls or escape sequences pollute bounded ledger notes in `events.detail`. | Keep raw ANSI bytes on disk logs for terminal fidelity; sanitize/strip ANSI sequences before formatting the 4096-byte `formatStreamDetail` tail for ledger notes. | `internal/run/run.go:89,125-144,422-435`, `internal/ledger/schema.go:34-43` |
| Exit-0 successful lane execution discarding stream transcripts | Successful dispatches (`status == lane.Done`) have empty reason strings, causing `Execute` to skip writing stream details to `events` and drop in-memory buffers on exit. | Unconditionally spool stdout and stderr to disk files starting at process spawn, persisting files independently of terminal lane status. | `internal/result/result.go:117-135`, `internal/run/run.go:402,416-435,501-508` |

## Rollback & Additivity

**Rollback Plan**: Reverting requires a single git revert (`git revert <sha>`) restoring in-memory `bytes.Buffer` capture in `internal/executor/` and execution pipelines in `internal/run/run.go:169-175,422-435`. Log files created under primary root `.lucind/` (gitignored at `.gitignore:2`) do not alter git history or branch tracking and can be safely left or deleted without database rollback.

**Additivity**: All changes are strictly additive:
- File Storage: Run logs are written under `<primaryRoot>/.lucind/` (`.gitignore:2`), coexisting with `.lucind/results/` (`cmd/lucind-ai/cli.go:655-660`) and `.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:30-38`).
- SQLite Ledger: Database schema remains at `schemaVersion = 5` (`internal/ledger/schema.go:9-10`). No tables or columns are removed; any optional milestone event types added to `events.type` (`internal/ledger/schema.go:38-39`) follow additive table rebuild patterns (`internal/ledger/schema.go:59-78,190-219`).
- Wire & Envelope Formats: Embedded JSON schema `result.schema.json` (`internal/result/schema.go:10-28`) and result envelope parsing (`internal/result/result.go:43-135`) remain unchanged.
- CLI & HTTP APIs: Existing subcommands (`cmd/lucind-ai/cli.go:99-127`) and HTTP endpoints (`internal/serve/handlers.go:36-118`) remain backward compatible while adding stream routes.

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| Executor Subprocess & Spooling Unit Tests (`internal/executor`) | Verify executors (`Agy`, `CursorAgent`, `Opencode`) tee stdout/stderr to provided destinations; assert `Outcome.ExitCode`, `Outcome.TimedOut`, and `Outcome.OutputTruncated` reporting remain identical; assert opencode subagent fallback warning detection is preserved. | `internal/executor/agy_test.go:28-60,158-218`, `internal/executor/cursor_agent_test.go:28-60`, `internal/executor/opencode_test.go:28-80` |
| Primary Root Path Resolution & Boundary Validation (`internal/ledgerpath`) | Add tests verifying log directory path resolution (`<primaryRoot>/.lucind/runs/...`) and asserting `ledgerpath.Validate` rejects paths inside ephemeral worktrees (`<repo>-worktrees/...`). | `internal/ledgerpath/ledgerpath_test.go:9-35,37-60`, `internal/ledgerpath/ledgerpath.go:23-58` |
| Run Lifecycle & Stream Preservation (`internal/run`) | Verify `Execute` persists log files to primary root across all status outcomes (`Done`, `Blocked`, `Failed`, `Deviated`); verify 4096-byte `formatStreamDetail` tail clipping in `EventLaneNote` continues alongside disk logs; verify `TestExecuteTruncatedOutcomeStillYieldsDoneAndReportsCapture` semantics. | `internal/run/run_test.go:645-730,820-950`, `internal/run/run.go:89,125-144,488-499` |
| Batch Concurrency & Worktree Teardown Safety (`internal/run`) | Verify concurrent lanes in `ExecuteBatch` write to isolated stream files without race conditions; verify `completeIntegration` removes lane worktrees while preserving primary root log files. | `internal/run/batch_test.go:66-100`, `internal/run/integrate_test.go:20-80`, `internal/run/integrate.go:159-165` |
| Loopback HTTP Log Streaming & Disconnect Handling (`internal/serve`) | Add tests for log streaming / SSE handlers; verify loopback enforcement (`ErrNonLoopback`); assert client disconnect (`r.Context().Done()`) terminates stream goroutines cleanly; test 404 responses for missing logs. | `internal/serve/server_test.go:17-40`, `internal/serve/handlers.go:36-118`, `internal/serve/server.go:19-53` |
| CLI End-to-End Dispatch Integration (`cmd/lucind-ai`) | Test `lucind-ai run` with mock child script generating output; verify primary root `.lucind/` log file creation and status reporting without hangs or panics. | `cmd/lucind-ai/cli_test.go:37-80`, `cmd/lucind-ai/cli.go:99-173,640-662` |

## Out of Scope

- Frontend web UI components, terminal emulator DOM views, xterm.js styling, and layout controls (owned by `control-room-ui-shell` and `control-room-ui-views`).
- Loopback HTTP server multiplexer architecture, listener lifecycle, and global route registration ownership (owned by `control-room-serve`).
- SQLite schema migrations, transactional database tables, and ledger event indexing (owned by `control-room-ledger`).
- Real-time token analytics, metric collectors, and timeline aggregations (owned by `control-room-telemetry`).
- Candidate evaluation, technical approach selection, and conceptual architecture (owned by Lens A).
- Capability impact tables, delta specification requirements, and user scenarios (owned by Lens B).

## Open Questions

- [ ] Log path convention: Should log files standardize on `.lucind/runs/<run-id>/lanes/<lane-id>.log` or `.lucind/logs/<run-id>/<lane-id>.log`?
- [ ] Stream organization: Should stdout and stderr be recorded in a single interleaved stream or split into `.stdout.log` and `.stderr.log` per lane?
- [ ] Retention & cleanup: What retention policy or automated pruning mechanism should apply to `.lucind/` log files during `lucind-ai archive` or when old runs expire?
- [ ] Skill contract precedence: `~/.claude/skills/sdd-propose/SKILL.md` describes a monolithic `proposal.md` with PRD proposal question rounds and Engram persistence, which is intentionally superseded by this packet's parallel three-lens partitioned workflow writing `propose-lens-c.md`.
