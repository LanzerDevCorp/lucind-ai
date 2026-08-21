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
| `executor` | Yes | Execution runtime to dispatch (currently `agy`, `cursor-agent`, or `opencode`). |
| `routed_by` | Yes | The explicit condition that triggered this routing decision — never the executor name. |
| `model` | No | Model name passed to executor. Omitted, each executor supplies its own default (`agy`: `gemini-3.7-flash-high`; `cursor-agent`: `cursor-grok-4.6-high`; `opencode`: `openai/gpt-5.6-sol`) — do not hardcode `gemini-3.7-flash-high` for a `cursor-agent` packet, it bills against Cursor's separate, more limited "Other Models" quota instead of the included "Cursor Models" quota. |
| `agent` | No | Opencode-only: names a purpose-built opencode agent (e.g. `lucind-dag` for DAG authoring, see `opencode agent list`) passed as `--agent`. Rejected before dispatch on any executor other than `opencode`, since agent selects a system prompt / tool-permission profile that only opencode has. |
| `read_only` | No | `true` or `false`. Omitted defaults to write. A `true` packet must produce no unique commits and leave a clean worktree. |
| `allowed_paths` | No | Single-line JSON array of repository-relative paths this packet may touch, e.g. `allowed_paths: ["internal/dag/", "cmd/lucind-ai/cli.go"]`. Omitted (or empty) is today's exact path: no overlap check across the batch, no post-run diff check. A YAML list under the key does not parse — the value after `:` must be one JSON array. |
| `feature` | No | Target feature identifier for parent integration. Required when targeting a feature branch unless `legacy_main: true` is set. |
| `parent_ref` | No | Target parent git reference (e.g. `refs/heads/feature/<id>`). |
| `base_sha` | No | Immutable commit SHA where the feature branch was created. |
| `expected_parent_sha` | No | Expected commit SHA of `parent_ref` before promotion. |
| `legacy_main` | No | `true` or `false`. Indicates legacy mode dispatch targeting `main`. |


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

### Where `.lucind/` ends and the change folder begins

Packet files go under `.lucind/packets/` for the reason in **Where to author packet files**. That
rule is one use of a larger split: `.lucind/` is runtime state, ignored on purpose;
`openspec/changes/<id>/` is the change's history, tracked.

| Location | Tracked? | Holds | Why this side exists |
|---|---|---|---|
| `.lucind/` | No (`.gitignore:2`) | In-flight packets, `.lucind/result.json`, the ledger, other worktree-local runtime files | Every lane writes `.lucind/result.json`. If `.lucind/` were tracked, that file would dirty `git status --porcelain`, so `enforceCompletionMode` would fail both write and read-only packets (`internal/run/run.go:628-661`). `Integrate` would refuse to promote while the primary root is dirty (`internal/integrate/integrate.go:25,112-126`). The same paths would fail the allowed-paths scope check, so the changed-path union skips `.lucind/` (`internal/run/run.go:599-601`). |
| `openspec/changes/<id>/` | Yes | Canonical phase artifacts, `apply-dag.yaml` when a DAG is wanted, and packet bodies copied in after a phase closes | These are the change. `lucind-ai split --dag` reads the sidecar from here; later phases, verify, and archive read the canonical files. |

**What the ignore costs, and the remedy.** Packet files are the instructions that produced the
work. They stay ignored while a batch runs for the reason above — which is why a sidecar authored
only under `.lucind/` never appears in git history, and reads as "never used" when the truth is
"used, never committed". Nothing stops copying a phase's packets into the change folder once that
phase closes. Worked precedent: `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-bodies/`
— packet bodies, tracked, archived, breaking nothing.

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

### Multi-lens planning fan-out convention (explore, propose, design, specs, tasks)

The standard planning fan-out convention across SDD planning phases (`explore`, `propose`, `design`, `specs`, `tasks`). Instead of two executors writing the same artifact twice, three `agy` lanes each own a disjoint slice of the phase document, and `cursor-agent` synthesizes the canonical document. The orchestrator's job shrinks from reading every draft to reading one notes file.

**Why three lenses and not three copies.** Running the same prompt three times converges harder, not less — you pay triple for one document. A lens is only worth a lane when it has its own required reading list, its own output skeleton, and an explicit cross-reference naming what the sibling lenses own. All lens template bodies carry those three things; strip any of them and the fan-out degenerates back into redundant copies.

