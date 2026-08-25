# Tasks: Ultrafixer

Single packet, sequential apply, three work-unit commits. No `apply-dag.yaml`: accepted one PR (`delivery_strategy: single-pr`); Strict-TDD RED/GREEN stays in one lane; `Integrate` bisects a failing combined tree (`internal/run/integrate.go:50-59`). Cross-slice deps (schema v8 `defect_records` DDL before `Ledger.RecordDefect`; `Ledger` methods before `cmd/lucind-ai/cli.go` subcommands) are sequenced here, not as parallel waves. Per zero-new-Go dispatch decision (`design.md:18-24`), execution infrastructure (`internal/run/`, `internal/executor/`, `internal/packet/`) is reused verbatim.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 900–1150 (impl ~350–430, tests ~480–600, asset/docs ~90–120) |
| 400-line budget risk | High |
| 800-line budget risk | Medium |
| 1200-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR, three work-unit commits |
| Delivery strategy | single-pr (size:exception) |
| Chain strategy | size-exception |

Decision needed before apply: No (with `delivery_strategy: single-pr` against the 1200-line review budget, the estimated 900–1150 lines fits comfortably within the 1200-line ceiling, but exceeds the standard 400-line threshold; orchestrator records `size:exception` before apply per Review Workload Guard)
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High
800-line budget risk: Medium
1200-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Schema v8 (`defect_records` table + index + migration step) + Go Ledger API (`DefectDisposition`, `DefectRecord`, `RecordDefect`, `ListDefects`, `GetDefect`) | PR 1 | `go test ./internal/ledger -count=1` | SQLite tempdir ledger fixture (`openTestLedger`, `createV7SchemaFixture`) | revert v8 DDL and `Ledger` CRUD methods; database stays at v7 |
| 2 | CLI defect subcommands (`lucind-ai defect record` and `lucind-ai defect list` + usage + routing) | PR 1 | `go test ./cmd/lucind-ai -run TestDefect -count=1` | in-process CLI execution via `run()` with temp repo / ledger | revert `cli.go` dispatch routing and defect helper functions |
| 3 | Ultrafixer packet template asset (`ultrafixer-packet-template.md`), template contract test, and coordination documentation update (`dependencies-defects.md`) | PR 1 | `go test ./internal/packet -run TestUltrafixerPacketTemplateContract -count=1` | `packet.Parse` template contract verification | delete `ultrafixer-packet-template.md`, revert `dependencies-defects.md` |

Apply order: Unit 1 (schema & ledger) → Unit 2 (CLI) → Unit 3 (asset & coordination). Unit 2 must not start until Unit 1's v8 schema and `ledger.Ledger` methods (`RecordDefect`, `ListDefects`, `GetDefect`) have landed. Shared-file edits stay sequential in this packet: `internal/ledger/schema.go` `schemaVersion` at `:10` then `migrateV7ToV8DDL` at `:359` then `migrate` step at `:458-472`; `internal/ledger/ledger.go` types and methods at `:1436`; `cmd/lucind-ai/cli.go` usage at `:56`, routing switch at `:150-164`, subcommand implementations at `:1838`.

## Phase 1: Schema migration and Ledger API

