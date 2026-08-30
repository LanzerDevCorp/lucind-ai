---
id: tasks-lane-status-observability-lens-a
executor: agy
routed_by: metadata/frontmatter/packet-body lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/tasks-lens-a.md"]
---

# Packet tasks-lane-status-observability-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-lane-status-observability-lens-a  ·  **Branch:** lucind/tasks-lane-status-observability-lens-a

## Goal

Produce `openspec/changes/lane-status-observability/tasks-lens-a.md`: the implementation task
checklist for lane dispatch metadata persistence, extended packet frontmatter, and the dispatched
packet-body HTTP endpoint — design's lens-A capability slice (Decisions 1-4 in `design.md`).

This is one of three parallel tasks lenses, sliced by **capability** (matching how `propose`,
`specs`, and `design` were already sliced for this change), not by the generic
decomposition/partition/proof three-way split this repository's tasks fan-out normally uses. Your
slice is self-contained: write its own ordered checklist, dependency order, requirement
traceability, RED tests, and acceptance evidence together. It is feedstock for a synthesis lane,
not the final `tasks.md`. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and the full `specs/` tree for `lane-status-observability` are accepted, frozen, and
committed on this branch. Lens B and lens C run in parallel against the same frozen inputs and
write to different files, so no lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/design.md` and `specs/` exist.
- `openspec/changes/lane-status-observability/tasks-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/lane-status-observability/design.md` — Decisions 1-4 (frontmatter keys,
   packet-path persistence, packet-body endpoint, DAG-wave scope), the File Changes table rows for
   `cmd/lucind-ai/cli.go` (Packet.Path only), `internal/packet/packet.go`,
   `internal/ledger/lanes_meta.go`, `internal/run/run.go`, `internal/run/batch.go`, and the
   `serve`-side rows for `Skill`/`PacketPath` (not the telemetry/tool_rate rows — lens B's).
3. `openspec/changes/lane-status-observability/specs/read-only-packet-schema/spec.md`,
   `specs/dispatched-packet-body/spec.md`, and `specs/lane-execution/spec.md`.
4. `internal/ledger/lanes_meta_test.go`, `internal/packet/packet_test.go`, `internal/run/run_test.go`
   — the real test files and conventions your RED tasks must extend.
5. `internal/serve/handlers.go` (route registration shape) and `internal/serve/model.go` (`Lane`,
   `laneDTO`) — for the packet-body GET task only; do not touch the telemetry DTO fields.

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line`
citation or is explicitly marked "new file".

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/tasks-lens-a.md`:

```markdown
# Tasks Lens A — Metadata, Frontmatter & Packet Body: Lane Status Observability

## Assumed decomposition

<2-4 sentences: how many phases in this slice, what each delivers, the critical
path. Lens B and lens C write this same block for their own slices; the
synthesizer compares all three for contradiction.>

## Phase A1: <name>

- [ ] A1.1 <Concrete action — file, change>
- [ ] A1.2 <Concrete action>

## Phase A2: <name>

- [ ] A2.1 <Concrete action>

## Dependency Order (this slice)

| Task | Depends on | Why |
|---|---|---|

## Suggested Work Unit

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|

<One row. This slice ships as part of the single accepted PR — name the unit
anyway so the synthesizer can order it against lens B's and lens C's units.>

## RED Tests from the Threat Matrix (this slice)

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|

<Only rows from design.md's threat matrix that this slice implements. If none
apply to this slice, write "None — see lens C".>

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|

## Requirement Traceability

| Requirement | Tasks |
|---|---|

<Every requirement in the three specs this slice covers maps to at least one
task; every task maps back to at least one requirement.>

## Open Questions

- [ ] <unresolved question, or "None">
```

Every task MUST be specific, actionable, verifiable, and small (one file or one logical unit).
Strict TDD is active for this project: a RED test task precedes its GREEN production task for every
behavior change in this slice, not only threat-matrix rows.

## Size budget

`tasks-lens-a.md` MUST be under 1000 words (Citation Manifest excluded).

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens B owns**: `ProgressEvent`/`LaneProgress` numeric fields, per-executor decoder wiring, the
  full v7 STRICT-table migration DDL (`runs.pid` and `lane_progress` usage columns), and the
  `tool_rate` derivation in `GetLaneProgress`.
- **Lens C owns**: `Run.PID` capture on `RegisterRun`, the `internal/serve/sweeper.go` sweep/ticker,
  PID-liveness syscall, and the process-integration threat-matrix row.
- Do not write the overall Review Workload Forecast — the synthesizer assembles it from all three
  slices' acceptance-evidence tables.

`internal/serve/model.go` and `internal/serve/static/app.js` are touched by both this lens (Skill,
PacketPath, packet link) and lens B (telemetry fields, tool_rate). Scope your tasks against those
two files to the `Skill`/`PacketPath` surface only; note the shared-file overlap in
`## Open Questions` so the synthesizer can decide whether apply needs these sequenced.

