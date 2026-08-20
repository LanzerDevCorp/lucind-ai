# Proposal: Verify-Phase Dual Dispatch

Split `lucind-ai`'s `verify` SDD phase into a deterministic mechanical check run once by tooling/orchestrator and a dual-dispatched qualitative judgment run across independent read-only packets to `agy` and `cursor-agent`.

**This change turns the final row of `plugin/claude-code/skills/lucind-ai/SKILL.md:80` from a blocked target into an operational workflow, building directly on the read-only packet foundation established by `read-only-packet-dispatch`.**

## Intent

In the SDD lifecycle, `verify` proves that an implemented change satisfies its specifications, passes tests, and introduces no unintended regressions before merging or archiving. Today, `verify` runs as a local Claude Code subagent session or manual inspection, where a single model both executes test commands and evaluates its own work.

`plugin/claude-code/skills/lucind-ai/SKILL.md:80` defines the target direction for `verify`: mechanical checks run exactly once deterministically, while qualitative judgment is dispatched concurrently to two distinct executor subscriptions (`agy` and `cursor-agent`). This proposal defines that split execution model.

Mechanical verification (`go test`, `go vet`, `lucind-checks.sh`, build scripts) is strictly deterministic — running compiler and test suites twice through two LLMs in separate worktrees yields identical stdout/stderr while doubling quota consumption, wall-clock time, and introducing potential LLM interpretation errors or flakiness. Conversely, qualitative judgment (assessing spec adherence, identifying edge case omissions, evaluating architectural coherence, and spotting under-specified behaviors) is non-deterministic and benefits heavily from dual-executor cross-checking across different model families.

By establishing this mechanical/qualitative split and dispatching judgment through read-only packets, `verify` achieves rigorous, multi-subscription verification without wasted compute or contaminated git history.

## Current behavior vs. target behavior

| Dimension | Current behavior (verified in codebase) | Target behavior (this proposal) | Evidence / Reference |
|---|---|---|---|
| **Execution model** | Run locally by Claude Code subagent or manual conversation; single-model self-evaluation. | Split: mechanical check run once by tooling/orchestrator; qualitative judgment dispatched to `agy` and `cursor-agent` in parallel. | `SKILL.md:80`, `docs/prd.md:60-70` |
| **Mechanical checks** | Invoked inside Claude session or lane prompt; results subject to prompt context limits and LLM interpretation. | Executed once by host tooling/orchestrator (`lucind-checks.sh` / `internal/integrate.Check`); output frozen as an immutable artifact. | `internal/integrate/integrate.go:79-99`, `lucind-checks.sh` |
| **Qualitative review** | Single-model perspective (Anthropic); potential bias toward its own implementation or task decomposition. | Independent dual-executor review (`agy` on Google Gemini + `cursor-agent` on Cursor/OpenAI/Anthropic) evaluating the exact same frozen diff and check artifact. | `SKILL.md:42-72`, `docs/prd.md:85-104` |
| **Lane write & git invariants** | Default packets require commits (`packet-template.md:40-41`); verification without changes forces artificial commits or fails write invariants. | Judgment packets declare `read_only: true`; `enforceCompletionMode` verifies zero unique commits and clean porcelain tree. | `openspec/changes/read-only-packet-dispatch/design.md:26-33,41-64` |
| **Scope boundary** | Verification runs risk of scope creep — modifying code directly to fix defects rather than reporting verdicts. | Read-only enforcement structurally prevents lanes from modifying code; judgment packets produce review verdicts only. | `read-only-packet-dispatch/design.md:50-54` |
| **Artifact output** | Inline transcript notes or ad-hoc test output. | Deterministic mechanical check log + dual independent verdict documents (`verify-agy.md`, `verify-cursor-agent.md`) synthesized by orchestrator. | `SKILL.md:53-59` |
| **SDD status** | `SKILL.md:80` lists `verify` as "Target direction, not yet built — do not attempt without addressing named blocker". | `SKILL.md:80` moves from blocked target to built orchestrator dispatch procedure. | `SKILL.md:74-81` |

## Scope

### What changes

