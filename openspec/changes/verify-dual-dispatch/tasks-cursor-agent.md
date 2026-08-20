# Tasks: Verify-Phase Dual Dispatch

Strict TDD. Runner: `go test ./...`. Every code item is one RED test, then the smallest GREEN that makes it pass. Do not bulk-write tests, then bulk-write code.

Canonical sources (read in full before coding):
- `openspec/changes/verify-dual-dispatch/design.md` (all 5 decisions final)
- `openspec/changes/verify-dual-dispatch/specs/verify-mechanical-check/spec.md`
- `openspec/changes/verify-dual-dispatch/specs/verify-judgment-packet/spec.md`
- `openspec/changes/verify-dual-dispatch/specs/verify-dual-dispatch/spec.md`
- `openspec/changes/read-only-packet-dispatch/design.md` (merged read-only baseline)

## Terminal Consumers and Indirection Trace

Every indirection introduced by this change is demonstrably consumed by a terminal consumer:

| Introduced Symbol / Asset | Location | Terminal Consumer | Purpose |
|---|---|---|---|
| `lucind-ai check [--out <path>]` CLI subcommand | `cmd/lucind-ai/cli.go` (`runCheck`) | Orchestrator executing Stage 1 of `sdd-verify` | Executes `lucind-checks.sh` deterministically once; prints summary/transcript and optionally writes log file |
| `formatMechanicalLog` header formatter | `cmd/lucind-ai/cli.go` | `runCheck` in `cli.go` when `--out` is provided | Formats structured metadata header (SHA, duration, exit code, command) prepended to stdout/stderr transcript |
| `openspec/changes/<id>/verify-mechanical.log` | Candidate branch filesystem | (1) Orchestrator embedding summary in packet `## Context`<br>(2) Judgment executors reading frozen artifact in linked worktrees<br>(3) Orchestrator and human auditor during `verify.md` synthesis and `openspec archive` | Durable, committed record of the single deterministic mechanical verification run |
| `plugin/.../assets/verify-packet-template.md` | Template asset file | Orchestrator authoring Stage 2 qualitative judgment packets | Standardized prompt contract with `read_only: true`, 3 read-only done criteria, and mechanical rerun hard stop |
| Reference note in `packet-template.md` | `plugin/.../assets/packet-template.md` | Human and orchestrator packet authors | Directs authors of qualitative verification lanes to `verify-packet-template.md` |
| Operational Two-Stage Verify Workflow in `SKILL.md` | `plugin/.../SKILL.md` (row 80 & section) | Orchestrator executing `sdd-verify` phase | Authoritative lifecycle guide for mechanical execution, dual packet dispatch, evidence cross-checking, and verdict reconciliation |

---

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | **740–1090** (impl ~150–220, tests ~380–560, docs/templates ~210–310) |
| `review_budget_lines` | 2000 (`state.yaml`) |
| Review-budget risk | **Low** — well within 2000-line budget |
| Chained PRs recommended | **No** — comfortable single PR |
| Suggested split | Single apply PR |
| Delivery strategy | `single-pr` (matches `state.yaml`) |

Unlike the sibling `apply-dag-dispatch` change (which required 2600–3800 lines due to greenfield DAG algorithms and git diff trees), `verify-dual-dispatch` is small and surgically bounded:
1. It reuses `internal/integrate.Check` unmodified via a lightweight CLI wrapper in `cmd/lucind-ai/cli.go`.
2. It consumes `Packet.ReadOnly` and `run.enforceCompletionMode` from `read-only-packet-dispatch` unmodified without adding new runner or parser code.
3. It reuses `result.schema.json` without schema churn or new database tables/columns.
4. The remaining work consists of CLI tests, asset template creation, and `SKILL.md` workflow documentation.

### Suggested Work Units

