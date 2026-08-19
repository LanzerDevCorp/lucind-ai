---
name: lucind-ai
description: "Author dispatch packets and drive the lucind-ai delegated execution binary."
license: Apache-2.0
metadata:
  author: "LanzerDevCorp"
  version: "2.0"
---

# lucind-ai

Authoring dispatch packets and driving the `lucind-ai` execution binary.

## 1. Writing a Packet

A packet defines a bounded, surgical unit of work executed in an isolated git worktree.

### Frontmatter

Every packet must open with a YAML frontmatter block enclosed by `---`:

| Key | Required | Description |
|---|---|---|
| `id` | Yes | Unique identifier for the lane. Names the branch (`lucind/<id>`) and worktree directory. |
| `executor` | Yes | Execution runtime to dispatch (currently `agy`). |
| `routed_by` | Yes | The explicit condition that triggered this routing decision — never the executor name. |
| `model` | No | Model name passed to executor. Defaults to `gemini-3.7-flash-high` when omitted. |

The document body following the closing `---` is the prompt passed to the executor and must not be empty.

### Packet Structure

1. **Goal**: One concise statement of what must be true upon completion (not how to do it).
2. **Why this is safe to dispatch now**: Why unresolved conversation questions cannot alter this work.
3. **Preconditions**: Verified environment state before step one. If a precondition depends on a later step in the same packet, the packet is misordered and must return `blocked`.
4. **Allowed paths**: Explicit list of files/directories permitted to change in the repository.
5. **Allowed paths outside the repository**: Paths outside the repo (e.g. `~/.config/...`) with exact revert commands.
6. **Out of scope**: Adjacent work explicitly forbidden.
7. **Context**: Ground-truth facts with `file:line` references; avoid forcing agents to re-derive context.

### Done Criteria & Hard Stops

- **Done criteria**: Verifiable, objective assertions checkable by someone who did not do the work. Each criterion requires concrete evidence (command output or `file:line`), not assertions of success.
  - *Mandatory criterion 1*: Every indirection introduced is demonstrably consumed by a terminal consumer (name the consumer and provide proof).
  - *Mandatory criterion 2*: The work is committed with a conventional commit and no AI attribution (`git status --porcelain` empty and `git log --oneline -1`).
- **Hard stops**: Explicit failure/boundary conditions that require stopping immediately with `status: blocked` rather than guessing. Every declared hard stop must be explicitly evaluated and reported in the result envelope whether or not it fired.

### Judging Returned Evidence

Reviewing returned evidence is a human/orchestrator judgment task:
- Green criteria are not proof of complete work; verify evidence independently against the codebase.
- On `blocked`: inspect the returned question and recommendation, answer the decision point, and resume the context.

## 2. Driving the Binary

The `lucind-ai` CLI orchestrates worktrees, dispatches runners, records state, and evaluates batch barriers.

### Invocation

Run from the primary repository root (the binary refuses to run from inside a linked worktree):

```bash
lucind-ai run --packet <path> [--packet <path> ...] [--timeout <duration>]
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--packet <path>` | String (repeatable) | *(required)* | Path to a packet file. Each instance adds one concurrent lane. |
| `--timeout <duration>` | Duration | `20m` | Wall-clock budget granted to each lane independently. |

### Concurrency & Barrier

- **Parallel lanes**: Passing multiple `--packet` flags executes lanes concurrently in isolated worktrees (`../<repo>-worktrees/<id>`).
- **Independent clocks**: Each lane gets an independent deadline derived from `--timeout`; slow lanes never consume a sibling lane's budget.
- **Failure isolation**: Lanes never cancel sibling lanes. If one lane blocks, fails, or times out, all other lanes run to completion.

### Reports & Preserved Worktrees

- **Ledger**: SQLite database at `.lucind/lucind.db` records lane registrations, status transitions, and barrier events.
- **Envelope**: Dispatched runners write structured envelopes to `.lucind/result.json`, validated against `.lucind/result.schema.json`.
- **Preservation**: All lane worktrees are preserved on completion or failure.
- **Exit code**: Returns `0` only when every lane in the batch achieves `done`. Returns `1` if any lane blocked, deviated, or failed.