- **Two-phase verify execution model**:
  1. **Mechanical check (run once)**: Tooling/orchestrator executes `lucind-checks.sh` (or `internal/integrate.Check` / `go test ./...`) against the candidate tree once. The full output (stdout, stderr, exit code) is captured as a fixed, immutable mechanical-check artifact.
  2. **Qualitative judgment (dispatched twice)**: The orchestrator constructs two read-only judgment packets containing the diff, change specs, tasks, and the captured mechanical-check artifact. One is dispatched to `agy` and one to `cursor-agent` via `lucind-ai run --packet ... --packet ...`.
- **Consumption of `read-only-packet-dispatch`**:
  - Verification judgment packets set frontmatter `read_only: true`.
  - Terminal consumer: `internal/run.enforceCompletionMode` (designed in `openspec/changes/read-only-packet-dispatch/design.md`) verifies that both judgment lanes produce zero unique commits and maintain a clean working tree (`HEAD == merge-base`, `git status --porcelain` empty).
  - Why read-only is mandatory: Verification judgment packets produce review verdicts (verdict documents and envelope findings), not code changes. Requiring commits would force either fabricated no-op commits or scope creep into fixing discovered defects inline.
- **Verification packet authoring convention**:
  - A standardized prompt contract for qualitative judgment packets instructing executors to evaluate spec compliance, untested edge cases, regression risks, and criteria fulfillment against the frozen diff and mechanical check artifact.
  - Executors output verdict documents (e.g. `openspec/changes/<change>/verify-agy.md` and `verify-cursor-agent.md` or envelope findings) without modifying repository source files.
- **Orchestrator synthesis & recording**:
  - Claude Code orchestrator reads both judgment verdicts, correlates them with the mechanical check output, and records the synthesized verdict in `openspec/changes/<change>/state.yaml` and engram.
- **Documentation update**:
  - `plugin/claude-code/skills/lucind-ai/SKILL.md:80` updates `verify` from "Target direction, not yet built" to the operational split-verify workflow.

### What stays untouched

- **`lucind-ai` binary execution core**:
  - `ExecuteBatch`, worktree isolation (`internal/worktree`), envelope validation (`internal/result`), and SQLite ledger (`internal/ledger`) operate without verify-specific Go code modifications.
  - Read-only lane verification relies entirely on `Packet.ReadOnly` and `enforceCompletionMode` from `read-only-packet-dispatch`.
- **Mechanical check execution**:
  - Reuses existing `lucind-checks.sh` and `internal/integrate.Check` (`internal/integrate/integrate.go:72-99`) as the canonical test script.
- **Combine, conflict resolution, and bisection**:
  - Reuses `internal/integrate/integrate.go` (`Combine`), `internal/resolve/resolve.go` (`Resolve`), and `internal/run/integrate.go` (`bisect`) untouched.
- **Dual-executor propose/design/specs/tasks workflow**:
  - Verified write-phase dual-dispatch pattern (`SKILL.md:49-64`) remains unchanged; propose/design/specs/tasks continue using write packets.
- **Sibling changes**:
  - `apply-dag-dispatch` (`AllowedPaths` and DAG wave sequencing) remains completely independent.
  - `approvals-web-ui` (`lucind-ai serve` and schema v3) remains completely independent.

## Non-goals

- **No re-running mechanical checks inside LLM lanes**: LLM judgment lanes will not execute `go test` or `lucind-checks.sh`; they receive the captured output.
- **No code mutation during verification**: Judgment packets are strictly read-only; remediation of findings belongs to a separate `apply` cycle or human intervention.
- **No Go binary changes specific to verify**: Unlike `read-only-packet-dispatch` (which adds `Packet.ReadOnly` and git checks) and `apply-dag-dispatch` (which adds `AllowedPaths` and wave scheduling), verify dual-dispatch is an orchestrator pattern and prompt contract layered on top of those primitives.
- **No automated code fixing or auto-remediation**: Discovered gaps or defects are reported as review findings; they are never patched autonomously inside the verify phase.
- **No modification of `gentle-ai` RDD**: `lucind-ai` verification verifies delegation integrity and SDD spec fulfillment; post-integration code-quality review remains `gentle-ai`'s RDD driven from `opencode` (`docs/prd.md` §9).

## Approach

1. **Mechanical Run**:
   Before dispatching LLMs, the orchestrator executes the mechanical suite against the target state (e.g. `sh lucind-checks.sh`). It captures:
   - Command exit status (0 for success, non-zero for failure).
   - Test execution logs (package test results, race detector warnings, vet output).
   - Build diagnostics and compilation errors.
   The output is saved as a fixed mechanical-check artifact (e.g., in `.lucind/mechanical-check.log` or directly in the packet prompt context).

