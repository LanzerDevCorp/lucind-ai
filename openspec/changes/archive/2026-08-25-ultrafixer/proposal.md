# Proposal: Ultrafixer

Dispatched as a dedicated, bounded `agy` Lane within `lucind-ai`'s native packet execution engine, `ultrafixer` diagnoses and fixes pre-existing defects across any orchestrated project, anchoring on `base_sha` origin diffs, evaluating critical and per-branch blocking impact independently, and delivering isolated repair commits via `blocked` result envelopes for human-initiated CAS promotion. A standalone background agent, auto-integrating approvals daemon, or baseline test snapshotting are not viable.

## Intent

When a test, linter, or build check fails during feature development across any project orchestrated by `lucind-ai`, operators have no automated mechanism to diagnose, isolate, or repair the defect.

Today, `dependencies-defects.md` (`plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md:7-17`) describes an entirely manual defect contract: the human Orchestrator must manually classify origin, record evidence, block affected lanes, and prepare separate fix Changes. While `lucind-ai-fixer` exists as a Claude Code agent, it is strictly scoped to defects in `lucind-ai`'s own repository and must never be duplicated or modified to handle target projects.

`ultrafixer` fills this gap for any orchestrated project as an ephemeral, auditable `agy` Lane (`internal/packet/packet.go:33-75`; `cmd/lucind-ai/cli.go:82-87`; `internal/executor/agy.go:69-83,136-150`). It requires a formal protocol to:
1. Classify whether an error is pre-existing or introduced by the active feature via `base_sha` diffs (`internal/overlap/overlap.go:1007-1025`; `internal/resolve/candidate.go:97-115`).
2. Evaluate two independent axes: critical impact (security risk, data loss/corruption, build failure) and per-branch blocking impact across all active features discovered via `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`; `internal/serve/model.go:536-555`).
3. Remediate in an isolated worktree when critical or blocking, returning a `blocked` result envelope (`internal/result/result.go:77-82,102-115,122-134`) carrying the repair commit and decision recommendation for human CAS integration (`internal/run/integrate.go:159-165`; `cmd/lucind-ai/cli.go:1794-1815`), or record a durable Defect Record in ledger schema v8 (`internal/ledger/schema.go:10`) when neither.

## Selected Candidate and Approach

**Candidate 1 — Ephemeral Native `agy` Lane Dispatch with `base_sha` Origin Classification, Independent Two-Axis Evaluation, Isolated Worktree Repair, and Human-Gated CAS Integration.**

Lane-lifecycle hook: The feature Orchestrator dispatches an on-demand ultrafixer packet (`internal/packet/packet.go:33-75`) via `lucind-ai run --packet` (`cmd/lucind-ai/cli.go:116-125,169-174`). `ExecuteBatch` (`internal/run/batch.go:66-80`) executes the lane through `Agy.Run` (`internal/executor/agy.go:136-150`) with model `gemini-3.7-flash-high` (`:86-88`) in an isolated worktree.

1. **Manual Trigger & Ephemeral Dispatch.** Dispatched only when an Orchestrator observes a failing check. Each detected defect spawns a single, fresh, short-lived Lane. No persistent background daemons or unmonitored agent loops.
2. **Step 1 — Origin Classification.** Upon invocation, ultrafixer diffs the failing state against the feature branch's `base_sha` (`internal/packet/packet.go:68`; `internal/ledger/ledger.go:1342-1350`) using direct `git` subprocess shellouts (`internal/resolve/candidate.go:97-115`) and diff normalization logic (`internal/overlap/overlap.go:353-370,1007-1025`). If the defect was introduced by the feature's own changes, ultrafixer exits immediately without action (the feature lane remains responsible for fixing its own regressions). If the defect pre-existed before the feature branched, ultrafixer proceeds to triage.
3. **Step 2 — Two Independent Evaluation Axes.** For pre-existing defects, ultrafixer evaluates two distinct orthogonal criteria:
   - **Critical Axis:** Global assessment of whether the defect poses a security vulnerability, risk of data loss/corruption, or total CI/build breakage.
   - **Blocking Axis:** Evaluated independently per active feature branch. Ultrafixer discovers all active branches via CLI `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`; `internal/serve/model.go:536-555`; `internal/ledger/ledger.go:1353-1365`) and determines if the defect blocks progress on that specific branch.
