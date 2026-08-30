---
id: design-skill-provisioning-and-phase-specialist-lens-a
executor: agy
routed_by: decisions lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/design-lens-a.md"]
---

# Packet design-skill-provisioning-and-phase-specialist-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-skill-provisioning-and-phase-specialist-lens-a  ·  **Branch:** lucind/design-skill-provisioning-and-phase-specialist-lens-a

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/design-lens-a.md`: the technical
approach and every architecture decision for making required skills a typed, binary-derived,
frozen part of the packet contract, plus the per-phase specialist. This lens MUST make a final,
concrete decision on Open Questions 1, 3, and 4 (see `## Context`) — not another punt.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final
design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` for `skill-provisioning-and-phase-specialist` is accepted, frozen, and already
committed on this branch. Lens B and lens C run in parallel against the same frozen input and
write to different files, so no lane races another. This lens owns the architectural choice; the
other two consume it.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/skill-provisioning-and-phase-specialist/design-lens-a.md` does not yet exist.
- No `openspec/changes/skill-provisioning-and-phase-specialist/specs/` tree exists yet (specs is a
  sibling phase running concurrently, not a dependency of design).

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` (full) — its Selected
   Candidate & Approach items 1, 6, 7, 8, and Open Questions 1, 3, 4.
3. `internal/packet/packet.go` (full — the frontmatter switch that must gain `lane_role`).
4. `internal/packetauthor/contract.go` (full — `Contract`, `normalizedContract` is in `compile.go`)
   and `internal/packetauthor/compile.go` (full — `Compile`, `validateContract`, `renderBody`).
5. `internal/ledger/authoring.go` (full — `AuthoringEvidence`, `Contract json.RawMessage`,
   freeze/decode).
6. `cmd/lucind-ai/packet_authoring.go` (full — `admitDispatchBatch`, the batch admission seam).
7. `.opencode/agent/lucind-packet-author.md` (full — the specialist's tool-denial profile).
8. `internal/dag/parse.go` (full — `Node`) — for Decision 3's DAG-wave scope call.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/design-lens-a.md`:

```markdown
# Design Lens A — Decisions: Skill Provisioning and the SDD Phase Specialist

## Assumed architecture

<2-4 sentences naming the structural shape you are designing against: which new
packages get added (skillset, skillroots, lucindconfig, phasespec), which
existing files gain a case/field. Lens B and lens C write this same block
independently; the synthesizer compares all three for contradiction.>

## Technical Approach

<Concise strategy: three-tier derivation, dual delivery, two-site enforcement,
composing specialist. Reference the proposal's Selected Candidate items by
number.>

## Decision 1 — Ad-hoc authoring surface shape (resolves Open Question 1)

**Choice**: <final: new frontmatter key vs a typed `packetauthor.Contract` field
vs both, for how an orchestrator supplies ad-hoc skill names>
**Alternatives considered**: <the other options from the proposal's framing>
**Rationale**: <grounded in `Contract`'s existing shape and the manual-packet path
that has no `Contract` at all>
**Terminal consumer**: <file:line>

## Decision 2 — Budget default and override shape (resolves Open Question 3)

**Choice**: <final: is 3 a Go constant, a `lucind.yaml` field, or both — and what
rejects a batch that exceeds it>
**Alternatives considered**: <hard-coded only vs config-overridable>
**Rationale**: <grounded in what `admitDispatchBatch` already validates before
worktree allocation>
**Terminal consumer**: <file:line>

## Decision 3 — Specialist CLI shape (resolves Open Question 4)

**Choice**: <final: one parameterized `lucind-ai phase <name>` subcommand and one
opencode profile, vs per-phase subcommands/profiles>
**Alternatives considered**: <the other shape, and its cost in generated files>
**Rationale**: <grounded in the existing subcommand dispatch table in
cmd/lucind-ai/cli.go and the fixed six-phase SDD lifecycle>
**Terminal consumer**: <file:line>

## Decision 4 — `internal/skillset` derivation function shape

**Choice**: <exact function signature: `Derive(sddPhase, laneRole string, stack,
adhoc []string) ([]string, error)` or equivalent, final>
**Alternatives considered**: <e.g. a method on a config struct instead>
**Rationale**: <grounded in `(sdd_phase, lane_role)` being the only two proposal
inputs to derivation>
**Terminal consumer**: <file:line>

## Decision 5 — `internal/skillroots` resolution and `lucind.yaml`/`skill-roots.yaml` loading

**Choice**: <exact resolution order: `.lucind/skill-roots.yaml` root list with `~`
expansion, first match wins or all roots searched; and how `lucind.yaml` unknown
keys are rejected>
**Alternatives considered**: <alternatives to ordered-root lookup>
**Rationale**: <grounded in `internal/dag/parse.go`'s existing YAML-loading
pattern>
**Terminal consumer**: <file:line>

## Decision 6 — `internal/phasespec` specialist composition

**Choice**: <exact shape: what it reads from `gentle-ai sdd-status`, what it
calls in lucind-ai's own dispatch machinery, and what write path it owns for
`openspec/changes/<change>/<phase>.md`>
**Alternatives considered**: <intercepting gentle-ai output vs composing it, per
the proposal's rejected alternative>
**Rationale**: <grounded in the proposal's "gentle-ai keeps authority" framing>
**Terminal consumer**: <file:line>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`design-lens-a.md` MUST be under 1000 words. Decisions as compact blocks, not essays.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens B owns**: the file-changes table, `## Required skills` rendered-body format, envelope
  `skills_loaded` schema delta, `LUCIND_REQUIRED_SKILLS` env-var shape, `lane_role` closed-set
  values, and Open Questions 2 and 5.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity
  decision.

