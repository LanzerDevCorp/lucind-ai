---
id: spec-lane-status-observability-lens-b
executor: agy
routed_by: packet-body inspection and telemetry lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/spec-lens-b.md"]
---

# Packet spec-lane-status-observability-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-lane-status-observability-lens-b  ·  **Branch:** lucind/spec-lane-status-observability-lens-b

## Goal

Produce `openspec/changes/lane-status-observability/spec-lens-b.md`: the delta requirements and
scenarios for the two new observability capabilities — **Dispatched packet body inspection**
(`dispatched-packet-body`) and **Structured progress telemetry** (`lane-progress-telemetry`).

This is one of three parallel spec lenses. Like lens A, this fan-out splits by **capability
domain**, not by aspect: you own both the requirement text and its scenarios for your two
requirements, end to end. Both target capabilities are brand new, so there is no live spec to
open — you are writing FULL specs, not deltas, for `dispatched-packet-body` and
`lane-progress-telemetry`. Do not write anything under
`openspec/changes/lane-status-observability/specs/`.

## Why this is safe to dispatch now

The proposal for `lane-status-observability` is accepted and frozen (merge commit `97e9bbc`). Lens
A and lens C run in parallel against the same frozen inputs and write to different files, so no
lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/proposal.md` exists and is accepted.
- `openspec/changes/lane-status-observability/spec-lens-b.md` does not yet exist.
- `openspec/specs/dispatched-packet-body/` and `openspec/specs/lane-progress-telemetry/` do NOT
  exist — confirm this. If either exists, that contradicts the proposal's classification and is a
  hard stop.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill, in particular the
   **"For NEW Specs (No Existing Spec)"** section — both your capabilities are new, so this
   governs your output shape, not the MODIFIED-block workflow.
2. `openspec/changes/lane-status-observability/proposal.md` in full. Its `## Delta Specifications`
   section already has full requirement text and two scenarios each for **Dispatched packet body
   inspection** and **Structured progress telemetry** — a strong draft to verify and refine, not
   invent from scratch. Its `## Capabilities` section lists both under *New Capabilities*.
3. `openspec/changes/lane-status-observability/proposal-synthesis-notes.md` — records citations the
   propose synthesizer opened and dropped (full list in `## Context` below). Do NOT resurrect any.
4. `internal/serve/handlers.go:190` (`NewHandlerWithConfig`) — the real mux registration point for
   your new packet-body GET route. Verify this yourself; do not trust any other citation for it.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/spec-lens-b.md`:

```markdown
# Spec Lens B — Packet Body & Telemetry: Lane Status Observability

## Assumed requirements

<2-4 sentences: the two requirements you are specifying, which new capability
each targets, and why both are full specs (no live block to inherit from). Lens
A and lens C write their own version of this block independently; the
synthesizer compares all three.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

<Two rows: dispatched-packet-body and lane-progress-telemetry. Both "new" —
target `openspec/specs/<capability>/spec.md` directly (a full spec, not a
delta under the change folder). Cite nothing in the "Live spec" column.>

## Requirements

### Requirement: Dispatched packet body inspection

<RFC 2119 requirement text. Start from proposal.md's draft; verify every
citation independently before repeating it.>

**Terminal consumer**: <file:line for the new GET route and the packet-path
mapping it serves — verify independently, do not assume `handlers.go:190` is
the only citation needed>

#### Scenario: Retrieve packet content

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome — HTTP 200, exact markdown>

#### Scenario: Unknown lane returns 404

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome — HTTP 404>

<Add one edge-case scenario beyond proposal.md's two, if a testable one exists —
e.g. a lane whose packet path was never persisted (Open Question 2 is still
open; write this only if it does not require deciding that question).>

### Requirement: Structured progress telemetry

<same shape as above>

**Terminal consumer**: <file:line for `ProgressEvent`, `LaneProgress`, and the
three populating decoders — verify each independently>

#### Scenario: Decoders populate usage

...

#### Scenario: Cursor-agent emits zeroed metrics

...

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|

## Untestable Assertions

<Any assertion you wanted to write but could not because its THEN is not
observable through anything that exists yet. "None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-b.md` MUST be under 1000 words.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: `lane-execution` and `read-only-packet-schema` — their requirements, scenarios,
  and capability rows.
- **Lens C owns**: `orphan-lane-reconciliation`'s requirement and scenarios; the live-spec-conflict
  role for `lane-execution`, `read-only-packet-schema`, and `batch-wave-view`; and the
  cross-cutting schema v7 note (v7 touches your `lane_progress` usage columns too — lens C will
  flag that as cross-cutting; you do not need to write it).

Do NOT create or write any file under `openspec/changes/lane-status-observability/specs/`. That
tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/lane-status-observability/spec-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its `references/`.

Precedence is **not symmetric**. The skill is authority on *what a delta spec must contain*: the
full-spec format for new capabilities, the RFC 2119 rule, the one-scenario-minimum rule. Where this
packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`. This packet is authority on *how this phase is executed here*: the
capability-domain split, which requirements this lane owns, its word budget, output path/skeleton,
out-of-scope list, and done criteria. The skill describes one sub-agent writing the whole delta
tree alone and will read as instructing you to write files under `specs/`, persist to Engram, or
return the phase summary block — those are superseded here. Note the conflict in
`## Open Questions`, follow this packet.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, ascending line numbers.

| citation | claim |
|---|---|
| `internal/serve/handlers.go:190` | `NewHandlerWithConfig` is the real mux registration point |

