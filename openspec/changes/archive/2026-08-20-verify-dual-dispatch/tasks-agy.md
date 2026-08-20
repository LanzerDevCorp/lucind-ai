# Tasks: Verify-Phase Dual Dispatch

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 500–750 lines |
| `review_budget_lines` | 2000 (`state.yaml`) |
| Review-budget risk | **Low** — well under the 2000-line review budget |
| Chained PRs recommended | No |
| Suggested split | Single apply PR |
| Delivery strategy | `single-pr` |
| Chain strategy | pending |

Largest slices are `cmd/lucind-ai/cli_test.go` (CLI subcommand tests, `--out` log capture, passing/failing script handling) and `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` (template asset). CLI wiring and documentation updates are compact.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `cmd/lucind-ai`: `check` subcommand + `--out` flag wrapping `integrate.Check` | PR 1 | `go test ./cmd/lucind-ai -race -count=1` | CLI execution harness | `cmd/lucind-ai/` |
| 2 | `internal/worktree`: Pre-commit `verify-mechanical.log` & worktree inheritance | PR 1 | `go test ./internal/worktree -race -count=1` | Real git via `initRepo` | `internal/worktree/` |
| 3 | Prompt assets: `verify-packet-template.md` asset & `packet-template.md` pointer | PR 1 | `go test ./internal/packet -race -count=1` | File content assertions | `plugin/claude-code/skills/lucind-ai/assets/` |
| 4 | Orchestrator docs: `SKILL.md` row 80 update + operational verify protocol | PR 1 | `go test ./internal/packet -race -count=1` | Documentation assertions | `plugin/claude-code/skills/lucind-ai/` |
| 5 | End-to-end verification: Real CLI check execution & dual read-only dispatch | PR 1 | `go test ./... -race -count=1` | CLI + real worktrees | Full repository |

---

## Sequencing notes (not blockers)

`design.md` already established the architectural decisions (5 final decisions). These implementation details govern the apply phase:

1. **Reusing existing core primitives**: `internal/integrate.Check` (`internal/integrate/integrate.go:79`) and `internal/run.enforceCompletionMode` (`internal/run/run.go`) are already implemented and tested; they are consumed directly without behavioral modification.
2. **Deterministic execution invariant**: Stage 1 (`lucind-ai check`) runs once deterministically and halts the workflow immediately on failure; qualitative judgment packets are dispatched in Stage 2 only after mechanical checks pass and the log is committed.
3. **Commit before dispatch**: `openspec/changes/<change-id>/verify-mechanical.log` is committed to candidate branch `HEAD` prior to `lucind-ai run`. Linked worktrees created by `worktree.Create` inherit the committed log through normal git branch inheritance, requiring zero custom file-injection machinery.
4. **Purely additive rollback**: Rollback requires only reverting commits touching `cmd/lucind-ai/` and `plugin/claude-code/skills/lucind-ai/`. No SQLite migrations or envelope schema changes exist.

---

## Constraints (do not)

- Do not modify `internal/integrate/integrate.go` or add new fields to `internal/ledger/`.
- Do not modify `internal/run/run.go` or `internal/run/batch.go` for file injection — worktree branch inheritance handles log availability.
- Do not alter `.lucind/result.schema.json` to add phase-specific fields or change `required` properties; judgment lanes reuse the existing envelope schema with `commit` omitted.
- Do not run `lucind-checks.sh` or build/test commands inside qualitative judgment lanes (`read_only: true`); executors evaluate the frozen mechanical log in prompt context.

---

## Terminal consumers and indirection trace