4. **Cross-Branch Impact Candidate Filtering & Signal Reproduction.** Ultrafixer uses CodeGraph (`codegraph impact`/`codegraph affected`) as an external candidate filter to identify active features that touch relevant symbol or file dependencies. For each candidate branch, ultrafixer MUST reproduce the actual failure signal (e.g., executing the failing test, lint, or build command in that branch's context). Mere path or symbol overlap is explicitly insufficient to declare a branch blocked.
5. **Step 3 — Repair & Disposition per Branch.**
   - **Critical OR Blocking:** If critical globally, or blocking for a specific branch, ultrafixer creates an isolated repair branch/worktree, implements the minimal fix, runs tests, commits, and returns a schema-valid `blocked` result envelope (`internal/result/result.go:102-115,122-134`). The envelope contains the repair `Commit`, `Finding.Affects` (`:95-99`), and structured `Question` entries (`:77-82`) containing why it is blocking, candidate options, and a clear recommendation.
   - **Non-Critical AND Non-Blocking:** If the defect is neither critical nor blocking, ultrafixer touches no code and generates no fix Change. Instead, it persists a durable Defect Record in SQLite ledger schema v8 (`internal/ledger/schema.go:10`) recording failure evidence, stack traces, and affected components.
6. **Human-Gated CAS Integration.** Ultrafixer never integrates its own commits. The human Orchestrator reviews the `blocked` envelope and recommendation. If approved, the Orchestrator initiates CAS promotion via `lucind-ai integrate` or `lucind-ai integrate retry` (`internal/run/integrate.go:159-165`; `cmd/lucind-ai/cli.go:1794-1815`; `internal/run/integrate_retry.go:18-45`), preserving multi-wave parent reference integrity.
7. **Decline Handling & Worktree Preservation.** If the Orchestrator declines the proposed fix, the repair worktree and branch remain preserved on disk ("cheap to keep, expensive to lose"), and a declined disposition is written to the ledger.

## Conceptual Changes

- **Audited Lane vs Standalone Daemon:** Defect remediation operates within the deterministic, schema-validated Lane lifecycle (`internal/packet/packet.go:33-75`; `internal/result/result.go:102-115`) rather than unmanaged background processes.
- **Two-Axis Defect Classification:** Separates global defect severity (critical) from local operational impact (blocking per branch).
- **Multi-Branch Active Discovery:** Active feature branches are dynamically discovered via `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`) and triaged independently.
- **Signal Reproduction over Syntactic Overlap:** Cross-branch impact verification requires reproducing runtime failures, not merely static CodeGraph dependency links.
- **Ledger Defect Records:** Introduces first-class ledger records (schema v8) for tracking non-critical, non-blocking pre-existing defects without polluting git history.
- **Human-in-the-Loop Integration:** Retains strict CAS promotion control with the human operator, eliminating auto-integration race conditions.

## Capabilities

### New Capabilities
- `ultrafixer-dispatch`: Ephemeral agy-lane execution for project defect remediation, `base_sha` origin diffing, two-axis evaluation, isolated worktree repair, and `blocked` envelope delivery.
- `defect-records`: Ledger schema v8 persistence for durable evidence tracking of non-critical, non-blocking pre-existing defects without code modifications.

### Modified Capabilities
- `dependencies-defects`: Upgrades the manual coordination protocol (`plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md:7-17`) into structured agy-lane packet execution and `blocked` envelope decision responses.
- `lane-execution`: Executes ultrafixer repair packets targeting feature branches with CLI discovery via `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`).

## User and Capability Impact

| Capability | Impact | Description | Seam |
|---|---|---|---|
| `ultrafixer-dispatch` | Added | Ephemeral `agy` Lane that diffs origin against `base_sha`, evaluates critical/blocking axes, repairs in isolated worktree, and emits `blocked` envelope. | `internal/executor/agy.go:69-83,136-150`; `internal/packet/packet.go:33-75`; `internal/result/result.go:77-82,102-115` |
| `defect-records` | Added | Schema v8 ledger table (`defects`/`defect_records`) storing evidence of non-critical, non-blocking pre-existing defects. | `internal/ledger/schema.go:10`; `internal/ledger/ledger.go:574-584,1342-1350` |
| `dependencies-defects` | Modified | Formalizes manual defect assessment (`dependencies-defects.md:7-17`) into automated origin classification, multi-branch impact checks, and question-based blocked envelopes. | `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md:7-23`; `internal/result/result.go:77-82,95-99` |
| `lane-execution` | Modified | Dispatches ultrafixer packets targeting feature branches, discovering active peers via `lucind-ai feature status`. | `cmd/lucind-ai/cli.go:82-87,954-1045`; `internal/run/batch.go:66-80`; `internal/serve/model.go:536-555` |

