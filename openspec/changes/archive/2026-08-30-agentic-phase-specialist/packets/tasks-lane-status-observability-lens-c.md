---
id: tasks-lane-status-observability-lens-c
executor: agy
routed_by: orphan-lane reconciliation lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/tasks-lens-c.md"]
---

# Packet tasks-lane-status-observability-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-lane-status-observability-lens-c  ·  **Branch:** lucind/tasks-lane-status-observability-lens-c

## Goal

Produce `openspec/changes/lane-status-observability/tasks-lens-c.md`: the implementation task
checklist for orphan-lane reconciliation — PID capture on `RegisterRun`, the serve-side
sweep/ticker, and PID-liveness — design's lens-C capability slice.

This is one of three parallel tasks lenses, sliced by **capability** (matching how `propose`,
`specs`, and `design` were already sliced for this change), not the generic
decomposition/partition/proof three-way split. Your slice is self-contained: write its own ordered
checklist, dependency order, requirement traceability, RED tests, and acceptance evidence together.
It is feedstock for a synthesis lane, not the final `tasks.md`. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and the full `specs/` tree for `lane-status-observability` are accepted, frozen, and
committed on this branch. Lens A and lens B run in parallel against the same frozen inputs and
write to different files, so no lane races another.

## Preconditions

- `openspec/changes/lane-status-observability/design.md` and `specs/` exist.
- `openspec/changes/lane-status-observability/tasks-lens-c.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill.
2. `openspec/changes/lane-status-observability/design.md` — the "PID on `RegisterRun`; serve-side
   sweeper (OQ3, OQ4)" decision, the full `## Flow and Invariants` Sweeper line, the "Process
   integration" threat-matrix row (`design.md:188`) — this is YOUR row, not `N/A` — and the File
   Changes rows for `internal/serve/sweeper.go` (create), `internal/ledger/runs.go` (`Run.PID`),
   and the `cmd/lucind-ai/cli.go` rows for `RegisterRun`/`Sweeper` launch (NOT the `Packet.Path`
   row — lens A's).
3. `openspec/changes/lane-status-observability/specs/orphan-lane-reconciliation/spec.md`.
4. `internal/serve/hub.go:213-235` (`Hub.Run` — the loop pattern this sweeper mirrors, not a seam
   to reuse directly) and `internal/ledger/ledger.go:452-484,366-378` (`SetStatus`, `AppendEvent` —
   the sweeper's two terminal writes).

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line`
citation or is explicitly marked "new file".

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/tasks-lens-c.md`:

```markdown
# Tasks Lens C — Orphan-Lane Reconciliation: Lane Status Observability

## Assumed decomposition

<2-4 sentences: how many phases in this slice, what each delivers, the critical
path — PID capture must land before the sweeper can probe it, and both depend on
lens B's schema-v7 `runs.pid` column existing. Lens A and lens B write this same
block for their own slices.>

## Phase C1: <name>

- [ ] C1.1 <Concrete action — file, change>

## Phase C2: <name>

- [ ] C2.1 <Concrete action>

## Dependency Order (this slice)

| Task | Depends on | Why |
|---|---|---|

<Include the explicit cross-lens row: this slice's `internal/ledger/runs.go` task
depends on lens B's schema-v7 migration landing first.>

## Suggested Work Unit

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|

<One row. This slice ships as part of the single accepted PR — name the unit
anyway so the synthesizer can order it against lens A's and lens B's units.>

## RED Tests from the Threat Matrix (this slice)

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|

<This is the ONE applicable row in design.md's threat matrix: "Process
integration" (live PID, dead PID, PID 0, PID recycling, EPERM). Copy its
"Planned RED tests" column from design.md verbatim as your starting point and
expand each into a concrete task-preceding RED test.>

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|

## Requirement Traceability

| Requirement | Tasks |
|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

Every task MUST be specific, actionable, verifiable, and small. Strict TDD is active for this
project: a RED test task precedes its GREEN production task for every behavior change in this
slice.

## Size budget

`tasks-lens-c.md` MUST be under 1000 words (Citation Manifest excluded).

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: `internal/packet/packet.go` frontmatter fields, `LaneMetadata.Skill`/
  `PacketPath`, `UpdateLaneMetadata` call sites, and the packet-body HTTP endpoint.
- **Lens B owns**: `ProgressEvent`/`LaneProgress` numeric fields, per-executor decoder wiring, and
  the complete v7 STRICT-table migration DDL (both `runs.pid` and `lane_progress` usage columns —
  yes, `runs.pid` too; lens B specifies the whole migration exactly once even though this slice
  consumes the column it creates).

`cmd/lucind-ai/cli.go` is touched by both this lens (`RegisterRun` PID, `Sweeper` launch near
`cli.go:314-324,770-774`) and lens A (`Packet.Path` near `cli.go:160-174`) — scope your tasks to
your own line ranges and note the shared-file overlap in `## Open Questions` so the synthesizer can
decide whether apply needs these sequenced.

## Allowed paths

`openspec/changes/lane-status-observability/tasks-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence is **not symmetric**. The skill is authority on *what a task file must contain*,
including that every applicable threat-matrix row becomes an explicit RED-test task before its
production task. This packet is authority on *how this phase is executed here* — the
capability-sliced split, this lens's slice, its word budget, output path and skeleton,
out-of-scope list, and done criteria. Note any conflict in `## Open Questions` and follow this
packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` cited, one row per unique
citation, grouped by file, files alphabetical, line numbers ascending. The claim column is what YOU
assert that range shows. This is a worklist for the synthesizer, not a certificate.

| citation | claim |
|---|---|
| `internal/serve/hub.go:213-235` | Hub.Run is the immediate-run-then-ticker loop pattern the sweeper mirrors |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice — before committing, and again after
committing and writing `.lucind/result.json`:

```
./lucind-lane-check.sh --file openspec/changes/lane-status-observability/tasks-lens-c.md \
  --budget 1000 --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Dependency Order (this slice)" \
  --require-section "Suggested Work Unit" --require-section "RED Tests from the Threat Matrix (this slice)" \
  --require-section "Acceptance Evidence" --require-section "Requirement Traceability" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` instead of narrating the same
facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every task names a concrete path**, and every "modify" task's path resolves in this worktree.
- [ ] **The "Process integration" threat-matrix row is expanded into explicit RED tests** — live PID, dead PID, PID 0, PID recycling, `EPERM` — each preceding its production task.
- [ ] **The cross-lens dependency on lens B's schema-v7 `runs.pid` column and the shared-file overlap with lens A's `cli.go` changes are named in `## Open Questions`.**
- [ ] **Every requirement in `orphan-lane-reconciliation/spec.md` appears in the traceability table.**
- [ ] **`tasks-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty; check `git log -1 --format=%B` for an unwanted `Co-authored-by:` trailer and strip it if present).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- A requirement in `orphan-lane-reconciliation/spec.md` cannot be decomposed into tasks because
  `design.md` does not say how it is built.
- Two tasks in this slice are mutually circular.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth, already resolved in `design.md` — cite it, do not re-derive it:**

- `RegisterRun` from `runDispatch` (`cmd/lucind-ai/cli.go:314-324`) gets `PID: os.Getpid()` on
  `ledger.Run`.
- New `internal/serve/sweeper.go`: `Sweeper.Run(ctx)` does one immediate sweep, then ticks every
  `10 * time.Second` (Open Question 3 resolved), mirroring `Hub.Run`
  (`internal/serve/hub.go:213-235`) as a loop-pattern reference only — it does not share
  `defaultPollInterval`. Launched beside `Hub` in `serveDispatch` (`cmd/lucind-ai/cli.go:770-774`).
- Liveness (Open Question 4 resolved): `os.FindProcess(pid).Signal(syscall.Signal(0))`. Alive:
  `err == nil` or `errors.Is(err, syscall.EPERM)`. Dead: `errors.Is(err, syscall.ESRCH)` or
  `errors.Is(err, os.ErrProcessDone)`. `pid <= 0` skips the probe and leaves `running` unchanged —
  it means "untracked" (the v7 default), never "dead". Unknown probe errors log; never crash.
  Linux-only; no cross-platform requirement.
- Dead-PID reconciliation: `SetStatus(..., lane.Failed)` (`internal/ledger/ledger.go:452-484`) plus
  `EventLaneNote` ("orphaned: driving process no longer running") via `AppendEvent`
  (`ledger.go:366-378`). No process supervision or restart, ever.
- Threat matrix row "Process integration" (`design.md:188`) is `Applicable` with minimum
  adversarial cases: live PID, dead PID, PID 0, PID recycling, `EPERM`. Planned RED tests:
  `TestSweeper_LivePIDRetained`, `TestSweeper_DeadPIDReconciled`, `TestSweeper_ZeroPIDIgnored`.
- `Run.PID` field, plus its insert/select/scan wiring, lands in `internal/ledger/runs.go:29-41,
  165-188` — this depends on lens B's v7 `runs.pid` column existing first; do not let a task here
  compile or test against a column lens B has not yet created.

**User-approved decisions, already final — do not re-litigate:** full six-item scope ships as one
PR (`size:exception`); `delivery_strategy` is `exception-ok`; no historical-row backfill anywhere;
Strict TDD is active for this project; Linux-only deployment.

**Out of scope:** process supervision or auto-restart, cross-platform PID-liveness,
`internal/resolve`/`internal/conflicttriage` (an unrelated change), anything lens A or lens B owns
(see above).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
