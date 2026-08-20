# Design: Verify-Phase Dual Dispatch

Split `sdd-verify` into two decoupled steps: (1) a single deterministic mechanical-check execution (`lucind-checks.sh` executed via a new `lucind-ai check` command or `internal/integrate.Check`), and (2) two concurrent, independent read-only judgment packets (`agy` and `cursor-agent`), each producing a structured verdict envelope synthesized into a canonical verification report by the orchestrator. This eliminates redundant, non-deterministic test suite execution by LLMs while preserving dual-model qualitative judgment over specification satisfaction and code quality.

This design builds directly upon the merged `read-only-packet-dispatch` architecture (`Packet.ReadOnly`, `enforceCompletionMode`), adhering to the established dual-executor synthesis model used across `propose`, `design`, `specs`, and `tasks`.

## Recommendations at a glance

| # | Question | Recommendation | Source / Rationale |
|---|---|---|---|
| 1 | Where does mechanical check output live? | Executed once deterministically; output archived to `openspec/changes/<id>/verify-mechanical.txt` on the primary branch and embedded directly in each judgment packet's `## Context` block. | Guarantees immutable, identical check input across lanes without worktree isolation leakage or LLM re-execution. |
| 2 | What is the judgment packet shape and frontmatter? | `read_only: true` frontmatter (`Packet.ReadOnly = true`), zero unique commits, clean tree invariant per `read-only-packet-dispatch` Decision 2. | Reuses proven `enforceCompletionMode` runtime enforcement; prevents dummy commits. |
| 3 | How are independent verdicts reconciled? | Orchestrator synthesizes one canonical `openspec/changes/<id>/verify.md` report from both lane envelopes; substantive spec defects or hard stop blocks become blocking findings. | Matches session dual-dispatch precedent (`propose`/`design`/`specs`/`tasks`); prevents halting on cosmetic differences while preserving rigor. |
| 4 | What CLI/binary surface is introduced? | New `lucind-ai check` subcommand wrapping `internal/integrate.Check` to run `lucind-checks.sh` and output structured status. | Direct terminal consumer for mechanical execution; zero changes to `ExecuteBatch`. |
| 5 | How is rollback handled? | Purely additive CLI subcommand, packet template prose, and skill instructions. Reverting the apply commits cleanly restores the previous single-pass verify workflow. | Zero database/ledger schema migrations; full backward compatibility. |

---

## Decision 1 — Mechanical check execution and artifact capture

**Choice**: Mechanical verification checks (`lucind-checks.sh` wrapping build, unit tests, race detector, linter) are executed exactly **once** deterministically by the orchestrator using a dedicated CLI entry point (`lucind-ai check`). The resulting combined stdout/stderr and exit status are captured and handled via a dual-destination strategy:
1. **Archival artifact**: Saved to `openspec/changes/<change-id>/verify-mechanical.txt` on the primary repository branch before dispatch, establishing a durable, reviewable record in git.
2. **Packet context payload**: Embedded directly into the Markdown body of each judgment packet under `## Context` as a fenced block (````text ... ````), accompanied by explicit instructions forbidding executors from re-running build scripts or test suites.

```
+-------------------------------------------------------------------------+
| Primary Repository: `lucind-ai check`                                   |
| -> Executes `lucind-checks.sh` via `internal/integrate.Check`           |
| -> Captures exit code, duration, stdout/stderr                          |
+-------------------------------------------------------------------------+
                                    |
          +-------------------------+-------------------------+
          v                                                   v
+------------------------------------+  +------------------------------------+
| Archival Artifact:                 |  | Embedded Packet Context:           |
| `openspec/changes/<id>/`           |  | Authored into `verify-agy.md` and  |
| `verify-mechanical.txt`            |  | `verify-cursor-agent.md` context   |
+------------------------------------+  +------------------------------------+
```

### Why this structure is chosen

- **Worktree isolation compatibility**: `lucind-ai run` creates fresh, isolated linked worktrees (`../<repo>-worktrees/<id>`) branching from `primaryRoot`'s `HEAD`. Untracked files in the primary working tree or gitignored `.lucind/` directories are intentionally not copied to linked worktrees. Embedding the mechanical output in the packet file itself ensures 100% self-containment: when the agent reads its prompt, all test logs and failure traces are immediately present in context without filesystem traversal.
- **Deterministic reproducibility**: Automated test suites are deterministic. Executing them twice through two separate LLM subshells wastes execution quota, introduces variance from non-deterministic test timings or port collisions, and risks divergent outcomes based on agent environment differences.
- **Strict separation of concerns**: Mechanical checks answer "did the code compile and do automated tests pass?"; judgment packets answer "does the implementation faithfully fulfill the specification requirements and maintain architectural integrity?".

