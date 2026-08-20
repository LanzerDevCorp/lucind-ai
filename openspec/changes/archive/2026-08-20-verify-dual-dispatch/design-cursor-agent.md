# Design: Verify-Phase Dual Dispatch

Split `sdd-verify` into (1) a single deterministic mechanical-check execution and (2) two parallel, independent read-only judgment packets (`agy` and `cursor-agent`) evaluating qualitative correctness, spec conformance, edge cases, and test quality. The mechanical-check output is frozen and provided as context to both judgment lanes, eliminating duplicate check execution costs and test flakiness. The orchestrator synthesizes the two independent verdicts into a single canonical verification report (`verify-report.md`), matching the battle-tested dual-dispatch synthesis pattern already used for propose, design, specs, and tasks.

This design builds directly upon `openspec/changes/read-only-packet-dispatch/design.md` (canonical `Packet.ReadOnly` and `enforceCompletionMode` invariants) without redesigning packet completion invariants.

## Recommendations at a glance

| # | Question | Recommendation | Source / Rationale |
|---|---|---|---|
| 1 | Where does the mechanical check run and live? | Run once via a new CLI helper `lucind-ai check` (invoking `internal/integrate.Check`). Output is captured to `openspec/changes/<id>/verify-mechanical.log` and embedded in the judgment packet's `## Context` section. | Prevents duplicate test runs; provides both on-disk durable receipt and instant in-prompt context. |
| 2 | What is the judgment packet shape and done-criteria? | Frontmatter sets `read_only: true`. Mandatory criterion 2 is the exact read-only invariant from `read-only-packet-dispatch/design.md` Decision 2 (no unique commits, clean porcelain). Findings and qualitative verdict return in `.lucind/result.json`. | Reuses canonical read-only packet invariant; avoids mutating git history with ephemeral draft branches. |
| 3 | How is mechanical re-running forbidden and enforced? | Explicit prohibition in packet `## Out of scope` and `## Hard stops`; enforced structurally by `enforceCompletionMode` (any build/test artifact left in the tree fails porcelain cleanliness). | Prevents duplicate CPU/quota waste while preserving necessary read/analysis tooling. |
| 4 | How are dual verdicts reconciled? | The orchestrator synthesizes a single canonical `verify-report.md`, deduplicating findings, validating evidence against code, and escalating to the human only if irreconcilable conflicts emerge. | Follows propose/design/specs/tasks precedent; leverages complementary reviewer strengths rather than halting on single false positives. |
| 5 | Rollback plan? | Purely additive: new CLI helper, template asset, and SKILL.md updates. Zero ledger migrations; revert apply commits. | Clean, zero-state rollback. |

---

## Decision 1 — Mechanical Check Execution and Output Storage

**Choice**: A single, deterministic run of `lucind-checks.sh` executed before dispatching qualitative judgment packets. A new CLI subcommand `lucind-ai check [--out <path>]` provides a standardized entry point wrapping `internal/integrate.Check(ctx, targetPath)` (`internal/integrate/integrate.go:79`).

```
Candidate Branch / HEAD -> lucind-ai check -> openspec/changes/<id>/verify-mechanical.log
                                            -> Embedded into packets/verify-*.md Context
```

The mechanical-check output (command line, exit code, execution duration, git commit SHA, stdout/stderr transcript) is written to:
`openspec/changes/<change-id>/verify-mechanical.log`

In addition, the orchestrator extracts the summary status and relevant test output sections directly into the `## Context` section of both judgment packets (`verify-<change-id>-agy.md` and `verify-<change-id>-cursor-agent.md`).

**Why this location**:
1. **Durable Receipt**: Storing `verify-mechanical.log` within the change directory ensures an immutable, version-controlled audit record of deterministic checks alongside the qualitative review report.
2. **Zero-Tooling In-Prompt Context**: Embedding the log summary directly in the packet body gives both executors immediate access to the build/test outcomes without requiring extra tool round-trips or file parsing.
3. **Worktree Inherited Access**: Because worktrees are created from primary `HEAD` (or the candidate integration branch), committing or staging `verify-mechanical.log` prior to worktree creation makes the full log available locally at `openspec/changes/<id>/verify-mechanical.log` inside each worktree if deeper inspection is required.

### Rejected Alternatives

