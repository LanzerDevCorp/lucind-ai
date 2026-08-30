---
id: propose-lane-status-observability-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel propose lenses into one canonical proposal document
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/lane-status-observability/proposal.md", "openspec/changes/lane-status-observability/proposal-synthesis-notes.md"]
---

# Packet propose-lane-status-observability-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/propose-lane-status-observability-synthesis  ·  **Branch:** lucind/propose-lane-status-observability-synthesis

## Goal

Read the three propose lens drafts for `lane-status-observability`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/lane-status-observability/proposal.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `propose-lens-a.md`, `propose-lens-b.md`, and `propose-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `propose-lens-a.md`, `propose-lens-b.md`, and `propose-lens-c.md` all exist in this worktree.
- `openspec/changes/lane-status-observability/proposal.md` does not yet exist.

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

### `openspec/changes/lane-status-observability/proposal.md`

The canonical proposal. Under 1800 words. Covers the proposal spine. Contains only claims whose citations you verified in step 2 and which survive lens A's approach.

### `openspec/changes/lane-status-observability/proposal-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: Lane Status Observability

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
- Do NOT resolve any of the five open questions carried forward from `explore.md` (see Context
  below). They must stay open in `proposal.md`'s own `## Open Questions`.

## Allowed paths

`openspec/changes/lane-status-observability/proposal.md` and `openspec/changes/lane-status-observability/proposal-synthesis-notes.md` only.

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

**Read `openspec/changes/lane-status-observability/explore.md` first.** It is committed in this
worktree, it is the accepted exploration for this change, and it recommends **Candidate 1**: wire
the existing metadata path, add PID-based orphan sweep, and add structured telemetry, all in one
PR under `size:exception`.

**The human already decided the following. They are DECIDED. Do not re-litigate them, do not
present them as alternatives, and do not quietly widen or narrow them in `proposal.md`:**

1. **Full six-item scope ships as one PR, accepting `size:exception`.** Explore's own
   recommendation was to split into two PRs; the user explicitly overrode that. If any draft
   argues for the two-PR split as the chosen path rather than a rejected alternative, that is a
   drafting error to correct during synthesis, not a genuine contradiction to escalate.
2. **"Skill" observability is static, not live telemetry.** A new `skill:` frontmatter key, set by
   the authoring orchestrator, records which skill/phase authored a packet. Live runtime "Skill"
   telemetry from any executor (`agy`, `cursor-agent`, `opencode` — none are Claude Code) is
   explicitly out of scope; the live proxy is generic per-lane tool-call counts, reusing the same
   stream decoders already parsing usage.
3. **`delivery_strategy` is `exception-ok`** for this change: single PR, size exception accepted
   up front.
4. **SDD Session Preflight for this session**: `execution_mode=auto`,
   `artifact_store=hybrid` (Engram + OpenSpec), `review_budget_lines=1200` (deliberately exceeded
   per the `size:exception` above — do not treat a review-budget overage finding as a defect to
   fix; it is an accepted, named risk).

**Ground truth every lens was given** (from `explore.md`; re-verify citations independently per
step 2, do not merely trust that the lenses cited them correctly):

- `ledger.LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`); `UpdateLaneMetadata`/
  `GetLaneMetadata` (`lanes_meta.go:39,89`); `serve.Lane` (`internal/serve/model.go:163-184`) and
  `app.js:532-538` already consuming it; **zero production callers** of `UpdateLaneMetadata`
  (`internal/run/run.go:334`, `internal/run/batch.go:184` call `RegisterLane` only).
- `packet.Parse` (`internal/packet/packet.go:78-167`) recognized-keys list; no `sdd_phase`/
  `fanout_group`/`skill` key exists yet; `Packet` has no `Path` field (only `cli.go:160-166`'s
  index-aligned slices know it).
- `agy_stream.go:12-18`, `claude_stream.go`, `opencode_stream.go:100-113` parsing real usage
  numbers into discarded prose (`executor.go:17-21`); `cursor_agent.go` has no usage struct.
- SSE hub reads `ledger.LaneProgress` (`internal/ledger/progress.go:15-20`) from a STRICT
  `lane_progress` table (`schema.go:298-307`) with no usage/tool-call columns.
- No PID/heartbeat stored anywhere (`ledger.Run`, `runs.go:16-24`); no orphan-sweep code exists.
- `runs`/`lane_progress` are STRICT (SQLite cannot widen in place, `schema.go:183-184,221-224`); a
  v7 migration is required, following `migrateV4ToV5DDL` (`schema.go:182-219`) /
  `migrateV5ToV6DDL` (`schema.go:221-308`)'s create-copy-drop-rename shape.

**Open questions that MUST survive into `proposal.md`'s own `## Open Questions` verbatim in
substance** (from `explore.md`; if a lens resolved one of these on its own initiative, that
resolution does not survive synthesis — put it back as open and note the divergence):

- Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs
  `generated_by`.
- Packet path persistence: `LaneMetadata.PacketPath` field vs. a real `lanes` column.
- Ticker interval for the periodic orphan sweep.
- PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and cross-platform scope.
- Whether `internal/dag/parse.go`'s `Node`/`internal/dag/emit.go`'s `EmitPacketContent` get the
  same new fields in this change or a follow-up.

**Out of scope, and a proposal that includes any of it is wrong:** live runtime "Skill" telemetry
from any executor; backfilling historical ledger rows for already-run lanes; `cursor-agent` usage
telemetry; a general-purpose process-supervision/restart mechanism; changing
`internal/dag`'s DAG-wave packet emission unless the open question above explicitly brings it in
scope; cross-platform PID-liveness beyond what the open question above settles.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
