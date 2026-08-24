# Explore: Lane Status Observability

The live-status dashboard (`internal/serve`) renders most lane-card fields as literal
"Unavailable", and lanes can stay shown as "Running" for 25+ hours after their driving
process and worktree are gone. Six related gaps, one change.

## Problem

`ledger.LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`) already carries
`Model, Agent, SDDPhase, FanoutGroup, Change, Feature, AllowedPaths, Dependencies, BodyDigest`,
and `UpdateLaneMetadata`/`GetLaneMetadata` (`lanes_meta.go:39,89`) already read/write it —
`model`/`agent`/`feature` through real schema-v6 columns, `SDDPhase`/`FanoutGroup`/`Change`
through a `lane_metadata:v1:` JSON audit-event in `events`. `serve.Lane`
(`internal/serve/model.go:163-184`) and `app.js:532-538` already consume it end to end.
**`UpdateLaneMetadata` has zero production callers** — only test files call it. The two real
dispatch sites, `internal/run/run.go:334` (`Execute`) and `internal/run/batch.go:184`
(`ensureLaneFailed`), call `RegisterLane` and never follow up. "Unavailable" is therefore an
honest render of empty data, not a display bug.

`packet.Parse` (`internal/packet/packet.go:78-167`) recognizes only
`id, executor, routed_by, model, agent, read_only, feature, parent_ref, base_sha,
expected_parent_sha, legacy_main, allowed_paths` — no `sdd_phase`, `fanout_group`, or
`skill` key exists anywhere, confirmed against the `.lucind/packets/*.md` corpus and
`plugin/claude-code/skills/lucind-ai/SKILL.md:18-36`. The fan-out-lens lanes the dashboard
was showing (`design-/spec-conflict-triage-fixture-lens-*`) are hand-authored packets
dispatched via repeated `--packet <path>` flags (`cmd/lucind-ai/cli.go:141-166`), a separate
mechanism from `internal/dag/emit.go`'s DAG-wave `Emit`/`EmitPacketContent` (used only by
`lucind-ai split --dag`). `Packet` has no `Path` field either — only `cli.go:160-166`'s
`packetFlags[i]` ↔ `ps[i]` index-aligned slices know the on-disk path.

Structured token/cost usage is parsed but discarded into prose: `agy_stream.go:12-18`
(`agyUsage`), `claude_stream.go` (`claudeUsage`+`costUSD`), and `opencode_stream.go:100-113`
(`opencodeTokens`+`Cost`) all parse real numbers before formatting them into
`ProgressEvent.Message` strings (`executor.go:17-21` has no numeric fields). Only
`cursor_agent.go` genuinely has no usage struct. Reaching the live dashboard needs more than
an executor-layer change, though: the SSE hub reads from durable ledger cursors
(`ledger.LaneProgress`, `internal/ledger/progress.go:15-20`), backed by a STRICT
`lane_progress` table (`schema.go:298-307`) with no usage/tool-call columns — a v7 migration
is required, following the create-copy-drop-rename pattern `migrateV5ToV6DDL` already used
for v6.

Live "skill" telemetry from the executor is not obtainable at all: `agy` runs
`gemini-3.7-flash-high`, `cursor-agent` runs `cursor-grok-4.6-high`, `opencode` runs
`openai/gpt-5.6-sol` — none are Claude Code, none have a "Skill" tool concept, only generic
tool calls (Read/Write/Edit-equivalents) which the same three decoders already semi-parse.
Only `internal/executor/claude.go` (pinned to `claude-opus-5`) is real Claude Code, and it
doesn't drive these fan-out lanes.

Finally, no PID or heartbeat is stored for a run or lane anywhere (`ledger.Run`,
`runs.go:16-24`, has no PID field), and no reconciliation/orphan-sweep code exists anywhere
in the codebase. A lane a driving process abandons mid-flight (killed terminal, machine
sleep) simply stays `running` in SQLite forever; `serve` faithfully renders that stale state.
`runs` is also STRICT, so a new `pid` column needs the same v7 migration as the usage
columns above — the two can share one migration.

## Candidates

