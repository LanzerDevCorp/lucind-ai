# Delta for Lane Execution

## ADDED Requirements

### Requirement: Process group isolation and termination

Lane execution on Linux MUST configure child processes with a dedicated process group (`Setpgid: true`), and termination on timeout or cancellation MUST signal the entire process group (`-pgid`) so no surviving child or grandchild processes remain.

#### Scenario: Linux process group creation

- GIVEN a lane dispatched on Linux
- WHEN the child process is configured
- THEN it MUST set `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` to isolate descendant processes

#### Scenario: Process group termination on timeout

- GIVEN a dispatched lane with active child and grandchild processes that times out
- WHEN timeout or cancellation fires
- THEN the executor MUST signal `-pgid` so no descendant processes survive