- **Re-running mechanical checks inside each judgment worktree**: Rejected. This is the exact anti-pattern this change exists to eliminate. Running `go test ./... -race` inside two LLM executor worktrees doubles CPU and execution time, risks non-deterministic timing flakiness across concurrent runs, and adds zero qualitative signal.
- **Writing output only to `/tmp` or process stdout**: Rejected. `/tmp` violates repository worktree locality (`docs/prd.md:104-105`), is volatile across system reboots or container boundaries (e.g. WSL2 environments), and fails to provide a persistent audit trail for RDD or human reviewers.
- **Embedding mechanical verification into `ExecuteBatch` inside `internal/run`**: Rejected. `ExecuteBatch` is a generic parallel lane runner. Hardcoding verification-phase workflows into `ExecuteBatch` would tightly couple binary runtime execution to SDD lifecycle rules, violating the separation of concerns established in `docs/prd.md` §8.4.

---

## Decision 2 — Judgment Packet Frontmatter, Shape, and Read-Only Done-Criteria

**Choice**: Judgment packets are declared as `read_only: true` in YAML frontmatter, consuming the mechanism established by `openspec/changes/read-only-packet-dispatch/design.md`.

### Packet Frontmatter

```yaml
---
id: verify-<change-id>-<executor>
executor: agy # or cursor-agent
routed_by: qualitative verification of spec compliance, edge cases, and test quality
model: gemini-3.7-flash-high # or cursor-grok-4.6-high
read_only: true
---
```

### Done-Criteria Definition

Because the judgment packet is `read_only: true`, mandatory criterion 2 no longer requires a git commit. In accordance with `read-only-packet-dispatch/design.md` Decision 2, criterion 2 is restated exactly as:

- [ ] **The worktree carries no unique commits and no working-tree changes relative to the lane's birth point.** Evidence: `git status --porcelain` empty **and** the worktree's `HEAD` equals `git merge-base HEAD <primary HEAD>`.

The judgment packet's criteria set is:

1. **Every indirection introduced is demonstrably consumed by a terminal consumer.** (Evidence: verification citations trace to concrete symbols, tests, and spec requirements).
2. **The worktree carries no unique commits and no working-tree changes relative to the lane's birth point.** (Evidence: `git status --porcelain` empty and `HEAD` equals merge base).
3. **Qualitative evaluation completed.** (Evidence: `.lucind/result.json` populated with `status: done`, `summary`, and structured `findings`).

### Verdict Envelope Shape

The executor outputs its verdict directly into `.lucind/result.json`. Because `.lucind/` is gitignored, writing this file does not dirty the git working tree and passes `enforceCompletionMode`:

```json
{
  "packet_id": "verify-auth-session-cursor-agent",
  "status": "done",
  "summary": "VERDICT: PASS. Implementation satisfies all 4 spec requirements in specs/auth/spec.md. Edge case on token expiration during refresh is properly handled in internal/auth/session.go:84. Mechanical checks passed cleanly.",
  "hard_stops": [
    {
      "hard_stop": "Executing mechanical test suites or build commands when mechanical results are already provided.",
      "fired": false
    },
    {
      "hard_stop": "Any credential value would need to be chosen, generated, or written.",
      "fired": false
    }
  ],
  "findings": [
    {
      "finding": "Missing negative test case for malformed bearer token prefix",
      "evidence": "internal/auth/session_test.go:142",
      "affects": "Non-blocking coverage gap; recommended for follow-up"
    }
  ],
  "done_criteria": [
    {
      "criterion": "The worktree carries no unique commits and no working-tree changes relative to the lane's birth point.",
      "met": true,
      "evidence": "git status --porcelain is empty; HEAD equals merge-base"
    },
    {
      "criterion": "Qualitative evaluation completed.",
      "met": true,
      "evidence": "Report and findings recorded in envelope"
    }
  ]
}
```

### Rejected Alternatives

- **Write packets committing `verify-<executor>.md` to lane branches**: Rejected. Verification is an inspection phase, not a code generation phase. Merging draft verify documents to git branches pollutes git history with ephemeral review notes, triggers unnecessary branch merge operations in `Combine`, and requires commit overhead for what is fundamentally read-only analysis.
- **Adding new custom verdict fields to `result.schema.json`**: Rejected. The existing schema (`result.schema.json`) already provides `status`, `summary`, `findings` (with `finding`, `evidence`, `affects`), and `questions`. Reusing these fields avoids schema churn and keeps tooling universal across all SDD phases.

---

## Decision 3 — Mechanical Check Re-run Prohibition and Enforcement

