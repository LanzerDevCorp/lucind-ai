---
id: design-lane-status-observability-lens-c
executor: agy
routed_by: orphan-reconciliation lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/lane-status-observability/design-lens-c.md"]
---

# Packet design-lane-status-observability-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-lane-status-observability-lens-c  ·  **Branch:** lucind/design-lane-status-observability-lens-c

## Goal

Produce `openspec/changes/lane-status-observability/design-lens-c.md`: the architecture for
orphan-lane reconciliation — PID storage on `RegisterRun`, the `serve`-side startup-sweep-plus-
ticker, and PID-liveness detection. This lens owns proposal item #6 and MUST make a final, concrete
decision on Open Questions 3 and 4 (see `## Context`) — not another punt.

This is one of three parallel design lenses, sliced by capability. It is feedstock for a synthesis
lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` and the full `specs/` tree for `lane-status-observability` are accepted, frozen, and
already committed on `main`. Lens A and lens B run in parallel against the same frozen inputs and
write to different files, so no lane races another. Lens B owns the exact v7 migration DDL
(including the `runs.pid` column's type/default) and is running concurrently, so you do not have
its final text. Assume `runs.pid` is an `INTEGER NOT NULL DEFAULT 0`, where `0` means "no PID
recorded" (pre-migration rows are never backfilled) — cite lens B's design once it exists; do not
re-derive the column DDL yourself.

## Preconditions

- `openspec/changes/lane-status-observability/proposal.md` exists and is accepted.
- `openspec/changes/lane-status-observability/specs/` exists with all six capability deltas.
- `openspec/changes/lane-status-observability/design-lens-c.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. Because this design
   touches process integration (reading another process's liveness), also read
   `references/threat-matrix.md` and include the applicability matrix.
2. `openspec/changes/lane-status-observability/proposal.md` (full) and
   `specs/orphan-lane-reconciliation/spec.md` (full).
3. `internal/ledger/runs.go` (full file — `Run`, `RegisterRun`, `UpdateRunStatus`, `GetRun`).
4. `internal/serve/hub.go` (full file — the existing `Hub.Run(ctx)` startup-pass-plus-ticker
   shape already running in this exact package).
5. `cmd/lucind-ai/cli.go:283-338` (run-registration sequence in `runDispatch`) and
   `cmd/lucind-ai/cli.go:723-795` (`serveDispatch`, full).
6. `internal/ledger/ledger.go:426-480` (`Event` type, event-type constants, `SetStatus`).
7. `internal/run/run.go:300-360` (the `lane.Running` transition your sweep must reverse).
8. Go's `os` package process-signaling primitives available on this platform (`os.FindProcess`,
   `(*os.Process).Signal(syscall.Signal(0))`) versus reading `/proc/<pid>` directly. This is a
   Linux-only deployment (confirmed prior session context) — state that plainly in your Decision
   rather than engineering portability nothing here needs.

## Output format

Write exactly this skeleton to `openspec/changes/lane-status-observability/design-lens-c.md`:

```markdown
# Design Lens C — Orphan-Lane Reconciliation: Lane Status Observability

## Assumed architecture

<2-4 sentences. Lens A and lens B write this same block independently against
their own slices; the synthesizer compares all three for contradiction.>

## Decision 1 — PID capture point

**Choice**: <exactly which call site captures which process's PID, and how it reaches
RegisterRun>
**Alternatives considered**: <e.g. capturing per-lane PID instead of per-run>
**Rationale**: <grounded in what "the driving process" means for this ledger>
**Terminal consumer**: <file:line>

## Decision 2 — Sweep architecture (where it hooks into serveDispatch)

**Choice**: <the concrete type/function shape, its constructor, exactly where in
cmd/lucind-ai/cli.go:723-795 it is constructed and started, and its relationship — same
goroutine pattern or a distinct one — to the existing `hub.Run(ctx)` launch>
**Alternatives considered**: <e.g. sweeping inside `lucind-ai run` instead of `serve`>
**Rationale**: <grounded in the proposal's "serve and lucind-ai run are separate processes on the
same ledger file" framing, and the existing Hub precedent>
**Terminal consumer**: <file:line>

## Decision 3 — Ticker interval (resolves Open Question 3)

**Choice**: <exact duration, final>
**Alternatives considered**: <other intervals and why they lose>
**Rationale**: <grounded in cost of a sweep query vs. staleness tolerated>
**Terminal consumer**: <file:line>

## Decision 4 — PID-liveness mechanism (resolves Open Question 4)

**Choice**: <exact syscall/stdlib call, final; explicit Linux-only scope statement>
**Alternatives considered**: <the other one from the proposal's named pair, and cross-platform
portability — say explicitly why it is out of scope>
**Rationale**: <grounded in reliability and simplicity for a single-OS deployment>
**Terminal consumer**: <file:line>

## Decision 5 — Historical/zero-PID rows

**Choice**: <what the sweep does when a run's recorded pid is 0 (pre-migration row)>
**Rationale**: <must not misclassify "no data" as "dead process">
**Terminal consumer**: <file:line>

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|

## Testing Strategy (this slice)

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|

## Threat Matrix

<Process-integration boundary applies here (reading another process's liveness). Every row
`Applicable` or `N/A: reason`. For the applicable row: expected safe behavior, expected failure
behavior, and the planned RED test.>

## Rollback and Additivity (this slice)

**Choice**: <what reverting looks like for the PID column and the sweep>
**Rationale**: <grounded in what the v7 migration and the sweep actually move>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-c.md` MUST be under 1000 words. Tables over prose. Threat-matrix rows count toward
the budget — keep reasons to one clause.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens A owns**: frontmatter, `LaneMetadata.PacketPath`, packet-body HTTP endpoint,
  `UpdateLaneMetadata` call sites, DAG-wave scope.