- [x] 1.1 RED: In `internal/ledger/schema_test.go` (seams after `:384` and helper fixtures at `:386-410`), add `TestMigrateV7ToV8PreservesRowsAndAddsSchema`, `TestSchemaV8ConstraintsAndIndexes`, `TestSchemaV8ReopenIsIdempotent`, and `createV7SchemaFixture` helper — asserting schema migration to version 8, creation of `defect_records` STRICT table with columns `(id, feature_id, run_id, lane_id, error_signature, evidence, disposition, created_at, updated_at)` and index `idx_defect_records_feature`, row preservation across all existing tables (`runs`, `lane_progress`, `lanes`, `events`, `approvals`), CHECK constraint on `disposition IN ('recorded','repaired','declined','deferred')`, and idempotency on database reopen.
- [x] 1.2 GREEN: In `internal/ledger/schema.go`, bump `schemaVersion = 8` (`:10`), add `migrateV7ToV8DDL` (`:359`), and wire the `currentVersion < 8` migration step in `migrate` (`:458-472`). Terminal consumer: `ledger.Open` (`internal/ledger/ledger.go:146`). Prove: `go test ./internal/ledger -run 'MigrateV7ToV8|SchemaV8'`.
- [x] 1.3 RED: In `internal/ledger/ledger_test.go` (after `:1923`), add `TestLedgerRecordAndGetDefect` and `TestLedgerRecordDefectRejectsInvalidDisposition` — asserting `RecordDefect` persists a `DefectRecord` and `GetDefect` retrieves it by ID with timestamp round-tripping, unknown ID returns error, and invalid disposition string is rejected by database constraint.
- [x] 1.4 GREEN: In `internal/ledger/ledger.go` (after `:1436`), add `DefectDisposition` type with constants (`DefectDispositionRecorded`, `DefectDispositionRepaired`, `DefectDispositionDeclined`, `DefectDispositionDeferred`), `DefectRecord` struct, and implement `RecordDefect(ctx context.Context, rec DefectRecord) error` and `GetDefect(ctx context.Context, id string) (DefectRecord, error)` on `*Ledger`. Terminal consumer: `runDefectRecord` (`cmd/lucind-ai/cli.go`) and `serve.Model` (`internal/serve/model.go`). Prove: `go test ./internal/ledger -run 'TestLedgerRecordAndGetDefect|TestLedgerRecordDefectRejectsInvalidDisposition'`.
- [x] 1.5 RED: In `internal/ledger/ledger_test.go` (after `:1923`), add `TestLedgerListDefects` — asserting `ListDefects` filters defect records by `feature_id` and orders them chronologically by `created_at ASC`, returning empty slice for features with no defect records.
- [x] 1.6 GREEN: In `internal/ledger/ledger.go` (after `:1436`), implement `ListDefects(ctx context.Context, featureID string) ([]DefectRecord, error)` on `*Ledger`. Terminal consumer: `runDefectList` (`cmd/lucind-ai/cli.go`) and `serve.Model` (`internal/serve/model.go`). Prove: `go test ./internal/ledger -run 'TestLedgerListDefects'`.

## Phase 2: CLI defect inspection commands

- [x] 2.1 RED: In `cmd/lucind-ai/cli_test.go` (after `:4234`), add `TestDefectSubcommandUnknownAction` and `TestDefectListCLIRequiresFeature` — asserting `lucind-ai defect` without action or with unknown action prints usage error, and `lucind-ai defect list` without `--feature` fails with usage error.
- [x] 2.2 GREEN: In `cmd/lucind-ai/cli.go`, update `usage` const (`:56`), add `case "defect": return defectDispatch(ctx, args[1:], stdout, stderr)` to `run` switch (`:140-164`), and define `defectDispatch` routing `list` and `record` subcommands (`:1838`). Terminal consumer: CLI driver `run()` (`cmd/lucind-ai/cli.go:134`). Prove: `go test ./cmd/lucind-ai -run 'TestDefectSubcommandUnknownAction|TestDefectListCLIRequiresFeature'`.
- [x] 2.3 RED: In `cmd/lucind-ai/cli_test.go` (after `:4234`), add `TestDefectRecordCLI` and `TestDefectRecordCLIRequiresFlags` — asserting `lucind-ai defect record --id <id> --feature <id> --signature <sig> --evidence <ev> --disposition <disp>` persists record in ledger and prints confirmation to stdout, and missing required flags (`--id`, `--feature`, `--signature`) results in usage error.
- [x] 2.4 GREEN: In `cmd/lucind-ai/cli.go` (after `:1838`), implement `runDefectRecord(ctx context.Context, args []string, stdout, stderr io.Writer) int` using `Ledger.RecordDefect`. Terminal consumer: Operator CLI invocations (`lucind-ai defect record`). Depends on 1.4, 2.2. Prove: `go test ./cmd/lucind-ai -run 'TestDefectRecordCLI'`.
- [x] 2.5 RED: In `cmd/lucind-ai/cli_test.go` (after `:4234`), add `TestDefectListCLI` — asserting `lucind-ai defect list --feature <id>` reads from ledger via `Ledger.ListDefects` and outputs formatted list containing defect ID, signature, disposition, and timestamp.
- [x] 2.6 GREEN: In `cmd/lucind-ai/cli.go` (after `:1838`), implement `runDefectList(ctx context.Context, args []string, stdout, stderr io.Writer) int` using `Ledger.ListDefects`. Terminal consumer: Operator CLI invocations (`lucind-ai defect list --feature <id>`). Depends on 1.6, 2.2. Prove: `go test ./cmd/lucind-ai -run 'TestDefectListCLI'`.

## Phase 3: Ultrafixer packet template asset and coordination protocol

