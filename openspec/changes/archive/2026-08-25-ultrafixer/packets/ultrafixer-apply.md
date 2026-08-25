---
id: ultrafixer-apply
executor: agy
routed_by: SDD simple (no fan-out) phase dispatch for feature/ultrafixer — apply phase (single packet, sequential, per tasks.md's own recommendation), following tasks (integrated at 92b2ae8).
allowed_paths: ["internal/ledger/schema.go", "internal/ledger/schema_test.go", "internal/ledger/ledger.go", "internal/ledger/ledger_test.go", "cmd/lucind-ai/cli.go", "cmd/lucind-ai/cli_test.go", "internal/packet/packet_test.go", "plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md", "plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md", "openspec/changes/ultrafixer/tasks.md", "openspec/changes/ultrafixer/state.yaml"]
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: 92b2ae89c1945da93145932d684b4712c11123e9
expected_parent_sha: 92b2ae89c1945da93145932d684b4712c11123e9
---

# Packet ultrafixer-apply

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/ultrafixer-apply  ·  **Branch:** lucind/ultrafixer-apply

## Goal

Implement all three work units from `openspec/changes/ultrafixer/tasks.md` in this single packet,
sequentially, Strict-TDD (RED then GREEN for every step, no exceptions):

1. **Unit 1** — Ledger schema v8 (`defect_records` table) + Go `Ledger` API
   (`DefectDisposition`, `DefectRecord`, `RecordDefect`, `GetDefect`, `ListDefects`).
2. **Unit 2** — CLI `lucind-ai defect record` / `lucind-ai defect list` subcommands.
3. **Unit 3** — New packet template asset
   `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md` (with a parse
   contract test) + updated
   `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md`.

Mark every completed task checkbox `[x]` in `openspec/changes/ultrafixer/tasks.md` as you finish
it. Update `openspec/changes/ultrafixer/state.yaml`'s `phases.apply` to `status: done`, recording
`size:exception` per the Review Workload Guard (tasks.md's own forecast: 900-1150 estimated
changed lines against the 1200-line `single-pr` budget — within budget, but the guard still
requires size:exception to be recorded on record since it exceeds the standard 400-line
threshold; this was already anticipated in tasks.md's own Review Workload Forecast note).

## Why this is safe to dispatch now

`tasks.md` (committed at this packet's `base_sha`) already fully specifies every RED/GREEN step
with exact `file:line` seams, dependency order, and `Prove:` commands. `design.md` gives the exact
DDL and Go signatures. The three delta specs under `openspec/changes/ultrafixer/specs/` give the
exact MUST/MUST NOT behavior each implementation must satisfy. Nothing here requires new design
judgment — follow tasks.md's own ordering (Unit 1 → Unit 2 → Unit 3) exactly, since Unit 2 depends
on Unit 1's `Ledger` methods and Unit 3.4 depends on Unit 3.2.

## Preconditions

- `openspec/changes/ultrafixer/proposal.md`, `design.md`, `specs/*/spec.md`, and `tasks.md` all
  exist at this packet's `base_sha`. Read `tasks.md` in full — it is your primary task list, task
  numbering and ordering below must be followed exactly as written there.
- This repo runs Strict TDD Mode. Test runner: `go test ./...`. Every implementation step MUST be
  preceded by its own failing test (RED), committed or at minimum verified failing, before writing
  the minimal code to pass (GREEN). Do not write GREEN code before RED test exists and fails for
  the right reason.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** For the
      new `defect_records` table, `Ledger` methods, CLI verbs, and packet template asset: name and
      prove the terminal consumer for each, per `tasks.md`'s own citations.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and `git log --oneline
      -1`. Conventional commit(s), no AI attribution. Prefer one commit per work unit (three
      commits total) matching `tasks.md`'s "three work-unit commits" framing, unless that is
      impractical — a single well-described commit is acceptable if cleaner, but state which you
      did in your result summary.
- [ ] `go build ./...` succeeds.
- [ ] `go vet ./...` is clean.
- [ ] `gofmt -l .` reports no files (formatting clean).
- [ ] `go test ./... -race -count=1` passes in full — not just the new packages, the WHOLE suite,
      to prove the v8 migration and new code do not regress anything.
- [ ] Every task checkbox in `openspec/changes/ultrafixer/tasks.md` that you completed is marked
      `[x]`.
- [ ] `openspec/changes/ultrafixer/state.yaml`'s `phases.apply` is updated to `status: done` with
      a `note:` naming: which units completed, the commit SHA(s), the final test evidence command
      and result, and explicit confirmation that `size:exception` was recorded per the Review
      Workload Guard (delivery_strategy: single-pr, estimated 900-1150 lines, 1200-line budget).

## Allowed paths

- `internal/ledger/schema.go`
- `internal/ledger/schema_test.go`
- `internal/ledger/ledger.go`
- `internal/ledger/ledger_test.go`
- `cmd/lucind-ai/cli.go`
- `cmd/lucind-ai/cli_test.go`
- `internal/packet/packet_test.go`
- `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`
- `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md`
- `openspec/changes/ultrafixer/tasks.md`
- `openspec/changes/ultrafixer/state.yaml`

## Allowed paths outside the repository

None.

## Out of scope

- Any file not listed in Allowed paths above.
- Writing `verify.md` or archiving — later phases, dispatched separately.
- Modifying `explore.md`/`proposal.md`/`design.md`/`specs/` (read-only inputs) or
  `/home/lanzerdev/.claude/agents/lucind-ai-fixer.md` (never touch it).
- Any behavior beyond exactly what `tasks.md` and `design.md` specify — no extra CLI flags, no
  extra ledger columns, no refactor of unrelated code.
- Pushing, opening a PR, or running `lucind-ai integrate` yourself — this Lane only commits; the
  Orchestrator handles CAS promotion after this Lane reports `done`.

## Hard stops

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and `tasks.md`/`design.md` do not say which.
- Satisfying one instruction in this packet would require violating another.
- `go test ./... -race -count=1` fails for a reason unrelated to your own new code (a pre-existing
  flake or regression) — stop, report the exact failure, do not attempt to fix unrelated code
  (that would be scope creep outside `allowed_paths`).
- A `file:line` citation in `tasks.md`/`design.md` no longer matches the actual current file at
  this `base_sha` in a way that changes correct placement — stop and name the discrepancy rather
  than guessing.

## Context

### tasks.md's complete task list (authoritative — follow exactly, in this order)

**Unit 1 — Schema migration and Ledger API** (Phase 1, tasks 1.1-1.6):
- 1.1 RED: `internal/ledger/schema_test.go` — `TestMigrateV7ToV8PreservesRowsAndAddsSchema`,
  `TestSchemaV8ConstraintsAndIndexes`, `TestSchemaV8ReopenIsIdempotent`, `createV7SchemaFixture`
  helper.
- 1.2 GREEN: `internal/ledger/schema.go` — bump `schemaVersion = 8` (`:10`), add
  `migrateV7ToV8DDL` (near `:359`, following the existing `migrateV6ToV7DDL` pattern exactly), wire
  the `currentVersion < 8` step in `migrate` (near `:458-472`).
- 1.3 RED: `internal/ledger/ledger_test.go` — `TestLedgerRecordAndGetDefect`,
  `TestLedgerRecordDefectRejectsInvalidDisposition`.
- 1.4 GREEN: `internal/ledger/ledger.go` (append near end, current file is 1435 lines) —
  `DefectDisposition` type + constants (`DefectDispositionRecorded`, `DefectDispositionRepaired`,
  `DefectDispositionDeclined`, `DefectDispositionDeferred`), `DefectRecord` struct, `RecordDefect`,
  `GetDefect`.
- 1.5 RED: `internal/ledger/ledger_test.go` — `TestLedgerListDefects` (filters by `feature_id`,
  orders by `created_at ASC`, empty slice for no records).
- 1.6 GREEN: `internal/ledger/ledger.go` — `ListDefects`.

**Unit 2 — CLI defect inspection commands** (Phase 2, tasks 2.1-2.6):
- 2.1 RED: `cmd/lucind-ai/cli_test.go` — `TestDefectSubcommandUnknownAction`,
  `TestDefectListCLIRequiresFeature`.
- 2.2 GREEN: `cmd/lucind-ai/cli.go` — update `usage` const (`:56`), add `case "defect":` to the
  `run` switch (`:140-164`), define `defectDispatch` routing `list`/`record`.
- 2.3 RED: `cmd/lucind-ai/cli_test.go` — `TestDefectRecordCLI`, `TestDefectRecordCLIRequiresFlags`
  (required flags: `--id`, `--feature`, `--signature`; also needs `--evidence`, `--disposition`
  per the CLI signature named in tasks.md).
- 2.4 GREEN: `cmd/lucind-ai/cli.go` — `runDefectRecord` using `Ledger.RecordDefect`.
- 2.5 RED: `cmd/lucind-ai/cli_test.go` — `TestDefectListCLI` (`lucind-ai defect list --feature
  <id>` prints ID, signature, disposition, timestamp).
- 2.6 GREEN: `cmd/lucind-ai/cli.go` — `runDefectList` using `Ledger.ListDefects`.

**Unit 3 — Packet template asset and coordination doc** (Phase 3, tasks 3.1-3.4):
- 3.1 RED: `internal/packet/packet_test.go` — `TestUltrafixerPacketTemplateContract` (parses
  `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md` via `packet.Parse`,
  asserts frontmatter `executor: agy`, `routed_by: pre-existing defect triage and repair`, `model:
  gemini-3.7-flash-high`, and required body sections).
- 3.2 GREEN: create `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`
  matching the exact markdown already drafted in `design.md`'s Interfaces/Contracts section (read
  it there — do not redesign the template, transcribe it faithfully, adjusting only if the parse
  contract test in 3.1 reveals a real mismatch with how `packet.Parse` actually works).
- 3.3 RED: confirm `plugin/claude-code/skills/lucind-ai/references/coordination/
  dependencies-defects.md`'s current manual-only contract (lines ~7-23) needs updating against
  `openspec/changes/ultrafixer/specs/dependencies-defects/spec.md`.
- 3.4 GREEN: update `dependencies-defects.md` to describe structured ultrafixer packet dispatch,
  `base_sha` origin diffing, two-axis triage, isolated worktree repair, and human-gated CAS
  promotion — per the `dependencies-defects` delta spec's exact MUST/MUST NOT language, so the doc
  and the spec do not drift apart.

### design.md's exact DDL (for task 1.2 — transcribe exactly)

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

### design.md's exact Go API shape (for task 1.4/1.6 — adapt to this file's actual existing
conventions, e.g. how other `Ledger` methods open transactions/handle context, by reading a
neighboring method first)

```go
type DefectDisposition string

const (
	DefectDispositionRecorded DefectDisposition = "recorded"
	DefectDispositionRepaired DefectDisposition = "repaired"
	DefectDispositionDeclined DefectDisposition = "declined"
	DefectDispositionDeferred DefectDisposition = "deferred"
)

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

func (l *Ledger) RecordDefect(ctx context.Context, rec DefectRecord) error
func (l *Ledger) ListDefects(ctx context.Context, featureID string) ([]DefectRecord, error)
func (l *Ledger) GetDefect(ctx context.Context, id string) (DefectRecord, error)
```

### Full text of `design.md`'s drafted `ultrafixer-packet-template.md` (for task 3.2 — this exact
content already exists committed in `design.md`'s Interfaces/Contracts section at this packet's
`base_sha`; open `openspec/changes/ultrafixer/design.md` and copy the fenced ```markdown block
under "Ultrafixer Packet Template" verbatim into the new asset file, adjusting only what the 3.1
parse-contract test proves is actually required by `packet.Parse`'s real frontmatter schema
(e.g. confirm whether `base_sha`/`parent_ref` without `feature`/`expected_parent_sha` parses
validly as a template placeholder file, since this is a TEMPLATE with `<placeholder>` values, not
a real dispatchable packet — it may need to be excluded from strict frontmatter validation the way
`assets/packet-template.md` itself already is, since that file also contains `<id>` placeholders
and is not itself a real packet; check how `packet.Parse` or its test suite already handles the
existing template assets, if at all, and follow that precedent).**

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well
the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
