# SDD Apply Specification

## Purpose

Shift apply from in-process filesystem edits in the primary repository to authoring packet files, driving sequential `lucind-ai run` waves, and handling returned integration reports — without modifying combine, resolve, bisect, or the ledger schema.

## Requirements

### Requirement: Apply Authors Packets, Not Primary Diffs

After this change, `sdd-apply` MUST implement an SDD apply by authoring packet files (and, when a DAG is wanted, `apply-dag.yaml`) and dispatching them through `lucind-ai run`. It MUST NOT write the apply diff itself via in-session Read/Edit/Write against the primary repository. (Design: Decision 1, Decision 3; proposal: Impact on the existing `sdd-apply` flow)

#### Scenario: Apply is a DAG of lucind-ai run packets

- GIVEN an SDD change whose tasks have been split into packets
- WHEN apply runs
- THEN each packet MUST execute as a real lane (worktree, envelope, barrier) via `lucind-ai run`, not as an in-process edit on primary

#### Scenario: Primary is not the apply session's write target

- GIVEN `sdd-apply` is performing this change's apply path
- WHEN a task's code is written
- THEN the write MUST occur in the lane worktree created by `lucind-ai run`, not in the orchestrator's primary checkout

### Requirement: Absent Sidecar Preserves Hand-Split Apply

When `apply-dag.yaml` is absent, apply MUST still be allowed to proceed as today: one packet, or a hand-split set of packets, without running `lucind-ai split`. The sidecar is optional and authored only when DAG dispatch is wanted. (Design: Decision 1, Decision 5)

#### Scenario: No sidecar, apply still runs

- GIVEN an SDD change with `tasks.md` and no `apply-dag.yaml`
- WHEN apply is dispatched
- THEN the orchestrator MUST NOT be required to run `lucind-ai split`, and apply MAY proceed as a single packet or hand-authored packets

#### Scenario: Sidecar present drives split then waves

- GIVEN an SDD change with a valid `apply-dag.yaml`
- WHEN apply is dispatched as a DAG
- THEN the orchestrator MUST run `lucind-ai split` and MUST execute the printed wave commands in order

### Requirement: Orchestrator Consumes Integrate Reports

The orchestrator MUST advance to wave N+1 only when wave N's `lucind-ai run` exits 0. Exit 0 MUST mean every lane is `done` and none were reverted. On a non-zero exit the orchestrator MUST halt the remaining DAG for human review or replanning. (Design: Decision 3, Decision 4)

#### Scenario: Passed wave advances

- GIVEN wave N stdout reports `passed=true` and the process exits 0
- WHEN the orchestrator considers wave N+1
- THEN it MUST dispatch the next printed `lucind-ai run` command

#### Scenario: Reverted or blocked wave stops the DAG

- GIVEN wave N exits non-zero because a lane is `blocked`, `deviated`, `failed`, or listed in `reverted_ids`
- WHEN the orchestrator considers further waves
- THEN it MUST NOT dispatch them

### Requirement: Stdout Lists Integrated and Reverted IDs

`printReport` MUST print `integrated_ids` and `reverted_ids` alongside the existing `integrate:` counts and `reason`. It MUST NOT add a `--json` flag or a `.lucind/runs/<run_id>.json` file as the way to learn those IDs. The six existing ledger event types MUST remain sufficient; no new event type is required. (Design: Decision 4)

#### Scenario: Bisected-out lane ID is printed

- GIVEN a wave where one lane printed `status: done` at dispatch time and `revertLanes` later bisected it out
- WHEN `printReport` writes stdout
- THEN `reverted_ids` MUST list that lane's ID and `integrated_ids` MUST NOT

#### Scenario: Counts and IDs appear together

- GIVEN `IntegrateReport` with `Integrated=["apply-ledger"]` and `Reverted=["apply-serve"]`
- WHEN `printReport` writes stdout
- THEN stdout MUST include the existing `integrate: attempted=… passed=… integrated=1 reverted=1` line and also `integrated_ids: apply-ledger` and `reverted_ids: apply-serve`

### Requirement: Combine, Resolve, and Bisect Stay Untouched

Apply-wave integration MUST call Combine, `resolve.Resolve`, Check, bisect, and Promote exactly as they exist. `internal/run/integrate.go`, `internal/resolve/resolve.go`, and `internal/integrate/integrate.go` MUST NOT be modified by this change. (Design: Decision 3, Decision 5; proposal: What stays untouched)

#### Scenario: Existing integrator is called, not copied

- GIVEN a wave whose `done` lanes reach Integrate
- WHEN conflicts, a failing check, or a mixed batch occur
- THEN the existing 400-line resolver and bisection path MUST handle them, with no DAG-specific replacement

### Requirement: Additive Rollback With No Ledger Migration

`AllowedPaths` MUST NOT be stored on the `lanes` table. This change MUST NOT add a ledger column or a new ledger event type. Omitting `AllowedPaths`, leaving `lucind-ai split` unused, and omitting `apply-dag.yaml` MUST each restore today's corresponding behavior without a schema migration. (Design: Decision 5)

#### Scenario: No SQLite schema change

- GIVEN apply-DAG dispatch running against an existing ledger
- WHEN packets with `AllowedPaths` are dispatched
- THEN the ledger MUST accept the run with the existing schema and event types

#### Scenario: Field omitted skips enforcement

- GIVEN packets that omit `allowed_paths`
- WHEN they are dispatched after this change ships
- THEN both the overlap check and the diff check MUST stay skipped, matching pre-change behavior
