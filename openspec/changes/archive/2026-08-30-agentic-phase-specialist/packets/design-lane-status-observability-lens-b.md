---
id: design-lane-status-observability-lens-b
executor: agy
routed_by: telemetry/schema-v7 lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/design-lens-b.md"]
---

# Packet design-lane-status-observability-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-lane-status-observability-lens-b  ·  **Branch:** lucind/design-lane-status-observability-lens-b

## Goal

Produce `openspec/changes/lane-status-observability/design-lens-b.md`: structured progress
telemetry (`ProgressEvent`/`LaneProgress` field additions, per-executor decoder wiring) AND the
**complete v7 STRICT-table migration DDL** — both `runs.pid` and `lane_progress`'s new usage/tool
columns, as one migration. This lens owns proposal item #5 and writes the FULL v7 migration text
even though lens C consumes the `runs.pid` half of it — the migration is specified exactly once,
here, so lens C can cite it rather than re-derive a second, possibly divergent version.

This is one of three parallel design lenses, sliced by capability. It is feedstock for a synthesis
lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` and the full `specs/` tree for `lane-status-observability` are accepted, frozen, and
already committed on `main`. Lens A and lens C run in parallel against the same frozen inputs and
write to different files, so no lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/proposal.md` exists and is accepted.
- `openspec/changes/lane-status-observability/specs/` exists with all six capability deltas.
- `openspec/changes/lane-status-observability/design-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill.
2. `openspec/changes/lane-status-observability/proposal.md` (full) and
   `specs/lane-progress-telemetry/spec.md` (full) and
   `specs/orphan-lane-reconciliation/spec.md`'s "Requirement: Orphaned lane reconciliation"
   paragraph (the v7-migration sentence only — the sweep itself is lens C's).
3. `internal/ledger/schema.go` (full file — every migration function, the exact
   create-copy-drop-rename pattern, and `migrate`'s version-gated block structure).
4. `internal/executor/executor.go` (full — `ProgressEvent`).
5. `internal/ledger/progress.go` (full — `LaneProgress`, `AppendProgressBatch`).
6. `internal/executor/agy_stream.go`, `claude_stream.go`, `opencode_stream.go`,
   `cursor_agent.go`, `cursor_agent_stream.go` (all five, in full).
7. `internal/serve/model.go:186-193` (the serve-facing `LaneProgress` DTO) and
   `internal/serve/static/app.js:520-548` (`normalizeApproval`/fleet-card data shaping — the exact
   field names and fallback chain the dashboard already reads).

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/design-lens-b.md`:

```markdown
# Design Lens B — Telemetry & Schema v7: Lane Status Observability

## Assumed architecture

<2-4 sentences. Lens A and lens C write this same block independently against
their own slices; the synthesizer compares all three for contradiction.>

## Decision 1 — Telemetry field names and per-decoder mapping

**Choice**: <exact new field names on ProgressEvent/LaneProgress/the serve DTO, and the exact
arithmetic each of agy/claude/opencode uses to populate total_tokens (see `## Context` — none of
the three usage structs already carries a literal total)>
**Alternatives considered**: <other names/formulas>
**Rationale**: <grounded in the existing per-decoder usage structs and the dashboard's existing
field-name fallback chain>
**Terminal consumer**: <file:line>

## Decision 2 — Tool-activity metric: count or rate (resolves the count/rate discrepancy)

**Choice**: <final: what is persisted in the ledger, what is emitted over the wire, and whether
those are the same value — see `## Context` for the exact discrepancy this must resolve>
**Alternatives considered**: <the other reading>
**Rationale**: <grounded in the spec text and the dashboard's existing `tool_rate` field>
**Terminal consumer**: <file:line>

## Decision 3 — v7 migration shape

**Choice**: one `migrateV6ToV7DDL` constant, following the exact create-copy-drop-rename pattern
of `migrateV5ToV6DDL`. State plainly: does `runs` get its own `runs_new` rebuild, does
`lane_progress` get its own `lane_progress_new` rebuild, and what every new column's type, NOT
NULL, and DEFAULT are (including for pre-migration rows this change does not backfill).
**Alternatives considered**: <e.g. a non-STRICT ALTER TABLE ADD COLUMN — say why SQLite's STRICT
tables rule this out, citing the existing comment that already explains it>
**Rationale**: <grounded in schema.go's own established pattern>
**Terminal consumer**: <file:line>

## Interfaces / Contracts

<The COMPLETE `migrateV6ToV7DDL` SQL text, both CREATE TABLE ... _new blocks, both INSERT ...
SELECT statements, both DROP/RENAME pairs, and the index recreation for lane_progress. This is
the one place a full code block is appropriate — cite it, do not merely describe it.>

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|

## Testing Strategy (this slice)

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-b.md` MUST be under 1000 words, **but the `## Interfaces / Contracts` SQL block is
excluded from that count** — DDL is not prose to be trimmed. Tables over prose everywhere else.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens A owns**: `sdd_phase`/`fanout_group`/`skill` frontmatter, `LaneMetadata.PacketPath`,
  packet-body HTTP endpoint, `UpdateLaneMetadata` call sites, DAG-wave scope.
- **Lens C owns**: `RegisterRun`'s PID call site, the startup-sweep-plus-ticker architecture, and
  PID-liveness syscall choice. You own the `runs.pid` COLUMN's exact DDL (type/default); lens C
  owns what writes and reads it operationally.

