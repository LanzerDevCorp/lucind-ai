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

**What the ignore costs, and who pays it back.** Packet files are the instructions that produced
the work, and the result envelopes are what each lane returned. Both stay ignored while a batch
runs for the reason above — which is why a sidecar authored only under `.lucind/` never appears in
git history, and reads as "never used" when the truth is "used, never committed".

They are also invisible from inside a lane. A lane worktree is a fresh checkout, so it inherits no
ignored file; `run.Execute` creates `.lucind/` there holding only that lane's `result.schema.json`
and `result.json` (`internal/run/run.go:689-701`). A lane that needs the primary root's
`.lucind/packets/` must be granted it as a read-only path outside the repository.

Preserving them is the archive phase's job, and `assets/archive-packet-template.md` does it
mechanically — see **Archive dispatch** below. Earlier precedent, narrower:
`openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-bodies/` — apply packet bodies only,
tracked, archived, breaking nothing.

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
| `specs` | Capabilities & Requirements (`spec-lens-a.md`) | Scenarios & Coverage (`spec-lens-b.md`) | Live-Spec Conflicts & Migration (`spec-lens-c.md`) | `specs/<capability>/spec.md` + `spec-synthesis-notes.md` |
| `tasks` | Decomposition & Ordering (`tasks-lens-a.md`) | Partition & Dispatch Shape (`tasks-lens-b.md`) | Proof & Review Burden (`tasks-lens-c.md`) | `tasks.md` + `tasks-synthesis-notes.md` |

Templates: `assets/<phase>-lens-{a,b,c}-packet-template.md` and `assets/<phase>-synthesis-packet-template.md` for each of `explore`, `propose`, `design`, `spec`, and `tasks` — twenty files. The template basename for the `specs` phase is singular (`spec-lens-a-packet-template.md`), matching its draft paths; the phase itself is named `specs` everywhere else.

**Dispatch — two invocations, no sidecar.** These are hand-authored write packets; `lucind-ai split` and `apply-dag.yaml` are not involved and sidecars are not required.

**`specs` and `design` are siblings, not a sequence.** Both consume the accepted proposal and nothing else, so they can run as two concurrent fan-outs. This is not a local convention — the real `gentle-ai` design skill marks its spec input `optional — may not exist if running in parallel with sdd-spec` (`~/.claude/skills/sdd-design/SKILL.md:43`). Running `specs` first out of habit costs a full phase's wall clock for nothing.

Concurrently means one wave-1 invocation with six lens packets (`spec-lens-{a,b,c}` plus `design-lens-{a,b,c}`) and one wave-2 invocation with both synthesizers. Draft paths are already pairwise disjoint across the two phases, so the overlap check passes as written.

Two consequences to write into the packets when they run this way:

- Each design lens's `## Context` must state that `openspec/changes/<change-id>/specs/` does not exist yet. Its preconditions already admit that case; leaving it unsaid makes a lane hunt for a directory that is being written next to it.
- Design lenses then reason from the proposal's Capabilities section, not from requirement ids. A design that cites `specs/` requirement names it never read is citing something it invented.

`tasks` is the one planning phase that is genuinely downstream: it needs both the delta specs and the design, so it runs after both synthesizers integrate.

**Feature-branch ownership.** The orchestrator creates the parent branch in git *and* runs `lucind-ai feature create --id … --parent … --base-sha …` before dispatch to initialize the ledger record — the command writes the row, it does not create the branch. It then checks that branch out in the primary repository, because integration promotes into the current checkout rather than into `parent_ref`; see **Multi-feature orchestration** below for what that does and does not buy. Packets declare `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha`, or declare legacy mode with `legacy_main: true` (or dispatch with `--legacy-main`). Lanes do not create or move parent refs.

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
| `specs` | the capability map and every requirement statement (`assets/spec-lens-a-packet-template.md`) | `## Assumed requirements` |
| `tasks` | the phased checklist and its dependency order (`assets/tasks-lens-a-packet-template.md`) | `## Assumed decomposition` |

The synthesizer treats lens A's declaration as authoritative, recording what B or C assumed instead under that phase's divergence section — `## Architecture Divergence` for design, `## Approach Divergence` for explore, `## Scope Divergence` for propose, `## Requirement Divergence` for specs, `## Decomposition Divergence` for tasks. Independent convergence on the same choice is corroboration and is recorded as such; divergence means the decision was underdetermined, which is signal worth having.

