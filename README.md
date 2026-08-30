# lucind-ai

The delegated-execution layer for work paid by subscription. A Go binary routes *execution* work to
CLI agents already covered by a subscription, isolates each in its own worktree, runs them in
parallel, and refuses to believe what comes back until it satisfies a schema.

It owns parallel execution and the integrity of what returns. Review, delivery and lifecycle belong
to [`gentle-ai`](https://github.com/Gentleman-Programming/gentle-ai).

**[`docs/prd.md`](docs/prd.md) is the source of truth.** This file is the short version.

## Why it exists

Delegating to a CLI agent is easy. Trusting what comes back is not.

The first real packet returned `done` with every criterion green and real command output attached —
and still had a defect, because the defect lived just outside what the criteria asked. The next round
did it again. Both times the packet had an explicit hard stop covering exactly that case, and the
agent walked past it and reported success without contradicting itself.

Both runs happened without review, and the failure was in *execution*. That is the seam this project
lives in: `gentle-ai` asks whether a diff is any good, after the fact. This asks whether the executor
did what it was asked and declared where it stopped, at the moment it returns.

The second reason is arithmetic. Several subscriptions are already paid for and only one of them ever
works at a time. **The unit of parallelism is the subscription, not the CLI** — two CLIs drawing on
one subscription add no capacity.

## The roster

| Subscription | CLI | Role |
|---|---|---|
| Anthropic | Claude Code | orchestrator; bounded conflict resolution |
| ChatGPT | `opencode` · `openai/gpt-5.6-sol` | reviewer — drives RDD |
| Google Antigravity | `agy` | executor — sweeps and volume |
| Cursor | `cursor-agent` | executor — single-piece precision |

Excluded on purpose: `codex` (would draw on the same ChatGPT subscription `opencode` already uses —
a second door to the same room), `opencode-go` (metered per token), the `gemini` CLI (dead; `agy`
succeeded it).

Never a metered API key. That constraint is the whole reason this exists.

## How it works

The orchestrator writes the packets and decides the lane split — that part is judgment, so it stays
prose. Everything after it is the binary:

admission → one worktree per lane → parallel headless dispatch → each executor writes its envelope
to `.lucind/result.json` → schema validation → **barrier** → integrate → cleanup → stop.

Admission runs first and has zero side effects: unknown executor, a model outside that executor's
closed list, overlapping `allowed_paths`, an incomplete or mixed feature target, exhausted `agy`
quota, a dispatch launched from inside a linked worktree, or a skill tree that has drifted between
the Claude and OpenCode distributions — any of these exits non-zero before a single worktree or
ledger row exists.

The barrier is the load-bearing piece. It releases only when **every** lane reaches a terminal state,
not just `done` — otherwise one blocked lane hangs the run forever. And if any lane is not `done`,
**nothing merges**: worktrees are preserved and the blocking question is relayed verbatim. Merging
half of a parallel batch produces a repository state nobody designed.

`done` is not something a lane can claim. The envelope's own `status` is only the entry to a ladder
of runtime checks: exit code and wall clock, schema validity, **any declared hard stop with
`fired: true`**, the real four-way git diff against the recorded `base_sha` versus `allowed_paths`,
`skills_loaded` versus the lane's required skills, and commits-and-porcelain versus the packet's
`read_only` mode. Each rung has its own demotion target — `blocked`, `deviated`, or `failed`.

Integration takes one of two shapes. A **feature-targeted** batch runs a durable, recoverable
attempt (`recorded → leased → combining → checking → cas_pending → promoted`) holding a fenced
exclusive lease, evaluates cross-feature overlap, and promotes by compare-and-swap on the named
parent ref — the primary checkout is never touched. A **legacy** batch merges into a temporary
combined tree, runs the project's checks, bisects to the clean subset if red, and fast-forwards the
currently checked-out branch.

The binary stops at cleanup. It does not trigger review; RDD is driven separately from an `opencode`
session, so the reviewer is not the same model family that orchestrated the work.

There is no approval pause inside a dispatch and no approvals UI — both were removed with the
control room. Human judgment enters after the barrier: `lucind-ai accept` re-verifies a frozen
candidate out of the ledger and issues a mechanical receipt, and Promotion stays a human decision.

## The command surface

| Command | What it does |
|---|---|
| `run` | Dispatches N packets as N concurrent lanes, joins at the barrier, integrates, promotes. |
| `split` | Validates an `apply-dag.yaml`, emits one packet per node, prints one `run` command per wave. It does not schedule them. |
| `check` | Runs `lucind-checks.sh` where you stand; `--out` freezes the transcript as evidence. |
| `accept` | Re-verifies a frozen candidate from the ledger in a detached worktree and issues an immutable receipt. Never moves a ref. |
| `feature` | `create · status · recover · renew · lease release · lease status · disable` — feature anchors, fenced leases, attempt recovery. |
| `reconcile` | `approve · decline · cancel · renew · resolve` — the cross-feature overlap resolution cycle. |
| `defect` | `record · list · resolve · decline · defer` — durable defect records, written by the ultrafixer protocol. |
| `worktree` | `cleanup --lane <id> [--force]`. Removes the worktree; the `lucind/<id>` branch is a separate manual delete. |
| `integrate` | `retry --run <run-id>` — rebuilds a reverted batch from the ledger and preserved worktrees, with no AI dispatch. |
| `phase` | The SDD synthesis gate against `gentle-ai sdd-status`; generates the synthesis packet when none exists. |

Run `lucind-ai` with no arguments for the live flag syntax rather than trusting this table.

## Status

Honest, because a plan that claims to be finished is the same failure this project exists to catch.

| Piece | State |
|---|---|
| Requirements | see `docs/prd.md` |
| The `lucind-ai` binary | written — ten subcommands; see the table above |
| SQLite ledger | written (schema v10) — runs, lanes, events, progress, candidates, receipts, features, leases, attempts, overlap, reconciliation, defects |
| Barrier / parallel dispatch | written — joins N lanes concurrently under one barrier |
| Envelope schema + runtime enforcement ladder | written — embedded and enforced on every dispatch (`internal/result/`, `internal/run`) |
| Execution lane (`agy`) | written — dispatched headlessly; many end-to-end runs completed |
| Execution lane (`claude`, `opencode`) | written — admitted routes with closed model lists |
| Execution lane (`cursor-agent`) | written and admitted; has never executed end to end |
| Feature targets, fenced leases, CAS promotion | written — durable recoverable attempts (`internal/run/attempt.go`) |
| Overlap classification and reconciliation | written — `internal/overlap`, `internal/reconcile` |
| Mechanical acceptance receipts | written — `lucind-ai accept` (`internal/accept`) |
| Apply-DAG split into waves | written — `lucind-ai split`; emits packets and prints wave commands, schedules nothing |
| Defect records / ultrafixer triage | written — `lucind-ai defect`, dispatched via the ultrafixer packet template |
| Approvals web UI / control room | **removed** — decommissioned in `751c6b1`; the per-lane approval gate went with it |
| Review via RDD | never run |
| Human lane | one packet, closed — it found two defects in its own instructions |
| Claude Code plugin / OpenCode integration | written — two distributions, byte-identical skill trees enforced at dispatch |
| Automated conflict resolution past 400 lines | not written — `internal/conflicttriage`'s production invoker is deliberately unwired |

Install with `make install` — never `go build` to an ad-hoc path. `make install` bakes a real
`git describe` into the binary, so `lucind-ai -v` always reflects what was actually built. A stale
binary silently lacks recent executors and flags; that cost one whole session once.

[`docs/estado-real.html`](docs/estado-real.html) is the same picture, drawn, with the same
distinction between what exists and what does not.

## What is in here

```
docs/prd.md                            the source of truth
docs/estado-real.html                  the design, drawn, with an honesty legend
docs/research/meta-harness-landscape.md  what already exists in the field

cmd/lucind-ai/                         the binary CLI entry point
internal/run/                          composition root: dispatch, enforcement ladder, barrier wiring
internal/run/attempt.go                the durable feature-attempt state machine
internal/barrier/                      pure in-memory join over lane states — no clock, no I/O
internal/executor/                     one adapter + stream decoder per CLI, plus the agy quota gate
internal/ledger/                       SQLite schema v10 and every durable table
internal/{feature,reconcile,overlap}/  leases, cross-feature resolution, diff classification
internal/{accept,integrate,dag}/       receipts, combined trees and CAS, apply-DAG waves
internal/result/result.schema.json     result envelope schema, embedded into the binary

lucind-checks.sh                       the full-tree gate: CGO_ENABLED=0 build + go test -race
scripts/agy-pool                       Antigravity account pool behind --min-quota

plugin/claude-code/skills/lucind-ai/
├── SKILL.md                           canonical Claude Code orchestrator skill
├── references/ and assets/             complete skill support tree

plugin/opencode/
├── lucind-ai.ts                        native OpenCode custom tool
├── process.mjs                         shell-free argv subprocess runner
├── skills/lucind-ai/                   byte-for-byte copy of the Claude tree
└── install.sh                           idempotent global installer

templates/project-routing.md           superseded by the binary's routing
```

## OpenCode integration

Claude Code and OpenCode are separate runtimes: `/lucind-ai` in Claude Code is
provided by the Claude plugin, while OpenCode loads the native plugin and skill
globally. Install and verify with `make install-opencode-plugin` and `make
verify-opencode-plugin`; the installer honors `XDG_CONFIG_HOME` and falls back
to `$HOME/.config`. Restart OpenCode after installing or changing config,
plugin, or skill files. See [`plugin/opencode/README.md`](plugin/opencode/README.md).

## Prior art

The supervisor and Magentic task-ledger patterns, the three classes of agent error, and the circuit
breaker come from Chapter 19 of the Gentleman Programming book on AI orchestration patterns. The
second error class it describes — technically valid parameters, syntax fine, semantics wrong,
*treacherous* — is exactly what the early packets produced.
