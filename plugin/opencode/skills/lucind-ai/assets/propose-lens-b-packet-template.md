---
id: propose-<change-id>-lens-b
executor: agy
routed_by: capability impact and delta specs lens of the three-lens propose fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/propose-lens-b.md"]
---

# Packet propose-<change-id>-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/propose-<change-id>-lens-b  ·  **Branch:** lucind/propose-<change-id>-lens-b

## Goal

Produce `openspec/changes/<change-id>/propose-lens-b.md`: user capability impact, modified/added capabilities, and delta specification requirements and scenarios for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `<change-id>` is initiating. Lens A and lens C run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/<change-id>/` exists (and `explore.md` exists if explore was run).
- `openspec/changes/<change-id>/propose-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to capability impact and delta specs:

1. The real `gentle-ai` propose skill (delivered under `## Required skills`). It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. Existing delta and base specifications in `openspec/specs/`.
3. The interface declarations, CLI commands, and format definitions touched by this change.
4. `openspec/changes/archive/` for precedent delta spec structure in this repository.

Never guess at signatures or spec shapes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/propose-lens-b.md`:

```markdown
# Proposal Lens B — Capability Impact & Specs: <Change Title>

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|

## Delta Specifications

### Requirement: <Requirement Name>

<Requirement text using RFC 2119 keywords: MUST, SHALL, SHOULD, MAY.>

#### Scenario: <Scenario Name>

- GIVEN <initial condition>
- WHEN <trigger event>
- THEN <observable outcome>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`propose-lens-b.md` MUST be under 1000 words. Tables and structured delta specs over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write architecture rationale or rollback mechanisms here. They belong to lenses A and C.

## Allowed paths

`openspec/changes/<change-id>/propose-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` propose skill and its `references/` (delivered under
`## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the
capability impact table, and delta spec formatting conventions.
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
./lucind-lane-check.sh --file openspec/changes/<change-id>/propose-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "User & Capability Impact" --require-section "Delta Specifications" \
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
./lucind-lane-check.sh --file openspec/changes/<change-id>/propose-lens-b.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every capability impact and delta spec requirement carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final
      `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.
      Check `git log -1 --format=%B` after committing: some executors' commit wrappers append a
      `Co-authored-by:` trailer the message never contained. Strip it if present.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Delta requirements conflict with existing base specs without explicit migration path.
- Capability impacts cannot be determined from packet context or code inspection.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: existing specs,
interface declarations, capabilities, and any decision the human has already
made in conversation and does not want re-litigated.>

## Required skills

- <sdd-propose>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
