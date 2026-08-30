---
id: spec-lane-status-observability-lens-c
executor: agy
routed_by: orphan reconciliation and live-spec conflict/migration lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/spec-lens-c.md"]
---

# Packet spec-lane-status-observability-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-lane-status-observability-lens-c  ·  **Branch:** lucind/spec-lane-status-observability-lens-c

## Goal

Produce `openspec/changes/lane-status-observability/spec-lens-c.md`: two things combined.

1. **Your own new capability**, like lens A and lens B: the requirement and scenarios for
   **Orphaned lane reconciliation** (`orphan-lane-reconciliation`, a new capability).
2. **The live-spec-conflict and migration role for this entire change**: open the three
   *Modified Capabilities* the proposal names — `lane-execution`, `read-only-packet-schema`, and
   `batch-wave-view` — in full, confirm or correct lens A's ADDED classification for the two
   requirements it owns (`Lane metadata dispatch persistence` → `lane-execution`,
   `Extended packet frontmatter parsing` → `read-only-packet-schema`), and author
   `batch-wave-view`'s own requirement, which has **no draft anywhere in proposal.md** — you are
   the only lane that will read `batch-wave-view/spec.md`.

You also flag the one cross-cutting concern in this change: schema v7 touches columns for two
different capabilities (`runs.pid` for your own `orphan-lane-reconciliation`, and `lane_progress`
usage columns for lens B's `lane-progress-telemetry`). Do not write lens B's requirement — flag
the overlap for the synthesizer instead.

This is one of three parallel spec lenses. Do not write anything under
`openspec/changes/lane-status-observability/specs/`.

## Why this is safe to dispatch now

The proposal for `lane-status-observability` is accepted and frozen (merge commit `97e9bbc`). Lens
A and lens B run in parallel against the same frozen inputs and write to different files, so no
lane races another. Lens A owns the requirement text and scenarios for `lane-execution` and
`read-only-packet-schema` — this lens is downstream of it in one specific sense (classification
correction), same as the standard fan-out's "one deliberate exception to lens A's authority": if
your live-spec evidence contradicts lens A's ADDED classification, yours wins on classification
only. You do not have lens A's draft (parallel dispatch) — derive what you're checking against from
proposal.md directly, exactly as lens A does.

## Why this lens exists

Archive replaces a live requirement with whatever a MODIFIED block says. A partial MODIFIED block
silently deletes every scenario it failed to copy, and nothing catches it until the capability is
already wrong in `openspec/specs/`. This lens is the lane that opens the three live specs and
copies whole blocks forward where needed.

## Preconditions

- `openspec/changes/lane-status-observability/proposal.md` exists and is accepted.
- `openspec/changes/lane-status-observability/spec-lens-c.md` does not yet exist.
- `openspec/specs/lane-execution/spec.md`, `openspec/specs/read-only-packet-schema/spec.md`, and
  `openspec/specs/batch-wave-view/spec.md` all exist.
- `openspec/specs/orphan-lane-reconciliation/` does NOT exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill, and the **MODIFIED
   Requirements Workflow** section in particular. It is the phase contract this draft feeds; read
   it rather than trusting this packet's paraphrase of it.
2. `openspec/specs/lane-execution/spec.md` **in full** (61 lines, 3 requirements: "Gate Placement
   in the Lifecycle", "Resolve Before Barrier Observation", "Additive Schema, Unchanged Enum").
3. `openspec/specs/read-only-packet-schema/spec.md` **in full** (82 lines, 5 requirements:
   "Frontmatter Read-Only Field Parsing", "Default Value and Backward Compatibility",
   "Explicit Flag Only — No Inference", "The Envelope Cannot Declare or Override Mode",
   "Additive Rollback").
4. `openspec/specs/batch-wave-view/spec.md` **in full** (29 lines, 1 requirement: "Batch and DAG
   Wave Inspection"). Not the index, not a grep — the whole file for all three. You cannot report
   what a change collides with from a search result.
5. `openspec/changes/lane-status-observability/proposal.md` in full — for what the change intends
   to do to each capability, and for your own **Orphaned lane reconciliation** requirement draft
   (already has full text and two scenarios in `## Delta Specifications`).
6. Consumers of anything you find yourself modifying: tests, docs, other specs that reference it by
   name. Cite each with `file:line`.

Never claim a live requirement says something without opening it. This lens is the only lane that
reads these three live specs in full; a wrong claim here is not caught downstream.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/spec-lens-c.md`:

```markdown
# Spec Lens C — Orphan Reconciliation & Live-Spec Conflicts: Lane Status Observability

## Assumed requirements

<2-4 sentences: your own new requirement (Orphaned lane reconciliation), and the
requirement set you are checking against live specs for the three modified
capabilities. Lens A and lens B write their own version of this block
independently; the synthesizer compares all three.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

<Four rows: orphan-lane-reconciliation (new, target
`openspec/specs/orphan-lane-reconciliation/spec.md`, cite nothing), and
lane-execution / read-only-packet-schema / batch-wave-view (existing, target
`openspec/changes/lane-status-observability/specs/<capability>/spec.md`, cite
the live spec file:line from your Required Reading).>

## ADDED Requirements

### Requirement: Orphaned lane reconciliation

<RFC 2119 requirement text. Start from proposal.md's draft; verify every
citation independently before repeating it.>

**Terminal consumer**: <file:line for `RegisterRun`, the sweep, and `SetStatus`
— verify each independently>

#### Scenario: Dead-process lane swept to failed

...

#### Scenario: Active process lanes unchanged

...

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|

<Three rows — lane-execution, read-only-packet-schema, batch-wave-view. Counts
come from opening the file, not from estimating.>

## Conflicts

<Every place this change contradicts a live requirement rather than extending
it. A conflict is a MODIFIED requirement, not an ADDED one — say so. "None" if
there are none.>

## Classification Corrections

<For lane-execution and read-only-packet-schema: does lens A's likely ADDED
classification hold, or does your live-spec reading show the new behavior
actually edits an existing requirement's text (making it MODIFIED instead)?
State your finding plainly — this is the one place your evidence outranks
lens A's, and only on classification, not on requirement text. "Confirmed
ADDED for both" is a legitimate finding.>

## MODIFIED Full Blocks

### Requirement: <Live Requirement Name, if any capability needs one>

**Source**: `openspec/specs/<capability>/spec.md:<line>` — <N> scenarios

<The COMPLETE live block, copied verbatim, if `batch-wave-view`'s existing
"Batch and DAG Wave Inspection" requirement needs editing to cover rendering
metadata/packet-link/usage/swept-orphan-failed. Do not summarize, do not elide
a scenario. Author the edited requirement text and at least one new scenario
covering the new rendered fields; keep every existing scenario unless it is now
false. If no live requirement needs a MODIFIED block, write "None — see ADDED
Requirements" here instead and put batch-wave-view's new behavior there.>

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|

<"None" is a legitimate table if empty — state it in prose instead of an empty table.>

## Cross-Cutting Schema v7 Note

<Flag, do not resolve: schema v7 is one STRICT create-copy-drop-rename touching
both `runs.pid` (your own orphan-lane-reconciliation capability) and
`lane_progress` usage/tool columns (lens B's lane-progress-telemetry capability).
Name both capabilities and the shared migration seam so the synthesizer keeps
the two requirements pointing at one coherent v7 design rather than two
independent, possibly conflicting migration descriptions.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-c.md` MUST be under 1000 words **excluding the verbatim blocks under
`## MODIFIED Full Blocks`**. Those blocks are copied evidence, not authored prose — truncating one
to fit a budget is the exact failure this lens exists to prevent. Everything you write in your own
words stays under the cap.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: the requirement TEXT for `Lane metadata dispatch persistence` and
  `Extended packet frontmatter parsing`, and their scenarios. You may CORRECT their classification
  (ADDED vs MODIFIED) with live-spec evidence, but do not rewrite their requirement text.
- **Lens B owns**: `dispatched-packet-body` and `lane-progress-telemetry` — their requirements,
  scenarios, and capability rows. Do not write the telemetry requirement even though you are
  flagging the v7 overlap with it.

Do NOT create or write any file under `openspec/changes/lane-status-observability/specs/`. That
tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/lane-status-observability/spec-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its `references/`.

Precedence is **not symmetric**. The skill is authority on *what a delta spec must contain*: the
MODIFIED copy-full-then-edit workflow, the REMOVED Reason-and-Migration rule, the RENAMED
both-names rule. Where this packet paraphrases any of that and drifts, the skill wins and the drift
belongs in `## Open Questions`. This packet is authority on *how this phase is executed here*: the
capability-domain split, which capabilities this lane checks, its word budget, output
path/skeleton, out-of-scope list, and done criteria. The skill describes one sub-agent writing the
whole delta tree alone and will read as instructing you to write files under `specs/`, persist to
Engram, or return the phase summary block — those are superseded here. Note the conflict in
`## Open Questions`, follow this packet.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, ascending line numbers.

| citation | claim |
|---|---|
| `openspec/specs/batch-wave-view/spec.md:9` | "Batch and DAG Wave Inspection" is the sole requirement in this capability today |

The manifest is a worklist for the synthesizer, not a certificate.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim it supports.**
- [ ] **All three modified capabilities (`lane-execution`, `read-only-packet-schema`,
      `batch-wave-view`) were opened in full**, and the Live Spec Inventory row's requirement and
      scenario counts came from the file, not an estimate.
- [ ] **Every `## MODIFIED Full Blocks` entry (if any) is the complete live block**, scenario for
      scenario, with nothing summarized or elided.
- [ ] **`batch-wave-view` has a concrete requirement** — either a MODIFIED full block or an ADDED
      requirement — since proposal.md has no draft for it.
- [ ] **Your own `Orphaned lane reconciliation` requirement carries an RFC 2119 keyword** and states
      observable behavior, not implementation.
- [ ] **`spec-lens-c.md` exists, is under 1000 words excluding verbatim blocks, and carries
      `## Assumed requirements` and `## Live Spec Inventory`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- Any of the three capabilities' live spec is missing or does not resolve.
- Copying a MODIFIED block whole would exceed what you can write, so the copy would have to be
  partial. Report which requirement forces it; never write a partial block.
- `batch-wave-view`'s existing requirement's scenarios conflict with the new rendered fields in a
  way the proposal does not resolve (e.g. an existing scenario asserts a field is always
  "Unavailable" and the new behavior contradicts it without the proposal saying which wins).
- Your own requirement cannot be stated without deciding an implementation detail design has not
  decided (e.g. ticker interval or PID-liveness syscall — Open Questions 3 and 4 stay open).
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/lane-status-observability/proposal.md` is committed in this worktree and is the
accepted proposal.** Read it first.

**User-approved decisions, already final — do not re-litigate:**

- Full six-item scope ships as **one PR**, accepting `size:exception` for the review-budget risk.
- "Skill" observability is static-only (not your requirement).
- `delivery_strategy` is `exception-ok` for this change.

**Five open questions from proposal.md's `## Open Questions` remain OPEN. Write requirements and
scenarios that are correct regardless of how these resolve later at design time:**

1. Frontmatter key names — not this lens's requirement (lens A's).
2. Packet path persistence mechanism — not this lens's requirement (lens B's).
3. **Ticker interval for the periodic orphan sweep — directly your requirement.** Do NOT pick a
   number (e.g. "every 30s"). State "on a periodic ticker" without a concrete interval, and record
   the open question explicitly.
4. **PID-liveness syscall choice and cross-platform scope — directly your requirement.** Do NOT
   pick `/proc/<pid>` vs `syscall.Kill(pid, 0)`, and do NOT assert Linux-only or cross-platform.
   State "a liveness check on the stored PID" without naming the syscall.
5. DAG-wave `Node`/`EmitPacketContent` scope — not this lens's requirement, but note in
   `## Open Questions` if your sweep requirement's scope interacts with it (it should not).

**Process note — verify citations independently.** Propose-phase lens citations into
`internal/serve/handlers.go` were frequently wrong (citing `:33-60` or `:30-120` for concepts that
are actually elsewhere). The same discipline applies here with extra weight: you are the only lane
opening `openspec/specs/lane-execution/spec.md`, `read-only-packet-schema/spec.md`, and
`batch-wave-view/spec.md` in full. A wrong requirement-count or wrong classification here is not
caught by anyone downstream except the synthesizer, who trusts your inventory as the reason it
does not re-open all three itself.

**Dropped citations from the propose-phase synthesis — do NOT resurrect these:**

- `internal/ledger/schema.go:310-330` is NOT v7 DDL (it is the `migrate` function comment/start).
  The v7 *pattern* to follow is `schema.go:182-219` (v4→v5) and `:221-308` (v5→v6).
- `internal/serve/handlers.go:33-60`/`:30-120` are `ServerState` fields and `/api/state` pagination
  bounds — NOT the sweep seam. Sweep is explore/proposal ground truth with no existing citation.
- `internal/serve/server.go:1-60`/`:19-53` is `ListenAndServe` — NOT sweep, PID liveness, or ticker.
- `internal/ledger/lanes.go:35-50` does not exist. `SetStatus` is `internal/ledger/ledger.go:452`.
  The running-transition seam is `internal/run/run.go:355`.
- `internal/serve/server_test.go:42-93` is `TestBulkRequestBodyReturns400` — NOT sweep-coverage
  evidence.
- `internal/ledger/schema.go:298-308` is `CREATE TABLE lane_progress` — NOT the `runs.pid` seam.
  `runs` without `pid` today is `schema.go:226-234`.
- `internal/ledger/runs.go:103-137` is `RunIDsByRecentEvent` — NOT a v7 no-backfill policy citation.
- `internal/run/run_test.go:25-60` and `internal/run/batch_test.go:170-210` define test fixtures,
  not existing metadata-dispatch coverage.

**Ground truth for your own requirement — cite it, verify it, do not re-derive it:**

- No PID or heartbeat is stored today: `ledger.Run` (`internal/ledger/runs.go:16-24`);
  `RegisterRun` (`runs.go:29-40`, and production call site `cmd/lucind-ai/cli.go:314-321`) inserts
  no `pid`. No orphan-sweep code exists anywhere today.
- `runs` and `lane_progress` are STRICT tables; SQLite cannot widen them in place
  (`schema.go:182-189,221-224`) — this is why v7 is a create-copy-drop-rename, same pattern as v5/v6.
- Running-transition seam: `internal/run/run.go:355` (`SetStatus`); `ledger.SetStatus` writes
  `lane_status_changed`; pair the transition with an `EventLaneNote` explaining the sweep fired.
- PID reuse risk is accepted (Risks table in proposal.md): "sweep again when the reassigned PID
  later dies; do not invent a second identity check until design."

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
