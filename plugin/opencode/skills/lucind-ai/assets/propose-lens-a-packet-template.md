---
id: propose-<change-id>-lens-a
executor: agy
routed_by: candidate and approach lens of the three-lens propose fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/propose-lens-a.md"]
---

# Packet propose-<change-id>-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/propose-<change-id>-lens-a  ·  **Branch:** lucind/propose-<change-id>-lens-a

## Goal

Produce `openspec/changes/<change-id>/propose-lens-a.md`: the candidate selection, proposed technical approach, changes to system concepts, and architecture rationale for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `<change-id>` is initiating. Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns candidate selection and core approach; the other two explore capability specs and risk/rollback.

## Preconditions

- `openspec/changes/<change-id>/` exists (and `explore.md` exists if explore was run).
- `openspec/changes/<change-id>/propose-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to candidate selection and approach, not to spec delta authoring or test matrices:

1. The real `gentle-ai` propose skill (delivered under `## Required skills`). It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/explore.md` (if present).
3. The entry points and module structure of the packages relevant to the proposal.
4. The existing patterns and conventions those packages already follow — how comparable problems were already solved in this repository.
5. `openspec/changes/archive/` for prior proposals that addressed similar changes.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/propose-lens-a.md`:

```markdown
# Proposal Lens A — Candidate & Approach: <Change Title>

## Selected Candidate & Approach

<State the chosen candidate from exploration, summarize the core approach, and explain why this approach solves the problem. Cite file:line for existing code behavior.>

## Conceptual Changes & Architecture Rationale

<Describe additions, modifications, or deprecations to system concepts, interfaces, or architectural patterns. Cite file:line for existing concepts.>

## Alternatives Considered & Rejected

<What alternative approaches were considered during candidate selection and why they were rejected.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`propose-lens-a.md` MUST be under 1000 words. Approach descriptions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write delta spec requirements or a rollback plan here. They belong to lenses B and C.

## Allowed paths

`openspec/changes/<change-id>/propose-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` propose skill and its `references/` (delivered under
`## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the
candidate selection, approach, and conceptual change shape of a proposal.
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
./lucind-lane-check.sh --file openspec/changes/<change-id>/propose-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Selected Candidate & Approach" \
  --require-section "Conceptual Changes & Architecture Rationale" \
  --require-section "Alternatives Considered & Rejected" --require-section "Open Questions" \
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
./lucind-lane-check.sh --file openspec/changes/<change-id>/propose-lens-a.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every candidate and approach claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final
      `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.
      Check `git log -1 --format=%B` after committing: some executors' commit wrappers append a
      `Co-authored-by:` trailer the message never contained. Strip it if present.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The technical approach cannot be grounded in existing codebase patterns or proposal context.
- Candidate selection contradicts frozen exploration conclusions without justification.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the packages involved, the
relevant existing types, the change goals, and any decision the
human has already made in conversation and does not want re-litigated.>

## Required skills

- <sdd-propose>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
