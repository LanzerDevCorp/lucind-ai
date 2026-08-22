# Spec Lens C — Live-Spec Conflicts & Migration: Control Room Ledger

## Assumed requirements

We assume the proposal establishes three new capabilities (`run-lifecycle-ledger`, `lane-progress-stream`, `progress-stream-pruning`) and modifies two existing live capabilities (`lane-execution`, `approvals-web-ui`). For `lane-execution`, this change is expected to assert schema v6 lane dispatch metadata persistence (`model`, `agent`, `feature`) during `RegisterLane` and widen `events.type` CHECK constraints to admit `run_status_changed`, while preserving existing lifecycle gate placement and the six-value `lane.Status` enum. For `approvals-web-ui`, this change is expected to assert typed, shell-free SQLite DTO read queries on `serve.Model` (`GetRun`, `ListRuns`, `GetLaneProgress`) for run summaries and progress tails, leaving loopback binding, per-item decisions, inline evidence rendering, and approver wrong-approval rate calculations unchanged. For the new capabilities, requirements assert durable `runs` table row lifecycle tracking, sequenced `lane_progress` appending with cursor tail reads, and isolated progress cutoff pruning without cascading deletions.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `lane-execution` | `openspec/specs/lane-execution/spec.md:1-62` | 3 | 6 | Extended: schema v6 persists `model`, `agent`, `feature` on `RegisterLane` and admits `run_status_changed` in `events`; existing gate placement, barrier observation, and six-value `lane.Status` enum invariants preserved |
| `approvals-web-ui` | `openspec/specs/approvals-web-ui/spec.md:1-83` | 4 | 9 | Extended: `serve.Model` adds shell-free SQLite DTOs for run summaries and progress tails; existing loopback binding, individual decisions without bulk approval, inline evidence, and approver rate requirements unchanged |

## Conflicts

None. The change extends `lane-execution` and `approvals-web-ui` additively rather than contradicting or invalidating any live requirement:
- In `lane-execution`, `Gate Placement in the Lifecycle`, `Resolve Before Barrier Observation`, and `Additive Schema, Unchanged Enum` remain strictly true. Persisting `model`, `agent`, and `feature` columns on `lanes` via `RegisterLane` and widening `events.type` to include `run_status_changed` adds dispatch metadata and audit types without modifying approval gate sequencing or adding a 7th value to `lane.Status`.
- In `approvals-web-ui`, `Loopback Binding`, `Individual Decisions Without Bulk Approval`, `Inline Evidence and Batch Review Command`, and `Approver Wrong-Approval Rate` remain strictly true. Adding shell-free SQLite query DTOs on `serve.Model` enables handlers to serve run metadata and progress tails without altering approval decision workflows or rate calculations.

Because the new behaviors do not contradict any existing requirement text or scenarios, these are ADDED requirements for the respective delta specs, not MODIFIED requirement conflicts.

## MODIFIED Full Blocks

None. No existing live requirements in `openspec/specs/lane-execution/spec.md` or `openspec/specs/approvals-web-ui/spec.md` are being modified or replaced. All changes to both capabilities introduce new requirements (e.g., `Lane Dispatch Metadata Persistence` and `Admit Run Status Changed Event Type` for `lane-execution`, and `Shell-Free Run and Progress Model DTOs` for `approvals-web-ui`).

If the synthesizer determines that any live requirement should be updated rather than extended, the complete live blocks from the inventory above are available verbatim in their respective source files (`openspec/specs/lane-execution/spec.md:10-62` and `openspec/specs/approvals-web-ui/spec.md:10-83`).

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| None | None | No requirements are removed or renamed by this change. | N/A | None |

## Open Questions

- [ ] Execution-topology note: Spec authoring fans out across three parallel lenses (Lens A, Lens B, Lens C) feeding a synthesis lane per packet authorization, superseding single-subagent delta spec generation in `~/.claude/skills/sdd-spec/SKILL.md`.
