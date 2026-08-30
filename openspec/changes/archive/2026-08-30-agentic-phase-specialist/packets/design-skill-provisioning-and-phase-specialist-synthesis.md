---
id: design-skill-provisioning-and-phase-specialist-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel design lenses into one canonical design
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/design.md", "openspec/changes/skill-provisioning-and-phase-specialist/design-synthesis-notes.md"]
---

# Packet design-skill-provisioning-and-phase-specialist-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/design-skill-provisioning-and-phase-specialist-synthesis  ·  **Branch:** lucind/design-skill-provisioning-and-phase-specialist-synthesis

## Goal

Read the three design lens drafts for `skill-provisioning-and-phase-specialist`, verify their claims
against the real code, arbitrate where they disagree, and produce one canonical
`openspec/changes/skill-provisioning-and-phase-specialist/design.md` plus a separate synthesis notes
file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from
the integrated result, so `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` are all
present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/design-lens-a.md`, `-lens-b.md`, and
  `-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-provisioning-and-phase-specialist/design.md` does not yet exist.
- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` and
  `proposal-synthesis-notes.md` are present and accepted.

## What each lens owns

This fan-out used the standard aspect split, not a capability split.

| Draft | Owns |
|---|---|
| `design-lens-a.md` | Technical approach; every architecture decision — ad-hoc authoring surface shape, skill-budget default and override, specialist CLI shape, `internal/skillset.Derive` function shape, `internal/skillroots`/`lucind.yaml` loading — with alternatives and rationale |
| `design-lens-b.md` | Flow and invariants; `lane_role` closed-set values and frontmatter wiring; `## Required skills` rendered body format; `lucind.yaml`/`.lucind/skill-roots.yaml` file naming; surface deltas (types, schemas, frontmatter, CLI); file changes |
| `design-lens-c.md` | Missing archive/ultrafixer child skills decision; testing strategy and test seams; threat matrix; rollback and additivity; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them — but read this packet's `##
Context` section first: two of the three open questions raised independently by lens B and lens C
are already resolved by evidence this packet cites, and must not be carried into `design.md` as
still-open.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it
reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this
worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop
  the claim from `design.md`** and record it under `## Dropped Citations` in the notes with what you
  found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Architecture arbitration

The three drafts each opened with `## Assumed architecture`. Compare them.

- **Lens A's assumed architecture is authoritative.** It is the lens that owned the decision.
- Any content in lens B or lens C that does not survive lens A's architecture does not go into
  `design.md`. Record it under `## Architecture Divergence` in the notes, with what B or C assumed
  instead.
- If lens B or lens C converged independently on lens A's architecture, say so in the notes.
  Independent convergence is corroboration and is worth recording.
- If lens A's own architecture is refuted by code you verified in step 2, do not silently substitute
  your own. That is a hard stop.

### 4. Resolve the two carried-forward open items

Neither of these should survive into `design.md`'s own `## Open Questions` in the form the lens
drafts left them.

- **`sdd_phase` closed set omits `remediate`.** `proposal-synthesis-notes.md`'s `## Coverage Gaps`
  flagged that the propose phase left open whether `sdd_phase` closed-set membership (checked only
  when `lane_role` is present) includes `remediate`, and said design should confirm it. `design-lens-b.md`
  Decision 1 sets the closed set to `{explore, propose, spec, design, tasks, apply, verify, archive}`
  — `remediate` absent — without discussing the omission. Confirm whether this was a deliberate
  decision or a silent drop of the propose-phase gap. If lens B gave no rationale for the omission,
  treat the gap as still open in `design.md` and say so explicitly, rather than letting the omission
  read as a considered choice it was not.
