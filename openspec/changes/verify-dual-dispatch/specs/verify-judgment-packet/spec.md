# Verify Judgment Packet Specification

## Purpose

Define the contract and schema for read-only qualitative verification judgment packets:
`read_only: true` frontmatter, the unchanged-tree done-criteria set, result-envelope reuse
without schema churn, explicit prohibition of mechanical re-runs, structural enforcement via git
porcelain cleanliness, tool-selection guidance, and the standardized template asset.

## Requirements

### Requirement: Read-Only Judgment Packet Frontmatter

Each verify judgment packet MUST set `read_only: true` in its YAML frontmatter, alongside
`id: verify-<change-id>-<executor>`, `executor: agy` (or `executor: cursor-agent`), and a
descriptive `routed_by` string. `internal/packet.Parse` MUST parse `read_only: true` into
`Packet.ReadOnly = true`, which `internal/run.enforceCompletionMode` inspects to enforce
read-only completion invariants.

#### Scenario: Read-only frontmatter parsing
- GIVEN a packet file `packets/verify-add-auth-agy.md` containing `read_only: true`, `id: verify-add-auth-agy`, and `executor: agy`
- WHEN `packet.Parse` parses the file
- THEN `Packet.ReadOnly` MUST be `true`, `Packet.ID` MUST be `verify-add-auth-agy`, and `Packet.Executor` MUST be `agy`

#### Scenario: Dual executor packet authoring
- GIVEN an SDD verify phase for change `add-auth`
- WHEN the orchestrator generates judgment packets
- THEN it MUST author two distinct packet files: `packets/verify-add-auth-agy.md` with `executor: agy` and `packets/verify-add-auth-cursor-agent.md` with `executor: cursor-agent`

#### Scenario: Verification packet without read_only is rejected pre-dispatch
- GIVEN a verify judgment packet draft that omits `read_only` or sets `read_only: false`
- WHEN the orchestrator validates verify packet declarations before dispatch
- THEN it MUST reject the packet as non-compliant with the verify contract

### Requirement: Read-Only Done-Criteria Contract

Verify judgment packets MUST define exactly three done-criteria: (1) every indirection
introduced is demonstrably consumed by a terminal consumer, with evidence citing concrete
symbols, spec requirements, or tests; (2) the unchanged-tree invariant — `git status --porcelain`
empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>` — restated exactly from
`read-only-packet-dispatch/design.md` Decision 2; (3) qualitative evaluation completed, evidenced
by `.lucind/result.json` populated with `status`, `summary`, and structured `findings`. Verify
judgment packets MUST NOT declare a write-packet criterion requiring a new git commit.

#### Scenario: Criterion 2 unchanged-tree evidence
- GIVEN a verify judgment packet reporting `status: done`
- WHEN done-criterion 2 is evaluated
- THEN evidence MUST confirm `git status --porcelain` is empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`

#### Scenario: Unique commits violate criterion 2
- GIVEN a verify lane that authored commits on its branch
- WHEN `enforceCompletionMode` evaluates criterion 2
- THEN the criterion MUST NOT be considered met and the lane MUST be demoted to `lane.Failed`

#### Scenario: Commit criterion forbidden on judgment packets
- GIVEN a verify judgment packet document
- WHEN its done-criteria are authored
- THEN it MUST NOT require a unique git commit or non-empty commit hash

### Requirement: Existing Envelope Schema Reuse Without Churn

Judgment executors MUST return their qualitative verdict via `.lucind/result.json` adhering
strictly to the existing `result.schema.json` (`status`, `summary`, `findings`, `hard_stops`,
`done_criteria`). The `commit` field MUST be omitted or left empty. The envelope MUST NOT
introduce custom verdict properties or phase-specific schema extensions. Qualitative
observations MUST be reported in the standard `findings` array with `finding`, `evidence`
(`file:line`), and `affects`.

#### Scenario: Standard envelope validation without commit
- GIVEN a judgment lane result envelope with `packet_id`, `status: "done"`, `summary`, `hard_stops`, `done_criteria`, and `findings`, with `commit` omitted
- WHEN validated against `result.schema.json`
- THEN validation MUST succeed with exit code 0