| Unit | Goal | Focused Test Command | Runtime Harness | Rollback Boundary |
|---|---|---|---|---|
| 1 | `check` subcommand + `--out` flag wrapping `integrate.Check` | `go test ./cmd/lucind-ai -run "TestRunCheck.*" -race -count=1` | `run(ctx, []string{"check", ...}, ...)` against temp test repos | `cmd/lucind-ai/cli.go`, `cmd/lucind-ai/cli_test.go` |
| 2 | `verify-mechanical.log` format + worktree inheritance | `go test ./cmd/lucind-ai ./internal/worktree -run "Test.*(MechanicalLog|Inherits).*" -race -count=1` | Real git via `initRepo` (`worktree.Create`) | `cmd/lucind-ai/cli.go`, `internal/worktree/worktree_test.go` |
| 3 | Read-only verify packet template asset + pointer note | `go test ./internal/packet ./internal/result -run "TestVerifyPacket.*" -race -count=1` | `packet.Parse` + schema validation against `result.schema.json` | `plugin/.../assets/verify-packet-template.md`, `packet-template.md`, test assertions |
| 4 | `SKILL.md` row 80 update + two-stage verify documentation | `go test ./internal/packet -run "TestSkillMD.*" -race -count=1` | String assertions on `SKILL.md` | `plugin/claude-code/skills/lucind-ai/SKILL.md`, test assertions |
| 5 | End-to-end CLI check & dual read-only dispatch verification | `go test ./cmd/lucind-ai -run "TestRunCheckEndToEnd.*" -race -count=1` | Real `lucind-checks.sh` execution + `go test ./...` sweep | Whole change |

---

## Sequencing Notes (Not Blockers)

`design.md` already chose the architecture (all 5 decisions final). These are implementation details to observe during apply:

1. **`internal/integrate.Check` is reused unmodified.** `integrate.Check(ctx, targetPath)` (`internal/integrate/integrate.go:79`) already executes `lucind-checks.sh` at the root of `targetPath` via `sh`, distinguishes missing scripts from exit errors, and returns `(passed bool, output string, err error)`. `cli.go` wraps this function without altering its signature or behavior.
2. **Metadata header formatting for `verify-mechanical.log`.** When `--out <path>` is passed to `lucind-ai check`, the written file prepends a fixed header to the output transcript:
   ```text
   === lucind-ai mechanical check ===
   Git Commit SHA: <40-char-hex>
   Command: lucind-checks.sh
   Duration: <elapsed-time>
   Exit Code: <0 or non-zero>
   ==================================
   <combined stdout/stderr transcript>
   ```
3. **Overwrite semantics for `--out`.** Re-running `lucind-ai check --out <path>` overwrites any existing log file cleanly using `os.WriteFile`, ensuring retry runs never append corrupted logs.
4. **Pre-commit worktree inheritance.** Committing `verify-mechanical.log` to candidate branch `HEAD` before `lucind-ai run` dispatches judgment packets ensures that `worktree.Create` automatically makes the log available inside linked worktrees via normal git branch inheritance, requiring zero custom file-copying code in `ExecuteBatch`.
5. **Read-only packet invariants require zero new Go types.** Judgment packets specify `read_only: true` in YAML frontmatter. `packet.Parse` and `run.enforceCompletionMode` (merged in `read-only-packet-dispatch`) already enforce that the worktree carries no unique commits and a clean porcelain tree. No new parser fields or completion hooks are needed.
6. **No schema churn in `result.schema.json`.** Judgment packets report qualitative verdicts using standard `status`, `summary`, and `findings` (`finding`, `evidence`, `affects`) fields with `commit` omitted. `result.schema.json` is not modified.

---

## Constraints (Do Not)

- **Do NOT modify `internal/integrate/integrate.go`.** `Check`, `Combine`, and `Promote` are reused unmodified. Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand`.
- **Do NOT modify `internal/run/run.go` or `internal/run/integrate.go`.** `ExecuteBatch`, `decideStatus`, and `enforceCompletionMode` are reused unmodified. Spec: `specs/verify-judgment-packet/spec.md#Requirement: Structural Cleanliness Enforcement via Git Porcelain`.
- **Do NOT modify `internal/ledger/ledger.go` or add new ledger event types.** SQLite schema and event types remain untouched. Spec: `specs/verify-dual-dispatch/spec.md#Requirement: Additive Rollback Without Ledger Migration`.
- **Do NOT modify `internal/result/result.schema.json`.** No new verdict properties (e.g. `"verdict"`) or phase-specific extensions may be added. Spec: `specs/verify-judgment-packet/spec.md#Requirement: Existing Envelope Schema Reuse Without Churn`.
- **Do NOT modify `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md`.** It remains the human-tier template baseline. Spec: `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset`.
- **Do NOT execute `lucind-checks.sh` more than once per verify cycle.** Mechanical verification is deterministic; dual LLM lanes must evaluate the frozen artifact in context rather than re-running test commands. Spec: `specs/verify-mechanical-check/spec.md#Requirement: Single Deterministic Execution`.
- **Do NOT write Go code during this tasks phase.** This phase is strictly for sequencing and operationalizing the task checklist.

