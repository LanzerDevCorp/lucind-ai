# Spec Lens A — Capabilities & Requirements: Control Room Telemetry

## Assumed requirements

This change specifies five capabilities comprising five total requirements: two new capabilities (`lane-telemetry-streaming` and `shell-free-telemetry-query`, each receiving 1 full-spec requirement) and three existing capabilities (`approvals-web-ui`, `lane-execution`, and `parent-feature-integration`, each receiving 1 delta requirement). All five requirements are ADDED, introducing worktree-local log teeing with grandchild wait-delay handling, loopback Server-Sent Events streaming, high-frequency SQLite ledger isolation, shell-free run lifecycle DTO queries, and feature attempt audit trail preservation. No existing requirements are modified, renamed, or removed.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `lane-telemetry-streaming` | New | `openspec/specs/lane-telemetry-streaming/spec.md` | - |
| `shell-free-telemetry-query` | New | `openspec/specs/shell-free-telemetry-query/spec.md` | - |
| `approvals-web-ui` | Existing | `openspec/changes/control-room-telemetry/specs/approvals-web-ui/spec.md` | `openspec/specs/approvals-web-ui/spec.md:1-83` |
| `lane-execution` | Existing | `openspec/changes/control-room-telemetry/specs/lane-execution/spec.md` | `openspec/specs/lane-execution/spec.md:1-62` |
| `parent-feature-integration` | Existing | `openspec/changes/control-room-telemetry/specs/parent-feature-integration/spec.md` | `openspec/specs/parent-feature-integration/spec.md:1-65` |

## ADDED Requirements

### Requirement: Worktree-Local Log Teeing and Process Invariants

The lane execution dispatch loop MUST stream child process stdout and stderr concurrently to a worktree-local log file under `.lucind/` and broadcast chunks to an in-memory telemetry hub via `io.MultiWriter`. The executor MUST honor `cmd.WaitDelay` when grandchild processes hold stdio descriptors open, returning `Outcome.OutputTruncated = true` while preserving the observed child exit code without process hangs.

**Terminal consumer**: `internal/executor/agy.go:160-197`, `internal/executor/cursor_agent.go:82-118`, `internal/executor/opencode.go:121-159`, and `internal/run/run.go:368-375`.

### Requirement: Loopback Server-Sent Events Telemetry Stream

`lucind-ai serve` MUST expose a loopback-only Server-Sent Events endpoint (`/api/telemetry/events`) using standard library HTTP and `http.Flusher` on the existing serve mux to stream live lane stdout/stderr chunks to subscribed clients. The endpoint MUST enforce loopback binding via `serve.IsLoopback`, MUST unregister subscribers upon client disconnection, and MUST NOT bypass or alter individual per-item approval decision controls.

**Terminal consumer**: `internal/serve/handlers.go:36-85`, `internal/serve/server.go:16-23,55-73`, and `cmd/lucind-ai/cli.go:674-725`.

### Requirement: High-Frequency SQLite Ledger Isolation

High-frequency stdout and stderr stream chunks MUST NOT be inserted into the SQLite `events` ledger table. The ledger MUST record only the six admitted lifecycle event types (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`) via `SetStatus` and `AppendEvent`, and failure diagnostics in `lane_note` rows MUST remain strictly bounded to at most 4096 bytes per stream with truncation markers.

**Terminal consumer**: `internal/ledger/schema.go:34-43`, `internal/ledger/ledger.go:360-381,448-485`, and `internal/run/run.go:71-89,422-435`.

### Requirement: Shell-Free Run Lifecycle Query

`serve.Model` MUST query run lifecycle events, lane execution outcomes, execution durations, and terminal statuses directly from the SQLite ledger without invoking `os/exec`, git subprocesses, or shell commands. Telemetry streaming MUST NOT add a seventh `lane.Status` value and MUST NOT delay batch barrier release beyond bounded in-memory stream flush.

**Terminal consumer**: `internal/serve/model.go:14-25`, `internal/serve/model_test.go:595-627`, and `internal/barrier/barrier.go:36-47`.

### Requirement: Feature Attempt Audit Preservation

Feature integration attempts and mechanical check validations SHALL record phase transitions and check outcomes exclusively through `WriteWithAudit` into `integration_events`, and SHALL NOT route raw execution stdout/stderr chunks into the SQLite ledger `events` table.

**Terminal consumer**: `internal/run/attempt.go:213-214,408-443`, `internal/integrate/integrate.go:90-109`, and `internal/ledger/ledger.go:832-873`.

## Open Questions

- [ ] Should worktree logs be archived automatically before worktree deletion (`RemoveLaneWorktree` / `worktree cleanup`) to `.lucind/results/<lane-id>.log` or `.lucind/logs/<run-id>/`?
- [ ] Should the SSE payload framing transmit raw byte stream chunks or structured JSON envelopes containing lane ID, stream name, timestamp, and chunk payload?
- [ ] Should coarse progress milestones (turn index, elapsed time) be routed strictly via the in-memory hub or persisted through an additive ledger schema v6 migration?
