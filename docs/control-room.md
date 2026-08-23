# Control Room

A browser console for `lucind-ai`, served from the binary itself. It shows what the
dispatcher is doing — batches and waves, feature leases, approvals, lane envelopes,
reconciliation candidates — without shelling out to git or the CLI to find out.

Built across six changes between 2026-08-20 and 2026-08-22, all dispatched through
`lucind-ai` itself. This document is the durable record: what shipped, how it was
built, and where the raw artifacts live.

## Running it

```sh
lucind-ai serve --addr :8080
```

Read-only by default. Every mutating route answers 403 until dispatch is explicitly
enabled:

```sh
lucind-ai serve --addr :8080 --enable-dispatch --dispatch-token <token>
```

With `--enable-dispatch`, mutations additionally require a same-origin request and a
constant-time bearer token match. The default-off posture is deliberate: the console
is meant to be safe to leave open.

## What it serves

The client runs off one composed `GET /api/state` snapshot plus a `GET /api/stream`
SSE feed, falling back to a 2-second poll when the stream drops. Individual read
routes exist alongside it for direct queries:

| Route | Returns |
|---|---|
| `GET /api/state` | Composed snapshot the client boots from |
| `GET /api/stream` | SSE feed of live updates |
| `GET /api/approvals` | Pending approvals |
| `GET /api/features` · `/api/features/{id}` | Feature records |
| `GET /api/features/{id}/attempts` · `/events` · `/lease` · `/overlap` | Per-feature detail |
| `GET /api/leases` | Every lease, with live TTL |
| `GET /api/batch/{id}/lanes` | Lane lifecycle and barrier outcome for a run |
| `GET /api/attempts/{id}` · `/api/candidates/{id}` | Attempt and candidate detail |
| `GET /api/reconciliations/{id}` · `/candidates` | Reconciliation requests |
| `POST /approvals/{id}/{decision}` | Individual approval decision |

Approvals are individual by contract, at both layers. A request body carrying an array
or multiple items is rejected with 400, and the UI offers no bulk control — so neither
surface can be used to approve a batch in one gesture.

The static assets are embedded via `StaticFS()`; there is no build step and no npm
dependency. Client state patches keyed cards in place rather than rewriting containers,
so expanding a card survives the next tick. All interpolation goes through text nodes
rather than `innerHTML`.

## The six changes

Each shipped as its own feature branch with its own lease, integrated through CAS
promotion, and merged forward into `dev` before the next one started.

| Change | Tip | What it added |
|---|---|---|
| `control-room-telemetry` | `8dd2630` | Ledger schema v6: `runs` and `lane_progress` tables, nullable lane metadata, declared indexes |
| `control-room-ledger` | `4959d69` | Run, progress, and lane record types over the v6 schema |
| `control-room-capture` | `56327fc` | Executor progress-event contract, per-executor stream decoders, batched lane-progress persistence |
| `control-room-serve` | `f6b3032` | Server model, hub, worktree queries, HTTP handlers |
| `control-room-ui-shell` | `f3d2292` | Console shell, live store, fleet grid, apply-DAG view, SDD flow rails |
| `control-room-ui-views` | `b12531c` | Feature swimlanes, merged timeline, approvals inbox, CLI wiring and the dispatch gate |

Their specs are live under `openspec/specs/`; the capability ids are `approvals-web-ui`,
`batch-wave-view`, `feature-lease-monitor`, `lane-envelope-inspector`, and
`reconciliation-workspace`.

## How it was built

Planning ran five phases per change — explore, propose, design, spec, tasks — each as
three parallel `agy` lens lanes plus one `cursor-agent` synthesis lane. 120 planning
lanes in total across the six changes.

Apply ran as per-change DAGs: `apply-dag.yaml` sidecars declaring each lane's
`allowed_paths` and `depends_on`, split into waves by `lucind-ai split` and dispatched
with `lucind-ai run`. 21 apply lanes plus one verify, one remediation, and one archive
lane on the final change.

A1–A5 dispatched apply to `opencode` (`openai/gpt-5.6-sol`). A6 switched to `agy`
(`gemini-3.7-flash-high`) and got a better first-pass rate — 4 of 6 lanes promoted on
first attempt, versus roughly fifteen distinct dispatch failures across the seventeen
opencode lanes. The original plan reserved `agy` for planning judgment on quota
grounds, not because it could not write production code.

