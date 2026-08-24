---
id: design-<change-id>-lens-a
executor: agy
routed_by: decisions lens of the three-lens design fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/design-lens-a.md"]
---

# Packet design-<change-id>-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/design-<change-id>-lens-a  ·  **Branch:** lucind/design-<change-id>-lens-a

## Goal

Produce `openspec/changes/<change-id>/design-lens-a.md`: the technical approach and every architecture decision for this change, each with its choice, the alternatives rejected, the rationale, and the terminal consumer that makes the decision observable.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `<change-id>` are accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another. This lens owns the architectural choice; the other two consume it.

## Preconditions

- `openspec/changes/<change-id>/proposal.md` exists and is accepted.
- `openspec/changes/<change-id>/specs/` exists (or the packet `## Context` states it does not).
- `openspec/changes/<change-id>/design-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to structure, not to signatures or tests:

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/proposal.md` and `openspec/changes/<change-id>/specs/`.
3. The entry points and module structure of the packages the change lands in.
4. The existing patterns and conventions those packages already follow — how comparable problems were already solved in this repository.
5. `openspec/changes/archive/` for a prior change that solved a structurally similar problem, if one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/design-lens-a.md`:

```markdown
# Design Lens A — Decisions: <Change Title>

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

`openspec/changes/<change-id>/design-lens-a.md` only. Create no other file.

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
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft -- never copy this placeholder row verbatim> |

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

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice. It is a deterministic script, not a judge:
it reports whether these facts hold; it does not decide whether your draft is good, and it does
not replace your own judgment against `## Done criteria` below.

**Before you commit**, while content is still cheap to fix:

```
./lucind-lane-check.sh --file openspec/changes/<change-id>/design-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" --require-section "Assumed architecture" \
  --require-section "Technical Approach" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

`--verify-citations` is an existence/grep-level check over your own `## Citation Manifest`: does
the cited file exist, does it have enough lines to contain the cited range. It asserts nothing
about whether the range supports your claim — the synthesizer still opens and checks every
citation itself in the next phase. A FAIL here is cheaper to fix now than after synthesis catches
it.

**After you commit and write `.lucind/result.json`**, confirm the bookkeeping:

```
./lucind-lane-check.sh --file openspec/changes/<change-id>/design-lens-a.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every decision names a terminal consumer with a `file:line` citation**, and that citation points at real code in this worktree.
- [ ] **`design-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section including `## Assumed architecture`, plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final
      `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.
      Check `git log -1 --format=%B` after committing: some executors' commit wrappers append a
      `Co-authored-by:` trailer the message never contained. Strip it if present.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposal or specs do not determine an architectural choice, and two reasonable shapes are equally supported.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Making this decision would require designing the file-change surface or the test strategy, which this packet forbids.

## Context

<Ground-truth facts with file:line references: the packages involved, the
relevant existing types, the accepted proposal summary, and any decision the
human has already made in conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
