# Verify Mechanical Check Specification

## Purpose

Execute deterministic mechanical checks (`lucind-checks.sh`) exactly once via a new CLI subcommand `lucind-ai check`, capture execution logs and metadata to a durable log artifact on the candidate branch before judgment dispatch, and short-circuit verification immediately upon failure.

## Requirements

### Requirement: Mechanical Check CLI Subcommand

`cmd/lucind-ai/cli.go` MUST provide a `check` subcommand (`lucind-ai check [--out <path>]`) that wraps `internal/integrate.Check(ctx, targetPath)` to execute `lucind-checks.sh` (build, unit tests, race detector, linter) deterministically against the candidate repository. The command MUST exit 0 when all checks pass and MUST exit non-zero when any check fails or when `lucind-checks.sh` is missing. (Design Decision 1.)

#### Scenario: Passing mechanical check exits 0
- GIVEN a candidate repository state where `lucind-checks.sh` succeeds
- WHEN `lucind-ai check` runs
- THEN it MUST exit 0 and print execution status, duration, and git commit SHA to stdout

#### Scenario: Failing mechanical check exits non-zero
- GIVEN a candidate repository state where `lucind-checks.sh` fails with exit code 1
- WHEN `lucind-ai check` runs
- THEN it MUST exit with non-zero status and output the failure transcript to stdout and stderr

#### Scenario: Missing check script exits non-zero
- GIVEN a candidate target directory that lacks `lucind-checks.sh`
- WHEN `lucind-ai check` runs
- THEN it MUST exit non-zero and report an error stating that `lucind-checks.sh` was not found

### Requirement: Output Capture to Durable Log Artifact

When the `--out <path>` flag is supplied, `lucind-ai check` MUST write the complete execution record to the specified file path. The captured log MUST include a structured metadata header (command line, exit code, execution duration, git commit SHA) followed by the combined stdout/stderr transcript of `lucind-checks.sh`. (Design Decision 1.)

#### Scenario: Log file written with structured header
- GIVEN `lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log`
- WHEN the command executes against candidate HEAD
- THEN `openspec/changes/<change-id>/verify-mechanical.log` MUST be created containing candidate git SHA, execution duration, exit code, and full check output

#### Scenario: Existing log file overwritten
- GIVEN an existing `openspec/changes/<change-id>/verify-mechanical.log` from a prior check run
- WHEN `lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log` runs again
- THEN the existing log file MUST be overwritten with the new run's output and updated metadata

### Requirement: Candidate Branch Pre-Commit of Mechanical Log

The orchestrator MUST commit `openspec/changes/<change-id>/verify-mechanical.log` to the candidate branch prior to worktree creation and packet dispatch. The terminal consumer `worktree.Create` MUST inherit the committed log into every linked judgment worktree at `openspec/changes/<change-id>/verify-mechanical.log` through normal git branch inheritance, requiring zero custom file-injection machinery in `ExecuteBatch`. (Design Decision 1.)

#### Scenario: Log committed to candidate HEAD before dispatch
- GIVEN a passed `lucind-ai check` that created `openspec/changes/<change-id>/verify-mechanical.log`
- WHEN the orchestrator prepares candidate state for dual dispatch
- THEN the log file MUST be committed to candidate `HEAD` before `lucind-ai run` creates lane worktrees

#### Scenario: Linked worktrees inherit log file automatically
- GIVEN `openspec/changes/<change-id>/verify-mechanical.log` committed to candidate `HEAD`
- WHEN `lucind-ai run` creates linked worktrees for judgment lanes
- THEN each judgment worktree MUST have `openspec/changes/<change-id>/verify-mechanical.log` present locally without cross-worktree file copying

### Requirement: Mechanical Failure Short-Circuit

If `lucind-ai check` exits non-zero, the orchestrator MUST halt the verification workflow immediately and MUST NOT dispatch qualitative judgment packets to `agy` or `cursor-agent`. (Design Decision 1.)

#### Scenario: Failing mechanical check halts verification
- GIVEN `lucind-ai check` exits non-zero due to a build or test failure
- WHEN the orchestrator evaluates the mechanical check outcome
- THEN it MUST halt the verify phase, record the failure, and MUST NOT dispatch judgment packets

#### Scenario: Passing mechanical check proceeds to dual dispatch
- GIVEN `lucind-ai check` exits 0 with all mechanical checks passing
- WHEN the orchestrator evaluates the mechanical check outcome
- THEN it MUST proceed to commit the log and construct judgment packets for dual dispatch

### Requirement: Single Deterministic Execution

Mechanical checks MUST execute exactly once per verification cycle. Tooling and orchestrator MUST NOT invoke `lucind-checks.sh` separately inside each qualitative judgment lane. (Design Decision 1.)

#### Scenario: Exactly one mechanical execution per verify cycle
- GIVEN an SDD change entering the `verify` phase
- WHEN mechanical verification is performed
- THEN `lucind-ai check` MUST execute once, and both judgment lanes MUST evaluate the identical frozen check artifact