### What went wrong, and what it taught

Three failure modes recurred often enough to be worth naming. All three are documented
in full in `plugin/claude-code/skills/lucind-ai/SKILL.md` under "Multi-wave apply-DAG
orchestration".

**Hand-authored `allowed_paths` are usually wrong.** Planning invents plausible
filenames — `views/features.js`, `features_static_test.go`, `cli_control_room_test.go` —
and the executor correctly writes to the files that actually exist instead, so the lane
lands `deviated`. Eight lanes across A3–A6 failed this way. The fix is to check the
declared paths against the real tree before dispatching, not after.

**A `deviated` or reverted lane has not lost its work.** Its worktree and branch are
preserved deliberately. The right move is to inspect the commit, and hand-integrate it
if it is valid — not to clean up and redispatch, which throws away good work and pays
for it twice.

**`lucind-ai check` runs in the current working directory, not on the feature branch.**
Running it before the merge validates the old tree and produces a mechanical log that
looks authoritative and proves nothing. Merge first, then check.

### The defect that nearly shipped

The final change's verify lane reported `done` with no blocker, despite carrying an
explicit done-criterion that every indirection introduced must be consumed by a
terminal consumer. It was not: `(*serve.Model).ListBatchLanes` and its `BatchLane` DTO
were built by that change and had zero callers anywhere in the repository. The route
that was supposed to expose them was never mounted. No spec named that route, so a
spec-compliance reading alone could not surface it.

It was caught by checking `tasks.md`'s unchecked boxes against the code, one at a time.
Three of seven implementation tasks turned out to be genuinely undelivered rather than
stale. A remediation lane closed them.

The lesson is recorded as a live follow-up: a single qualitative verify lane is not
enough. Either dispatch two lanes with different reference documents — the way
`sdd-fan-out-lens` did, where the two disagreed and only one was right — or point the
lane explicitly at the unchecked checkboxes.

## Raw artifacts

- `openspec/plans/control-room/` — the RUNBOOK, MASTER-PLAN, per-change `apply-dag.yaml`
  sidecars, dispatch packets, and frozen mechanical logs for all six changes.
- `openspec/changes/archive/2026-08-22-control-room-ui-views/` — the final change
  archived in full, including its preserved packets, result envelopes, verification
  record, and archive report.
- `openspec/specs/` — the live capability specs the six changes merged into.

Note that the `apply-dag.yaml` sidecars are preserved exactly as dispatched, wrong
`allowed_paths` included. They are a record of what ran, not a template to re-run:
anything re-dispatched from them would deviate the same way it did the first time.

## Follow-ups

Carried from the final change's archive report.

### Closed

**Lease countdown source.** The question was whether the model should return
`remaining_seconds` or `expires_at` plus a server timestamp. `expires_at` wins: a
precomputed `remaining_seconds` is stale the moment it is serialized, and every
subsequent second of its life is a lie. The console ships `Lease.ExpiresAt` and
differences against it client-side, with a server-supplied UTC anchor so the countdown
does not drift with the viewer's own clock.

**Overlap evidence rendering.** The question was `<pre>` plus `escapeHtml` versus an
inline diff tokenizer for `evidence_json`. Keeping `escapeHtml`. A tokenizer would have
to reconstruct markup around attacker-influenced content — overlap evidence contains
paths and diff fragments from dispatched lanes — and the readability it buys does not
justify reopening an injection surface the console currently has closed by construction.
Revisit only with a tokenizer that emits text nodes rather than markup.

### Open

**Reconciliation mutation surface.** The console shows reconciliation requests and
candidates read-only. Whether it should also POST `approve`/`decline`/`cancel`/`renew`/
`resolve`, or keep pointing at the CLI, was escalated from explore and never answered.
`--enable-dispatch` now provides exactly the gating such a surface would need, so the
blocker is a product decision rather than a missing mechanism.

**Dual verification dispatch.** A single qualitative verify lane passed a change
containing dead code. Either dispatch two lanes with different reference documents, or
point the lane explicitly at `tasks.md`'s unchecked boxes. See "The defect that nearly
shipped" above.
