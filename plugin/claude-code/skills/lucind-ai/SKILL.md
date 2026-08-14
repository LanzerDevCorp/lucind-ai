---
name: lucind-ai
description: "Trigger: parallel work, multi-agent, delegate to agy, antigravity, opencode, work packet, worktree, task ledger, approval queue, human packet. Route already-decided work across execution, audit and human lanes without stalling the conversation."
license: Apache-2.0
metadata:
  author: "LanzerDevCorp"
  version: "2.0"
---

# lucind-ai

Supervisor pattern, two levels, never three. Opus holds the ledger and dispatches surgical
tasks; `agy` executes in parallel, `opencode` audits, the human runs whatever touches a
secret. Context lives in engram and in the worktree — the prompt says where to start.

## Activation Contract

Load when the conversation is still deciding and some work has become required regardless of
what gets decided next. Also load when the user asks to delegate, parallelize, or drive
`agy` / `opencode`.

Do NOT load to run work whose decision is still open, or for a single edit the main thread
can make faster inline.

## Hard Rules

1. Dispatch only work that stays correct no matter how the open questions resolve. If a
   pending answer could change it, do not dispatch.
2. Every packet runs in its own git worktree. Never dispatch an agent into the main tree.
3. **No agent ever generates, enters, or writes a credential value.** A packet may prepare a
   rotation — env indirection, `.env.example`, runbook — and must stop before the secret.
   That work is the human's lane.
4. `blocked` is a terminal result, not a prompt. A one-shot CLI cannot ask mid-run, so the
   agent stops and returns the question. Never let it guess.
5. **Every hard stop must be declared in the result envelope** — for each one, either it did
   not fire, or it fired and the agent stopped. An undeclared hard stop makes the result
   invalid regardless of the criteria.
6. Resume blocked packets with `agy --conversation <id>` or `opencode run -s <id>`. Never
   restart from scratch — the context is the asset.
7. A packet whose precondition is satisfied by one of its own later steps is misordered.
   Return `blocked`; do not work around it.
8. Route packet execution to a non-Claude model. A same-family model echoes the caller's
   reasoning instead of checking it. What buys independence is ignorance of the caller's
   context, not the vendor — see **Fallback** for what that permits.
9. The routing decision goes into the ledger together with the condition that triggered it.
   There is no implicit routing.

## Routing

Two independent axes. Decide them separately — mixing them is what makes a trivial task drag
the whole machine behind it. A project may extend this table in its own `CLAUDE.md`; the
project table wins.

### Axis 1 — who executes

By the nature of the work, never by its size.

| Situation | Executor |
|---|---|
| Verify or decide by reading 1–3 files | Opus, inline |
| Map or explore 4+ files | `agy`, sweep, no writes |
| Bounded execution with checkable done-criteria | `agy`, own worktree |
| Write 2+ non-trivial files | `agy`, one writer per worktree |
| Adversarial judgment on a diff or a decision | `opencode -m openai/gpt-5.6-sol` |
| Needs a credential value, or critical supervision | **the human** |

Parallelise only when the tasks share no files. One worktree per packet.

### Axis 2 — what verification it warrants

Default is zero. A control fires only through the condition that names it. If the condition
cannot be named, the control does not fire.

| Named condition | Control |
|---|---|
| Always | Every hard stop declared in the envelope |
| Touches config, secrets, security posture, schema, or CI | `gpt-5.6-sol` audit, mandatory before merge |
| Two hard-stop violations by the same runtime on one packet | Circuit breaker — the lane closes, escalate to the human, no retry |
| The audit ran on a same-family fallback | Tier B degrades to human merge |
| Substantial ambiguity a durable contract would reduce | SDD, and only on explicit request or accepted proposal |

Size, file count, and perceived risk never select SDD on their own.

## Fallback

Quotas are finite and the audit is mandatory for a whole class of change. Without a written
alternative, an exhausted quota blocks that class entirely.

**Only a quota or rate-limit error triggers a fallback.** Any other failure is an execution
error: retry with backoff, three attempts, then the circuit breaker. Treating a network
timeout as an exhausted quota makes the system hop providers for no reason.

**Execution lane** — stay inside the provider before leaving it:
`gemini-3.7-flash-high` → `3.6` → `3.5`. Changing family to *execute* buys nothing.

**Audit lane** — `openai/gpt-5.6-sol`, then a blind Claude panel:

Independence comes from ignorance of the caller's context, not from the vendor. A frontier
Claude model that never saw the conversation is not echoing anything — it does not know what
to echo. So the fallback auditor is **two blind judges** (`jd-judge-a`, `jd-judge-b`,
`model: opus`), given the frozen diff and the packet and nothing else. Never the orchestrator
itself, and never an agent carrying conversation context.

**A same-family verdict is recorded as such** — `audit.family: "same"` in the ledger — and it
**degrades Tier B to human merge**. The whole justification for auto-merge is that an
independent family checked the work; when independence is what is missing, the human is what
compensates. A same-family verdict still blocks, still raises objections, still counts as an
audit having run. It just cannot approve a merge on its own.

## The Task Ledger

The orchestrator holds no context — it holds a ledger. One entry per task: id, state,
dependency, executor, the condition that routed it, and where the result landed. Everything
else is read from engram when needed.

The ledger is the answer to "what is happening". If a question cannot be answered from the
ledger plus one engram lookup, the thread is carrying context it should not.

## Prompt Contract

Every dispatch, to any lane, carries the same four parts:

1. **A surgical task** — one outcome, not a project.
2. **Scope and starting files** — where to begin. The agent may investigate further; it may
   not widen the scope.
3. **A response contract** — the envelope, enforced by schema where the runtime supports it.
4. **Die** — the run ends when the task ends. No open-ended sessions.

Context is not pasted into the prompt. It lives in engram and in the worktree.

## Execution Steps

1. Read the ledger and `.lucind/approvals.md`. Surface pending approvals before anything else.
2. Write the packet from `assets/packet-template.md`, or `assets/human-packet-template.md`
   when the human is the executor.
3. Create the worktree per `references/runtime.md` — a sibling of the repo, never a temp dir.
4. Dispatch with the envelope enforced (`assets/result.schema.json`).
5. On `done`: verify the evidence independently. Green criteria are not proof of correct work
   — the defect usually lives just outside what the criteria asked.
6. Run the audit lane when Axis 2 names it. The verdict is advisory; Opus arbitrates.
7. Tier A → human merge. Tier B → merge, then report.
8. On `blocked` or `deviated`: append to `.lucind/approvals.md`, surface it verbatim, resume
   with the session id once answered.
9. Update the ledger after every transition.

## Output Contract

Per packet: id, tier, status, worktree, files changed, hard-stop declarations, audit verdict,
and either the merge result or the exact question awaiting an answer. Never report a packet
done before its evidence was checked independently. Never summarize away a `blocked` question
— relay it verbatim.

## References

- `references/runtime.md` — verified CLI surface, models, MCP wiring, worktree mechanics.
- `references/state-files.md` — ledger, approval queue, and what stays on disk.
- `assets/packet-template.md` — agent packet.
- `assets/human-packet-template.md` — human packet.
- `assets/result.schema.json` — result envelope schema.
