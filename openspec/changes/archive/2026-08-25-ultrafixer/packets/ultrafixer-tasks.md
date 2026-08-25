---
id: ultrafixer-tasks
executor: agy
routed_by: SDD simple (no fan-out) phase dispatch for feature/ultrafixer — tasks phase, following spec (integrated at b03c7f6).
allowed_paths: ["openspec/changes/ultrafixer/tasks.md", "openspec/changes/ultrafixer/state.yaml"]
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: b03c7f642546263a92a3538ccbc07835bce5ae4f
expected_parent_sha: b03c7f642546263a92a3538ccbc07835bce5ae4f
---

# Packet ultrafixer-tasks

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/ultrafixer-tasks  ·  **Branch:** lucind/ultrafixer-tasks

## Goal

Write `openspec/changes/ultrafixer/tasks.md`, the implementation task breakdown for `ultrafixer`,
following this repo's own tasks convention (see
`openspec/changes/archive/2026-08-24-lane-status-observability/tasks.md` in this worktree for the
exact shape: title + one-paragraph rationale, `## Review Workload Forecast` table, `### Suggested
Work Units` table (Unit/Goal/Likely PR/Focused test command/Runtime harness/Rollback boundary), an
apply-order note on shared-file sequencing, then `## Phase N: <name>` sections with numbered
`- [ ] N.M RED: ...` / `- [ ] N.M GREEN: ...` task pairs, each citing exact `file:line` seams and
ending with a `Prove: <go test invocation>` command). **This project runs Strict TDD Mode** — every
implementation task MUST be a RED (failing test first) / GREEN (minimal code to pass) pair; no
task may skip straight to GREEN. Update `openspec/changes/ultrafixer/state.yaml`'s
`phases.tasks.status` to `done`.

## Why this is safe to dispatch now

`design.md` (committed at this packet's `base_sha`) already specifies the exact DDL, Go API
signatures, CLI verbs, packet template asset, and File Changes table needed. The three delta specs
under `openspec/changes/ultrafixer/specs/` (also committed) give the exact MUST/MUST NOT behavior
each implementation task must satisfy. This packet's job is to slice that already-decided scope
into an ordered, TDD-paired task list with review-workload estimates — not to make new design
decisions.

## Preconditions

- `openspec/changes/ultrafixer/proposal.md`, `design.md`, and `specs/*/spec.md` already exist at
  this packet's `base_sha`. Read all of them first, especially `design.md`'s `## File Changes`
  table and `## Interfaces / Contracts` section (exact DDL and Go signatures) and the three spec
  files (exact MUST/MUST NOT requirements each task must satisfy).
- This session's collected SDD preflight: `delivery_strategy: single-pr`,
  `review_budget_lines: 1200`. Compute the Review Workload Forecast honestly against this budget —
  do not under- or over-estimate to force a particular outcome.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** Every task
      that adds a type, field, table, or CLI verb must cite the exact caller/consumer from
      `design.md`'s File Changes table.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and `git log --oneline
      -1`. Conventional commit, no AI attribution.
- [ ] `openspec/changes/ultrafixer/tasks.md` exists, follows the local convention cited above.
- [ ] Every task is a genuine RED/GREEN pair (Strict TDD) — no task skips a failing test.
- [ ] The `## Review Workload Forecast` table gives an honest estimated changed-line range against
      the 1200-line budget, states 400/800/1200-line risk explicitly, and states whether a
      decision is needed before apply given `delivery_strategy: single-pr` (per this repo's
      Review Workload Guard: if risk is High or a decision is needed, `single-pr` means the
      orchestrator must record `size:exception` before apply — say so plainly in the forecast if
      that condition is met).
- [ ] `openspec/changes/ultrafixer/state.yaml`'s `phases.tasks` is updated to `status: done` with
      a short `note:` summarizing the phase/unit count and the forecast headline (updated lines
      estimate, risk level).
- [ ] Every `file:line` citation used in a task is verified against the actual current worktree.

## Allowed paths

- `openspec/changes/ultrafixer/tasks.md`
- `openspec/changes/ultrafixer/state.yaml`

## Allowed paths outside the repository

None.

## Out of scope

- Any actual code change under `internal/`, `cmd/`, or `plugin/` — this phase is planning-only,
  apply comes next.
- Modifying `explore.md`/`proposal.md`/`design.md`/`specs/` (read-only inputs) or
  `/home/lanzerdev/.claude/agents/lucind-ai-fixer.md` (never touch it).
- Inventing scope beyond what `design.md`'s File Changes table already specifies.

## Hard stops

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and the packet does not say which.
- Satisfying one instruction in this packet would require violating another.
- `design.md`'s File Changes table turns out to be incomplete or internally inconsistent in a way
  that blocks writing a coherent task breakdown — name the exact gap, do not silently patch over
  a design decision yourself.

## Context

### design.md's File Changes table (the scope this tasks.md must fully cover, nothing more)

1. `internal/ledger/schema.go` — bump `schemaVersion = 8`, add `migrateV7ToV8DDL` (exact DDL
   already specified in design.md's Interfaces/Contracts section: `defect_records` table with
   columns `id, feature_id, run_id, lane_id, error_signature, evidence, disposition, created_at,
   updated_at`, `disposition CHECK (...)`, plus `idx_defect_records_feature` index), wire the
   migration step following the existing `migrateV6ToV7DDL` pattern (verify the exact
   `migrate` function structure at `internal/ledger/schema.go` around where `migrateV6ToV7DDL` is
   wired — confirmed present in this codebase).
2. `internal/ledger/ledger.go` — add `DefectRecord` struct, `DefectDisposition` type + constants,
   `RecordDefect`, `ListDefects`, `GetDefect` methods (exact signatures in design.md).
3. `cmd/lucind-ai/cli.go` — add `defect` subcommand routing (`record`, `list`).
4. `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md` — new packet
   template asset (exact shape already drafted in design.md's Interfaces/Contracts section —
   verify it matches the structure of the existing `assets/packet-template.md` in this worktree).
5. `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md` — update
   the manual defect protocol to describe structured ultrafixer packet execution (per the
   `dependencies-defects` delta spec under `openspec/changes/ultrafixer/specs/`).

### Existing precedent for schema migration tests (follow this exact pattern for the v7→v8 RED/GREEN pair)

`internal/ledger/schema_test.go` already has precedent tests for the v6→v7 migration (e.g.
`TestMigrateV6ToV7PreservesRowsAndAddsSchema`, `TestSchemaV7ConstraintsAndIndexes`,
`TestSchemaV7ReopenIsIdempotent` per the lane-status-observability tasks.md referenced above) —
read that file to confirm the exact naming/assertion pattern and mirror it for v7→v8.

### Test/validation impact already scoped in design.md

design.md's `## Testing Strategy and Test Seams` table already names the layers (ledger schema
migration + CRUD, origin-classification diff logic — noting this may already be adequately covered
by existing `internal/overlap`/`internal/resolve` tests since no new Go code is needed there per
the "zero new Go dispatch plumbing" decision, result envelope multi-branch encoding, ultrafixer
packet parsing/execution, CAS retry, and the new `defect` CLI verbs) — use it as the basis for
phase/unit slicing, but verify against the actual current test files rather than assuming test
function names that don't yet exist are already present.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well
the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
