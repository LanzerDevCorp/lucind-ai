---
id: apply-lane-status-observability
executor: cursor-agent
routed_by: single-packet sequential apply of an accepted three-phase tasks checklist under strict TDD
model: cursor-grok-4.6-high
allowed_paths: ["internal/ledger/schema.go", "internal/ledger/schema_test.go", "internal/ledger/progress.go", "internal/ledger/progress_test.go", "internal/packet/packet.go", "internal/packet/packet_test.go", "internal/ledger/lanes_meta.go", "internal/ledger/lanes_meta_test.go", "internal/ledger/runs.go", "internal/ledger/runs_test.go", "cmd/lucind-ai/cli.go", "cmd/lucind-ai/cli_test.go", "internal/executor/executor.go", "internal/executor/agy_stream.go", "internal/executor/agy_stream_test.go", "internal/executor/claude_stream.go", "internal/executor/claude_stream_test.go", "internal/executor/opencode_stream.go", "internal/executor/opencode_stream_test.go", "internal/executor/cursor_agent.go", "internal/executor/cursor_agent_stream_test.go", "internal/run/run.go", "internal/run/run_test.go", "internal/run/batch.go", "internal/serve/model.go", "internal/serve/model_test.go", "internal/serve/handlers.go", "internal/serve/server_test.go", "internal/serve/static/app.js", "internal/serve/static_test.go", "internal/serve/sweeper.go", "internal/serve/sweeper_test.go", "openspec/changes/lane-status-observability/tasks.md"]
---

# Packet apply-lane-status-observability

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/apply-lane-status-observability  ·  **Branch:** lucind/apply-lane-status-observability

## Goal

Implement every task in `openspec/changes/lane-status-observability/tasks.md`, in its stated
dependency order, as three sequential work-unit commits (Phase 1: schema/frontmatter/metadata/PID
column; Phase 2: CLI capture/decoders/dispatch wiring; Phase 3: serve/UI/sweeper). When you are
finished the repository contains schema v7 (`runs.pid`, `lane_progress` telemetry columns),
extended packet frontmatter (`sdd_phase`, `fanout_group`, `skill`) with a persisted `Packet.Path`
and lane-metadata snapshot, decoder-populated progress telemetry broadcast in real time, a
`GET /api/packets/{runID}/{laneID}` endpoint serving raw packet markdown, and a serve-side sweeper
that reconciles orphaned lanes (dead driving process) to `failed` on a 10s ticker — with
`./lucind-checks.sh` green on the combined tree and every checkbox in `tasks.md` ticked.

## Why this is safe to dispatch now