- **Precedence conflict between the real `sdd-design` skill and this packet's own parameters.** Lens
  B and lens C independently raised the same `## Open Questions` item: the real
  `~/.claude/skills/sdd-design/` skill prescribes an 800-word budget, Engram persistence, and a
  phase-summary return block, while this packet's own parameters (1800-word canonical budget,
  `.lucind/result.json` return) were followed instead. This is not actually unresolved — it is the
  same content-versus-execution split `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:43`
  already states: *"The real phase skill governs required document content. The packet governs
  topology, ownership, budgets, paths, and done criteria."* That split is precisely what
  `internal/phasespec` (Decision 6, lens A) must encode when it composes `gentle-ai sdd-status` with
  lucind-ai dispatch — content authority stays with gentle-ai's phase skills, execution authority
  (budgets, paths, done criteria) stays with the dispatched packet. Record this resolution in
  `design.md` under the specialist's design (it is substantively part of Decision 6, not a leftover
  question) and do not re-list it under `## Open Questions`.

### 5. Compress — do not concatenate

`design.md` MUST be under 1800 words. The three drafts total roughly 2600. Cutting is the job: merge
overlapping statements, drop restatement, keep the specific sentence over the general one. A
concatenation of three drafts is a failed synthesis even if every word in it is true.

### 6. Coverage check

`design.md` must cover this repository's actual design spine, derived from every archived design in
`openspec/changes/archive/`:

1. Technical approach or recommendations at a glance
2. Architecture decisions, each with choice / alternatives considered / rationale
3. Flow and invariants
4. File changes, with terminal consumers
5. Testing strategy and test seams
6. Threat matrix — every row `Applicable` or `N/A: reason`
7. Rollback and additivity
8. Open questions and out of scope

Section headings may follow the change's own vocabulary — archived designs vary — but every one of
the eight must be substantively present. Anything no draft covered goes under `## Coverage Gaps` in
the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/skill-provisioning-and-phase-specialist/design.md`

The canonical design. Under 1800 words. Covers all eight spine items. Contains only claims whose
citations you verified in step 2 and which survive lens A's architecture. States the resolution of
both carried-forward items from step 4 explicitly.

### `openspec/changes/skill-provisioning-and-phase-specialist/design-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it. State both positions
and what evidence each has. Do NOT pick — this section is the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered, INCLUDING whether the `remediate`/`sdd_phase` gap from step 4 was
resolved or is genuinely still open. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code actually says.
"None" if there are none.>

## Architecture Divergence

<What lens B or lens C assumed that differed from lens A, what content that cost them, and where
they converged independently. Include how the two carried-forward open items from step 4 were
resolved. "None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write specs, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/design.md` and
`openspec/changes/skill-provisioning-and-phase-specialist/design-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill. Check the
canonical document against the contract as written: its required sections, the choice /
alternatives / rationale shape of a decision, and the threat-matrix applicability rule. On those,
the skill wins over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word budget — the skill's nominal 800 is not
honored in this repository, as `openspec/changes/archive/` shows — along with the synthesis
procedure, the notes file, and the done criteria. The skill's own Engram persistence and return-block
steps are superseded: your output is the two files named above plus `.lucind/result.json`.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as
your **verification worklist, never as evidence**. A manifest row was written by the same lane that
made the claim, so a wrong citation arrives with a confident row beside it. The property that makes
this fan-out trustworthy is that you open every cited range yourself and check it against the real
code in this worktree. That property is not negotiable and the manifests do not relax it.

What the manifests are for is speed without loss:

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose
  mention.
- **Batch by file.** Open each cited file once and check every citation into it, instead of jumping
  between files in prose order.
- **Verify the claim, not the line's existence.** A row states what the lens asserts that range
  shows. A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.** An
  incomplete manifest does not shrink your obligation.

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `design.md` the moment it is written, before you begin the notes file.** Then write the
notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit leaves
finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies before a
single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup` deletes without
warning — which has already cost this project one full synthesis run. Two commits convert a timeout
from lost work into resumable work.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each
one: some executors' commit wrappers append a `Co-authored-by:` trailer the message never contained.
Strip it if present.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit. It is a deterministic
script, not a judge: it reports whether these facts hold; it does not decide whether your synthesis
is good, and it does not replace your own judgment against `## Done criteria` below.

