# Design: Ultrafixer

## Technical Approach

Dispatched as an ephemeral, bounded `agy` Lane (`internal/packet/packet.go:33-75`; `cmd/lucind-ai/cli.go:82-87`; `internal/executor/agy.go:69-88,136-150`), `ultrafixer` diagnoses and remediates pre-existing defects across any project orchestrated by `lucind-ai`. It operates entirely through existing lane-execution machinery (`internal/run/batch.go:66-80`; `internal/run/run.go:368-435`) without requiring ANY new Go dispatch plumbing. The capability adds a dedicated packet template asset (`plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`), formalizes defect coordination rules (`plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md:7-23`), and introduces ledger schema v8 (`internal/ledger/schema.go:10,363-472`) for durable `defect_records` persistence when defects are non-critical and non-blocking.

When an Orchestrator encounters a failing test, linter, or build check during feature development, it dispatches an ultrafixer packet carrying the failing command and error transcript in Context. Within its isolated worktree (`internal/worktree/worktree.go:150-175`), ultrafixer executes a deterministic three-step workflow:
1. **Origin Classification**: Diffs the failing code against the target feature's immutable `base_sha` (`internal/packet/packet.go:68`; `internal/resolve/candidate.go:97-120`; `internal/overlap/overlap.go:353-375,1007-1025`). If the error was introduced by changes on the current feature branch, ultrafixer exits with `status: done` and touches no files.
2. **Two-Axis Evaluation**: For pre-existing defects, independently assesses:
   - **Critical severity** (global security hazard, data corruption risk, or total CI/build failure).
   - **Blocking impact** per active feature branch discovered via `lucind-ai feature status` (`cmd/lucind-ai/cli.go:954-1045`; `internal/serve/model.go:536-555`). Cross-branch candidates are filtered via CodeGraph (`codegraph impact`/`codegraph affected`) and MUST be verified by runtime signal reproduction (re-executing the failing check command in the candidate branch's worktree).
3. **Repair & Disposition**:
   - **Critical OR Blocking**: Ultrafixer creates an isolated repair commit in its worktree, verifies test passage, and emits a schema-valid `blocked` result envelope (`internal/result/result.go:77-82,102-115`) carrying the repair `commit`, `findings` (with `affects`), and `questions` for human-gated CAS promotion (`cmd/lucind-ai/cli.go:1719-1838`; `internal/run/integrate_retry.go:18-45,160-200`).
   - **Non-Critical AND Non-Blocking**: Ultrafixer creates no repair commit and touches no code, instead recording a durable Defect Record in SQLite ledger schema v8 (`internal/ledger/schema.go:10`).

## Architecture Decisions

### Decision: Zero new Go dispatch plumbing for ultrafixer-dispatch

**Choice**: Reuse `lucind-ai run --packet` (`cmd/lucind-ai/cli.go:116-125,169-174`), `internal/packet/packet.go:33-75`, `internal/executor/agy.go:136-150`, and worktree isolation (`internal/worktree/worktree.go:150-175`) verbatim. Dispatch capability is delivered entirely via the new packet template asset (`plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`) and skill documentation.
**Alternatives considered**: A new `lucind-ai fix` CLI command; a persistent background failure-watching daemon; a custom subagent inside `lucind-ai-fixer`.
**Rationale**: `supportedExecutors["agy"]` (`cmd/lucind-ai/cli.go:82-87`) already dispatches `gemini-3.7-flash-high` headlessly in an isolated worktree with strict `allowed_paths` enforcement (`internal/run/run.go:697-715`), timeout tracking, ledger event streaming (`internal/run/run.go:425-434`), and result schema validation (`internal/result/result.go:137-160`). Ephemeral single-packet dispatch avoids unmonitored agent loops and background race conditions.
**Terminal consumer**: Feature Orchestrator executing `lucind-ai run --packet .lucind/packets/fix-<slug>.md` (`cmd/lucind-ai/cli.go:169-174`) -> `ExecuteBatch` (`internal/run/batch.go:66-80`) -> `Agy.Run` (`internal/executor/agy.go:136-150`).

### Decision: Defect Record schema v8 naming and column structure

**Choice**: Name the ledger table `defect_records` in schema migration v8 (`internal/ledger/schema.go:10,363-472`), structured as:
```sql
CREATE TABLE IF NOT EXISTS defect_records (
  id              TEXT PRIMARY KEY,
  feature_id      TEXT NOT NULL,
  run_id          TEXT NOT NULL DEFAULT '',
  lane_id         TEXT NOT NULL DEFAULT '',
  error_signature TEXT NOT NULL,
  evidence        TEXT NOT NULL DEFAULT '',
  disposition     TEXT NOT NULL CHECK (disposition IN ('recorded','repaired','declined','deferred')),
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_defect_records_feature ON defect_records(feature_id, id);
```
**Alternatives considered**: Table named `defects`; embedding in `events.detail` JSON; overloading `approvals.defect_surfaced_later` (`internal/ledger/ledger.go:574-584`).
**Rationale**: Follows this repo's strict compound naming convention (`integration_attempts`, `feature_leases`, `reconciliation_requests`, `overlap_evidence`, `lane_progress` in `internal/ledger/schema.go:106-180,298-308`). Distinct from `approvals.defect_surfaced_later` (which tracks post-approval human regret rates, `internal/ledger/ledger.go:797`). Uses `CREATE TABLE IF NOT EXISTS` matching additive migrations `migrateV2ToV3DDL` (`:80-93`) and `migrateV3ToV4DDL` (`:95-180`) without requiring table rebuilds.
**Terminal consumer**: `Ledger.RecordDefect` and `Ledger.ListDefects` (`internal/ledger/ledger.go`) called by `runDefectRecord` / `runDefectList` (`cmd/lucind-ai/cli.go`) and `serve.Model` (`internal/serve/model.go`).

### Decision: Multi-branch blocked disposition encoding via existing Questions and Findings

**Choice**: Encode multi-branch triage dispositions directly into the existing `Questions []Question` (`internal/result/result.go:77-82`) and `Findings []Finding` (`internal/result/result.go:95-99`) fields of `.lucind/result.json` without modifying `result.schema.json` (`internal/result/schema.go:10-28`).
**Alternatives considered**: Extending `result.schema.json` with a dedicated `affected_branches` array; adding a top-level `multi_branch_dispositions` map.
**Rationale**: `result.schema.json` already defines `questions` and `findings` as unbounded arrays of objects (`result.schema.json:103-124,140-153`). Downstream consumers (`result.Read` in `internal/result/result.go:137-160`, `decideStatus` in `internal/run/run.go:681-694`, and `PersistEnvelope` in `cmd/lucind-ai/cli.go:753-760`) read and persist the envelope without error. Each affected branch receives a distinct `Question` entry with its branch-specific `WhyBlocking`, `Options`, and `Recommendation`, while `Findings` records the failure reproduction `Evidence` and target branch in `Affects`.
**Terminal consumer**: Human Orchestrator inspecting `.lucind/results/<lane_id>.json` (`cmd/lucind-ai/cli.go:446,753-760`) and `result.Read` in `internal/run/run.go:682-694`.

### Decision: Worktree retention policy with existing cleanup CLI

**Choice**: Retain repair worktrees and branches on disk upon block or decline ("cheap to keep, expensive to lose"). Rely entirely on the existing `lucind-ai worktree cleanup --lane <id>` command (`cmd/lucind-ai/cli.go:1649-1685`) and `worktree.Cleanup` (`internal/worktree/worktree.go:240-253`) for operator-driven pruning. Zero new CLI flags or commands needed.
**Alternatives considered**: Automatic deletion of worktrees for declined repairs; adding `--declined` or `--force` flags to `worktree cleanup`; background timer-based garbage collection.
**Rationale**: Synthesizing a high-quality defect repair is computationally expensive. Retaining the worktree at `../<repo>-worktrees/<lane-id>` allows the human operator to inspect the diff or manually cherry-pick commits later. When pruning is desired, `lucind-ai worktree cleanup --lane <id>` is already idempotent and removes the linked worktree cleanly via `worktree.Remove` (`internal/worktree/worktree.go:255-261`).
**Terminal consumer**: Operator executing `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1655-1685`) -> `worktree.Cleanup` (`internal/worktree/worktree.go:247-253`).