---

## Phase 1: `cmd/lucind-ai/cli.go` — `check` Subcommand and `--out` Flag

Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand`, `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact`.

- [ ] 1.1 **RED** `cmd/lucind-ai/cli_test.go`: `TestRunCheckMissingScriptFails`. Invoke `run(ctx, []string{"check"}, &stdout, &stderr)` in a temporary directory lacking `lucind-checks.sh`. Assert non-zero exit code (1), and assert `stderr` contains `"no lucind-checks.sh found at the project root"`. Expected failure: `run` returns 1 with `"lucind-ai: unknown subcommand \"check\""`.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand` scenario "Missing check script exits non-zero".
  Consumer: Consumed by task 1.2 (`runCheck` subcommand handler in `cli.go`).

- [ ] 1.2 **GREEN** `cmd/lucind-ai/cli.go`: update `run()` switch to add `case "check": return runCheck(ctx, args[1:], stdout, stderr)`. Implement `runCheck` to resolve candidate repository root via `resolvePrimaryRoot(ctx)` (or working directory), invoke `integrate.Check(ctx, root)`, and output missing-script message to `stderr` when `passed == false`. `go test ./cmd/lucind-ai -run TestRunCheckMissingScriptFails -count=1` green for 1.1.

- [ ] 1.3 **RED** `cmd/lucind-ai/cli_test.go`: `TestRunCheckScriptFails`. In a temporary git repository created via `initRepo`, write a failing `lucind-checks.sh` (`#!/bin/sh\necho "FAIL: linter error"\nexit 1\n`, chmod 0755). Invoke `run(ctx, []string{"check"}, &stdout, &stderr)`. Assert exit code 1, and assert combined output printed to `stderr`/`stdout` contains `"FAIL: linter error"`. Skip if `testing.Short()`.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand` scenario "Failing mechanical check exits non-zero".
  Consumer: Consumed by task 1.4 (`runCheck` exit code handling).

- [ ] 1.4 **GREEN** `cmd/lucind-ai/cli.go`: in `runCheck`, when `integrate.Check` returns `passed == false` from a non-zero exit code, print check output to `stderr`/`stdout` and return exit code 1. `go test ./cmd/lucind-ai -run TestRunCheckScriptFails -count=1` green.

- [ ] 1.5 **RED** `cmd/lucind-ai/cli_test.go`: `TestRunCheckScriptPasses`. In a temporary git repository created via `initRepo`, write a passing `lucind-checks.sh` (`#!/bin/sh\necho "PASS: all suites clean"\nexit 0\n`, chmod 0755). Invoke `run(ctx, []string{"check"}, &stdout, &stderr)`. Assert exit code 0, and assert `stdout` contains `"PASS: all suites clean"`, execution status `"passed"`, execution duration, and git commit SHA from `git rev-parse HEAD`. Skip if `testing.Short()`.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand` scenario "Passing mechanical check exits 0".
  Consumer: Consumed by task 1.6 (`runCheck` passing output formatting).

- [ ] 1.6 **GREEN** `cmd/lucind-ai/cli.go`: in `runCheck`, when `integrate.Check` returns `passed == true`, extract candidate git commit SHA via `git rev-parse HEAD`, calculate execution duration, print output transcript and summary line (`status: passed`, duration, commit SHA) to `stdout`, and return exit code 0. `go test ./cmd/lucind-ai -run TestRunCheckScriptPasses -count=1` green.

