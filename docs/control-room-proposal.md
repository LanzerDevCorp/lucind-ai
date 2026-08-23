# lucind-ai Control Room — Proposal

Status: proposal
Target: replace `internal/serve` (approvals-only UI) with a full operational console
Scope: telemetry schema, capture pipeline, push transport, REST/SSE API, single-page UI

---

## 1. Problem statement

The current web surface is an approvals inbox, not a control room:

- `internal/serve/static/app.js` is 97 lines that poll `GET /api/state` every 2s.
- `GET /api/state` returns exactly four fields: approver, approver rate, one opencode
  command string, and the list of pending approvals.
- `internal/serve/model.go` already implements a rich query model over features,
  integration attempts, leases, overlap evidence, reconciliation requests and
  candidates, and audit events — **none of it is exposed over HTTP.** It is reachable
  only from `lucind-ai feature status` (`cmd/lucind-ai/cli.go:852`).

The requested view ("what is each dispatched agent doing, which SDD phase, which
fan-out group, which model, which SDD flow, which worktrees") cannot be rendered from
the data that exists today, because the ledger does not record most of it.

### What the ledger stores today

`lanes` (`internal/ledger/schema.go:18`):
`run_id, lane_id, packet_id, executor, routing_condition, status, worktree_path,
worktree_preserved, attempt, started_at, ended_at`.

`events` (`internal/ledger/schema.go:34`) with a frozen CHECK constraint admitting only
six types: `run_started, lane_registered, lane_status_changed, lane_note,
barrier_released, run_ended`.

### The five gaps

| # | Gap | Consequence |
|---|-----|-------------|
| G1 | No `runs` table. `run_id` exists only as a column on `lanes` and `events`. | No run metadata: no change id, no wave index, no packet count, no start/end, no dispatching command. Runs cannot be listed. |
| G2 | `lanes` has no `model`, `agent`, `sdd_phase`, `fanout_group`, `feature_id`, `change_id`, `wave`, or `allowed_paths`. `dag.Node` carries `Model`, `Agent`, `Feature`, `ParentRef`, `BaseSHA` — none is persisted. | The fleet view, the fan-out grouping, and the per-model breakdown are unrenderable. |
| G3 | `executor.Outcome.Stdout/Stderr` is consumed for routing and discarded. Executors run `--output-format json`, which returns one blob at process exit. | Zero visibility into a lane between `running` and its terminal status. A 20-minute lane is a spinner for 20 minutes. |
| G4 | No push transport. HTTP polling of one aggregate endpoint. | Latency and O(clients x fullstate) cost; no event ordering guarantees. |
| G5 | `events.type` CHECK is closed. | Any new event type requires a table-rebuild migration (the shape `migrateV1ToV2DDL` already uses). |

**G3 is the one that decides whether this project is a dashboard or a control room.**
`plugin/claude-code/skills/lucind-ai/references/runtime.md:59` already documents the
answer: `stream-json` for live progress. Switching the executors to NDJSON streaming and
decoding it per lane is what turns "status: running" into "editing internal/serve/hub.go,
tool call 14, 3 files touched".

---

## 2. Architecture — five layers

```
  agy / cursor-agent / opencode  (stream-json NDJSON on stdout)
            |
  [L2] Capture: per-lane decoder goroutine -> throttled progress events
            |
  [L1] Telemetry: ledger schema v6 (runs, lane_progress, lanes columns, open events)
            |
  [L3] Transport: event tailer -> SSE hub -> N subscribers (cursor = events.id)
            |
  [L4] API: REST snapshots + /api/stream SSE deltas + guarded control endpoints
            |
  [L5] UI: zero-build ES modules, go:embed, six views over one live store
```

### L1 — Telemetry (ledger schema v6)

New migration `migrateV5ToV6DDL`, following the existing create-copy-drop-rename shape
used by `migrateV1ToV2DDL` and `migrateV4ToV5DDL` (STRICT tables cannot ALTER a CHECK).

**New table `runs`**

```sql
CREATE TABLE IF NOT EXISTS runs (
  id           TEXT PRIMARY KEY,
  change_id    TEXT NOT NULL DEFAULT '',
  sdd_phase    TEXT NOT NULL DEFAULT '',   -- explore|propose|design|spec|tasks|apply|verify|archive
  wave         INTEGER NOT NULL DEFAULT 0, -- index within the apply DAG, 0 for planning fan-out
  feature_id   TEXT NOT NULL DEFAULT '',
  parent_ref   TEXT NOT NULL DEFAULT '',
  base_sha     TEXT NOT NULL DEFAULT '',
  lane_count   INTEGER NOT NULL DEFAULT 0,
  status       TEXT NOT NULL CHECK (status IN ('running','done','failed','partial')),
  started_at   TEXT NOT NULL,
  ended_at     TEXT
) STRICT;
```

**`lanes` rebuild — new columns** (all `NOT NULL DEFAULT ''`, so every existing row
migrates verbatim):
`model, agent, sdd_phase, fanout_group, change_id, feature_id, allowed_paths (CSV),
depends_on (CSV), body_sha256`.

`fanout_group` is the field that makes the lens grouping first-class: `lens-a`, `lens-b`,
`lens-c`, `synthesis`, or `''` for a solo lane. Today the 3+1 convention lives only in
the 20 asset templates under `plugin/claude-code/skills/lucind-ai/assets/` and is
invisible to the machine. Persisting it is what lets the UI draw the fan-out as a group
instead of four unrelated cards.

**New table `lane_progress`** — the live feed, append-only, one row per decoded stream
event:

```sql
CREATE TABLE IF NOT EXISTS lane_progress (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id    TEXT NOT NULL,
  lane_id   TEXT NOT NULL,
  seq       INTEGER NOT NULL,
  kind      TEXT NOT NULL,          -- tool_call|tool_result|assistant_text|file_edit|usage|error
  tool      TEXT NOT NULL DEFAULT '',
  target    TEXT NOT NULL DEFAULT '',
  summary   TEXT NOT NULL DEFAULT '',
  tokens_in INTEGER NOT NULL DEFAULT 0,
  tokens_out INTEGER NOT NULL DEFAULT 0,
  cost_usd  REAL NOT NULL DEFAULT 0,
  at        TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_lane_progress_lane ON lane_progress(run_id, lane_id, seq);
CREATE INDEX IF NOT EXISTS idx_lane_progress_id ON lane_progress(id);
```

**`events` rebuild — open the CHECK.** Replace the closed six-value CHECK with a
non-empty check (`length(trim(type)) > 0`). Rationale: the CHECK was buying type safety
that Go-side validation already provides (`RegisterLane` validates status in Go before
any write — same pattern applies), and it makes every future event type a table rebuild.
Add `run_started_wave`, `lane_progress`, `feature_promoted`, `lease_renewed`,
`reconciliation_opened` without another migration.

**Retention.** `lane_progress` is high-volume. Reuse the existing
`PruneIntegrationEvents` pattern: `PruneLaneProgress(ctx, cutoff)` plus a per-lane cap
(keep last N=2000 rows per lane, roll up the rest into a single `summary` row). A
`lucind-ai ledger prune` subcommand exposes it.

### L2 — Capture pipeline

`internal/executor` gains a streaming path:

```go
type Request struct {
    // ... existing fields
    Progress chan<- ProgressEvent // nil = today's blocking behavior, unchanged
}

type ProgressEvent struct {
    Seq       int
    Kind      string
    Tool      string
    Target    string
    Summary   string
    TokensIn  int
    TokensOut int
    CostUSD   float64
    At        time.Time
}
```

When `Progress != nil`, `Agy.Run` swaps `--output-format json` for
`--output-format stream-json`, attaches a `bufio.Scanner` to stdout, decodes each NDJSON
line, and emits a `ProgressEvent`. The final message still produces the same `Outcome`,
so **routing and every existing test keep working unchanged** — this is strictly
additive.

Per-executor decoders (`agy_stream.go`, `cursor_stream.go`, `opencode_stream.go`) behind
one `StreamDecoder` interface. An executor whose stream shape is unknown or that fails to
produce parseable NDJSON degrades gracefully to today's blocking JSON path and logs one
`lane_note`. **No executor is allowed to fail a lane because its telemetry did not
parse.** Telemetry is observation, never control flow.

`internal/run` owns the writer side: one goroutine per lane drains the channel and writes
to `lane_progress` in batches (flush every 250ms or 32 events, whichever first) to keep
SQLite write pressure bounded when 8 lanes stream concurrently. WAL is already implied by
the concurrent-run design; set `busy_timeout=5000` and `synchronous=NORMAL` explicitly.

### L3 — Transport

`internal/serve/hub.go`: a tailer goroutine polls `SELECT ... FROM events WHERE id > ?`
and `lane_progress WHERE id > ?` every 200ms (SQLite has no LISTEN/NOTIFY; a 200ms tail
on an indexed integer cursor is cheap and correct), and fans results out to subscribers
over buffered channels.

`GET /api/stream?since=<event_id>` is Server-Sent Events. SSE over WebSocket because the
data is strictly server-to-client, it survives proxies, reconnects natively with
`Last-Event-ID`, and needs no new dependency. Each frame carries `id:` = the ledger row
id, so a reconnecting client resumes exactly where it stopped — no gaps, no replays.

Slow-consumer policy: a subscriber whose buffer fills is dropped and sent one
`event: resync` frame; the client refetches the snapshot. Never block the tailer.

The existing 2s polling of `/api/state` stays as a fallback so the UI degrades instead of
breaking if SSE is unavailable.

### L4 — API surface

Read (all JSON, all derived from `serve.Model` extended):

```
GET  /api/state                      (kept, unchanged shape — back-compat)
GET  /api/overview                   fleet summary: counts by status/executor/model/phase
GET  /api/runs                       ?status=&change=&feature=
GET  /api/runs/{run}                 run + its lanes + wave position
GET  /api/runs/{run}/lanes/{lane}    lane detail
GET  /api/runs/{run}/lanes/{lane}/progress?since=  live feed for one lane
GET  /api/flows                      SDD flows: change_id -> phases -> fanout groups
GET  /api/flows/{change}/dag         parsed apply-dag.yaml -> waves + edges
GET  /api/features                   (exists in Model, never exposed)
GET  /api/features/{id}              + attempts + lease + audit events
GET  /api/leases                     with live TTL countdown
GET  /api/reconciliations            requests + candidates
GET  /api/worktrees                  path, branch, lane, disk bytes, stale flag
GET  /api/stream?since=              SSE
```

Control (guarded — see Security):

```
POST /approvals/{run}/{lane}         (exists, individual-only; bulk stays 400)
POST /approvals/{run}/{lane}/defect  (exists)
POST /api/reconciliations/{id}/approve | /decline
POST /api/features/{id}/renew        lease renewal from the UI
POST /api/worktrees/cleanup          wraps `worktree cleanup`
POST /api/runs                       dispatch the next wave  [PHASE 2 — see Security]
```

### L5 — UI

**No build step.** Vanilla ES modules + modern CSS, served from `go:embed static/*`
exactly as today. Rationale: `make install` must stay the single command that produces a
correct binary (CLAUDE.md is explicit about stale binaries). Adding npm/vite to the build
introduces a second artifact that can silently go stale — the precise failure mode this
project already got burned by. Modern browsers give us ES modules, CSS nesting,
`container-queries`, and `<dialog>` natively; a framework buys nothing here.

Six views, one shared live store (`store.js` applies SSE deltas to an in-memory model;
every view is a pure render of that store):

1. **Fleet** (default). One card per dispatched lane, live. Shows: lane id, executor badge
   (agy / cursor-agent / opencode / human), **model**, **SDD phase**, **fan-out group**
   (lenses grouped visually into one bordered cluster with their synthesizer), feature,
   worktree path, attempt number, elapsed timer, status ring, and — the point of the whole
   project — **the current activity line**: latest `lane_progress` row rendered as
   "Edit internal/serve/hub.go" / "Bash go test ./..." / "thinking", plus a sparkline of
   tool-call rate and a running token/cost counter.
2. **DAG canvas**. `apply-dag.yaml` rendered as waves (columns) x packets (nodes) with
   `depends_on` edges, each node live-colored by lane status. This is the view that makes
   "is my parallelism real?" answerable at a glance: a wave whose nodes all light up
   simultaneously is real parallelism; a staircase is not. Overlap violations from
   `dag.ValidateGlobalOverlap` render as red edges.
3. **SDD flows**. Per change id, a horizontal rail
   `explore -> propose -> design -> spec -> tasks -> apply -> verify -> archive`, each
   phase expandable into its 3 lenses + synthesis. Answers "which SDD flow is this agent
   serving" directly.
4. **Features**. Swimlanes, one per active feature, N in parallel. Per lane: parent_ref,
   base_sha, lease holder + fence + **live TTL countdown** (the renewal work from
   commit 60694ad becomes visible), the integration attempt state machine
   (`recorded -> leased -> combining -> checking -> cas_pending -> promoted`) as a stepper,
   and overlap/reconciliation badges.
5. **Timeline**. Global merged stream of `events` + `integration_events` +
   `lane_progress`, virtualized, filterable by run / lane / feature / type. The forensic
   view for "what happened at 03:14".
6. **Approvals**. Today's inbox, restyled, with the evidence block and the
   individual-decision-only guarantee intact.

Plus a persistent **top bar**: live counts (running / blocked / failed), aggregate
tokens + cost for the session, active worktree count, and a connection indicator for the
SSE stream.

Design direction: dark-first, monospace for every identifier, one accent per executor,
status carried by both color and shape (never color alone), density over decoration.
Whatever is on screen must be true; nothing renders a value the ledger does not hold.

---

## 3. Delivery as an apply DAG (the dogfood)

This proposal is deliberately shaped so that building it exercises exactly the machinery
being visualized: fan-out planning phases, then a multi-wave apply DAG with disjoint
`allowed_paths`, then feature integration.

**Planning**: one SDD flow (`control-room`), all five phases with 3 agy lenses + 1
cursor-agent synthesizer. Nothing new here — the assets already exist.

**Apply DAG** — 6 waves, disjoint paths, maximum width 4:

| Wave | Packets (parallel) | allowed_paths |
|------|--------------------|---------------|
| W1 | `schema-v6` | `internal/ledger/schema.go` |
| W2 | `ledger-runs` / `ledger-progress` / `ledger-lanes` | `internal/ledger/ledger.go` split by new file: `runs.go` / `progress.go` / `lanes_meta.go` |
| W3 | `exec-stream-agy` / `exec-stream-cursor` / `exec-stream-opencode` / `run-writer` | `internal/executor/agy_stream.go` / `cursor_stream.go` / `opencode_stream.go` / `internal/run/progress.go` |
| W4 | `serve-model` / `serve-hub` / `serve-handlers` / `serve-worktrees` | `internal/serve/model_ext.go` / `hub.go` / `handlers_api.go` / `worktrees.go` |
| W5 | `ui-shell` / `ui-fleet` / `ui-dag` / `ui-flows` | `static/index.html`+`app.css` / `static/views/fleet.js` / `views/dag.js` / `views/flows.js` |
| W6 | `ui-features` / `ui-timeline` / `ui-approvals` / `cli-wiring` | `views/features.js` / `views/timeline.js` / `views/approvals.js` / `cmd/lucind-ai/cli.go` |

Note W2's shape: `ledger.go` is a single 1400-line file, so three packets cannot edit it
disjointly. Splitting the new code into new files per packet is what makes the wave legal
under `packet.DisjointAllowedPaths` — a real constraint the DAG author has to design for,
and exactly the kind of thing the DAG canvas will make visible.

---

## 4. Risks and honest constraints

**R1 — SQLite write contention.** N concurrent runs x M lanes each streaming progress is
a lot of small writes to one file. Mitigation: batched flush (250ms / 32 rows), WAL,
`busy_timeout=5000`, and progress writes are best-effort — a failed progress insert logs
and drops, it never fails a lane. If this proves insufficient under real load, the
fallback is a per-run progress file tailed by the server instead of a shared table.

**R2 — Stream format drift.** `agy --output-format stream-json` is documented in this
repo's own runtime reference, but the exact NDJSON schema is an external contract that
can change, and cursor-agent / opencode shapes are unverified. Mitigation: decoders are
tolerant (unknown message types become a generic `assistant_text` event or are skipped),
and a decode failure degrades that lane to today's blocking path. Verify each executor's
actual stream shape empirically before writing its decoder — do not write a decoder from
an assumed schema.

**R3 — Security.** `serve.ListenAndServe` refuses any non-loopback bind
(`internal/serve/server.go:20`), and the API is unauthenticated by design because it is
localhost-only. Read-only endpoints keep that property safely. **Dispatch control
(`POST /api/runs`) does not**: it turns an unauthenticated local port into arbitrary
agent execution reachable by any process on the machine, including a browser page on
another site issuing a cross-origin POST. Recommendation: ship v1 read-only + the
existing approval/reconciliation decisions; gate dispatch behind an explicit
`--enable-dispatch` flag plus an origin check and a per-session token printed to stderr
at startup. Do not enable it by default.

**R4 — Scope.** This is six waves of work. The single highest-value slice, if only one
thing ships, is W1 + W3 + the Fleet view: persisted per-lane activity plus a live grid.
Everything else is composition on top of that spine.

---

## 5. Out of scope (named explicitly)

- Automatic wave advancement (running the whole DAG unattended). That is an orchestrator
  capability, not a UI one, and it deserves its own change.
- Automatic merge of finished feature branches into main. Same reason.
- Multi-user / remote access. Loopback-only stands.
