---
id: spec-deterministic-lucind-ai-orchestrator-lens-b
executor: agy
routed_by: scenarios and coverage lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-b.md"]
---

# Packet spec-deterministic-lucind-ai-orchestrator-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/spec-deterministic-lucind-ai-orchestrator-lens-b  ·  **Branch:** lucind/spec-deterministic-lucind-ai-orchestrator-lens-b

## Goal

Produce `openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-b.md`: a Given/When/Then
scenario set for every requirement this change introduces or changes, plus the coverage argument
that says which happy paths, edge cases, and error states are proven and which are not.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final
delta spec. Do not write anything under `openspec/changes/deterministic-lucind-ai-orchestrator/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens A and lens C run in
parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the
requirements you are writing scenarios for from the proposal itself, declare them in
`## Assumed requirements`, and key every scenario to one of them by name. The synthesizer
arbitrates divergence; scenarios keyed to a requirement nobody else named are dropped, so name
them the way the proposal does.

## Preconditions

- `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md` exists and is accepted.
- `openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill.
2. `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md`, and its **Capabilities**
   section in particular — that is where you derive the requirement names your scenarios key to.
3. Two or three archived delta specs under `openspec/changes/archive/*/specs/` — read how this
   repository actually writes a scenario.
4. The code paths the proposal names (`cmd/lucind-ai`, `internal/packet`, `internal/run`,
   `internal/dag`, `internal/ledger`, `internal/accept`, `internal/worktree`), enough to know what
   a precondition and an observable outcome actually are here.

Never invent a state the system cannot be in. A precondition you cannot reach is a scenario nobody
can write a test from.

## Output format

Write exactly this skeleton to
`openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-b.md`:

```markdown
# Spec Lens B — Scenarios & Coverage: Deterministic lucind-ai Orchestrator

## Assumed requirements

<2–4 sentences naming the requirement set you are writing scenarios for: which
capabilities this change touches and what each requirement asserts. Lens A and
lens C write this same block independently; the synthesizer compares all three.
Be specific enough that a disagreement is visible.>

## Scenarios

### Requirement: <Name, as the proposal names it>

#### Scenario: <Happy path>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>
- AND <additional outcome, if any>

#### Scenario: <Edge case>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>

#### Scenario: <Error state>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable failure — the error, the exit code, the refusal>

### Requirement: <Next name>

<same shape>

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|

<One row per requirement. Mark each column covered or missing — "missing" is a
legitimate and useful answer. The last column cites the seam a test would assert
through, or states "new seam required".>

## Untestable Assertions

<Every scenario you wanted to write but could not, because its THEN is not
observable through anything that exists. Name the requirement and what would have
to exist. "None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-b.md` MUST be under 1000 words. Scenarios are the bulk of it, so keep every
GIVEN / WHEN / THEN to one line. If the scenario set does not fit, cover every requirement's happy
path first, then edge cases, then error states, and record what you had to leave out under
`## Open Questions`.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: the capability map, the requirement statements themselves, and their
  ADDED / MODIFIED / REMOVED / RENAMED classification.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement,
  and every Migration note.

Do not restate a requirement's text above its scenarios beyond the `### Requirement: <Name>`
heading. The name is the join key; the text is lens A's.

Do NOT create or write any file under
`openspec/changes/deterministic-lucind-ai-orchestrator/specs/`. That tree belongs to the
synthesizer.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-b.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Precedence is **not symmetric**: the skill wins on *what a delta spec must contain*
(Given/When/Then scenario format, one-scenario-minimum rule, happy-path-and-edge-case rule, WHAT
not HOW rule). This packet wins on *how this phase is executed here* (three-lane split, this
lens's slice, word budget, skeleton, done criteria). Note any conflict in `## Open Questions`,
follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file.

| citation | claim |
|---|---|
| `internal/run/run.go:608` | run.go persists frozen candidate identity before acceptance, a seam a wave-barrier scenario can assert through |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Scenarios" \
  --require-section "Coverage" --require-section "Untestable Assertions" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-b.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the
  claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL
  against this draft's own manifest.**
- [ ] **Every requirement named in `## Assumed requirements` has at least one scenario**, and every
  scenario is in GIVEN / WHEN / THEN form.
- [ ] **Every scenario's THEN names an observable outcome**, and the coverage table cites the seam
  it is observable through or marks it "new seam required".
- [ ] **`spec-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries
  `## Assumed requirements`, `## Coverage`, `## Untestable Assertions`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The proposal does not determine what the system should do in a case, so the scenario would
  assert an outcome nobody chose.
- Every scenario for a requirement would be untestable, meaning the requirement as proposed is
  unobservable.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- The proposal's Capabilities section (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:19-28`)
  names five capabilities: **New** — `deterministic-orchestrator-contract`. **Modified** —
  `packet-authoring-contract`, `sdd-apply`, `parent-feature-integration`, `acceptance-verifier`.
- The proposal's `## Approach` section (`proposal.md:30-34`) names the exact runtime call sites the
  scenarios should be testable through: `cmd/lucind-ai`, `internal/packet`, `internal/run`,
  `internal/dag`, `internal/ledger`, `internal/accept`, `internal/worktree`.
- The explore doc (`openspec/changes/deterministic-lucind-ai-orchestrator/explore.md:4`) confirms
  concrete existing seams already exist for candidate/evidence persistence: `internal/run/run.go:608-665`
  and `:1004-1019`; acceptance revalidation at `internal/accept/accept.go:213-341`; deterministic
  wave computation and overlap rejection at `internal/dag/waves.go:11-18,43-66` and
  `internal/dag/overlap.go:10-15,52-67`; CAS promotion and no-redispatch retry at
  `internal/run/integrate_feature.go:13-48` and `internal/run/integrate_retry.go:16-43`.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