### Decision: Two-step origin classification via base_sha diff and two-axis triage

**Choice**: Step 1 classifies origin by diffing the failing context against the feature's recorded `base_sha` (`internal/packet/packet.go:68`; `internal/ledger/ledger.go:1342-1350`) using direct `git diff` shellouts (`internal/resolve/candidate.go:97-120`; `internal/overlap/overlap.go:353-375`). If feature-introduced, exit immediately. Step 2 independently assesses global critical severity vs branch-level blocking impact across active features (`lucind-ai feature status`, `cmd/lucind-ai/cli.go:954-1045`).
**Alternatives considered**: Git bisect across repository history; single coupled severity metric; auto-assigning blame to the active lane.
**Rationale**: `base_sha` is the immutable git anchor where the active feature branched. A diff against `base_sha` deterministically identifies whether the defect pre-existed without expensive git bisect loops. Decoupling critical severity (security/corruption/build) from blocking impact ensures critical defects are fixed even if they don't block the current branch, while non-critical defects only block branches they actively break.
**Terminal consumer**: Ultrafixer agy subprocess executing Step 1 and Step 2 instructions from `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`.

### Decision: Cross-branch impact candidate filtering via CodeGraph + mandatory signal reproduction

**Choice**: Use CodeGraph CLI commands (`codegraph impact` / `codegraph affected`) as an external candidate filter, but mandate active failure signal reproduction (executing the failing check command in the candidate branch's worktree) before declaring any peer feature blocked.
**Alternatives considered**: Declaring branches blocked based purely on static syntactic file/symbol overlap; running all test suites across all active worktrees without filtering.
**Rationale**: CodeGraph index lookups are fast but syntactic overlap frequently produces false positives where dependent symbols are unaffected. Blind test execution across all active worktrees wastes execution time and quota. Combining CodeGraph filtering with mandatory runtime reproduction guarantees zero false-positive blockages.
**Terminal consumer**: Ultrafixer agy subprocess executing Step 2 triage; emitted `Finding.Evidence` and `Question.WhyBlocking` in `.lucind/result.json` (`internal/result/result.go:77-99`).

### Decision: Human-gated CAS promotion and retry mechanics

**Choice**: Ultrafixer never self-integrates or auto-promotes fixes. Fixes are delivered exclusively via `blocked` result envelopes (`internal/result/result.go:102-115`). CAS promotion is initiated manually by the human Orchestrator running `lucind-ai integrate` or `lucind-ai integrate retry --run <run_id> --lane <lane_id>` (`cmd/lucind-ai/cli.go:1719-1838`; `internal/run/integrate_retry.go:18-45,160-200`).
**Alternatives considered**: Auto-integration on passing checks; background webhook approval; `--approval-timeout` auto-promotion.
**Rationale**: Preserves the strict principle that human operators own CAS integration across feature branches. Avoids race conditions on multi-wave feature baselines, utilizing the proven `RetryFeatureTarget` recovery mechanism (`internal/run/integrate_retry.go:160-200`).
**Terminal consumer**: Human Orchestrator running `lucind-ai integrate retry` (`cmd/lucind-ai/cli.go:1719-1838`) -> `IntegrateFeature` (`internal/run/integrate_feature.go:26-40`).

## Flow and Invariants

```
Orchestrator observes failing check in any project
  │
  ▼
Dispatch ultrafixer packet: `lucind-ai run --packet .lucind/packets/fix-<slug>.md`
  │
  ▼
`internal/run/batch.go` (ExecuteBatch) → `internal/executor/agy.go` (Agy.Run)
  │
  ▼
Isolated Worktree: `../<repo>-worktrees/fix-<slug>`
  │
  ├─► Step 1: Origin Classification (git diff against base_sha)
  │     │
  │     ├─► [Feature-Introduced] ──► Exit `done` (Summary: "Feature regression; no fix Change generated")
  │     │
  │     └─► [Pre-Existing Defect] ──► Proceed to Step 2
  │
  ├─► Step 2: Two-Axis Evaluation (Active features via `lucind-ai feature status`)
  │     │
  │     ├─► Critical Axis: Security / Data Loss / Build Failure
  │     │
  │     └─► Blocking Axis: CodeGraph candidate filter + Signal Reproduction per active branch
  │
  └─► Step 3: Repair & Disposition
        │
        ├─► [Critical OR Blocking]
        │     │
        │     ├─► Minimal repair in worktree + test pass + conventional commit
        │     │
        │     └─► Emit `.lucind/result.json` (`status: "blocked"`, Commit, Questions, Findings)
        │           │
        │           ▼
        │     Worktree Preserved (`worktree_preserved: 1` in ledger)
        │           │
        │           ├─► Human runs `lucind-ai integrate retry` ──► CAS Promotion
        │           │
        │           └─► Human Declines ──► Worktree Kept (or pruned via `worktree cleanup`)
        │
        └─► [Non-Critical AND Non-Blocking]
              │
              ├─► Touch NO source code
              │
              ├─► Persist Defect Record to SQLite ledger schema v8 (`defect_records`)
              │
              └─► Emit `.lucind/result.json` (`status: "done"`, Summary: "Recorded Defect Record")
```

### Invariants

1. **Origin Before Evaluation**: Origin classification against `base_sha` MUST complete before any repair or multi-branch evaluation occurs (`internal/packet/packet.go:68`; `internal/resolve/candidate.go:97-120`). Feature-introduced regressions MUST NOT produce a repair Change.
2. **Independent Evaluation Axes**: Critical severity (global risk) and blocking impact (branch-specific) MUST be evaluated independently. Neither axis implies the other.
3. **Signal Reproduction Before Declaring Blocked**: No peer feature branch may be marked as affected or blocked without successful reproduction of the failing check command in that branch's worktree. Syntactic CodeGraph overlap alone is strictly insufficient.
4. **Ephemeral Dispatch Isolation**: Ultrafixer executes as an ephemeral, single-packet `agy` Lane inside an isolated linked worktree (`internal/worktree/worktree.go:150-175`). It MUST NEVER run as a persistent daemon or mutate unassigned worktrees.
5. **Human-Gated CAS Integration**: Ultrafixer MUST NEVER self-integrate. All repairs are delivered via `blocked` envelopes (`internal/result/result.go:102-115`), and promotion is executed solely by human operators via `lucind-ai integrate` / `integrate retry` (`internal/run/integrate_retry.go:18-45`).
6. **Schema-Strict Ledger Persistence (v8)**: Non-critical, non-blocking pre-existing defects MUST be persisted in the `defect_records` ledger table (schema v8, `internal/ledger/schema.go:10`) without touching source files.
7. **Preserve on Block or Decline**: Worktrees for blocked or declined repairs remain preserved on disk (`internal/run/run.go:631-645`). Pruning is operator-driven via `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1649-1685`).
8. **Conventional Commits Without Attribution**: All repair commits MUST use conventional commit formatting with zero AI attribution or `Co-Authored-By` trailers (`internal/result/result.go:101`).