| Introduced symbol / asset | Location | Terminal consumer | Purpose |
|---|---|---|---|
| `lucind-ai check` CLI command | `cmd/lucind-ai/cli.go` | Orchestrator invoking mechanical suite during `sdd-verify` stage 1 | Executes `lucind-checks.sh` deterministically, outputs status, duration, and commit SHA. |
| `--out <path>` flag | `cmd/lucind-ai/cli.go` | Orchestrator saving log artifact during `sdd-verify` stage 1 | Writes structured header and combined check transcript to `verify-mechanical.log`. |
| `runCheck` function | `cmd/lucind-ai/cli.go` | Subcommand router in `cmd/lucind-ai/cli.go:run` | Parses flags, resolves repo root, invokes `integrate.Check`, writes output/log. |
| `verify-mechanical.log` | `openspec/changes/<id>/verify-mechanical.log` | Orchestrator (packet `## Context`), judgment executors, auditor, `openspec archive` | Frozen archival record of deterministic mechanical verification. |
| `verify-packet-template.md` | `plugin/.../assets/verify-packet-template.md` | Orchestrator authoring `packets/verify-<id>-agy.md` and `packets/verify-<id>-cursor-agent.md` | Standardized template with `read_only: true`, 3 done-criteria, and re-run hard stop. |
| Pointer note in `packet-template.md` | `plugin/.../assets/packet-template.md` | Packet authors drafting verification packets | References `verify-packet-template.md` for qualitative review lanes. |
| `SKILL.md` verify protocol (row 80) | `plugin/claude-code/skills/lucind-ai/SKILL.md` | Claude Code orchestrator driving `sdd-verify` | Prescribes two-stage execution, dual dispatch, evidence cross-checking, and report synthesis. |
| Canonical `verify.md` | `openspec/changes/<id>/verify.md` | Human reviewer, `state.yaml` updater, `openspec archive` | Definitive verification gate report combining mechanical log and qualitative verdicts. |

---

## Unit 1: `cmd/lucind-ai` — `check` Subcommand & Output Capture

Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand`, `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact`, `specs/verify-mechanical-check/spec.md#Requirement: Single Deterministic Execution`.

Terminal consumers: `runCheck` is consumed by `cmd/lucind-ai/cli.go:run`; the `check` subcommand and `--out` flag are consumed by the orchestrator during `sdd-verify` stage 1.

- [ ] 1.1 **RED** `cmd/lucind-ai/cli_test.go`: Write `TestRunCheckMissingScriptFails`. Invoking `run(ctx, []string{"check"}, stdout, stderr)` against a directory lacking `lucind-checks.sh` returns exit code 1 and writes an error message to stderr indicating `lucind-checks.sh` was not found. Expected failure: `unknown subcommand "check"`. Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand#Scenario: Missing check script exits non-zero`.
- [ ] 1.2 **GREEN** `cmd/lucind-ai/cli.go`: Add `case "check": return runCheck(ctx, args[1:], stdout, stderr)` to `run()`. Implement `runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int` using `resolvePrimaryRoot(ctx)` and invoking `integrate.Check(ctx, targetDir)`. Return exit code 1 when `integrate.Check` reports missing script. `go test ./cmd/lucind-ai -count=1 -run TestRunCheckMissingScriptFails` green for 1.1.
- [ ] 1.3 **RED** `cmd/lucind-ai/cli_test.go`: Write `TestRunCheckPassingScriptSucceeds` and `TestRunCheckFailingScriptFails`.
  - In a repository with an executable `lucind-checks.sh` that exits 0, `lucind-ai check` exits 0 and prints execution status, elapsed duration, git commit SHA, and check transcript to stdout.
  - In a repository with a `lucind-checks.sh` that exits 1, `lucind-ai check` exits 1 and writes the failure transcript and non-zero status to stdout/stderr.
  - Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand#Scenario: Passing mechanical check exits 0`, `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand#Scenario: Failing mechanical check exits non-zero`.
- [ ] 1.4 **GREEN** `cmd/lucind-ai/cli.go`: In `runCheck`, execute `integrate.Check(ctx, targetDir)`. Query candidate git commit SHA via `git rev-parse HEAD`. Print elapsed duration, commit SHA, pass/fail status, and combined transcript. Exit 0 when `passed == true`, exit 1 when `passed == false` or execution fails. `go test ./cmd/lucind-ai -count=1` green for 1.3.
- [ ] 1.5 **RED** `cmd/lucind-ai/cli_test.go`: Write `TestRunCheckOutFlagWritesLogArtifact` and `TestRunCheckOutFlagOverwritesExistingLog`.
  - When `--out <path>` is provided (e.g. `--out openspec/changes/foo/verify-mechanical.log`), `runCheck` creates the parent directories if necessary, writes a structured metadata header (command line, exit code, execution duration, git commit SHA) followed by the complete stdout/stderr transcript, and emits summary to stdout.
  - When the target file already exists, `runCheck --out` overwrites the file with new execution metadata and transcript.
  - When `--out` is omitted, output goes to stdout/stderr only and no file is created.
  - Spec: `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact#Scenario: Log file written with structured header`, `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact#Scenario: Existing log file overwritten`, `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact#Scenario: Default stdout/stderr emission`.