**Right after the first commit** (`design.md`, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/design.md --budget 1800 --skip-result
```

A `git status --porcelain` FAIL here (the default check, not skipped) means the first commit did not
actually land everything it should have — catch that before you start the notes file, not after the
second commit buries it.

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/design-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Architecture Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose. The canonical document's own spine coverage is substantive, not a
fixed set of heading strings, so the script does not and cannot check it — that judgment stays yours.

## Done criteria

- [ ] **`design.md` was committed as its own commit before `design-synthesis-notes.md` was started**,
      confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `design.md` was opened and confirmed in this
      worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Both carried-forward open items from step 4 are explicitly resolved or explicitly left open
      with reason in `design.md`** — neither silently vanishes and neither is silently invented.
- [ ] **`design.md` exists, is under 1800 words, and substantively covers all eight spine items**,
      with anything missing reported under `## Coverage Gaps`.
- [ ] **`design-synthesis-notes.md` exists with exactly the four required sections**, each either
      populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the
      final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid
      `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The three `## Assumed architecture` blocks are mutually irreconcilable and the proposal and specs
  do not choose between them. Write the notes file, leave `design.md` uncreated, and block.
- Lens A's architecture is refuted by code you verified. Do not substitute your own.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words. Report which item
  forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: Skill Provisioning and the SDD Phase Specialist. Chosen candidate: Candidate 1 —
Deterministic Three-Tier Skill Provisioning with a Non-Intercepting SDD Phase Specialist
(`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:3`). Read the full proposal
first — it is committed in this worktree.

**`proposal-synthesis-notes.md` is committed alongside the proposal.** Its `## Dropped Citations`
lists four citations the propose-phase synthesis already found wrong or stale
(`internal/ledger/authoring.go:23`, `internal/packetauthor/compile.go:49-65` as a budget seam,
`internal/accept/authoring_evidence_test.go:56-127` as a reflection pin, and
`internal/skillcontent/skillcontent.go:90-100` as the full `HashDir` range — the correct full range
is `73-100`). If any surviving lens draft repeats one of these, drop it without re-verifying; it is
already confirmed wrong.

**Verified by the orchestrator before dispatch** (you should still re-verify independently per step
2, but these are not expected to surprise you):

- `internal/run/run.go:876-904` (`enforceAllowedPaths`) demotes to `lane.Deviated` on an out-of-scope
  git diff; it has no skill-awareness today — `enforceRequiredSkills` is proposed net-new, sitting
  beside it.
- `internal/accept/accept.go:275-286` is today's inline decode struct inside
  `validateVersionedEvidence`; it has no `LaneRole` or `RequiredSkills` fields yet.
- `internal/ledger/authoring.go:14` is `AuthoringEvidenceVersion = "lane-authoring-evidence/v1"`;
  `:20-42` is the `AuthoringEvidence` struct, whose `Contract` field (line 26) is `json.RawMessage`.
- `internal/packetauthor/compile.go:171-183` (`renderBody`) today ends with `## Hard stops` then
  `## Return`; there is no `## Required skills` section yet — the proposed insertion point is
  between them.
- `internal/result/result.go:103-116` (`Envelope`) has no `SkillsLoaded` field yet;
  `internal/result/result.schema.json:5` sets `"additionalProperties": false`.
- `cmd/lucind-ai/packet_authoring.go:32-54` (`admitDispatchBatch`) is the real pre-worktree admission
  seam that resolves target bindings before calling `packetauthor.AdmitBatch`.

**Decided already — do not re-open, re-offer, or widen:**

1. No `AuthoringEvidence` struct field addition and no `AuthoringEvidenceVersion` bump — skills ride
   inside the existing `Contract json.RawMessage` blob.
2. No SQLite migration.
3. The specialist never intercepts or wraps `gentle-ai`; it composes `gentle-ai sdd-status` output
   with lucind-ai dispatch.
4. External skill content hashing (`HashDir`) is observation-only, never a blocking gate.
5. Single PR, `single-pr` delivery strategy (`openspec/config.yaml:6-7` authorizes it at 10000 lines).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations
verified, citations dropped, contradictions escalated, coverage gaps, and how each of the two
carried-forward open items from step 4 was resolved. Report `done` only when every done-criterion
carries evidence and every hard stop is declared.
