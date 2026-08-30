---
id: tasks-lane-status-observability-lens-b
executor: agy
routed_by: structured telemetry & schema v7 migration lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/tasks-lens-b.md"]
---

# Packet tasks-lane-status-observability-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-lane-status-observability-lens-b  ·  **Branch:** lucind/tasks-lane-status-observability-lens-b

## Goal

Produce `openspec/changes/lane-status-observability/tasks-lens-b.md`: the implementation task
checklist for structured progress telemetry (`ProgressEvent`/`LaneProgress` numeric fields,
per-executor decoder wiring) and the schema v7 migration — design's lens-B capability slice.

This is one of three parallel tasks lenses, sliced by **capability** (matching how `propose`,
`specs`, and `design` were already sliced for this change), not the generic
decomposition/partition/proof three-way split. Your slice is self-contained: write its own ordered
checklist, dependency order, requirement traceability, RED tests, and acceptance evidence together.
It is feedstock for a synthesis lane, not the final `tasks.md`. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and the full `specs/` tree for `lane-status-observability` are accepted, frozen, and
committed on this branch. Lens A and lens C run in parallel against the same frozen inputs and
write to different files, so no lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/design.md` and `specs/` exist.
- `openspec/changes/lane-status-observability/tasks-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill.
2. `openspec/changes/lane-status-observability/design.md` — the "Aggregate telemetry fields and
   per-decoder mapping" and "Persist `tool_calls`, derive `tool_rate`" decisions, the complete
   `migrateV6ToV7DDL` in `## Interfaces / Contracts`, and the File Changes rows for
   `internal/executor/executor.go`, the four `*_stream.go`/`cursor_agent.go` decoders,
   `internal/ledger/schema.go`, `internal/ledger/progress.go`, and the `serve.LaneProgress`
   telemetry/`tool_rate` fields (NOT the `Skill`/`PacketPath` rows — lens A's).
3. `openspec/changes/lane-status-observability/specs/lane-progress-telemetry/spec.md` and
   `specs/batch-wave-view/spec.md` (the `tool_rate` scenarios only).
4. `internal/ledger/schema_test.go`, `internal/ledger/progress_test.go`,
   `internal/executor/agy_stream_test.go`, `claude_stream_test.go` (usage fixtures),
   `opencode_stream_test.go`, `cursor_agent_stream_test.go` — the real test files and fixture
   conventions your RED tasks must extend.

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line`
citation or is explicitly marked "new file". Do not re-derive the v7 DDL — copy it verbatim from
`design.md` into your production task's description.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/tasks-lens-b.md`:

```markdown
# Tasks Lens B — Structured Telemetry & Schema v7: Lane Status Observability

## Assumed decomposition

<2-4 sentences: how many phases in this slice, what each delivers, the critical
path — schema migration must land before any decoder or query task depends on
the new columns. Lens A and lens C write this same block for their own slices.>

## Phase B1: <name>

- [ ] B1.1 <Concrete action — file, change>

## Phase B2: <name>

- [ ] B2.1 <Concrete action>

## Dependency Order (this slice)

| Task | Depends on | Why |
|---|---|---|

<Name the schema-v7-before-decoders and schema-v7-before-progress-SQL constraints
explicitly — they are real, not stylistic.>

## Suggested Work Unit

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|

<One row. This slice ships as part of the single accepted PR — name the unit
anyway so the synthesizer can order it against lens A's and lens C's units.>

## RED Tests from the Threat Matrix (this slice)

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|

<This slice's threat-matrix applicability is schema/decoder correctness, not the
"Process integration" row (lens C's). If `design.md`'s matrix has no row scoped to
this slice, write "None — see lens C" and instead name the STRICT-table
create-copy-drop-rename correctness as an explicit RED/GREEN pair in your phases.>

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|

## Requirement Traceability

| Requirement | Tasks |
|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

Every task MUST be specific, actionable, verifiable, and small. Strict TDD is active for this
project: a RED test task precedes its GREEN production task for every behavior change in this
slice, including the v7 migration's own row-preservation/STRICT/reopen-idempotency behavior.

## Size budget

`tasks-lens-b.md` MUST be under 1000 words (Citation Manifest excluded). The verbatim v7 DDL you
paste into a task description is excluded from this count, same as it was excluded from
`design.md`'s own budget — do not trim the SQL to fit.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: `internal/packet/packet.go` frontmatter fields, `LaneMetadata.Skill`/
  `PacketPath`, `UpdateLaneMetadata` call sites, and the packet-body HTTP endpoint.
- **Lens C owns**: `Run.PID` capture on `RegisterRun` and its call site in `cmd/lucind-ai/cli.go`,
  `internal/serve/sweeper.go`, PID-liveness, and the process-integration threat-matrix row.

`internal/ledger/runs.go` (`Run.PID` field, insert/select/scan) is lens C's, but it depends on the
`runs.pid` column your v7 DDL creates. Name that cross-lens dependency explicitly in
`## Open Questions` so the synthesizer sequences it (schema v7 before lens C's `runs.go` changes).
`internal/serve/model.go` and `internal/serve/static/app.js` are touched by both this lens
(telemetry fields, `tool_rate`) and lens A (`Skill`, `PacketPath`, packet link) — scope your tasks
to the telemetry surface only and note the shared-file overlap in `## Open Questions`.

## Allowed paths

`openspec/changes/lane-status-observability/tasks-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence is **not symmetric**. The skill is authority on *what a task file must contain*; this
packet is authority on *how this phase is executed here* — the capability-sliced split, this
lens's slice, its word budget, output path and skeleton, out-of-scope list, and done criteria. Note
any conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` cited, one row per unique
citation, grouped by file, files alphabetical, line numbers ascending. The claim column is what YOU
assert that range shows. This is a worklist for the synthesizer, not a certificate.

| citation | claim |
|---|---|
| `internal/ledger/schema.go:10` | schemaVersion constant is currently 6 |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice — before committing, and again after
committing and writing `.lucind/result.json`:

```
./lucind-lane-check.sh --file openspec/changes/lane-status-observability/tasks-lens-b.md \
  --budget 1000 --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Dependency Order (this slice)" \
  --require-section "Suggested Work Unit" --require-section "RED Tests from the Threat Matrix (this slice)" \
  --require-section "Acceptance Evidence" --require-section "Requirement Traceability" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` instead of narrating the same
facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every task names a concrete path**, and every "modify" task's path resolves in this worktree.
- [ ] **A RED test task precedes every GREEN production task**, including for the v7 migration itself.
- [ ] **The v7 DDL task cites `design.md`'s `## Interfaces / Contracts` verbatim, not a re-derived version.**
- [ ] **The cross-lens dependency on lens C's `internal/ledger/runs.go` and the shared-file overlap with lens A are named in `## Open Questions`.**
- [ ] **Every requirement in the two owned specs' `tool_rate`/telemetry scenarios appears in the traceability table.**
- [ ] **`tasks-lens-b.md` exists, is under 1000 words excluding the Citation Manifest and the verbatim DDL, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty; check `git log -1 --format=%B` for an unwanted `Co-authored-by:` trailer and strip it if present).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The v7 DDL in `design.md` is missing or incomplete — do not invent or repair it yourself; block
  instead.