The manifest is a worklist for the synthesizer, not a certificate.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim it supports.**
- [ ] **Both capability-map rows are classified "new" correctly** (confirmed absent from `openspec/specs/`).
- [ ] **Both requirements carry an RFC 2119 keyword** and state observable behavior, not implementation.
- [ ] **Every scenario's THEN is observable**; the coverage table names the seam or marks "new seam required".
- [ ] **`spec-lens-b.md` exists, is under 1000 words, and carries `## Assumed requirements`, `## Coverage`, and `## Untestable Assertions`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- `openspec/specs/dispatched-packet-body/` or `openspec/specs/lane-progress-telemetry/` already
  exists, contradicting the proposal's "New Capabilities" classification.
- Either requirement cannot be stated without deciding an implementation detail the design phase
  has not decided (e.g. the packet-path persistence mechanism — Open Question 2 stays open).
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/lane-status-observability/proposal.md` is committed in this worktree and is the
accepted proposal.** Read it first.

**User-approved decisions, already final — do not re-litigate:**

- Full six-item scope ships as **one PR**, accepting `size:exception` for the review-budget risk.
- "Skill" observability is static-only (not your requirement, but do not write a telemetry scenario
  implying live skill/tool-name introspection beyond generic tool-call counts).
- Generic "tool calls made" per lane is the live proxy for skill activity, reusing existing stream
  decoders (`agy_stream.go`, `claude_stream.go`, `opencode_stream.go`) — this belongs inside your
  **Structured progress telemetry** requirement.
- `delivery_strategy` is `exception-ok` for this change.

**Five open questions from proposal.md's `## Open Questions` remain OPEN. Write requirements and
scenarios that are correct regardless of how these resolve later at design time:**

1. Frontmatter key names — not this lens's requirement.
2. Packet path persistence mechanism (new `LaneMetadata.PacketPath` field vs. a real `lanes`
   column) — directly affects your **Dispatched packet body inspection** requirement. Do NOT
   hardcode either mechanism into the requirement text or a scenario's GIVEN; phrase the
   requirement in terms of "the CLI preserves the packet-path-to-lane mapping" (as proposal.md
   already does) without naming the storage mechanism.
3. Ticker interval for orphan sweep — not this lens's requirement.
4. PID-liveness syscall/portability — not this lens's requirement.
5. DAG-wave `Node`/`EmitPacketContent` scope — not this lens's requirement, but your telemetry
   requirement should note (in `## Open Questions`) that DAG-wave packets may or may not gain the
   same fields depending on how this resolves.

**Process note — verify citations independently.** Propose-phase lens citations into
`internal/serve/handlers.go` were frequently wrong (citing `:33-60` or `:30-120` for the
packet-route concept when the real route registration point is `NewHandlerWithConfig` at
`handlers.go:190`). This directly affects your `dispatched-packet-body` requirement — cite
`handlers.go:190` yourself after opening it, do not trust this packet's paraphrase.

**Dropped citations from the propose-phase synthesis — do NOT resurrect these:**

- `internal/ledger/schema.go:310-330` is NOT v7 DDL.
- `internal/serve/handlers.go:33-60` and `:30-120` are `ServerState` fields and `/api/state`
  pagination bounds — NOT the packet-route seam. `handlers.go:190` is correct.
- `internal/serve/server.go:1-60`/`:19-53` is `ListenAndServe` — not relevant to either of your
  requirements.
- `internal/ledger/schema.go:298-308` is `CREATE TABLE lane_progress` (message/seq only, no usage
  columns yet) — this IS relevant to your telemetry requirement as the pre-v7 shape, just do not
  cite it as already containing usage columns.
- `openspec/specs/lane-envelope-inspector` is NOT your packet-body capability. It is demotion
  diagnosis for deviated lanes (a different, pre-existing capability). Your capability is
  `dispatched-packet-body`, confirmed absent from `openspec/specs/` today.
- `internal/run/run_test.go:25-60` (defines `fakeExecutor`) and
  `internal/run/batch_test.go:170-210` (defines `newBatchTestDeps`) were dropped as proof of
  existing metadata-dispatch coverage — irrelevant to your telemetry/packet-body slice but do not
  cite either for anything.

**Ground truth for your two requirements — cite it, verify it, do not re-derive it:**

- `cli.go:160-174` — the CLI maps packet index to on-disk path today; this is what persistence must
  build on.
- `ProgressEvent`: `internal/executor/executor.go:17-21` — no numeric fields today. `LaneProgress`:
  `internal/ledger/progress.go:15-20` — no numeric fields today. Serve DTO: `internal/serve/model.go:187-193`.
- Decoders: `agy_stream.go:12-18,27-39` (usage struct + `formatAgyUsage` at `:160-162`);
  `claude_stream.go:17-36,46-60` (usage+`CostUSD` + `formatClaudeUsage` at `:212-218`);
  `opencode_stream.go:100-125` (tokens+cost into `ProgressEvent.Message` at `:226-228`).
  `cursor_agent.go:1-60` has no usage struct at all; `cursor_agent_stream.go` parses tools only —
  `cursor-agent` MUST leave numeric fields zero, not omitted.
- `app.js:542-544` already looks for `total_tokens`, `cost_usd`, and `tool_rate` — your requirement
  should use these exact field names since the frontend already expects them.
- SSE hub reads `lane_progress` (`schema.go:298-307`) — no numeric columns exist there yet; v7 adds
  them (not your job to design the migration, only to state the requirement).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