### Rejected alternatives

- **Rejected: Letting judgment executors run `lucind-checks.sh` independently inside their own worktrees.**
  *Reason*: Directly violates the core mandate of `SKILL.md:80`. LLM execution of shell test runners is non-deterministic, consumes unnecessary compute quota, and risks false negatives if agents alter environment flags.
- **Rejected: Storing mechanical check output exclusively in primary `.lucind/` (e.g., `.lucind/verify-mechanical.log`) without embedding in the packet.**
  *Reason*: `.lucind/` is gitignored. Linked worktrees created by `git worktree add` do not share `.lucind/` contents with the primary root (each lane gets its own isolated `.lucind/` containing only `result.schema.json` and `result.json`). Reading from primary's `.lucind/` would require worktree escape paths or complex cross-worktree filesystem coupling.
- **Rejected: Dynamic runtime injection of check logs into lane worktrees during `ExecuteBatch`.**
  *Reason*: Adding ad-hoc file-copying logic inside `internal/run/run.go` to inject files into newly spawned worktrees couples the generic `lucind-ai run` runner with verify-phase domain logic. `run.ExecuteBatch` must remain a general-purpose packet orchestrator.

---

## Decision 2 — Judgment packet schema, frontmatter, and read-only criteria

**Choice**: Judgment packets use the `read_only: true` frontmatter introduced and merged in `read-only-packet-dispatch/design.md`.

### Exact Frontmatter

```yaml
---
id: verify-<executor>
executor: <agy | cursor-agent>
routed_by: independent qualitative verification of change specifications
model: gemini-3.7-flash-high   # optional for agy
read_only: true
---
```

### Exact Definition of "Done" for Read-Only Judgment Lanes

Because a judgment packet is strictly read-only, it does not produce code commits. In accordance with `read-only-packet-dispatch/design.md` Decision 2, mandatory done-criterion 2 is restated as follows:

> **The worktree carries no unique commits and no working-tree changes relative to the lane's birth point.** Evidence: `git status --porcelain` empty **and** the worktree's `HEAD` equals `git merge-base HEAD <primary HEAD>`.

### Result Envelope Contract

Each judgment executor writes its findings to `.lucind/result.json` in its respective worktree:
- `packet_id`: Echoes `verify-agy` or `verify-cursor-agent`.
- `status`:
  - `done`: All spec requirements verified and satisfied with concrete `file:line` proof; no regressions or unhandled edge cases found.
  - `deviated`: Implementation fulfills requirements but contains minor deviations or non-blocking technical debt (recorded in `deviations`).
  - `blocked`: Hard stop fired, severe spec violation discovered, or missing requirement identified (recorded in `questions` and `hard_stops`).
  - `failed`: Agent encountered unrecoverable technical error during analysis.
- `done_criteria`: Objective verification of each spec item with file:line citations.
- `findings`: Qualitative observations, edge-case coverage gaps, or architectural insights.
- `hard_stops`: Mandatory evaluation of all declared hard stops.
- `commit`: Omitted (per `read-only-packet-dispatch` envelope schema).

Runtime enforcement is performed automatically by `run.enforceCompletionMode` (`internal/run/run.go:340-360`): when envelope status is `lane.Done`, the runtime verifies that the lane branch has no unique commits (`HasUniqueLaneCommits == false`) and a clean working tree (`PorcelainEmpty == true`).

### Rejected alternatives

- **Rejected: Requiring a dummy or metadata commit in judgment lanes to satisfy standard write-packet criterion 2.**
  *Reason*: Pollutes git history with synthetic commits that must be filtered or discarded at integrate time. `read-only-packet-dispatch` specifically solved this problem.
- **Rejected: Creating a distinct `VerdictPacket` YAML schema separate from `Packet`.**
  *Reason*: Unnecessary schema fragmentation. `Packet` with `ReadOnly: true` already expresses all required metadata for execution, barrier synchronization, and result capture.

---

## Decision 3 — Verdict reconciliation and canonical report synthesis

**Choice**: The orchestrator (Claude Code) reconciles the two independent verdict envelopes and synthesizes one canonical verification report: `openspec/changes/<change-id>/verify.md`.