- [ ] 1.7 **RED** `cmd/lucind-ai/cli_test.go`: `TestRunCheckOutFlagWritesLogFile` and `TestRunCheckOutFlagOverwritesExistingLog`.
  - In a temporary git repository with a passing `lucind-checks.sh`, invoke `run(ctx, []string{"check", "--out", logPath}, &stdout, &stderr)`. Assert exit code 0. Read `logPath` from disk; assert it contains structured header (`=== lucind-ai mechanical check ===`, `Git Commit SHA:`, `Duration:`, `Exit Code: 0`) followed by `"PASS: all suites clean"`.
  - Re-run `run(ctx, []string{"check", "--out", logPath}, &stdout, &stderr)` with a modified script output (`"PASS: second run"`). Assert `logPath` is cleanly overwritten and contains `"PASS: second run"`.
  - Skip if `testing.Short()`.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact` scenarios "Log file written with structured header", "Existing log file overwritten".
  Consumer: Consumed by task 1.8 (`--out` flag parsing and file writer).

- [ ] 1.8 **GREEN** `cmd/lucind-ai/cli.go`: add `--out` string flag to `flag.FlagSet("check", flag.ContinueOnError)` in `runCheck`. When `--out` is specified, format the complete record (structured metadata header + combined output transcript), ensure parent directory exists via `os.MkdirAll(filepath.Dir(outPath), 0o755)`, and write to `outPath` via `os.WriteFile(outPath, []byte(content), 0o644)`. `go test ./cmd/lucind-ai -run "TestRunCheckOutFlag.*" -count=1` green.

- [ ] 1.9 **RED** `cmd/lucind-ai/cli_test.go`: `TestRunCheckUsageAndUnknownFlags`.
  - Invoke `run(ctx, []string{"check", "--bogus"}, &stdout, &stderr)`. Assert exit code 1 and usage printed.
  - Invoke `run(ctx, nil, &stdout, &stderr)`. Assert `stderr` usage text documents `check` subcommand: `"usage: lucind-ai run --packet <path> ...\n       lucind-ai check [--out <path>]"`.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand`.
  Consumer: Consumed by task 1.10 (usage constant update).

- [ ] 1.10 **GREEN** `cmd/lucind-ai/cli.go`: update `usage` constant in `cli.go` to document `lucind-ai check [--out <path>]` alongside `run`. `go test ./cmd/lucind-ai -count=1` green.

---

## Phase 2: `verify-mechanical.log` Artifact Capture and Worktree Inheritance

Spec: `specs/verify-mechanical-check/spec.md#Requirement: Candidate Branch Pre-Commit and Worktree Inheritance`, `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Failure Short-Circuit`, `specs/verify-mechanical-check/spec.md#Requirement: Terminal Consumers of the Mechanical Log Artifact`.

- [ ] 2.1 **RED** `cmd/lucind-ai/cli_test.go`: `TestFormatMechanicalLogHeader`. Unit test for log formatter helper `formatMechanicalLog(commitSHA string, exitCode int, duration time.Duration, output string) string`. Assert resulting string starts with header block containing 40-character hex commit SHA, exact exit code integer, formatted duration string, and command line `lucind-checks.sh`, followed by the raw transcript.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact` scenario "Log file written with structured header".
  Consumer: Consumed by task 2.2 (`formatMechanicalLog` helper in `cli.go`).

- [ ] 2.2 **GREEN** `cmd/lucind-ai/cli.go`: extract `formatMechanicalLog` helper in `cli.go` and consume it in `runCheck` for `--out` file writing. `go test ./cmd/lucind-ai -run TestFormatMechanicalLogHeader -count=1` green.

- [ ] 2.3 **RED** `internal/worktree/worktree_test.go`: `TestLinkedWorktreeInheritsCommittedMechanicalLog`.
  - In a temporary git repository initialized via `initRepo`, create directory `openspec/changes/verify-test/`.
  - Write `openspec/changes/verify-test/verify-mechanical.log` with sample check output.
  - Commit the log to `main` (`git add` + `git commit -m "chore: record mechanical check"`).
  - Call `wt, err := worktree.Create(ctx, primaryRoot, "verify-lane-test")`.
  - Read `filepath.Join(wt.Path, "openspec/changes/verify-test/verify-mechanical.log")`.
  - Assert the log file is present in the linked worktree with identical content, with zero custom file copying.
  - Skip if `testing.Short()`.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Candidate Branch Pre-Commit and Worktree Inheritance` scenario "Linked worktrees inherit the log automatically".
  Consumer: Consumed by task 2.4 (worktree inheritance validation).