**Risk**: A judgment packet's worktree is a full checkout of the implementation. If the executor runs `go test ./...`, `go vet`, or `sh lucind-checks.sh` "to double-check," it silently reintroduces the duplicate-execution overhead, potential quota burn, and test race conditions that this change exists to eliminate.

**Choice**: Prohibit mechanical re-runs through prompt contracts and hard stops, backed by git porcelain cleanliness enforcement.

### 1. Packet Prompt & Hard Stop Contract

The verify packet template explicitly defines re-running mechanical checks as out of scope and a hard stop:

```markdown
## Out of scope

Do NOT execute `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suites.
Deterministic mechanical checks have already executed once; their frozen output is provided in
`## Context`. Re-running them wastes execution quota and adds no new signal.

## Hard stops

- Executing mechanical test suites or build commands (`go test`, `go build`, `go vet`, `sh lucind-checks.sh`)
  when mechanical check results are already provided in Context.
```

### 2. Binary Invariant Enforcement via `enforceCompletionMode`

Under `read-only-packet-dispatch/design.md`, every `read_only: true` lane must satisfy `PorcelainEmpty(ctx, worktreePath)` to achieve `lane.Done`.
In Go and common test toolchains, running test suites often creates untracked artifacts (e.g. `coverage.out`, test binaries, compiled test fixtures, temporary test databases). Any untracked or modified file left behind causes `enforceCompletionMode` to fail the lane (`lane.Failed`).

### 3. Tool Selection Guidance

The packet prompt instructs executors to restrict their investigation to read and navigation tools (`Read`, `Glob`, `Grep`, `codegraph`, `ctx_execute_file` for analysis), discouraging shell execution beyond read-only git queries.

### Rejected Alternatives

- **Removing execution permissions (`chmod -x`) or deleting `lucind-checks.sh` in the worktree**: Rejected. Modifying repository files in the worktree creates working-tree diffs that would violate porcelain cleanliness checks, and may break static analysis tools or linters that inspect build configurations.
- **Disabling shell/terminal tool access entirely in executors**: Rejected. Executors require basic shell access for internal execution mechanisms, executing read-only git queries (`git status`), and tool environment operation.

---

## Decision 4 — Orchestrator Reconciliation of Dual Verdicts into a Canonical Verify Report

**Choice**: The orchestrator (Claude Code) synthesizes a single canonical verification report (`openspec/changes/<change-id>/verify-report.md`) from the two returned envelopes, matching the pattern established for propose, design, specs, and tasks.

```
[ mechanical-check.log ]
          +
[ agy result.json ]        -> Orchestrator Synthesis -> openspec/changes/<id>/verify-report.md
          +                                                    |
[ cursor-agent result.json ]                                   v
                                                     PASS -> state.yaml: verify done
                                                     FAIL -> state.yaml: corrective tasks
