---
id: propose-lane-status-observability-lens-c
executor: agy
routed_by: risks, rollback, and test impact lens of the three-lens propose fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/propose-lens-c.md"]
---

# Packet propose-lane-status-observability-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-lane-status-observability-lens-c  ·  **Branch:** lucind/propose-lane-status-observability-lens-c

## Goal

Produce `openspec/changes/lane-status-observability/propose-lens-c.md`: risk assessment, rollback strategy, additivity, and test impact for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `lane-status-observability` is initiating. Lens A and lens B run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/` exists, and `explore.md` is committed there.
- `openspec/changes/lane-status-observability/propose-lens-c.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to risks, rollback, and test impact:

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/lane-status-observability/explore.md` — the accepted exploration. Read it in
   full.
3. `internal/ledger/schema.go` (especially `migrateV4ToV5DDL` and `migrateV5ToV6DDL`, and the
   comment on why SQLite STRICT tables cannot be widened in place), existing test suites for
   `internal/ledger`, `internal/run`, `internal/serve`, and `internal/executor`.
4. Wire and persisted formats: `ledger.LaneMetadata`, `ledger.LaneProgress`, `ledger.Run`, result
   envelopes.
5. `openspec/changes/archive/` for rollback plans and test strategy precedents.

Never guess at seams or failure modes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/propose-lens-c.md`:

```markdown
# Proposal Lens C — Risks, Rollback & Test Impact: Lane Status Observability

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|

## Rollback & Additivity

**Rollback Plan**: <exact mechanism for reversal, git revert vs schema rollback>
**Additivity**: <state explicitly whether formats, schemas, or ledgers change additively or destructively, citing file:line>

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|

## Out of Scope

<Work explicitly excluded from this proposal.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`propose-lens-c.md` MUST be under 1000 words. Tables over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.

Rollback and test impact are yours. Conceptual design and delta spec requirements belong to lenses A and B.

## Allowed paths

`openspec/changes/lane-status-observability/propose-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the
risk assessment, rollback plan, and test impact shape of a proposal.
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

- [ ] **Every risk, test seam, and rollback claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-c.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Whether a schema or format change is additive cannot be determined, making rollback a guess.
- A critical failure mode has no identifiable mitigation or test seam.
- Satisfying one instruction in this packet would require violating another.

## Context

**Read `openspec/changes/lane-status-observability/explore.md` first.** It is committed in this
worktree, it is the accepted exploration for this change, and it recommends **Candidate 1**: wire
the existing metadata path, add PID-based orphan sweep, and add structured telemetry, all in one
PR under `size:exception`.

**The human already decided the following. They are DECIDED. Do not re-litigate them, do not
present them as alternatives, and do not quietly widen or narrow them:**

1. **Full six-item scope ships as one PR, accepting `size:exception`.** Explore's own
   recommendation was to split into two PRs specifically to isolate the one real schema migration
   and the new orphan-sweep subsystem for focused review. The user explicitly overrode that. Your
   risk table MUST include the review-budget risk this creates and name it explicitly as an
   accepted, not mitigated, risk — the mitigation is the exception itself, not a technical control.
2. **"Skill" observability is static, not live telemetry.** A new `skill:` frontmatter key is set
   by the authoring orchestrator. Live runtime "Skill" telemetry from any executor is out of
   scope — none of `agy`, `cursor-agent`, or `opencode` are Claude Code. This has no rollback
   implications since nothing runtime-observed is being added; note it only if it affects test
   impact.
3. **`delivery_strategy` is `exception-ok`** for this change: single PR, size exception accepted
   up front.
4. **SDD Session Preflight for this session**: `execution_mode=auto`,
   `artifact_store=hybrid` (Engram + OpenSpec), `review_budget_lines=1200` (deliberately exceeded).

**Ground truth that must not be re-derived** (from `explore.md`; cite, do not re-investigate):

