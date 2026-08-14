# Project routing table

Paste this into the consuming project's `CLAUDE.md`. It belongs at **project level**, not in
the global one — a global `CLAUDE.md` managed by another tool gets overwritten on update, and
this table is the part you cannot afford to lose.

Adjust the executors to whatever CLIs that project actually has. The two axes and the hard
rules stay.

---

## lucind-ai routing

Multi-agent routing for this project. Extends the global implementation-routing rules; it does
not replace them.

Two independent axes. Decide them separately — mixing them is what makes a trivial task drag
the whole machine behind it.

### Axis 1 — who executes

Chosen by the nature of the work, never by its size.

| Situation | Executor |
|---|---|
| Verify or decide by reading 1–3 files | Opus, inline |
| Map or explore 4+ files | `agy`, sweep, no writes |
| Bounded execution with checkable done-criteria | `agy`, own worktree |
| Write 2+ non-trivial files | `agy`, one writer per worktree |
| Adversarial judgment on a diff or a decision | `opencode -m openai/gpt-5.6-sol` |
| Touches credential values, or needs critical supervision | **the human** |

Parallelise only when the tasks share no files. One worktree per packet. Never more than two
levels of supervision.

### Axis 2 — what verification it warrants

Default is zero. Each control switches on only through the condition that names it.

| Named condition | Control |
|---|---|
| Always | Every hard stop declared in the result envelope: either it did not fire, or it fired and the agent stopped |
| Touches config, secrets, security posture, schema, or CI | `gpt-5.6-sol` audit, mandatory before merge |
| Two hard-stop violations by the same runtime on the same packet | Circuit breaker — that lane closes, escalate to the human, no retry |
| The audit ran on a same-family fallback | Tier B degrades to human merge |
| Substantial ambiguity a durable contract would reduce | SDD, and only on an explicit request or an accepted proposal |

### Hard rules

1. The default is the minimum. A control fires only through a named condition. If the
   condition cannot be named, the control does not fire.
2. Size, file count, and perceived risk never select SDD on their own.
3. The routing decision is written to the ledger together with its condition. There is no
   implicit routing.
4. No agent ever writes a credential value. That work is the human's lane.
5. A hard stop reached is a terminal `blocked` result carrying the question — never a silent
   choice.

If this two-axis table proves hard to apply consistently, fall back to a single ladder of
tiers where each tier already carries its executor and its verification.
