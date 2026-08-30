---
id: propose-lane-status-observability-lens-b
executor: agy
routed_by: capability impact and delta specs lens of the three-lens propose fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/propose-lens-b.md"]
---

# Packet propose-lane-status-observability-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-lane-status-observability-lens-b  ·  **Branch:** lucind/propose-lane-status-observability-lens-b

## Goal

Produce `openspec/changes/lane-status-observability/propose-lens-b.md`: user capability impact, modified/added capabilities, and delta specification requirements and scenarios for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `lane-status-observability` is initiating. Lens A and lens C run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/` exists, and `explore.md` is committed there.
- `openspec/changes/lane-status-observability/propose-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to capability impact and delta specs:

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/lane-status-observability/explore.md` — the accepted exploration, especially
   its `## User and capability impact` and `## Scenarios` sections. Read it in full.
3. Existing delta and base specifications in `openspec/specs/`.
4. `internal/serve/model.go`, `internal/packet/packet.go`, `internal/ledger/lanes_meta.go` — the
   interfaces, packet frontmatter schema, and ledger fields this change extends.
5. `openspec/changes/archive/` for precedent delta spec structure in this repository.

Never guess at signatures or spec shapes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/propose-lens-b.md`:

```markdown
# Proposal Lens B — Capability Impact & Specs: Lane Status Observability

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|

## Delta Specifications

### Requirement: <Requirement Name>

<Requirement text using RFC 2119 keywords: MUST, SHALL, SHOULD, MAY.>

#### Scenario: <Scenario Name>

- GIVEN <initial condition>
- WHEN <trigger event>
- THEN <observable outcome>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`propose-lens-b.md` MUST be under 1000 words. Tables and structured delta specs over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write architecture rationale or rollback mechanisms here. They belong to lenses A and C.

## Allowed paths

`openspec/changes/lane-status-observability/propose-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the
capability impact table, and delta spec formatting conventions.
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

- [ ] **Every capability impact and delta spec requirement carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-b.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Delta requirements conflict with existing base specs without explicit migration path.
- Capability impacts cannot be determined from packet context or code inspection.
- Satisfying one instruction in this packet would require violating another.

## Context

**Read `openspec/changes/lane-status-observability/explore.md` first.** It is committed in this
worktree, it is the accepted exploration for this change, and it recommends **Candidate 1**: wire
the existing metadata path, add PID-based orphan sweep, and add structured telemetry, all in one
PR under `size:exception`.

**The human already decided the following. They are DECIDED. Do not re-litigate them, do not
present them as alternatives, and do not quietly widen or narrow them:**

1. **Full six-item scope ships as one PR, accepting `size:exception`.** Explore's own
   recommendation was to split into two PRs; the user explicitly overrode that and chose to keep
   it as a single PR. All six capabilities below belong in one delta spec set, not two.
2. **"Skill" observability is static, not live telemetry.** A new `skill:` frontmatter key, set by
   whichever skill/SDD-phase orchestrator authors the packet, records which skill/phase authored
   it. Live runtime "Skill" telemetry from the executor is explicitly out of scope: none of `agy`,
   `cursor-agent`, or `opencode` are Claude Code, so none have a "Skill" tool concept to observe.
   Instead, expose generic "tool calls made" per lane (Read/Write/Edit-equivalents) as a live
   proxy, reusing the same stream decoders already being extended for token/cost telemetry.
3. **`delivery_strategy` is `exception-ok`** for this change: single PR, size exception accepted
   up front.
4. **SDD Session Preflight for this session**: `execution_mode=auto`,
   `artifact_store=hybrid` (Engram + OpenSpec), `review_budget_lines=1200` (deliberately exceeded).

**Ground truth that must not be re-derived** (from `explore.md`; cite, do not re-investigate):

- `ledger.LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`) already carries `Model, Agent,
  SDDPhase, FanoutGroup, Change, Feature, AllowedPaths, Dependencies, BodyDigest`.
  `serve.Lane` (`internal/serve/model.go:163-184`) and `app.js:532-538` already consume it end to
  end. **`UpdateLaneMetadata` has zero production callers** — the two real dispatch sites
  (`internal/run/run.go:334` `Execute`, `internal/run/batch.go:184` `ensureLaneFailed`) call
  `RegisterLane` and never follow up, so "Unavailable" is an honest render of empty data.
- `packet.Parse` (`internal/packet/packet.go:78-167`) recognizes only `id, executor, routed_by,
  model, agent, read_only, feature, parent_ref, base_sha, expected_parent_sha, legacy_main,
  allowed_paths` — no `sdd_phase`, `fanout_group`, or `skill` key exists yet. `Packet` has no
  `Path` field; only `cli.go:160-166`'s index-aligned `packetFlags[i]`/`ps[i]` slices know the
  on-disk path dispatched to a lane — this is the seam a packet-body-link endpoint would use.
- `agy_stream.go:12-18` (`agyUsage`), `claude_stream.go` (`claudeUsage`+`costUSD`), and
  `opencode_stream.go:100-113` (`opencodeTokens`+`Cost`) already parse real usage numbers, then
  discard them into prose `ProgressEvent.Message` strings (`executor.go:17-21` has no numeric
  fields). Only `cursor_agent.go` has no usage struct.
- The SSE hub reads from durable ledger cursors (`ledger.LaneProgress`,
  `internal/ledger/progress.go:15-20`), backed by a STRICT `lane_progress` table
  (`schema.go:298-307`) with no usage/tool-call columns — a v7 migration is required.
- No PID or heartbeat is stored for a run or lane anywhere (`ledger.Run`, `runs.go:16-24`), and no
  reconciliation/orphan-sweep code exists. A lane whose driving process is abandoned mid-flight
  stays `running` in SQLite for 25+ hours in the observed case.
- The five capability-impact bullets already accepted in exploration (map 1:1 onto scenarios 1-5
  and belong in your capability table): real `MODEL`/`SDD PHASE`/`FANOUT GROUP` values reaching
  the dashboard (no historical backfill — existing rows for already-run lanes stay empty); a link
  per lane to the exact packet body dispatched to it (new `internal/serve` endpoint); which
  skill/SDD-phase authored a lane's packet (new static `skill:` frontmatter key); token/cost usage
  and generic tool-call activity for `agy`/`claude`/`opencode` lanes only (not `cursor-agent`,
  which has no usage data to surface); lanes no longer stuck "Running" for hours after their
  driving process died (periodic + startup orphan sweep to `failed` with a clear reason).

**Open questions that MUST stay open in the proposal** (from `explore.md`; do not resolve them
here):

- Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs
  `generated_by`.
- Packet path persistence: a new `LaneMetadata.PacketPath` field (audit-event JSON, no migration)
  vs. a real `lanes` column (migration, but queryable/indexable).
- Ticker interval for the periodic orphan sweep.
- PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and cross-platform scope.
- Whether `internal/dag/parse.go`'s `Node`/`internal/dag/emit.go`'s `EmitPacketContent` (the
  DAG-wave path) get the same new fields in this change or a follow-up.

**Out of scope, and a proposal that includes any of it is wrong:** live runtime "Skill" telemetry
from any executor; backfilling historical ledger rows for already-run lanes; `cursor-agent` usage
telemetry (it has none to surface); changing `internal/dag`'s DAG-wave packet emission unless the
open question above explicitly brings it in scope.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
