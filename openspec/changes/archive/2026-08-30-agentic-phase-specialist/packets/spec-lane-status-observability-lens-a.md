---
id: spec-lane-status-observability-lens-a
executor: agy
routed_by: metadata and frontmatter parsing lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/spec-lens-a.md"]
---

# Packet spec-lane-status-observability-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-lane-status-observability-lens-a  ·  **Branch:** lucind/spec-lane-status-observability-lens-a

## Goal

Produce `openspec/changes/lane-status-observability/spec-lens-a.md`: the delta requirements and
scenarios for the two dispatch-side requirements — **Lane metadata dispatch persistence** and
**Extended packet frontmatter parsing** — targeting capabilities `lane-execution` and
`read-only-packet-schema`.

This is one of three parallel spec lenses. Unlike the standard aspect split (capabilities-vs-
scenarios-vs-conflicts), this fan-out splits by **capability domain**: you own both the requirement
text and its scenarios for your two requirements, end to end. You do NOT open the live specs for
`lane-execution` or `read-only-packet-schema` — that verification and any full-block copy is lens
C's job (it owns the live-spec-conflict role for all three modified capabilities in this change).
Do not write anything under `openspec/changes/lane-status-observability/specs/`.

## Why this is safe to dispatch now

The proposal for `lane-status-observability` is accepted and frozen (merge commit `97e9bbc`). Lens
B and lens C run in parallel against the same frozen inputs and write to different files, so no
lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/proposal.md` exists and is accepted.
- `openspec/changes/lane-status-observability/spec-lens-a.md` does not yet exist.
- `openspec/specs/lane-execution/spec.md` and `openspec/specs/read-only-packet-schema/spec.md`
  exist (confirm existence only — do not read them in full; that is lens C's job).

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/lane-status-observability/proposal.md` in full. Its `## Delta Specifications`
   section already contains full requirement text and two scenarios each for **Lane metadata
   dispatch persistence** and **Extended packet frontmatter parsing** — treat that as a strong
   draft to verify and refine, not something to invent from scratch. Its `## Capabilities` section
   maps both requirements to *Modified Capabilities*: `lane-execution` and
   `read-only-packet-schema`.
3. `openspec/changes/lane-status-observability/proposal-synthesis-notes.md` — records citations the
   propose synthesizer opened and dropped. Do NOT resurrect any of them (full list in `## Context`
   below).

Do not open `openspec/specs/lane-execution/spec.md` or `openspec/specs/read-only-packet-schema/spec.md`
in full. Cite their existence and requirement-header count only (given to you in `## Context`);
opening them in full is lens C's exclusive responsibility so the live-spec read happens exactly
once and its findings are reconciled in one place.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/spec-lens-a.md`:

```markdown
# Spec Lens A — Metadata & Frontmatter: Lane Status Observability

## Assumed requirements

