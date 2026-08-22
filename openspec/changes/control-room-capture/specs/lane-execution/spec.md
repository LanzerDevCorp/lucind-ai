# Delta for lane-execution

## ADDED Requirements

### Requirement: Non-interfering WaitDelay drain

Stream spooling MUST NOT alter `exec.Cmd.WaitDelay` timeout or process exit handling. When grandchild processes hold output pipes past `WaitDelay`, the executor MUST set `Outcome.OutputTruncated` and `Report.OutputCaptureIncomplete`, preserve the child process exit code, and MUST NOT fail an otherwise valid `Done` lane.

#### Scenario: Clean exit with complete drain

- GIVEN a child process that exits 0 and closes pipes within WaitDelay
- WHEN the executor awaits process completion
- THEN OutputTruncated MUST be false and the lane status is Done

#### Scenario: Grandchild holds pipes past WaitDelay

- GIVEN a child process that exits 0 while a grandchild retains stdout or stderr pipes
- WHEN WaitDelay expires with incomplete pipe drain
- THEN OutputTruncated MUST be true, the real exit code 0 is preserved, and status remains Done

#### Scenario: Truncation diagnostic recorded without failing lane

- GIVEN a completed lane with OutputTruncated set to true
- WHEN terminal status is persisted
- THEN an EventLaneNote recording truncated capture MUST be appended to the ledger without altering lane status