- [ ] 2.4 **GREEN** `internal/worktree/worktree_test.go`: test passes against existing `worktree.Create` implementation, proving git branch inheritance satisfies log delivery without cross-worktree filesystem coupling. `go test ./internal/worktree -run TestLinkedWorktreeInheritsCommittedMechanicalLog -count=1` green.

---

## Phase 3: Read-Only Judgment Packet Template Asset and Pointer Note

Spec: `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Judgment Packet Frontmatter`, `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Done-Criteria Contract`, `specs/verify-judgment-packet/spec.md#Requirement: Existing Envelope Schema Reuse Without Churn`, `specs/verify-judgment-packet/spec.md#Requirement: Mechanical Re-Run Prohibition Contract`, `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Tool Selection Guidance`, `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset`.

- [ ] 3.1 **RED** `internal/packet/packet_test.go`: `TestVerifyPacketTemplateAssetStructure`.
  - Read `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` from the repository root.
  - Pass content to `packet.Parse(strings.NewReader(content))`.
  - Assert `Packet.ReadOnly == true`.
  - Assert body contains exact frontmatter skeleton with `read_only: true`.
  - Assert body contains the three read-only done criteria:
    1. "Every indirection introduced is demonstrably consumed by a terminal consumer."
    2. "The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`)."
    3. "Qualitative evaluation completed" (`.lucind/result.json` populated with `status`, `summary`, and structured `findings`).
  - Assert body does NOT contain a write-packet commit done criterion (`"The work is committed"`).
  - Assert `## Out of scope` explicitly forbids executing `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suite.
  - Assert `## Hard stops` contains exact hard stop string: `"Executing mechanical test/build commands when mechanical results are already provided."`
  - Assert `## Context` contains sections for embedding frozen mechanical log transcript and summary.
  - Assert tool-selection guidance instructs using read/navigation tools (`Read`, `Glob`, `Grep`, `codegraph`) and read-only git queries (`git diff`, `git log`, `git show`).
  Spec: `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Judgment Packet Frontmatter`, `specs/verify-judgment-packet/spec.md#Requirement: Read-Only Done-Criteria Contract`, `specs/verify-judgment-packet/spec.md#Requirement: Mechanical Re-Run Prohibition Contract`, `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset`.
  Consumer: Consumed by task 3.2 (template asset creation).

- [ ] 3.2 **GREEN** `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md`: author the standardized verify judgment packet template asset containing all required frontmatter, sections, done-criteria, hard stops, context placeholders, tool guidance, and envelope return instructions. `go test ./internal/packet -run TestVerifyPacketTemplateAssetStructure -count=1` green for 3.1.

- [ ] 3.3 **RED** `internal/packet/packet_test.go`: `TestPacketTemplateVerifyPointerNote`.
  - Read `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`.
  - Assert file contains a pointer note referencing `verify-packet-template.md` for qualitative verification lanes.
  Spec: `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset` scenario "Packet template reference note".
  Consumer: Consumed by task 3.4 (`packet-template.md` pointer note).

- [ ] 3.4 **GREEN** `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`: add note in `packet-template.md` under Frontmatter / Done Criteria directing authors of qualitative verification packets to `verify-packet-template.md`. `go test ./internal/packet -run TestPacketTemplateVerifyPointerNote -count=1` green.

- [ ] 3.5 **RED** `internal/result/result_test.go`: `TestVerifyResultEnvelopeSchemaCompliance`.
  - Construct a valid verify judgment envelope JSON:
    ```json
    {
      "packet_id": "verify-sample-cursor-agent",
      "status": "done",
      "summary": "VERDICT: PASS. Implementation satisfies all spec requirements in specs/sample/spec.md. Mechanical checks passed cleanly.",
      "hard_stops": [
        {"hard_stop": "Executing mechanical test/build commands when mechanical results are already provided.", "fired": false}
      ],
      "findings": [
        {"finding": "Non-blocking documentation typo in helper comment", "evidence": "internal/sample/sample.go:42", "affects": "Documentation only"}
      ],
      "done_criteria": [
        {"criterion": "no unique commits, clean tree", "met": true, "evidence": "git status --porcelain empty; HEAD == merge-base"}
      ]
    }
    ```
  - Validate JSON against `result.SchemaJSON()`. Assert validation succeeds with zero errors (verifying `commit` omission is valid).
  - Add unauthorized top-level property `"verdict": "pass"` to the JSON. Validate against `result.SchemaJSON()`. Assert validation fails due to `additionalProperties: false`.
  Spec: `specs/verify-judgment-packet/spec.md#Requirement: Existing Envelope Schema Reuse Without Churn` scenarios "Standard envelope validation without commit", "Additional verdict properties rejected", "Findings report file and line evidence".
  Consumer: Consumed by task 3.6 (envelope schema validation).

