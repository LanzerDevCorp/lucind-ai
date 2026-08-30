---
id: design-deterministic-lucind-ai-orchestrator-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel design lenses into one canonical design
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/design.md", "openspec/changes/deterministic-lucind-ai-orchestrator/design-synthesis-notes.md"]
---

# Packet design-deterministic-lucind-ai-orchestrator-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/design-deterministic-lucind-ai-orchestrator-synthesis  ·  **Branch:** lucind/design-deterministic-lucind-ai-orchestrator-synthesis

## Goal

Read the three design lens drafts for `deterministic-lucind-ai-orchestrator`, verify their claims
against the real code, arbitrate where they disagree, and produce one canonical
`openspec/changes/deterministic-lucind-ai-orchestrator/design.md` plus a separate synthesis notes
file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from
the integrated result, so `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` are all
present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` all exist in this worktree.
- `openspec/changes/deterministic-lucind-ai-orchestrator/design.md` does not yet exist.
- The proposal and specs for `deterministic-lucind-ai-orchestrator` are present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `design-lens-a.md` | Technical approach; every architecture decision except rollback, with alternatives and rationale |
| `design-lens-b.md` | Flow and invariants; surface deltas (types, schemas, frontmatter, CLI); file changes |
| `design-lens-c.md` | Testing strategy and test seams; threat matrix; rollback and additivity; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it
reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this
worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim:
  **drop the claim from `design.md`** and record it under `## Dropped Citations` in the notes with
  what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

**One specific thing to re-verify:** this Change's `base_sha` is `main` tip `705cf49`, 639 commits
behind the unrelated, still in-flight `feature/skill-provisioning-and-phase-specialist` branch.
That other branch has `LUCIND_REQUIRED_SKILLS` env delivery, a `required_skills` packet frontmatter
field, `integrate retry` as a CLI verb, and `defect record/list/resolve/decline/defer` subcommands
— none of which exist in this worktree. If any lens draft cites a symbol, flag, or subcommand that
does not actually resolve in this worktree's `cmd/lucind-ai/cli.go` or `internal/`, drop it as a
dropped citation; do not silently "fix" it by assuming the other branch's shape.

### 3. Architecture arbitration

The three drafts each opened with `## Assumed architecture`. Compare them.

- **Lens A's assumed architecture is authoritative.** It is the lens that owned the decision.
- Any content in lens B or lens C that does not survive lens A's architecture does not go into
  `design.md`. Record it under `## Architecture Divergence` in the notes, with what B or C assumed
  instead.
- If lens B or lens C converged independently on lens A's architecture, say so in the notes.
  Independent convergence is corroboration and is worth recording.
- If lens A's own architecture is refuted by code you verified in step 2, do not silently
  substitute your own. That is a hard stop.

### 4. Compress — do not concatenate

`design.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job:
merge overlapping statements, drop restatement, keep the specific sentence over the general one. A
concatenation of three drafts is a failed synthesis even if every word in it is true.

### 5. Coverage check

`design.md` must cover this repository's actual design spine, derived from every archived design
in `openspec/changes/archive/`:

1. Technical approach or recommendations at a glance
2. Architecture decisions, each with choice / alternatives considered / rationale
3. Flow and invariants
4. File changes, with terminal consumers
5. Testing strategy and test seams
6. Threat matrix — every row `Applicable` or `N/A: reason`
7. Rollback and additivity
8. Open questions and out of scope

Section headings may follow the change's own vocabulary — archived designs vary — but every one of
the eight must be substantively present. Anything no draft covered goes under `## Coverage Gaps`
in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/deterministic-lucind-ai-orchestrator/design.md`

The canonical design. Under 1800 words. Covers all eight spine items. Contains only claims whose
citations you verified in step 2 and which survive lens A's architecture.

### `openspec/changes/deterministic-lucind-ai-orchestrator/design-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: Deterministic lucind-ai Orchestrator

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it.
State both positions and what evidence each has. Do NOT pick — this section is
the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
actually says. "None" if there are none.>

## Architecture Divergence

<What lens B or lens C assumed that differed from lens A, what content that cost
them, and where they converged independently. "None — all three converged" if
that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write specs, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/design.md` and
`openspec/changes/deterministic-lucind-ai-orchestrator/design-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill. Check the
canonical document against the contract as written: its required sections, the choice /
alternatives / rationale shape of a decision, and the threat-matrix applicability rule. On those,
the skill wins over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word budget — the skill's nominal 800 is
not honored in this repository, as `openspec/changes/archive/` shows — along with the synthesis
procedure, the notes file, and the done criteria. The skill's Step 4 Engram persistence and Step 5
return block are superseded: your output is the two files named above plus `.lucind/result.json`.

Write nothing outside this repository.

## Done criteria

- [ ] **Every `file:line` citation surviving into `design.md` was opened and confirmed in this
  worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`design.md` exists, is under 1800 words, and substantively covers all eight spine items**,
  with anything missing reported under `## Coverage Gaps`.
- [ ] **`design-synthesis-notes.md` exists with exactly the four required sections**, each either
  populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).
- [ ] **No cited symbol, flag, or subcommand belongs only to the unrelated
  `feature/skill-provisioning-and-phase-specialist` branch** — every citation resolves in this
  worktree.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The three `## Assumed architecture` blocks are mutually irreconcilable and the proposal and
  specs do not choose between them. Write the notes file, leave `design.md` uncreated, and block.
- Lens A's architecture is refuted by code you verified. Do not substitute your own.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words. Report which item
  forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**The change**: `deterministic-lucind-ai-orchestrator` — a two-layer contract (canonical Claude
skill/reference state machine plus machine-observable CLI/runtime invariants) making SDD execution
reproducible across Claude Code and OpenCode.

**Accepted proposal summary** (`proposal.md`): New capability
`deterministic-orchestrator-contract`; Modified `sdd-apply`, `parent-feature-integration`. New
(not Modified, verified against this Change's `main` base): `packet-authoring-contract`,
`acceptance-verifier`. Five accepted spec requirements exist under
`openspec/changes/deterministic-lucind-ai-orchestrator/specs/`, one per capability.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`); executor/model/provider/profile selection and semantic approval/promotion
remain human-owned (`proposal.md:16`); Rollback Plan (`proposal.md:51-53`) commits to reverting
skill/parity/runtime commits independently while retaining existing packet, ledger, lifecycle, and
CAS behavior, never migrating or rewriting prior evidence.

This Change is deliberately isolated from `feature/skill-provisioning-and-phase-specialist`
(639 commits ahead of this Change's `main` base) — the human explicitly asked for the two Changes
to stay on separate isolated feature branches. Do not let `design.md` describe or depend on that
branch's unmerged runtime mechanisms.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter:
citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only
when every done-criterion carries evidence and every hard stop is declared.
