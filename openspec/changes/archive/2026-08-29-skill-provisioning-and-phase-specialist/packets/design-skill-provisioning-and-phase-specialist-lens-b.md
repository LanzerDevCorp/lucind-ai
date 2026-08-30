---
id: design-skill-provisioning-and-phase-specialist-lens-b
executor: agy
routed_by: surface-and-flow lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/design-lens-b.md"]
---

# Packet design-skill-provisioning-and-phase-specialist-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-skill-provisioning-and-phase-specialist-lens-b  ·  **Branch:** lucind/design-skill-provisioning-and-phase-specialist-lens-b

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/design-lens-b.md`: how required
skills move from derivation through to a dispatched packet (body text and environment variable),
the exact frontmatter/contract/envelope/schema surface deltas, and the file-change table. This
lens MUST make a final, concrete decision on Open Question 5 (see `## Context`) — not another
punt, and MUST fully specify the `lane_role` closed-set values and the `## Required skills`
rendered format.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final
design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens A and lens C run in
parallel against the same frozen input and write to different files, so no lane races another.

Lens A owns the architecture decisions (three-tier derivation, resolution order, specialist shape)
and is running concurrently, so you do not have its final text. Declare the architecture you are
assuming in `## Assumed architecture` and design against it consistently. The synthesizer
arbitrates divergence; a silent second architecture does not survive that arbitration.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/skill-provisioning-and-phase-specialist/design-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill.
2. `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` (full) — Selected
   Candidate items 2, 4, 5, 8, and Conceptual Changes, and Open Question 5.
3. `internal/packet/packet.go` (full — the frontmatter switch and `Packet` struct fields).
4. `internal/packetauthor/compile.go` (full — `renderBody`, `normalizedContract`) and
   `internal/packetauthor/contract.go` (full — `Contract`).
5. `internal/executor/executor.go:1-40` (`requestEnv`, `readOnlyPathsEnv` — the existing env-var
   delivery pattern this change must follow for `LUCIND_REQUIRED_SKILLS`).
6. `internal/result/result.go:95-130` (`Envelope` struct) and `internal/result/result.schema.json`
   (full — the `additionalProperties: false` schema and its `properties` object).
7. `internal/accept/accept.go:263-328` (`validateVersionedEvidence`, including the duplicated
   decode struct at `:275-286`).
8. `internal/ledger/acceptance.go:1-110` (`SetDoneCandidate`'s `reflect.DeepEqual` and its SQL
   INSERT column list).

Never guess at a signature. Every row in your tables carries a `file:line` citation to real code in
this worktree.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/design-lens-b.md`:

```markdown
# Design Lens B — Surface & Flow: Skill Provisioning and the SDD Phase Specialist

## Assumed architecture

<2-4 sentences. Lens A and lens C write this same block independently against
their own slices; the synthesizer compares all three for contradiction.>

## Flow and Invariants

<How a required-skill name moves: (sdd_phase, lane_role) -> internal/skillset
union -> internal/skillroots resolution -> dual delivery (rendered body text +
LUCIND_REQUIRED_SKILLS env) -> agent declares skills_loaded in the envelope ->
enforceRequiredSkills demotes at run time -> accept re-verifies from frozen
evidence. A simple ASCII diagram, then the invariant at each hop and what
observably breaks if it does not hold.>

## Decision 1 — `lane_role` closed-set values and frontmatter wiring

**Choice**: <the exact seven-value closed set from the proposal, the exact
frontmatter key name, and how packet.Parse validates it, final>
**Alternatives considered**: <e.g. a numeric enum, a separate boolean per role>
**Rationale**: <grounded in the existing lower-snake-case frontmatter convention>
**Terminal consumer**: <file:line>

## Decision 2 — `## Required skills` rendered body format (resolves how dual
delivery's body half looks)

**Choice**: <the exact Markdown section renderBody emits: heading text, one line
per skill, resolved-path format>
**Alternatives considered**: <e.g. a table instead of a list>
**Rationale**: <grounded in the existing `## Done criteria`/`## Hard stops`
rendering pattern in the same function>
**Terminal consumer**: <file:line>

## Decision 3 — `lucind.yaml` and `.lucind/skill-roots.yaml` file naming
(resolves Open Question 5)