| Approach | Pros | Cons | Feasibility |
|---|---|---|---|
| **1. Wire existing metadata path + add PID sweep + structured telemetry, one PR** (recommended, user-selected) | Fixes all six gaps together; metadata items (#1-4) are near-zero-risk "wire the missing caller"; single coherent review | Likely exceeds the 1200-line review budget — needs an explicit `size:exception` | High |
| 2. Split into two PRs (metadata/observability first, telemetry+recovery second) | Isolates the one real schema migration and the one new subsystem (PID sweep) for focused review; PR1 ships fast with zero migration risk | Explore's own recommendation; user explicitly declined it in favor of Candidate 1 | High |
| 3. Metadata wiring only, defer telemetry/reconciliation to a follow-up change | Smallest possible slice; no migration at all | Leaves the 25h+ stuck-lane bug (item #6) and tokens/cost unresolved — user wants both in scope now | Low (under-scopes user's ask) |

Recommend **Candidate 1**, per explicit user decision: ship all six items as one PR under
`size:exception`, having been shown and having accepted the review-budget risk that made
Candidate 2 the exploration's own default recommendation.

## User and capability impact

- **Operators watching the live dashboard** get real `MODEL`/`SDD PHASE`/`FANOUT GROUP`
  values instead of "Unavailable" for every future dispatch (existing ledger rows for
  already-run lanes stay historically empty — no backfill in scope).
- **Operators** get a link per lane to the exact packet body that was dispatched to that
  lane's executor (new `internal/serve` endpoint, packet content served by lane).
- **Operators** see which skill/SDD-phase authored a lane's packet (new `skill:` frontmatter
  key, static, set by the authoring orchestrator — not runtime telemetry).
- **Operators** see token/cost usage and generic tool-call activity for `agy`, `claude`, and
  `opencode` lanes (not `cursor-agent`, which has no usage data to surface).
- **Operators** stop seeing lanes stuck at "Running" for hours/days after their driving
  process died — `serve` sweeps orphaned lanes to `failed` with a clear reason, on startup
  and periodically thereafter.

## Scenarios

1. **Metadata reaches the dashboard.** A lane dispatched via `internal/run.Execute` calls
   `RegisterLane` then `UpdateLaneMetadata` with `Model`/`Agent`/`Feature` from the packet.
   `serve`'s `ListLanes` renders real values, not "Unavailable".
2. **New frontmatter drives SDD Phase / Fanout Group / Skill.** A hand-authored packet under
   `.lucind/packets/` declares `sdd_phase:`, `fanout_group:`, and `skill:` in frontmatter;
   `packet.Parse` reads them; they flow through the same `UpdateLaneMetadata` call as #1
   (audit-event JSON, no new migration for these three).
3. **Packet link.** A lane's dashboard card links to a new endpoint that serves the exact
   `.md` packet body dispatched to that lane, sourced from `cli.go`'s already-index-aligned
   `packetFlags[i]`/`ps[i]`.
4. **Structured usage for agy/claude/opencode.** `ProgressEvent`/`LaneProgress` gain optional
   numeric usage and generic tool-call fields; the three decoders that already parse real
   numbers populate them alongside their existing prose `emit()` calls; `cursor-agent` leaves
   them empty. A v7 STRICT-table migration adds the needed columns to `lane_progress`.
5. **Orphaned lane reconciliation.** `RegisterRun` stores the driving process's PID (new
   `runs.pid` column, same v7 migration). On `serve` startup, and on a periodic ticker
   thereafter, any run whose stored PID is no longer alive has its still-`running` lanes
   marked `failed` via `SetStatus` + an `AppendEvent` note ("orphaned: driving process no
   longer running"). `serve` and `lucind-ai run` are separate processes against the same
   ledger file — the sweep checks PID liveness, not process identity.

## Open questions

- [ ] Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`,
      `skill` vs `generated_by`.
- [ ] Packet path persistence: new `LaneMetadata.PacketPath` field (audit-event JSON, no
      migration) vs. a real `lanes` column (migration, but queryable/indexable).
- [ ] Ticker interval for the periodic orphan sweep.
- [ ] PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and whether
      cross-platform portability beyond Linux is in scope.
- [ ] Whether `internal/dag/parse.go`'s `Node`/`internal/dag/emit.go`'s `EmitPacketContent`
      (the DAG-wave path, lower priority than the hand-authored path) get the same new
      fields in this change or a follow-up.
