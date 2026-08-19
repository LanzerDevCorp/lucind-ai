---
id: <id>
executor: agy
routed_by: <the condition that selected this lane and this verification level — never the executor's name; that is the outcome of routing, not its reason>
model: <model, e.g. gemini-3.7-flash-high>
---

# Packet <id>

**Tier:** A (human merge) | B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/<id>  ·  **Branch:** lucind/<id>

## Goal

One paragraph. What must be true when this is finished — not how to get there.

## Why this is safe to dispatch now

Name the open questions in the main conversation and state why none of them can change this
work. If you cannot write this paragraph, the packet is not ready.

## Preconditions

Environment state that must already hold before step one — ports free, stacks down, a
migration applied. Verify them first.

**A precondition satisfied by one of this packet's own later steps is a misordered packet.**
Return `blocked` and say so; do not work around it.

## Done criteria

Each must be checkable by someone who did not do the work. Prefer a command whose output
proves it over a claim that it works.

Two are mandatory in every packet:

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** For
      each variable, flag, or config key added, name the program or file that *reads* it and
      attach the output that proves it. Another mention of the name is not consumption.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and
      `git log --oneline -1`. Conventional commit, no AI attribution.

Then the packet's own:

- [ ] …
- [ ] …

## Allowed paths

Only these may be created or modified. Touching anything else is a **deviation** — finish
nothing further, report it, and stop.

- `path/one`
- `path/two`

## Allowed paths outside the repository

By default this packet may touch **nothing** outside the repository. Anything that must be
touched — configuration files, dotfiles, machine-level setup — must be named explicitly here.

- `~/path/outside`

Every path listed here must be reported in the result envelope's `external_changes`, with a
`revert` — git cannot undo work outside the repository, so the envelope is the only record of
how.

## Out of scope

Name the adjacent work you must NOT do, especially anything the conversation has not decided.

## Hard stops

Stop and return `status: blocked` — do not guess. **Declare every one of these in the
envelope**, whether or not it fired. An undeclared hard stop invalidates the result.

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did
  not anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and the packet does not say which.
- Satisfying one instruction in this packet would require violating another.

## Context

Facts already established, with `file:line` where they came from. Do not make the agent
re-derive what has already been verified. Anything not written here is available through
engram and the worktree — the agent may investigate, but may not widen the scope.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how
well the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