## Delta Specifications

### Requirement: Origin classification via `base_sha` diffing

Ultrafixer MUST classify defect origin before evaluating severity. It MUST diff the failure context against the target feature's `base_sha` (`internal/packet/packet.go:68`; `internal/ledger/ledger.go:1342-1350`). If the error was introduced by changes on the current feature branch, ultrafixer MUST exit without modifying files.

#### Scenario: Defect introduced by current feature exits cleanly
- GIVEN a failing check caused by code modified between `base_sha` and `HEAD`
- WHEN ultrafixer executes origin classification
- THEN ultrafixer MUST exit with status `done` and a note stating the defect is local to the feature, touching no files

#### Scenario: Pre-existing defect continues to evaluation
- GIVEN a failing check caused by code unmodified between `base_sha` and `HEAD`
- WHEN ultrafixer executes origin classification
- THEN ultrafixer MUST proceed to two-axis critical and blocking evaluation

### Requirement: Independent two-axis evaluation and multi-branch triage

Ultrafixer MUST evaluate pre-existing defects along two independent axes: (1) critical severity (security vulnerability, data loss/corruption risk, or total CI/build failure), and (2) blocking impact, evaluated separately for the originating branch and every active feature branch returned by `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`).

#### Scenario: Critical non-blocking defect triggers repair
- GIVEN a pre-existing defect classified as a security risk or data corruption hazard that does not block the current feature's build
- WHEN ultrafixer completes two-axis evaluation
- THEN ultrafixer MUST generate an isolated repair, commit the fix, and return a `blocked` result envelope

#### Scenario: Non-critical blocking defect triggers repair for affected branch
- GIVEN a pre-existing defect that breaks tests on feature branch A but does not affect feature branch B
- WHEN ultrafixer completes two-axis evaluation across active features
- THEN ultrafixer MUST generate a repair for branch A, record `Finding.Affects` targeting branch A, and return a `blocked` result envelope

#### Scenario: Non-critical non-blocking defect records Defect Record only
- GIVEN a pre-existing defect that is neither critical nor blocking for any active feature branch
- WHEN ultrafixer completes evaluation
- THEN ultrafixer MUST persist a Defect Record in ledger schema v8 and MUST NOT create a fix commit or Change

### Requirement: Signal reproduction for cross-branch impact

Ultrafixer MUST use CodeGraph (`codegraph impact`/`codegraph affected`) only as a preliminary candidate filter. Before marking any peer feature branch as affected or blocked, ultrafixer MUST reproduce the failing test, lint, or build signal in that specific branch's worktree.

#### Scenario: CodeGraph candidate filter confirmed by failure reproduction
- GIVEN a candidate peer branch identified by CodeGraph symbol impact
- WHEN ultrafixer reproduces the failing check command in the candidate branch worktree and it fails with the same signal
- THEN ultrafixer MUST record the branch as affected in the result envelope's questions and findings

#### Scenario: Syntactic overlap without failure reproduction is not blocked
- GIVEN a candidate peer branch with file or symbol overlap identified by CodeGraph
- WHEN ultrafixer executes the check command in the candidate branch worktree and it passes
- THEN ultrafixer MUST NOT mark the candidate branch as blocked or affected

### Requirement: Isolated repair delivery and human-gated CAS integration

Ultrafixer MUST implement repairs in an isolated git worktree and MUST NOT auto-integrate fixes into any branch. Ultrafixer MUST deliver the repair via a schema-valid `blocked` result envelope (`internal/result/result.go:102-115`). The human Orchestrator MUST manually review and initiate CAS promotion via `lucind-ai integrate` or `lucind-ai integrate retry` (`internal/run/integrate.go:159-165`; `cmd/lucind-ai/cli.go:1794-1815`).

#### Scenario: Repair delivered via blocked result envelope
- GIVEN a successful repair and passing test suite in ultrafixer's isolated worktree
- WHEN ultrafixer completes execution
- THEN it MUST emit a `.lucind/result.json` with `status: "blocked"`, populated `commit`, `files_changed`, and a `Question` recommending human integration

#### Scenario: Human accepts fix and triggers integration
- GIVEN a `blocked` ultrafixer result envelope carrying a repair commit
- WHEN the operator runs `lucind-ai integrate retry`
- THEN `lucind-ai` promotes the repair commit using CAS verification against the wave's recorded `expected_parent_sha`

