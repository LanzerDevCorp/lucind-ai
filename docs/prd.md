# lucind-ai — PRD

**Status:** requirements fixed, nothing built. **Date:** 2026-08-17. **Audience:** the author only.

This document supersedes `handoff-2026-08-13.md` (deleted) and the orchestration model described in
the pre-2026-08-14 README. Where it disagrees with anything older, it wins.

---

## 1. What this is

`lucind-ai` is the **delegated-execution layer for work paid by subscription**. A single Go binary
routes *execution* work to CLI agents already covered by a subscription, isolates each in its own
worktree, and refuses to believe what comes back until it satisfies a schema.

It owns **parallel execution and the integrity of what returns**. It does not own review, delivery,
or lifecycle — those belong to `gentle-ai`.

Three earlier framings were considered and rejected:

| Framing | Rejected because |
|---|---|
| Full orchestrator (old README) | Competes with `gentle-ai`, which already owns SDD and RDD |
| Trust-contract layer | `gentle-ai` 2.3.0 already is one, with far more machinery |
| Runtime-admission layer for reviewers | Not needed — the executors do not have to be reviewers |

## 2. Why it exists

> "I can't use all my subscriptions at once in the same workflow, and I can't take advantage of
> each agent for what it does best."

Two halves, and both are load-bearing: **parallel capacity across subscriptions**, and **routing by
aptitude**.

There is also a founding defect this layer exists to catch. A packet came back `done` with every
criterion green and real command output attached, and still had a defect. The next round did it
again. Both times the packet carried an explicit hard stop covering exactly that case, and the
executor walked past it and reported success without contradicting itself. Both runs happened
without RDD, and the failure was in *execution*, not review — RDD arrives too late to see it.

That is the split this project depends on:

- **`gentle-ai` RDD** answers *"is this diff any good?"* — code quality, post-hoc, over a frozen candidate.
- **The `lucind-ai` envelope** answers *"did the executor do what I asked, and did it declare where it stopped?"* — delegation integrity, at the moment of return.

Different objects. Not redundant.

## 3. Hard constraints

1. **Subscriptions only.** Never a metered per-token API key. This is the constraint that gives the
   project a reason to exist; if API keys were acceptable, existing meta-harnesses already cover
   this ground.
2. **Single user.** No distribution, no compatibility matrix, no install UX for third parties.
3. **`gentle-ai` is not ours to patch.** It is `Gentleman-Programming/gentle-ai`, runtime admission
   lives in its compiled Go, and modifying it from a consumer workflow is forbidden. Every design
   here works through surfaces it already exposes.
4. **Strict TDD** for the binary.

## 4. The roster

**The unit of parallelism is the subscription, not the CLI.** Two CLIs drawing on one subscription
add no capacity. All rows verified by execution on 2026-08-17 (WSL2):

| Subscription | CLI | Auth | Role |
|---|---|---|---|
| Anthropic | Claude Code | — | orchestrator; bounded conflict resolution |
| ChatGPT / OpenAI | `opencode` → `openai/gpt-5.6-sol` | `oauth` | reviewer (drives RDD) |
| Google Antigravity | `agy` 1.1.13 | `~/.gemini/antigravity-cli/antigravity-oauth-token` | executor |
| Cursor | `cursor-agent` 2026.08.11 | logged in | executor |

Excluded, with reasons:

- **`codex`** — currently `Not logged in`, and logging it in would draw on the *same* ChatGPT
  subscription `opencode` already uses through OAuth. A second door to the same room; zero parallel
  capacity gained.
- **`opencode-go`** — authenticated `api`, i.e. metered per token. Violates constraint 1.
- **`gemini` CLI** — dead; `agy` is its architectural successor.

**Execution capacity is two subscriptions, not four.** The four are role-specialized and not
interchangeable: Anthropic orchestrates, ChatGPT reviews. Only Antigravity and Cursor execute, and
the aptitude map means they do not substitute for each other either. A subscription burned by a
runaway lane is not "one of four" — it is half the execution capacity, and specifically the half
that does that kind of work.

## 5. Aptitude map

Recorded as a hypothesis to falsify — but the research supports it, so it is no longer a guess.