- `runs` and `lane_progress` are both STRICT tables (`internal/ledger/schema.go:298-307`,
  `runs.go:16-24`), and SQLite cannot widen a STRICT table's shape in place
  (`schema.go:183-184,221-224`). A v7 migration is required for the new usage/tool-call columns on
  `lane_progress` and the new `pid` column on `runs`, following the exact create-copy-drop-rename
  shape already used twice: `migrateV4ToV5DDL` (`schema.go:182-219`, widening `lanes`'s executor
  CHECK constraint) and `migrateV5ToV6DDL` (`schema.go:221-308`, adding `runs`, widening `lanes`
  and `events`, adding `lane_progress`) — create a `_new` table with the wider shape, `INSERT ...
  SELECT` every row verbatim preserving PRIMARY KEY identity, `DROP TABLE`, `ALTER TABLE ... RENAME
  TO`. The two new column sets (usage/tool-call on `lane_progress`, `pid` on `runs`) can share one
  migration. `migrate` (`schema.go:313`) applies migrations inside one transaction and is
  idempotent — a safe re-run against an already-migrated database.
- `UpdateLaneMetadata` (`internal/ledger/lanes_meta.go:39`) currently has **zero production
  callers**; wiring it into `internal/run/run.go:334` (`Execute`) and
  `internal/run/batch.go:184` (`ensureLaneFailed`) is additive — it starts populating columns and
  an audit-event JSON blob that already exist and are already consumed
  (`internal/serve/model.go:163-184`, `app.js:532-538`), with no schema change of its own.
- New frontmatter keys (`sdd_phase`/`fanout_group`/`skill` or their alternates) are additive to
  `packet.Parse` (`internal/packet/packet.go:78-167`): existing packets without these keys must
  continue to parse unchanged — this is a required backward-compatibility test, not an assumption.
- `agy_stream.go:12-18`, `claude_stream.go`, and `opencode_stream.go:100-113` already parse real
  usage numbers today, currently discarded into prose (`executor.go:17-21` has no numeric fields);
  adding numeric fields to `ProgressEvent`/`LaneProgress` and populating them is additive to those
  three decoders. `cursor_agent.go` has no usage struct and must leave the new fields empty rather
  than fabricate values.
- No PID or heartbeat is stored for a run or lane anywhere today (`ledger.Run`, `runs.go:16-24`),
  and no reconciliation/orphan-sweep code exists in the codebase — this is wholly new subsystem
  surface, not a modification of existing sweep logic, so failure-mode analysis has no existing
  precedent in this repository to lean on. `serve` and `lucind-ai run` are separate processes
  against the same ledger file; the sweep checks PID liveness, not process identity — a reused PID
  (the original process died, an unrelated process was later assigned the same PID) is a real
  false-negative risk the mitigation table must name, since no start-time or process-identity
  disambiguation is mentioned anywhere in `explore.md`.

**Open questions that MUST stay open in the proposal** (from `explore.md`; several bear directly
on your risk/test analysis — do not resolve them, but do name the risk of leaving them open):

- Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs
  `generated_by`.
- Packet path persistence: a new `LaneMetadata.PacketPath` field (audit-event JSON, no migration)
  vs. a real `lanes` column (migration, but queryable/indexable) — this changes your migration
  risk surface depending on which is chosen.
- Ticker interval for the periodic orphan sweep — too aggressive risks false-positive sweeps of
  slow-but-alive lanes; too sparse leaves lanes stuck longer than necessary.
- PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and cross-platform scope —
  bears directly on test impact (what can be tested on Linux CI vs elsewhere).
- Whether `internal/dag/parse.go`'s `Node`/`internal/dag/emit.go`'s `EmitPacketContent` get the
  same new fields in this change or a follow-up.

**Out of scope, and a proposal that includes any of it is wrong:** live runtime "Skill" telemetry
from any executor; backfilling historical ledger rows for already-run lanes; a general-purpose
process-supervision/restart mechanism (the sweep only marks lanes `failed`, it does not restart
anything); cross-platform PID-liveness beyond what the open question above settles.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
