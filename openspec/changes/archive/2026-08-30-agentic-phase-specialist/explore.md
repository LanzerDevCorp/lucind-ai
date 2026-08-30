# Explore: Agentic Phase Specialist

## Problem

Every SDD planning phase (explore, propose, design, spec, tasks) already runs a 3-lens
fan-out + synthesis through lucind-ai (`references/strategies/fan-out.md`), but today the
**Orchestrator** — the top-level conversation — is the one who reads the synthesis notes,
arbitrates unresolved contradictions, and judges Acceptance directly. That keeps the
Orchestrator's context loaded with full Lane evidence for every phase of every Change.

Goal: insert a phase-scoped **Specialist** that owns its phase end to end and hands the
Orchestrator only a compressed **Phase Verdict** — never the raw fan-out evidence.

(Source: `docs/sdd-phase-specialist.md`, decisions 1–9 treated as settled; `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md` records the same decision as Accepted.)

## Current-state findings (verified against code, 2026-08-30)

### 1. `internal/phasespec.Adapter` is still deterministic/non-agentic — CONFIRMED

- `cmd/lucind-ai/cli.go:2517` `phaseDispatch` parses `--change`/`--force`/`--packet` flags,
  resolves the primary root, refuses linked worktrees, builds a `phasespec.NewAdapter`
  (`internal/phasespec/phasespec.go:346`), and wires `adapter.Dispatcher` to a closure that
  either locates an on-disk synthesis packet or **template-generates one from a Go string
  literal** (`cli.go:2605-2637`) before shelling out to `runDispatch(ctx, []string{"--packet", packetPath}, ...)` (`cli.go:2644`).
- The status query goes through `phasespec.CLIStatusQuerier.QueryStatus`
  (`internal/phasespec/phasespec.go:308-333`), which shells to `gentle-ai sdd-status --json`
  (`phasespec.go:318-320`) — this exactly matches the note's claim ("reads `gentle-ai sdd-status`
  JSON"), it is a different binary (`gentle-ai`) than `lucind-ai` itself.
- Nothing in this path invokes an LLM/agent directly, selects skills for itself, or exercises
  judgment — it is control flow + string templating + one subprocess call. Confirms the note's
  characterization as "deterministic Go with zero agency."
- The archived rejection of "Specialist-side skill selection" is confirmed at
  `openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/proposal.md:188`:
  `"Specialist-side skill selection — rejected: packet-author has no tools (.opencode/agent/lucind-packet-author.md:1-8)."`

**Verdict: no drift.** Point 1 of the note matches the current code exactly.

### 2. `references/strategies/fan-out.md` — "Orchestrator reads synthesis notes" still current

- `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47`:
  `"The Orchestrator reads synthesis notes: unresolved contradictions, coverage gaps, dropped citations, and phase divergence. A populated contradiction section requires human judgment."`
- The OpenCode mirror (`plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47`)
  has byte-identical wording.

**Verdict: no drift.** This is exactly the line that decision 7 in the note says must move to
the Specialist.

### 3. OPEN SCOPING QUESTION — not decided here, surfaced for `sdd-propose`

Two materially different execution paths already coexist for SDD planning phases in this
repository, and this Change's own explore/propose/design/tasks phases are themselves running
under one of them right now:

- **Generic sdd-* subagent path** (what produced *this* explore.md): the Claude Code
  `sdd-explore`…`sdd-archive` subagents read/write `openspec/` and Engram directly per
  `~/.claude/skills/_shared/sdd-orchestrator-workflow.md` and `sdd-phase-common.md`. One
  subagent per phase does the phase's work itself — no fan-out, no lens Lanes, no synthesis
  notes, no Phase Verdict. This is the path currently in effect for `agentic-phase-specialist`.
