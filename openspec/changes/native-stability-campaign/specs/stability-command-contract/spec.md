# Delta for Stability Command Contract

## ADDED Requirements

### Requirement: CLI preflight admission and safety gates

The `stability run` command MUST execute a non-mutating preflight validation checking Linux OS, clean checkout, candidate build matching `HEAD`, passing baseline check, and zero active campaigns before creating state or worktrees.

#### Scenario: Preflight success

- GIVEN a clean Linux checkout matching candidate HEAD build with no active campaign
- WHEN `lucind-ai stability run` runs with `yes` confirmation
- THEN preflight MUST succeed and create an active campaign record

#### Scenario: Dirty checkout rejected

- GIVEN uncommitted changes in primary worktree
- WHEN `lucind-ai stability run` executes
- THEN preflight MUST exit non-zero without creating state or worktrees

### Requirement: Interactive confirmation without non-interactive bypass

The `stability run` command MUST display the plan forecasting 15 model dispatches and require interactive confirmation defaulting to `no` before initializing state, and SHALL NOT support non-interactive bypass flags.

#### Scenario: Confirmation rejected by default

- GIVEN preflight displaying the 15-dispatch plan
- WHEN an operator rejects confirmation or accepts default `no`
- THEN the command MUST exit non-zero without initializing state

#### Scenario: Non-interactive bypass rejected

- GIVEN `stability run` invoked with bypass flags like `--yes` or in non-interactive shell
- WHEN arguments are parsed
- THEN the command MUST halt non-zero