- [ ] 1.6 **GREEN** `cmd/lucind-ai/cli.go`: Add `fs := flag.NewFlagSet("check", flag.ContinueOnError)` in `runCheck` to parse `--out <path>`. If `--out` is specified, construct the structured header and write the combined transcript to the file path using `os.MkdirAll` and `os.WriteFile`. `go test ./cmd/lucind-ai -count=1` green for 1.5.
- [ ] 1.7 **RED** `cmd/lucind-ai/cli_test.go`: Write `TestRunCheckUsageErrors`. Invoking `lucind-ai check` with unrecognized flags (e.g. `--invalid`) or unexpected positional arguments writes usage instructions to stderr and returns exit code 1.
- [ ] 1.8 **GREEN** `cmd/lucind-ai/cli.go`: Handle flag parsing errors and reject unexpected positional arguments in `runCheck`, printing `usage: lucind-ai check [--out <path>]` on stderr. `go test ./cmd/lucind-ai -count=1` green.

---

## Unit 2: Candidate Branch Pre-Commit & Worktree Inheritance

Spec: `specs/verify-mechanical-check/spec.md#Requirement: Candidate Branch Pre-Commit and Worktree Inheritance`, `specs/verify-mechanical-check/spec.md#Requirement: Terminal Consumers of the Mechanical Log Artifact`, `specs/verify-dual-dispatch/spec.md#Requirement: Dual Parallel Judgment Dispatch and Barrier Join`.

Terminal consumers: `verify-mechanical.log` is consumed by (1) the orchestrator embedding its summary in packet `## Context`, (2) judgment executors (`agy` and `cursor-agent`) accessing it inside their worktrees, and (3) orchestrator & human auditor during `verify.md` synthesis and `openspec archive`.

- [ ] 2.1 **RED** `internal/worktree/worktree_test.go`: Write `TestVerifyMechanicalLogInheritedInWorktree`. In a real git test repository:
  - Commit `openspec/changes/test-change/verify-mechanical.log` to candidate branch `HEAD`.
  - Invoke `worktree.Create(ctx, primaryRoot, worktreePath, branchName)`.
  - Assert that `openspec/changes/test-change/verify-mechanical.log` exists in the created worktree with matching content.
  - Assert `PorcelainEmpty(ctx, worktreePath)` returns `true, nil` (working tree remains clean).
  - Assert `HasUniqueCommits(ctx, worktreePath, primaryRoot)` returns `false, nil` (zero unique lane commits).
  - Expected failure: test is red if git worktree creation or gitignore interaction alters file presence or tree cleanliness. Spec: `specs/verify-mechanical-check/spec.md#Requirement: Candidate Branch Pre-Commit and Worktree Inheritance#Scenario: Linked worktrees inherit the log automatically`.
- [ ] 2.2 **GREEN** `internal/worktree/worktree_test.go`: Confirm test passes with existing `worktree.Create` implementation, proving that normal git branch inheritance delivers the committed mechanical log without any custom file-injection machinery in `internal/run` or `ExecuteBatch`. `go test ./internal/worktree -count=1 -run TestVerifyMechanicalLogInheritedInWorktree` green.

---

## Unit 3: Read-Only Judgment Packet Template & Authoring Assets

Spec: `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Judgment Packet Frontmatter`, `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Done-Criteria Contract`, `specs/verify-judgment-packet/spec.md#Requirement: Existing Envelope Schema Reuse Without Churn`, `specs/verify-judgment-packet/spec.md#Requirement: Mechanical Re-Run Prohibition Contract`, `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset`.

