---
id: design-lane-status-observability-lens-a
executor: agy
routed_by: metadata/frontmatter/packet-body lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/design-lens-a.md"]
---

# Packet design-lane-status-observability-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-lane-status-observability-lens-a  ·  **Branch:** lucind/design-lane-status-observability-lens-a

## Goal

Produce `openspec/changes/lane-status-observability/design-lens-a.md`: the architecture for lane
dispatch metadata wiring, extended packet frontmatter, packet-path persistence, and the dispatched
packet-body HTTP endpoint. This lens owns proposal items #1-#4 and MUST make a final, concrete
decision on Open Questions 1, 2, and 5 (see `## Context`) — not another punt.

This is one of three parallel design lenses, sliced by capability rather than by generic
decisions/surface/testing role (the usual three-lens split does not fit six capabilities cleanly).
Cover your own slice's decisions, surface deltas, file changes, and testing notes together. It is
feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` and the full `specs/` tree for `lane-status-observability` are accepted, frozen, and
already committed on `main`. Lens B and lens C run in parallel against the same frozen inputs and
write to different files, so no lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/proposal.md` exists and is accepted.
- `openspec/changes/lane-status-observability/specs/` exists with all six capability deltas.
- `openspec/changes/lane-status-observability/design-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/lane-status-observability/proposal.md` (full) and these specs:
   `specs/lane-execution/spec.md`, `specs/read-only-packet-schema/spec.md`,
   `specs/dispatched-packet-body/spec.md`.
3. `internal/ledger/lanes_meta.go` (full file — `LaneMetadata`, `UpdateLaneMetadata`,
   `GetLaneMetadata`).
4. `internal/packet/packet.go` (full file — `Packet` struct and `Parse`'s frontmatter switch).
5. `internal/run/run.go:300-360` and `internal/run/batch.go:167-210` (the two
   `RegisterLane` call sites this change must follow with `UpdateLaneMetadata`).
6. `internal/serve/handlers.go:1-357` (full `NewHandlerWithConfig` — every existing mux route
   shape, especially the two-segment `/approvals/{runID}/{laneID}` parsing at handlers.go:316-350)
   and `internal/serve/model.go:140-333` (`Lane`, `laneDTO`, `ListLanes`, `GetLane`).
7. `internal/dag/parse.go` (full — `Node`) and `internal/dag/emit.go` (full — `EmitPacketContent`).
8. `cmd/lucind-ai/cli.go:160-174` (packet-path-to-lane mapping today).

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/design-lens-a.md`:

```markdown
# Design Lens A — Metadata, Frontmatter & Packet Body: Lane Status Observability

## Assumed architecture

<2-4 sentences naming the structural shape you are designing against. Lens B
and lens C write this same block independently against their own slices; the
synthesizer compares all three for contradiction, not overlap.>

## Decision 1 — Frontmatter key names (resolves Open Question 1)

**Choice**: <the exact three key names, final>
**Alternatives considered**: <the named alternatives from the proposal>
**Rationale**: <grounded in this repository's existing frontmatter naming convention and the
existing `LaneMetadata` Go field names>
**Terminal consumer**: <file:line>

## Decision 2 — Packet-path persistence mechanism (resolves Open Question 2)

**Choice**: <JSON audit-event field vs. a real `lanes` column, final>
**Alternatives considered**: <the other one, and why it loses>
**Rationale**: <grounded in what the other five extended `LaneMetadata` fields already do>
**Terminal consumer**: <file:line>

## Decision 3 — Packet-body HTTP endpoint

**Choice**: <exact method, path shape, response contract, 404 conditions>
**Alternatives considered**: <other path/param shapes>
**Rationale**: <grounded in an existing route's shape in this file>
**Terminal consumer**: <file:line>

## Decision 4 — DAG-wave packet metadata scope (resolves Open Question 5)

**Choice**: <in this change vs. explicit follow-up, final — not "open">
**Alternatives considered**: <the other option>
**Rationale**: <what breaks or does not break for a DAG-emitted packet either way>
**Terminal consumer**: <file:line>

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

`design-lens-a.md` MUST be under 1000 words. Tables over prose. Code snippets only for a
non-obvious pattern (e.g. the exact new mux route), never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens B owns**: `ProgressEvent`/`LaneProgress` numeric field additions, per-executor decoder
  wiring, the full v7 STRICT-table migration DDL (both `runs.pid` and `lane_progress` usage
  columns — yes, `runs.pid` too, even though lens C consumes it; lens B owns the whole migration
  so it is specified exactly once).
- **Lens C owns**: PID storage on `RegisterRun`, the startup-sweep-plus-ticker architecture, and
  PID-liveness syscall choice.

You may reference lens B's or lens C's future decisions in your `## Assumed architecture`, but do
not design them.

## Allowed paths

`openspec/changes/lane-status-observability/design-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**. The skill is authority on *what a design document
must contain*: required sections, the choice/alternatives/rationale shape of a decision, and the
threat-matrix applicability rule. This packet is authority on *how this phase is executed here*:
the three-lane capability split, this lens's slice, its word budget, output path and skeleton,
out-of-scope list, and done criteria. The skill describes one sub-agent writing a whole `design.md`
alone — parts of it will read as instructing you to do what this packet forbids (write the complete
document, persist to Engram, return the phase summary block, hold an 800-word budget). Those are
superseded here on purpose; note the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, line numbers ascending within each file.
The claim column is what YOU assert that range shows — one line, no hedging. This is a worklist for
the synthesizer, not a certificate: it opens and checks every row itself.

| citation | claim |
|---|---|
| `internal/ledger/lanes_meta.go:20-32` | LaneMetadata has no Skill or PacketPath field today |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Decisions 1, 2, and 5 each state a FINAL choice** — no "still open," no "either could work."
- [ ] **Every decision and file-change row names a terminal consumer with a `file:line` citation**
  that points at real code in this worktree.
- [ ] **`design-lens-a.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The proposal or specs contradict each other on a point this lens must decide, with no way to
  reconcile from the code.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Deciding Open Question 5 would require designing lens B's or lens C's slice.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- `LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`) has exactly these fields today: `RunID`,
  `LaneID`, `Model`, `Agent`, `SDDPhase`, `FanoutGroup`, `Change`, `Feature`, `AllowedPaths`,
  `Dependencies`, `BodyDigest`. **There is no `Skill` field and no `PacketPath` field.** Both are
  new work for this lens.