<2-4 sentences: the two requirements you are specifying, which capability each
targets, and why each is ADDED (new requirement inside an existing capability's
delta) rather than MODIFIED (editing an existing requirement's text). Lens B and
lens C write their own version of this block independently; the synthesizer
compares all three.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

<Two rows: lane-execution and read-only-packet-schema. Both "existing" — target
`openspec/changes/lane-status-observability/specs/<capability>/spec.md`, cite
`openspec/specs/<capability>/spec.md` (existence only, per Required Reading above).>

## ADDED Requirements

### Requirement: Lane metadata dispatch persistence

<RFC 2119 requirement text. Start from proposal.md's draft; verify every citation
independently before repeating it.>

**Terminal consumer**: <file:line for `Execute`, `ensureLaneFailed`, and
`UpdateLaneMetadata` — verify each citation yourself>

#### Scenario: Dispatch persists metadata

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>

#### Scenario: Historical rows preserved

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>

<Add one edge-case or error-state scenario beyond proposal.md's two, if a testable
one exists — e.g. metadata write failing after RegisterLane succeeds. "None
testable" is a legitimate answer; say so under Open Questions instead of
inventing an untestable one.>

### Requirement: Extended packet frontmatter parsing

<same shape as above, for `packet.Parse`>

**Terminal consumer**: <file:line — verify independently>

#### Scenario: Parse frontmatter keys

...

#### Scenario: Optional keys omitted

...

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-a.md` MUST be under 1000 words. Requirement text is terse — one or two sentences with an
RFC 2119 keyword.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens B owns**: `dispatched-packet-body` and `lane-progress-telemetry` — their requirements,
  scenarios, and capability rows.
- **Lens C owns**: `orphan-lane-reconciliation`'s requirement and scenarios; opening
  `lane-execution/spec.md` and `read-only-packet-schema/spec.md` in full, confirming your ADDED
  classification against the live text (or correcting it to MODIFIED with evidence — lens C's
  live-spec evidence outranks yours on classification only), copying any MODIFIED full block
  forward, and `batch-wave-view`'s own requirement (which has no draft in proposal.md at all).

Do NOT create or write any file under `openspec/changes/lane-status-observability/specs/`. That
tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/lane-status-observability/spec-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its `references/`.

Precedence is **not symmetric**. The skill is authority on *what a delta spec must contain*: the
ADDED/MODIFIED/REMOVED/RENAMED format, the RFC 2119 rule, the one-scenario-minimum rule. Where this
packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`. This packet is authority on *how this phase is executed here*: the
capability-domain split (not the standard capabilities-vs-scenarios-vs-conflicts split), which
requirements this lane owns, its word budget, output path/skeleton, out-of-scope list, and done
criteria. The skill describes one sub-agent writing the whole delta tree alone and will read as
instructing you to write files under `specs/`, persist to Engram, or return the phase summary
block — those are superseded here. Note the conflict in `## Open Questions`, follow this packet.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, ascending line numbers.

| citation | claim |
|---|---|
| `internal/run/run.go:334` | `Execute` calls `RegisterLane` and never follows up with metadata |

The manifest is a worklist for the synthesizer, not a certificate. Listing a citation does not
excuse getting it wrong — it makes getting it wrong cheaper to catch.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim it supports.**
- [ ] **Both capability-map rows cite the live spec's existence with `file:line` that resolves in this worktree** (existence check only — full content is lens C's job).
- [ ] **Both requirements carry an RFC 2119 keyword**, state observable behavior, and are independently classified ADDED with reasoning (not merely copied from proposal.md's classification).
- [ ] **Every scenario's THEN is observable**; the coverage table names the seam or marks "new seam required".
- [ ] **`spec-lens-a.md` exists, is under 1000 words, and carries `## Assumed requirements` and `## Capability Map`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- Either requirement cannot be stated without deciding an implementation detail the design phase
  has not decided (e.g. the exact frontmatter key names — Open Question 1 stays open).
- A capability-map citation does not resolve in this worktree.
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/lane-status-observability/proposal.md` is committed in this worktree and is the
accepted proposal.** Read it first.

**User-approved decisions, already final — do not re-litigate:**

- Full six-item scope ships as **one PR**, accepting `size:exception` for the review-budget risk.
- "Skill" observability is **static only**: which skill/SDD-phase authored a packet, via a new
  `skill:` frontmatter key (part of your `Extended packet frontmatter parsing` requirement, along
  with `sdd_phase` and `fanout_group`). Live runtime skill telemetry is explicitly OUT of scope —
  do not write a requirement or scenario implying live skill telemetry.
- `delivery_strategy` is `exception-ok` for this change.

**Five open questions from proposal.md's `## Open Questions` remain OPEN. Write requirements and
scenarios that are correct regardless of how these resolve later at design time:**

1. Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs
   `generated_by`. Your `Extended packet frontmatter parsing` requirement MUST NOT hardcode one of
   these as if decided — write the requirement using the working names from proposal.md
   (`sdd_phase`, `fanout_group`, `skill`) but note in `## Open Questions` that the exact key names
   are still open, so a scenario naming a concrete key is a necessary placeholder, not a decision.
2. Packet path persistence mechanism (new `LaneMetadata.PacketPath` field vs. a real `lanes`
   column) — not this lens's requirement, but do not assume an answer if you reference it.
3. Ticker interval for orphan sweep — not this lens's requirement.
4. PID-liveness syscall/portability — not this lens's requirement.
5. DAG-wave `Node`/`EmitPacketContent` scope — not this lens's requirement.

**Process note — verify citations independently.** Propose-phase lens citations into
`internal/serve/handlers.go` were frequently wrong (citing `:33-60` or `:30-120` for the
packet-route concept when the real route registration point is `NewHandlerWithConfig` at
`handlers.go:190`). This does not directly affect your two requirements, but the same discipline
applies: do not trust old lens citations blindly, verify independently in this worktree.

**Dropped citations from the propose-phase synthesis — do NOT resurrect these:**

- `internal/ledger/schema.go:310-330` is NOT v7 DDL (it is the `migrate` function comment/start).
- `internal/serve/handlers.go:33-60` and `:30-120` are `ServerState` fields and `/api/state`
  pagination bounds — NOT the packet-route or orphan-sweep seam. `handlers.go:190`
  (`NewHandlerWithConfig`) is the real mux registration point.
- `internal/serve/server.go:1-60`/`:19-53` is `ListenAndServe` — NOT orphan sweep, PID liveness, or
  a ticker.
- `internal/ledger/lanes.go:35-50` does not exist. `SetStatus` is `internal/ledger/ledger.go:452`.
- `internal/ledger/schema.go:298-308` is `CREATE TABLE lane_progress` (message/seq only), NOT the
  `runs.pid` seam. `runs` without `pid` is `schema.go:226-234`.
- `openspec/specs/lane-envelope-inspector` is NOT the packet-body capability (it is demotion
  diagnosis for deviated lanes). The new capability is `dispatched-packet-body` — lens B's, not
  yours, but do not cite `lane-envelope-inspector` for anything in your slice either.

**Ground truth for your two requirements — cite it, verify it, do not re-derive it:**

- `ledger.LaneMetadata` struct: `internal/ledger/lanes_meta.go:20-32`. `UpdateLaneMetadata` /
  `GetLaneMetadata`: `lanes_meta.go:39,89`. Zero production callers today — only tests.
- `internal/run/run.go:334` (`Execute`) and `internal/run/batch.go:184` (`ensureLaneFailed`) call
  `RegisterLane` and never follow up with metadata.
- `packet.Parse`: `internal/packet/packet.go:78-167`, recognized keys at `:94-138`. `Packet` struct:
  `packet.go:33-75`. No `sdd_phase`/`fanout_group`/`skill` key exists yet; `Packet` has no `Path` or
  `Skill` field yet.
- Existing packet parse test coverage: `internal/packet/packet_test.go:15-67` — your new optional
  keys must not break these.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
