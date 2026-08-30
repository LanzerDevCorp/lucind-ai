---
id: propose-conflict-triage-fixture-lens-c
executor: agy
routed_by: risks, rollback, and test impact lens of the three-lens propose fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/propose-lens-c.md"]
---

# Packet propose-conflict-triage-fixture-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-conflict-triage-fixture-lens-c  ·  **Branch:** lucind/propose-conflict-triage-fixture-lens-c

## Goal

Produce `openspec/changes/conflict-triage-fixture/propose-lens-c.md`: risk assessment, rollback strategy, additivity, and test impact for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `conflict-triage-fixture` is initiating. Lens A and lens B run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/conflict-triage-fixture/` exists (and `explore.md` exists if explore was run).
- `openspec/changes/conflict-triage-fixture/propose-lens-c.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to risks, rollback, and test impact:

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. Existing test suites, failure modes, error paths, and regression test patterns.
3. Wire and persisted formats, database/ledger schemas, result envelopes.
4. `openspec/changes/archive/` for rollback plans and test strategy precedents.

Never guess at seams or failure modes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/conflict-triage-fixture/propose-lens-c.md`:

```markdown
# Proposal Lens C — Risks, Rollback & Test Impact: Conflict Triage Fixture

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|

## Rollback & Additivity

**Rollback Plan**: <exact mechanism for reversal, git revert vs schema rollback>
**Additivity**: <state explicitly whether formats, schemas, or ledgers change additively or destructively, citing file:line>

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|

## Out of Scope

<Work explicitly excluded from this proposal.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`propose-lens-c.md` MUST be under 1000 words. Tables over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.

Rollback and test impact are yours. Conceptual design and delta spec requirements belong to lenses A and B.

## Allowed paths

`openspec/changes/conflict-triage-fixture/propose-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the
risk assessment, rollback plan, and test impact shape of a proposal.
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

## Done criteria

- [ ] **Every risk, test seam, and rollback claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-c.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Whether a schema or format change is additive cannot be determined, making rollback a guess.
- A critical failure mode has no identifiable mitigation or test seam.
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

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
