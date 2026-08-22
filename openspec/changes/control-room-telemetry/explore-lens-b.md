# Explore Lens B — Capabilities & Scenarios: Control Room Telemetry

## User & Capability Impact

### Impacted Roles

- **Operator / Dispatcher**: Running parallel multi-lane execution batches (`cmd/lucind-ai/cli.go:95-127,261-311`) or managing feature integration attempts (`cmd/lucind-ai/cli.go:739-744`, `internal/serve/model.go:167-203`). Currently, during headless execution with up to 20-minute timeouts (`cmd/lucind-ai/cli.go:42`), the operator has no live visibility into process liveness, token consumption, or progress milestones until lane completion and barrier release (`cmd/lucind-ai/cli.go:512-540`, `internal/barrier/barrier.go:34-52`).
- **Control Room UI & API Layer**: The localhost HTTP server (`cmd/lucind-ai/cli.go:676-725`, `internal/serve/server.go:16-53`) and query model (`internal/serve/model.go:14-25`). Extends the `/api/state` endpoint (`internal/serve/handlers.go:15-22,79-86`) from static approval listings to live run timelines, metrics, and progress streams.
- **Executors & Run Engine**: The composition root (`internal/run/run.go:30-87`, `internal/run/batch.go:19-68`) and CLI executor adapters (`internal/executor/agy.go:37-67`, `internal/executor/cursor_agent.go:37-65`, `internal/executor/opencode.go:38-66`). Emits fine-grained telemetry to durable SQLite storage (`internal/ledger/schema.go:34-43,171-180`, `internal/ledger/ledger.go:360-381,832-873`).

### New & Modified Capabilities

