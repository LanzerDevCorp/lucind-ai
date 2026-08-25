---
id: ultrafixer-spec
executor: agy
routed_by: SDD simple (no fan-out) phase dispatch for feature/ultrafixer — spec phase, following design (integrated at ac0c816).
allowed_paths: ["openspec/changes/ultrafixer/specs/", "openspec/changes/ultrafixer/state.yaml"]
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: ac0c816db4542014678b988ea7813382d29457b4
expected_parent_sha: ac0c816db4542014678b988ea7813382d29457b4
---

# Packet ultrafixer-spec

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/ultrafixer-spec  ·  **Branch:** lucind/ultrafixer-spec

## Goal

Write delta spec files under `openspec/changes/ultrafixer/specs/<capability>/spec.md`, one per
affected capability, following this repo's own delta-spec convention (see
`openspec/changes/control-room-capture/specs/lane-execution/spec.md` and
`openspec/changes/control-room-capture/specs/control-room-capture/spec.md` in this worktree for
the exact shape: `# Delta for <capability>`, then `## ADDED Requirements` and/or `## MODIFIED
Requirements`, each a `### Requirement: <name>` with normative MUST/MUST NOT prose, followed by
one or more `#### Scenario: <name>` blocks in GIVEN/WHEN/THEN form). Update
`openspec/changes/ultrafixer/state.yaml`'s `phases.spec.status` to `done`.

## Why this is safe to dispatch now

`proposal.md` already contains four fully-formed `### Requirement:` blocks with GIVEN/WHEN/THEN
scenarios under its own `## Delta Specifications` section, and `design.md` resolved every open
engineering question (schema v8 DDL, multi-branch encoding, worktree pruning) and made one
important correction to proposal.md's capability list (see Context below). This packet's job is
to reorganize and finalize those already-written requirements into the correct per-capability
delta spec files — not invent new requirements from scratch, and not re-litigate the confirmed
design.

## Preconditions

- `openspec/changes/ultrafixer/proposal.md` and `design.md` already exist at this packet's
  `base_sha` (both committed and integrated). Read both in full first.
- No `openspec/changes/ultrafixer/specs/` directory exists yet — you are creating it.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** Every
      Requirement you write must map to a concrete mechanism named in `design.md`'s Architecture
      Decisions or File Changes table — not an invented capability with nothing behind it.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and `git log --oneline
      -1`. Conventional commit, no AI attribution.
- [ ] One `spec.md` file exists per capability that genuinely needs a delta, under
      `openspec/changes/ultrafixer/specs/<capability>/spec.md`, following the local convention.
- [ ] You explicitly resolved the capability-list question in Context below (does `lane-execution`
      still need a MODIFIED delta given design.md's "zero new Go dispatch plumbing" conclusion, or
      does it drop out entirely) rather than copying proposal.md's capability list uncritically.
- [ ] `openspec/changes/ultrafixer/state.yaml`'s `phases.spec` is updated to `status: done` with a
      short `note:` listing exactly which capability spec files were written (do not touch any
      other phase's entry).
- [ ] Every `file:line` citation used inside a Requirement's prose (if any) is verified against
      the actual current worktree, not copied blind.

## Allowed paths

- `openspec/changes/ultrafixer/specs/`
- `openspec/changes/ultrafixer/state.yaml`

## Allowed paths outside the repository

None.

## Out of scope

- Writing tasks.md — the next phase, dispatched separately.
- Any actual code change under `internal/`, `cmd/`, or `plugin/` — this phase is spec-only.
- Modifying `explore.md`/`proposal.md`/`design.md` (read-only inputs) or
  `/home/lanzerdev/.claude/agents/lucind-ai-fixer.md` (never touch it).
- Inventing requirements not grounded in `proposal.md`'s Delta Specifications or `design.md`'s
  Architecture Decisions.

## Hard stops

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and the packet does not say which.
- Satisfying one instruction in this packet would require violating another.
- `proposal.md` and `design.md` genuinely disagree about a capability's scope in a way you cannot
  resolve by design.md taking precedence (design.md's engineering conclusions supersede
  proposal.md's earlier product-level draft where they conflict) — if even that doesn't resolve
  it, stop and name the exact contradiction.

## Context

### Capabilities named in proposal.md (verify against design.md before finalizing)

- `ultrafixer-dispatch` (New) — the packet-driven triage/repair Lane itself.
- `defect-records` (New) — ledger schema v8 persistence for non-critical/non-blocking defects.
- `dependencies-defects` (Modified) — the docs-only coordination contract in
  `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md:7-23`,
  upgraded from a purely manual contract to one describing structured ultrafixer packet execution.
- `lane-execution` (Modified, per proposal.md) — **check this against `design.md`'s "Zero new Go
  dispatch plumbing for ultrafixer-dispatch" decision**, which concluded `internal/run/`,
  `internal/executor/`, and `internal/packet/` are reused **verbatim**, with zero code changes.
  If `lane-execution` genuinely has no behavioral delta (nothing in it actually changes), it should
  NOT get a delta spec file here — drop it, and say so explicitly in your `state.yaml` note. Do
  not write an empty or content-free delta spec just to match proposal.md's original list.

### proposal.md's four already-drafted Delta Specifications (source material — reorganize, do not rewrite from scratch)

1. "Origin classification via `base_sha` diffing" (with two scenarios: introduced-by-feature exits
   cleanly; pre-existing continues to evaluation) — belongs under `ultrafixer-dispatch`.
2. "Independent two-axis evaluation and multi-branch triage" (three scenarios: critical
   non-blocking triggers repair; non-critical blocking triggers repair for the affected branch;
   non-critical non-blocking records a Defect Record only) — the first two scenarios belong under
   `ultrafixer-dispatch`; the third scenario ("persist a Defect Record... MUST NOT create a fix
   commit") is really a `defect-records` requirement (ledger persistence), not an
   `ultrafixer-dispatch` one — split accordingly.
3. "Signal reproduction for cross-branch impact" (two scenarios: CodeGraph filter confirmed by
   reproduction; syntactic overlap without reproduction is not blocked) — belongs under
   `ultrafixer-dispatch`.
4. "Isolated repair delivery and human-gated CAS integration" (three scenarios: repair delivered
   via blocked envelope; human accepts and triggers integration; human declines and worktree is
   preserved) — belongs under `ultrafixer-dispatch`.

### design.md's concrete DDL/API for grounding the `defect-records` delta spec

Schema v8 `defect_records` table (`internal/ledger/schema.go:10`, new `migrateV7ToV8DDL`), columns
`id, feature_id, run_id, lane_id, error_signature, evidence, disposition, created_at, updated_at`,
`disposition CHECK (... IN ('recorded','repaired','declined','deferred'))`. New Go API:
`Ledger.RecordDefect`, `Ledger.ListDefects`, `Ledger.GetDefect` (`internal/ledger/ledger.go`). New
CLI: `lucind-ai defect record` / `lucind-ai defect list` (`cmd/lucind-ai/cli.go`, per design.md's
File Changes table).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well
the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