State explicitly, for lens C's benefit, what a pre-migration `runs` row's `pid` column value is
after the rebuild (there is no backfill) — lens C's sweep must not misread that value as a dead
process.

## Allowed paths

`openspec/changes/lane-status-observability/design-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`. Precedence is **not symmetric**: the skill wins on *what a design document must
contain* (required sections, decision shape, threat-matrix rule); this packet wins on *how this
phase is executed here* (capability split, this lens's slice, word budget, skeleton, done
criteria). The skill's 800-word budget, Engram persistence step, and phase-summary return block
are superseded here; note the conflict in `## Open Questions`, follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file. The claim is
what YOU assert that range shows. This is the synthesizer's worklist, not a certificate.

| citation | claim |
|---|---|
| `internal/executor/executor.go:18-21` | ProgressEvent has only Message and At today |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Decisions 1, 2, and 3 each state a FINAL choice**, including Decision 2's count-vs-rate
  resolution — no "either could work."
- [ ] **`## Interfaces / Contracts` contains the complete, syntactically plausible
  `migrateV6ToV7DDL` SQL text** covering both tables, excluded from the word budget.
- [ ] **Every decision and file-change row names a terminal consumer with a `file:line` citation**
  that points at real code in this worktree.
- [ ] **`design-lens-b.md` exists, is under 1000 words (SQL block excluded), and carries every
  skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The specs do not determine whether a telemetry field is additive.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Deciding this lens's scope would require designing lens A's or lens C's slice.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- `ProgressEvent` (`internal/executor/executor.go:18-21`) is exactly `{ Message string; At
  time.Time }`. `LaneProgress` (`internal/ledger/progress.go:15-20`) is exactly `{ RunID, LaneID
  string; Seq int64; Message string; At time.Time }`. Neither carries any numeric field today.
- **None of the three decoded usage structs already carries a literal total-tokens field except
  agy's.** `agyUsage` (`agy_stream.go:12-18`) has `TotalTokens int` directly from the CLI.
  `claudeUsage` (`claude_stream.go:17-22`) has only `InputTokens`, `OutputTokens`,
  `CacheReadInputTokens`, `CacheCreationInputTokens` — no total field; `claudeStreamRecord.CostUSD`
  (`:35`, JSON key `total_cost_usd`) is the only pre-summed number claude reports.
  `opencodeTokens` (`opencode_stream.go:105-113`) has `Input`, `Output`, `Reasoning`, `Cache.Read`,
  `Cache.Write` — its own doc comment (`:104`) notes a `total` field exists on the wire but is
  deliberately not decoded today. **`cursor-agent` has no usage struct anywhere in
  `cursor_agent.go` or `cursor_agent_stream.go` — confirmed by reading both files in full; only
  tool-call lifecycle is decoded.** This is why the spec requires cursor-agent to emit zeros rather
  than fail decode.
- **A real, unreconciled naming discrepancy exists between two accepted specs and the live
  dashboard, and resolving it is this lens's job — not a re-derivation, a genuine decision:**
  `specs/lane-progress-telemetry/spec.md` requires "generic tool-call counts" (a count).
  `specs/batch-wave-view/spec.md`'s MODIFIED requirement instead says "tool rates." The live
  dashboard (`internal/serve/static/app.js:544`) already reads a fallback chain of `tool_rate`,
  `ToolRate`, `tools_per_minute`, `ToolsPerMinute` — a **rate**, formatted with a `/min` suffix
  (`app.js:544`, `displayMetric(..., value => \`${value}/min\`)`) — not a raw count, and no such
  field is ever populated by the backend today (`app.js:532-538` is the existing "Unavailable"
  fallback path this whole change exists to fix). Decide plainly: persist a raw count, persist a
  computed rate, or persist a count and derive a rate only at the JSON-response layer. Whatever you
  choose, name the exact field(s) on `LaneProgress`, `ProgressEvent`, and the serve DTO
  (`internal/serve/model.go:186-193`).
- `schema.go`'s established create-copy-drop-rename pattern (`migrateV4ToV5DDL:190-219`,
  `migrateV5ToV6DDL:225-308`): SQLite cannot ALTER a STRICT table's column set or CHECK constraint
  in place, so every widening migration creates a `_new` table with the wider shape, copies every
  row verbatim via `INSERT ... SELECT` (ordered), `DROP TABLE` the old one, `ALTER TABLE ... RENAME
  TO`. `runs` (`schema.go:226-234`, no CHECK on `status`, seven columns) and `lane_progress`
  (`schema.go:298-305`, PK `(run_id, lane_id, seq)`, one index at `:306-307`) are the two tables
  this migration widens. `schemaVersion` is currently `6` (`schema.go:10`); `migrate`
  (`schema.go:313-409`) gates each step behind `if currentVersion < N` and is idempotent — your
  new block follows the exact same shape as the `currentVersion < 6` block at `:395-406`.
- `internal/serve/model.go:186-193`'s `LaneProgress` DTO and `internal/serve/handlers.go`'s
  `ServerState.LaneProgress` field (`:55-59`) are what actually reaches `app.js`; both need the
  same new field names your Decision 1/2 settle on.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
accepted); `cursor-agent` usage telemetry stays out of scope and reports zeros, never omitted
fields or a decode error; no historical backfill.

**Out of scope, and including any of it is wrong:** live executor Skill telemetry, PID storage or
liveness (lens C's), packet-body/frontmatter (lens A's), `internal/resolve`/`internal/conflicttriage`
(an unrelated change).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