- A requirement in the two owned specs cannot be decomposed into tasks because `design.md` does not
  say how it is built.
- Two tasks in this slice are mutually circular.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth, already resolved in `design.md` — cite it, do not re-derive it:**

- New `ProgressEvent` fields: `TotalTokens int64`, `CostUSD float64`, `ToolCalls int64`
  (`internal/executor/executor.go:18-21`), mirrored on `ledger.LaneProgress`
  (`internal/ledger/progress.go:15-20`) and `serve.LaneProgress` (`internal/serve/model.go:186-193`;
  JSON `total_tokens`, `cost_usd`, `tool_calls`).
- Per-decoder mapping: `agy` — `TotalTokens` as-is, `CostUSD=0`
  (`internal/executor/agy_stream.go:12-18`); `claude` — sum of input+output+cache-read+
  cache-creation tokens, `CostUSD` from the record (`internal/executor/claude_stream.go:18-21,35`);
  `opencode` — sum of input+output+reasoning+cache read/write, `CostUSD` from the part
  (`internal/executor/opencode_stream.go:106-112,123`); `cursor-agent` emits `0`/`0.0`
  (`internal/executor/cursor_agent.go:239-270`). All four decoders increment tool-call counts on
  tool events.
- Store cumulative `tool_calls INTEGER` on `lane_progress`; `GetLaneProgress`
  (`internal/serve/model.go:336-346`) derives `tool_rate` (tools/min) as
  `float64(tool_calls) / max(elapsed_minutes, 1.0/60.0)` from lane `StartedAt` to the progress `At`
  (1s floor) — never persisted as a rate in SQLite.
- v7 is one `migrateV6ToV7DDL` constant: create-copy-drop-rename of both `runs` (`+pid INTEGER NOT
  NULL DEFAULT 0 CHECK (pid >= 0)`) and `lane_progress` (`+total_tokens`, `+cost_usd`,
  `+tool_calls`, all `NOT NULL DEFAULT 0` with `CHECK >= 0`). Recreate
  `idx_lane_progress_run_lane_seq`. Pre-migration rows get `pid=0` and zeroed usage, no backfill.
  Bump `schemaVersion` 6→7 in the `migrate` version loop (`internal/ledger/schema.go:313-409`). The
  full DDL text is in `design.md`'s `## Interfaces / Contracts` — copy it verbatim.
- Invariant: `cursor-agent` numeric token/cost fields are always zero, never omitted.

**User-approved decisions, already final — do not re-litigate:** full six-item scope ships as one
PR (`size:exception`); `delivery_strategy` is `exception-ok`; no historical-row backfill anywhere;
Strict TDD is active for this project.

**Out of scope:** live executor Skill telemetry (unrelated to this slice's numeric usage fields),
`internal/resolve`/`internal/conflicttriage` (an unrelated change), anything lens A or lens C owns
(see above).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
