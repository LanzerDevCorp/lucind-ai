---
id: explore-conflict-triage-fixture-lens-a
executor: agy
routed_by: problem and candidates lens of the three-lens explore fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/explore-lens-a.md"]
---

# Packet explore-conflict-triage-fixture-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/explore-conflict-triage-fixture-lens-a  ·  **Branch:** lucind/explore-conflict-triage-fixture-lens-a

## Goal

Produce `openspec/changes/conflict-triage-fixture/explore-lens-a.md`: the problem space definition, motivation, background, and candidate approaches for this change, each with its description, pros, cons, and feasibility assessment.

This is one of three parallel explore lenses. It is feedstock for a synthesis lane, not the final explore document. Do not write a complete `explore.md`.

## Why this is safe to dispatch now

The exploration for `conflict-triage-fixture` is initiating. Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns the problem definition and candidate solutions; the other two explore capabilities and risks.

## Preconditions

- `openspec/changes/conflict-triage-fixture/` exists (or the packet `## Context` states the change goal).
- `openspec/changes/conflict-triage-fixture/explore-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to problem space and candidates, not to scenarios or risk matrices:

1. `~/.claude/skills/sdd-explore/SKILL.md` — the real `gentle-ai` explore skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. The entry points and module structure of the packages relevant to the problem space.
3. The existing patterns and conventions those packages already follow — how comparable problems were already solved in this repository.
4. `openspec/changes/archive/` for prior explorations or changes that addressed similar problem spaces, if one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/conflict-triage-fixture/explore-lens-a.md`:

```markdown
# Explore Lens A — Problem & Candidates: Conflict Triage Fixture

## Problem Space

<Concise description of the problem, background, current limitations, and motivation for the change. Cite file:line for existing code behavior.>

## Candidate Approaches

### Candidate 1 — <title>

**Approach**: <summary of candidate approach>
**Pros**: <advantages>
**Cons**: <disadvantages and costs>
**Feasibility**: <assessment grounded in this codebase with file:line citations>

### Candidate N — <title>

<same four fields>

## Initial Recommendations

<Preliminary recommendation among candidates, with technical rationale.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`explore-lens-a.md` MUST be under 1000 words. Candidate descriptions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: capabilities, user scenarios, and success criteria.
- **Lens C owns**: technical risks, unknowns, trade-offs matrix, and potential spikes/proof-of-concepts.

Do not write a risks matrix or detailed user scenarios here. They belong to lenses B and C.

## Allowed paths

`openspec/changes/conflict-triage-fixture/explore-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what an explore document must contain*: its required sections, the
problem / candidates / feasibility shape of exploration, and the exploration taxonomy.
Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the exploration is split
across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
a whole `explore.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the complete document, persist it to Engram, return the phase summary block.
Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in
`## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every candidate and problem claim carries `file:line` citations to real code in this worktree.**
- [ ] **`explore-lens-a.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The problem scope cannot be identified from codebase inspection or packet context.
- Candidate approaches cannot be formulated without designing complete implementation details (which belongs to design phase).
- Satisfying one instruction in this packet would require violating another.

## Context

Ground truth verified in this session by direct inspection. Do NOT re-derive any of it. Cite it,
and spend your lane's budget on what is still unknown.

**What this change builds — two halves, one change.**

1. `internal/conflicttriage/` — the `conflict-triage` agent. Designed, never implemented.
2. A reproducible conflict fixture that makes a `required`-class overlap happen on demand.

**Ledger facts, queried directly from `.lucind/lucind.db` this session:**

| table | rows |
|---|---|
| `lanes` | 308 |
| `features` | 8 |
| `feature_leases` | 7 |
| `integration_attempts` | 36 |
| `overlap_evidence` | **0** |
| `reconciliation_requests` | **0** |
| `reconciliation_candidates` | **0** |

Read that carefully, because it is the whole motivation. Multi-feature promotion, leases and CAS
have run 36 times in this ledger and are exercised. The overlap gate has **never** classified
anything above `informational` here — not one `overlap_evidence` row — and no reconciliation
request has ever existed. The reconcile half of this binary is production-wired and has never
fired in this clone.

**Where a reconciliation request actually comes from.** `evaluateOverlapGate`
(`internal/run/attempt.go:685`) compares an attempt's candidate against every other active feature
in the same ledger and classifies via `overlap.Classify` (`internal/overlap/overlap.go:622`).
`ClassRequired` triggers: predicted git merge conflict (merge-tree), rename/delete collision,
shared binary, intersecting hunks, nearby hunks within threshold, or hotspot weight >= 0.50
(`DefaultThresholds`, `internal/overlap/overlap.go:93`). Two plain git branches that conflict
produce nothing in the ledger — the feature layer is mandatory.

**Clearing a required block takes two steps, not one.** `reconcile approve` only authorizes a
candidate; it resolves no text. A human resolves out of band, produces a real commit, and
registers it with `lucind-ai reconcile resolve --candidate <id> --sha <sha>`; the blocked
feature's retry then promotes that registered SHA. A blocked lane's worktree is preserved, so
re-dispatching the same packet id fails until `lucind-ai worktree cleanup --lane <id>`.

**Multi-feature admission refuses four things before any lane dispatches**
(`internal/run/integrate_feature.go`): two packets naming different features in one batch; legacy
and feature-targeted packets mixed; the same feature with divergent `expected_parent_sha`; and a
`parent_ref` that is `main`, inside the `lucind/` lane namespace, or empty.

**Decomposition already decided by the human — do not re-litigate it.** Two path-disjoint
features, built as two separate `lucind-ai run` invocations:

| feature | `parent_ref` | owns |
|---|---|---|
| `conflict-triage-agent` | `feature/conflict-triage-agent` | `internal/conflicttriage/**` |
| `conflict-fixture` | `feature/conflict-fixture` | fixture generator, judge packets, rubric |

They are disjoint **on purpose**: the deliberate `required` collision is a product of the fixture,
engineered in a controlled place and repeatable, never an accident of this repository's own
delivery. Proposing that the two build features collide with each other is out of scope; that
option was raised and rejected.

**The `conflict-triage` design is settled (Engram #1111) — treat as given:**

- Primary objective: turn a merge conflict into a decision a human can make in thirty seconds —
  explain the real cause, leave a prepared resolution, and honestly declare what is risked by
  accepting it unverified and what verifying costs.
- Explain the CAUSE, not the differing lines ("lane-A moved validation to middleware, lane-B
  duplicated it inline" — never "conflict at auth.go:40").
- Always propose an exit and leave it prepared as a commit.
- When the choice is BUSINESS and not technical: say so explicitly, state that the proposal is
  ARBITRARY and why that side was picked, and ratchet risk to high — the agent may not lower it.
- It deliberately does NOT fail closed, unlike the existing resolver
  (`internal/resolve/candidate.go:305`, `ErrSemanticAmbiguity` at `:26`, which MUST NOT choose
  direction or invent business semantics). Keep both disciplines distinct; do not unify them.
- STILL OPEN, and the fixture exists to answer them with data: the exact risk formula and its
  thresholds, how the verify budget is estimated, and which executor/model runs it.

**The fixture's intended collision shape** (validate or challenge it — do not merely ratify):
two features editing one toy file, producing three conflicting hunks — one BUSINESS hunk where
both versions compile and pass their own tests and encode different product rules (no technical
criterion exists to choose), plus two mechanical controls (a slice-literal union, and a rename
colliding with an edit to the old name). The business hunk is the measurement; the mechanical
hunks are the control. A judge that treats all three alike fails, in either direction.

**Executors.** There are now four, each pinned to exactly one provider family as an
anti-cross-billing invariant: `agy`=gemini-3.7-flash-high, `cursor-agent`=cursor-grok-4.6-high,
`opencode`=openai/gpt-5.6-sol, `claude`=claude-opus-5. All four emit `lane_progress`. Note the
skill's own frontmatter table (`plugin/claude-code/skills/lucind-ai/SKILL.md`) still lists only
three and omits `claude`; the code (`cmd/lucind-ai/cli.go:66`) is authoritative.

**Explicitly out of bounds for this change:** wiring the five reconcile POST routes into the web
UI (the surface stays read-only), changing overlap thresholds, and touching production dispatch
paths.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
