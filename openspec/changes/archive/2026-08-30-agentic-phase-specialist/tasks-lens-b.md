# Tasks Lens B — Partition & Dispatch Shape: Agentic Phase Specialist

## Assumed decomposition

The change decomposes into three functional deliverables derived from the design file-changes table (`openspec/changes/agentic-phase-specialist/design.md:90-108`):

1. **Skill Documents & Acceptance Contract**: Update `SKILL.md:19` (Hard Rule carve-out for `sdd-*` Specialist Acceptance), `fan-out.md:47-48` (Specialist reads synthesis notes and arbitrates contradictions), and `acceptance-promotion.md:18-36` (decision-bearing Specialist Acceptance and `sdd_phase`-conditional checks) in both `plugin/claude-code/` and `plugin/opencode/` keeping mirror trees byte-identical (`internal/packet/packet_test.go:943-967`).
2. **`internal/accept` Phase-Gated Verification**: Lift `GetLaneMetadata` out of the versioned branch (`internal/accept/accept.go:84-96`), gate `CheckPolicySnapshot` and `v.check` (`:120-137`) to run only when `SDDPhase == "apply"`, empty/missing, or explicit exception, while preserving schema/scope validation (`:214-261`), and add skip/run tests in `internal/accept/accept_test.go:26-67`.
3. **`internal/run` Attempt Gated Verification**: Gate `checkFunc` during `ExecuteAttempt` in `CHECKING` state (`internal/run/attempt.go:431-448`) when all combined lanes are non-apply without exception, and spy check executions via `attemptSpies` in `internal/run/attempt_test.go:24-44,83-92`.

The critical path across all three deliverables is independent because they touch disjoint packages, but all three are required to complete the agentic phase specialist capabilities under strict TDD (`openspec/config.yaml:7-8`).

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Update Hard Rule carve-out, synthesis review, and acceptance checklist steps across Claude Code and OpenCode skill trees | `plugin/claude-code/skills/lucind-ai/SKILL.md`<br>`plugin/opencode/skills/lucind-ai/SKILL.md`<br>`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md`<br>`plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md`<br>`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md`<br>`plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md` | `cursor-agent` | Single revert commit restoring unconditional Agent acceptance ban in `SKILL.md:19`, Orchestrator synthesis review in `fan-out.md:47-48`, and evidence-only subagent acceptance in `acceptance-promotion.md:18-36` |
| 2 | Unconditionally load metadata and gate `v.check` / `CheckPolicySnapshot` on `sdd_phase` in `internal/accept/accept.go` with unit tests | `internal/accept/accept.go`<br>`internal/accept/accept_test.go` | `cursor-agent` | Single revert commit restoring unconditional `v.check` execution in `internal/accept/accept.go` and removing phase-gate test fixtures in `internal/accept/accept_test.go` |
| 3 | Gate `checkFunc` during `ExecuteAttempt` in `internal/run/attempt.go` based on combined lane `SDDPhase` metadata with spy tests | `internal/run/attempt.go`<br>`internal/run/attempt_test.go` | `cursor-agent` | Single revert commit restoring unconditional `checkFunc` execution in `internal/run/attempt.go` and removing `checkCalls` spy assertions in `internal/run/attempt_test.go` |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1, Unit 2, Unit 3 | Yes | Yes: Unit 1 updates doc pairs identically passing `TestSkillTreesByteIdentical` (`internal/packet/packet_test.go:943-967`); Unit 2 implements `internal/accept` gating and unit tests passing `go test ./internal/accept`; Unit 3 implements `internal/run/attempt.go` gating and tests passing `go test ./internal/run`. The combined tree compiles with `CGO_ENABLED=0 go build ./...` and passes full check suite (`go test ./... -race -count=1`) via `lucind-checks.sh:1-4`. |

## Disjointness Check

Evaluation of intra-wave unit pairs under component-boundary prefix matching rules (`internal/packet/disjoint.go:8-22,24-47`):

- **Unit 1 vs Unit 2**:
  - Unit 1 `allowed_paths`: `plugin/claude-code/...`, `plugin/opencode/...` (6 concrete skill files)
  - Unit 2 `allowed_paths`: `internal/accept/accept.go`, `internal/accept/accept_test.go`
  - Prefix rule evaluation (`internal/packet/disjoint.go:8-22`): `plugin/...` and `internal/accept/...` share no component prefix. Neither path prefixes the other under `PathInScope`. Verdict: **DISJOINT (PASS)**.
