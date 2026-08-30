---
id: propose-conflict-triage-fixture-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel propose lenses into one canonical proposal document
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/proposal.md", "openspec/changes/conflict-triage-fixture/proposal-synthesis-notes.md"]
---

# Packet propose-conflict-triage-fixture-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/propose-conflict-triage-fixture-synthesis  ·  **Branch:** lucind/propose-conflict-triage-fixture-synthesis

## Goal

Read the three propose lens drafts for `conflict-triage-fixture`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/conflict-triage-fixture/proposal.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `propose-lens-a.md`, `propose-lens-b.md`, and `propose-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `propose-lens-a.md`, `propose-lens-b.md`, and `propose-lens-c.md` all exist in this worktree.
- `openspec/changes/conflict-triage-fixture/proposal.md` does not yet exist.

## What each lens owns

| Draft | Owns |
|---|---|
| `propose-lens-a.md` | Candidate selection; technical approach; conceptual changes; alternatives considered |
| `propose-lens-b.md` | User and capability impact table; delta specification requirements and scenarios |
| `propose-lens-c.md` | Technical risks and failure modes; rollback plan and additivity; test and validation impact; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `proposal.md`** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Candidate and scope arbitration

Compare the technical approaches and delta specs across drafts.

- Lens A's candidate selection and approach is authoritative.
- Any content in lens B or lens C that contradicts lens A's chosen candidate does not go into `proposal.md`. Record it under `## Scope Divergence` in the notes.
- If lens B or lens C converged independently on lens A's approach, record that corroboration in the notes.

### 4. Compress — do not concatenate

`proposal.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job: merge overlapping statements, drop restatement, keep the specific sentence over the general one. A concatenation of three drafts is a failed synthesis even if every word in it is true.

### 5. Coverage check

`proposal.md` must cover this repository's proposal spine:

1. Executive summary and problem statement
2. Selected candidate and proposed technical approach
3. Changes to system concepts and architecture rationale
4. User and capability impact table
5. Delta specifications (requirements and scenarios)
6. Technical risks and failure modes
7. Rollback plan and additivity
8. Test and validation impact
9. Out of scope and open questions

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/conflict-triage-fixture/proposal.md`

The canonical proposal. Under 1800 words. Covers the proposal spine. Contains only claims whose citations you verified in step 2 and which survive lens A's approach.

### `openspec/changes/conflict-triage-fixture/proposal-synthesis-notes.md`

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

## Scope Divergence

<What lens B or lens C assumed that differed from lens A, what content that cost
them, and where they converged independently. "None — all three converged" if
that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write specs, design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis lane.

## Allowed paths

`openspec/changes/conflict-triage-fixture/proposal.md` and `openspec/changes/conflict-triage-fixture/proposal-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill. Check the
canonical document against the contract as written.

This packet sets the 1800-word budget along with the synthesis procedure, the notes file, and the done criteria.

Write nothing outside this repository.

## Done criteria

- [ ] **Every `file:line` citation surviving into `proposal.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`proposal.md` exists, is under 1800 words, and substantively covers the proposal spine**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`proposal-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposed approaches across drafts are mutually irreconcilable. Write the notes file, leave `proposal.md` uncreated, and block.
- One or more lens drafts is missing from this worktree.
- Covering the proposal spine honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**Read `openspec/changes/conflict-triage-fixture/explore.md` first.** It is committed in this
worktree, it is the accepted exploration for this change, and it recommends Candidate 1. Its
`## Open questions` section is partly superseded by the decisions below — those are settled.
`explore-synthesis-notes.md` records seven citations the synthesizer opened and dropped; do not
resurrect a dropped citation.

**The human answered these four product questions. They are DECIDED. Do not re-litigate them, do
not present them as alternatives, and do not quietly widen them:**

1. **Verify-budget unit: wall clock plus the concrete command.** The agent declares verification
   cost as e.g. "~4 min: `./lucind-checks.sh` on the combined tree". Rejected on the record: test
   weight, and tokens/pricing. Reason: it is the only unit this binary can actually measure and
   the only one actionable without translation.
2. **Risk shape: three bands** — low / medium / high — with a business conflict pinned to `high`
   and the agent unable to lower it. Rejected: a 0-100 score (invents precision the discrete
   signals do not have) and a binary applicable/needs-human (loses the mechanical-but-large middle
   case).
3. **The fixture generator lives at `internal/conflicttriage/fixture/`**, beside the agent it
   exercises. Rejected: `test/fixture/`, and a public `lucind-ai fixture` CLI subcommand.
4. **A/B win criterion: correct classification of the three hunks** — the business hunk separated
   from the two mechanical controls, with arbitrariness declared where it belongs. Rejected:
   also grading the prepared resolution, and timing a human to thirty seconds.

**Two questions are deliberately still open and MUST stay open in the proposal.** Answering either
without the fixture's data is exactly the guess this change exists to avoid:

- the exact non-decreasing risk formula and its thresholds, including mixed business+mechanical
  hunks;
- which executor/model runs production triage (the judges are `opencode`/`openai/gpt-5.6-sol` and
  `claude`/`claude-opus-5`; the production runtime is a separate decision).

**Scope — two halves, built as two path-disjoint features:**

| feature | `parent_ref` | owns |
|---|---|---|
| `conflict-triage-agent` | `feature/conflict-triage-agent` | `internal/conflicttriage/**` |
| `conflict-fixture` | `feature/conflict-fixture` | `internal/conflicttriage/fixture/`, judge packets, rubric |

Disjoint on purpose: the deliberate `required` collision is a product of the fixture, engineered
and repeatable, never an accident of this repository's own delivery. Two packets naming different
features are refused in one batch (`internal/run/integrate_feature.go:17,41`), so these are two
separate dispatches.

**Ground truth that must not be re-derived** (verified this session; cite, do not re-investigate):

- Ledger counts from `.lucind/lucind.db`: 308 lanes, 8 features, 7 feature_leases, 36
  integration_attempts, and **0** each of overlap_evidence, reconciliation_requests and
  reconciliation_candidates. Multi-feature promotion is exercised; the overlap gate has never
  classified above `informational` in this clone.
- A `required` classification comes from `evaluateOverlapGate` (`internal/run/attempt.go:685`) via
  `overlap.Classify` (`internal/overlap/overlap.go:622`); thresholds at
  `internal/overlap/overlap.go:93`.
- `overlap_evidence` for `ClassRequired` is written inside `CreateRequest`
  (`internal/reconcile/reconcile.go:266`), NOT at `attempt.go:768-775` — those lines insert only on
  `ClassWarning`. The explore fan-out already caught and corrected this misattribution.
- An in-memory harness with no registered shared `base_sha` can never reach `ClassRequired`:
  `ErrNoMergeBase` makes `evaluateOverlapGate` `continue` (`internal/run/attempt.go:743-747`).
  Real features with a common base are mandatory; there is no shortcut.
- Clearing a required block takes two steps: `reconcile approve` only authorizes a candidate;
  a human resolves out of band and registers the commit with `reconcile resolve --candidate --sha`.
- `conflict-triage` deliberately does NOT fail closed, unlike `internal/resolve`
  (`internal/resolve/candidate.go:305`, `ErrSemanticAmbiguity` at `:26`). Keep the two disciplines
  distinct; a proposal that unifies them is wrong.
- The triage result's JSON shape is stored in `reconciliation_candidates.output`
  (`internal/reconcile/reconcile.go:105`).

**Out of scope, and a proposal that includes any of it is wrong:** wiring the five reconcile POST
routes (the web surface stays read-only), changing overlap thresholds, and touching production
dispatch paths.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
