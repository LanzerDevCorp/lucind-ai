# Tasks Lens C — Proof & Review Burden: Agentic Phase Specialist

## Assumed decomposition

This analysis assumes a three-unit sequential delivery: Unit 1 delivers skill contract documentation updates across Claude Code and OpenCode trees for Specialist Acceptance authority and synthesis note inspection; Unit 2 delivers `internal/accept` phase gating skipping test suites on declared non-apply planning phases; Unit 3 delivers `internal/run` attempt execution gating for combined lanes. The critical path requires the skill contract updates and `internal/accept` verifier gating to land before attempt-level execution integrates the phase-aware verification flow.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 250–350 lines (basis: `internal/accept/accept.go:84-137` metadata load and check gating ~55 lines, `internal/accept/accept_test.go:26-67` skip/run test cases ~100 lines, `internal/run/attempt.go:431-448` attempt check gating ~20 lines, `internal/run/attempt_test.go:24-44,83-92` attempt spy tests ~70 lines, skill doc edits in `plugin/claude-code/skills/lucind-ai/SKILL.md:19-19`, `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-36` across Claude Code and OpenCode trees ~90 lines) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

The line estimate of 250–350 lines is grounded in existing line counts across touched files: `internal/accept/accept.go:84-137` (54 lines touched for unconditional metadata loading and check execution gating), `internal/accept/accept_test.go:26-67` (~100 lines for fixture tests covering apply, empty, missing, and exception cases), `internal/run/attempt.go:431-448` (18 lines touched for `checkFunc` execution gating), `internal/run/attempt_test.go:24-44,83-92` (~70 lines of spy verification), and skill document updates across both Claude Code and OpenCode trees (`plugin/claude-code/skills/lucind-ai/SKILL.md:19-19`, `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-36`, ~90 lines total). This estimate is well under both the 400-line chained PR threshold and the human's 1500-line session review budget.

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A | None (N/A) | N/A: Gating relies on declared `SDDPhase`, not path heuristics (`internal/accept/accept_test.go:127-140`) | None (N/A) |
| Git repository selection | N/A | None (N/A) | N/A: Isolated detached worktrees and `canonicalRoot` unchanged (`internal/accept/accept.go:149-158`, `internal/accept/accept_test.go:297-318`) | None (N/A) |
| Commit state | N/A | None (N/A) | N/A: Candidate verification operates on frozen detached tree (`internal/accept/accept_test.go:142-166`) | None (N/A) |
| Push state | N/A | None (N/A) | N/A: Verification produces local ledger receipts without remote ref mutation (`internal/accept/accept.go:1-3`, `internal/accept/accept_test.go:80-100`) | None (N/A) |
| PR commands | N/A | None (N/A) | N/A: No PR commands or platform API interactions in scope (`CONTEXT.md:23-26`) | None (N/A) |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Acceptance verifier phase gating | `go test -v ./internal/accept -run 'TestAccept_SDDPhase'` (`internal/accept/accept_test.go:26-67`) | `Verifier.Accept` skips `lucind-checks.sh` execution for declared non-apply planning phases (`sdd_phase != "apply"`), while executing full verification suite for `"apply"`, `""`, missing lane metadata, or lanes with check exceptions; non-apply lanes still enforce schema validation, hard stops, done criteria, and allowed paths (`internal/accept/accept.go:84-137,214-261`). | Does not prove attempt execution phase gating in `internal/run` or CLI rendering. |
| Attempt execution phase gating | `go test -v ./internal/run -run 'TestExecuteAttempt_SDDPhase'` (`internal/run/attempt_test.go:24-44,83-92`) | `ExecuteAttempt` evaluates combined lane metadata via `Deps.Ledger` and invokes `checkFunc` during `CHECKING` only when at least one combined lane is `"apply"`, empty/missing, or has an exception; skips `checkFunc` when all combined lanes are non-apply planning phases (`internal/run/attempt.go:431-448`). | Does not prove receipt persistence in `internal/accept` or ungated `integrate.Check` behavior. |
| Ungated check primitive regression | `go test -v ./internal/integrate -run 'TestCheck'` (`internal/integrate/integrate_test.go:471-500`) | `integrate.Check` remains ungated, executing `lucind-checks.sh` directly and returning error if missing or non-zero (`internal/integrate/integrate.go:159-200`). | Does not prove caller-level gating in `accept.go` or `attempt.go`. |
| Skill tree parity & glossary regression | `go test -v ./internal/packet -run 'TestSkillTreesByteIdentical\|TestSkillAssetContract'` (`internal/packet/packet_test.go:924-967`) | The Claude Code and OpenCode skill trees remain byte-identical after Hard Rule, fan-out, and acceptance-promotion doc edits, and `references/core/domain.md` remains in lockstep with `CONTEXT.md` (`plugin/claude-code/skills/lucind-ai/SKILL.md:19-19`). | Does not prove runtime subagent comprehension of the updated Hard Rule carve-out. |
| Repository suite compilation & tests | `./lucind-checks.sh` (`lucind-checks.sh:1-4`) | Entire repository compiles cleanly and passes all unit, integration, and race detector tests. | Does not prove external `~/.claude/skills/sdd-*/SKILL.md` configurations updated by human maintainer. |