2. **Judgment Packet Construction**:
   The orchestrator authors two identical judgment packets (`verify-agy.md` and `verify-cursor-agent.md` configurations), both specifying `read_only: true`.
   The prompt body includes:
   - Target change context (`state.yaml`, `proposal.md`, `design.md`, `specs/`, `tasks.md`).
   - Git diff of the change (`git diff main...HEAD`).
   - The captured mechanical-check artifact.
   - Specific qualitative review questions:
     1. Did the implementation satisfy all RFC 2119 requirements in `specs/`?
     2. Are there missing edge cases or boundary conditions?
     3. Is test coverage sufficient for all new code paths?
     4. Did the implementation deviate from `design.md` or touch undeclared scope?

3. **Dual Dispatch & Barrier Join**:
   The orchestrator invokes `lucind-ai run --packet packet-verify-agy.md --packet packet-verify-cursor-agent.md`.
   Both lanes execute in parallel in isolated worktrees.
   `decideStatus` and `enforceCompletionMode` verify that both lanes finish with zero commits and clean working trees.
   The barrier releases when both reach `lane.Done` (or report `blocked`/`failed`).

4. **Synthesis & Gating**:
   The orchestrator reviews both verdict documents:
   - If mechanical checks passed AND both executors return clean approval (no blocking defects/gaps), `verify` is marked `done` in `state.yaml`.
   - If either executor identifies actionable defects or missing spec requirements, the orchestrator aggregates the findings into `state.yaml`, transitions `verify` to `blocked` or `failed`, and queues an apply remediation packet.

## Open design questions (left to `design`; this proposal does not pick)

1. **Mechanical check artifact transport**: How should the fixed mechanical check output be passed into each judgment packet?
   - *Option A*: Written to a fixed path within the worktree (e.g. `.lucind/mechanical-check.log`), referenced in `Context`.
   - *Option B*: Embedded directly inline within the Markdown prompt body of the judgment packet.
   - *Option C*: Passed via an orchestrator-managed shared artifact path outside git.
2. **Disagreement arbitration policy**: What happens when the two qualitative judgment verdicts disagree (e.g. `agy` flags a blocking spec gap while `cursor-agent` approves, or vice versa)?
   - *Option A*: Orchestrator synthesis (same as propose/design): Claude Code reviews both arguments, assesses codebase evidence, and decides the canonical verdict.
   - *Option B*: Pessimistic blocking: Any unresolved finding from either executor is treated as a blocking defect requiring explicit resolution or human override.
   - *Option C*: Severity-tiered threshold: Trivial/nit observations are noted in `state.yaml` without blocking, while functional/spec deviations block progression.
3. **Verdict artifact format**: How should executors format and return their qualitative verdicts?
   - *Option A*: Markdown verdict documents written to disk (e.g. `openspec/changes/<change>/verify-agy.md` and `verify-cursor-agent.md`).
   - *Option B*: Structured entries inside `.lucind/result.json` (`findings`, `summary`, and `done_criteria`).
   - *Option C*: Hybrid: Structured summary in `result.json` with detailed review report in a designated draft markdown path.
4. **Failing mechanical check short-circuit**: If the mechanical check fails (`exit != 0` on `lucind-checks.sh`), should the orchestrator skip qualitative dual-dispatch entirely to conserve quota, or dispatch anyway to obtain qualitative diagnostic feedback on the failure?
5. **Tooling invocation interface**: Should mechanical check execution be driven via a dedicated `lucind-ai verify --mechanical` CLI subcommand or handled directly by the orchestrator via standard shell execution of `lucind-checks.sh` / `internal/integrate.Check`?

## Impact on the existing SDD / verify flow

- `sdd-verify` shifts from a single-model in-session check to an explicit two-stage pipeline: deterministic tooling execution followed by dual-executor qualitative judgment.
- Eliminates confirmation bias and self-review blind spots by ensuring that the models evaluating the implementation are independent of the single orchestrator context.
- Guarantees zero quota waste on mechanical testing by running deterministic builds/tests once.
- Preserves clean git history: read-only judgment lanes cannot accidentally commit partial fixes, dummy files, or dirty trees.

## Affected areas