- Of those eleven fields, only `Model`, `Agent`, and `Feature` are real `lanes` table columns
  (schema v6, `internal/ledger/schema.go:249-251`). Every other field — `SDDPhase`, `FanoutGroup`,
  `Change`, `AllowedPaths`, `Dependencies`, `BodyDigest` — lives ONLY in the
  `lane_metadata:v1:`-prefixed JSON snapshot appended to `events` (`lanes_meta.go:12,67-77`), read
  back by `GetLaneMetadata` (`lanes_meta.go:89-127`). This is the established precedent Decision 2
  must weigh.
- `packet.Packet` (`internal/packet/packet.go:33-75`) and `Parse`'s frontmatter switch
  (`packet.go:94-138`) recognize `id, executor, routed_by, model, agent, read_only, feature,
  parent_ref, base_sha, expected_parent_sha, legacy_main, allowed_paths`. There is no `sdd_phase`,
  `fanout_group`, or `skill` case in that switch today, and no `Packet.Path` field is ever set by
  `Parse` — the on-disk path is known only to the CLI caller
  (`cmd/lucind-ai/cli.go:160-174`, index-aligned `packetFlags`/`ps`).
- Existing frontmatter keys are all lower-snake-case matching their Go field name
  (`read_only`→`ReadOnly`, `parent_ref`→`ParentRef`, `base_sha`→`BaseSHA`,
  `expected_parent_sha`→`ExpectedParentSHA`, `legacy_main`→`LegacyMain`,
  `allowed_paths`→`AllowedPaths`). `LaneMetadata` already names its fields `SDDPhase` and
  `FanoutGroup` in Go — the working frontmatter names `sdd_phase`/`fanout_group`/`skill` used
  throughout `proposal.md` and every spec scenario already match that convention.
- `internal/serve/handlers.go`'s `NewHandlerWithConfig` (`:190`) registers every route on one
  `http.ServeMux` via `mux.HandleFunc`. Two existing precedents matter for Decision 3:
  single-ID routes parsed with `singlePathID` (e.g. `/api/attempts/`, `handlers.go:205-216`), and
  **the one existing two-ID nested route**, `/approvals/{runID}/{laneID}[/defect]`
  (`handlers.go:316-350`), which does `strings.TrimPrefix` then `strings.Split` on `"/"` and
  switches on segment count. A packet-body route needs exactly this shape: two IDs (run, lane),
  GET-only, 404 on any unknown identity or unreadable file — see
  `openspec/changes/lane-status-observability/specs/dispatched-packet-body/spec.md` for the
  binding scenarios (200 with verbatim markdown; 404 for unknown run/lane; 404, not a crash, for a
  deleted/unreadable packet file).
- `serve.Lane` (`internal/serve/model.go:163-184`) and `laneDTO`
  (`model.go:322-333`) already surface `SDDPhase`, `FanoutGroup`, `Change`, `AllowedPaths`,
  `Dependencies`, `BodyDigest` from `LaneMetadata` — but no `Skill` and no packet-path/link field.
  `batch-wave-view`'s spec requires both: "lane dispatch metadata (model, agent, SDD phase, fanout
  group, feature, and skill when present)" and "a link to the dispatched packet body."
- `internal/dag/parse.go`'s `Node` (`:21-37`) and `internal/dag/emit.go`'s `EmitPacketContent`
  (`:26-76`) carry and emit `id, executor, routed_by, model, agent, feature, parent_ref, base_sha,
  expected_parent_sha, legacy_main, allowed_paths, read_only_paths` — no SDD/fanout/skill fields
  anywhere in the DAG-wave path. A DAG-emitted packet's frontmatter text is produced by `Emit`,
  then parsed by the same `packet.Parse` your extended frontmatter switch touches. If `Node` gains
  no new fields, a DAG-driven lane simply never populates `sdd_phase`/`fanout_group`/`skill` (they
  parse as empty strings, which is already a defined, non-failing case per
  `specs/read-only-packet-schema/spec.md`'s "Optional keys omitted" scenario) — it does not break.
  `apply-dag.yaml` today is used for implementation task waves (see
  `plugin/claude-code/skills/lucind-ai/SKILL.md`'s "Apply dispatch" section), not for SDD-phase
  planning fan-outs like this one, which dispatch via hand-authored packets. Weigh that real-usage
  gap against the cost of adding three more `Node`/`EmitPacketContent` fields with no live caller.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
accepted); "skill" observability is static frontmatter only, never live executor telemetry;
`delivery_strategy` is `exception-ok`.

**Out of scope, and including any of it is wrong:** live executor Skill telemetry decoding,
backfilling historical rows, `internal/resolve`/`internal/conflicttriage` (an unrelated change),
`cursor-agent` usage telemetry (lens B's exclusion, not yours).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