- [ ] 3.6 **GREEN** `internal/result/result_test.go`: test passes against existing `result.schema.json` without schema modifications, proving the envelope contract is preserved without churn. `go test ./internal/result -run TestVerifyResultEnvelopeSchemaCompliance -count=1` green.

---

## Phase 4: `SKILL.md` Documentation — Two-Stage Verify Protocol, Evidence Cross-Checking, and Reconciliation

Spec: `specs/verify-dual-dispatch/spec.md#Requirement: Two-Stage SDD Verify Protocol`, `specs/verify-dual-dispatch/spec.md#Requirement: Dual Parallel Judgment Dispatch and Barrier Join`, `specs/verify-dual-dispatch/spec.md#Requirement: Mechanical Log Context Embedding`, `specs/verify-dual-dispatch/spec.md#Requirement: Independent Evidence Cross-Checking`, `specs/verify-dual-dispatch/spec.md#Requirement: Unanimous Pass Reconciliation`, `specs/verify-dual-dispatch/spec.md#Requirement: Disagreement and False-Positive Adjudication`, `specs/verify-dual-dispatch/spec.md#Requirement: Lane Execution Failure Handling`, `specs/verify-dual-dispatch/spec.md#Requirement: Irreconcilable Ambiguity Escalation`, `specs/verify-dual-dispatch/spec.md#Requirement: Canonical Verification Report and State Update`, `specs/verify-dual-dispatch/spec.md#Requirement: Additive Rollback Without Ledger Migration`.

- [ ] 4.1 **RED** `internal/packet/packet_test.go`: `TestSkillMDVerifyOperationalWorkflow`.
  - Read `plugin/claude-code/skills/lucind-ai/SKILL.md`.
  - Assert row 80 in SDD lifecycle table describes `verify` as operational two-stage dispatch (Stage 1 mechanical check run once via `lucind-ai check`; Stage 2 qualitative judgment dual-dispatched to `agy` + `cursor-agent`).
  - Assert "Target direction, not yet built" table no longer contains a `verify` blocker row.
  - Assert `SKILL.md` documents the complete 3-stage verification procedure:
    1. **Stage 1: Mechanical Check**: `lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log`. Halts immediately if checks fail. Commits log to candidate branch `HEAD` on pass.
    2. **Stage 2: Dual Qualitative Judgment Dispatch**: Authors `packets/verify-<id>-agy.md` and `packets/verify-<id>-cursor-agent.md` using `verify-packet-template.md` (`read_only: true`, frozen mechanical summary in `## Context`). Dispatches in parallel with `lucind-ai run --packet packets/verify-<id>-agy.md --packet packets/verify-<id>-cursor-agent.md`. Barrier joins when both lanes reach terminal status.
    3. **Stage 3: Evidence Cross-Checking & Verdict Reconciliation**:
       - Reads both `.lucind/result.json` envelopes.
       - Independently verifies every cited `file:line` against real codebase (`SKILL.md:102`).
       - Four-case reconciliation:
         - **Unanimous Pass** (`done`/`done`): Synthesizes `openspec/changes/<id>/verify.md` with overall status `PASSED`, consolidates complementary findings, updates `state.yaml` to `verify: { status: done }`.
         - **Disagreement / Disputed Defects** (`blocked`/`deviated`): Confirmed spec violations mark overall verdict `BLOCKED` with remediation tasks in `state.yaml`; demonstrable false positives are refuted with concrete `file:line` evidence in `verify.md` without blocking.
         - **Lane Failure** (`failed` due to timeout/infra): Re-dispatches the single failing lane before synthesis.
         - **Irreconcilable Ambiguity**: Contradictory interpretations of underspecified requirements unresolvable from specs/design set overall verdict `BLOCKED` and escalate decision options to human.
  Spec: `specs/verify-dual-dispatch/spec.md` (all requirements).
  Consumer: Consumed by task 4.2 (`SKILL.md` update).