## Allowed paths

`openspec/changes/lane-status-observability/tasks-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence is **not symmetric**. The skill is authority on *what a task file must contain*: the
specific/actionable/verifiable/small rule and the RED-before-GREEN rule for applicable
threat-matrix rows. This packet is authority on *how this phase is executed here*: the
capability-sliced three-lane split, this lens's slice, its word budget, output path and skeleton,
out-of-scope list, and done criteria. Note any conflict in `## Open Questions` and follow this
packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` cited, one row per unique
citation, grouped by file, files alphabetical, line numbers ascending. The claim column is what YOU
assert that range shows. This is a worklist for the synthesizer, not a certificate.

| citation | claim |
|---|---|
| `internal/packet/packet.go:94-138` | Parse's frontmatter switch has no sdd_phase/fanout_group/skill case today |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice — before committing, and again after
committing and writing `.lucind/result.json`:

```
./lucind-lane-check.sh --file openspec/changes/lane-status-observability/tasks-lens-a.md \
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
- [ ] **A RED test task precedes every GREEN production task** for a behavior change in this slice.
- [ ] **Every requirement in the three owned specs appears in the traceability table with at least one task.**
- [ ] **The shared-file overlap on `internal/serve/model.go` and `app.js` with lens B is named in `## Open Questions`.**
- [ ] **`tasks-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty; check `git log -1 --format=%B` for an unwanted `Co-authored-by:` trailer and strip it if present).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- A requirement in the three owned specs cannot be decomposed into tasks because `design.md` does
  not say how it is built.
- `design.md`'s Decisions 1-4 and the specs disagree about what this slice does.
- Two tasks in this slice are mutually circular.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth, already resolved in `design.md` — cite it, do not re-derive it:**

- Frontmatter keys are final: `sdd_phase`, `fanout_group`, `skill` (Decision 1, `design.md:17-22`).
  `packet.Parse`'s switch (`internal/packet/packet.go:94-138`) needs three new cases; `Packet`
  (`packet.go:33-75`) needs `SDDPhase`, `FanoutGroup`, `Skill`, `Path` fields. `Path` is set by the
  CLI caller from the `--packet` argument, not parsed from frontmatter.
- Packet path lives in `LaneMetadata.PacketPath string \`json:"packet_path"\`` inside the existing
  `lane_metadata:v1:` JSON snapshot, not a new `lanes` column (Decision 2, `design.md:24-29`).
  `LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`) needs `Skill` and `PacketPath` added the
  same way.
- `UpdateLaneMetadata` must be called after `RegisterLane` in two places: `internal/run/run.go`
  (`Execute`) and `internal/run/batch.go` (`ensureLaneFailed`) — `design.md:94,97-98`.
- Packet-body endpoint: `GET /api/packets/{runID}/{laneID}` → 200 `text/markdown; charset=utf-8` on
  success, 404 via `writeJSONError` when the lane is unknown, `PacketPath` is empty, or the file is
  missing/unreadable — never abort the serve process (Decision 3, `design.md:31-36`). Same
  two-segment parse shape as `/approvals/{runID}/{laneID}` (`internal/serve/handlers.go:316-350`).
- DAG-wave `Node`/`EmitPacketContent` stay unchanged this change; omitted frontmatter keys default
  to empty strings, never a parse error (Decision 4, `design.md:38-43`).
- `serve.Lane`/`laneDTO` (`internal/serve/model.go:322-333`) needs `Skill`, `PacketPath` fields
  wired through; `internal/serve/static/app.js:534-536` needs `skill` rendered beside `sdd_phase`/
  `fanout_group`/`feature`, plus the packet link.
- Threat matrix (`design.md:179-188`): every row is `N/A` except "Process integration," which is
  entirely lens C's. This slice has no applicable threat-matrix row of its own.

**User-approved decisions, already final — do not re-litigate:** full six-item scope ships as one
PR (`size:exception`); static `skill:` frontmatter only, never live runtime telemetry;
`delivery_strategy` is `exception-ok`; Strict TDD is active for this project.

**Out of scope:** live executor Skill telemetry, historical-row backfill, `internal/resolve`/
`internal/conflicttriage` (an unrelated change), anything lens B or lens C owns (see above).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
