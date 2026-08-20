# Mechanical Rerun Prohibition Specification

## Purpose

Prohibit judgment lanes from re-executing mechanical build and test suites inside LLM subshells, enforcing the prohibition through explicit prompt contracts, mandatory hard stops, and runtime git porcelain cleanliness checks.

## Requirements

### Requirement: Explicit Prompt Contract and Out-of-Scope Prohibition

The verify judgment packet prompt MUST explicitly state in `## Out of scope` that executors MUST NOT run `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell build/test commands, because deterministic mechanical results are already frozen in `## Context`. (Design Decision 3.)

#### Scenario: Prompt explicitly forbids mechanical suites
- GIVEN a generated verify judgment packet
- WHEN `## Out of scope` is inspected
- THEN it MUST state that executing `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test suite is out of scope

#### Scenario: Frozen mechanical output provided in context
- GIVEN a generated verify judgment packet
- WHEN `## Context` is inspected
- THEN it MUST embed the frozen execution log from `verify-mechanical.log`

### Requirement: Mandatory Hard Stop Declaration

Every verify judgment packet MUST include the exact hard stop: `Executing mechanical test/build commands when mechanical results are already provided.` Every judgment envelope MUST declare this hard stop in its `hard_stops` array with `fired: false` (or `fired: true` and `status: blocked` if triggered). (Design Decision 3.)

#### Scenario: Hard stop listed in packet prompt
- GIVEN a verify judgment packet
- WHEN `## Hard stops` is parsed
- THEN it MUST include `Executing mechanical test/build commands when mechanical results are already provided.`

#### Scenario: Compliant envelope declares hard stop not fired
- GIVEN a judgment lane that did not execute build or test commands
- WHEN `.lucind/result.json` is written
- THEN `hard_stops` MUST include `{"hard_stop": "Executing mechanical test/build commands when mechanical results are already provided.", "fired": false}`

#### Scenario: Hard stop fires on test execution
- GIVEN a judgment lane executor that runs `go test ./...`
- WHEN `.lucind/result.json` is written
- THEN the hard stop entry MUST have `fired: true`, `note` explaining the execution, and `status` MUST be `blocked`

### Requirement: Structural Cleanliness Enforcement via Git Porcelain

The runtime `run.enforceCompletionMode` MUST demote any read-only judgment lane to `lane.Failed` if any untracked or modified files (such as compiled test binaries, `coverage.out`, `.test` files, or temporary test fixtures) exist in the worktree when the lane claims `Done`. The binary MUST NOT rely solely on prompt compliance. (Design Decision 3.)

#### Scenario: Leftover test artifacts fail porcelain check
- GIVEN a judgment lane whose envelope reports `status: done`, but whose worktree contains an untracked `coverage.out` or compiled test binary
- WHEN `run.enforceCompletionMode` executes
- THEN `PorcelainEmpty` MUST be `false` and the lane status MUST become `lane.Failed` with a descriptive ledger note

#### Scenario: Clean worktree passes completion mode
- GIVEN a judgment lane whose envelope reports `status: done`, with zero commits and empty git porcelain
- WHEN `run.enforceCompletionMode` executes
- THEN the lane status MUST remain `lane.Done`

### Requirement: Read-Only Tool Selection Guidance

Judgment packets MUST guide executors to perform their review using read and navigation tools (`Read`, `Glob`, `Grep`, `codegraph`) and read-only git query commands (such as `git diff`, `git log`, `git show`), avoiding shell execution of build or test runners. (Design Decision 3.)

#### Scenario: Qualitative review via read tools
- GIVEN an executor evaluating spec compliance and test coverage in a judgment lane
- WHEN navigating the codebase
- THEN the executor MUST use code navigation and read tools without running compilation or test commands