#### Scenario: Additional verdict properties rejected
- GIVEN a judgment lane result envelope containing a custom top-level property such as `"verdict": "pass"`
- WHEN validated against `result.schema.json` with `additionalProperties: false`
- THEN validation MUST fail

#### Scenario: Findings report file and line evidence
- GIVEN a judgment lane identifying an edge-case gap
- WHEN `.lucind/result.json` is written
- THEN `findings` MUST contain an entry with a `finding` description, concrete `file:line` `evidence`, and `affects` impact

### Requirement: Mechanical Re-Run Prohibition Contract

The verify judgment packet prompt MUST explicitly prohibit executing mechanical build and test
suites (`go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build command) in
both `## Out of scope` and `## Hard stops`, because deterministic mechanical results are already
frozen in `## Context`. Every judgment envelope MUST declare this hard stop in its `hard_stops`
array (`fired: false` when compliant; `fired: true` with `status: blocked` when violated).

#### Scenario: Out-of-scope prompt clause
- GIVEN the verify judgment packet prompt body
- WHEN `## Out of scope` is inspected
- THEN it MUST explicitly state that executing `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suite is forbidden

#### Scenario: Mandatory hard stop declared and reported
- GIVEN a verify judgment packet declaring the hard stop "Executing mechanical test/build commands when mechanical results are already provided."
- WHEN the executor writes `.lucind/result.json`
- THEN `hard_stops` MUST contain that exact hard stop string with `fired: false`

#### Scenario: Hard stop fires on mechanical execution
- GIVEN a judgment lane executor that runs `go test ./...`
- WHEN `.lucind/result.json` is written
- THEN the hard stop entry MUST have `fired: true`, an explanatory note, and `status` MUST be `blocked`

### Requirement: Structural Cleanliness Enforcement via Git Porcelain

`internal/run.enforceCompletionMode` MUST inspect real git state after `decideStatus` returns
`Done`, not trust prompt compliance alone. If a judgment lane's test/build execution leaves
untracked or modified artifacts (`coverage.out`, compiled test binaries, temporary fixtures) in
the worktree, `enforceCompletionMode` MUST fail `PorcelainEmpty` and demote the lane to
`lane.Failed` with a descriptive ledger note.

#### Scenario: Leftover test artifacts fail the porcelain check
- GIVEN a judgment lane whose envelope reports `status: done`, but whose worktree contains an untracked `coverage.out` or compiled test binary
- WHEN `run.enforceCompletionMode` executes
- THEN `PorcelainEmpty` MUST be `false` and the lane status MUST become `lane.Failed` with a ledger note

#### Scenario: Clean worktree passes completion mode
- GIVEN a judgment lane whose envelope reports `status: done`, with zero unique commits and empty git porcelain
- WHEN `run.enforceCompletionMode` executes
- THEN the lane status MUST remain `lane.Done`

### Requirement: Read-Only Tool Selection Guidance

Judgment packets MUST guide executors to perform their review using read and navigation tools
(`Read`, `Glob`, `Grep`, `codegraph`) and read-only git query commands (`git diff`, `git log`,
`git show`), rather than shell execution of build or test runners.

#### Scenario: Qualitative review performed with read tools only
- GIVEN an executor evaluating spec compliance and test coverage in a judgment lane
- WHEN navigating the codebase
- THEN the executor MUST use code navigation and read tools without running compilation or test commands

### Requirement: Standardized Verify Packet Template Asset

The system MUST provide a standardized template at
`plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`, including `read_only:
true` frontmatter, the three read-only done-criteria, the mechanical-rerun hard stop, and
evaluation sections for spec compliance, edge cases, and test quality.
`plugin/claude-code/skills/lucind-ai/assets/packet-template.md` MUST include a reference note
pointing authors of qualitative review lanes to `verify-packet-template.md`.

#### Scenario: Verify packet template skeleton
- GIVEN `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`
- WHEN inspected
- THEN frontmatter MUST include `read_only: true` and the body MUST include the three read-only done criteria and the mechanical-rerun hard stop

#### Scenario: Packet template reference note
- GIVEN `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`
- WHEN inspected
- THEN it MUST contain a pointer note directing authors of qualitative verification lanes to `verify-packet-template.md`
