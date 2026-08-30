---
id: propose-lane-status-observability-lens-a
executor: agy
routed_by: candidate and approach lens of the three-lens propose fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/propose-lens-a.md"]
---

# Packet propose-lane-status-observability-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-lane-status-observability-lens-a  ·  **Branch:** lucind/propose-lane-status-observability-lens-a

## Goal

Produce `openspec/changes/lane-status-observability/propose-lens-a.md`: the candidate selection, proposed technical approach, changes to system concepts, and architecture rationale for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `lane-status-observability` is initiating. Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns candidate selection and core approach; the other two explore capability specs/scenarios and risk/rollback/test impact.

## Preconditions

- `openspec/changes/lane-status-observability/` exists, and `explore.md` is committed there.
- `openspec/changes/lane-status-observability/propose-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to candidate selection and approach, not to spec delta authoring or test matrices:

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/lane-status-observability/explore.md` — the accepted exploration. Read it in full.
3. `internal/ledger/lanes_meta.go`, `internal/ledger/schema.go`, `internal/packet/packet.go`,
   `internal/run/run.go`, `internal/run/batch.go`, `internal/serve/model.go` — the packages this
   candidate touches.
4. The existing patterns and conventions those packages already follow — how comparable problems
   were already solved in this repository (e.g. `migrateV4ToV5DDL`/`migrateV5ToV6DDL`'s
   create-copy-drop-rename shape for a prior STRICT-table widening).
5. `openspec/changes/archive/` for prior proposals that addressed similar changes.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/propose-lens-a.md`:

```markdown
# Proposal Lens A — Candidate & Approach: Lane Status Observability

## Selected Candidate & Approach

<State the chosen candidate from exploration, summarize the core approach, and explain why this approach solves the problem. Cite file:line for existing code behavior.>

## Conceptual Changes & Architecture Rationale

<Describe additions, modifications, or deprecations to system concepts, interfaces, or architectural patterns — the v7 migration shape, new frontmatter keys, the packet-body-link endpoint, the orphan-sweep subsystem. Cite file:line for existing concepts.>

## Alternatives Considered & Rejected

<What alternative approaches were considered during candidate selection (the two-PR split, metadata-only slice) and why they were rejected — cite the explicit user override.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`propose-lens-a.md` MUST be under 1000 words. Approach descriptions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write delta spec requirements or a rollback plan here. They belong to lenses B and C.

## Allowed paths

`openspec/changes/lane-status-observability/propose-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the
candidate selection, approach, and conceptual change shape of a proposal.
Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the proposal is split
across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
a whole `proposal.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the complete document, persist it to Engram, return the phase summary block.
Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in
`## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every candidate and approach claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-a.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The technical approach cannot be grounded in existing codebase patterns or exploration context.
- Candidate selection contradicts frozen exploration conclusions without justification.
- Satisfying one instruction in this packet would require violating another.

## Context

**Read `openspec/changes/lane-status-observability/explore.md` first.** It is committed in this
worktree, it is the accepted exploration for this change, and it recommends **Candidate 1**: wire
the existing metadata path, add PID-based orphan sweep, and add structured telemetry, all in one
PR under `size:exception`.

**The human already decided the following. They are DECIDED. Do not re-litigate them, do not
present them as alternatives, and do not quietly widen or narrow them:**

1. **Full six-item scope ships as one PR, accepting `size:exception`.** Explore's own
   recommendation was to split into two PRs (metadata/observability first, telemetry+recovery
   second) to isolate the one real schema migration and the new orphan-sweep subsystem for focused
   review. The user explicitly overrode that and chose to keep it as a single PR. Do not
   resurrect the two-PR split as the recommended approach; it belongs only in
   `## Alternatives Considered & Rejected`.
2. **"Skill" observability is static, not live telemetry.** A new `skill:` frontmatter key, set by
   whichever skill/SDD-phase orchestrator authors the packet, records which skill/phase authored
   it. Live runtime "Skill" telemetry from the executor is explicitly out of scope: none of `agy`
   (`gemini-3.7-flash-high`), `cursor-agent` (`cursor-grok-4.6-high`), or `opencode`
   (`openai/gpt-5.6-sol`) are Claude Code, so none have a "Skill" tool concept to observe. Instead,
   expose generic "tool calls made" per lane (Read/Write/Edit-equivalents) as a live proxy, reusing
   the same stream decoders already being extended for token/cost telemetry.