**One deliberate exception to lens A's authority.** In `specs`, lens C's live-spec evidence outranks lens A on *classification only*: if lens A called a requirement `ADDED` and lens C found the live requirement it contradicts, it is `MODIFIED`. Lens C is the lane that opened `openspec/specs/`; lens A read only the index. Lens A still owns the requirement set and its text. Everywhere else, and for everything else in `specs`, lens A is authoritative without exception.

**Budgets — and why there is one.** Each lens draft is capped under 1000 words; the canonical document under 1800 words.

The gap between them is the entire mechanism. If the synthesizer's output budget were as large as the sum of its inputs, "synthesize" could be satisfied by stapling three drafts together and nothing would force a choice. Roughly 3000 words of feedstock compressed to 1800 makes arbitration mandatory. A synthesis that lands near 3000 words concatenated rather than synthesized, and is a failed run even if every sentence in it is true. **The one number that must never invert: the canonical budget stays below the sum of the lens budgets.** Raise them together or not at all. Go binary does not parse or enforce word counts.

The second reason for a cap is downstream cost: planning artifacts are re-read by subsequent phases, apply, verify, and every judgment lane, so length is a tax multiplied by every consumer.

**The one budget exception, and why it is not a loophole.** In `specs`, both lens C's cap and the synthesizer's cap exclude scenarios copied verbatim from a live spec inside a `MODIFIED` block. Archive replaces the live requirement with exactly what the delta says, so a scenario trimmed to hit a word count is deleted from the capability — a silent regression with no failing test behind it. The exclusion covers copied evidence only; every word either lane writes itself still counts, so the compression gap that forces arbitration is untouched.

**Reading the real contract, and who wins.** All planning packets grant read-only access to `~/.claude/skills/sdd-*/` so the lanes read the `gentle-ai` phase contract as written instead of trusting a packet's paraphrase of it. Precedence between the two is deliberately **asymmetric**, and getting it backwards breaks the fan-out:

- **The skill wins on what a document must contain** — required sections, schemas, and content rules. A packet that paraphrases those and drifts is the thing that is wrong.
- **The packet wins on how the phase is executed** — the three-lane split, slice ownership, word budgets, output paths and skeletons, out-of-scope, done criteria.

The distinction is load-bearing because phase skills describe one sub-agent writing a whole document alone. Read as blanket authority it would tell every lens to write the complete document, persist it to Engram, and return the phase summary block — which would collapse three lenses back into three redundant full documents. Lanes follow the packet on execution topology and record conflicts in notes. Nothing outside the repository is ever written.

**What the orchestrator reads.** `<phase>-synthesis-notes.md`, and only that: `## Unresolved Contradictions`, `## Coverage Gaps`, `## Dropped Citations`, and that phase's divergence section. The synthesizer is instructed to escalate contradictions rather than pick, so a populated first section is a decision waiting for a human, not a defect.

**The risk this moves rather than removes.** In the dual pattern the orchestrator independently verified every `file:line` before accepting it. Here the synthesizer does that, in a worktree with the real code — which it can do, and which the template makes a done-criterion. But it is now the single place where a hallucinated citation can pass. If the citation-verification pass or the `## Dropped Citations` section is ever weakened, the fan-out loses the property that made it safe.

**Coverage checklist — per phase.** The synthesizer checks the canonical document against that phase's spine, not against the design spine. The eight-item design list is one instance. Headings may follow the change's own vocabulary; every spine item must be substantively present. Explore and propose below are the sections of the archived canonical artifacts (`openspec/changes/archive/2026-08-21-sdd-fan-out-lens/explore.md`, `proposal.md`); design is the list `assets/design-synthesis-packet-template.md` already checks. The explore and propose synthesizer templates encode the same concerns in lens-slice vocabulary. The `specs` and `tasks` spines below are the lists `assets/spec-synthesis-packet-template.md` and `assets/tasks-synthesis-packet-template.md` check; both are derived from the real `gentle-ai` skills rather than from an archived artifact, because no archived change in this repository ran either phase as a fan-out yet.