- [ ] 4.2 **GREEN** `plugin/claude-code/skills/lucind-ai/SKILL.md`: update row 80, remove verify from unbuilt blocker table, and add complete operational two-stage verify dispatch, evidence cross-checking, 4-case reconciliation, and canonical report synthesis procedures. `go test ./internal/packet -run TestSkillMDVerifyOperationalWorkflow -count=1` green for 4.1.

- [ ] 4.3 **RED** `internal/packet/packet_test.go`: `TestHumanPacketTemplateUntouched`.
  - Verify `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md` is unmodified and matches git `HEAD`.
  Spec: `specs/verify-judgment-packet/spec.md#Requirement: Standardized Verify Packet Template Asset`.
  Consumer: Consumed by task 4.4 (regression assertion).

- [ ] 4.4 **GREEN** `internal/packet/packet_test.go`: test passes, verifying human packet template remains untouched. `go test ./internal/packet -run TestHumanPacketTemplateUntouched -count=1` green.

---

## Phase 5: End-to-End Mechanical Check and Dual Read-Only Dispatch Verification

Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand`, `specs/verify-dual-dispatch/spec.md#Requirement: Two-Stage SDD Verify Protocol`, `specs/verify-dual-dispatch/spec.md#Requirement: Dual Parallel Judgment Dispatch and Barrier Join`.

- [ ] 5.1 **RED** `cmd/lucind-ai/cli_test.go`: `TestRunCheckEndToEndWithRealScript`.
  - In a temporary git repository initialized via `initRepo`, write a real executable shell script `lucind-checks.sh`:
    ```sh
    #!/bin/sh
    set -e
    echo "BUILD: ok"
    echo "TESTS: ok"
    ```
  - Run `run(ctx, []string{"check", "--out", filepath.Join(repoDir, "openspec/changes/e2e/verify-mechanical.log")}, &stdout, &stderr)`.
  - Assert exit code 0.
  - Assert log file exists at `openspec/changes/e2e/verify-mechanical.log` and contains git commit SHA, duration, `Exit Code: 0`, and the check transcript.
  - Skip if `testing.Short()`.
  Spec: `specs/verify-mechanical-check/spec.md#Requirement: Mechanical Check CLI Subcommand`, `specs/verify-mechanical-check/spec.md#Requirement: Output Capture to Durable Log Artifact`.
  Consumer: Consumed by task 5.2 (end-to-end CLI check verification).

- [ ] 5.2 **GREEN** `cmd/lucind-ai/cli.go`: verified by end-to-end test execution. `go test ./cmd/lucind-ai -run TestRunCheckEndToEndWithRealScript -count=1` green.

- [ ] 5.3 **End-to-End Dual Read-Only Verify Dispatch Verification**:
  - Author two verify judgment packets with `read_only: true` targeting a candidate repository where `verify-mechanical.log` is committed.
  - Dispatch via `lucind-ai run --packet ... --packet ...`.
  - Assert both lanes complete with `lane.Done`, zero unique commits on their branches, and empty `git status --porcelain`.
  - Assert orchestrator synthesizes `verify.md` and updates `state.yaml` cleanly.
  Spec: `specs/verify-dual-dispatch/spec.md#Requirement: Dual Parallel Judgment Dispatch and Barrier Join`.

- [ ] 5.4 **Full Test Suite & Regression Sweep**:
  - Run `go test ./... -race -count=1`.
  - Assert all packages pass cleanly.
  - Verify `git status --porcelain` is clean with zero untracked artifacts.
  - Verify zero database migrations or ledger changes were introduced.

---

## Review Budget & Delivery Strategy Validation

- **Estimated changed lines**: 740–1090 lines.
- **Budget comparison**: Well below the 2000-line budget in `state.yaml` (`review_budget_lines: 2000`).
- **Delivery strategy**: `delivery_strategy: single-pr` in `state.yaml` is fully appropriate and at zero risk of exceeding review limits. No chained PR split is required.