```

### Reconciliation Workflow

1. **Envelope Harvesting**: Orchestrator inspects the `.lucind/result.json` envelopes from both lanes upon `lucind-ai run` completion.
2. **Evidence Cross-Checking**: The orchestrator independently verifies every reported finding against the codebase (`SKILL.md:102`: *"Green criteria are not proof of complete work; verify evidence independently against the codebase"*).
3. **Finding Classification**:
   - **Confirmed Defect (Blocking)**: Verified spec violation, unhandled failure mode, or critical logic bug. Results in an overall verification verdict of `FAIL`, requiring corrective tasks in `apply` or a return to `design`.
   - **Valid Improvement (Non-Blocking)**: Minor code style, test coverage enhancement, or optimization opportunity that does not violate spec contracts. Recorded as advisory in the report.
   - **False Positive / Refuted Finding**: Finding disproven by inspecting the code (e.g. an executor claims a check is missing, but it is handled in an upstream middleware). Refuted with explicit code evidence in the report.
4. **Canonical Report Generation**: Orchestrator writes `openspec/changes/<change-id>/verify-report.md` documenting:
   - Mechanical check summary (pass/fail, duration, coverage).
   - Qualitative consensus items (where both judges agree).
   - Resolved discrepancies (arbitration of conflicting findings).
   - Final Verification Verdict: `PASS` or `FAIL`.
5. **State Update**:
   - If `PASS`: `state.yaml` updates `verify: { status: done, completed_at: ... }`.
   - If `FAIL`: `state.yaml` updates `verify: { status: failed }` and links to the generated corrective tasks.
6. **Escalation on Genuine Ambiguity**: If a fundamental architectural disagreement arises where the two executors interpret an ambiguous spec requirement in contradictory ways and the orchestrator cannot resolve it from existing docs, the orchestrator sets `status: blocked` and presents the decision to the human via `AskQuestion`.

### Rejected Alternatives

- **Mechanical Unanimous Gate (Disagreement automatically halts and blocks)**: Rejected. LLM reviewers frequently produce complementary observations (e.g. `agy` spots a concurrency edge case; `cursor-agent` catches a subtle off-by-one error). A naive unanimous voting rule treats any difference in output as a fatal collision, triggering unnecessary human escalations over benign non-blocking nits or single false positives. The orchestrator's role is specifically to synthesize evidence-backed artifacts and filter noise.
- **Single-Executor Verification**: Rejected. Single-executor review reintroduces single-model blind spots. Dual dispatch across Antigravity (`gemini-3.7-flash-high`) and Cursor (`cursor-grok-4.6-high`) provides cross-family perspective and redundancy.

---

## Decision 5 — Rollback and Compatibility

**Choice**: Purely additive changes with zero database or schema migrations.

| Layer | Rollback Behavior |
|---|---|
| `lucind-ai check` CLI command | Revert the CLI command in `cmd/lucind-ai/cli.go`. |
| Packet templates | Revert template additions in `plugin/claude-code/skills/lucind-ai/assets/`. |
| Skill documentation | Revert `plugin/claude-code/skills/lucind-ai/SKILL.md` verify workflow instructions. |
| Ledger / SQLite | **Zero impact**. No SQLite schema changes, no new event types. Existing lane and barrier tables remain untouched. |

Reverting the apply commit completely restores today's manual verification workflow. Existing packets and historical runs are unaffected.

---

## Terminal Consumers and Indirection Trace

Every new indirection introduced by this design is demonstrably consumed by an explicit downstream caller:

| Introduced Symbol / Asset | Exact Type / Signature | Direct Caller / Producer | Terminal Consumer |
|---|---|---|---|
| `lucind-ai check` CLI command | CLI command in `cmd/lucind-ai/cli.go` calling `integrate.Check` | Orchestrator / User via shell | Executes `internal/integrate.Check`, captures exit code/output, writes `verify-mechanical.log`. |
| `verify-mechanical.log` | File at `openspec/changes/<id>/verify-mechanical.log` | Written by `lucind-ai check` | Read by orchestrator to populate `## Context` in judgment packets and attached to `verify-report.md`. |
| Verify Judgment Packet Template | `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` | Authored by orchestrator | Dispatched to `lucind-ai run` as input packet for `agy` and `cursor-agent`. |
| `Packet.ReadOnly: true` | Bool field on `packet.Packet` | Parsed by `packet.Parse` from packet frontmatter | Read by `run.enforceCompletionMode` to verify 0 commits and clean working tree. |
| Judgment Envelope Findings | `findings` array in `.lucind/result.json` | Written by `agy` / `cursor-agent` in worktrees | Read by orchestrator to assemble the consolidated findings section of `verify-report.md`. |
| Canonical `verify-report.md` | Markdown file at `openspec/changes/<id>/verify-report.md` | Authored by orchestrator | Read by human reviewer and `gentle-ai` delivery gate to confirm change completion. |

---

## Data Flow

```
1. PREPARATION
   Primary HEAD / Candidate Branch
         |
         v
   [ lucind-ai check ] -------------> Executes internal/integrate.Check(ctx, primaryRoot)
         |                            Runs lucind-checks.sh (build, test, vet)
         |
         +--------------------------> Writes: openspec/changes/<id>/verify-mechanical.log
         |
2. DUAL PACKET DISPATCH
   Orchestrator authors:
   - packets/verify-agy.md          (read_only: true, Context: mechanical log summary)
   - packets/verify-cursor-agent.md (read_only: true, Context: mechanical log summary)
         |
         v
   [ lucind-ai run --packet packets/verify-agy.md --packet packets/verify-cursor-agent.md ]
         |
         +---> Lane 1 (agy):          ../worktrees/verify-agy          (read-only inspection)
         +---> Lane 2 (cursor-agent): ../worktrees/verify-cursor-agent (read-only inspection)
         |
3. COMPLETION & ENFORCEMENT
   Each lane writes .lucind/result.json (status, summary, findings)
         |
         v
   run.enforceCompletionMode:
   - HasUniqueLaneCommits == false (0 commits)
   - PorcelainEmpty == true (clean working tree)
         |
         v
   lucind-ai run exits 0
         |
4. RECONCILIATION & SYNTHESIS
   Orchestrator reads both .lucind/result.json envelopes
   Orchestrator cross-checks evidence against codebase
   Orchestrator synthesizes: openspec/changes/<id>/verify-report.md
         |
         +---> PASS: Update state.yaml (verify: done) -> Ready for delivery
         +---> FAIL: Update state.yaml (verify: failed) -> Corrective tasks created
```