| Phase | Spine |
|---|---|
| `explore` | What exists today; Built versus convention; Constraints and hard blockers; Candidate scopes (buys, costs, forecloses, would-touch); Prior art; The deciding question; Open questions |
| `propose` | Intent; Scope (in / out); Capabilities (new / modified); Approach; Affected Areas; Risks; Rollback Plan; Dependencies; Success Criteria; Review burden; Rejected alternatives; Open questions left to design |
| `design` | Technical approach; architecture decisions with alternatives and rationale; flow and invariants; file changes with terminal consumers; testing strategy and test seams; threat matrix with every row `Applicable` or `N/A: reason`; rollback and additivity; open questions and out of scope |
| `specs` | Every capability the proposal names has a file at the right path (new full spec versus delta); every requirement classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`; an RFC 2119 keyword in every requirement; at least one scenario per requirement; happy-path and edge-case scenarios in GIVEN / WHEN / THEN; every `MODIFIED` block the complete live block; Reason and Migration on every removal and rename; no implementation detail |
| `tasks` | Review Workload Forecast with every field populated; Suggested Work Units, each a standalone deliverable with a rollback boundary; a phased checklist whose tasks are specific, actionable, verifiable and small; a RED-test task before its production task for every threat-matrix row the design marked `Applicable` and none for an `N/A` row; explicit dependency order; every wave green on its own under `Integrate` and every same-wave unit pair path-disjoint; an executor named per unit where a DAG is intended; every requirement traced to at least one task |

**Tasks fan-out — every wave must survive `Integrate`.** Path disjointness and ordered dependencies are not enough. `Integrate` runs `lucind-checks.sh` on the combined tree (`internal/run/integrate.go:50-59`; `internal/integrate/integrate.go:83-91`) and bisects a failing batch (`internal/run/integrate.go:28-30,83-84`). A wave whose accepted done criterion is that tests fail is reverted before its successor can turn them green.

A partition is viable only when every wave can pass those checks on the combined tree by itself. Strict-TDD wave splitting is incompatible with that gate: RED and GREEN for one unit belong in one lane. Repository precedent: `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` declined a DAG split (a two-node DAG was possible; Unit 1 was too small to pay for sidecar orchestration) and used a single packet.

**Size forecast for template work.** Forecast fan-out template work at roughly 150 lines per template. The `sdd-fan-out-lens` tasks lens C forecast 120–250 changed lines against an actual 1730, and neither sibling lens nor the synthesizer challenged it, with eight ~150-line templates already visible in the existing design set.

### Archive dispatch — one mechanical lane

Archive is dispatched like every other phase, through `lucind-ai run`, from
`assets/archive-packet-template.md`. It is deliberately **not** a fan-out: one `agy` lane, no lens
split, no synthesizer, and no word budget.

**Why no fan-out here.** Three lenses would produce three opinions about a `git mv`, and a
synthesizer's job is compression — applied to an audit trail whose entire value is that nothing was
compressed. The phase's one real judgment, whether the change may close at all, is a gate with
fixed inputs: an unchecked task in `tasks.md` or a CRITICAL in `verify.md` blocks, with no override.

**What it moves, in order.** The ordering is load-bearing: once the change folder moves there is
nowhere left to copy into.

1. Gates: task completion, CRITICAL verification issues, missing artifacts.
2. Preserve the session's dispatch record — every packet from `.lucind/packets/` and every envelope
   from `.lucind/results/` into `openspec/changes/<id>/packets/` and `envelopes/`. This is the step
   that pays back the ignore cost above, and it supersedes the narrower `apply-bodies/` precedent by
   covering every phase rather than apply alone.
3. Merge the delta specs into `openspec/specs/`. A `MODIFIED` block replaces the **entire** live
   requirement, scenarios included — which is why the spec phase made lens C copy the whole block.
4. Write `archive-report.md`.
5. `git mv` the change folder into `openspec/changes/archive/<YYYY-MM-DD>-<id>/`.

**The copy rule, and its only acceptable evidence.** The `gentle-ai` archive skill's Mechanical
Copy Contract governs: file content never passes through the model's Read/Write path. Copies and
moves use `cp -R`, `mv`, or `git mv`, and every one is followed by a `diff -r` readback whose
**verbatim** output goes in the result envelope. Empty is the only pass; a skipped `diff -r` fails
the phase, because an agent's self-report that bytes survived is not evidence that they did. A
model that truncates one file while reporting success corrupts the audit trail with nothing
downstream to catch it.

Editing a live spec in step 3 is a targeted structural edit, not a whole-file copy, and is the one
place Read/Write is correct.

**Allowed paths name the change, not the directory.** `openspec/changes/<change-id>/` and
`openspec/changes/archive/` are granted separately rather than `openspec/changes/`, so an archive
lane cannot reach another in-flight change — which starts mattering the moment two changes are open
at once.

### Multi-feature orchestration

A batch that names a feature target promotes through the durable attempt state machine
(`internal/run/attempt.go:217`), reached from `lucind-ai run` via `IntegrateFeature`
(`internal/run/integrate_feature.go`). A legacy batch keeps the ff-merge it always used. The route
is decided by the packets, before dispatch.

| | Legacy batch (`--legacy-main`) | Feature batch |
|---|---|---|
| Lane worktree base | primary checkout's `HEAD` | the packet's `base_sha` |
| Promotion | `git merge --ff-only` into the primary checkout | compare-and-swap on `parent_ref` |
| Primary working tree | must be clean; receives the merge | never touched |
| Lease | none | held for the whole attempt |
| Cross-feature overlap gate | not run | runs before promotion |
| Failure isolation | bisects to the clean subset | no bisection; the attempt fails whole |
| Durable record | lane rows only | an `integration_attempts` row, recoverable |

**Which checkout you are on stops mattering for a feature batch.** Promotion is a CAS on the named
ref and does not check out, merge into, or otherwise mutate the primary working tree, and lane
worktrees start at `base_sha` rather than at `HEAD`. Both halves are required: before the second
one landed, lanes branched from whatever was checked out and CAS-promoted a tree the parent never
contained — a silent wrong-base merge that succeeded.

**Four things are refused before any lane dispatches**, so a batch that cannot land coherently
burns no quota (`internal/run/integrate_feature.go`, `FeatureTarget`):

1. Two packets naming different features. One batch produces one combined tree and promotes it
   once; there is no correct answer, so it is not guessed.
2. Legacy and feature-targeted packets mixed in one batch.
3. The same feature with divergent `expected_parent_sha`.
4. A feature whose `parent_ref` is `main`, or the `lucind/` lane namespace, or empty
   (`feature.ValidateParentRef`). **A change targeting `main` is a legacy dispatch**, not a feature
   whose parent happens to be main — `--legacy-main` with `--expected-parent-sha`.

A packet declaring no target at all, dispatched with no flags, is likewise refused up front and
told both exits. That used to surface as a per-lane `status: failed` after every worktree already
existed, with no reason printed anywhere.

**What the overlap gate does now that it runs.** `evaluateOverlapGate`
(`internal/run/attempt.go:623`) compares the candidate against every other active feature in the
same ledger and classifies (`internal/overlap/overlap.go:623-678`):

- **required** — a predicted merge conflict, a rename/delete collision, a shared binary,
  intersecting or nearby hunks, or a hotspot weight over threshold. Blocks promotion, releases the
  lease, and creates one awaiting reconciliation request per feature pair, deduplicated across
  retries. The lanes are demoted with the block as their reason and their worktrees preserved.

  **Clearing it takes two steps, not one.** `lucind-ai reconcile approve --request <id> --source
  <feature> --target <feature>` only authorizes a candidate (`reconciliation_candidates`, status
  `candidate_running`) — it does not itself unblock anything, because approving does not resolve
  the actual textual conflict. A human resolves it out of band (by hand, or via a bounded `claude
  -p --model sonnet` session against the candidate's `allowed_paths`), produces a real commit, and
  registers it with `lucind-ai reconcile resolve --candidate <id> --sha <sha> [--actor <name>]`
  (`sha` is verified against this repo's real commit graph before being accepted). Retrying the
  blocked feature's own `lucind-ai run --packet …` afterward promotes that registered SHA instead
  of the retry's own fresh combined tree — matched by whether the *other* feature's tip has moved
  since the resolution was registered (`internal/run/attempt.go`'s `evaluateOverlapGate`), not by
  re-deriving a new hash every retry, since a retry's own candidate SHA is never bit-identical to
  the one that got blocked. `decline`/`cancel` remain the "this cannot be reconciled" exits, with
  no candidate ever authorized.

  **The retried lane's own worktree must not already exist.** A blocked lane's worktree is
  preserved for inspection (by design), so re-dispatching the identical packet id fails with
  `worktree: target worktree path already exists` until that worktree is removed by hand (`git
  worktree remove --force <path>`) — there is no `lucind-ai` command for this yet.
- **warning** — merely shared paths, or a hotspot over the lower threshold. Records evidence, does
  not block.
- **informational** — a no-op.

**The attempt is durable, and its id is printed.** `lucind-ai run` prints
`attempt:   <id> (<status>)`. If the process dies mid-attempt the row is non-terminal, and
`lucind-ai feature recover --attempt <id>` resumes it. Until this route existed, that command had
nothing to recover.

**The lease is not renewed mid-attempt.** It is held across combine and the full check run, so its
TTL is pinned to the same `--timeout` a lane gets rather than the package's 30-second default,
which would expire during `lucind-checks.sh` and land a passing attempt in `stale`. A second
concurrent dispatch on the same feature is blocked, not raced: `feature.AcquireLease` grants only
on an expired lease, with no same-owner exception (`internal/feature/feature.go:307`).

**Two features at once — what is now true, and what is still unproven.** The structural blocker is
gone: with CAS promotion there is no shared checkout for two batches to fight over, and because one
clone means one ledger, the overlap gate can actually see both features. That is a better shape
than two clones, where the two ledgers would be blind to each other.

It has never been run. Every one of `features`, `integration_attempts`, `feature_leases`,
`reconciliation_requests`, and `overlap_evidence` was empty when this route was wired. Treat
concurrency here as designed and unexercised, not as working — the `opencode` executor was in the
same position, built and tested, and three stacked defects appeared on its first real dispatch,
each reachable only after fixing the one before it.

**Setup for a feature batch:**

1. Create the parent branch in git. `lucind-ai feature create --id … --parent … --base-sha …`
   writes the ledger row; it does not create the branch.
2. Name `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha` on every packet in the
   batch, identically.
3. Dispatch with no target flags — the flags are the legacy path.


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
lucind-ai serve [--addr <addr>] [--approver <name>] [--approval-timeout <duration>]
lucind-ai feature create --id <id> --parent <ref> --base-sha <sha> [--expected-parent-sha <sha>]
lucind-ai feature status [--id <id>]
lucind-ai feature recover --attempt <id>
lucind-ai reconcile approve --request <id> --source <feature> --target <feature> [--actor <name>]
lucind-ai reconcile decline --request <id> [--actor <name>] [--reason <reason>]
lucind-ai reconcile cancel --request <id> [--actor <name>] [--reason <reason>]
lucind-ai reconcile renew --request <id> [--base-sha <sha>] [--source-sha <sha>] [--target-sha <sha>]
lucind-ai reconcile resolve --candidate <id> --sha <sha> [--actor <name>]
lucind-ai --version
```

