# Mechanical Check CLI Specification

## Purpose

Execute repository mechanical checks deterministically exactly once via a new CLI subcommand `lucind-ai check` wrapping `internal/integrate.Check`, capture execution logs to `openspec/changes/<change-id>/verify-mechanical.log`, and commit the log artifact to the candidate branch prior to judgment packet dispatch.

## Requirements

### Requirement: Deterministic Check Execution via CLI Subcommand

`lucind-ai check` MUST wrap `internal/integrate.Check(ctx, targetPath)` to execute the repository check script (`lucind-checks.sh`) in the target worktree. It MUST execute the mechanical check suite deterministically exactly once per verify phase before qualitative judgment dispatch. (Design Decision 1.)

#### Scenario: Passing check execution exits zero
- GIVEN a candidate worktree with a passing `lucind-checks.sh` script
- WHEN `lucind-ai check` executes
- THEN it MUST exit with returncode 0 and print execution duration and summary

#### Scenario: Failing check execution exits non-zero
- GIVEN a candidate worktree with failing tests or lint errors in `lucind-checks.sh`
- WHEN `lucind-ai check` executes
- THEN it MUST exit with a non-zero returncode and print stdout/stderr diagnostics

#### Scenario: Missing check script returns explanatory error
- GIVEN a candidate directory lacking `lucind-checks.sh`
- WHEN `lucind-ai check` executes
- THEN it MUST exit with a non-zero returncode and report that `lucind-checks.sh` is missing

### Requirement: Mechanical Output Artifact and File Output Flag

`lucind-ai check` MUST accept an optional `--out <path>` flag. When `--out` is specified, `lucind-ai check` MUST write the combined stdout/stderr transcript, execution duration, exit status, and candidate git commit SHA to the designated file path. For verify workflows, this path MUST be `openspec/changes/<change-id>/verify-mechanical.log`. When `--out` is omitted, output MUST be written to stdout and stderr. (Design Decision 1.)

#### Scenario: Writing mechanical log to specified path
- GIVEN `lucind-ai check --out openspec/changes/add-auth/verify-mechanical.log`
- WHEN `lucind-ai check` completes execution
- THEN `openspec/changes/add-auth/verify-mechanical.log` MUST contain the captured transcript, exit code, duration, and git SHA

#### Scenario: Default stdout and stderr emission
- GIVEN `lucind-ai check` invoked without `--out`
- WHEN the check completes
- THEN transcript and summary MUST be emitted directly to stdout/stderr

### Requirement: Pre-Dispatch Commit to Candidate Branch

The orchestrator MUST commit `openspec/changes/<change-id>/verify-mechanical.log` to the candidate branch HEAD prior to creating linked worktrees and dispatching judgment packets via `lucind-ai run`. This commit MUST make the log artifact available inside all linked worktrees via standard git checkout inheritance without out-of-band file copying or cross-worktree filesystem coupling. (Design Decision 1.)

#### Scenario: Log inherited by linked judgment worktrees
- GIVEN `openspec/changes/<change-id>/verify-mechanical.log` committed to the candidate branch HEAD
- WHEN `lucind-ai run` creates linked worktrees for judgment packets
- THEN each judgment worktree MUST contain `openspec/changes/<change-id>/verify-mechanical.log` at its root

#### Scenario: Mechanical failure halts verification before dispatch
- GIVEN `lucind-ai check` exits with a non-zero exit code
- WHEN the orchestrator evaluates step 1 of `sdd-verify`
- THEN the orchestrator MUST NOT dispatch judgment packets and MUST halt the verify phase for mechanical remediation

### Requirement: Terminal Consumers of Mechanical Log Artifact

The `verify-mechanical.log` artifact MUST be consumed directly by three terminal consumers: (1) the orchestrator embedding its summary and transcript into the `## Context` section of `packets/verify-<change-id>-<executor>.md`, (2) judgment executors reading it as an immutable baseline in their worktrees, and (3) the orchestrator and human auditor referencing it during `verify.md` synthesis and `openspec archive`. (Design Decision 1, Terminal Consumers Table.)

#### Scenario: Context embedding for judgment executors
- GIVEN a generated `verify-mechanical.log` artifact
- WHEN the orchestrator constructs `packets/verify-<change-id>-agy.md` and `packets/verify-<change-id>-cursor-agent.md`
- THEN each packet's `## Context` section MUST contain the embedded summary and transcript from `verify-mechanical.log`

#### Scenario: Archival record cited in verification report
- GIVEN a completed `openspec/changes/<change-id>/verify.md` report
- WHEN audited by a human reviewer or `openspec archive`
- THEN `verify.md` MUST reference `openspec/changes/<change-id>/verify-mechanical.log` as the mechanical verification record
