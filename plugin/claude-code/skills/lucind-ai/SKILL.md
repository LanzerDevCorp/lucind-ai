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
| `executor` | Yes | Execution runtime to dispatch (currently `agy` or `cursor-agent`). |
| `routed_by` | Yes | The explicit condition that triggered this routing decision — never the executor name. |
| `model` | No | Model name passed to executor. Omitted, each executor supplies its own default (`agy`: `gemini-3.7-flash-high`; `cursor-agent`: `cursor-grok-4.6-high`) — do not hardcode `gemini-3.7-flash-high` for a `cursor-agent` packet, it bills against Cursor's separate, more limited "Other Models" quota instead of the included "Cursor Models" quota. |

The document body following the closing `---` is the prompt passed to the executor and must not be empty.

### Where to author packet files

Write every packet file under `.lucind/packets/` (e.g. `.lucind/packets/<id>.md`), never at the
primary repository root or anywhere else inside the tracked tree. `.lucind/` is gitignored
(`.gitignore:2`), so packet files there never show up in `git status --porcelain` on the primary
root.

This is not cosmetic: `lucind-ai run`'s own `Integrate` step refuses to merge completed lanes
back to `main` when the primary root has uncommitted changes at merge time
(`internal/run/integrate.go`), and dispatching a packet requires that file to exist on disk while
`lucind-ai run` is invoked from the primary root. A packet written anywhere inside the tracked
tree — repo root included — makes the primary root dirty for the whole batch and reliably fails
auto-integration with `integrate: primary root has uncommitted changes` on every single batch,
turning a should-be-automatic merge into manual per-lane recovery work every time. Authoring
under `.lucind/packets/` instead avoids this failure mode entirely; no other packet content or
dispatch step changes.

### Executor preference by SDD phase

Prefer this `executor:` value by SDD lifecycle phase when writing a packet. It is a preference the author applies by hand, not a rule enforced by any code — `executor` stays a value a human writes by hand (`docs/prd.md` section 6 step 1), and there is and will remain no code-level routing. It is a second, complementary lens to the aptitude map in `docs/prd.md` section 5 (sweeps-vs-precision); a packet author may weigh both when they point in different directions.

| SDD phase | Preferred executor | Why |
|---|---|---|
| design, proposal, specs, tasks | `cursor-agent` | Editorial/planning judgment on a bounded artifact -- matches its "single-piece precision" strength. |
| apply (implementation) | `agy` | Broad, mechanical, multi-file execution -- matches its "sweeps and volume" strength. |

`validate` deliberately has no entry here. It is not a phase `lucind-ai` dispatches at all. Reviewing/validating a diff is `gentle-ai`'s RDD, run by a human from an `opencode` session with `gpt-5.6-sol` (`docs/prd.md` section 9) — outside this binary's dispatch model entirely, not a third executor choice.

### Dual-executor SDD-phase dispatch (orchestrator pattern)

A Claude Code orchestrator convention layered on top of the preference table above, exercised and
verified twice (session 3, `approvals-web-ui`: propose, design). Not enforced by any code in this
binary — like the preference table itself, a human/orchestrator decision applied packet by packet,
not a default the binary forces.

**Verified pattern (propose, design, specs, tasks):**

1. Write one packet body per phase artifact. Dispatch to `agy` and `cursor-agent` in parallel with
   `--packet` twice, each writing to a distinct draft path
   (`openspec/changes/<change>/<artifact>-agy.md` / `-cursor-agent.md`, or a `<artifact>s-<executor>/`
   subdirectory for multi-file artifacts like specs) so their branches never conflict.
2. The orchestrator reads both drafts and synthesizes one canonical artifact — never picks one
   draft wholesale — then merges both draft branches and the canonical file to `main` by hand
   (`git merge` to `main` is classifier-gated in auto mode; ask the user once per merge round).
3. Update `openspec/changes/<change>/state.yaml`'s phase entry with `status`, `engram_topic`, and a
   short note on what each draft contributed.
4. When the preference table above (or an explicit human instruction in conversation) names a
   single executor for a phase — as happened for `design` in session 3 — skip the dual dispatch
   and run that one executor only. Dual dispatch is the default for propose/design/specs/tasks,
   not a hard rule.

**Whether the double dispatch is worth the extra quota**: judge it per phase, not by default
faith. Session 3's `propose` comparison (engram `sdd/approvals-web-ui/proposal`) found the two
drafts converged almost completely but were still genuinely complementary — the canonical document
pulled specific sentences from both (agy correctly named `Modified Capabilities: lane-execution`
where cursor-agent's draft said "None"; cursor-agent's rollback plan and its explicit rejection of
extending `lane.Status` to a 7th value were sharper). Neither draft alone was the final document.
That is the bar for "worth it" — complementary specificity, not necessarily a contradiction to
arbitrate.

**Target direction, not yet built — do not attempt without addressing the named blocker:**

| Phase | Target | Blocker |
|---|---|---|
| `explore` | Dispatch via `lucind-ai run`, not a local Claude subagent — matches this project's own identity (Claude Code orchestrates, `agy`/`cursor-agent` execute). | Unblocked: frontmatter supports `read_only: true`; criterion 2 is replaced by `git status --porcelain` empty and `HEAD` equals `git merge-base HEAD <primary HEAD>`. |
| `apply` | Split `tasks.md` into independent packets and dispatch as a DAG (parallel where tasks share no file scope) via `lucind-ai run`, not `sdd-apply`'s own Read/Edit/Write. Not new engineering — bisection + `internal/resolve` (`docs/prd.md` §6 steps 6-8) already exist for exactly this. | Needs an orchestrator step that turns `tasks.md` into a DAG of packets with non-overlapping `allowed_paths`, dispatched in dependency order. Not built. |
| `verify` | Dual-dispatch `agy` + `cursor-agent` for the *qualitative* half of verification (does the implementation satisfy the spec's intent, are there coverage gaps) — not the mechanical half. | Mechanical checks (`go test`, `go vet`, `lucind-checks.sh`) are deterministic; running them twice through two LLMs adds no information, only cost. Dispatch only the judgment portion twice, once tooling exists to run mechanical checks a single time and hand both executors the same result to judge independently. |

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
  - *Mandatory criterion 2*: The work is committed with a conventional commit and no AI attribution (`git status --porcelain` empty and `git log --oneline -1`). For `read_only: true` packets, replaced by: `git status --porcelain` empty and `HEAD` equals `git merge-base HEAD <primary HEAD>`.
- **Hard stops**: Explicit failure/boundary conditions that require stopping immediately with `status: blocked` rather than guessing. Every declared hard stop must be explicitly evaluated and reported in the result envelope whether or not it fired.

### Judging Returned Evidence

Reviewing returned evidence is a human/orchestrator judgment task:
- Green criteria are not proof of complete work; verify evidence independently against the codebase.
- On `blocked`: inspect the returned question and recommendation, answer the decision point, and resume the context.

## 2. Driving the Binary

The `lucind-ai` CLI orchestrates worktrees, dispatches runners, records state, and evaluates batch barriers.

`lucind-ai -v` (or `--version`) prints the exact build (`git describe`) baked in at compile time.

### Invocation

Run from the primary repository root (the binary refuses to run from inside a linked worktree):

```bash
lucind-ai run --packet <path> [--packet <path> ...] [--timeout <duration>]
lucind-ai --version
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