```
+--------------------------+          +----------------------------------+
| verify-agy               |          | verify-cursor-agent              |
| (.lucind/result.json)    |          | (.lucind/result.json)            |
+--------------------------+          +----------------------------------+
             \                                      /
              \                                    /
               v                                  v
        +------------------------------------------------+
        | Orchestrator Synthesis (Claude Code)           |
        | - Compares verdicts & spec coverage maps       |
        | - Validates cited file:line evidence           |
        | - Resolves false positives vs real gaps        |
        +------------------------------------------------+
                                |
                                v
        +------------------------------------------------+
        | Canonical Report:                              |
        | `openspec/changes/<id>/verify.md`              |
        | - Consensus status (PASSED / BLOCKED)          |
        | - Comprehensive spec compliance matrix        |
        | - Consolidated findings & audit trail          |
        +------------------------------------------------+
```

### Reconciliation Logic

1. **Unanimous Approval (`done` / `done`)**:
   - Orchestrator synthesizes `openspec/changes/<id>/verify.md`, combining the verified spec matrix and complementary findings from both drafts.
   - Verification status is marked `PASSED`. The change is ready for archival (`openspec archive`).
2. **Disagreement on Defects (`done` vs `blocked`, `done` vs `deviated`, or `blocked` / `blocked`)**:
   - The orchestrator independently verifies the disputed finding against the codebase and spec requirements.
   - If the finding represents a **genuine spec violation or gap**, the overall verification outcome is marked `BLOCKED`. A blocking finding is added to `verify.md` detailing the required remediation tasks.
   - If the finding is a **demonstrable false positive** (e.g., an executor misunderstood a helper function), the orchestrator documents the refutation in `verify.md` with exact `file:line` proof and marks the item resolved.
3. **Execution Failure (`failed` on either lane)**:
   - The lane is evaluated for transient execution issues (e.g., timeout, context length exhaustion). The orchestrator may re-dispatch the single failing lane or investigate the failure cause.

### Rejected alternatives

- **Rejected: Automatic binary-level verdict merging without orchestrator synthesis.**
  *Reason*: Two independent LLMs will always vary in phrasing, depth of explanation, and finding prioritization. A mechanical binary cannot judge whether a qualitative observation is a blocking defect, a benign suggestion, or a hallucinated false positive. Synthesis belongs in the orchestrator layer, matching `propose`, `design`, `specs`, and `tasks`.
- **Rejected: Hard-blocking unconditionally on any textual or verdict divergence without synthesis.**
  *Reason*: Destroys workflow efficiency. Minor differences in qualitative style or false alarms would halt every verify phase, forcing human intervention even when code review against the spec shows clear compliance.

---

## Decision 4 — Terminal consumers and indirection traceability

In accordance with Mandatory Done Criterion 1, every new type, CLI flag, and function introduced by this design has an explicit terminal consumer:

| Indirection / Component | Location | Terminal Consumer | Purpose |
|---|---|---|---|
| Subcommand `lucind-ai check` | `cmd/lucind-ai/cli.go` (`runCheck`) | Orchestrator invoking mechanical suite during `sdd-verify` step 1 | Runs `lucind-checks.sh` deterministically and prints structured results to stdout/file. |
| Function `integrate.Check` | `internal/integrate/integrate.go` | Called by `runCheck` in `cmd/lucind-ai/cli.go` and `lucindrun.Integrate` | Executes the root check script and captures `(passed bool, output string, err error)`. |
| Flag `read_only: true` | `openspec/changes/<id>/packets/verify-*.md` | `packet.Parse` in `internal/packet/packet.go`, consumed by `run.enforceCompletionMode` | Enforces zero commits and clean tree invariant upon lane completion. |
| Archival File `verify-mechanical.txt` | `openspec/changes/<id>/verify-mechanical.txt` | Read by orchestrator and human auditor | Durable repository record of mechanical test run. |
| Result Envelopes | `../<repo>-worktrees/verify-*/.lucind/result.json` | Read by orchestrator during `sdd-verify` step 3 | Feeds raw verdict data into canonical `verify.md` synthesis. |
| Canonical Report `verify.md` | `openspec/changes/<id>/verify.md` | Human reviewer and `openspec archive` | Final gate for change completion. |

---

## Decision 5 — Rollback

**Choice**: Rollback is purely additive and requires zero database migrations or state repair.

- **Ledger Compatibility**: No changes are made to `internal/ledger` SQLite schemas or event types. `read_only` lanes already record standard `lane_registered`, `status_changed`, and `lane_note` events.
- **Binary Reversion**: Reverting the apply commit(s) touching `cmd/lucind-ai/cli.go` and documentation removes the `lucind-ai check` subcommand without impacting existing `lucind-ai run` behavior.
- **Workflow Fallback**: If verify dual-dispatch is reverted, the orchestrator immediately falls back to running `lucind-checks.sh` manually and reviewing code via local inspect tools, exactly as prior to this change.