- **Unit 1 vs Unit 3**:
  - Unit 1 `allowed_paths`: `plugin/claude-code/...`, `plugin/opencode/...` (6 concrete skill files)
  - Unit 3 `allowed_paths`: `internal/run/attempt.go`, `internal/run/attempt_test.go`
  - Prefix rule evaluation (`internal/packet/disjoint.go:8-22`): `plugin/...` and `internal/run/...` share no component prefix. Neither path prefixes the other under `PathInScope`. Verdict: **DISJOINT (PASS)**.
- **Unit 2 vs Unit 3**:
  - Unit 2 `allowed_paths`: `internal/accept/accept.go`, `internal/accept/accept_test.go`
  - Unit 3 `allowed_paths`: `internal/run/attempt.go`, `internal/run/attempt_test.go`
  - Prefix rule evaluation (`internal/packet/disjoint.go:8-22`): `internal/accept/` and `internal/run/` are distinct component directories naming concrete files. Neither path prefixes the other under `PathInScope`. Verdict: **DISJOINT (PASS)**.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**:
- **Review Budget**: Total change is ~100–200 lines across 4 Go files and 3 paired skill docs, well within the 10,000-line review budget (`openspec/config.yaml:7-8`).
- **Low Orchestration Payoff**: A 3-node DAG sidecar introduces `apply-dag.yaml` authoring, packet body files, and `lucind-ai split` orchestration overhead that exceeds the implementation itself.
- **Archived Precedent**: Precedent `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` declined an `apply-dag.yaml` sidecar for a larger 650–1200 line change on identical grounds.
- **Integration Safety**: Executing the work in a single packet sequentially with 3 work-unit commits guarantees that strict TDD RED and GREEN cycles remain self-contained, avoiding bisection reverts at the `Integrate` gate (`internal/run/integrate.go:50-59`).

## Open Questions

- [ ] Task skill contract supersession: `~/.claude/skills/sdd-tasks/SKILL.md` prescribes a monolithic `tasks.md` with checklist, review workload forecast, and Engram persistence, which is superseded by this 3-lens parallel task decomposition workflow returning `.lucind/result.json`.
- [ ] Future CLI tool bridge for Specialists: `openspec/changes/agentic-phase-specialist/design.md:139-140` leaves open what tool or CLI bridge will allow `sdd-*` Specialists to dispatch `lucind-ai run` without Orchestrator mediation in a future Change.
- [ ] Acceptance forced-check flag: `openspec/changes/agentic-phase-specialist/design.md:139-140` leaves open whether `lucind-ai accept` should expose a `--force-checks` CLI flag or if packet-level exception metadata is sufficient.

## Citation Manifest

| citation | claim |
|---|---|
| `internal/accept/accept.go:84-96` | `GetLaneMetadata` loaded during authoring evidence validation |
| `internal/accept/accept.go:120-137` | `CheckPolicySnapshot` and `v.check` execution during mechanical acceptance |
| `internal/accept/accept.go:214-261` | `validateResultAndScope` verifying envelope status, hard stops, done criteria, and path scope |
| `internal/accept/accept_test.go:26-67` | `newVerifierFixture` test fixture constructor for acceptance verification |
| `internal/packet/disjoint.go:8-22` | `PathInScope` implementing component-boundary prefix matching rule for POSIX paths |
| `internal/packet/disjoint.go:24-47` | `DisjointAllowedPaths` verifying pairwise path disjointness across packet definitions |
| `internal/packet/packet_test.go:943-967` | `TestSkillTreesByteIdentical` enforcing byte identity between Claude Code and OpenCode skill trees |
| `internal/run/attempt.go:431-448` | `checkFunc` execution and lease renewal during `ExecuteAttempt` in `CHECKING` state |
| `internal/run/attempt_test.go:24-44` | `attemptSpies` struct definition with `checkCalls` tracking slice |
| `internal/run/attempt_test.go:83-92` | `RunChecks` spy closure recording `checkCalls` invocations |
| `internal/run/integrate.go:50-59` | `Integrate` executing check suite against combined tree and triggering bisection on failure |
| `lucind-checks.sh:1-4` | Repository check script executing `go build ./...` and `go test ./... -race -count=1` |
| `openspec/changes/agentic-phase-specialist/design.md:90-108` | Design file-changes table defining deliverables, modified files, and terminal consumers |
| `openspec/changes/agentic-phase-specialist/design.md:139-140` | Design open questions regarding future specialist tool bridge and acceptance exception flag |
| `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` | Archived precedent declining `apply-dag.yaml` sidecar for change fitting review budget |
| `openspec/config.yaml:7-8` | Change configuration specifying `review_budget_lines: 10000` and `strict_tdd: true` |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19` | Hard Rule specifying Orchestrator authority and forbidding Agent Acceptance |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-36` | Canonical acceptance checklist protocol and subagent delegation contract |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Planning fan-out strategy defining Orchestrator synthesis-note review and contradiction arbitration |