3. **`delivery_strategy` is `exception-ok`** for this change: single PR, size exception accepted
   up front. Do not propose a delivery strategy that contradicts this.
4. **SDD Session Preflight for this session**: `execution_mode=auto`,
   `artifact_store=hybrid` (Engram + OpenSpec), `review_budget_lines=1200` (deliberately exceeded
   per the `size:exception` above).

**Ground truth that must not be re-derived** (from `explore.md`; cite, do not re-investigate):

- `ledger.LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`) already carries `Model, Agent,
  SDDPhase, FanoutGroup, Change, Feature, AllowedPaths, Dependencies, BodyDigest`.
  `UpdateLaneMetadata`/`GetLaneMetadata` (`lanes_meta.go:39,89`) already read/write it —
  `model`/`agent`/`feature` through real schema-v6 columns, `SDDPhase`/`FanoutGroup`/`Change`
  through a `lane_metadata:v1:` JSON audit-event in `events`.
- `serve.Lane` (`internal/serve/model.go:163-184`) and `app.js:532-538` already consume the
  metadata end to end. **`UpdateLaneMetadata` has zero production callers** — only test files call
  it. `internal/run/run.go:334` (`Execute`) and `internal/run/batch.go:184`
  (`ensureLaneFailed`), the two real dispatch sites, call `RegisterLane` and never follow up.
  "Unavailable" is an honest render of empty data, not a display bug.
- `packet.Parse` (`internal/packet/packet.go:78-167`) recognizes only `id, executor, routed_by,
  model, agent, read_only, feature, parent_ref, base_sha, expected_parent_sha, legacy_main,
  allowed_paths` — no `sdd_phase`, `fanout_group`, or `skill` key exists anywhere yet. `Packet`
  also has no `Path` field; only `cli.go:160-166`'s index-aligned `packetFlags[i]`/`ps[i]` slices
  know the on-disk path dispatched to a lane.
- `agy_stream.go:12-18` (`agyUsage`), `claude_stream.go` (`claudeUsage`+`costUSD`), and
  `opencode_stream.go:100-113` (`opencodeTokens`+`Cost`) already parse real usage numbers, then
  discard them into prose `ProgressEvent.Message` strings (`executor.go:17-21` has no numeric
  fields). Only `cursor_agent.go` has no usage struct at all.
- The SSE hub reads from durable ledger cursors (`ledger.LaneProgress`,
  `internal/ledger/progress.go:15-20`), backed by a STRICT `lane_progress` table
  (`schema.go:298-307`) with no usage/tool-call columns.
- No PID or heartbeat is stored for a run or lane anywhere (`ledger.Run`, `runs.go:16-24`, has no
  PID field), and no reconciliation/orphan-sweep code exists in the codebase. A lane whose driving
  process is abandoned mid-flight stays `running` in SQLite forever.
- `runs` and `lane_progress` are both STRICT tables (`schema.go:298-307`), and SQLite cannot widen
  a STRICT table's shape in place (`schema.go:183-184,221-224`, the comment on
  `migrateV5ToV6DDL`). A v7 migration is required for both the usage/tool-call columns and the
  `pid` column, following the exact create-copy-drop-rename shape `migrateV4ToV5DDL`
  (`schema.go:182-219`) and `migrateV5ToV6DDL` (`schema.go:221-308`) already used — create the
  `_new` table with the wider shape, `INSERT ... SELECT` every row verbatim, `DROP TABLE`, `ALTER
  TABLE ... RENAME TO`. The two new column sets can share one migration.

**Open questions that MUST stay open in the proposal** (from `explore.md`'s own open-questions
list; answering any of them here without the fixture/implementation-time evidence is a guess this
proposal should not make):

- Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs
  `generated_by`.
- Packet path persistence: a new `LaneMetadata.PacketPath` field (audit-event JSON, no migration)
  vs. a real `lanes` column (migration, but queryable/indexable).
- Ticker interval for the periodic orphan sweep.
- PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and whether cross-platform
  portability beyond Linux is in scope.
- Whether `internal/dag/parse.go`'s `Node`/`internal/dag/emit.go`'s `EmitPacketContent` (the
  DAG-wave path) get the same new fields in this change or a follow-up.

**Out of scope, and a proposal that includes any of it is wrong:** live runtime "Skill" telemetry
from any executor; backfilling historical ledger rows for already-run lanes; changing
`internal/dag`'s DAG-wave packet emission (unless resolving the open question above explicitly
brings it in scope); cross-platform PID-liveness beyond what the open question above settles.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
