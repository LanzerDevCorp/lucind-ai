# Design: Verify-Phase Dual Dispatch

Split `sdd-verify` into (1) a single deterministic mechanical-check execution (`lucind-checks.sh`
via a new `lucind-ai check` CLI subcommand wrapping `internal/integrate.Check`) and (2) two
parallel, independent read-only judgment packets (`agy` + `cursor-agent`) evaluating spec
conformance, edge cases, and test quality against the frozen mechanical output. The orchestrator
synthesizes the two verdict envelopes into one canonical `verify.md` report, matching the
dual-dispatch synthesis pattern already used for propose/design/specs/tasks.

This design builds directly on the merged `read-only-packet-dispatch` architecture
(`Packet.ReadOnly`, `enforceCompletionMode`) without redesigning packet completion invariants.

Both independently-drafted designs (agy, cursor-agent) converged far more heavily here than on
the sibling `apply-dag-dispatch` design — no real technical bug was caught between them, only
naming and level-of-detail differences. Where they diverged, this document picks one and records
why.

## Recommendations at a glance

| # | Question | Recommendation | Source |
|---|---|---|---|
| 1 | Where does the mechanical check run and live? | Run once via new CLI subcommand `lucind-ai check` (wrapping `internal/integrate.Check`). Output captured to `openspec/changes/<id>/verify-mechanical.log`, committed to the candidate branch before dispatch, and embedded in each judgment packet's `## Context`. | Both drafts agreed on the mechanism; canonical adopts cursor-agent's `.log` naming and field set (command line, exit code, duration, git SHA). |
| 2 | Judgment packet shape and done-criteria? | `read_only: true` frontmatter; criterion 2 restated exactly per `read-only-packet-dispatch/design.md` Decision 2 (no unique commits, clean porcelain). Verdict returns via existing `result.schema.json` fields — no new envelope properties. | Strong agreement; canonical adopts cursor-agent's explicit "no schema churn" decision. |
| 3 | How is mechanical re-running forbidden? | Explicit `## Out of scope` + named hard stop in the packet template, backed structurally by `enforceCompletionMode`'s porcelain check (test artifacts left behind fail the lane). | cursor-agent's Decision 3 treatment adopted verbatim — more concrete than agy's inline mention. |
| 4 | How are dual verdicts reconciled? | Orchestrator synthesizes one canonical `verify.md`; confirmed defects block, false positives are refuted with evidence, genuine irreconcilable ambiguity escalates to the human. | Canonical combines agy's 3-case reconciliation table with cursor-agent's explicit human-escalation step. |
| 5 | Rollback? | Purely additive: new CLI subcommand, template asset, `SKILL.md` update. Zero ledger/schema migrations. | Both agreed. |

---

## Decision 1 — Mechanical check execution and artifact capture

**Choice**: Mechanical checks (`lucind-checks.sh` — build, unit tests, race detector, linter) run
exactly **once**, deterministically, via a new CLI subcommand `lucind-ai check [--out <path>]`
wrapping `internal/integrate.Check(ctx, targetPath)` (`internal/integrate/integrate.go:79`). The
combined stdout/stderr, exit status, duration, and candidate git SHA are captured to:

- **Archival artifact**: `openspec/changes/<change-id>/verify-mechanical.log`, committed to the
  candidate branch **before** judgment packets are dispatched. Because `lucind-ai run` creates
  linked worktrees from the candidate branch's `HEAD`, committing the log first is what makes it
  present inside each judgment worktree without any special file-injection machinery.
- **Packet context payload**: the log's summary (and, for failures, the full transcript) is also
  embedded directly into each judgment packet's `## Context` section, so the executor has the
  result immediately in-prompt without an extra file read.

```
Candidate Branch / HEAD -> lucind-ai check -> openspec/changes/<id>/verify-mechanical.log
                                            -> committed to candidate branch
                                            -> embedded into packets/verify-*.md ## Context
```

### Why this structure

- **Deterministic reproducibility**: test suites are deterministic; running them twice through
  two LLM subshells wastes quota and risks divergent outcomes from timing/port-collision
  flakiness, with zero qualitative benefit.
- **Worktree isolation compatibility**: `.lucind/` is gitignored and not shared across linked
  worktrees, so a primary-root-only artifact would be invisible to a judgment lane. Committing
  the log to the candidate branch is what makes it reachable through ordinary worktree
  inheritance — no cross-worktree filesystem coupling required.
- **Strict separation of concerns**: mechanical checks answer "did it compile and pass tests";
  judgment packets answer "does it faithfully satisfy the spec's intent."

### Rejected alternatives

