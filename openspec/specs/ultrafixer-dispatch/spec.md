# Ultrafixer Dispatch Specification

## Purpose

Execute ephemeral agy lanes to diagnose and repair pre-existing defects across orchestrated projects via base_sha origin diffing, two-axis evaluation, isolated worktree repair, and human-gated CAS integration.

## Requirements

### Requirement: Origin classification via base_sha diffing

Ultrafixer MUST classify defect origin before evaluating severity or performing repairs. It MUST diff the failure context against the target feature's immutable `base_sha` (`internal/packet/packet.go:68`; `internal/ledger/ledger.go:1342-1350`). If the defect was introduced by changes on the current feature branch between `base_sha` and `HEAD`, ultrafixer MUST exit with status `done` and an explanatory summary, and MUST NOT modify workspace files or generate a repair commit. If the defect pre-existed `base_sha`, ultrafixer MUST proceed to two-axis evaluation.

#### Scenario: Defect introduced by current feature exits cleanly

- GIVEN a failing check caused by code modified between `base_sha` and `HEAD`
- WHEN ultrafixer executes origin classification
- THEN ultrafixer MUST exit with status `done` and a note stating the defect is local to the feature, touching no files

#### Scenario: Pre-existing defect continues to evaluation

- GIVEN a failing check caused by code unmodified between `base_sha` and `HEAD`
- WHEN ultrafixer executes origin classification
- THEN ultrafixer MUST proceed to two-axis critical and blocking evaluation

### Requirement: Independent two-axis evaluation and multi-branch triage

Ultrafixer MUST evaluate pre-existing defects along two independent orthogonal axes: (1) global critical severity (security vulnerability, data loss or corruption hazard, or total CI and build failure), and (2) blocking impact, evaluated separately for the originating branch and every active feature branch discovered via `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`). If a defect is classified as critical globally or blocking for any active feature branch, ultrafixer MUST generate an isolated repair in its worktree.

#### Scenario: Critical non-blocking defect triggers repair

- GIVEN a pre-existing defect classified as a security risk or data corruption hazard that does not block the current feature's build
- WHEN ultrafixer completes two-axis evaluation
- THEN ultrafixer MUST generate an isolated repair, commit the fix, and return a `blocked` result envelope

#### Scenario: Non-critical blocking defect triggers repair for affected branch

- GIVEN a pre-existing defect that breaks tests on feature branch A but does not affect feature branch B
- WHEN ultrafixer completes two-axis evaluation across active features
- THEN ultrafixer MUST generate a repair for branch A, record `Finding.Affects` targeting branch A, and return a `blocked` result envelope

### Requirement: Signal reproduction for cross-branch impact

Ultrafixer MUST use CodeGraph (`codegraph impact`/`codegraph affected`) only as a preliminary candidate filter. Before marking any peer feature branch as affected or blocked in the result envelope, ultrafixer MUST reproduce the failing test, lint, or build signal by executing the check command in that specific candidate branch's worktree. Mere syntactic path or symbol overlap without reproduced failure MUST NOT mark a branch as blocked or affected.

#### Scenario: CodeGraph candidate filter confirmed by failure reproduction

- GIVEN a candidate peer branch identified by CodeGraph symbol impact
- WHEN ultrafixer reproduces the failing check command in the candidate branch worktree and it fails with the same signal
- THEN ultrafixer MUST record the branch as affected in the result envelope's questions and findings

#### Scenario: Syntactic overlap without failure reproduction is not blocked

- GIVEN a candidate peer branch with file or symbol overlap identified by CodeGraph
- WHEN ultrafixer executes the check command in the candidate branch worktree and it passes
- THEN ultrafixer MUST NOT mark the candidate branch as blocked or affected

### Requirement: Isolated repair delivery and human-gated CAS integration

Ultrafixer MUST implement repairs in an isolated git worktree (`../<repo>-worktrees/<lane-id>`) and MUST NOT auto-integrate or push fixes to any branch. Ultrafixer MUST deliver the repair via a schema-valid `blocked` result envelope (`internal/result/result.go:102-115`). Repair worktrees and branches MUST remain preserved on disk upon `blocked` emission or operator decline. CAS promotion MUST be initiated manually by the human operator via `lucind-ai integrate` or `lucind-ai integrate retry` (`internal/run/integrate_retry.go:18-45`). All repair commits MUST use conventional commit formatting with zero AI attribution or `Co-Authored-By` trailers.

#### Scenario: Repair delivered via blocked result envelope

- GIVEN a successful repair and passing test suite in ultrafixer's isolated worktree
- WHEN ultrafixer completes execution
- THEN it MUST emit a `.lucind/result.json` with `status: "blocked"`, populated `commit`, `files_changed`, and a `Question` recommending human integration

#### Scenario: Human accepts fix and triggers integration

- GIVEN a `blocked` ultrafixer result envelope carrying a repair commit
- WHEN the operator runs `lucind-ai integrate retry`
- THEN `lucind-ai` promotes the repair commit using CAS verification against the wave's recorded `expected_parent_sha`

#### Scenario: Human declines fix and worktree is preserved

- GIVEN an operator decision to decline the proposed ultrafixer repair
- WHEN the decision is recorded
- THEN the repair branch and worktree MUST remain preserved on disk, and a declined disposition MUST be recorded in the ledger

### Requirement: Multi-branch blocked disposition encoding

When delivering triage results affecting multiple active feature branches in a `blocked` result envelope, ultrafixer MUST encode per-branch dispositions into repeated `Question` items (`internal/result/result.go:77-82`) and `Finding` items (`internal/result/result.go:95-99`) within the existing result envelope schema (`.lucind/result.schema.json`). Each affected branch MUST have its branch-specific `WhyBlocking`, candidate `Options`, and `Recommendation` populated in a distinct `Question` entry, and failure reproduction evidence recorded in `Finding.Evidence` with the branch identifier in `Finding.Affects`.

#### Scenario: Multi-branch disposition encoded via questions and findings

- GIVEN a pre-existing defect affecting feature branch A and feature branch B with different impact
- WHEN ultrafixer constructs the `blocked` result envelope
- THEN the envelope MUST contain distinct `Question` entries and `Finding` entries targeting each branch respectively without violating `result.schema.json`