## Interfaces / Contracts

### Ledger Schema v8 DDL (`internal/ledger/schema.go`)

```go
const schemaVersion = 8

const migrateV7ToV8DDL = `
CREATE TABLE IF NOT EXISTS defect_records (
  id              TEXT PRIMARY KEY,
  feature_id      TEXT NOT NULL,
  run_id          TEXT NOT NULL DEFAULT '',
  lane_id         TEXT NOT NULL DEFAULT '',
  error_signature TEXT NOT NULL,
  evidence        TEXT NOT NULL DEFAULT '',
  disposition     TEXT NOT NULL CHECK (disposition IN ('recorded','repaired','declined','deferred')),
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_defect_records_feature ON defect_records(feature_id, id);
`
```

### Go Ledger API (`internal/ledger/ledger.go`)

```go
// DefectDisposition represents the triage or lifecycle state of a recorded defect.
type DefectDisposition string

const (
	DefectDispositionRecorded DefectDisposition = "recorded"
	DefectDispositionRepaired DefectDisposition = "repaired"
	DefectDispositionDeclined DefectDisposition = "declined"
	DefectDispositionDeferred DefectDisposition = "deferred"
)

// DefectRecord represents one row of the defect_records table.
type DefectRecord struct {
	ID             string
	FeatureID      string
	RunID          string
	LaneID         string
	ErrorSignature string
	Evidence       string
	Disposition    DefectDisposition
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RecordDefect inserts a new defect record into the ledger.
func (l *Ledger) RecordDefect(ctx context.Context, rec DefectRecord) error

// ListDefects returns all defect records for a feature, ordered by created_at.
func (l *Ledger) ListDefects(ctx context.Context, featureID string) ([]DefectRecord, error)

// GetDefect returns a single defect record by ID.
func (l *Ledger) GetDefect(ctx context.Context, id string) (DefectRecord, error)
```

### Ultrafixer Packet Template (`plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`)

```markdown
---
id: <id>
executor: agy
routed_by: pre-existing defect triage and repair
model: gemini-3.7-flash-high
base_sha: <base_sha>
parent_ref: <parent_ref>
allowed_paths: ["<path1>", "<path2>"]
---

# Packet <id>

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/<id>  ·  **Branch:** lucind/<id>

## Goal

Triage pre-existing defect `<error-signature>`, assess critical and blocking impact across active features, and either deliver an isolated repair commit via a `blocked` result envelope or persist a Defect Record in the ledger.

## Preconditions

- Target feature `<feature-id>` is active with recorded `base_sha` `<base_sha>`.
- Failing check command is runnable in the current worktree environment.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.**
- [ ] **The work is committed (if critical/blocking) OR recorded in the ledger (if non-critical/non-blocking).**
- [ ] Origin classification executed against `base_sha`. If feature-introduced, exit `done` with explanatory summary.
- [ ] Two-axis evaluation completed across active features discovered via `lucind-ai feature status`.
- [ ] Cross-branch impact verified via failure signal reproduction in candidate worktrees.
- [ ] Schema-valid `.lucind/result.json` emitted with appropriate `status`, `questions`, and `findings`.

## Allowed paths

- `<path1>`
- `<path2>`

## Hard stops

- Origin classification reveals the defect was introduced on the current feature branch between `base_sha` and `HEAD` (exit `status: done`, touch no files).
- The defect cannot be reproduced locally with the provided check command.
- Any credential value would need to be chosen, generated, or written.
- Auto-integrating or merging the repair commit directly into any branch.

## Context

### Failing check command
`<exact-failing-command, e.g. go test ./... / npm test / cargo test / pytest>`

### Error transcript and signature
`<error-output-and-stack-trace>`

### Feature metadata
- Target feature: `<feature-id>`
- Base SHA: `<base_sha>`
- Parent ref: `<parent_ref>`

## Return

Write the result envelope to `.lucind/result.json` in this worktree.
```

## File Changes

| File | Action | Terminal consumer |
|---|---|---|
| `internal/ledger/schema.go` | Bump `schemaVersion = 8`, add `migrateV7ToV8DDL` and migration step (`:10,363-472`) | `ledger.Open` (`internal/ledger/ledger.go:128`) |
| `internal/ledger/ledger.go` | Add `DefectRecord` struct, `RecordDefect`, `ListDefects`, `GetDefect` | `cmd/lucind-ai/cli.go` (CLI defect commands) and `internal/serve/model.go` |
| `cmd/lucind-ai/cli.go` | Add `defect` subcommand routing (`record`, `list`) | Operator CLI invocations (`lucind-ai defect list --feature <id>`) |
| `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md` | New packet template asset carrying exact failing command Context and three-step triage instructions | Feature Orchestrator generating ultrafixer packets for `lucind-ai run` |
| `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md` | Update defect protocol from manual steps (`:7-23`) to structured ultrafixer packet execution | Human Orchestrator defect triage workflow |

*Note: As confirmed in Technical Approach and Architecture Decisions, `ultrafixer-dispatch` requires zero new Go execution code — `internal/run/`, `internal/executor/`, and `internal/packet/` are reused verbatim.*

## Testing Strategy and Test Seams

| Layer | What | Approach | Seam |
|---|---|---|---|
| Unit | Ledger schema v8 migration & CRUD | Idempotent migration from v7 to v8, insert/query `DefectRecord` rows | `internal/ledger/schema_test.go`; `internal/ledger/ledger_test.go` |
| Unit | Origin classification diff logic | Verify base_sha diff accurately separates feature edits from pre-existing base code | `internal/overlap/overlap_test.go`; `internal/resolve/candidate_test.go` |
| Unit | Result envelope multi-branch encoding | Validate `.lucind/result.json` with multiple `Question` and `Finding` entries against schema | `internal/result/result_test.go:48-105` |
| Integration | Ultrafixer packet parsing & execution | Dispatch ultrafixer packet via `lucind-ai run` with fake executor, assert worktree preservation on `blocked` | `cmd/lucind-ai/cli_test.go:1512-1550`; `internal/run/run_test.go:651-695` |
| Integration | CAS retry on repair commit | Execute `lucind-ai integrate retry` against preserved ultrafixer repair lane | `cmd/lucind-ai/cli_test.go:4076-4125`; `internal/run/integrate_retry_test.go` |
| E2E / CLI | `lucind-ai defect list` and `worktree cleanup` | CLI tests verifying defect query output and idempotent worktree cleanup | `cmd/lucind-ai/cli_test.go:3280-3305` |

**Existing seams**: `fakeExecutor`; `run.Deps`; `ledger.Open`; `worktree.Cleanup`; `result.Read`; `integrate.Check`.
**New seams**: `Ledger.RecordDefect`; `Ledger.ListDefects`.

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | Applicable | Ultrafixer operates strictly against the provided failing check command in Context. It does not treat documentation edits as defect fixes unless targeted by a doc/linter check. | `candidate_test.go` verifying `allowed_paths` enforcement on doc changes |
| Git repository selection | Applicable | Ultrafixer operates strictly inside its isolated worktree (`../<repo>-worktrees/fix-<slug>`). Candidate branch checks during Step 2 signal reproduction execute read-only check commands inside peer worktrees without modifying files. Any out-of-scope mutation triggers `allowed_paths` deviation. | `run_test.go` verifying no file modifications occur outside `allowed_paths` |
| Commit state | Applicable | Repair commits must use conventional commit formatting (`fix(...)`) with no AI attribution or `Co-Authored-By` trailers. Worktree must be clean (`git status --porcelain` empty) before envelope emission. | `result_test.go` and `cli_test.go` asserting commit presence and clean git status on `blocked` repair envelopes |
| Push state | N/A | Ultrafixer never pushes branches or commits to remote repositories. Confirmed out of scope. | None |
| PR commands | N/A | Ultrafixer never creates or manages PRs; CAS promotion is human-initiated via `lucind-ai integrate`. Confirmed out of scope. | None |

## Rollback and Additivity

**Choice**: `git revert <sha>` removes the `defect_records` schema migration, CLI verbs, and packet template asset.
**Alternatives considered**: Schema downgrade scripts.
**Rationale**:
- **Ledger Schema Additivity**: Schema v8 adds the standalone table `defect_records` without modifying or rebuilding existing tables (`lanes`, `events`, `approvals`, `runs`, `features`, `lane_progress`). Prior schema versions remain fully functional.
- **Envelope Compatibility**: Ultrafixer emits existing schema-validated `.lucind/result.json` envelopes (`status: "blocked"` / `"done"`) without modifying `result.schema.json` (`internal/result/schema.go:10-28`).
- **CLI & Dispatch Compatibility**: Existing `lucind-ai run`, `lucind-ai feature status`, and `lucind-ai integrate retry` workflows remain strictly backwards-compatible.

## Open Questions and Out of Scope

### Resolved Design Decisions (from proposal.md)
1. **Defect Record schema naming/columns**: RESOLVED. Table is named `defect_records` with typed columns `(id, feature_id, run_id, lane_id, error_signature, evidence, disposition, created_at, updated_at)` in ledger schema v8 (`internal/ledger/schema.go:10,363-472`).
2. **Multi-branch question encoding**: RESOLVED. Encoded via repeated `Question` objects (`internal/result/result.go:77-82`) and `Finding.Affects` (`internal/result/result.go:95-99`) in `.lucind/result.json`. Zero changes to `result.schema.json`.
3. **Worktree pruning policy**: RESOLVED. Preserved by default; operators use existing `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1649-1685`; `internal/worktree/worktree.go:240-253`). Zero new flags or cleanup commands needed.

### Out of Scope
- Standalone background daemons or continuous test watchers (dispatch remains strictly manual via the Orchestrator).
- Modifying or duplicating `/home/lanzerdev/.claude/agents/lucind-ai-fixer.md` (which remains exclusive to defects within `lucind-ai`'s own repository).
- Automated self-integration or unattended CAS promotion (all promotions require explicit human operator invocation).
- Internal Go library wrapping of CodeGraph CLI commands (CodeGraph remains an external tool dependency).
- Implementing code changes during this design phase (implementation is deferred to subsequent spec/tasks/apply phases).