Do not write a rollback decision here even though it is shaped like one. It belongs to lens C. Do
not design the exact `## Required skills` rendered text or the envelope schema delta — those are
lens B's.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/design-lens-a.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**. The skill is authority on *what a design document
must contain*: required sections, the choice/alternatives/rationale shape of a decision, and the
threat-matrix applicability rule. This packet is authority on *how this phase is executed here*:
the three-lane split, this lens's slice, its word budget, output path and skeleton, out-of-scope
list, and done criteria. The skill describes one sub-agent writing a whole `design.md` alone —
parts of it will read as instructing you to do what this packet forbids. Those are superseded here
on purpose; note the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, line numbers ascending within each file.
The claim column is what YOU assert that range shows — one line, no hedging.

| citation | claim |
|---|---|
| `internal/packet/packet.go:159-164` | sdd_phase and skill are parsed as plain strings today, with no closed-set validation and no lane_role case |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Decisions 1, 2, and 3 each state a FINAL choice** (resolving Open Questions 1, 3, and 4) —
  no "still open," no "either could work."
- [ ] **Every decision names a terminal consumer with a `file:line` citation** that points at real
  code in this worktree.
- [ ] **`design-lens-a.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The proposal does not determine an architectural choice, and two reasonable shapes are equally
  supported.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Deciding Open Question 4 would require designing lens B's or lens C's slice.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- `packet.Packet` (`internal/packet/packet.go:43-103`) and `Parse`'s frontmatter switch
  (`packet.go:122-179`) already recognize `sdd_phase` (`:159-160`) and `skill` (`:163-164`) as
  plain strings with **no validation** — confirmed by direct read of this exact file at this exact
  commit. There is **no `lane_role` case anywhere in the switch.** This is genuinely new work, not
  drift from an earlier snapshot.
- `packetauthor.Contract` (`internal/packetauthor/contract.go:45-56`) has no `LaneRole` or
  `RequiredSkills` field today. `normalizedContract` (`internal/packetauthor/compile.go:15-25`) and
  `renderBody` (`compile.go:171-183`) likewise carry no skill-related field — `renderBody` today
  emits only `# Goal`, `## Done criteria`, `## Hard stops`, `## Return` (confirmed by direct read).
  Any `## Required skills` section is new output this change introduces.
- `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:26`) is `json.RawMessage` inside a
  struct whose `Hash` is computed over the whole encoded struct (`FreezeAuthoringEvidence`,
  `authoring.go:44-59`) — adding bytes inside `Contract` changes nothing about the struct shape or
  `AuthoringEvidenceVersion` (`:14`, `"lane-authoring-evidence/v1"`), confirmed unchanged for this
  proposal.
- `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) is the pre-worktree,
  pre-quota admission seam: it builds `packetauthor.BatchItem` for every packet in the batch and
  calls `packetauthor.AdmitBatch` once, before any lane creation. A budget-reject or
  unresolved-skill reject belongs here, not later.
- `.opencode/agent/lucind-packet-author.md:6-7` denies all tools (`permission: "*": deny`),
  confirmed by direct read — the specialist cannot itself scan a filesystem for skill content;
  skill *selection* stays outside its typed contract, per the proposal's rejected-alternative list.
- `internal/dag/parse.go:22-37`'s `Node` struct has fields for `id, executor, routed_by, model,
  agent, feature, parent_ref, base_sha, expected_parent_sha, legacy_main, allowed_paths,
  read_only_paths, depends_on, body_path` — **no `sdd_phase`, `fanout_group`, `skill`, or
  `lane_role` field**, confirmed by direct read. `EmitPacketContent` (`internal/dag/emit.go:26-31`)
  emits only what `Node` carries. Weigh whether Decision 3's specialist ever emits DAG-wave
  packets (apply/verify roles do, per proposal item 1) against adding fields with no current
  caller — `apply-dag.yaml` today serves implementation task waves, not SDD-phase planning
  fan-outs like this one.
- `cmd/lucind-ai/cli.go`'s subcommand switch already dispatches `run`, `split`, `check`, `accept`,
  `feature ...`, `reconcile ...`, `defect ...`, `worktree`, `integrate` by literal string match on
  `args[0]` (confirmed by direct read of the dispatch table around `cli.go:155-168`) — a `phase`
  subcommand fits the identical shape, whichever way Decision 3 resolves.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
pre-accepted, `openspec/config.yaml:6-7`: `delivery_strategy: single-pr`, `review_budget_lines:
10000`); execution mode is `auto` (`openspec/config.yaml:5`); artifact store is `hybrid`
(`openspec/config.yaml:3-4`); `AuthoringEvidence` shape and version never change; no SQLite
migration; the specialist never wraps or intercepts gentle-ai; skill content authoring itself is
out of scope.

**Out of scope, and including any of it is wrong:** `internal/ledger/schema.go` migrations,
specialist-side skill *selection* (the specialist plans/dispatches, it does not choose which
skills a lane needs — that is `internal/skillset`'s job, called by the CLI, not the specialist),
treating an external skill content edit as a blocking gate, CLI failure-guidance banners
(`cmd/lucind-ai/cli.go:699,737,759,2004`).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
