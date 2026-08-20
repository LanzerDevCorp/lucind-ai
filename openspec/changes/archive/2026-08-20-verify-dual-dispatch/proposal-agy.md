# Proposal: Verify-Phase Dual Dispatch

Split the `verify` SDD phase into two decoupled operations: a single deterministic mechanical check executed once by tooling/orchestrator (`lucind-checks.sh`, test runners, linters), and a dual-dispatched qualitative judgment executed in parallel across `agy` and `cursor-agent` as independent read-only packets. Each executor inspects the implementation diff against the accepted specs, designs, and the mechanical check output, producing independent verification verdict documents without mutating repository code.

## Intent

In the SDD lifecycle (`plugin/claude-code/skills/lucind-ai/SKILL.md:74-80`), the `verify` phase evaluates whether an implemented change satisfies its specification and is safe to merge. Today, verification lacks a dedicated dual-dispatch pipeline because mechanical test executions and qualitative review judgments have been conflated.

`SKILL.md:80` defines the target direction: split verification into mechanical checks (run once deterministically by tooling) and qualitative reviews (dispatched twice to `agy` and `cursor-agent`).

Running deterministic mechanical checks (`go test`, `go vet`, `lucind-checks.sh`, linters) inside multiple LLM sessions provides zero novel information: compilers and test runners produce binary pass/fail truths, and running them through two separate LLM executors only burns agent quota, increases latency, and risks non-deterministic test runner timeouts or environmental flakiness without improving correctness.

In contrast, qualitative verification requires semantic reasoning: checking whether the code faithfully implements the spec's intent, identifying missed edge cases, detecting test suite coverage gaps, flagging under-specified requirements, and auditing architectural integrity. Running qualitative judgment across two independent models (`agy` and `cursor-agent`) yields complementary perspectives and catches subtle defects that a single reviewer might miss.

This proposal establishes the architecture and authoring contracts for verify dual-dispatch, directly building on the read-only packet abstraction provided by the `read-only-packet-dispatch` change.

## Scope

### What Changes

- **Two-stage verification flow**:
  1. *Stage 1 (Mechanical Check)*: The orchestrator or tooling executes the project's verification suite (`lucind-checks.sh` or `internal/integrate.Check`) exactly once against the candidate worktree/branch, capturing stdout, stderr, and exit status into a fixed artifact.
  2. *Stage 2 (Qualitative Judgment)*: The orchestrator constructs two read-only judgment packets (`verify-<change>-agy` and `verify-<change>-cursor-agent`) providing the fixed mechanical check artifact, the change's design/spec artifacts, and the git diff. Both packets are dispatched in parallel via `lucind-ai run`.