Terminal consumers: `verify-packet-template.md` is consumed by the orchestrator authoring judgment packets, parsed by `internal/packet.Parse`; pointer note in `packet-template.md` is consumed by packet authors.

- [ ] 3.1 **RED** `internal/packet/packet_test.go`: Write `TestVerifyPacketTemplateStructure`. Load `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` and parse via `packet.Parse`. Assert:
  - Frontmatter parses with `Packet.ReadOnly == true`, `Packet.Executor == "agy"`, and descriptive `routed_by`.
  - Contains the three mandatory read-only done-criteria: (1) terminal consumer indirection trace, (2) unchanged worktree (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`), (3) qualitative evaluation completed in `.lucind/result.json`.
  - `## Out of scope` explicitly prohibits executing `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suite.
  - `## Hard stops` includes `"Executing mechanical test/build commands when mechanical results are already provided."`
  - Body contains review sections for spec compliance, edge cases, and test quality.
  - Expected failure: template file does not exist. Spec: `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset#Scenario: Verify packet template skeleton`.
- [ ] 3.2 **GREEN** `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`: Create the standardized template asset with `read_only: true` frontmatter, the three read-only done-criteria, the mechanical re-run prohibition in Out of scope and Hard stops, and review evaluation sections. `go test ./internal/packet -count=1 -run TestVerifyPacketTemplateStructure` green for 3.1.
- [ ] 3.3 **RED** `internal/packet/packet_test.go`: Write `TestPacketTemplateVerifyPointer`. Read `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` and assert it contains a reference note pointing authors of qualitative verification lanes to `verify-packet-template.md`. Expected failure: pointer note is absent. Spec: `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset#Scenario: Packet template reference note`.
- [ ] 3.4 **GREEN** `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`: Add reference note pointing authors of qualitative verification lanes to `verify-packet-template.md`. `go test ./internal/packet -count=1 -run TestPacketTemplateVerifyPointer` green for 3.3.

---

## Unit 4: Orchestrator Dual-Dispatch & Synthesis Procedure (`SKILL.md`)

Spec: `specs/verify-dual-dispatch/spec.md#Requirement: Two-Stage SDD Verify Protocol`, `specs/verify-dual-dispatch/spec.md#Requirement: Dual Parallel Judgment Dispatch and Barrier Join`, `specs/verify-dual-dispatch/spec.md#Requirement: Mechanical Log Context Embedding`, `specs/verify-dual-dispatch/spec.md#Requirement: Independent Evidence Cross-Checking`, `specs/verify-dual-dispatch/spec.md#Requirement: Unanimous Pass Reconciliation`, `specs/verify-dual-dispatch/spec.md#Requirement: Disagreement and False-Positive Adjudication`, `specs/verify-dual-dispatch/spec.md#Requirement: Lane Execution Failure Handling`, `specs/verify-dual-dispatch/spec.md#Requirement: Irreconcilable Ambiguity Escalation`, `specs/verify-dual-dispatch/spec.md#Requirement: Canonical Verification Report and State Update`.

Terminal consumers: `SKILL.md` instructions are consumed by Claude Code driving the `sdd-verify` command; `verify.md` is consumed by the human reviewer, `state.yaml`, and `openspec archive`.

- [ ] 4.1 **RED** `internal/packet/packet_test.go`: Write `TestSkillVerifyDocumentation`. Read `plugin/claude-code/skills/lucind-ai/SKILL.md` and assert:
  - Row 80 in the target direction table no longer lists `verify` as "Target direction, not yet built" or blocked.
  - Documents operational Stage 1: run `lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log`, halt on failure, commit log to candidate branch on success.
  - Documents operational Stage 2: author dual read-only judgment packets (`verify-<id>-agy.md` and `verify-<id>-cursor-agent.md`) using `verify-packet-template.md` with embedded mechanical log context, dispatch in parallel via `lucind-ai run --packet ... --packet ...`.
  - Documents reconciliation logic: unanimous pass, disagreement adjudication, false-positive refutation with `file:line` evidence, single-lane failure retry, and ambiguity escalation to human operator.
  - Documents synthesis of canonical `openspec/changes/<change-id>/verify.md` and `state.yaml` update (`verify: { status: done }` or `verify: { status: blocked }`).
  - Expected failure: `SKILL.md:80` currently lists `verify` as unbuilt with blocker notes. Spec: `specs/verify-dual-dispatch/spec.md#Requirement: Two-Stage SDD Verify Protocol#Scenario: SKILL.md documents the operational verify protocol`.
- [ ] 4.2 **GREEN** `plugin/claude-code/skills/lucind-ai/SKILL.md`: Update row 80 in `SKILL.md` to declare `verify` operational. Add the two-stage verify workflow documentation detailing Stage 1 execution and log commit, Stage 2 dual parallel packet dispatch, evidence cross-checking rules, verdict reconciliation matrix, and `openspec/changes/<change-id>/verify.md` report synthesis. `go test ./internal/packet -count=1 -run TestSkillVerifyDocumentation` green for 4.1.

---

## Unit 5: End-to-End Integration & CLI Check Verification

Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand`, `specs/verify-dual-dispatch/spec.md#Requirement: Two-Stage SDD Verify Protocol`, `specs/verify-judgment-packet/spec.md#Requirement: Structural Cleanliness Enforcement via Git Porcelain`.

Terminal consumers: End-to-end integration check verifies composition across CLI, git worktree isolation, packet parser, runtime completion mode enforcement, and result schema validation.

- [ ] 5.1 **End-to-End Mechanical CLI Check**:
  - In a temporary git repository containing a real `lucind-checks.sh` (executing `go test` and `go vet`):
    - Run `lucind-ai check --out openspec/changes/test-verify/verify-mechanical.log`.
    - Confirm command exits 0 and writes structured header (git SHA, duration, exit code 0) and check transcript to `verify-mechanical.log`.
    - Modify `lucind-checks.sh` to exit 1; run `lucind-ai check --out openspec/changes/test-verify/verify-mechanical.log`.
    - Confirm command exits 1 and overwrites `verify-mechanical.log` with non-zero exit code and failure transcript.
    - Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand#Scenario: Passing mechanical check exits 0`, `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact#Scenario: Existing log file overwritten`.
- [ ] 5.2 **End-to-End Dual Read-Only Dispatch Verification**:
  - In a repository with committed candidate changes and `verify-mechanical.log`:
    - Author two judgment packets `packets/verify-test-agy.md` (`executor: agy`, `read_only: true`) and `packets/verify-test-cursor-agent.md` (`executor: cursor-agent`, `read_only: true`) using `verify-packet-template.md`.
    - Dispatch concurrently via `lucind-ai run --packet packets/verify-test-agy.md --packet packets/verify-test-cursor-agent.md`.
    - Confirm both lanes execute in isolated worktrees inheriting `verify-mechanical.log`.
    - Confirm both lanes reach `status: done`, report `HasUniqueCommits == false` and `PorcelainEmpty == true`, and write valid `.lucind/result.json` envelopes omitting `commit`.
    - Spec: `specs/verify-dual-dispatch/spec.md#Requirement: Dual Parallel Judgment Dispatch and Barrier Join#Scenario: Single command dispatches both judgment lanes`, `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Done-Criteria Contract#Scenario: Criterion 2 unchanged-tree evidence`.
- [ ] 5.3 **Full Repository Test Suite Verification**:
  - Run `go test -race -count=1 ./...` across all packages in the repository. Confirm all unit, integration, and CLI tests pass cleanly with zero regressions.

---

## Out of scope & invariant protections (do not implement)

- Do not implement custom file-injection logic in `internal/run` or `internal/executor` — `worktree.Create` branch inheritance is the sole distribution mechanism for `verify-mechanical.log`.
- Do not modify `internal/result/result.schema.json` to add custom top-level fields — qualitative verdicts map to standard `status`, `summary`, `hard_stops`, `done_criteria`, and `findings`.
- Do not add SQLite ledger migrations or new event types — dual dispatch relies entirely on standard `EventLaneStarted`, `EventLaneFinished`, and `EventLaneNote` events.
- Do not implement automated defect remediation — blocked verify outcomes queue follow-up apply tasks in `state.yaml`, not autonomous fixes.
