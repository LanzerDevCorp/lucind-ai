# Proposal: Verify-Phase Dual Dispatch

Split `lucind-ai`'s `verify` SDD phase into two decoupled operations: a single deterministic
mechanical check run once by tooling/orchestrator, and a dual-dispatched qualitative judgment
run in parallel across `agy` and `cursor-agent` as independent read-only packets. Each executor
inspects the implementation diff against the accepted specs/design and the frozen mechanical
check output, producing an independent verdict without mutating repository code.

**This turns the final row of `plugin/claude-code/skills/lucind-ai/SKILL.md:80` from a blocked
target into an operational workflow, building directly on the read-only packet foundation
established by `read-only-packet-dispatch`.**

## Intent

In the SDD lifecycle, `verify` proves that an implemented change satisfies its specifications,
passes tests, and introduces no unintended regressions before merging or archiving. Today
`verify` runs as a local Claude Code subagent session or manual inspection, where a single model
both executes test commands and evaluates its own work.

`SKILL.md:80` defines the target direction: mechanical checks run exactly once deterministically,
while qualitative judgment is dispatched concurrently to two distinct executor subscriptions
(`agy` and `cursor-agent`). Mechanical verification (`go test`, `go vet`, `lucind-checks.sh`,
build scripts) is strictly deterministic — running compiler and test suites twice through two
LLMs in separate worktrees yields identical stdout/stderr while doubling quota consumption and
wall-clock time, and introduces potential LLM interpretation errors or flakiness. Qualitative
judgment (spec adherence, edge-case coverage, architectural coherence, under-specified behavior)
is exactly the opposite: non-deterministic and benefiting from dual-executor cross-checking
across different model families.

## Current behavior vs. target behavior

| Dimension | Current behavior (verified in codebase) | Target behavior (this proposal) | Evidence / Reference |
|---|---|---|---|
| **Execution model** | Run locally by Claude Code subagent or manual conversation; single-model self-evaluation. | Split: mechanical check run once by tooling/orchestrator; qualitative judgment dispatched to `agy` and `cursor-agent` in parallel. | `SKILL.md:80` |
| **Mechanical checks** | Invoked inside a Claude session or lane prompt; results subject to prompt context limits and LLM interpretation. | Executed once by host tooling/orchestrator (`lucind-checks.sh` / `internal/integrate.Check`); output frozen as an immutable artifact. | `internal/integrate/integrate.go:79` |
| **Qualitative review** | Single-model perspective; potential bias toward its own implementation or task decomposition. | Independent dual-executor review (`agy` + `cursor-agent`) evaluating the exact same frozen diff and check artifact. | `SKILL.md:42-72` |
| **Lane write & git invariants** | Default packets require commits (`packet-template.md:40-41`); verification without changes forces artificial commits or fails write invariants. | Judgment packets declare `read_only: true`; `enforceCompletionMode` verifies zero unique commits and clean porcelain tree. | `openspec/changes/read-only-packet-dispatch/design.md` Decisions 2-3 |
| **Scope boundary** | Verification runs risk scope creep — modifying code directly to fix defects rather than reporting verdicts. | Read-only enforcement structurally prevents lanes from modifying code; judgment packets produce review verdicts only. | `read-only-packet-dispatch/design.md` Decision 3 |
| **Artifact output** | Inline transcript notes or ad-hoc test output. | Deterministic mechanical check log + dual independent verdict envelopes synthesized by the orchestrator into one canonical report. | `SKILL.md:53-59` |
| **SDD status** | `SKILL.md:80` lists `verify` as "Target direction, not yet built." | `SKILL.md:80` moves from blocked target to a built orchestrator dispatch procedure. | `SKILL.md:74-81` |

## Scope

### What changes

- **Two-stage verify execution model**:
  1. *Mechanical check (run once)*: the orchestrator executes the project's verification suite
     (`lucind-checks.sh` / `internal/integrate.Check`) against the candidate worktree/branch
     exactly once, capturing stdout, stderr, and exit status into a fixed artifact.
  2. *Qualitative judgment (dispatched twice)*: the orchestrator constructs two read-only
     judgment packets, each supplying the diff, change specs/design, and the frozen mechanical
     artifact. Both are dispatched in parallel via `lucind-ai run --packet ... --packet ...`.
