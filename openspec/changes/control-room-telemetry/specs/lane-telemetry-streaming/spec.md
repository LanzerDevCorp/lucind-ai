# Delta for Lane Telemetry Streaming

## ADDED Requirements

### Requirement: Worktree-Local Log Teeing and Process Invariants

The lane dispatch loop MUST stream child stdout and stderr concurrently to a worktree-local log under `.lucind/` and to an in-memory telemetry hub. The executor MUST honor `cmd.WaitDelay` when a grandchild holds stdio open, returning `Outcome.OutputTruncated = true` while preserving the observed child exit code, and MUST NOT hang.

#### Scenario: Live stdout and stderr streamed concurrently

- GIVEN a lane running in an isolated worktree with output capture configured
- WHEN the child process writes to stdout and stderr
- THEN output bytes MUST be written to the worktree log and broadcast to subscribers without blocking the child
- AND the captured outcome MUST include the final exit code and stream content upon child termination

#### Scenario: Grandchild holds stdio open past process exit

- GIVEN a child process that has exited with code 0 while a grandchild keeps stdio descriptors open
- WHEN `cmd.WaitDelay` elapses
- THEN the executor MUST return `Outcome.OutputTruncated = true` with exit code 0 and MUST NOT hang

#### Scenario: Non-zero exit code with partial capture

- GIVEN a lane execution where the child process exits with code 1 after partial execution
- WHEN the child run completes with an exit error
- THEN the outcome exit code MUST be 1 and the worktree log MUST retain all output produced prior to exit