- [x] 3.1 RED: In `internal/packet/packet_test.go` (seams after `:2041`), add `TestUltrafixerPacketTemplateContract` — parsing `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md` via `packet.Parse`, asserting frontmatter contains `executor: agy`, `routed_by: pre-existing defect triage and repair`, `model: gemini-3.7-flash-high`, body contains sections for Goal, Preconditions, Done criteria, Allowed paths, Hard stops, Context (with Failing check command, Error transcript and signature, and Feature metadata), and Return.
- [x] 3.2 GREEN: Create `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md` matching `design.md:204-265` verbatim. Terminal consumer: Feature Orchestrator generating ultrafixer packets for `lucind-ai run --packet` (`cmd/lucind-ai/cli.go:169-174`). Prove: `go test ./internal/packet -run TestUltrafixerPacketTemplateContract`.
- [x] 3.3 RED: In `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md`, verify the manual contract (`:7-23`) requires updating to document structured ultrafixer packet dispatch and `blocked` envelope handling per `specs/dependencies-defects/spec.md:5-26`.
- [x] 3.4 GREEN: Update `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md` to specify automated ultrafixer packet generation (`ultrafixer-packet-template.md`) via `lucind-ai run --packet`, `base_sha` origin diffing, two-axis triage, isolated worktree repair, and human-gated CAS promotion (`lucind-ai integrate retry`). Terminal consumer: Human Orchestrator defect triage workflow. Depends on 3.2. Prove: verify documentation integrity and link consistency with git diff.

## Dependency order

| Task | Depends on | Why |
|------|------------|-----|
| 1.2 | 1.1 | TDD: v8 DDL |
| 1.4 | 1.2, 1.3 | Record/Get Go API needs v8 schema and RED test |
| 1.6 | 1.4, 1.5 | ListDefects Go API needs DefectRecord struct and RED test |
| 2.2 | 2.1 | CLI router needs RED test |
| 2.4 | 1.4, 2.2, 2.3 | `defect record` CLI needs `RecordDefect` and CLI routing |
| 2.6 | 1.6, 2.2, 2.5 | `defect list` CLI needs `ListDefects` and CLI routing |
| 3.2 | 3.1 | Packet template asset needs template contract RED test |
| 3.4 | 3.2, 3.3 | Coordination reference doc update needs packet template asset and RED review |

Shared `schema.go` / `ledger.go` / `cli.go`: no extra table edges. One packet, sequential commits; `allowed_paths` does not apply.

## Threat-matrix RED tests

Only Documentation-like paths, Git repository selection, and Commit state are Applicable (`design.md:294-302`). Push state and PR commands are N/A. Schema migration tests (1.1) are TDD for v8.

| Adversarial case | RED task | Asserts | Precedes |
|------------------|----------|---------|----------|
| Documentation-like paths | 3.1 | Ultrafixer operates strictly against provided failing check command; template enforces `allowed_paths` | 3.2 |
| Git repository selection | 3.1 | Ultrafixer operates strictly inside isolated worktree (`../<repo>-worktrees/fix-<slug>`); candidate signal reproduction is read-only | 3.2 |
| Commit state | 3.1, 2.3 | Repair commits require conventional formatting with zero AI attribution; `defect record` validates required fields | 3.2, 2.4 |

## Requirement traceability

| Requirement | Tasks |
|-------------|-------|
| Ledger schema v8 persistence for defect records (`specs/defect-records/spec.md:5-20`) | 1.1, 1.2 |
| Non-critical non-blocking defect persistence (`specs/defect-records/spec.md:21-36`) | 1.3, 1.4 |
| Defect record query and retrieval API (`specs/defect-records/spec.md:37-52`) | 1.3, 1.4, 1.5, 1.6 |
| CLI defect inspection commands (`specs/defect-records/spec.md:53-68`) | 2.1, 2.2, 2.3, 2.4, 2.5, 2.6 |
| Structured ultrafixer defect triage coordination (`specs/dependencies-defects/spec.md:5-26`) | 3.1, 3.2, 3.3, 3.4 |
| Origin classification via base_sha diffing (`specs/ultrafixer-dispatch/spec.md:5-20`) | 3.1, 3.2, 3.3, 3.4 |
| Independent two-axis evaluation and multi-branch triage (`specs/ultrafixer-dispatch/spec.md:21-36`) | 3.1, 3.2, 3.3, 3.4 |
| Signal reproduction for cross-branch impact (`specs/ultrafixer-dispatch/spec.md:37-52`) | 3.1, 3.2, 3.3, 3.4 |
| Isolated repair delivery and human-gated CAS integration (`specs/ultrafixer-dispatch/spec.md:53-74`) | 3.1, 3.2, 3.3, 3.4 |
| Multi-branch blocked disposition encoding (`specs/ultrafixer-dispatch/spec.md:75-84`) | 3.1, 3.2, 3.3, 3.4 |

## Schema v8 DDL

Verbatim from `design.md:149-163`. Additive schema migration using `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`:

```sql
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
