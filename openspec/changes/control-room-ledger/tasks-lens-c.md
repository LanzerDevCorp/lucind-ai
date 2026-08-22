# Tasks Lens C — Proof & Review Burden: Control Room Ledger

## Assumed decomposition

Assumed 3 work units: Unit 1 implements schema v6 transactional migration and domain store files (`internal/ledger/schema.go`, `runs.go`, `lanes_meta.go`, `progress.go`, `events.go`); Unit 2 wires CLI run lifecycle registration and lane metadata persistence (`cmd/lucind-ai/cli.go`, `internal/run/run.go`, `internal/run/batch.go`); Unit 3 adds shell-free SQLite DTO query methods to `internal/serve/model.go`. The critical path runs through Unit 1 schema and domain methods, which unblocks concurrent implementation of Unit 2 CLI lifecycle writes and Unit 3 Serve model reads.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,200–1,600 lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Schema v6 & Ledger domain split) → PR 2 (CLI lifecycle & Lane metadata persistence) → PR 3 (Serve model DTOs) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Basis: ~750–900 lines across 8 production files (`schema.go` ~90, `ledger.go` extraction ~50, `runs.go` ~140, `lanes_meta.go` ~160, `progress.go` ~130, `events.go` ~110, `model.go` ~120, `cli.go`/`run.go` ~40) plus ~550–700 lines across 3 test suites (`ledger_test.go` ~450, `model_test.go` ~130, `cli_test.go` ~70). Derived from comparable schema migrations (`migrateV4ToV5DDL` in `internal/ledger/schema.go:190-219` and `internal/ledger/ledger_test.go:934-970`) and feature-store additions (`internal/ledger/ledger.go:816-930`).

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A: change does not classify or execute documentation/script paths | N/A | N/A | N/A |
| Git repository selection | Applicable | `TestRunRefusesLinkedWorktree` and `TestServeRefusesLinkedWorktree` in `cmd/lucind-ai/cli_test.go` | Subcommand in linked worktree exits with code 1 and stderr matching `"refusing to run from inside a linked worktree"` before `ledger.Open` | Task 2.1 (Linked worktree validation in `cmd/lucind-ai/cli.go:277-280,702-705`) |
| Commit state | N/A: change does not manipulate git index or commit state | N/A | N/A | N/A |
| Push state | N/A: change does not manage git push or remote refs | N/A | N/A | N/A |
| PR commands | N/A: change does not compose or run PR automation commands | N/A | N/A | N/A |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Schema v6 transactional migration | `go test -v -run 'TestMigrateV5ToV6Database' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:579`, `ledger_test.go:934`) | `runs` and `lane_progress` tables created; `lanes` and `events` rebuilt; v5 rows preserved; repeated `Open` is no-op | Does not prove concurrency safety under multi-process write saturation |
| First-class run persistence | `go test -v -run 'TestRegisterAndGetRun|TestUpdateRunStatus' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:24`, `ledger_test.go:43`) | `RegisterRun` sets `running` and UTC `started_at`; duplicate rejected; `UpdateRunStatus` records terminal status and `ended_at` | Does not prove CLI invokes methods at dispatch and completion boundaries |
| Lane dispatch metadata persistence | `go test -v -run 'TestRegisterLanePersistsMetadata' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:608-616`, `ledger_test.go:957-971`) | `RegisterLane` persists packet `model`, `agent`, and `feature` columns | Does not prove all runtime callers pass non-empty packet attributes |
| Event type constraint expansion | `go test -v -run 'TestEventsAdmitRunStatusChanged' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:571-575`, `ledger_test.go:618-630`) | `events` CHECK constraint admits `run_status_changed` and rejects unknown event literals | Does not prove audit log ordering across concurrent lane transitions |
| Progress stream append and cursor tail | `go test -v -run 'TestAppendProgressAndCursorTail' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:24`, `ledger_test.go:579-630`) | Monotonic append isolated from `events`; duplicate seq rejected; `GetProgressAfter` returns `seq > afterSeq` ascending | Does not prove client-side stream chunk rendering |
| Isolated progress cutoff pruning | `go test -v -run 'TestPruneProgressIsolated' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:1584-1600`) | Deletes `lane_progress` rows where `at < cutoff`; 0 rows deleted when all newer; leaves runs, lanes, events, approvals intact | Does not prove scheduler trigger mechanism (CLI vs serve background ticker) |
| Multi-connection pool concurrency | `go test -v -run 'TestConcurrentProgressAndLeaseContention' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:360-367`, `internal/run/attempt.go:434-441`) | Concurrent progress appends, lane status updates, and lease validations succeed without unhandled `SQLITE_BUSY` errors | Does not prove sub-millisecond tail latency under heavy disk I/O pressure |
| CLI run lifecycle dispatch wiring | `go test -v -run 'TestRunLifecycleRegistration|TestRunRefusesLinkedWorktree' ./cmd/lucind-ai` (derived from `cmd/lucind-ai/cli_test.go:212-255`, `cli_test.go:1060-1100`) | `lucind-ai run` inserts `runs` row at start, updates status after batch, and exits code 1 in linked worktrees | Does not prove unclean process crash/SIGKILL transitions status |
| Shell-free serve Model DTOs | `go test -v -run 'TestModelRunAndProgressQueries|TestModelSourceDoesNotShellOut' ./internal/serve` (derived from `internal/serve/model_test.go:585-627`) | Typed run summary and progress tail DTOs query SQLite directly; AST verifies zero `os/exec` or `git` imports/invocations | Does not prove frontend HTTP 2-second polling integration in `app.js:96-97` |

## Verification Gaps

- **Unclean process abort handling**: If `lucind-ai run` is terminated abruptly via `SIGKILL` or power loss, the `runs` row remains `running` indefinitely until an external reconciliation pass runs. Proving crash recovery requires a dedicated process termination test harness in `internal/run/`.
- **End-to-end frontend stream ingestion**: Proving live UI cursor consumption and DOM updates requires an end-to-end HTTP/browser test harness, which is deferred to downstream changes `control-room-serve` and `control-room-ui-views`.

## Open Questions

- [ ] Should `lane_progress` pruning execute via a background ticker in `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) or via a scheduled CLI command?
- [ ] Should `lane_progress.message` be constrained to structured JSON or remain unstructured text?
- [ ] Packet precedence notice: `~/.claude/skills/sdd-tasks/SKILL.md` prescribes full single-author `tasks.md` generation, which this 3-lens parallel synthesis packet explicitly supersedes.
