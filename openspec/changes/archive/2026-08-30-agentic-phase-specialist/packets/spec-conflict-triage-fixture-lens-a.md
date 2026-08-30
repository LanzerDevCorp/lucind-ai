---
id: spec-conflict-triage-fixture-lens-a
executor: agy
routed_by: capabilities and requirements lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/spec-lens-a.md"]
---

# Packet spec-conflict-triage-fixture-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-conflict-triage-fixture-lens-a  ·  **Branch:** lucind/spec-conflict-triage-fixture-lens-a

## Goal

Produce `openspec/changes/conflict-triage-fixture/spec-lens-a.md`: the capability-to-file map for this change, and every requirement statement it introduces or changes, each classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/conflict-triage-fixture/specs/`.

## Why this is safe to dispatch now

The proposal for `conflict-triage-fixture` is accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another. This lens owns the requirement set; the other two write scenarios for it and check it against live specs.

## Preconditions

- `openspec/changes/conflict-triage-fixture/proposal.md` exists and is accepted.
- `openspec/changes/conflict-triage-fixture/spec-lens-a.md` does not yet exist.
- `openspec/specs/` exists (or the packet `## Context` states this repository has no main specs yet).

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *which* requirements exist and *what they say* — not to scenarios and not to migration:

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/conflict-triage-fixture/proposal.md`, and its **Capabilities section** in particular.
   That section is the primary contract: each entry under *New Capabilities* becomes a full spec
   at `openspec/specs/<capability>/spec.md`; each entry under *Modified Capabilities* becomes a
   delta at `openspec/changes/conflict-triage-fixture/specs/<capability>/spec.md`.
3. The index of `openspec/specs/` — enough to know which named capabilities already exist and
   which of the proposal's names are new.
4. `openspec/changes/archive/` for a prior change that touched the same capability, if one exists.

Never guess at a capability name. A capability you cannot cite in the proposal or in `openspec/specs/` does not exist yet, and saying so is the useful answer.

## Output format

Write exactly this skeleton to `openspec/changes/conflict-triage-fixture/spec-lens-a.md`:

```markdown
# Spec Lens A — Capabilities & Requirements: Conflict Triage Fixture

## Assumed requirements

<2–4 sentences naming the requirement set you are specifying: which capabilities
this change touches, how many requirements each gets, and whether each capability
is new (full spec) or existing (delta). Lens B and lens C write this same block
independently; the synthesizer compares all three. Be specific enough that a
disagreement is visible.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

<One row per capability the proposal names. "New" rows target
`openspec/specs/<capability>/spec.md` and cite nothing. "Existing" rows target
`openspec/changes/conflict-triage-fixture/specs/<capability>/spec.md` and MUST cite the live
spec they delta.>

## ADDED Requirements

### Requirement: <Name>

<The requirement text, using an RFC 2119 keyword — MUST, SHALL, SHOULD, MAY.
State the observable behavior, not the implementation. Write no scenarios here;
lens B owns them.>

**Terminal consumer**: <the code, CLI surface, or artifact that makes this
requirement observable, with file:line — or "new, introduced by this change".>

## MODIFIED Requirements

### Requirement: <Existing Requirement Name>

<The full updated requirement text — it replaces the live one entirely.>
(Previously: <one line on what changed>)

**Live block**: <file:line of the requirement in `openspec/specs/`, and the
number of scenarios it currently carries. The synthesizer copies that whole block
forward; your job is to make it findable and to say what the new text is.>

## REMOVED Requirements

### Requirement: <Name>

(Reason: <why>)
(Migration: <what replaces it, or "None">)

## RENAMED Requirements

### Requirement: <Old Name> → <New Name>

(Reason: <why>)

## Open Questions

- [ ] <unresolved question, or "None">
```

Omit any of the four classification sections that has no entries. Do not write an empty heading.

## Size budget

`spec-lens-a.md` MUST be under 1000 words. Requirement text is terse by nature — one or two sentences with an RFC 2119 keyword. If the requirement set does not fit, that is a signal the change is too large for one spec phase; say so in `## Open Questions` rather than truncating silently.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: every `#### Scenario:` block, in Given/When/Then form, and the happy-path/edge-case coverage argument.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement, and every Migration note.

You name a requirement as `REMOVED` or `RENAMED` and give its Reason. Lens C writes the Migration guidance behind it. State a Migration line only if the proposal already fixed it.

Do NOT create or write any file under `openspec/changes/conflict-triage-fixture/specs/`. That tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/conflict-triage-fixture/spec-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a delta spec must contain*: the ADDED / MODIFIED / REMOVED /
RENAMED section format, the RFC 2119 requirement-strength rule, the "every requirement has at
least one scenario" rule, and the MODIFIED copy-full-then-edit workflow. Where this packet
paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the spec is split across
three parallel lanes, which slice this lane owns, its word budget, its output path and skeleton,
its out-of-scope list, and its done criteria. The skill describes one sub-agent writing the whole
delta spec tree by itself, so parts of it will read as instructing you to do what this packet
forbids — write scenarios, write files under `specs/`, persist to Engram, return the phase summary
block. Those are superseded here on purpose. Do not correct yourself toward them; note the conflict
in `## Open Questions` and follow this packet.

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
- [ ] **Every capability-map row is classified new or existing**, and every "existing" row cites the live spec with `file:line` that resolves in this worktree.
- [ ] **Every requirement carries an RFC 2119 keyword** and states observable behavior rather than implementation.
- [ ] **`spec-lens-a.md` exists, is under 1000 words, and carries `## Assumed requirements` and `## Capability Map`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposal has no Capabilities section and no *Affected Areas* section to infer one from.
- A capability the proposal names as *Modified* has no live spec in `openspec/specs/`, so it is really new and the proposal is wrong about which.
- A requirement cannot be stated without deciding an implementation detail the design phase has not decided.
- Satisfying one instruction in this packet would require violating another.

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

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