---

## Data flow

```
[ sdd-verify Phase Initiated ]
            |
            v
1. Orchestrator executes mechanical checks:
   $ lucind-ai check > openspec/changes/<id>/verify-mechanical.txt
            |
            +--> Checks FAIL -> Verification HALTS immediately.
            |                   Remediate mechanical failures first.
            v (Checks PASS)
2. Orchestrator generates dual read-only judgment packets:
   - openspec/changes/<id>/packets/verify-agy.md
   - openspec/changes/<id>/packets/verify-cursor-agent.md
   (Both contain `read_only: true` and embed verify-mechanical.txt in ## Context)
            |
            v
3. Orchestrator dispatches parallel judgment lanes:
   $ lucind-ai run --packet packets/verify-agy.md --packet packets/verify-cursor-agent.md
            |
            |--> worktree.Create (verify-agy, verify-cursor-agent)
            |--> executor.Run (concurrent agy & cursor-agent)
            |--> decideStatus -> enforceCompletionMode (verify 0 commits, clean tree)
            |--> barrier join (Released: true)
            v
4. Orchestrator reads preserved result envelopes:
   - ../<repo>-worktrees/verify-agy/.lucind/result.json
   - ../<repo>-worktrees/verify-cursor-agent/.lucind/result.json
            |
            v
5. Orchestrator synthesizes canonical verification report:
   - openspec/changes/<id>/verify.md
   (If unresolvable defects found -> mark BLOCKED; if specs satisfied -> mark PASSED)
            |
            v
[ Phase Complete / Ready for Archive ]
```

---

## File changes (apply phase — not this design document)

| File | Action | Rationale |
|---|---|---|
| `cmd/lucind-ai/cli.go` | Modify | Add `check` subcommand handler calling `integrate.Check`; print pass/fail status and output. |
| `cmd/lucind-ai/cli_test.go` | Modify | Unit test for `lucind-ai check` with passing, failing, and missing `lucind-checks.sh`. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modify | Update row 80 in target-direction table; add detailed `sdd-verify` mechanical-run + dual-dispatch instructions. |
| `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` | Create | Standardized template for verify judgment packets with `read_only: true` and spec evaluation sections. |

---

## Testing strategy

| Layer | RED Test Case | GREEN Test Case |
|---|---|---|
| CLI `lucind-ai check` | Running in a repo without `lucind-checks.sh` exits 1 with explanatory error; failing script exits 1. | Running with valid, succeeding `lucind-checks.sh` prints output and exits 0. |
| Judgment Packet Parsing | Non-boolean `read_only` frontmatter fails `packet.Parse`. | `read_only: true` parses cleanly to `Packet.ReadOnly = true`. |
| Runtime Verification | Judgment lane attempting a commit or leaving dirty files is demoted to `lane.Failed` by `enforceCompletionMode`. | Clean judgment lane with zero commits and valid envelope achieves `lane.Done`. |
| Orchestrator Verification | Incomplete spec coverage or unverified criteria triggers blocking review. | Full spec coverage matrix with valid evidence compiles into `verify.md`. |

---

## Threat matrix

| Boundary | Applicability | Mitigation |
|---|---|---|
| Non-deterministic mechanical checks | Applicable | Mechanical suite is run exactly once deterministically by CLI, never re-run by judgment LLMs. |
| Accidental history mutation by judgment lanes | Applicable | `read_only: true` invariant enforced by `enforceCompletionMode` via `git merge-base` equality and clean porcelain. |
| Hallucinated compliance in judgment envelopes | Applicable | Orchestrator synthesizes canonical report by independently verifying cited `file:line` proof against codebase. |
| False positive blocker escalation | Applicable | Orchestrator acts as judgment filter, documenting refutations for invalid findings with concrete code evidence. |
| Cross-worktree state leakage | Applicable | Packet context is self-contained; worktrees operate on independent branches without sharing `.lucind/`. |

---

## Out of scope

- Modifications to `internal/integrate/integrate.go` check execution logic (reused as-is).
- Changes to `internal/barrier` or `internal/ledger` schemas.
- Interactive web UI visualization for verification reports (handled by future UI iterations).
- Automated remediation loops for failed verification (remediation tasks are authored in separate apply packets).