`agy` is documented as built for asynchronous workflows that orchestrate multiple agents in the
background to run large-scale refactors, multi-agent by default, defaulting to a fast Flash-tier
model that trades error rate for speed. Cursor's CLI is documented as excelling at "IDE-style code
editing tasks", and the published Cursor Router sends routine broad work to cheap models while its
complexity predictor escalates non-trivial logic to frontier reasoning models. No head-to-head
benchmark exists, and a cited SWE-Bench figure putting a Gemini Pro tier at a 19.4% resolve rate on
long-horizon multi-file refactoring should be treated as unverified. The split mirrors documented
engineering intent on both sides.

Still corrected when envelope data says otherwise.

| Executor | Gets | Rationale |
|---|---|---|
| `agy` | Sweeps and volume — exploring 4+ files, broad mechanical change, repetitive refactors | Fast and cheap, more error-prone; the envelope contains the risk |
| `cursor-agent` | Single-piece precision — one file, non-trivial logic | Its repo context pays off on focused work |

## 6. The v1 flow

1. **The orchestrator (Claude Code) writes the packets and decides the lane split.** This stays
   prose — it is judgment, not flow control.
2. **The binary creates one worktree per lane** at `<repo-parent>/<repo-name>-worktrees/<name>`,
   never a temp dir. Each worktree needs its own `.codegraph` index; never copy or share one.
3. **The binary dispatches both lanes in parallel**, headless (`-p`).
4. **Each executor writes its envelope to `.lucind/result.json` inside its own worktree.** The
   binary reads it from disk and validates it against `result.schema.json`.
   This is deliberate: `agy` has `--json-schema` (real enforcement) but `cursor-agent` has only
   `--output-format json`, which is its own agent wrapper, not our schema. Writing to a known path
   normalizes the asymmetry — validation happens once, in one place, identically for every lane, and
   adding a third executor later changes nothing. `agy`'s `--json-schema` stays on as a belt in
   addition to the braces.

   For the lane that cannot be constrained at the source, the packet **injects the schema into the
   prompt itself** — raw JSON, no code fences — and the binary validates what lands on disk. This is
   the documented pattern for non-cooperative CLIs; result-to-known-file is documented too, in
   `openai/codex-action@v1` (`output-file`) and in Claude Code subagents (`outputFile`).
5. **Barrier.** It releases when **every** lane reaches a terminal state — `done | blocked |
   deviated | failed` — never on `done` alone, or one `blocked` lane hangs the run forever.
6. **Integrate the `done` lanes through a temporary combined tree** — never straight into the target
   branch. Merge them together first, resolve conflicts, then run the project's checks against that
   combined state.
   - **Green** → integrate the batch.
   - **Red** → bisect to isolate the lane that broke it, integrate the rest, and return the isolated
     lane to `blocked` with its worktree preserved.

   Conflicts go to `claude -p --model sonnet`, **bounded to 400 lines**; if it cannot close, it
   escalates to the human with both versions intact.
7. **Lanes that were not `done` at the barrier never enter integration.** Their worktrees are
   preserved and their questions are relayed verbatim.
8. **Remove only the worktrees of lanes that integrated.**
9. **Stop.** The binary does not trigger RDD.
10. **The human runs RDD from an `opencode` session** (`--agent opencode`) with `gpt-5.6-sol`.

## 7. Behavioral rules

**Quota.** When a subscription runs dry, **that lane dies** — the packet returns `blocked`. It is
never re-routed to another executor: re-routing breaks the aptitude map and silently changes who did
the work, and it couples lanes that must stay independent. Downgrading the model *within the same
subscription* is allowed (`agy` 3.7-flash-high → 3.6 → 3.5), because neither the subscription nor
the aptitude class changes.

**Lane bounds.** A lane gets a generous budget and a hard ceiling. Iteration is how the work gets
done — an executor that tries, fails and retries is working, and the budget should not fight that.
The ceiling exists for a different failure: an executor that misreads an external error as its own
and thrashes without converging. That lane produces nothing while it burns, and no amount of
subscription headroom makes it acceptable.

Three bounds, and **the binary owns all of them**:

| Bound | Enforced by | Note |
|---|---|---|
| Wall clock per lane | The binary — `context.WithTimeout` and killing the child | The only one actually available: `cursor-agent` exposes no timeout flag at all, and `agy`'s `--print-timeout` defaults to 5m |
| Attempts per packet | The binary | Beyond it, the lane closes; no silent retry |
| Turns per run | **Not available** | Neither executor exposes a turn or iteration cap |

`agy --print-timeout` is set *above* the binary's own deadline, so the binary is always the one that
decides. A lane killed on its ceiling returns **`blocked` with the worktree preserved** — never
`failed` with cleanup, because the evidence of why it looped is inside that worktree.