- **Read-only judgment packet contract**:
  Judgment packets declare `read_only: true` (consuming `read-only-packet-dispatch`'s `Packet.ReadOnly` field and enforced at runtime by `enforceCompletionMode`). Judgment packets produce an evaluative review verdict rather than modifying source files. Forcing a commit on a judgment packet would result in either an artificial no-op commit or scope creep into attempting inline code fixes during review.
- **Verdict output artifacts**:
  Each executor produces an independent verdict document (e.g. `openspec/changes/<change>/verify-<executor>.md` and/or structured envelope fields in `.lucind/result.json`) reporting findings, severity, coverage gaps, and an approval or rejection recommendation.
- **Orchestration evolution**:
  `plugin/claude-code/skills/lucind-ai/SKILL.md:80` moves from "target direction, not yet built" to the standard operational SDD verify workflow.

### What Stays Untouched

- **Mechanical check runner**: Reuses `internal/integrate/integrate.go:79` (`Check`) and `lucind-checks.sh` conventions without alteration.
- **Read-only packet enforcement**: Directly relies on `read-only-packet-dispatch` (`Packet.ReadOnly` in `internal/packet/packet.go`, `enforceCompletionMode` in `internal/run/run.go`, `HasUniqueCommits`/`PorcelainEmpty` in `internal/worktree/worktree.go`). Verify dual-dispatch does not re-derive or duplicate read-only gating.
- **Status vocabulary and barrier rules**: Retains the 6-value `lane.Status` enum (`internal/lane/status.go:10-18`) and standard barrier release logic (`internal/run/batch.go:25-27`, `internal/barrier/barrier.go:22-31`).
- **Core write phases**: Propose, design, specs, tasks, and apply dual-dispatch pipelines continue their established conventions.

## Non-goals

- **No re-implementation of read-only packet mechanics**: The `read_only` frontmatter flag, `Packet.ReadOnly` field, and git-truth verification in `enforceCompletionMode` are owned by `read-only-packet-dispatch`.
- **No DAG scheduling**: Task partitioning and wave sequencing for apply belong to `apply-dag-dispatch`.
- **No automated code repair in verify**: Judgment packets are strictly evaluative; reviewers do not edit source files or patch tests. Addressing defects flagged during verification requires authoring new tasks in the apply phase.
- **No changes to approvals web UI**: The localhost approval UI and schema v3 (`openspec/changes/approvals-web-ui/`) remain separate and untouched.

## Approach

1. **Stage 1 (Deterministic Execution)**: Before dispatching qualitative review packets, the orchestrator invokes `lucind-checks.sh` (or `integrate.Check`) against the candidate worktree once. The output (stdout, stderr, exit code) is captured as a fixed baseline artifact.
2. **Stage 2 (Dual Judgment Dispatch)**: The orchestrator creates two read-only packets (`verify-<change>-agy.md` and `verify-<change>-cursor-agent.md`) targeting the candidate change. Each packet specifies `read_only: true` in its frontmatter, instructs the agent to audit the diff against `specs/` and `design.md`, and supplies the fixed mechanical check artifact.
3. **Parallel Execution & Barrier**: `lucind-ai run --packet ... --packet ...` executes both judgment packets in parallel worktrees. Each agent writes its envelope to `.lucind/result.json` and its qualitative review to its verdict document.
4. **Git Enforcement & Integration**: Upon reaching `lane.Done`, `internal/run.enforceCompletionMode` verifies that neither lane created commits and that the worktrees remain clean (`git status --porcelain` empty). The barrier releases when both lanes reach a terminal state.
5. **Synthesis**: The orchestrator (or human) reviews the dual verdict documents. If both reviewers agree that the implementation satisfies specs with no blocking findings, the change transitions to verified; if discrepancies or defects are identified, findings are converted into follow-up apply tasks or escalated for human decision.

## Open Design Questions (Left to `design`; this proposal does not pick)

1. **Mechanical check artifact delivery**:
   How should the fixed mechanical check artifact (test outputs, linter logs, exit status) be delivered into each judgment packet?
   - *Option A*: Written to a fixed repository file path (e.g. `openspec/changes/<change>/verify-mechanical.log`) and referenced in the packet's `Context` section.
   - *Option B*: Inlined directly as text within the packet prompt body.
   - *Option C*: Hybrid approach where a summary of test counts and status is inlined in the prompt, with a file path link to the complete raw log.
2. **Handling verdict discrepancies and disagreements**:
   What is the resolution protocol when the two judgment executors return conflicting verdicts (e.g., `agy` passes with minor notes, while `cursor-agent` flags a critical spec violation)?
   - *Option A*: Orchestrator synthesis (analogous to propose/design/specs/tasks), where Claude Code reviews both drafts, reconciles minor differences, and presents a consolidated verdict to the human.
   - *Option B*: Disagreement as a hard barrier / blocking finding, where any material divergence between verdicts automatically marks verification `blocked` and halts integration until human resolution.
   - *Option C*: Asymmetric weighting based on executor strengths (e.g., cursor-agent precision on algorithmic/semantic logic vs. agy broad sweep coverage).
3. **Verdict artifact format and placement**:
   Should qualitative judgments be delivered as standalone Markdown files (e.g. `openspec/changes/<change>/verify-<executor>.md`), structured findings inside `.lucind/result.json`'s `findings` array, or both?
4. **Pre-verification short-circuiting on mechanical failure**:
   If Stage 1 mechanical checks fail (e.g. compile error or broken unit test), should the orchestrator skip Stage 2 qualitative dispatch entirely to conserve quota, or should qualitative packets proceed anyway to provide root-cause analysis and identify spec misalignments?

## Impact on the Existing SDD / Verify Flow

- **Elimination of redundant test runs**: Test suites and linters run once deterministically, saving execution time, reducing quota consumption, and eliminating agent test execution timeouts.
- **Enhanced review depth**: Dual independent reviews from different LLM model families provide high-confidence qualitative verification, catching subtle regressions, unhandled edge cases, and missing tests.
- **Clean separation of concerns**: Verification lanes remain strictly read-only and evaluative. Executors avoid mixed review/edit sessions that lead to unverified dirty worktrees.
- **Durable review record**: Both independent reviews and mechanical logs are archived alongside the change proposal and specs, creating a permanent audit trail for architectural decisions.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modified | Update verify row (`SKILL.md:80`) from target direction to implemented two-stage dual-dispatch flow. |
| `plugin/claude-code/skills/lucind-ai/assets/` | Maybe | Add or update verification packet templates (e.g. `verify-packet-template.md`). |
| `internal/integrate/integrate.go` | Reused | `Check` function executing `lucind-checks.sh` is reused as the Stage 1 mechanical runner without redesign. |
| `internal/packet/packet.go` | Reused | Consumes `Packet.ReadOnly` introduced by `read-only-packet-dispatch`. |
| `internal/run/run.go` | Reused | Consumes `enforceCompletionMode` to enforce zero commits and clean working trees for judgment lanes. |
| `internal/ledger/ledger.go` | Reused | Standard event types record lane notes, barrier outcomes, and verification packet execution. |

## Capabilities

### New Capabilities

- `verify-dual-dispatch`: Orchestrator capability to split the SDD `verify` phase into a single deterministic mechanical check and two parallel read-only qualitative judgment lanes (`agy` and `cursor-agent`), followed by verdict reconciliation.

### Modified Capabilities

- `sdd-verify`: Shifts from manual single-agent verification or in-tree test execution to structured two-stage verification dispatch via `lucind-ai run`.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Stage 1 mechanical check output is too large to inline in packet prompts. | Medium | Design can specify artifact-based passing via file path (`verify-mechanical.log`) rather than prompt inlining. |
| Qualitative reviewer attempts to fix code or modify tests. | Low | `Packet.ReadOnly` and `enforceCompletionMode` enforce a clean tree and zero commits at the git level, failing any mutating lane. |
| Conflicting qualitative verdicts stall the release pipeline. | Medium | Establish clear orchestrator synthesis conventions and explicit escalation paths for human review. |
| High latency from serializing mechanical checks before qualitative dispatch. | Low | Mechanical checks are fast and deterministic; running them once is significantly faster than two concurrent full test runs inside LLM environments. |

## Rollback Plan

This proposal is scoping and design documentation only — reverting its commit changes nothing at runtime. For the eventual implementation:
1. Revert the `SKILL.md` verify-row text and any packet template additions.
2. The underlying binary capabilities (`Packet.ReadOnly`, `enforceCompletionMode`, `Check`) remain backwards-compatible and reusable by other phases (`explore`, `apply`).
3. Verification reverts to single-agent execution or manual orchestrator checks without affecting database schema or repository state.

## Dependencies

- `read-only-packet-dispatch` (`Packet.ReadOnly` field and `enforceCompletionMode` git verification) — canonical design in `openspec/changes/read-only-packet-dispatch/design.md`.
- Deterministic check runner (`internal/integrate.Check` / `lucind-checks.sh`).
- Dual-executor packet dispatch conventions in `plugin/claude-code/skills/lucind-ai/SKILL.md`.

## Success Criteria

- [ ] `verify` phase is explicitly structured as a two-stage split: mechanical checks run once by tooling, qualitative judgment dispatched twice to `agy` and `cursor-agent`.
- [ ] Qualitative judgment packets are declared and enforced as `read_only: true` lanes without repository code mutation.
- [ ] Both executors receive identical mechanical check outputs and change diffs to judge independently.
- [ ] Open questions regarding mechanical artifact delivery and verdict conflict resolution are clearly recorded for the design phase.
- [ ] `SKILL.md` reflects `verify` as a dispatchable phase rather than an unbuilt target direction.
