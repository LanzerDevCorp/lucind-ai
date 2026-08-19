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

one worktree per lane → parallel headless dispatch → each executor writes its envelope to
`.lucind/result.json` → schema validation → **barrier** → merge → cleanup → stop.

The barrier is the load-bearing piece. It releases only when **every** lane reaches a terminal state,
not just `done` — otherwise one blocked lane hangs the run forever. And if any lane is not `done`,
**nothing merges**: worktrees are preserved and the blocking question is relayed verbatim. Merging
half of a parallel batch produces a repository state nobody designed.

The binary stops at cleanup. It does not trigger review; RDD is driven separately from an `opencode`
session, so the reviewer is not the same model family that orchestrated the work.

Approvals live in a web UI the binary serves itself on localhost — no npm, no build step. It has no
"approve all" button, it starts with nothing selected, it shows evidence inline, and it tracks your
own rate of approvals that later went wrong.

## Status

Honest, because a plan that claims to be finished is the same failure this project exists to catch.

| Piece | State |
|---|---|
| Requirements | fixed — see `docs/prd.md` |
| The `lucind-ai` binary | written — dispatches via `run --packet <path>` (repeatable) |
| SQLite ledger | written (schema v2) — used by `internal/run` |
| Approvals web UI | not written |
| Barrier / parallel dispatch | written — joins N lanes concurrently under one barrier |
| Execution lane (`agy`) | written — dispatched headlessly by the binary; multiple end-to-end runs completed |
| Execution lane (`cursor-agent`) | logged in 2026-08-17, has never executed |
| Review via RDD | never run |
| Human lane | one packet, closed — it found two defects in its own instructions |
| Packet templates, envelope schema | written — embedded and validated on every dispatch (`internal/result/`) |
| Claude Code plugin / marketplace | to be removed — distribution machinery for an audience of one |

Installable via `go install ./cmd/lucind-ai`. [`docs/estado-real.html`](docs/estado-real.html) is the same picture,
drawn, with the same distinction between what exists and what does not.

## What is in here

```
docs/prd.md                            the source of truth
docs/estado-real.html                  the design, drawn, with an honesty legend
docs/research/meta-harness-landscape.md  what already exists in the field

cmd/lucind-ai/                         the binary CLI entry point
internal/                              barrier, ledger, executor, and run packages
internal/result/result.schema.json     result envelope schema, embedded into the binary

plugin/claude-code/skills/lucind-ai/
├── SKILL.md                           to shrink to: how to write a packet, how to drive the binary
├── references/runtime.md              verified CLI surface and verification traps
├── references/state-files.md          superseded by the SQLite ledger
├── assets/packet-template.md          agent packet
└── assets/human-packet-template.md    human packet

templates/project-routing.md           superseded by the binary's routing
```

## Prior art

The supervisor and Magentic task-ledger patterns, the three classes of agent error, and the circuit
breaker come from Chapter 19 of the Gentleman Programming book on AI orchestration patterns. The
second error class it describes — technically valid parameters, syntax fine, semantics wrong,
*treacherous* — is exactly what the early packets produced.