**Worktree isolation is a requirement, not tidiness.** It is what prevents the documented failure
class of parallel agents: editing files another lane is reading, misreading a neighbour's in-progress
broken build as their own defect, and drifting into adjacent directories to "helpfully" fix lint
errors and imports that another lane is mid-way through changing. No lane ever runs in the main tree.

**A failed lane must not block a good one** — but nothing merges unverified either. That is what the
combined tree in step 6 is for.

**Blocked.** The binary exits **non-zero**. That exit code replaces the `UserPromptSubmit` hook that
was designed and never built. An orchestrator that has to *remember* to read the approval queue is
the most fragile possible arrangement; a binary that fails in your face is not.

**The human lane.** Human packets run **serially, outside the parallel batch**. A human cannot be
raced against a `--print-timeout`. No agent ever generates, enters, or writes a credential value.

**Hard stops.** Every hard stop listed in a packet must appear in the returned envelope whether or
not it fired. An envelope that omits one is invalid regardless of its criteria — this field exists
because green criteria have twice concealed a violated hard stop.

## 8. What gets built

### 8.1 The binary

Go 1.24.2, module `github.com/LanzerDevCorp/lucind-ai`, one command: `lucind-ai`. Simple, fast,
deterministic flow control. It owns worktrees, dispatch, envelope validation, the barrier, the
merge, cleanup, and the ledger.

### 8.2 The ledger — SQLite in the primary repo's `.lucind/`

Two concurrent lanes writing state make a lock-and-JSON-file arrangement fragile. SQLite gives
atomic transactions and a pure-Go driver with no cgo.

**There is exactly one ledger, and it lives in the primary repository's `.lucind/`** — never inside a
worktree. It tracks every lane in the batch, so it cannot belong to any single lane. A worktree's own
`.lucind/` holds only that lane's `result.json` (§6 step 4). Two directories, same name, different
jobs.

It replaces both `state.json` and `approvals.md`. It stays **small**: it is a ledger, not a record
of what happened — the narrative goes to engram. A routing decision is stored together with the
condition that triggered it; there is no implicit routing.

Adopting Gas Town's convoy/bead model was considered and rejected, and the research settles it: its
beads are prefixed 5-character work items (`gt-abc12`) bundled into labelled convoys, but **the
actual SQL schema is not documented anywhere in the sources**, and the model exists to coordinate
20–30 background agents behind a three-tier watchdog chain. Over-engineered for two lanes.

What is worth stealing is its *event list*, not its schema — Gas Town records session lifecycle,
agent state changes, calls with duration, worker spawn and removal, and completion. Those are the
right things to write down.

### 8.3 The approvals web UI

Served by the binary itself — `lucind-ai serve`. Plain HTML/CSS/JS embedded with `embed`, stdlib
`net/http`, reading the same SQLite. No npm, no build step, no dependency. **Bound to localhost
only**, because this interface can approve work.

When a lane requests approval the binary **waits**, with a configurable timeout — blocking keeps the
batch alive and the worktree warm, and the timeout prevents the infinite hang.

It is also the control surface for the whole cycle: it shows a merged batch as ready for review,
with the exact `opencode` command to run.

**Four hard rules, in response to a stated habit — "it asks me several things and I always say yes
to everything":**

1. **No "approve all" button.** Not anywhere, not hidden.
2. **Every item is decided individually,** starting with nothing selected.
3. **Evidence inline and visible** — command output or `file:line`, never a claim. You cannot
   approve what you cannot see.
4. **The ledger records who approved, when, and whether a defect later surfaced in that same
   packet** — and the UI shows your own rate of approvals that went wrong.

Rule 4 is the one that matters. It turns "I always say yes" from an invisible habit into a number
you look at. Without it, the other three are friction you learn to route around in two weeks.

### 8.4 What remains prose

`SKILL.md` shrinks to two things: **how to write a good packet** (the prompt contract, done
criteria, hard stops) and **how to drive the binary**. All flow control becomes Go.

The Claude Code marketplace and `plugin.json` are removed — distribution machinery for an audience
of one.

## 9. The boundary with gentle-ai

`lucind-ai` owns parallel execution and its integrity. `gentle-ai` owns review and delivery. The
orchestrator is the hinge.