**Choice**: <final: does `lucind.yaml` at repo root collide with any existing
toolchain filename convention in this repository or common Go tooling; if a
collision risk exists, the final chosen filename>
**Alternatives considered**: <an alternative filename, if collision risk found>
**Rationale**: <grounded in what you actually found searching this repository
and common Go/YAML tooling conventions>
**Terminal consumer**: <file:line>

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-b.md` MUST be under 1000 words. Tables over prose. Do not restate code the reader can
open; cite it.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens A owns**: the three-tier derivation architecture, resolution order, budget default/config
  shape, and specialist CLI shape (Open Questions 1, 3, 4).
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity
  decision, including Open Question 2 (missing archive/ultrafixer child skills).

You will be tempted to argue for a derivation architecture while tabulating its surface. Do not.
If you believe the architecture you assumed is wrong, say so under `## Open Questions` with the
evidence — do not quietly design a different one.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/design-lens-b.md` only. Create no other
file.

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
| `internal/executor/executor.go:20-39` | requestEnv injects LUCIND_READ_ONLY_PATHS as a JSON-array env var today, the pattern LUCIND_REQUIRED_SKILLS must follow |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Decisions 1, 2, and 3 each state a FINAL choice**, including Decision 3's Open-Question-5
  resolution — no "either could work."
- [ ] **Every `Surface Deltas` and `File Changes` row carries a `file:line` citation** that points
  at real code in this worktree, and every `File Changes` row names a terminal consumer.
- [ ] **`design-lens-b.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The proposal does not determine whether a surface delta is additive or breaking.
- A file change cannot name any terminal consumer.
- Two reasonable architectures produce incompatible surface tables and the proposal does not
  choose between them.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- `packet.Packet` (`internal/packet/packet.go:43-103`) has no `LaneRole` field and `Parse`'s
  switch (`:122-179`) has no `lane_role` case, confirmed by direct read. Existing keys are all
  lower-snake-case matching their Go field (`read_only`→`ReadOnly`, `base_sha`→`BaseSHA`,
  `sdd_phase`→`SDDPhase`) — the working convention `lane_role` must follow.
- `renderBody` (`internal/packetauthor/compile.go:171-183`) today emits exactly: `# Goal`, `##
  Done criteria` (one `- ` line per criterion), `## Hard stops` (one `- ` line per stop), and `##
  Return` (a fenced code block naming the result-contract shape) — confirmed by direct read of the
  full function body. **There is no `## Required skills` section today**; this decision adds one.
- `requestEnv` (`internal/executor/executor.go:20-39`) builds an env slice from `os.Environ()`,
  strips any inherited `LUCIND_READ_ONLY_PATHS=` prefix, then appends a fresh
  `LUCIND_READ_ONLY_PATHS=<json array>` only when `req.ReadOnlyPaths` is non-empty — this exact
  strip-then-append shape is the established pattern `LUCIND_REQUIRED_SKILLS` must reuse.
- `result.Envelope` (`internal/result/result.go:103-116`) has these fields today: `PacketID,
  Status, Summary, HardStops, FilesChanged, ExternalChanges, DoneCriteria, Commit, Questions,
  Deviations, Findings, SessionID` — **no `SkillsLoaded` field.** `result.schema.json` line 5 is
  `"additionalProperties": false`, confirmed by direct read — a schema-only addition without a
  matching Go field (or vice versa) silently desyncs.
- `validateVersionedEvidence` (`internal/accept/accept.go:263-328`) decodes a **duplicated** inline
  struct at `:275-286` (`Version, Mode, WritePaths, ReadOnlyPaths, DoneCriteria, HardStops, Result`
  — confirmed by direct read) separately from `packetauthor.Contract`
  (`internal/packetauthor/contract.go:45-56`). A `LaneRole`/`RequiredSkills` field added to
  `Contract` but not to this duplicate is frozen into evidence but never verified at acceptance —
  exactly the risk the proposal's risk table flags as High.
- `SetDoneCandidate` (`internal/ledger/acceptance.go:96-103`, confirmed by direct read) falls back
  to `reflect.DeepEqual(got, candidate)` on a zero-rows-affected INSERT — any new Go-side field on
  `LaneCandidate` must stay reflected in the SQL INSERT/SELECT column list or this equality check
  silently diverges from what was actually persisted.
- `openspec/config.yaml` is the existing tracked SDD config at repo root (confirmed present); no
  other tracked file currently named `lucind.yaml` exists at repo root (confirmed by directory
  listing) — verify against actual Go toolchain conventions (`go.mod`, `.golangci.yml`, etc.) rather
  than assuming no collision.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
pre-accepted); `AuthoringEvidence` shape and version (`lane-authoring-evidence/v1`) never change,
skills ride inside the existing `Contract json.RawMessage` blob; no SQLite migration.

**Out of scope, and including any of it is wrong:** the derivation algorithm itself (lens A's), the
threat matrix and rollback plan (lens C's), live executor Skill telemetry, backfilling historical
rows.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