- **Letting judgment executors run `lucind-checks.sh` themselves.** Directly defeats the point of
  this change — non-deterministic, quota-burning, and the exact anti-pattern `SKILL.md:80` exists
  to eliminate.
- **Storing the log only in primary `.lucind/`.** `.lucind/` is gitignored and each linked
  worktree gets its own empty one; a judgment lane could never read a primary-only file without
  worktree-escape logic.
- **Writing output only to `/tmp` or stdout.** Volatile, no durable audit trail, violates
  repository-worktree locality already established elsewhere in this project.
- **Injecting the log into worktrees at `ExecuteBatch` time.** Would couple the generic
  `lucind-ai run` runner to verify-phase domain logic; `ExecuteBatch` must stay a general-purpose
  packet orchestrator.

---

## Decision 2 — Judgment packet frontmatter, shape, and read-only done-criteria

**Choice**: judgment packets set `read_only: true`, consuming the mechanism merged in
`read-only-packet-dispatch/design.md`.

```yaml
---
id: verify-<change-id>-<executor>
executor: agy   # or cursor-agent
routed_by: qualitative verification of spec compliance, edge cases, and test quality
model: gemini-3.7-flash-high   # or the cursor-agent equivalent
read_only: true
---
```

Done-criteria set:

1. **Every indirection introduced is demonstrably consumed by a terminal consumer** — verification
   citations trace to concrete symbols, tests, and spec requirements.
2. **The worktree carries no unique commits and no working-tree changes relative to the lane's
   birth point.** Evidence: `git status --porcelain` empty **and** `HEAD` equals
   `git merge-base HEAD <primary HEAD>` — restated exactly from
   `read-only-packet-dispatch/design.md` Decision 2, not reinvented.
3. **Qualitative evaluation completed** — `.lucind/result.json` populated with `status`,
   `summary`, and structured `findings`.

**Verdict envelope**: reuses the existing `result.schema.json` shape as-is — `status`, `summary`,
`findings` (`finding`/`evidence`/`affects`), `hard_stops`, `done_criteria`. `commit` is omitted,
per the read-only envelope convention. No new envelope properties are added.

```json
{
  "packet_id": "verify-<change-id>-cursor-agent",
  "status": "done",
  "summary": "VERDICT: PASS. Implementation satisfies all spec requirements in specs/.../spec.md. Mechanical checks passed cleanly.",
  "hard_stops": [
    {"hard_stop": "Executing mechanical test suites or build commands when results are already provided.", "fired": false}
  ],
  "findings": [
    {"finding": "Missing negative test case for malformed input", "evidence": "internal/x/x_test.go:142", "affects": "Non-blocking coverage gap"}
  ],
  "done_criteria": [
    {"criterion": "no unique commits, clean tree", "met": true, "evidence": "git status --porcelain empty; HEAD == merge-base"}
  ]
}
```

Runtime enforcement is unchanged — `run.enforceCompletionMode` verifies `HasUniqueLaneCommits ==
false` and `PorcelainEmpty == true` on `Done`, exactly as designed for any read-only lane.

### Rejected alternatives

- **Requiring a dummy/metadata commit to satisfy the standard write-packet criterion 2.** Pollutes
  git history with synthetic commits; `read-only-packet-dispatch` exists specifically to avoid
  this.
- **A distinct `VerdictPacket` YAML schema separate from `Packet`.** Unnecessary fragmentation —
  `Packet` with `ReadOnly: true` already expresses everything needed.
- **Adding new custom verdict fields to `result.schema.json`.** The existing schema already
  provides everything a verdict needs; extending it here would be schema churn for no new
  capability, and would diverge the envelope contract per-phase instead of keeping it universal.

---

## Decision 3 — Mechanical re-run prohibition and enforcement

**Risk**: a judgment lane's worktree is a full checkout of the candidate implementation. If the
executor runs `go test ./...`, `go vet`, or `lucind-checks.sh` "to double-check," it silently
reintroduces the exact duplicate-execution cost and flakiness this change exists to eliminate.

**Choice**: prohibit mechanical re-runs through the packet's prompt contract, backed structurally
by git porcelain cleanliness enforcement.

