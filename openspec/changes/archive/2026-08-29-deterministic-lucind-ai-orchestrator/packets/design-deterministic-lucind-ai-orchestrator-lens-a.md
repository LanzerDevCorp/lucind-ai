---
id: design-deterministic-lucind-ai-orchestrator-lens-a
executor: agy
routed_by: decisions lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-a.md"]
---

# Packet design-deterministic-lucind-ai-orchestrator-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/design-deterministic-lucind-ai-orchestrator-lens-a  ·  **Branch:** lucind/design-deterministic-lucind-ai-orchestrator-lens-a

## Goal

Produce `openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-a.md`: the technical
approach and every architecture decision for this change, each with its choice, the alternatives
rejected, the rationale, and the terminal consumer that makes the decision observable.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final
design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `deterministic-lucind-ai-orchestrator` are accepted and frozen. Lens B
and lens C run in parallel against the same frozen inputs and write to different files, so no lane
races another. This lens owns the architectural choice; the other two consume it.

## Preconditions

- `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md` exists and is accepted.
- `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` exists with five capability deltas.
- `openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to structure, not to signatures or
tests:

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md` and
   `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` (all five capability files).
3. The entry points and module structure of `cmd/lucind-ai`, `internal/packet`, `internal/run`,
   `internal/dag`, `internal/ledger`, `internal/accept`, and `internal/worktree`.
4. The existing patterns and conventions those packages already follow — how comparable problems
   were already solved in this repository.
5. `openspec/changes/archive/` for a prior change that solved a structurally similar problem, if
   one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to
`openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-a.md`:

```markdown
# Design Lens A — Decisions: Deterministic lucind-ai Orchestrator

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

`design-lens-a.md` MUST be under 1000 words. Decisions as compact blocks, not essays. Code
snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens B owns**: the file-changes table, data-flow diagrams, invariants, and every exact
  type/schema/CLI signature delta.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity
  decision.

Do not write a rollback decision here even though it is shaped like a decision. It belongs to
lens C.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/design-lens-a.md` only. Create no other
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

- [ ] **Every decision names a terminal consumer with a `file:line` citation**, and that citation
  points at real code in this worktree.
- [ ] **`design-lens-a.md` exists, is under 1000 words, and carries every skeleton section
  including `## Assumed architecture`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The proposal or specs do not determine an architectural choice, and two reasonable shapes are
  equally supported.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Making this decision would require designing the file-change surface or the test strategy, which
  this packet forbids.

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
- The proposal's `## Approach` section (`proposal.md:30-34`) already names the two-layer split:
  a prompt/reference layer authored only in `plugin/claude-code/skills/lucind-ai/` (with
  `plugin/opencode/skills/lucind-ai/` as a byte-identical, parity-verified copy — never a second
  contract), and a runtime layer of narrow enforcement/reporting added only at existing
  `cmd/lucind-ai`, `internal/packet`, `internal/run`, `internal/dag`, `internal/ledger`,
  `internal/accept`, and `internal/worktree` boundaries.
- The explore doc (`openspec/changes/deterministic-lucind-ai-orchestrator/explore.md`) already
  cites concrete existing seams to extend rather than replace: candidate/evidence persistence at
  `internal/run/run.go:608-665,1004-1019`; acceptance revalidation at
  `internal/accept/accept.go:213-341`; deterministic wave computation and overlap rejection at
  `internal/dag/waves.go:11-18,43-66` and `internal/dag/overlap.go:10-15,52-67`; CAS promotion and
  no-redispatch retry at `internal/run/integrate_feature.go:13-48` and
  `internal/run/integrate_retry.go:16-43`.
- This Change's `base_sha` is `main` tip `705cf49` — 639 commits behind the unrelated, still
  in-flight `feature/skill-provisioning-and-phase-specialist` branch. Do not design against
  capabilities, CLI subcommands, or runtime mechanisms (e.g. `LUCIND_REQUIRED_SKILLS`,
  `integrate retry` as a CLI verb, `defect record/list/resolve`) that exist only on that other
  branch and not in this worktree's actual source — verify every cited symbol resolves here before
  citing it.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`); executor/model/provider/profile selection and semantic approval/promotion
remain human-owned (`proposal.md:16`); no cross-fork coordination, global state, automatic main
promotion, or redesign of unrelated SDD flows (`proposal.md:17`).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