| Phase | Lens A | Lens B | Lens C | Synthesis |
|---|---|---|---|---|
| `explore` | Problem & Candidates (`explore-lens-a.md`) | Capabilities & Scenarios (`explore-lens-b.md`) | Risks, Trade-offs & Spikes (`explore-lens-c.md`) | `explore.md` + `explore-synthesis-notes.md` |
| `propose` | Candidate & Approach (`propose-lens-a.md`) | Capability Impact & Specs (`propose-lens-b.md`) | Risks, Rollback & Test Impact (`propose-lens-c.md`) | `proposal.md` + `proposal-synthesis-notes.md` |
| `design` | Technical approach & decisions (`design-lens-a.md`) | Flow, invariants, deltas & file changes (`design-lens-b.md`) | Testing, threat matrix & rollback (`design-lens-c.md`) | `design.md` + `design-synthesis-notes.md` |
| `specs` | Delta specs lens A | Delta specs lens B | Delta specs lens C | Canonical specs + synthesis notes |
| `tasks` | Tasks lens A | Tasks lens B | Tasks lens C | Canonical `tasks.md` + synthesis notes |

Templates: `assets/explore-lens-{a,b,c}-packet-template.md`, `assets/explore-synthesis-packet-template.md`, `assets/propose-lens-{a,b,c}-packet-template.md`, `assets/propose-synthesis-packet-template.md`, `assets/design-lens-{a,b,c}-packet-template.md`, and `assets/design-synthesis-packet-template.md`.

**Dispatch — two invocations, no sidecar.** These are hand-authored write packets; `lucind-ai split` and `apply-dag.yaml` are not involved and sidecars are not required.

**Feature-branch ownership.** The orchestrator runs `lucind-ai feature create` before dispatch to initialize feature records in the ledger. Packets declare `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha`, or declare legacy mode with `legacy_main: true` (or dispatch with `--legacy-main`). Lanes do not create or move parent refs.

Dispatch supplies the target at run time, because the templates declare none. Against `main` that
means **both** flags, not either one: admission rejects legacy mode without an expected SHA, and an
expected SHA without legacy mode falls through to the four-field branch and fails there
(`internal/run/run.go:251-263`). Against a named feature parent, drop both flags and let the copied
packet name `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha` instead — the template
itself needs no edit either way, which is the property the target-less default exists to protect.

1. ```
   lucind-ai run --legacy-main --expected-parent-sha "$(git rev-parse refs/heads/main)" \
     --packet .lucind/packets/<phase>-<id>-lens-a.md \
     --packet .lucind/packets/<phase>-<id>-lens-b.md \
     --packet .lucind/packets/<phase>-<id>-lens-c.md
   ```
   Three lanes in parallel, each writing one distinct draft path, so the overlap check passes and no lane races another. The barrier joins when all three reach terminal status.
2. Confirm all three integrated, then dispatch the synthesizer the same way, recomputing the SHA — `main` moved when wave 1 integrated:
   ```
   lucind-ai run --legacy-main --expected-parent-sha "$(git rev-parse refs/heads/main)" \
     --packet .lucind/packets/<phase>-<id>-synthesis.md
   ```

**Two-tier operator remediation for wave-1 failure:**
1. *Admission failure* (`status: failed`, empty worktree path): Admission fails silently with no reason printed on stdout or stderr. The lane never reaches an executor. Check and repair the frontmatter target fields (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, or `legacy_main: true`) before looking anywhere else.
2. *Execution failure* (`blocked`, `failed`, or `deviated`): Remediate the issue and re-dispatch only the single failed lane. Dispatch wave 2 only after `integrated_ids` contains all three lens IDs. Unresolvable blockage stays with the operator; do not start synthesis.

The second invocation is **not optional and not merely sequencing**. Lens worktrees cannot see each other; the synthesis worktree is branched from the integrated result, which is the only point where all three drafts exist in one tree.

**The dependency this design accepts on purpose.** Lens B and lens C are downstream of lens A's choices, but run before it completes. The opening declaration is named for what lens A owns in that phase — not a hedged `## Assumed architecture` for every phase:

| Phase | Lens A owns | What B and C declare they are assuming |
|---|---|---|
| `explore` | problem space, candidate approaches, initial recommendations (`assets/explore-lens-a-packet-template.md`) | assumed problem and candidates |
| `propose` | selected candidate, technical approach, conceptual changes (`assets/propose-lens-a-packet-template.md`) | assumed candidate and approach |
| `design` | architecture decisions (`assets/design-lens-a-packet-template.md`) | `## Assumed architecture` |

The synthesizer treats lens A's declaration as authoritative, recording what B or C assumed instead under `## Architecture Divergence`. Independent convergence on the same choice is corroboration and is recorded as such; divergence means the decision was underdetermined, which is signal worth having.

**Budgets — and why there is one.** Each lens draft is capped under 1000 words; the canonical document under 1800 words.

The gap between them is the entire mechanism. If the synthesizer's output budget were as large as the sum of its inputs, "synthesize" could be satisfied by stapling three drafts together and nothing would force a choice. Roughly 3000 words of feedstock compressed to 1800 makes arbitration mandatory. A synthesis that lands near 3000 words concatenated rather than synthesized, and is a failed run even if every sentence in it is true. **The one number that must never invert: the canonical budget stays below the sum of the lens budgets.** Raise them together or not at all. Go binary does not parse or enforce word counts.

