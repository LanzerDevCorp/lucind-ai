# Verify Judgment Packet Specification

## Purpose

Define the packet contract, YAML frontmatter, done-criteria, result envelope reuse, and template assets for parallel read-only qualitative judgment packets evaluated by independent executor subscriptions.

## Requirements

### Requirement: Read-Only Judgment Packet Frontmatter

Each verify judgment packet MUST set `read_only: true` in its YAML frontmatter, alongside `id: verify-<change-id>-<executor>`, `executor: agy` (or `executor: cursor-agent`), and a descriptive `routed_by` string. `packet.Parse` MUST parse `read_only: true` into `Packet.ReadOnly = true`. (Design Decision 2.)

#### Scenario: Read-only frontmatter parsing
- GIVEN a packet file `packets/verify-add-auth-agy.md` containing `read_only: true`, `id: verify-add-auth-agy`, and `executor: agy`
- WHEN `packet.Parse` parses the file
- THEN `Packet.ReadOnly` MUST be `true`, `Packet.ID` MUST be `verify-add-auth-agy`, and `Packet.Executor` MUST be `agy`

#### Scenario: Dual executor packet authoring
- GIVEN an SDD verify phase for change `add-auth`
- WHEN the orchestrator generates judgment packets
- THEN it MUST author two distinct packet files: `packets/verify-add-auth-agy.md` with `executor: agy` and `packets/verify-add-auth-cursor-agent.md` with `executor: cursor-agent`

### Requirement: Read-Only Done-Criteria Contract

Verify judgment packets MUST define exactly three done-criteria:
1. Terminal consumer tracing: verification citations MUST trace to concrete symbols, tests, and spec requirements.
2. Unchanged tree invariant: `git status --porcelain` MUST be empty AND `HEAD` MUST equal `git merge-base HEAD <primary HEAD>`.
3. Qualitative evaluation completed: `.lucind/result.json` MUST be populated with `status`, `summary`, and structured `findings`.
Verify judgment packets MUST NOT declare write-packet criteria requiring new git commits. (Design Decision 2.)

#### Scenario: Criterion 2 unchanged-tree evidence
- GIVEN a verify judgment packet reporting `status: done`
- WHEN done-criterion 2 is evaluated
- THEN evidence MUST confirm `git status --porcelain` is empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`

#### Scenario: Indirection tracing in criterion 1
- GIVEN an executor completing a verify judgment packet
- WHEN it populates criterion 1 evidence
- THEN evidence MUST cite concrete symbol names, spec requirement headings, or test functions consumed in the review

#### Scenario: Commit criterion forbidden on judgment packets
- GIVEN a verify judgment packet document
- WHEN its done-criteria are authored
- THEN it MUST NOT require a unique git commit or non-empty commit hash

### Requirement: Result Envelope Shape and Schema Reuse

Judgment executors MUST return their qualitative verdict via `.lucind/result.json` adhering strictly to the existing `result.schema.json`. The envelope MUST omit the `commit` field (or leave it empty). The envelope MUST NOT introduce custom verdict properties or phase-specific schema extensions. Qualitative observations MUST be reported in the standard `findings` array with `finding`, `evidence` (`file:line`), and `affects`. (Design Decision 2.)

#### Scenario: Standard envelope validation without commit
- GIVEN a judgment lane result envelope with `packet_id`, `status: "done"`, `summary: "VERDICT: PASS..."`, `hard_stops`, `done_criteria`, and `findings`, with `commit` omitted
- WHEN validated against `result.schema.json`
- THEN validation MUST succeed with exit code 0

#### Scenario: Additional verdict properties rejected
- GIVEN a judgment lane result envelope containing a custom top-level property such as `"verdict": "pass"`
- WHEN validated against `result.schema.json` with `additionalProperties: false`
- THEN validation MUST fail

#### Scenario: Findings report file and line evidence
- GIVEN a judgment lane identifying an edge-case gap
- WHEN `.lucind/result.json` is written
- THEN `findings` MUST contain an entry with `finding` description, concrete `file:line` `evidence`, and `affects` impact

### Requirement: Verify Packet Template Asset

The system MUST provide a standardized template asset at `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`. The template MUST include `read_only: true` frontmatter, the three read-only done-criteria, standard mechanical rerun hard stops, and evaluation sections for spec compliance, edge cases, and test quality. `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` MUST include a reference note pointing authors of qualitative review lanes to `verify-packet-template.md`. (Design Decision 2, Decision 5.)

#### Scenario: Verify packet template skeleton
- GIVEN `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`
- WHEN inspected
- THEN frontmatter MUST include `read_only: true` and the body MUST include the three read-only done criteria

#### Scenario: Packet template reference note
- GIVEN `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`
- WHEN inspected
- THEN it MUST contain a pointer note directing authors of qualitative verification lanes to `verify-packet-template.md`