This block mirrors the binary's own `usage` string (`cmd/lucind-ai/cli.go:48`). `feature recover` and every `reconcile` action need a row that only the unwired attempt path creates — see **Multi-feature orchestration** above before reaching for either.

### Subcommands

- `lucind-ai run`: Dispatch one or more packet lanes concurrently in isolated worktrees.
- `lucind-ai split`: Split an `apply-dag.yaml` sidecar into per-lane packets and print wave dispatch commands.
- `lucind-ai check`: Run repository checks once via `lucind-checks.sh` (`internal/integrate.Check`).
- `lucind-ai serve`: Start the HTTP API/web server for approvals and status monitoring (`--addr`).
- `lucind-ai feature create|status|recover`: Ledger-side feature records. `create` writes the `features` row from `--id`, `--parent`, `--base-sha` — it does **not** create the git branch. `status` reads features and integration attempts. `recover --attempt <id>` resumes an existing integration attempt.
- `lucind-ai reconcile approve|decline|cancel|renew --request <id>`: human decision on an overlap request between two feature parents. `approve` names `--source` and `--target` features and authorizes a candidate — it does not by itself clear a block; see **Clearing it takes two steps, not one** above. `renew` re-anchors a stale request to current SHAs. It does not reconcile ledger state against worktrees or git refs (`cmd/lucind-ai/cli.go:48`).
- `lucind-ai reconcile resolve --candidate <id> --sha <sha>`: registers a human-produced resolution commit against an approved candidate, marking it `integrated`. This is what actually clears a `required`-overlap block on the next retry of the blocked feature's own `lucind-ai run`.
- `lucind-ai renew`: an undocumented top-level alias for `reconcile renew` — same handler, same flags (`cmd/lucind-ai/cli.go:110`, `runReconcileRenew`). It does not renew lane leases; nothing on the CLI does. Prefer the `reconcile renew` spelling.

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