The second reason for a cap is downstream cost: planning artifacts are re-read by subsequent phases, apply, verify, and every judgment lane, so length is a tax multiplied by every consumer.

**Reading the real contract, and who wins.** All planning packets grant read-only access to `~/.claude/skills/sdd-*/` so the lanes read the `gentle-ai` phase contract as written instead of trusting a packet's paraphrase of it. Precedence between the two is deliberately **asymmetric**, and getting it backwards breaks the fan-out:

- **The skill wins on what a document must contain** — required sections, schemas, and content rules. A packet that paraphrases those and drifts is the thing that is wrong.
- **The packet wins on how the phase is executed** — the three-lane split, slice ownership, word budgets, output paths and skeletons, out-of-scope, done criteria.

The distinction is load-bearing because phase skills describe one sub-agent writing a whole document alone. Read as blanket authority it would tell every lens to write the complete document, persist it to Engram, and return the phase summary block — which would collapse three lenses back into three redundant full documents. Lanes follow the packet on execution topology and record conflicts in notes. Nothing outside the repository is ever written.

**What the orchestrator reads.** `<phase>-synthesis-notes.md`, and only that: `## Unresolved Contradictions`, `## Coverage Gaps`, `## Dropped Citations`, `## Architecture Divergence`. The synthesizer is instructed to escalate contradictions rather than pick, so a populated first section is a decision waiting for a human, not a defect.

**The risk this moves rather than removes.** In the dual pattern the orchestrator independently verified every `file:line` before accepting it. Here the synthesizer does that, in a worktree with the real code — which it can do, and which the template makes a done-criterion. But it is now the single place where a hallucinated citation can pass. If the citation-verification pass or the `## Dropped Citations` section is ever weakened, the fan-out loses the property that made it safe.

**Coverage checklist — per phase.** The synthesizer checks the canonical document against that phase's spine, not against the design spine. The eight-item design list is one instance. Headings may follow the change's own vocabulary; every spine item must be substantively present. Explore and propose below are the sections of the archived canonical artifacts (`openspec/changes/archive/2026-08-21-sdd-fan-out-lens/explore.md`, `proposal.md`); design is the list `assets/design-synthesis-packet-template.md` already checks. The explore and propose synthesizer templates encode the same concerns in lens-slice vocabulary. `specs` and `tasks` have no template spine yet — do not apply the design list to them.

| Phase | Spine |
|---|---|
| `explore` | What exists today; Built versus convention; Constraints and hard blockers; Candidate scopes (buys, costs, forecloses, would-touch); Prior art; The deciding question; Open questions |
| `propose` | Intent; Scope (in / out); Capabilities (new / modified); Approach; Affected Areas; Risks; Rollback Plan; Dependencies; Success Criteria; Review burden; Rejected alternatives; Open questions left to design |
| `design` | Technical approach; architecture decisions with alternatives and rationale; flow and invariants; file changes with terminal consumers; testing strategy and test seams; threat matrix with every row `Applicable` or `N/A: reason`; rollback and additivity; open questions and out of scope |


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
lucind-ai run --packet <path> [--packet <path> ...] [--timeout <duration>] [--approval-timeout <duration>] [--legacy-main] [--expected-parent-sha <sha>]
lucind-ai split --dag <path> --out <dir>
lucind-ai check [--out <path>]
lucind-ai serve [--addr <addr>]
lucind-ai feature create <id>
lucind-ai reconcile
lucind-ai renew
lucind-ai --version
```

### Subcommands

- `lucind-ai run`: Dispatch one or more packet lanes concurrently in isolated worktrees.
- `lucind-ai split`: Split an `apply-dag.yaml` sidecar into per-lane packets and print wave dispatch commands.
- `lucind-ai check`: Run repository checks once via `lucind-checks.sh` (`internal/integrate.Check`).
- `lucind-ai serve`: Start the HTTP API/web server for approvals and status monitoring (`--addr`).
- `lucind-ai feature`: Manage feature branches and parent integration in the ledger (`feature create <id>`, etc.).
- `lucind-ai reconcile`: Reconcile SQLite ledger state with worktrees and git refs.
- `lucind-ai renew`: Renew active lane leases or approvals in the ledger.

### `run` Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--packet <path>` | String (repeatable) | *(required)* | Path to a packet file. Each instance adds one concurrent lane. |
| `--timeout <duration>` | Duration | `20m` | Wall-clock budget granted to each lane independently. |
| `--approval-timeout <duration>` | Duration | `30m` | Wall-clock timeout when waiting on operator approval before aborting. |
| `--legacy-main` | Bool | `false` | Dispatch in legacy mode targeting `main` without feature target metadata. |
| `--expected-parent-sha <sha>` | String | `""` | Specify expected commit SHA of parent reference before merging. |

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