The proposal, spec, design and tasks for this change are all accepted and present in this
worktree. `tasks.md` is the canonical checklist, synthesized from three lens drafts and promoted
after a human comparison; its citations were verified against this repository, including the
verbatim schema v7 DDL (`tasks.md`'s "Schema v7 DDL" section, sourced from `design.md:113-161`).

## Preconditions

- `openspec/changes/lane-status-observability/tasks.md`, `design.md`, `proposal.md` and `specs/`
  all exist in this worktree.
- `./lucind-checks.sh` is green before you start. If it is not, return `blocked` — you did not
  break it and must not repair someone else's failure inside this packet.
- `internal/ledger/schema.go:10` reads `const schemaVersion = 6` — this packet bumps it to 7.
- `internal/serve/sweeper.go` does not yet exist.

**A precondition satisfied by one of this packet's own later steps is a misordered packet.**
Return `blocked` and say so; do not work around it.

## Required procedure

`tasks.md` is the specification for this packet. Read it first, in full, and follow its Phase
1 → 2 → 3 ordering and its Dependency Order table. Do not re-derive its decisions. The verbatim
`migrateV6ToV7DDL` SQL is embedded in `tasks.md`'s "Schema v7 DDL" section — copy it exactly,
do not rewrite it from `design.md` a second time.

### Strict TDD is mandatory

Every task marked RED in `tasks.md` writes a failing test **before** the production code that
satisfies it. For each RED task you must actually observe the failure — run the focused test
command, see it fail for the intended reason, and only then write the GREEN production code that
follows it. `tasks.md` already pairs every RED task with its GREEN task adjacently (e.g. 1.1/1.2,
1.3/1.4, 2.1/2.2 ...); follow that pairing in order, do not collapse a RED+GREEN pair into work
done before the RED test is observed failing, and do not batch multiple RED tasks ahead of their
GREEN counterparts.

A test that passes the moment you write it is not RED. If a RED task's test passes immediately,
that is a finding, not a formality: say so in the envelope's notes and explain what already
satisfied it. Do not weaken the test to manufacture a failure, and do not silently proceed as if
it had failed.

### Three commits, not one

One commit per work unit (Phase 1, Phase 2, Phase 3), in order 1 → 2 → 3. Each commit contains
its phase's RED tests and the GREEN production that satisfies them, so the combined tree is
green at every commit — Integrate checks the combined tree (`internal/run/integrate.go:50-59`),
and a unit that is red on its own is a unit that cannot be reverted cleanly.

Unit 3 (Phase 3) must not begin until Unit 1's v7 migration has actually landed in this worktree's
history — `internal/ledger/schema.go`'s `schemaVersion` must already read `7` and `runs.pid` must
already exist before any Phase 3 task starts, because the sweeper persists lane failures keyed off
that column. Since all three phases run sequentially inside this one worktree, this is naturally
satisfied by following the phase order — do not skip ahead.

Shared-file edit ordering within this packet (do not reorder across phases):
- `cli.go`: `Packet.Path` wiring at `:160-174` (Phase 2, task 2.2) → `PID: os.Getpid()` at
  `:314-324` (Phase 2, task 2.4) → Sweeper launch at `:770-774` (Phase 3, task 3.15).
- `model.go`: `Lane`/`laneDTO` `skill`/`packet_path` fields at `:163-184,:322-333` (Phase 3, task
  3.2) → `LaneProgress`/`tool_rate` at `:186-193,:336-346` (Phase 3, task 3.4).
- `app.js` is Phase 3 only (task 3.7/3.8); no earlier phase touches it.

Before each commit, run the phase's focused test commands (below), then `./lucind-checks.sh` on
the full tree. Conventional commit messages. **No Co-Authored-By and no AI attribution of any
kind.**

Tick each task's checkbox in `tasks.md` as you complete it. `tasks.md` is in `allowed_paths` for
exactly this reason and for no other — do not edit its content, ordering, or citations.

### Focused test commands per phase

- **Phase 1** (tasks 1.1–1.10): `go test ./internal/ledger ./internal/executor ./internal/run ./internal/serve -count=1`
- **Phase 2** (tasks 2.1–2.16): `go test ./internal/packet ./internal/ledger ./internal/run ./internal/serve ./cmd/lucind-ai -count=1`
- **Phase 3** (tasks 3.1–3.15): `go test ./internal/ledger ./internal/serve ./cmd/lucind-ai -count=1`

These are the same commands the orchestrator will re-run independently after integration; make
sure each phase is actually green under its own command before committing it, not just under a
narrower `-run` filter.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** For each
      type, function, flag or config key added, name the program, test or file that *reads* it
      and attach the output that proves it. In particular: `Packet.Path` must be consumed by
      `UpdateLaneMetadata`/the packet GET handler; `Run.PID` must be consumed by `RegisterRun` and
      the sweeper's liveness probe; `Sweeper` must be consumed by `serveDispatch` (launched, not
      merely defined); `tool_rate` must be consumed by `GetLaneProgress`'s JSON output, not just
      computed and discarded.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and
      `git log --oneline -3`. Three conventional commits, one per phase, no AI attribution.
- [ ] **`./lucind-checks.sh` exits 0 on the combined tree.** Attach the tail of its output.
- [ ] **Every RED task was observed failing before its GREEN.** For each, name the focused test
      command and the failure message you saw. Where a RED test passed immediately, say which one
      and what already satisfied it.
- [ ] **Schema v7 migration is real and round-trips.** Evidence: the output of
      `go test ./internal/ledger -run 'Migrate|Schema' -count=1 -v`.