1. **Prompt & hard-stop contract** — the verify judgment packet template states explicitly:

   ```markdown
   ## Out of scope
   Do NOT execute `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build
   suite. Deterministic mechanical checks have already run once; their frozen output is in
   `## Context`. Re-running them wastes quota and adds no new signal.

   ## Hard stops
   - Executing mechanical test/build commands when mechanical results are already provided.
   ```

2. **Structural enforcement via `enforceCompletionMode`** — Go/test toolchains routinely leave
   untracked artifacts (`coverage.out`, compiled test binaries, temp fixtures). Any such leftover
   fails `PorcelainEmpty`, demoting the lane to `Failed` regardless of what the prompt said.

3. **Tool-selection guidance** — the packet instructs executors toward read/navigation tools
   (`Read`, `Glob`, `Grep`, `codegraph`) and read-only git queries, not build/test execution.

### Rejected alternatives

- **`chmod -x` or deleting `lucind-checks.sh` in the worktree.** Creates a working-tree diff,
  which would itself fail the porcelain check it's trying to protect — self-defeating.
- **Disabling shell/tool access entirely.** Executors legitimately need shell access for
  read-only git queries and normal tool operation; over-restricting breaks the lane, not just the
  bad behavior.

---

## Decision 4 — Verdict reconciliation and canonical report synthesis

**Choice**: the orchestrator (Claude Code) synthesizes one canonical verification report,
`openspec/changes/<change-id>/verify.md`, from the two independent envelopes — matching the
existing propose/design/specs/tasks synthesis convention rather than inventing a new one.

```
[ verify-mechanical.log ]
          +
[ agy result.json ]        -> Orchestrator synthesis -> openspec/changes/<id>/verify.md
          +                                                    |
[ cursor-agent result.json ]                                   v
                                                     PASS -> state.yaml: verify done
                                                     BLOCKED -> state.yaml: corrective tasks
```

### Reconciliation logic

1. **Unanimous approval** (`done` / `done`): synthesize `verify.md` combining both spec-compliance
   matrices and complementary findings; overall status `PASSED`, change ready for archive.
2. **Disagreement** (`done` vs. `blocked`/`deviated`, or `blocked`/`blocked`): the orchestrator
   independently verifies the disputed finding against the codebase and spec, per
   `SKILL.md:102` ("green criteria are not proof of complete work"). A genuine spec violation
   marks the overall outcome `BLOCKED` with a remediation-ready finding; a demonstrable false
   positive is refuted in `verify.md` with exact `file:line` evidence and marked resolved.
3. **Execution failure** (`failed` on either lane): evaluated for a transient cause (timeout,
   context exhaustion); the orchestrator may re-dispatch the single failing lane.
4. **Genuine ambiguity escalation**: if the two executors interpret an underspecified requirement
   in contradictory ways and the orchestrator cannot resolve it from existing specs/design, the
   overall status is set `blocked` and the decision is presented to the human — this is not the
   same case as (2), where the orchestrator *can* adjudicate against the codebase.

### Rejected alternatives

- **Automatic binary-level verdict merging without orchestrator synthesis.** Two independent LLMs
  vary in phrasing, depth, and prioritization; a mechanical merge cannot tell a blocking defect
  from a benign suggestion or a hallucinated finding.
- **Hard-blocking unconditionally on any divergence.** Destroys workflow efficiency — dual
  reviewers routinely produce complementary, non-contradictory observations (one spots a
  concurrency edge case, the other an off-by-one); treating every difference as fatal forces
  human intervention even when the codebase clearly shows compliance.

---

## Decision 5 — Rollback

**Choice**: purely additive; zero database or ledger migrations.

| Layer | Rollback behavior |
|---|---|
| `lucind-ai check` CLI command | Revert the subcommand in `cmd/lucind-ai/cli.go`. |
| Packet templates | Revert additions in `plugin/claude-code/skills/lucind-ai/assets/`. |
| Skill documentation | Revert `SKILL.md` verify-row and workflow instructions. |
| Ledger / SQLite | Zero impact — no schema changes, no new event types. |

Reverting the apply commit(s) fully restores today's manual verification workflow; existing
packets and historical runs are unaffected.

---

## Terminal consumers and indirection trace

| Introduced symbol / asset | Location | Terminal consumer | Purpose |
|---|---|---|---|
| `lucind-ai check` CLI command | `cmd/lucind-ai/cli.go` | Orchestrator invoking mechanical suite during `sdd-verify` step 1 | Runs `lucind-checks.sh` deterministically, writes structured results. |
| `integrate.Check` (existing) | `internal/integrate/integrate.go` | Called by the new `check` subcommand | Executes the check script, returns `(passed bool, output string, err error)`. |
| `verify-mechanical.log` | `openspec/changes/<id>/verify-mechanical.log` | Read by orchestrator (packet Context) and human auditor | Durable, committed record of the one mechanical run. |
| `read_only: true` (existing field) | Judgment packet frontmatter | `packet.Parse`, then `run.enforceCompletionMode` | Enforces zero commits / clean tree for judgment lanes. |
| Judgment envelope `findings` | `.lucind/result.json` in each worktree | Orchestrator during synthesis | Feeds the consolidated `verify.md` report. |
| Canonical `verify.md` | `openspec/changes/<id>/verify.md` | Human reviewer, `openspec archive`, `state.yaml` update | Final gate for the verified change. |

---

## Data flow

```
1. PREPARATION
   Candidate branch HEAD
         |
         v
   lucind-ai check -> internal/integrate.Check(ctx, candidatePath)
         |
         +--> Checks FAIL -> verify HALTS; remediate mechanical failures first.
         |
         v (checks PASS)
   Commit openspec/changes/<id>/verify-mechanical.log to candidate branch

