---
id: design-deterministic-lucind-ai-orchestrator-lens-b
executor: agy
routed_by: surface-and-flow lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-b.md"]
---

# Packet design-deterministic-lucind-ai-orchestrator-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/design-deterministic-lucind-ai-orchestrator-lens-b  ·  **Branch:** lucind/design-deterministic-lucind-ai-orchestrator-lens-b

## Goal

Produce `openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-b.md`: how data moves
through the change, the invariants that must hold at each hop, the exact signature and format
deltas the change introduces, and the file-change table with a terminal consumer per row.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final
design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `deterministic-lucind-ai-orchestrator` are accepted and frozen. Lens A
and lens C run in parallel against the same frozen inputs and write to different files, so no lane
races another.

Lens A owns the architecture decision and is running concurrently, so you do not have it. Declare
the architecture you are assuming in `## Assumed architecture` and design against it consistently.
The synthesizer arbitrates divergence; a silent second architecture does not survive that
arbitration.

## Preconditions

- `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md` exists and is accepted.
- `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` exists with five capability deltas.
- `openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to surfaces and formats, not to
rationale or tests:

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md` and
   `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` (all five capability files).
3. The exact type, struct, and interface declarations in `internal/packet`, `internal/dag`,
   `internal/run`, `internal/ledger`, `internal/accept`, and `internal/worktree` — read the
   declarations, not summaries of them.
4. Every persisted or wire format in scope: `.lucind/result.schema.json`, packet frontmatter
   parsing, ledger rows, result envelopes, `apply-dag.yaml`.
5. The CLI flag and argument surface in `cmd/lucind-ai/cli.go` for anything the change exposes to
   an operator.

Never guess at a signature. Every row in your tables carries a `file:line` citation to real code in
this worktree.

## Output format

Write exactly this skeleton to
`openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-b.md`:

```markdown
# Design Lens B — Surface & Flow: Deterministic lucind-ai Orchestrator

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

`design-lens-b.md` MUST be under 1000 words. Tables over prose. Do not restate code the reader can
open; cite it.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens A owns**: the technical approach, every architecture decision, the alternatives rejected,
  and the rationale.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity
  decision.

You will be tempted to argue for an architecture while tabulating its surface. Do not. If you
believe the architecture you assumed is the wrong one, say so under `## Open Questions` with the
evidence — do not quietly design a different one.

Do not assess whether the change is additively revertible. You supply the format deltas; lens C
decides rollback from them.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-b.md` only. Create no other
file.

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
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing a
whole `design.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the complete document, persist it to Engram, return the phase summary block, hold
an 800-word budget. Those are superseded here on purpose. Do not correct yourself toward them; note
the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every `Surface Deltas` and `File Changes` row carries a `file:line` citation that points at
  real code in this worktree**, and every `File Changes` row names a terminal consumer.
- [ ] **`design-lens-b.md` exists, is under 1000 words, and carries every skeleton section
  including `## Assumed architecture`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The specs do not determine whether a format delta is additive or breaking.
- A file change cannot name any terminal consumer.
- Two reasonable architectures produce incompatible surface tables and the proposal does not
  choose between them.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- Five accepted spec requirements, one per capability
  (`openspec/changes/deterministic-lucind-ai-orchestrator/specs/`):
  `deterministic-orchestrator-contract` → *Cross-Runtime Orchestrator Preflight and Sequencing*
  (ADDED); `packet-authoring-contract` → *Target-Free Packet Authoring and Late Binding* (ADDED);
  `acceptance-verifier` → *Frozen Evidence Acceptance Verification* (ADDED); `sdd-apply` →
  *Orchestrator Advances Only on a Passing Wave* (MODIFIED); `parent-feature-integration` →
  *Recoverable Idempotent Attempts* (MODIFIED).
- The proposal's `## Affected Areas` table (`proposal.md:36-42`) names exactly three surfaces:
  Claude skill/references and its OpenCode copy (Modified); CLI, packet/DAG, run/ledger/accept/
  worktree packages (Modified); `openspec/specs/` and tests (Modified).
- This Change's `base_sha` is `main` tip `705cf49`, 639 commits behind the unrelated, still
  in-flight `feature/skill-provisioning-and-phase-specialist` branch. `LUCIND_REQUIRED_SKILLS`
  env delivery, a `required_skills` packet frontmatter field, `integrate retry` as a CLI verb, and
  `defect record/list/resolve/decline/defer` subcommands all exist only on that other branch — none
  of them exist in `cmd/lucind-ai/cli.go` or `internal/` in this worktree. Verify every symbol,
  struct field, and CLI flag you cite actually resolves here before citing it; do not carry a
  surface delta forward from memory of a different branch.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