#### Scenario: Human declines fix and worktree is preserved
- GIVEN an operator decision to decline the proposed ultrafixer repair
- WHEN the decision is recorded
- THEN the repair branch and worktree MUST remain preserved on disk, and a declined disposition MUST be recorded in the ledger

## Alternatives Considered

- **Full autonomy / auto-integrating background daemon with `--approval-timeout` + `serve --approver` gate.** Rejected: introduces background-process complexity, timeout race conditions, and violates the conservative manual-trigger philosophy where the human Orchestrator retains full control over CAS promotion across feature branches.
- **Standalone Claude Code agent (e.g. `SendMessage` / persistent background worker).** Rejected: bypasses `lucind-ai`'s audited packet lifecycle, worktree isolation, ledger event logging, and barrier-controlled lane execution (`internal/packet/packet.go:33-75`; `internal/run/batch.go:66-80`). Lane-based `agy` dispatch provides clean ephemeral boundaries.
- **Feature-creation baseline test/lint snapshotting.** Rejected: capturing and storing full test/lint suite outputs at `feature create` adds brittle capture overhead and disk/ledger bloat; `base_sha`/`parent_ref` is already an immutable git anchor persisted in `features` (`internal/ledger/ledger.go:1342-1350`), and origin diffs can be evaluated on-demand using existing git primitives (`internal/overlap/overlap.go:1007-1025`; `internal/resolve/candidate.go:97-115`).
- **Coupled single critical/blocking evaluation axis.** Rejected: critical defects (e.g. severe security vulnerability, data corruption hazard) must be remediated even if they do not immediately block the current branch's compilation or test suite, whereas non-critical blocking bugs (e.g. broken interface type blocking a newly added lane) require immediate branch-level repair. Combining them would obscure severity and triage priority.
- **Pure syntactic file/path overlap for cross-branch impact.** Rejected: syntactic path touching or CodeGraph symbol overlap frequently produces false positives where dependent code is unaffected; requiring active failure signal reproduction (re-running the failing check in the candidate branch) ensures remediation is only flagged for genuinely broken branches.

## Technical Risks and Failure Modes

| Risk | Impact | Mitigation | Seam |
|---|---|---|---|
| External CodeGraph CLI or `.codegraph/` index missing in worktree | Cross-branch impact candidate filtering fails | Treat CodeGraph as optional accelerator; fall back to inspecting active feature worktrees via `git diff` against `base_sha` | `cmd/lucind-ai/cli.go:954-1045`; `internal/overlap/overlap.go:1007-1025` |
| `blocked` result envelope lacks native multi-branch fan-out field | Cannot express per-branch dispositions in a single field | Encode per-branch recommendations in `Question` list (`internal/result/result.go:77-82`) and `Finding.Affects` (`:95-99`) without breaking schema | `internal/result/result.go:77-115`; `internal/result/schema.go:10-28` |
| Concurrent ultrafixer lane and feature lane edit same files | Merge conflict during integration | Ultrafixer operates in an isolated worktree; human initiates sequential CAS promotion via `lucind-ai integrate` | `internal/worktree/worktree.go:150-171`; `internal/run/integrate.go:159-165` |
| Flaky check produces false failure signal reproduction | Non-affected peer branch marked as blocked | Require deterministic reproduction run with `-count=1` / clean caches before declaring branch affected | `internal/run/run.go:369-374`; `internal/executor/agy.go:136-150` |
| Stale CAS anchor on multi-wave feature during fix promotion | Integrate retry CAS failure | Rely on `RetryFeatureTarget` CAS baseline fix (`cmd/lucind-ai/cli.go:1794-1815`; `internal/run/integrate_retry.go:18-45`) | `internal/run/integrate_retry.go:18-45`; `cmd/lucind-ai/cli.go:1794-1815` |
| Fix worktree accumulation on declined repairs | Disk space consumption over time | Retain worktrees by default ("cheap to keep, expensive to lose"); provide pruning via explicit cleanup commands | `internal/run/integrate.go:159-165`; `cmd/lucind-ai/cli.go:56` |

## Rollback Plan and Additivity

Revert with `git revert <sha>`. Reverting restores the pure manual defect contract in `dependencies-defects.md` and removes ultrafixer packet dispatch definitions.

