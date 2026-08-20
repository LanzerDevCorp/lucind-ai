# Wave Dispatch Specification

## Purpose

Execute dependency waves sequentially through `lucind-ai run`, ensuring each wave branches from the previous wave's promoted primary HEAD, and surfacing per-lane integration and revert outcomes on stdout.

## Requirements

### Requirement: Sequential Per-Wave Execution Contract

Each wave of independent packets MUST be executed as a separate `lucind-ai run` invocation. The orchestrator MUST dispatch wave $N+1$ only after wave $N$ completes with exit code 0 and promotes its changes to the primary repository. (Trace: Decision 3 — sequential lucind-ai run per wave)

#### Scenario: Next wave dispatches after prior wave promotes
- GIVEN wave 1 successfully completes all lanes and promotes to primary repository HEAD
- WHEN the orchestrator advances to wave 2
- THEN wave 2 MUST execute against the newly promoted primary HEAD

#### Scenario: Worktree branches from updated primary HEAD
- GIVEN wave 1 has integrated and fast-forwarded the primary branch
- WHEN wave 2 creates worktrees via `worktree.Create`
- THEN the new worktrees MUST branch from primary's updated HEAD

### Requirement: Execution Engine Invariance

`ExecuteBatch` MUST remain an isolated concurrent execution engine for a flat batch of packets without internal wave or DAG scheduling logic. `Integrate`, conflict resolution, and bisection MUST execute per wave without alteration. (Trace: Decision 3 — sequential lucind-ai run per wave)

#### Scenario: Concurrent lane execution within a wave
- GIVEN a wave containing multiple independent packets
- WHEN `lucind-ai run` executes the wave
- THEN `ExecuteBatch` MUST run the lanes concurrently in isolated worktrees using a single barrier

#### Scenario: Integration and bisection executed per wave
- GIVEN lanes completing at the batch barrier
- WHEN `Integrate` runs
- THEN it MUST combine `done` lanes, resolve conflicts up to 400 lines, bisect if regressions occur, and promote passing lanes

### Requirement: Orchestrator Wave Halt on Failure

If any packet in a wave fails, is blocked, is demoted to deviated, or is reverted during bisection, `lucind-ai run` MUST exit with a non-zero exit code, and the orchestrator MUST halt execution before dispatching subsequent waves. (Trace: Decision 3 — sequential lucind-ai run per wave, Decision 4 — partial-failure surfacing)

#### Scenario: Wave failure halts subsequent wave dispatch
- GIVEN wave 1 produces a `blocked` or `failed` lane causing `lucind-ai run` to exit with code 1
- WHEN the orchestrator receives the non-zero exit code
- THEN it MUST halt and NOT execute wave 2

#### Scenario: Reverted lane triggers non-zero exit and halts sequence
- GIVEN all lanes in wave 1 report `done` but one lane is reverted during bisection integration
- WHEN `lucind-ai run` completes
- THEN it MUST exit with a non-zero exit code and prevent wave 2 dispatch

### Requirement: Surfacing Integrated and Reverted Lane IDs

The CLI `printReport` function MUST print the list of integrated lane IDs (`integrated_ids`) and reverted lane IDs (`reverted_ids`) to stdout alongside the integration summary counts. (Trace: Decision 4 — partial-failure surfacing)

#### Scenario: Print integrated and reverted lane IDs
- GIVEN a wave execution where packet `apply-ledger` is integrated and packet `apply-serve` is reverted
- WHEN `printReport` prints to stdout
- THEN stdout MUST contain `integrated_ids: apply-ledger` and `reverted_ids: apply-serve`

#### Scenario: Print empty list when no lanes reverted
- GIVEN a wave execution where all lanes are successfully integrated
- WHEN `printReport` prints to stdout
- THEN stdout MUST contain the integrated lane IDs and an empty `reverted_ids:` list
