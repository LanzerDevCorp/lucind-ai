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
| `read_only` | No | `true` or `false`. Omitted defaults to write. A `true` packet must produce no unique commits and leave a clean worktree. |
| `allowed_paths` | No | Single-line JSON array of repository-relative paths this packet may touch, e.g. `allowed_paths: ["internal/dag/", "cmd/lucind-ai/cli.go"]`. Omitted (or empty) is today's exact path: no overlap check across the batch, no post-run diff check. A YAML list under the key does not parse — the value after `:` must be one JSON array. |

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
| apply (implementation) | `agy` by default; `cursor-agent` per task when the task itself is precision/judgment work | `agy` for broad, mechanical, multi-file execution -- matches its "sweeps and volume" strength. But `apply` is not a single monolithic phase for executor-choice purposes: a DAG-wave apply dispatch names one `executor:` per node (`internal/dag`'s `Node.Executor`), so reassign individual apply tasks to `cursor-agent` when they read as one bounded, judgment-heavy artifact rather than a broad sweep -- e.g. a single new small file with careful edge-case DTOs, or a pure docs/README task -- the same "sweeps-vs-precision" aptitude map (`docs/prd.md` section 5) that drives the planning-phase preference, just applied per-task instead of per-phase. Not a hard split: most `apply` tasks (multi-package wiring, state machines, broad plumbing) still default to `agy`.

`validate` deliberately has no entry here. It is not a phase `lucind-ai` dispatches at all. Reviewing/validating a diff is `gentle-ai`'s RDD, run by a human from an `opencode` session with `gpt-5.6-sol` (`docs/prd.md` section 9) — outside this binary's dispatch model entirely, not a third executor choice.

**Verified precedent (`feature-parent-integration`, DAG-wave apply):** of 10 remaining apply tasks split across 7 waves, 2 were reassigned from the `agy` default to `cursor-agent` on user instruction: `internal/serve/model.go` (one new bounded file, shell-free DTOs) and the docs/README task (pure editorial). The other 8 (multi-package wiring, state machines, git plumbing) stayed `agy`. Reassigning meant editing the `executor:` field per node in the `apply-dag.yaml` sidecar and re-running `lucind-ai split` to regenerate consistent packet frontmatter -- nothing in `cmd/lucind-ai/cli.go`'s `supportedExecutors` map treats the two differently, so this is purely an authoring-time choice, exactly like the phase-level preference above.

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

**Target direction — do not attempt an unbuilt phase without addressing its named blocker:**

| Phase | Target | Blocker |
|---|---|---|
| `explore` | Dispatch via `lucind-ai run`, not a local Claude subagent — matches this project's own identity (Claude Code orchestrates, `agy`/`cursor-agent` execute). | Unblocked: frontmatter supports `read_only: true`; criterion 2 is replaced by `git status --porcelain` empty and `HEAD` equals `git merge-base HEAD <primary HEAD>`. |
| `apply` | Author `openspec/changes/<id>/apply-dag.yaml` (sidecar; `tasks.md` stays the human checklist) → `lucind-ai split --dag … --out …` → run each printed `lucind-ai run` line **sequentially**, stop on exit 1. | Built. See **Apply dispatch** below. |
| `verify` | Stage 1: mechanical check once via `lucind-ai check`. Stage 2: Dual-dispatch `agy` + `cursor-agent` for the *qualitative* half of verification. | Built. See **Verify dispatch** below. |

**Apply dispatch (built).** Apply authors packet files (and the sidecar when a DAG is wanted) and dispatches via `lucind-ai run`. It does **not** write the apply diff in the orchestrator's primary checkout.

An **absent** sidecar is still valid — one packet or a hand-split set, no `split` required (the pattern used for `read-only-packet-dispatch`'s own apply).

When a DAG is wanted:

1. Author `openspec/changes/<id>/apply-dag.yaml`. `tasks.md` stays the human checklist; it is not the parse source.
2. Run `lucind-ai split --dag openspec/changes/<id>/apply-dag.yaml --out .lucind/packets`. `split` writes one packet file per node under `--out` and prints one copy-pasteable `lucind-ai run --packet …` line per wave to stdout. That stdout *is* the wave plan; `split` does not write a `waves.json`. Point `--out` at `.lucind/packets/` (or a subdirectory of it) so the primary root stays clean.
3. Run each printed line **sequentially**. The orchestrator (this session, or a human) is the sequencer — the binary has no in-process `--dag` wave loop and no `--json` channel.

Wave N+1 is dispatched only when wave N exits 0: every lane `done`, and none listed in `reverted_ids`. On a non-zero exit, halt. Read `integrated_ids` and `reverted_ids` from that wave's stdout (not a new report format). Confirm every wave-N id is listed under `integrated_ids` before running the next printed line.

**Verify dispatch (built).** Verify is two-stage: mechanical checks (`lucind-checks.sh` via `lucind-ai check`) run once; Dual-dispatch `agy` + `cursor-agent` for the *qualitative* half of verification (spec intent, coverage gaps) — not the mechanical half. The orchestrator synthesizes one canonical `openspec/changes/<id>/verify.md`. Judgment lanes do **not** re-run the suite.

1. **Stage 1: Mechanical Check.** Run `lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log` on the candidate branch. `check` wraps `lucind-checks.sh` through `internal/integrate.Check` and, when `--out` is set, writes a structured log (git SHA, command, duration, exit code, transcript). `--out` is optional on the CLI; this protocol always supplies it. Halts immediately if checks fail — remediate mechanical failures before any judgment dispatch. On pass, commit the log to the candidate branch `HEAD` so linked judgment worktrees inherit it (`.lucind/` is gitignored and is not shared across worktrees).
2. **Stage 2: Dual Qualitative Judgment Dispatch.** Author `.lucind/packets/verify-<id>-agy.md` and `.lucind/packets/verify-<id>-cursor-agent.md` from `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` (`read_only: true`, frozen mechanical summary in `## Context`). Dispatch in parallel with `lucind-ai run --packet .lucind/packets/verify-<id>-agy.md --packet .lucind/packets/verify-<id>-cursor-agent.md`. The `run` barrier joins when both lanes reach terminal status. Do not execute `go test`, `go build`, `go vet`, or `lucind-checks.sh` in a judgment lane; the frozen transcript is already in `## Context`.
3. **Stage 3: Evidence Cross-Checking & Verdict Reconciliation.** Read both lanes' `.lucind/result.json` envelopes. Independently verify every cited `file:line` against the real codebase (green criteria are not proof of complete work). Four-case reconciliation:
   - **Unanimous Pass** (`done`/`done`): synthesizes `openspec/changes/<id>/verify.md` with overall status `PASSED`, consolidates complementary findings, updates `state.yaml` to `verify: { status: done }`.
   - **Disagreement / Disputed Defects** (`blocked`/`deviated`): confirmed spec violations mark overall verdict `BLOCKED` with remediation tasks in `state.yaml`; demonstrable false positives are refuted with concrete `file:line` evidence in `verify.md` without blocking.
   - **Lane Failure** (`failed` due to timeout/infra): re-dispatches the single failing lane before synthesis.
   - **Irreconcilable Ambiguity**: contradictory interpretations of underspecified requirements unresolvable from specs/design set overall verdict `BLOCKED` and escalate decision options to the human.

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
lucind-ai split --dag <path> --out <dir>
lucind-ai check [--out <path>]
lucind-ai --version
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--packet <path>` | String (repeatable) | *(required)* | Path to a packet file. Each instance adds one concurrent lane. |
| `--timeout <duration>` | Duration | `20m` | Wall-clock budget granted to each lane independently. |

`lucind-ai split` takes two required flags: `--dag` (path to an `apply-dag.yaml` sidecar) and `--out` (directory for emitted packet markdown). It prints one `lucind-ai run --packet …` line per wave; it does not dispatch those waves.

`lucind-ai check` runs `lucind-checks.sh` once via `internal/integrate.Check`. Transcript goes to stdout on pass and stderr on fail; `--out <path>` also writes the structured mechanical log (git SHA, command, duration, exit code, transcript). Exit 0 on pass, 1 on fail.

### Concurrency & Barrier

- **Parallel lanes**: Passing multiple `--packet` flags executes lanes concurrently in isolated worktrees (`../<repo>-worktrees/<id>`).
- **Independent clocks**: Each lane gets an independent deadline derived from `--timeout`; slow lanes never consume a sibling lane's budget.
- **Failure isolation**: Lanes never cancel sibling lanes. If one lane blocks, fails, or times out, all other lanes run to completion.

### Reports & Preserved Worktrees

- **Ledger**: SQLite database at `.lucind/lucind.db` records lane registrations, status transitions, and barrier events.
- **Envelope**: Dispatched runners write structured envelopes to `.lucind/result.json`, validated against `.lucind/result.schema.json`.
- **Preservation**: All lane worktrees are preserved on completion or failure.
- **Integrate IDs**: After the per-lane reports, stdout includes `integrated_ids:` and `reverted_ids:` (space-separated ids on the same line; an empty list prints the label with no ids). Read those lines — they are not a new report format.
- **Exit code**: Returns `0` only when every lane in the batch achieves `done` **and** none are listed in `reverted_ids`. Bisection can print `status: done` then revert; a `done` status line is not sufficient. Returns `1` if any lane blocked, deviated, failed, or was reverted.