Additivity:
- **Ledger Schema:** Schema migration v8 adds the `defects`/`defect_records` table following the established create-copy-drop-rename pattern (`internal/ledger/schema.go:59-78,221-308`). Existing tables (`lanes`, `events`, `approvals`, `runs`, `features`, `lane_progress`) remain strictly backwards-compatible. Current schema version is `7` (`internal/ledger/schema.go:10`).
- **Result Envelopes:** Ultrafixer emits existing schema-validated `.lucind/result.json` envelopes using `status: "blocked"` (`internal/result/result.go:102-115,122-134`) without altering `result.schema.json` (`internal/result/schema.go:10-28`).
- **CLI & Execution:** Adds ultrafixer packet templates and routing without breaking existing `lucind-ai run`, `lucind-ai feature status`, or `lucind-ai integrate` invocations.

## Test and Validation Impact

| Layer | Coverage | Existing seam |
|---|---|---|
| Origin classification | Base SHA diff correctly discriminates feature-introduced vs pre-existing defects | Extend `internal/overlap/overlap_test.go` and `internal/resolve/candidate_test.go` |
| Two-axis evaluation | Unit tests verifying critical global vs per-branch blocking matrix permutations | New tests in `internal/run/` or defect assessment unit test suite |
| Cross-branch triage & reproduction | Verification that failing checks run in peer worktrees and detect genuine breaks | Mocked feature status and test runner tests in `internal/run/` |
| Envelope emission | Verification that ultrafixer lane emits valid `blocked` envelope with `Question` and `Commit` | `internal/result/result_test.go`; `internal/run/run_test.go` |
| Ledger schema v8 | Idempotent migration from v7 to v8, Defect Record insertion and query | `internal/ledger/schema_test.go`; `internal/ledger/ledger_test.go` |
| Integration & Decline | Manual `integrate retry` CAS promotion and declined worktree preservation | `internal/run/integrate_test.go`; `internal/run/integrate_retry_test.go` |

## Out of Scope

- Modifying or duplicating the `lucind-ai-fixer` Claude Code agent (sole owner of defects in `lucind-ai`'s own repository).
- Background polling daemon or automatic failure watcher (trigger remains strictly manual via the feature Orchestrator).
- Automatic self-integration or `--approval-timeout` automated CAS promotion (all promotions require explicit human execution).
- Internal Go library wrapping of CodeGraph CLI commands (CodeGraph remains an external tool dependency).
- Automatic git bisect execution across deep commit histories (origin classification relies on direct `base_sha` diffs).

## Open Questions

- [ ] Defect Record schema naming: Should the new ledger table be named `defect_records` or `defects`, and what minimum columns beyond `(id, feature_id, run_id, lane_id, error_signature, evidence, disposition, created_at)` are required?
- [ ] Multi-branch question encoding: Should multi-branch dispositions be represented solely via multiple `Question` items in the existing `result.json` schema, or should a future schema migration add a dedicated `affected_branches` array to `result.schema.json`?
- [ ] Worktree pruning policy: Should declined ultrafixer worktrees be cleaned up by a flag on `lucind-ai worktree cleanup` or left strictly to manual operator deletion?

## Success Criteria

- [ ] Ultrafixer is formally defined as an ephemeral `agy` Lane (`internal/packet/packet.go:33-75`; `cmd/lucind-ai/cli.go:82-87`) triggered manually by the feature Orchestrator.
- [ ] Step 1 origin classification deterministically diffs defects against `base_sha` (`internal/packet/packet.go:68`; `internal/overlap/overlap.go:1007-1025`), exiting cleanly on feature-introduced errors.
- [ ] Step 2 evaluates critical and per-branch blocking axes independently across active features discovered via `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`).
- [ ] Cross-branch impact requires failure signal reproduction in candidate branch worktrees, not just CodeGraph candidate filtering.
- [ ] Step 3 repairs critical/blocking defects in isolated worktrees and emits schema-valid `blocked` envelopes (`internal/result/result.go:102-115,122-134`) with repair commits and recommendation questions.
- [ ] Non-critical, non-blocking defects are persisted as Defect Records in ledger schema v8 (`internal/ledger/schema.go:10`) without touching workspace code.
- [ ] Integration is strictly human-initiated via `lucind-ai integrate` / `integrate retry` (`internal/run/integrate.go:159-165`; `cmd/lucind-ai/cli.go:1794-1815`), and declined worktrees are preserved.
- [ ] `lucind-ai-fixer` remains untouched and exclusive to `lucind-ai` repository defects.
