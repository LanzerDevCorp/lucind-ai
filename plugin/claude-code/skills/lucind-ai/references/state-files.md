# State: engram first, disk for what cannot live there

Most of the shared truth belongs in **engram** — decisions, verdicts, findings, packet
results. It is the memory every lane can read, and it survives compaction of the main thread.

Disk holds only what engram cannot: the working tree itself, and a pointer small enough to
read at the start of a turn without thinking.

Everything under `.lucind/` is gitignored.

## `.lucind/state.json` — the ledger

One entry per packet, newest last. This is the only place a packet's status is authoritative.
Keep it small: it is a ledger, not a record of what happened. The narrative goes to engram.

```json
{
  "packets": [
    {
      "id": "harden-01-config-env",
      "tier": "A",
      "goal": "One line. What must be true when this is done.",
      "status": "awaiting_approval",
      "routed_by": "touches config and secrets -> Tier A, audit mandatory",
      "worktree": "../corp-marketplace-worktrees/harden-01-config-env",
      "branch": "lucind/harden-01-config-env",
      "runtime": "agy",
      "model": "gemini-3.7-flash-high",
      "session_id": "52b1a928-...",
      "depends_on": null,
      "blocks_on": "approval-0002",
      "containers_left_running": [],
      "audit": {
        "verdict": "objections",
        "by": "openai/gpt-5.6-sol",
        "family": "independent",
        "fallback_reason": null
      }
    }
  ]
}
```

`status`: `dispatched` → `done` | `blocked` | `deviated` | `failed`, then `in_review` →
`awaiting_approval` → `merged` | `abandoned`.

`routed_by` is not optional. A routing decision without its condition is implicit routing,
which the skill forbids.

`containers_left_running` exists because a worktree stack holds the same host ports as the
main one. An empty array is a claim that nothing is holding a port.

`audit.family` is `"independent"` or `"same"`. A `"same"` verdict came from the blind Claude
panel after a quota fallback: it still blocks and still raises objections, but it cannot
approve a Tier B auto-merge. `fallback_reason` carries the quota or rate-limit error that
caused the switch — it must be an actual quota error, never a generic failure.

## `.lucind/approvals.md` — the queue

Human-readable, append-only. Read it at the start of every turn. Nothing is ever deleted —
resolved items get their outcome appended, so the record of what was approved survives.

```markdown
## approval-0003 · <packet-id> · PENDING

**Type:** blocked | post-hoc objection
**Raised:** <date>, by <who>
**Session to resume:** `<id>`

### The objection

What is wrong, with the evidence that proves it — command output or file:line, not a claim.

### Options

1. …
2. …

**Recommendation:** … and why the alternative is worse.

**Resolution:** _(pending)_
```

On resolution, append the answer, the date, and the exact command that was run. Then update
the ledger.

## `.lucind/packets/` and `.lucind/results/`

The packet as dispatched and the envelope as returned. They exist so a defect can be traced
back to the exact instruction that produced it — which is how every defect so far was
understood. Do not clean them up on merge.

## Turn protocol

At the start of every turn: read `approvals.md`, surface every `PENDING` item verbatim, and
stop. Do not answer on the user's behalf, and do not continue the conversation with a pending
approval unmentioned — a blocked packet is holding a worktree, a live session, and possibly a
host port.

This belongs in a `UserPromptSubmit` hook rather than in an agent's discipline. An orchestrator
that has to *remember* to read the queue is the most fragile part of the design.