2. DUAL PACKET DISPATCH
   Orchestrator authors packets/verify-<id>-agy.md, packets/verify-<id>-cursor-agent.md
   (both read_only: true, mechanical log embedded in ## Context)
         |
         v
   lucind-ai run --packet ... --packet ...
         |
         +--> worktree.Create (forked from candidate HEAD, log already committed)
         +--> executor.Run (concurrent agy & cursor-agent)
         +--> decideStatus -> enforceCompletionMode (0 commits, clean tree)
         +--> barrier join

3. RECONCILIATION & SYNTHESIS
   Orchestrator reads both .lucind/result.json envelopes
   Cross-checks cited evidence against the codebase
   Synthesizes openspec/changes/<id>/verify.md
         |
         +--> PASSED  -> state.yaml: verify done  -> ready for archive
         +--> BLOCKED -> state.yaml: verify blocked -> corrective tasks queued
```

---

## File changes (apply phase — not this design document)

| File | Action |
|---|---|
| `cmd/lucind-ai/cli.go` | Add `check` subcommand invoking `integrate.Check`; print/write pass-fail status and captured output. |
| `cmd/lucind-ai/cli_test.go` | Cases: missing `lucind-checks.sh`, failing script, passing script, `--out` file write. |
| `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` | Create: standardized judgment-packet template (`read_only: true`, mechanical-rerun hard stop, spec-evaluation prompt sections). |
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | Add a pointer note to the new verify template for qualitative verification lanes. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Update row 80 from target-direction to the operational two-stage verify protocol; add `sdd-verify` dispatch instructions. |

`internal/run/integrate.go`, `internal/integrate/integrate.go` (beyond being called, not
modified), and `internal/ledger/` receive **no changes** — `integrate.Check` and
`enforceCompletionMode` are reused unmodified.

## Testing strategy

| Layer | RED | GREEN |
|---|---|---|
| `lucind-ai check` CLI | Missing `lucind-checks.sh`, or a failing script, exits non-zero with explanatory output. | Passing script exits 0, prints/writes timing + summary; `--out` writes the log file. |
| Judgment packet parsing | Non-boolean `read_only` rejected (already covered by `read-only-packet-dispatch`'s tests — no new parser test needed here). | `read_only: true` round-trips to `Packet.ReadOnly = true`. |
| `enforceCompletionMode` on verify lanes | A lane that commits or leaves dirty test artifacts is demoted to `lane.Failed`. | A clean judgment lane with zero commits and a valid envelope reaches `lane.Done`. |
| Orchestrator synthesis | Discrepant findings between the two envelopes are flagged during synthesis, not silently dropped. | Full spec coverage with valid evidence from both lanes compiles cleanly into `verify.md`. |

## Threat matrix

| Boundary | Applicability | Mitigation |
|---|---|---|
| Duplicate mechanical execution | Applicable | Prompt contract + hard stop prohibit re-running checks; `enforceCompletionMode` rejects any leftover build/test artifact via the porcelain check. |
| Flaky test contamination | Applicable | Mechanical suite runs exactly once in a controlled environment; both judges evaluate the identical frozen transcript. |
| State mutation in read-only lanes | Applicable | `HasUniqueLaneCommits == false` and `PorcelainEmpty == true`, enforced by the already-merged `enforceCompletionMode`. |
| Hallucinated review findings | Applicable | Orchestrator independently cross-checks every cited `file:line` against the real codebase before accepting a finding into `verify.md`. |
| Silent failure concealment | Applicable | Every declared hard stop must appear in the envelope's `hard_stops` array, whether or not it fired. |
| Ledger schema corruption | N/A | Zero SQLite schema modifications; standard lane/barrier event logging is preserved unchanged. |

## Out of scope (owned by sibling changes or deferred)

`read-only-packet-dispatch` owns `Packet.ReadOnly`, `enforceCompletionMode`, and the git
inspection helpers — this design consumes them, does not reimplement them. `apply-dag-dispatch`
owns `AllowedPaths` and DAG wave scheduling — unrelated to verify. Automated remediation of
verify findings is out of scope; a `BLOCKED` verify outcome queues follow-up apply tasks, it does
not auto-fix them. Integrating reviewers beyond `agy`/`cursor-agent` is deferred.
