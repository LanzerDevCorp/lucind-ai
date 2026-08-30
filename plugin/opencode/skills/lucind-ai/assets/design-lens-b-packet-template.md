---
id: design-<change-id>-lens-b
executor: agy
routed_by: surface-and-flow lens of the three-lens design fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/design-lens-b.md"]
---

# Packet design-<change-id>-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/design-<change-id>-lens-b  ·  **Branch:** lucind/design-<change-id>-lens-b

## Goal

Produce `openspec/changes/<change-id>/design-lens-b.md`: how data moves through the change, the invariants that must hold at each hop, the exact signature and format deltas the change introduces, and the file-change table with a terminal consumer per row.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `<change-id>` are accepted and frozen. Lens A and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the architecture decision and is running concurrently, so you do not have it. Declare the architecture you are assuming in `## Assumed architecture` and design against it consistently. The synthesizer arbitrates divergence; a silent second architecture does not survive that arbitration.

## Preconditions

- `openspec/changes/<change-id>/proposal.md` exists and is accepted.
- `openspec/changes/<change-id>/specs/` exists (or the packet `## Context` states it does not).
- `openspec/changes/<change-id>/design-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to surfaces and formats, not to rationale or tests:

1. The real `gentle-ai` design skill (delivered under `## Required skills`). It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/proposal.md` and `openspec/changes/<change-id>/specs/`.
3. The exact type, struct, and interface declarations the change touches — read the declarations, not summaries of them.
4. Every persisted or wire format in scope: JSON schemas, YAML sidecar structs, packet frontmatter parsing, ledger rows, result envelopes.
5. The CLI flag and argument surface in `cmd/` for anything the change exposes to an operator.

Never guess at a signature. Every row in your tables carries a `file:line` citation to real code in this worktree.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/design-lens-b.md`:

```markdown
# Design Lens B — Surface & Flow: <Change Title>

## Assumed architecture

<2–4 sentences naming the structural shape you are designing against: which
existing types or packages get extended, which are new. Lens A and lens C write
this same block independently; the synthesizer compares all three. Be specific
enough that a disagreement is visible.>

## Flow and Invariants

<How data moves for this change. A simple ASCII diagram when it clarifies —
clarity over beauty. Then the invariant that must hold at each hop, and what
observably breaks if it does not.>

    Component A ──→ Component B ──→ Component C

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|

<One row per type, field, schema property, frontmatter key, or CLI flag the
change adds, changes, or removes. "Backward compatible?" must be yes/no with a
one-clause reason, never blank.>

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|

<Create / Modify / Delete. The terminal consumer column names the symbol,
command, or spec requirement that reaches this file's change — with file:line
where it already exists.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-b.md` MUST be under 1000 words. Tables over prose. Do not restate code the reader can open; cite it.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: the technical approach, every architecture decision, the alternatives rejected, and the rationale.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity decision.

You will be tempted to argue for an architecture while tabulating its surface. Do not. If you believe the architecture you assumed is the wrong one, say so under `## Open Questions` with the evidence — do not quietly design a different one.

Do not assess whether the change is additively revertible. You supply the format deltas; lens C decides rollback from them.

## Allowed paths

`openspec/changes/<change-id>/design-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` design skill and its `references/` (delivered under
`## Required skills`). Read the contract as written, not as this packet paraphrases it.

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
./lucind-lane-check.sh --file openspec/changes/<change-id>/design-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed architecture" --require-section "Flow and Invariants" \
  --require-section "Surface Deltas" --require-section "File Changes" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

`--verify-citations` is an existence/grep-level check over your own `## Citation Manifest`: does
the cited file exist, does it have enough lines to contain the cited range. It asserts nothing
about whether the range supports your claim — the synthesizer still opens and checks every
citation itself in the next phase. A FAIL here is cheaper to fix now than after synthesis catches
it.

**After you commit and write `.lucind/result.json`**, confirm the bookkeeping:

```
./lucind-lane-check.sh --file openspec/changes/<change-id>/design-lens-b.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every `Surface Deltas` and `File Changes` row carries a `file:line` citation that points at real code in this worktree**, and every `File Changes` row names a terminal consumer.
- [ ] **`design-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section including `## Assumed architecture`, plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final
      `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.
      Check `git log -1 --format=%B` after committing: some executors' commit wrappers append a
      `Co-authored-by:` trailer the message never contained. Strip it if present.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The specs do not determine whether a format delta is additive or breaking.
- A file change cannot name any terminal consumer.
- Two reasonable architectures produce incompatible surface tables and the proposal does not choose between them.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the exact declarations in scope,
the schema files, the persisted formats and their current versions, and any
decision the human has already made in conversation and does not want
re-litigated.>

## Required skills

- <sdd-design>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
