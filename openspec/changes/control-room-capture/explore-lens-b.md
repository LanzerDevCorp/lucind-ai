# Explore Lens B — Capabilities & Scenarios: control-room-capture

## User & Capability Impact

Dispatched lane execution currently drops subprocess output streams upon successful completion (`internal/run/run.go:408-415`, `internal/executor/executor.go:42-63`). On failure or timeout, output is clipped to a 4096-byte tail (`internal/run/run.go:89,125-144`) stored as an event detail in the SQLite ledger (`internal/ledger/schema.go:34-43`, `internal/run/run.go:423-435`). Furthermore, `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:19-53`) only serves pending human approvals (`internal/serve/handlers.go:15-21`), leaving query models (`internal/serve/model.go:14-126`) unexposed and providing no live stream or historical log inspection.

This change introduces:
1. **File-Backed Subprocess Stream Spooling**: Directs child agent stdout and stderr during execution to persistent run logs under `.lucind/runs/<run-id>/lanes/<lane-id>.log` across all executors (`internal/executor/agy.go:135-175`, `internal/executor/cursor_agent.go:46-97`, `internal/executor/opencode.go:107-184`), preserving full transcripts for both successful and failed lanes without database bloat.
2. **Control Room Stream & Telemetry Endpoints**: Exposes HTTP endpoints in `internal/serve/handlers.go:36-118` for live log tailing (via Server-Sent Events or chunked stream) and static log download, backed by loopback-enforced HTTP serving (`internal/serve/server.go:19-53`).
3. **Decoupled Lifecycle Observability**: Operators running `lucind-ai run` (`cmd/lucind-ai/cli.go:129-180`) and `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) independently can observe active runs in real time or inspect historical run logs post-mortem without persistent daemon requirements.

## Scenarios & Use Cases

### Scenario 1 — Live Output Spooling During Batch Execution

- **Context**: An operator dispatches a concurrent batch via `lucind-ai run --packet ...` (`cmd/lucind-ai/cli.go:132-173`, `internal/run/batch.go:66-89`).
- **Action**: Child executors (`internal/executor/executor.go:66-67`) launch agent CLI subprocesses in isolated worktrees (`internal/worktree/worktree.go:168-238`, `internal/run/run.go:368-375`).
- **Outcome**: Subprocess stdio is piped to `.lucind/runs/<run-id>/lanes/<lane-id>.log` in real time using `io.MultiWriter` while populating in-memory `executor.Outcome` buffers (`internal/executor/executor.go:42-63`).

### Scenario 2 — Complete Transcript Preservation on Successful Lane Completion

- **Context**: A lane executes successfully, produces a valid result envelope, and evaluates to `status == lane.Done` (`internal/run/run.go:402-415`, `internal/result/result.go:43-125`).
- **Action**: `run.Execute` completes and persists terminal status to the SQLite ledger (`internal/run/run.go:470-485`, `internal/ledger/ledger.go:452-475`).
- **Outcome**: The complete execution log remains on disk at `.lucind/runs/<run-id>/lanes/<lane-id>.log` rather than being dropped from memory, enabling auditability without inflating SQLite database size (`internal/ledger/schema.go:34-43`).

### Scenario 3 — Unclipped Failure Diagnosis

- **Context**: A lane fails with a non-zero exit code or timeout (`internal/executor/executor.go:42-63`, `internal/run/run.go:390-403`).
- **Action**: `run.Execute` appends an `EventLaneNote` to SQLite with a bounded `diagnosisDetail` (`internal/run/run.go:89,125-144,423-435`) and marks the lane failed or blocked (`internal/run/run.go:470-485`).
- **Outcome**: The SQLite ledger maintains a compact summary for fast querying, while the full untruncated stream in `.lucind/runs/<run-id>/lanes/<lane-id>.log` allows developers to diagnose root-cause stack traces.

### Scenario 4 — Real-Time Log Streaming via Control Room UI

- **Context**: `lucind-ai serve` runs on `127.0.0.1:7433` (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:19-53`) while a batch executes concurrently.
- **Action**: A browser or HTTP client connects to `/api/runs/{runID}/lanes/{laneID}/stream` (`internal/serve/handlers.go:36-118`).
- **Outcome**: The server streams live log chunks using `http.Flusher` as they are written to disk, transitioning to a closed stream when the lane terminates.

### Scenario 5 — Uniform Redirection Across Heterogeneous Executors

- **Context**: A batch runs mixed executors (`agy` in `internal/executor/agy.go:135-175`, `cursor-agent` in `internal/executor/cursor_agent.go:46-97`, and `opencode` in `internal/executor/opencode.go:107-184`).
- **Action**: `executor.Request` accepts output destination parameters (`internal/executor/executor.go:15-37`).
- **Outcome**: All executors stream stdio identically to disk while preserving executor-specific flags, timeout tracking (`internal/executor/executor.go:1-9`), and grandchild pipe handling (`exec.ErrWaitDelay` in `internal/executor/agy.go:182-197`).

### Scenario 6 — Post-Mortem Log Retrieval After Run Termination

- **Context**: A batch run has finished and `lucind-ai run` has exited (`cmd/lucind-ai/cli.go:99-107`).
- **Action**: An operator launches `lucind-ai serve` and queries `/api/runs/{runID}/lanes/{laneID}/logs` (`internal/serve/handlers.go:36-118`).
- **Outcome**: The server reads `.lucind/runs/<run-id>/lanes/<lane-id>.log` and returns HTTP 200 with the full transcript, or HTTP 404 if the log is absent.

## Success Criteria

- [ ] Subprocess stdout and stderr are spooled continuously to file-backed logs under `.lucind/runs/<run-id>/lanes/<lane-id>.log` across `agy`, `cursor-agent`, and `opencode` executors (`internal/executor/agy.go:135-175`, `internal/executor/cursor_agent.go:46-97`, `internal/executor/opencode.go:107-184`).
- [ ] Complete execution logs are preserved on disk for all terminal statuses (`lane.Done`, `lane.Blocked`, `lane.Failed`, `lane.Deviated`) (`internal/run/run.go:408-435`).
- [ ] SQLite ledger `events` table stores bounded diagnosis summaries (`internal/ledger/schema.go:34-43`, `internal/run/run.go:89,125-144`) without inserting unbounded log blobs.
- [ ] `internal/serve/handlers.go:36-118` provides endpoints for downloading static transcripts and streaming active logs via HTTP/SSE.
- [ ] Control Room endpoints enforce loopback-only binding (`internal/serve/server.go:19-53`, `cmd/lucind-ai/cli.go:691-694`).
- [ ] Post-mortem log inspection works after `lucind-ai run` exits without requiring an active daemon (`cmd/lucind-ai/cli.go:99-113`).
- [ ] Subprocess pipe draining delays (`exec.ErrWaitDelay`) and timeout handling (`context.DeadlineExceeded`) function reliably during spooling (`internal/executor/agy.go:177-197`).

## Open Questions

- [ ] Should run logs under `.lucind/runs/` be pruned automatically by a retention policy or managed via an explicit `lucind-ai cleanup` command (`cmd/lucind-ai/cli.go:118-119`)?
- [ ] Should `internal/serve` provide separate stream endpoints for stdout and stderr or stream a single unified interleaved log file?