- **lucind-ai fan-out + synthesis dogfooding pattern**: verified via
  `openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/` file listing —
  that archived Change actually shipped with `explore.md`, `propose-lens-{a,b,c}.md`,
  `proposal-synthesis-notes.md`, `proposal.md`, `design-lens-{a,b,c}.md`,
  `design-synthesis-notes.md`, `design.md`, `spec-lens-{a,b,c}.md`, `spec-synthesis-notes.md`,
  `specs/`, `tasks-lens-{a,b,c}.md`, `tasks-synthesis-notes.md`, `tasks.md`, plus `envelopes/`
  and `packets/` directories recording each Lane's dispatch/result evidence. This is the pattern
  the feature being proposed here (agentic phase-Specialist) will eventually formalize as the
  new standard SDD execution mechanism.

These are not just cosmetically different — they produce different artifact shapes
(`explore.md` vs. the generic convention's `exploration.md`; single-author artifacts vs.
lens+synthesis artifact sets) and different authority models (no Acceptance step vs. a
Specialist owning Acceptance of its own phase's Lanes).

**Question for `sdd-propose` to resolve** (do not decide in this document): should
`agentic-phase-specialist`'s own planning phases (propose/spec/design/tasks) continue through
the generic `sdd-*` subagent path (as explore did), or should they instead be run through
lucind-ai's own fan-out+synthesis dogfooding pattern — since that pattern is structurally
closer to (and arguably a live rehearsal of) the exact agentic-Specialist mechanism this Change
is designing? Note that as a side effect of this Change already running under the generic path,
`explore.md` (this file) was produced as a single non-lensed artifact rather than as lens +
synthesis-notes artifacts — that choice itself is evidence of the very ambiguity this question
raises, not a precedent that resolves it.

### 4. Hard Rule "Agents own Lanes, not... Acceptance, or Promotion" — exact current wording confirmed

- `plugin/claude-code/skills/lucind-ai/SKILL.md:19`:
  `"Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion."`
- OpenCode mirror `plugin/opencode/skills/lucind-ai/SKILL.md:19` is byte-identical.
- This is the exact Hard Rule decision 4 in the ADR (`docs/adr/0002-...md:16`) says needs a
  carve-out for phase-scoped Specialist Acceptance — currently it flatly forbids any Agent from
  owning Acceptance, with no Specialist exception yet written.

**Verdict: no drift**, and the carve-out described in the note/ADR genuinely has not been made
yet — "Agents own Lanes, not... Acceptance" is still unconditional in both trees.

### 5. `internal/integrate.Check()` / `internal/accept.Verify()` run unconditionally, no `sdd_phase` gate

- `internal/integrate/integrate.go:159-200` `Check()` always looks for and runs
  `lucind-checks.sh` (`go build ./...` + `go test ./... -race`, per `openspec/config.yaml:16`)
  at the worktree root whenever called; it has no phase-awareness parameter at all.
- `internal/run/attempt.go:433` wires `checkFunc = integrate.Check` unconditionally for
  `lucind-ai run`.
- `internal/accept/accept.go:54-127` (`Verifier.Verify`) also calls it unconditionally: after
  loading the frozen candidate and validating scope/result, it calls
  `integrate.CheckPolicySnapshot()` (`accept.go:120`) and then `v.check(checkCtx, isolation)`
  (`accept.go:126`) with no branch on the packet's declared phase.
- **Where the gate would need to land** (more precise than the note): the field already
  exists — `internal/ledger/lanes_meta.go:25` defines `LaneMetadata.SDDPhase string
  \`json:"sdd_phase"\`` — and `accept.go:89` already calls
  `v.ledger.GetLaneMetadata(ctx, candidate.RunID, candidate.LaneID)` to get a `metadata` value,
  but currently only inside the `if candidate.AuthoringEvidenceVersion == ledger.AuthoringEvidenceVersion`
  branch (`accept.go:84-96`), and only to feed `validateTypedTargetBinding`, not to gate checks.
  The gate insertion point is between the end of that block (`accept.go:96`) and the
  `CheckPolicySnapshot`/`v.check` calls (`accept.go:120-126`): `metadata.SDDPhase` would need to
  be loaded unconditionally (not just inside the authoring-evidence-version branch) and checked
  against `"apply"` before invoking `v.check`. The equivalent gate for `lucind-ai run` would sit
  around `internal/run/attempt.go:433`, before `checkFunc` is invoked.

**Verdict: no drift** from the note's claim that checks run unconditionally; the note did not
specify the exact gate insertion point, which this exploration now pins down with file:line
precision.

### 6. `acceptance-promotion.md` "Acceptance Subagent delegation" — scope confirmed evidence-only

- `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36`:
  the Orchestrator "may delegate steps 1–9 to an ephemeral Acceptance Subagent"; tools are
  restricted to `Read`, `Grep`, and `Bash` within a scoped worktree; the subagent "returns
  structured evidence (diffstat, test semantics, envelope audit, check logs) **without
  inflating the Orchestrator's transcript**."
- The actual accept/reject judgment (step-10-adjacent decision authority) is not delegated in
  this text — the subagent gathers and returns evidence; the Orchestrator still judges. This
  matches the note's claim exactly: "that pattern only gathers evidence today; the actual
  accept/reject judgment still sits with the Orchestrator."
- Also confirmed: this genuinely conflicts with the Hard Rule text in point 4 above only in the
  sense the note already flagged — the Hard Rule says Agents don't own Acceptance at all, while
  this contract already lets a delegate touch Acceptance *evidence*. This is a pre-existing
  inconsistency, independent of the current Change, exactly as the note states.

**Verdict: no drift.**

## Other observed drift (not requested by name, surfaced for completeness)

- `openspec/config.yaml:4` sets `artifact_store.mode: hybrid` and `review_budget_lines: 10000`
  at the repository level, but this session's SDD preflight (given by the orchestrator) resolved
  to `artifact_store: openspec` and a `1500`-line review budget for this specific Change. Not
  contradictory (a session-level override is legitimate), but worth the Specialist/Orchestrator
  being aware the repo default differs, in case a later phase reads `openspec/config.yaml`
  directly and expects it to match the session's cached decision.
- The generic shared convention (`~/.claude/skills/_shared/openspec-convention.md:31`) names the
  sdd-explore artifact `exploration.md`; this task instructed `explore.md` (matching the archived
  dogfooding pattern's naming, not the generic convention's). This filename divergence is a small
  visible symptom of the same unresolved question raised in point 3 above.

## Affected areas (for `sdd-propose`)

- `plugin/claude-code/skills/lucind-ai/SKILL.md:19` and `plugin/opencode/skills/lucind-ai/SKILL.md:19` — Hard Rule carve-out for phase-scoped Specialist Acceptance.
- `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47` and its OpenCode mirror — move "Orchestrator reads synthesis notes" to the Specialist; Orchestrator sees only the Phase Verdict.
- `internal/accept/accept.go:84-127` — gate `v.check`/`CheckPolicySnapshot` on `metadata.SDDPhase == "apply"` (or explicit exception), reusing the already-loaded `LaneMetadata.SDDPhase` field.
- `internal/run/attempt.go:433` (and its call site context) — equivalent gate for the `lucind-ai run` path.
- `~/.claude/skills/sdd-explore/SKILL.md` … `sdd-archive/SKILL.md` (the 8 generic subagents) — if the Change decides these become the "agentic Specialist" runtime substrate (decision 1 in the note), their execution contract needs to actually drive lucind-ai fan-out+synthesis dispatch and Acceptance rather than doing phase work directly, which is a substantial behavior change from what they do today (confirmed in this very exploration: this explore.md was produced by the generic direct-work path, not a fan-out).
- `CONTEXT.md:103-109` and both packaged `domain.md` projections — glossary terms already committed; no further change needed there per the task's explicit instruction not to relitigate.

## Ready for Proposal

Yes, with one explicit caveat: `sdd-propose` MUST resolve the open scoping question in point 3
before drafting scope/approach, because the answer changes both the artifact shape of
`agentic-phase-specialist`'s own remaining phases and whether this Change dogfoods its own
mechanism while building it.