1. **Structured Telemetry Events**: Extends the ledger events table (`internal/ledger/schema.go:38-42`) and audit logging (`internal/ledger/schema.go:171-180`) with fine-grained telemetry types (e.g., `lane_heartbeat`, `lane_progress`, `tool_call`, `token_usage`, `check_output`) without breaking STRICT schema guarantees (`internal/ledger/schema.go:18-57`).
2. **Executor Metric & Resource Tracking**: Ingests execution metrics (duration, turn counts, and token usage from formats like `cursor-agent`'s `--output-format json` payload, `internal/executor/cursor_agent.go:37-65`) and correlates them with lane execution records (`internal/ledger/ledger.go:232-282`).
3. **Shell-Free Telemetry Query Surface**: Extends `internal/serve/model.go` with read-only DTOs and queries (`ListRunEvents`, `GetLaneTelemetry`, `GetAttemptMetrics`) reading directly from SQLite without filesystem or shell execution (`internal/serve/model_test.go:595-627`).
4. **Live Execution Timeline & Health Monitoring**: Provides incremental observation of long-running lanes, enabling early detection of deadlocks, quota exhaustion, or stalled turns before wall-clock timeouts expire (`cmd/lucind-ai/cli.go:42,523-539`, `internal/executor/agy.go:48-66`).

## Scenarios & Use Cases

### Scenario 1 — Live Progress and Heartbeat Tracking During Long-Running Batch

- **Context**: `lucindrun.ExecuteBatch` is executing two parallel lanes (`agy` and `cursor-agent`) in separate worktrees (`internal/run/batch.go:19-68`, `internal/worktree/worktree.go:32-68`) with default timeout (`cmd/lucind-ai/cli.go:42`).
- **Action**: Running executors emit progress milestones and heartbeat events via `ledger.AppendEvent` (`internal/ledger/ledger.go:360-381`) with event type `lane_progress` and step details.
- **Outcome**: The Control Room model (`internal/serve/model.go:14-25`) returns live status, current turn index, and elapsed time; the UI displays active progress instead of an uninformative pending state.

### Scenario 2 — Token Usage and Duration Capture from Executor Output

- **Context**: `executor.CursorAgent.Run` completes a headless invocation (`internal/executor/cursor_agent.go:37-65`), returning a structured result containing execution duration and token usage (`inputTokens`, `outputTokens`).
- **Action**: `internal/run/run.go:30-87` parses execution metadata from `executor.Result` (`internal/executor/executor.go:24-30`) and records a `token_usage` telemetry event via `ledger.AppendEvent` (`internal/ledger/ledger.go:360-381`).
- **Outcome**: Token consumption and exact execution duration are persisted in the ledger; `serve.Model` surfaces cumulative run cost and token metrics at `/api/state` (`internal/serve/handlers.go:15-22,79-86`).

### Scenario 3 — Integration State Machine Transitions and Check Output Streaming

- **Context**: `lucindrun.IntegrateFeature` advances an attempt through `leased`, `combining`, `checking`, and `cas_pending` phases (`internal/run/integrate_feature.go:34-118`).
- **Action**: During the `checking` phase, `integrate.Check` runs `lucind-checks.sh` (`internal/integrate/integrate.go:14-36`, `cmd/lucind-ai/cli.go:411-479`) and appends incremental test progress using `ledger.WriteWithAudit` (`internal/ledger/ledger.go:832-873`).
- **Outcome**: `integration_events` rows (`internal/ledger/schema.go:171-180`) capture phase durations and check transcripts; the Control Room UI renders live check execution status before CAS promotion (`internal/serve/model.go:87-125,370-384`).

### Scenario 4 — Early Stalled-Lane and Quota Exhaustion Alerting

- **Context**: An executor encounters subscription quota refusal or stalls in an iterative loop without progressing toward envelope creation (`docs/prd.md:143-166`, `cmd/lucind-ai/cli.go:523-539`).
- **Action**: Telemetry monitoring observes that no heartbeat event has been received within the configured heartbeat threshold, or receives a non-zero exit code with diagnosis output (`internal/run/run.go:73-86`).
- **Outcome**: A `lane_note` diagnostic event is recorded in the ledger (`internal/ledger/ledger.go:360-381`); the Control Room surfaces an immediate warning to the operator before the full 20-minute timeout is reached (`cmd/lucind-ai/cli.go:42`).

### Scenario 5 — Chronological Post-Mortem Timeline Replay

- **Context**: A batch completes with mixed terminal statuses (`done`, `blocked`, `deviated`, `failed`) (`internal/lane/status.go:11-30`, `cmd/lucind-ai/cli.go:361-369`).
- **Action**: The operator requests the full execution timeline for `run_id` via `serve.Model.ListRunEvents` (`internal/serve/model.go:14-25`, `internal/ledger/ledger.go:488-526`).
- **Outcome**: The system returns a chronological event sequence (`run_started` -> `lane_registered` -> `lane_progress` -> `lane_status_changed` -> `barrier_released` -> `run_ended`), providing full auditability of lane synchronization and decision lineage (`internal/barrier/barrier.go:34-52`).

## Success Criteria

- [ ] Every lane dispatched via `internal/run/run.go` emits structured lifecycle and progress telemetry to the primary ledger (`internal/ledger/ledger.go:360-381`).
- [ ] Executor duration, exit codes, and token usage metrics (from `cursor-agent` in `internal/executor/cursor_agent.go:37-65`) are captured and queryable via `internal/serve/model.go`.
- [ ] `internal/serve/model.go` provides read-only, shell-free telemetry query methods without importing `os/exec` (`internal/serve/model_test.go:595-627`).
- [ ] Telemetry ingestion and query endpoints do not degrade or alter the loopback security boundary (`internal/serve/server.go:16-53`) or approval invariants (`internal/serve/handlers.go:148-211`).
- [ ] Feature integration attempts in `internal/run/integrate_feature.go` record phase durations and check outputs into `integration_events` (`internal/ledger/schema.go:171-180`, `internal/ledger/ledger.go:832-873`).

## Open Questions

- [ ] Should process-level system metrics (CPU/RSS memory) of child CLIs be sampled into SQLite, or should telemetry remain strictly focused on agent events, tokens, and phase durations?
- [ ] Should live telemetry event streaming to the Control Room web UI use HTTP polling (matching `internal/serve/static/app.js:97`) or Server-Sent Events (SSE) over loopback?
