---
id: design-conflict-triage-fixture-lens-a
executor: agy
routed_by: decisions lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/design-lens-a.md"]
---

# Packet design-conflict-triage-fixture-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-conflict-triage-fixture-lens-a  ·  **Branch:** lucind/design-conflict-triage-fixture-lens-a

## Goal

Produce `openspec/changes/conflict-triage-fixture/design-lens-a.md`: the technical approach and every architecture decision for this change, each with its choice, the alternatives rejected, the rationale, and the terminal consumer that makes the decision observable.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `conflict-triage-fixture` are accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another. This lens owns the architectural choice; the other two consume it.

## Preconditions

- `openspec/changes/conflict-triage-fixture/proposal.md` exists and is accepted.
- `openspec/changes/conflict-triage-fixture/specs/` exists (or the packet `## Context` states it does not).
- `openspec/changes/conflict-triage-fixture/design-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to structure, not to signatures or tests:

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/conflict-triage-fixture/proposal.md` and `openspec/changes/conflict-triage-fixture/specs/`.
3. The entry points and module structure of the packages the change lands in.
4. The existing patterns and conventions those packages already follow — how comparable problems were already solved in this repository.
5. `openspec/changes/archive/` for a prior change that solved a structurally similar problem, if one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/conflict-triage-fixture/design-lens-a.md`:

```markdown
# Design Lens A — Decisions: Conflict Triage Fixture

## Assumed architecture

<2–4 sentences naming the structural shape you are designing against: which
existing types or packages get extended, which are new. Lens B and lens C write
this same block independently; the synthesizer compares all three. Be specific
enough that a disagreement is visible.>

## Technical Approach

<Concise strategy. How it maps to the proposal. Reference spec requirements by name.>

## Decision 1 — <title>

**Choice**: <what we chose>
**Alternatives considered**: <what we rejected>
**Rationale**: <why this over the alternatives, grounded in this repository's code>
**Terminal consumer**: <the concrete symbol, command, or spec requirement that
consumes this decision — with file:line. A decision no terminal consumer reaches
is not a decision, it is speculation.>

## Decision N — <title>

<same four fields>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`design-lens-a.md` MUST be under 1000 words. Decisions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: the file-changes table, data-flow diagrams, invariants, and every exact type/schema/CLI signature delta.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity decision.

Do not write a rollback decision here even though it is shaped like a decision. It belongs to lens C.

## Allowed paths

`openspec/changes/conflict-triage-fixture/design-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a design document must contain*: its required sections, the
choice / alternatives / rationale shape of a decision, and the threat-matrix applicability rule.
Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the design is split
across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
a whole `design.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the complete document, persist it to Engram, return the phase summary block, hold
an 800-word budget. Those are superseded here on purpose. Do not correct yourself toward them; note
the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation.

| citation | claim |
|---|---|
| `internal/run/attempt.go:743-747` | ErrNoMergeBase makes evaluateOverlapGate continue |

Rules:

- **One row per UNIQUE citation.** A range cited three times in the prose appears once here.
- **Group by file**, files alphabetical, line numbers ascending within each file.
- **The claim is what YOU assert that range shows** — one line, stated plainly, no hedging. Not a
  description of the file; the specific thing you are using it as evidence for.
- **This section does not count against the word budget.** Never trim analysis to make room for
  it, and never trim it to fit the budget.
- **The manifest is a worklist, not a certificate.** Listing a citation here asserts nothing about
  its correctness — the synthesizer opens and checks every single one. Writing a row does not
  spare you from getting the citation right; it makes getting it wrong cheaper to catch.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **Every decision names a terminal consumer with a `file:line` citation**, and that citation points at real code in this worktree.
- [ ] **`design-lens-a.md` exists, is under 1000 words, and carries every skeleton section including `## Assumed architecture`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposal or specs do not determine an architectural choice, and two reasonable shapes are equally supported.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Making this decision would require designing the file-change surface or the test strategy, which this packet forbids.

## Context

**`openspec/changes/conflict-triage-fixture/proposal.md` is committed in this worktree and is the
accepted proposal.** Read it first. `proposal-synthesis-notes.md` beside it records twelve
citations the propose synthesizer opened and dropped — do NOT resurrect any of them. Two examples,
so the shape is clear: `internal/reconcile/reconcile.go:163-165` is `NewService` assigning the
default evaluator, NOT the triage-JSON slot (that is `Candidate.Output` at `:105`);
`internal/serve/handlers.go:95-109` is `/api/state` pagination constants, NOT the read-only GET
reconcile routes.

**Four product questions are DECIDED. Do not re-open, re-offer, or widen them:**

1. Verify-budget unit is **wall clock plus the concrete command** (e.g. "~4 min:
   `./lucind-checks.sh` on the combined tree"). Token/pricing and test-weight units were rejected.
2. Risk is **three bands** — low / medium / high — with a business conflict pinned to `high` and
   the agent unable to lower it. A continuous 0-100 score was rejected.
3. The fixture generator lives at **`internal/conflicttriage/fixture/`**. `test/fixture/` and a
   public CLI subcommand were rejected.
4. The A/B win criterion is **correct classification of the three hunks** — business separated
   from the two mechanical controls, arbitrariness declared where it belongs. Grading the prepared
   resolution, and timing a human to thirty seconds, were both rejected.

**Two questions MUST stay open. Answering either by guess is a lane failure:**

- the exact non-decreasing risk formula and its thresholds, including mixed business+mechanical
  hunks;
- which executor/model runs production triage.

**One real ambiguity the proposal left unresolved — the orchestrator found it in review, and it
belongs to design, not specs.** The proposal's triage requirement says the agent must "leave a
prepared SHA (`internal/reconcile/reconcile.go:107`)", while its Approach says "a human resolves
out of band and registers the SHA" via `reconcile resolve --candidate --sha`. Both cannot hold. If
the agent writes `CandidateSHA` into the ledger itself it bypasses the human registration step
that makes the whole resolution safe. Design must decide explicitly who writes that field and say
why; specs must not assume an answer.

**Ground truth — cite it, do not re-derive it:**

- `evaluateOverlapGate` (`internal/run/attempt.go:687`) classifies via `overlap.Classify`
  (`internal/overlap/overlap.go:623`); `ClassRequired` at `:658-659`; thresholds at `:93-98`.
- Evidence for `ClassRequired` is inserted inside `CreateRequest`
  (`internal/reconcile/reconcile.go:266`), never on the warning branch.
- Without a registered shared `base_sha`, `ErrNoMergeBase` makes the gate `continue`
  (`internal/run/attempt.go:743-747`) and `ClassRequired` is unreachable.
- Triage does NOT fail closed, unlike `internal/resolve` (`internal/resolve/candidate.go:26`,
  prompt at `:303-312`). Keep the two disciplines separate.
- Ledger today: 36 `integration_attempts`, 0 `overlap_evidence`, 0 `reconciliation_requests`,
  0 `reconciliation_candidates`.

**Out of scope, and including any of it is wrong:** reconcile POST on the web surface (read-only
GET stays), overlap thresholds, and production dispatch paths.

**`openspec/changes/conflict-triage-fixture/specs/` does not exist yet.** The specs fan-out is
running concurrently with this one, in its own worktrees you cannot see. Reason from the
proposal's `## Capabilities` section, never from requirement ids: a design that cites a
requirement name it never read is citing something it invented.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
