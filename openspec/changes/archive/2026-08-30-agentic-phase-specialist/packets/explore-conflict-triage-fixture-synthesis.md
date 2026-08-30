---
id: explore-conflict-triage-fixture-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel explore lenses into one canonical explore document
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/explore.md", "openspec/changes/conflict-triage-fixture/explore-synthesis-notes.md"]
---

# Packet explore-conflict-triage-fixture-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/explore-conflict-triage-fixture-synthesis  ·  **Branch:** lucind/explore-conflict-triage-fixture-synthesis

## Goal

Read the three explore lens drafts for `conflict-triage-fixture`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/conflict-triage-fixture/explore.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `explore-lens-a.md`, `explore-lens-b.md`, and `explore-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `explore-lens-a.md`, `explore-lens-b.md`, and `explore-lens-c.md` all exist in this worktree.
- `openspec/changes/conflict-triage-fixture/explore.md` does not yet exist.

## What each lens owns

| Draft | Owns |
|---|---|
| `explore-lens-a.md` | Problem space; candidate approaches; initial recommendations |
| `explore-lens-b.md` | User and capability impact; scenarios and use cases; success criteria |
| `explore-lens-c.md` | Technical risks and unknowns; trade-offs matrix; potential spikes; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `explore.md`** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Approach and problem arbitration

Compare the problem statements and candidate feasibility across drafts.

- Lens A's problem and candidate analysis is primary.
- If lens B scenarios or lens C risks reveal unviable candidates, document the arbitration in synthesis notes.
- If all three converged independently on problem boundaries and approach viability, record that corroboration under `## Approach Divergence` in the notes.

### 4. Compress — do not concatenate

`explore.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job: merge overlapping statements, drop restatement, keep the specific sentence over the general one. A concatenation of three drafts is a failed synthesis even if every word in it is true.

### 5. Coverage check

`explore.md` must cover this repository's exploration spine:

1. Problem statement and background
2. Candidate approaches (pros, cons, feasibility)
3. User & capability impact
4. Scenarios and use cases
5. Technical risks and trade-offs matrix
6. Potential spikes or proof-of-concepts
7. Success criteria
8. Out of scope and open questions

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/conflict-triage-fixture/explore.md`

The canonical exploration document. Under 1800 words. Covers the exploration spine. Contains only claims whose citations you verified in step 2.

### `openspec/changes/conflict-triage-fixture/explore-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it.
State both positions and what evidence each has. Do NOT pick — this section is
the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
actually says. "None" if there are none.>

## Approach Divergence

<What lens B or lens C assumed that differed from lens A, what content that cost
them, and where they converged independently. "None — all three converged" if
that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write proposal, specs, design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis lane.

## Allowed paths

`openspec/changes/conflict-triage-fixture/explore.md` and `openspec/changes/conflict-triage-fixture/explore-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill. Check the
canonical document against the contract as written.

This packet sets the 1800-word budget along with the synthesis procedure, the notes file, and the done criteria.

Write nothing outside this repository.

## Done criteria

- [ ] **Every `file:line` citation surviving into `explore.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`explore.md` exists, is under 1800 words, and substantively covers the exploration spine**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`explore-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The problem space or candidate approaches across drafts are mutually irreconcilable. Write the notes file, leave `explore.md` uncreated, and block.
- One or more lens drafts is missing from this worktree.
- Covering the exploration spine honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
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

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