- [ ] **The sweeper's four adversarial cases are covered and pass.** Evidence: the output of
      `go test ./internal/serve -run 'TestSweeper_' -count=1 -v` (live PID retained, dead PID
      reconciled to `failed` with the orphan note, PID 0 skipped, `EPERM`/recycled PID kept
      alive).
- [ ] **Every checkbox in `openspec/changes/lane-status-observability/tasks.md` is ticked**, and
      no other line of that file changed. Evidence: `git diff` of that file against the lane's
      birth point showing only `- [ ]` → `- [x]`.

## Allowed paths

Only these may be created or modified. Touching anything else is a **deviation** — finish nothing
further, report it, and stop.

- `internal/ledger/schema.go`, `schema_test.go`, `progress.go`, `progress_test.go`,
  `lanes_meta.go`, `lanes_meta_test.go`, `runs.go`, `runs_test.go`
- `internal/packet/packet.go`, `packet_test.go`
- `cmd/lucind-ai/cli.go`, `cli_test.go`
- `internal/executor/executor.go`, `agy_stream.go`, `agy_stream_test.go`, `claude_stream.go`,
  `claude_stream_test.go`, `opencode_stream.go`, `opencode_stream_test.go`, `cursor_agent.go`,
  `cursor_agent_stream_test.go`
- `internal/run/run.go`, `run_test.go`, `batch.go`
- `internal/serve/model.go`, `model_test.go`, `handlers.go`, `server_test.go`,
  `static/app.js`, `static_test.go`, `sweeper.go` (new), `sweeper_test.go` (new)
- `openspec/changes/lane-status-observability/tasks.md` (checkboxes only)

## Allowed paths outside the repository

None. This packet may touch nothing outside the repository.

## Out of scope

- `apply-dag.yaml`. `tasks.md` decided against a sidecar; this is one sequential packet.
- DAG wave metadata (`design.md`'s Decision "DAG wave metadata is a follow-up (OQ5)") — do not
  implement it.
- The non-decreasing overlap risk formula / thresholds, or any change to
  `internal/overlap/overlap.go`.
- Any second identity check beyond `kill(pid, 0)` for recycled PIDs — the design explicitly
  accepts that a recycled live PID is kept until that PID itself dies (threat-matrix row
  "Recycling / EPERM", `tasks.md:113`).
- Live/production LLM calls or DAG dispatch of any kind from inside this worktree.

## Hard stops

Stop and return `status: blocked` — do not guess. **Declare every one of these in the envelope**,
whether or not it fired.

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and `tasks.md` does not say which.
- Satisfying one instruction in this packet would require violating another.
- Unit 3's sweeper cannot be implemented without a persisted `runs.pid` column already present
  (i.e. Unit 1's migration did not actually land before Unit 3 starts).
- A task would require resolving an open question `design.md` marks as deliberately deferred
  (e.g. DAG wave metadata).

## Context

Established facts, already verified — do not re-derive them.

- `tasks.md` is canonical and its citations were verified against this repository during
  synthesis. `tasks-synthesis-notes.md` records dropped citations, coverage gaps and
  decomposition divergence; read it if a task seems to contradict a lens draft.
- Current baseline: `internal/ledger/schema.go:10` is `const schemaVersion = 6`. No Go source has
  changed since the last binary build at this worktree's base — the installed `lucind-ai` binary
  is current.
- `Hub.Run` (`internal/serve/hub.go:213-235`) is the pattern to follow for the sweeper's
  goroutine/ticker shape (immediate sweep then 10s ticker), not `defaultPollInterval`
  (`hub.go:24`).
- `SetStatus` (`internal/ledger/ledger.go:452-484`) and `EventLaneNote`
  (`internal/ledger/ledger.go:366-378,440-446`) are the existing primitives the sweeper uses to
  fail an orphaned lane and attach its "orphaned: driving process no longer running" note —
  `lane.Failed` is defined at `internal/ledger/status.go:11-17`.
- The two-segment path parse and `/api/` 404 fallback the packet-body handler must reuse already
  exist at `internal/serve/handlers.go:316-350,352-354`.
- `./lucind-checks.sh` is the full-tree check. `./lucind-lane-check.sh` is the mechanical
  self-check for lane artifacts and is not needed here.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how
well the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
