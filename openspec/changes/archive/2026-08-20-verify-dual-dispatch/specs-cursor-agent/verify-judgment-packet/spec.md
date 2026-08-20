# Verify Judgment Packet Specification

## Purpose

Define the contract and schema for read-only qualitative verification packets, including `read_only: true` frontmatter, unchanged-tree done criteria, result envelope findings shape, explicit prohibition of mechanical test re-runs, and structural enforcement via git porcelain cleanliness.

## Requirements

### Requirement: Read-Only Packet Frontmatter

Qualitative verification judgment packets MUST declare `read_only: true` in YAML frontmatter. `internal/packet.Parse` MUST parse this into `Packet.ReadOnly = true`, which `internal/run.enforceCompletionMode` inspects to enforce read-only completion invariants. (Design Decision 2.)

#### Scenario: Valid verify judgment packet parsed as read-only
- GIVEN a verify packet containing `read_only: true`, `id: verify-<change-id>-agy`, `executor: agy`, and `routed_by: qualitative verification`
- WHEN `packet.Parse` parses the packet
- THEN `Packet.ReadOnly` MUST be `true` and parse MUST succeed

#### Scenario: Verification packet without read_only is rejected
- GIVEN a verification judgment packet that omits `read_only` or sets `read_only: false`
- WHEN the orchestrator validates verify packet declarations before dispatch
- THEN it MUST reject the packet as non-compliant with the verify contract

### Requirement: Read-Only Done Criteria Invariant

Verification judgment packets MUST specify the read-only done-criteria set from `read-only-packet-dispatch`, replacing the write-packet commit requirement (criterion 2) with the unchanged-tree invariant. The criteria set MUST be: (1) every indirection demonstrably consumed; (2) worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty and `HEAD == git merge-base HEAD <primary HEAD>`); (3) qualitative evaluation completed. (Design Decision 2.)

#### Scenario: Unchanged tree satisfies criterion 2
- GIVEN a verify lane reporting `status: done` in `.lucind/result.json`
- WHEN criterion 2 is evaluated
- THEN evidence MUST show `git status --porcelain` empty and `HEAD` equals `git merge-base HEAD <primary HEAD>`

#### Scenario: Unique commits violate criterion 2
- GIVEN a verify lane that authored commits on its branch
- WHEN criterion 2 is evaluated
- THEN the criterion MUST NOT be considered met and `enforceCompletionMode` MUST demote the lane to `lane.Failed`

### Requirement: Existing Envelope Schema Reuse Without Churn

Qualitative judgment verdicts MUST return through standard `.lucind/result.json` envelopes using existing `result.schema.json` fields (`status`, `summary`, `findings`, `hard_stops`, `done_criteria`). The `commit` field MUST be omitted. The envelope schema MUST NOT add custom verify-specific properties. (Design Decision 2.)

#### Scenario: Clean pass verdict envelope
- GIVEN a verify lane evaluating an implementation with full spec compliance and passing checks
- WHEN `.lucind/result.json` is written
- THEN `status` MUST be `done`, `summary` MUST record `VERDICT: PASS`, `commit` MUST be omitted, and `hard_stops` MUST report the re-run prohibition

#### Scenario: Defect verdict envelope with structured findings
- GIVEN a verify lane identifying an unhandled edge case or spec violation
- WHEN `.lucind/result.json` is written
- THEN `status` MUST be `blocked`, `summary` MUST describe the defect, and `findings` MUST contain structured objects with `finding`, `evidence` (`file:line`), and `affects`

#### Scenario: Result envelope passes strict schema validation
- GIVEN a written verify result envelope
- WHEN validated against `internal/result/result.schema.json` with `additionalProperties: false`
- THEN validation MUST succeed without schema errors

### Requirement: Mechanical Re-Run Prohibition Contract

The verify judgment packet prompt MUST explicitly prohibit executing mechanical build and test suites (`go test`, `go build`, `go vet`, `lucind-checks.sh`, or shell test scripts) in both `## Out of scope` and `## Hard stops`. The envelope's `hard_stops` array MUST report this condition explicitly. (Design Decision 3.)

#### Scenario: Out of scope prompt clause
- GIVEN the verify judgment packet prompt body
- WHEN inspected
- THEN `## Out of scope` MUST explicitly state that executing `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suite is forbidden

#### Scenario: Mandatory hard stop declared and reported
- GIVEN a verify judgment packet declaring the hard stop "Executing mechanical test suites or build commands when mechanical results are already provided."
- WHEN the executor writes `.lucind/result.json`
- THEN the `hard_stops` array MUST contain that exact hard stop string with `fired: false`

### Requirement: Structural Cleanliness Enforcement on Test Artifacts

`internal/run.enforceCompletionMode` MUST inspect git working tree state after `decideStatus` returns `Done`. If a judgment lane executes test commands that leave untracked artifacts (`coverage.out`, compiled test binaries, temporary test fixtures) or creates working-tree diffs, `enforceCompletionMode` MUST fail `PorcelainEmpty` and demote the lane to `lane.Failed`. (Design Decision 3.)

#### Scenario: Untracked test artifact fails the lane
- GIVEN a verify lane that ran a test suite leaving `coverage.out` in the worktree
- WHEN `enforceCompletionMode` runs after `decideStatus` maps to `Done`
- THEN `PorcelainEmpty` MUST return `false` and the lane status MUST become `lane.Failed` with a ledger note

#### Scenario: Clean read-only analysis passes enforcement
- GIVEN a verify lane that performed read-only inspection and wrote only `.lucind/result.json`
- WHEN `enforceCompletionMode` runs
- THEN `PorcelainEmpty` MUST return `true`, `HasUniqueLaneCommits` MUST return `false`, and the lane status MUST remain `lane.Done`

### Requirement: Standardized Verify Packet Template and Assets

A dedicated qualitative verify judgment packet template MUST be provided at `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`. `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` MUST include a reference note pointing to `verify-packet-template.md` for qualitative verification lanes. The template MUST guide executors toward read and navigation tools (`Read`, `Glob`, `Grep`, `codegraph`) and read-only git inspection. (Design Decision 2, Decision 3.)

#### Scenario: Verify packet template structure
- GIVEN `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`
- WHEN inspected
- THEN it MUST include `read_only: true` frontmatter, unchanged-tree done criteria, re-run prohibition hard stops, and qualitative evaluation prompt sections

#### Scenario: General packet template references verify template
- GIVEN `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`
- WHEN inspected
- THEN it MUST include a pointer note directing authors of qualitative verification packets to `verify-packet-template.md`