**The automated flow ends in RDD, never in judgment-day.** They are alternatives, not synonyms, and
`judgment-day/SKILL.md` v1.7 is explicit: *"A judgment issues no receipt and carries no delivery
authority: it satisfies no commit, push, PR, or release gate"* (line 24), and *"never run both"* as
the adversarial method for one target (line 12). A pipeline ending in judgment-day leaves every run
standing at the gate without a key. Judgment-day stays available as a tool invoked by hand.

**RDD is driven from `opencode`, not from Claude Code.** Each admitted runtime executes its reviewer
where the driver runs, so reviewing with `gpt-5.6-sol` means driving the lifecycle from an `opencode`
session. This is the intended outcome, not a compromise: a same-family model echoes the caller's
reasoning instead of checking it, and the work was orchestrated by Claude.

The binary cannot drive RDD itself — the review lifecycle is a negotiated transition machine that
launches reviewers as host subagents and relays consent envelopes to a human in conversation.

## 10. v1 acceptance criterion

> Two independent tasks are dispatched **simultaneously** to `agy` and `cursor-agent`, each in its
> own worktree; both return envelopes valid against the schema; neither blocks the other on quota.

The serial single-executor variant was rejected: it never demonstrates "all my subscriptions at
once", and it hides the real risk — concurrency and the ledger — until v2. Every time something in
this project ran for the first time, it exposed a defect deliberation had not found. If parallelism
is the point, it runs in v1.

## 11. Out of v1

| Deferred | Why it is worth doing later |
|---|---|
| Cross-family parallel judges on `agy`/`cursor` | Real independence — today's `jd-judge-a/b` are both Opus, the orchestrator's own family |
| Automated conflict resolution past 400 lines | Only if the data shows conflicts are frequent and boring |
| Distribution / marketplace | Audience of one |
| A third executor | Two lanes prove aptitude routing |

## 12. Known operational hazards

- **The cascade failure loop.** In parallel setups where lanes touch overlapping scope, one lane's
  change produces build errors a second lane sees. If that second executor reads the external error
  as its own defect, it thrashes — modifying things that were never broken — and drains the
  subscription without converging. Worktree isolation prevents the shared-scope half of this; the
  wall-clock ceiling in §7 prevents the rest.

- **`agy --print-timeout` defaults to 5m.** A long packet dies on its own unless this is raised
  explicitly.
- **`cursor-agent` keeps its own `.cursor/worktrees.json`,** which can collide with ours.
- **The conflict resolver shares the orchestrator's subscription.** `claude -p` draws on the same
  Anthropic entitlement this thread runs on — same room, by the project's own rule. Accepted because
  conflicts are rare and short, but recorded.
- **A same-family verdict still blocks and still raises objections;** it just cannot approve a merge
  on its own.

## 13. Open items inherited from the deleted handoff

These were live before this PRD and are not resolved by it:

1. **The orchestration skill is still installed in `agy`.** An executor holding a document that
   explains how to dispatch, route, and merge can be triggered by description and widen its own
   scope — the exact failure the rest of the design fights. Under this PRD the plugin goes away, so
   the action is to uninstall it, not to split it.
2. **`codegraph` and `context7` are not in `agy`'s `mcp_config.json`** — only engram is. They are
   copied by hand, not installed as plugins. Executors need them.
3. **"Which skill for which task" was never worked through.** Untouched.

## 14. What the research settled

Four questions were put to the notebook, derived from these requirements rather than from an open
field. Results, folded into the sections above:

| Question | Outcome |
|---|---|
| Prior art for parallel dispatch with a barrier | Gas Town (polecats in worktrees + tmux, Refinery merge queue, Bors-style bisection), multiclaude (opportunistic merge on individual CI), Omnigent/Polly (parallel worktrees, cross-vendor review, but *"You merge"*) |
| Partial failure at the join | Gas Town merges survivors and bisects; multiclaude merges each lane independently and relies on a supervisor to clean up after; nobody does serialized rebase-and-revalidate. **We adopted Gas Town's model.** |
| Gas Town's ledger schema | Not documented in the sources, and over-engineered for two lanes. Build our own; steal the event list. |
| Validating non-cooperative CLIs | Prompt-injected schema plus client-side validation, and result-to-known-file, are both documented patterns. **Confirms §6 step 4.** |
| Aptitude map | Supported by documented design intent on both sides. Not a superstition. |

Two claims from earlier research **did not survive**: Hermes Agent's "typed result contract" and the
"telephone game" framing are not documented in the sources at all, and Omnigent/Polly turns out not
to automate merging.

## 15. Next step

Build the binary, TDD, starting with the ledger and the barrier — the two pieces every other part
depends on.