## Verification Gaps

1. External Specialist prompts (`~/.claude/skills/sdd-*/SKILL.md`) reside outside the repository and cannot be verified by in-repo tests; human manual application is required (`openspec/changes/agentic-phase-specialist/design.md:102-107`).
2. Structured Phase Verdict chat completion format is emitted by the Specialist LLM into the Orchestrator conversation (`openspec/changes/agentic-phase-specialist/specs/phase-verdict-reporting/spec.md:9-12`) without a Go runtime JSON schema parser seam (`openspec/changes/agentic-phase-specialist/design.md:14-20`).

## Open Questions

- [ ] None. All architecture decisions, test seams, and verification mappings are fully frozen in `openspec/changes/agentic-phase-specialist/design.md:112-117` and capability spec deltas.

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:23-26` | Defines local Coordination Scope without remote PR automation or cross-machine coordination. |
| `CONTEXT.md:91-93` | Defines Promotion as human-confirmed integration into declared Integration Target. |
| `CONTEXT.md:107-109` | Defines Phase Verdict as compressed report returned to Orchestrator. |
| `internal/accept/accept.go:1-3` | Package accept documentation stating receipts are local mechanical evidence without ref mutation. |
| `internal/accept/accept.go:49-55` | Verifier struct definition and check function injection seam. |
| `internal/accept/accept.go:84-137` | Accept method loading metadata and gating check execution based on SDD phase. |
| `internal/accept/accept.go:149-158` | Canonical repository root resolution ensuring isolated worktree execution. |
| `internal/accept/accept.go:214-261` | Result envelope and declared scope validation enforcing fail-closed checks. |
| `internal/accept/accept.go:406-428` | Worktree isolation creation for candidate verification. |
| `internal/accept/accept_test.go:26-67` | Test fixture setup for candidate verification tests. |
| `internal/accept/accept_test.go:80-100` | Unit test asserting acceptance receipt generation without remote ref push. |
| `internal/accept/accept_test.go:127-140` | Unit test asserting rejection of candidate with undeclared path modifications. |
| `internal/accept/accept_test.go:142-166` | Unit test asserting rejection of candidate commit or tree hash mismatch. |
| `internal/accept/accept_test.go:297-318` | Unit test asserting worktree isolation and canonical root checks. |
| `internal/integrate/integrate.go:159-200` | Shared check primitive executing lucind-checks.sh directly without phase gating. |
| `internal/integrate/integrate_test.go:471-500` | Unit tests asserting check behavior on missing or passing script. |
| `internal/ledger/lanes_meta.go:20-47` | LaneMetadata struct defining SDDPhase field for phase tracking. |
| `internal/ledger/lanes_meta.go:49-60` | UpdateLaneMetadata updating lane metadata and audit log. |
| `internal/packet/packet_test.go:924-941` | Test ensuring domain.md glossary remains in lockstep with CONTEXT.md. |
| `internal/packet/packet_test.go:943-967` | Test ensuring Claude Code and OpenCode skill trees are byte-identical. |
| `internal/run/attempt.go:217-328` | ExecuteAttempt state machine managing attempt execution lifecycle. |
| `internal/run/attempt.go:431-448` | CHECKING state gating checkFunc invocation on combined lane SDD phases. |
| `internal/run/attempt_test.go:24-44` | Test spies recording check and promote calls during attempt execution. |
| `internal/run/attempt_test.go:83-92` | RunChecks test double recording worktree check calls. |
| `internal/run/run.go:165-165` | Deps.Ledger field providing ledger access for lane metadata resolution. |
| `internal/run/run.go:208-208` | Deps.RunChecks field providing check execution injection point. |
| `internal/run/run.go:377-394` | UpdateLaneMetadata call persisting dispatch-time SDD phase metadata. |
| `lucind-checks.sh:1-4` | Shell script running full build and race detector tests. |
| `openspec/changes/agentic-phase-specialist/design.md:14-20` | Architecture decision establishing structured markdown Phase Verdict in chat. |
| `openspec/changes/agentic-phase-specialist/design.md:21-32` | Architecture decision gating checks at callers based on SDDPhase. |
| `openspec/changes/agentic-phase-specialist/design.md:33-48` | Architecture decision carving out Specialist Acceptance authority in Hard Rule. |
| `openspec/changes/agentic-phase-specialist/design.md:102-107` | Instructions for human update of external SDD specialist skill prompts. |
| `openspec/changes/agentic-phase-specialist/design.md:112-117` | Testing strategy and test seams across unit and regression layers. |
| `openspec/changes/agentic-phase-specialist/design.md:121-127` | Threat matrix evaluating security boundaries and planned tests. |
| `openspec/changes/agentic-phase-specialist/specs/acceptance-verifier/spec.md:5-8` | Requirement for fail-closed mechanical criteria and SDD phase check gating. |
| `openspec/changes/agentic-phase-specialist/specs/acceptance-verifier/spec.md:41-57` | Acceptance scenarios for apply, planning, unlabeled, and exception lanes. |
| `openspec/changes/agentic-phase-specialist/specs/phase-specialist-dispatch/spec.md:5-8` | Requirement for Specialist sequencing and phase-scoped Acceptance authority. |
| `openspec/changes/agentic-phase-specialist/specs/phase-specialist-dispatch/spec.md:28-50` | Scenarios for Specialist Acceptance, Dual-Judge, and Promotion prohibition. |
| `openspec/changes/agentic-phase-specialist/specs/phase-verdict-reporting/spec.md:9-12` | Requirement for structured Phase Verdict reporting to Orchestrator. |
| `openspec/changes/agentic-phase-specialist/specs/phase-verdict-reporting/spec.md:13-29` | Scenarios for compressed Phase Verdict and bounded correction dispatch. |
| `openspec/changes/agentic-phase-specialist/specs/sdd-planning-fan-out/spec.md:5-8` | Requirement for two-wave planning fan-out and Specialist synthesis inspection. |
| `openspec/changes/agentic-phase-specialist/specs/sdd-planning-fan-out/spec.md:22-32` | Scenarios for contradiction arbitration and synthesis note withholding. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19-19` | Hard Rule defining Orchestrator authority and Agent constraints. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step mechanical acceptance sequence and checklist. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Acceptance subagent delegation protocol and tool restrictions. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25` | Fan-out dispatch rules requiring all lens receipts before synthesis. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Protocol for synthesis notes review and citation verification. |