---

## File Changes (Apply Phase — Not This Design Packet)

| File | Action | Description |
|---|---|---|
| `cmd/lucind-ai/cli.go` | Modify | Add `check` subcommand invoking `internal/integrate.Check` and formatting mechanical results. |
| `cmd/lucind-ai/cli_test.go` | Modify | Unit and integration tests for `lucind-ai check` command handling, exit codes, and output capture. |
| `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` | Create | Standardized template for qualitative verify judgment packets (`read_only: true`, out-of-scope rules, hard stops). |
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | Modify | Add reference note pointing to `verify-packet-template.md` for qualitative verification lanes. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modify | Update verify phase row (`SKILL.md:80`) from "Target, not built" to canonical dual-dispatch verify protocol; add verify workflow instructions. |

`internal/run/integrate.go`, `internal/integrate/integrate.go`, and `internal/ledger/` receive **no modifications** — `internal/integrate.Check` and `run.enforceCompletionMode` are reused unmodified.

---

## Testing Strategy

| Layer | RED Test Case | GREEN Test Case |
|---|---|---|
| `lucind-ai check` CLI | Target worktree missing `lucind-checks.sh` or failing script returns non-zero exit code and reports failure output. | Clean `lucind-checks.sh` execution returns exit 0, outputs timing and test summary, and writes log file when `--out` is specified. |
| Verify Packet Parsing | Packet with invalid frontmatter or missing `read_only: true` is rejected by verification validator. | Frontmatter correctly parsed with `read_only: true`, `routed_by`, and executor configuration. |
| Enforcement on Verify Lanes | Verify judgment lane that attempts to commit or leaves dirty test output files fails `enforceCompletionMode` (`lane.Failed`). | Read-only verify lane with 0 commits and clean porcelain passes `enforceCompletionMode` (`lane.Done`). |
| Orchestrator Report Synthesis | Discrepant findings between two envelopes are flagged during report synthesis. | Valid findings from both envelopes are successfully consolidated into `verify-report.md`. |

---

## Threat Matrix

| Boundary | Applicability | Mitigation |
|---|---|---|
| Duplicate Mechanical Check Execution | Applicable | Explicit packet contract and hard stops prohibit re-running tests; `enforceCompletionMode` rejects untracked test output artifacts. |
| Flaky Test Contamination | Applicable | Mechanical checks run exactly once in a controlled environment; both qualitative judges evaluate the identical frozen test execution transcript. |
| State Mutation in Read-Only Lanes | Applicable | Binary enforces `HasUniqueLaneCommits == false` and `PorcelainEmpty == true` via `run.enforceCompletionMode`. |
| Hallucinated Review Findings | Applicable | Orchestrator independently cross-checks reported `evidence` (`file:line`) against the real codebase before accepting findings into canonical `verify-report.md`. |
| Silent Failure Concealment | Applicable | Every packet hard stop must be explicitly reported in the envelope `hard_stops` array; green criteria cannot hide an undeclared hard stop. |
| Ledger Schema Corruption | N/A | Zero SQLite schema modifications; standard ledger event logging is preserved without change. |

---

## Out of Scope (Owned by Sibling Changes or Deferred)

- **`read-only-packet-dispatch`**: Implementation of `Packet.ReadOnly`, `enforceCompletionMode`, and git worktree inspectors (`HasUniqueLaneCommits`, `PorcelainEmpty`) belongs to `read-only-packet-dispatch`.
- **`apply-dag-dispatch`**: Task splitting, dependency DAG resolution, and `AllowedPaths` diff-union enforcement belong to `apply-dag-dispatch`.
- **Automated Code Fixing in Verify**: Automatic code remediation during verification is out of scope; failing verification generates corrective apply tasks.
- **Third-Party Review Runtimes**: Integrating external reviewers outside `agy` and `cursor-agent` is deferred to future multi-executor milestones.