- **Lens B owns**: `ProgressEvent`/`LaneProgress` telemetry fields, per-executor decoders, and the
  **exact `runs.pid` column DDL** (type/default) as part of the one v7 migration. You decide how
  the PID is captured and consumed operationally, not the column's SQL type.

## Allowed paths

`openspec/changes/lane-status-observability/design-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`, including `references/threat-matrix.md`. Precedence is **not symmetric**: the skill
wins on *what a design document must contain* (required sections, decision shape, threat-matrix
applicability rule). This packet wins on *how this phase is executed here* (capability split, this
lens's slice, word budget, skeleton, done criteria). The skill's 800-word budget, Engram
persistence step, and phase-summary return block are superseded here; note the conflict in
`## Open Questions`, follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file. The claim is
what YOU assert that range shows. This is the synthesizer's worklist, not a certificate.

| citation | claim |
|---|---|
| `internal/serve/hub.go:213-235` | Hub.Run does an initial pass then loops on a ticker until ctx.Done() |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Decisions 1-5 each state a FINAL choice**, including the exact ticker interval and the
  exact liveness mechanism — no "either could work."
- [ ] **Every applicable threat-matrix row names the expected safe/failure behavior and a planned
  RED test; every `N/A` row states a reason.**
- [ ] **Every decision and file-change row names a terminal consumer with a `file:line` citation**
  that points at real code in this worktree.
- [ ] **`design-lens-c.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The specs do not determine whether the sweep must be conservative (favor leaving a lane
  `running`) or aggressive (favor marking it `failed`) on ambiguous liveness evidence.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Deciding this lens's scope would require designing lens A's or lens B's slice.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- `RegisterRun` (`internal/ledger/runs.go:29-41`) inserts `run_id, feature_id, status, target_ref,
  lane_count, started_at, ended_at` — **no PID column exists on `Run` or in this INSERT today.**
  It is called exactly once in production, at `cmd/lucind-ai/cli.go:314-321`, inside `runDispatch`
  — the `lucind-ai run` process itself, not `serve`. **The PID this change records is the PID of
  that `lucind-ai run` process** (i.e. `os.Getpid()` at the point `runDispatch` calls
  `RegisterRun`) — it is the "driving process" the proposal and spec both refer to, not any child
  executor process and not `serve`'s own PID. `serve` (a long-lived, separate process on the same
  ledger file, per `cmd/lucind-ai/cli.go:723-795`) is the one that later reads this stored PID back
  to ask "is the process that dispatched this run still alive."
- `internal/serve/hub.go:213-235`'s `Hub.Run(ctx)` is the exact shape a sweep needs: one blocking
  initial pass (`h.poll(ctx)` at `:214`), then `for { select { case <-ctx.Done(): return; case
  <-ticker.C: poll again } }`. `defaultPollInterval` is `100 * time.Millisecond`
  (`hub.go:24`) — far tighter than an orphan sweep needs; do not copy the value, only the shape.
  `serveDispatch` already launches `Hub.Run` in its own goroutine: `hub := serve.NewHub(ledg, "",
  serve.HubConfig{}); go func() { _ = hub.Run(ctx) }()` (`cmd/lucind-ai/cli.go:770-773`), between
  `ledger.Open` (`:758`) and `serve.NewHandlerWithConfig` (`:781`). A sweep constructed and
  launched the same way, in the same block, is the path of least novelty in this codebase.
- `lane.Status` (`internal/lane/status.go:8-33`) defines `Running` (`"running"`) and `Failed`
  (`"failed"`) as the two states this sweep transitions between; `Valid()` confirms both are
  legal terminal-check inputs to `ledger.SetStatus`.
- `ledger.SetStatus` (`internal/ledger/ledger.go:452-483`, confirmed by direct read) updates
  `lanes.status` and appends a `lane_status_changed` event in one transaction, returning
  `ErrLaneUnknown` for an unregistered lane. `EventLaneNote` (`ledger.go:443`) is the existing
  event type for the required explanatory note ("orphaned: driving process no longer running" per
  the proposal) — append it via `AppendEvent`, the same two-write shape
  `UpdateLaneMetadata` already uses (`lanes_meta.go:67-77`) for its own audit trail, just as two
  separate calls rather than one transaction unless you find a reason to combine them.
- `internal/run/run.go:355`: `deps.Ledger.SetStatus(ctx, deps.RunID, p.ID, lane.Running, now)` is
  the one seam that puts a lane into `running` in the first place — your sweep is the only other
  writer of a lane's status transition away from `running` outside normal lane completion.
- This is a single-OS (Linux) deployment per prior session context — do not design cross-platform
  PID liveness; state the Linux-only scope explicitly rather than abstracting it away, per the
  proposal's own "Out of Scope: Cross-platform PID-liveness beyond what Open Question 4 settles."

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
accepted); the sweep marks lanes `failed` only — no process supervision or auto-restart; PID reuse
is an accepted, low-priority risk (proposal: "Sweep again when the reassigned PID later dies; do
not invent a second identity check until design" — that instruction is now resolved by whatever
this lens decides, or explicitly deferred, not re-opened as a blocker).

**Out of scope, and including any of it is wrong:** telemetry fields (lens B's), frontmatter/
packet-body (lens A's), `internal/resolve`/`internal/conflicttriage` (an unrelated change), any
heartbeat protocol, any auto-restart of a dead runner.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