- **Consumption of `read-only-packet-dispatch`**: judgment packets set `read_only: true` in
  frontmatter. Terminal consumer: `internal/run.enforceCompletionMode` verifies zero unique
  commits and a clean working tree for both lanes. Read-only is mandatory because judgment
  packets produce review verdicts, not code changes — requiring a commit would force either a
  fabricated no-op commit or scope creep into fixing discovered defects inline.
- **Verification packet authoring convention**: a standardized prompt contract instructing
  executors to evaluate spec compliance, untested edge cases, regression risk, and criteria
  fulfillment against the frozen diff and mechanical artifact — and explicitly forbidding
  re-execution of mechanical checks inside the lane.
- **Orchestrator synthesis & recording**: the orchestrator reads both judgment verdicts,
  cross-checks cited evidence against the codebase, and records a synthesized verification
  outcome in the change's `state.yaml` and engram — matching the dual-dispatch synthesis pattern
  already used for propose/design/specs/tasks.
- **Documentation update**: `SKILL.md:80` moves from "Target direction, not yet built" to the
  operational two-stage verify workflow.

### What stays untouched

- `ExecuteBatch`, worktree isolation, envelope validation, and the SQLite ledger operate without
  verify-specific Go modifications beyond a new mechanical-check CLI entry point (design's call).
- Read-only lane enforcement relies entirely on `Packet.ReadOnly` / `enforceCompletionMode` from
  `read-only-packet-dispatch` — not re-derived or duplicated here.
- Mechanical check execution reuses `internal/integrate.Check` / `lucind-checks.sh` unmodified.
- `Combine`, `Resolve`, and bisection (`internal/integrate/integrate.go`, `internal/resolve`)
  are untouched.
- The write-phase dual-dispatch pattern for propose/design/specs/tasks is unchanged.
- `apply-dag-dispatch` (`AllowedPaths`, wave sequencing) and `approvals-web-ui` remain completely
  independent.

## Non-goals

- No re-running mechanical checks inside LLM lanes — they receive the captured output only.
- No code mutation during verification — judgment packets are strictly read-only; remediation of
  findings belongs to a separate `apply` cycle or human decision.
- No re-implementation of read-only packet mechanics — `read_only`, `Packet.ReadOnly`, and
  `enforceCompletionMode` are owned by `read-only-packet-dispatch`.
- No DAG scheduling — task partitioning and wave sequencing belong to `apply-dag-dispatch`.
- No automated code repair — discovered gaps are reported as findings, never patched
  autonomously inside verify.
- No modification of `gentle-ai`'s RDD — `lucind-ai` verification checks SDD spec fulfillment;
  post-integration code-quality review remains `gentle-ai`'s separate concern.

## Approach

1. **Mechanical run**: before dispatching judgment packets, the orchestrator runs the mechanical
   suite against the candidate state once, capturing exit status, test logs, and build
   diagnostics into a fixed artifact.
2. **Judgment packet construction**: the orchestrator authors two read-only packets
   (`verify-<change>-agy.md`, `verify-<change>-cursor-agent.md`), each with `read_only: true`,
   supplying change context (state/proposal/design/specs/tasks), the diff, and the mechanical
   artifact.
3. **Parallel dispatch & barrier**: `lucind-ai run --packet ... --packet ...` executes both
   judgment lanes in parallel isolated worktrees. `enforceCompletionMode` verifies zero commits
   and a clean tree on `Done`. The barrier releases when both reach a terminal state.
4. **Synthesis & gating**: the orchestrator reads both verdict envelopes, cross-checks cited
   evidence against the codebase, and reconciles them into one verification outcome. If mechanical
   checks passed and both judgments agree with no blocking findings, verify is marked done; if
   either flags an actionable defect, findings are aggregated and verify transitions to a
   blocked/failed state pending remediation.

## Open questions (left to `design`; this proposal does not pick)

1. **Mechanical check artifact delivery**: written to a fixed repository path and referenced in
   `## Context`, inlined directly in the packet prompt body, or a hybrid (inline summary + file
   link to the full log)?
2. **Verdict disagreement arbitration**: orchestrator synthesis (analogous to propose/design),
   pessimistic hard-blocking on any divergence, or severity-tiered thresholds?
3. **Verdict artifact format and placement**: standalone Markdown verdict documents, structured
   `findings` inside `.lucind/result.json`, or both?
4. **Mechanical-failure short-circuit**: if the mechanical check fails outright, skip qualitative
   dispatch entirely to conserve quota, or dispatch anyway for root-cause diagnostic value?
5. **Tooling invocation interface**: a dedicated `lucind-ai check` CLI subcommand, or direct
   orchestrator shell invocation of the existing check script?

## Impact on the existing SDD / verify flow

- Eliminates redundant, non-deterministic test runs inside LLM sessions — mechanical checks run
  once, saving quota and removing agent-side test-runner flakiness/timeouts.
- Adds independent dual-model qualitative review, catching subtle regressions, missed edge
  cases, and spec/test coverage gaps a single reviewer could miss.
- Enforces a clean separation of concerns: verification lanes stay strictly read-only and
  evaluative, never mixed with in-session fixes.
- Produces a durable, archived review record (mechanical log + both verdicts + synthesized
  report) alongside the change's other SDD artifacts.

## Affected areas

| Area | Impact | Description |
|---|---|---|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modified | Row 80 updated from target-direction to the operational two-stage verify workflow. |
| `plugin/claude-code/skills/lucind-ai/assets/` | New/Modified | New verify judgment packet template leveraging `read_only: true`. |
| `internal/integrate/integrate.go` | Reused | `Check` remains the mechanical runner, unmodified. |
| `internal/packet/packet.go` | Reused | Consumes `Packet.ReadOnly` from `read-only-packet-dispatch`. |
| `internal/run/run.go` | Reused | Consumes `enforceCompletionMode` to enforce zero commits/clean tree for judgment lanes. |
| `internal/ledger/ledger.go` | Reused | Standard event types record lane notes and barrier outcomes; no schema changes. |
| `cmd/lucind-ai/cli.go` | Possibly modified | Candidate new `check` subcommand — decided in `design`. |

## Capabilities

### New

- `verify-dual-dispatch`: orchestrator capability to split `verify` into one deterministic
  mechanical check and two parallel read-only qualitative judgment lanes, followed by verdict
  reconciliation.

### Modified

- `sdd-verify`: shifts from single-model in-session review to a structured two-stage dispatch
  via `lucind-ai run`.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Mechanical check output too large to inline in packet prompts. | Medium | `design` selects file-path-based artifact delivery over full inlining. |
| Judgment executor attempts to modify files or re-run mechanical checks. | Medium | `Packet.ReadOnly` + `enforceCompletionMode` fail any lane with tree changes; prompt contract explicitly forbids re-running checks. |
| Conflicting qualitative verdicts stall the pipeline. | Medium | Orchestrator synthesis convention (as with propose/design/specs/tasks) plus an explicit human-escalation path for genuine ambiguity. |
| Dependency risk on `read-only-packet-dispatch`. | Low | Its design is already merged to main and stable; this change builds on a fixed contract. |

## Rollback plan

This proposal is documentation only — reverting its commit changes nothing at runtime. For the
eventual implementation: revert the `SKILL.md` verify-row text and any packet template
additions; the underlying `Packet.ReadOnly`/`enforceCompletionMode`/`Check` capabilities remain
backward-compatible and reusable by other phases; verification falls back to manual orchestrator
review without any database/ledger migration.

## Dependencies

- `read-only-packet-dispatch` (`Packet.ReadOnly`, `enforceCompletionMode`) — canonical design in
  `openspec/changes/read-only-packet-dispatch/design.md`. Hard dependency.
- Deterministic check runner (`internal/integrate.Check` / `lucind-checks.sh`).
- Dual-executor packet dispatch conventions already exercised for propose/design/specs/tasks.
- Independent from `apply-dag-dispatch` — verify evaluates an already-integrated change; it does
  not depend on DAG splitting or `AllowedPaths`.

## Success criteria

- [ ] `verify` is explicitly structured as a two-stage split: mechanical checks once, qualitative
      judgment dispatched twice to `agy` and `cursor-agent`.
- [ ] Qualitative judgment packets are declared and enforced as `read_only: true` lanes.
- [ ] Both executors receive identical mechanical check output and change diff to judge
      independently.
- [ ] Open questions on artifact delivery and verdict conflict resolution are recorded for `design`.
- [ ] `SKILL.md` reflects `verify` as a dispatchable phase rather than an unbuilt target direction.
