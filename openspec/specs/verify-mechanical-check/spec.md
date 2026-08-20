# Verify Mechanical Check Specification

## Purpose

Execute deterministic mechanical checks (`lucind-checks.sh`) exactly once via a new CLI
subcommand `lucind-ai check`, capture execution logs and metadata to a durable log artifact
committed to the candidate branch before judgment dispatch, and short-circuit verification
immediately on failure.

## Requirements

### Requirement: Mechanical Check CLI Subcommand

`cmd/lucind-ai/cli.go` MUST provide a `check` subcommand (`lucind-ai check [--out <path>]`) that
wraps `internal/integrate.Check(ctx, targetPath)` to execute `lucind-checks.sh` (build, unit
tests, race detector, linter) deterministically against the candidate repository. The command
MUST exit 0 when all checks pass and MUST exit non-zero when any check fails or when
`lucind-checks.sh` is missing.

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

When `--out <path>` is supplied, `lucind-ai check` MUST write the complete execution record to
that path: a structured metadata header (command line, exit code, execution duration, git commit
SHA) followed by the combined stdout/stderr transcript of `lucind-checks.sh`. When `--out` is
omitted, output MUST be written to stdout and stderr only. For verify workflows this path MUST be
`openspec/changes/<change-id>/verify-mechanical.log`.

#### Scenario: Log file written with structured header
- GIVEN `lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log`
- WHEN the command executes against candidate HEAD
- THEN the file MUST be created containing candidate git SHA, execution duration, exit code, and full check output

#### Scenario: Existing log file overwritten
- GIVEN an existing `verify-mechanical.log` from a prior check run
- WHEN `lucind-ai check --out` runs again against the same path
- THEN the existing file MUST be overwritten with the new run's output and updated metadata

#### Scenario: Default stdout/stderr emission
- GIVEN `lucind-ai check` invoked without `--out`
- WHEN the check completes
- THEN transcript and summary MUST be emitted directly to stdout/stderr, and no file MUST be written

### Requirement: Candidate Branch Pre-Commit and Worktree Inheritance

The orchestrator MUST commit `openspec/changes/<change-id>/verify-mechanical.log` to the
candidate branch HEAD prior to worktree creation and packet dispatch. `worktree.Create` MUST then
inherit the committed log into every linked judgment worktree through normal git branch
inheritance, requiring zero custom file-injection machinery in `ExecuteBatch`.

#### Scenario: Log committed to candidate HEAD before dispatch
- GIVEN a passed `lucind-ai check` run that created `verify-mechanical.log`
- WHEN the orchestrator prepares candidate state for dual dispatch
- THEN the log file MUST be committed to candidate HEAD before `lucind-ai run` creates lane worktrees

#### Scenario: Linked worktrees inherit the log automatically
- GIVEN `verify-mechanical.log` committed to candidate HEAD
- WHEN `lucind-ai run` creates linked worktrees for judgment lanes
- THEN each judgment worktree MUST contain `openspec/changes/<change-id>/verify-mechanical.log` at its expected path, with no cross-worktree file copying

### Requirement: Mechanical Failure Short-Circuit

If `lucind-ai check` exits non-zero, the orchestrator MUST halt the verify workflow immediately
and MUST NOT dispatch qualitative judgment packets to `agy` or `cursor-agent`.

#### Scenario: Failing mechanical check halts verification
- GIVEN `lucind-ai check` exits non-zero due to a build or test failure
- WHEN the orchestrator evaluates the mechanical check outcome
- THEN it MUST halt the verify phase, record the failure, and MUST NOT dispatch judgment packets

#### Scenario: Passing mechanical check proceeds to dual dispatch
- GIVEN `lucind-ai check` exits 0 with all mechanical checks passing
- WHEN the orchestrator evaluates the mechanical check outcome
- THEN it MUST proceed to commit the log and construct judgment packets for dual dispatch

### Requirement: Single Deterministic Execution

Mechanical checks MUST execute exactly once per verification cycle. Neither the orchestrator nor
any judgment lane MUST invoke `lucind-checks.sh` a second time for the same verify cycle.

#### Scenario: Exactly one mechanical execution per verify cycle
- GIVEN an SDD change entering the `verify` phase
- WHEN mechanical verification is performed
- THEN `lucind-ai check` MUST execute exactly once, and both judgment lanes MUST evaluate the identical frozen check artifact

### Requirement: Terminal Consumers of the Mechanical Log Artifact

`verify-mechanical.log` MUST be consumed by exactly three terminal consumers: (1) the
orchestrator, which embeds its summary and transcript into the `## Context` section of each
judgment packet; (2) judgment executors, who read it as an immutable baseline inside their
worktree; (3) the orchestrator and human auditor, who reference it during `verify.md` synthesis
and `openspec archive`.

#### Scenario: Context embedding for judgment executors
- GIVEN a generated `verify-mechanical.log` artifact
- WHEN the orchestrator constructs `packets/verify-<change-id>-agy.md` and `packets/verify-<change-id>-cursor-agent.md`
- THEN each packet's `## Context` section MUST contain the embedded summary and transcript from `verify-mechanical.log`

#### Scenario: Archival record cited in the verification report
- GIVEN a completed `openspec/changes/<change-id>/verify.md` report
- WHEN audited by a human reviewer or `openspec archive`
- THEN `verify.md` MUST reference `openspec/changes/<change-id>/verify-mechanical.log` as the mechanical verification record