| Area | Impact | Description |
|---|---|---|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modified | Row 80 updated from "Target direction, not yet built" to the operational split verify workflow; verify orchestrator procedure added. |
| `plugin/claude-code/skills/lucind-ai/assets/` | Modified / New | Add verify judgment packet template (leveraging `read_only: true`). |
| `openspec/config.yaml` | Untouched / Referenced | `verify.test_command` and `verify.build_command` remain the configuration for mechanical checks. |
| `lucind-checks.sh` | Untouched / Reused | Continues as the canonical mechanical verification script executed once by tooling. |
| `internal/run/` & `internal/packet/` | Untouched | Reuses `Packet.ReadOnly` and `enforceCompletionMode` delivered by `read-only-packet-dispatch`. |
| `internal/integrate/` | Untouched | `internal/integrate.Check` remains available for programmatic test invocation. |

## Capabilities

### New

- `verify-split-dispatch`: Orchestrator capability to execute deterministic mechanical checks once, freeze the output, and dispatch dual read-only judgment packets to `agy` and `cursor-agent`.
- `verify-qualitative-judgment`: Standardized prompt contract and evaluation schema for read-only qualitative verification lanes.

### Modified

- `sdd-verify`: Shifts from local single-model review into a two-stage deterministic-check plus dual-subscription judgment pipeline.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| **Judgment executor attempts to modify files to fix bugs** | Medium | `read_only: true` frontmatter causes `enforceCompletionMode` to fail any lane with working tree changes or commits (`read-only-packet-dispatch/design.md:52-54`). |
| **Mechanical check output exceeds prompt context budget** | Low-Medium | Mechanical runner truncates verbose passing test logs and preserves full error output for failures; design open question 1 evaluates file vs inline passing. |
| **Executors disagree on subjective code aesthetics rather than spec compliance** | Medium | Prompt contract strictly scopes qualitative judgment to RFC 2119 spec requirements, test coverage holes, and regression risks — ruling out stylistic nits. |
| **Mechanical checks pass but tests were under-specified** | High | The primary purpose of qualitative dual-dispatch is specifically auditing test coverage and spec intent against the implementation to catch this exact risk. |
| **Dependency risk on `read-only-packet-dispatch`** | Low | Canonical design for `Packet.ReadOnly` is already merged (`openspec/changes/read-only-packet-dispatch/design.md`); verify dual-dispatch builds directly on its stable contract. |

## Rollback plan

This proposal document is specification only — reverting its commit has zero runtime effect.
For the eventual implementation:
1. Revert `SKILL.md` verify-row text back to target-direction state.
2. Revert any verify packet template additions.
3. No Go code or database migrations to revert: verify dual-dispatch introduces no new binary code or SQLite ledger schemas.
4. If a verify run fails or blocks, `sdd-verify` can temporarily fallback to orchestrator local verification without unwinding `read-only-packet-dispatch` or `apply-dag-dispatch`.

## Dependencies

- **`read-only-packet-dispatch`**: Hard dependency on `Packet.ReadOnly` and `internal/run.enforceCompletionMode` (`openspec/changes/read-only-packet-dispatch/design.md`).
- **Mechanical test suite**: Canonical `lucind-checks.sh` and `openspec/config.yaml` verify commands.
- **Dual-executor dispatch**: Working `lucind-ai run --packet ... --packet ...` parallel batch execution with `agy` and `cursor-agent`.
- **Independent from `apply-dag-dispatch`**: Verification evaluates an integrated change; it does not depend on DAG splitting or `AllowedPaths`.

## Success criteria

- [ ] `openspec/changes/verify-dual-dispatch/proposal.md` establishes the split verify model: mechanical checks executed once, qualitative judgment dispatched twice.
- [ ] Proposal explicitly defines the dependency on `read-only-packet-dispatch` (`Packet.ReadOnly` and `enforceCompletionMode`) and explains why judgment packets must be read-only.
- [ ] Mechanical checks are proven deterministic and captured as a single fixed artifact, avoiding redundant LLM execution and quota waste.
- [ ] Qualitative review is established as independent dual-dispatch to `agy` and `cursor-agent` in isolated worktrees.
- [ ] Open questions section identifies artifact transport, verdict arbitration, verdict formatting, mechanical failure short-circuiting, and CLI invocation.
- [ ] Concrete evidence table contrasts current vs. target verification behaviors with codebase citations.
