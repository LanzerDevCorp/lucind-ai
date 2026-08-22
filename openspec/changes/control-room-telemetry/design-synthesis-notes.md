# Synthesis Notes: Control Room Telemetry

## Unresolved Contradictions

None.

## Coverage Gaps

- **Change-local specs directory missing.** Packet precondition named `openspec/changes/control-room-telemetry/specs/`. That directory is not in this worktree. Delta requirements live in `proposal.md` (`## Delta Specifications`); capability specs used as background are the repo-level files under `openspec/specs/` (`lane-execution`, `approvals-web-ui`, `parent-feature-integration`). Design proceeds from the accepted proposal, not from change-local spec files.

- **No HTTP route for lifecycle DTOs.** All three drafts add `serve.Model` queries over `Ledger.Events`. None specified a GET (or `/api/state` extension) that returns those DTOs. SSE covers live stream chunks only. Tasks will need a choice if the UI is to read run history over HTTP.

- **Skill 800-word budget vs packet 1800.** `~/.claude/skills/sdd-design/SKILL.md` sizes the artifact at 800 words. The packet and `openspec/changes/archive/` use 1800. Packet wins on execution; recorded here as schema-vs-execution drift, not a missing spine item.

- **Skill Engram Step 4 / return-block Step 5 superseded.** Packet output is `design.md`, these notes, and `.lucind/result.json`. Not a missing spine item.

- **Threat-matrix extra HTTP row.** Archived `approvals-web-ui` design added an Applicable loopback-HTTP row beyond the five-row skill template. Lens C kept the five template rows, all N/A. Canonical design does the same and points loopback RED tests at existing `IsLoopback` coverage rather than inventing a sixth row.

## Dropped Citations

Claims removed or re-cited because the listed `file:line` does not say what the draft claimed. Where the fact is true elsewhere, `design.md` uses the verified location.

- **`internal/run/run.go:348-351` as stream flush before persist (lens A Decision 5).** Those lines are `SetStatus(..., lane.Running)` *before* `exec.Run`. There is no flush today. Terminal persist is `internal/run/run.go:480-483`. Flush belongs after `:368-375` and before that `SetStatus`.

- **`internal/executor/agy.go:39` as `cmd.WaitDelay` (lens B).** Line 39 is the last word of the `defaultWaitDelay` comment (`down.`). The field is `agy.go:74-81`; assignment is `:167`.

- **`internal/executor/executor.go:52-62` as hang-until-deadline (lens B).** Those lines document `Outcome.OutputTruncated`. They do not say `Run` hangs. Hang-without-WaitDelay is `internal/executor/agy.go:15-23`.

- **`cmd/lucind-ai/cli.go:56` as CLI flags (lens B).** Line 56 is `var depsFactory = productionDeps`. Serve flags are `cli.go:683-685`; run flags are `cli.go:142-145`.

- **`cmd/lucind-ai/cli.go:641-660` as `completeIntegration` (lens B file-changes consumer).** Those lines are `productionDeps` closures for `RemoveLaneWorktree` and `PersistEnvelope`. `completeIntegration` is `internal/run/integrate.go:151-164` (calls them at `:161-162`).

- **`internal/executor/agy.go:10-38` as `writeStub` / `Binary` injection (lens C).** Those lines are `defaultBinary` and the `defaultWaitDelay` comment. `Agy.Binary` is `:68-72`. `writeStub` is `internal/executor/agy_test.go:18-26`.

- **`agy.go:12`, `cursor_agent.go:12`, `opencode.go:12` as WaitDelay fields (lens C).** Each `:12` is a package/const comment (`defaultCursorBinary` comment; `const defaultOpencodeBinary`). WaitDelay assignment is `agy.go:160-163`, `cursor_agent.go:82-85`, `opencode.go:121-124`.

- **`cursor_agent.go:10-23` / `opencode.go:10-38` as `writeStub` (lens C).** Production `Binary` fields exist; test helpers are `writeCursorStub` (`cursor_agent_test.go:18-26`) and `writeOpencodeStub` (`opencode_test.go:18-26`), not `writeStub`.

- **`internal/serve/handlers_test.go:18-40` (lens C).** File does not exist. Serve tests are `server_test.go`, `model_test.go`, `static_test.go`. Loopback coverage is `internal/serve/server_test.go:17-40`.

- **`internal/run/run_test.go:95-110` as `ledger.Open(":memory:")` + `fstest.MapFS` (lens C).** `newTestDeps` opens `ledger.Open(ctx, t.TempDir())` at `:91`. `fstest.MapFS` appears later (e.g. `:132`), not in that range. `:memory:` is `internal/ledger/ledger_test.go:102` (`openAtPath`) as a pragma-failure case, not the run-package seam.

- **`internal/result/result.go:10-40` as `Envelope` types (lens C).** Those lines are imports, `ErrSchemaInvalid`, `HardStop`, and the start of `FileChange`. `type Envelope struct` is `internal/result/result.go:102`.

- **`internal/ledger/ledger_test.go:733` as CHECK rejection of stream event types (lens C).** `TestMigrateIsIdempotent` reopens a migrated DB and checks version rows. It does not insert unadmitted `events.type` values. The CHECK itself is `internal/ledger/schema.go:38-39`; the RED test is new.

## Architecture Divergence

Lens A's assumed architecture is canonical: extend `executor.Request` with optional `io.Writer` sinks; preserve `Outcome` and `Executor.Run`; tee in `run.Execute` to a worktree log and an in-memory hub; leave SQLite v5 and six-value `lane.Status` unchanged; add loopback `/api/telemetry/events` and shell-free `serve.Model` DTOs over `Ledger.Events`.

**Independent convergence.** Lens B and lens C both opened with Candidate 2 (worktree-local logs + in-memory loopback SSE hub; `Request` writers; schema v5 unchanged; Model DTOs; `/api/telemetry/events`). They did not revive explore-era SQLite ingest. That is corroboration, not a second architecture.

**What B/C assumed that differed from A, and what that cost:**

- **Log archive (A closed; B/C left open).** A Decision 6 copies `<wt.Path>/.lucind/lane.log` to `.lucind/results/<lane-id>.log` before `RemoveLaneWorktree`. B asked results-vs-`.lucind/logs/<run-id>/`. C asked whether to archive at all, suggesting the logs path. Canonical design takes A's choice. B/C's archive open questions do not appear in `design.md`.

- **Hub injection surface (B).** B put `Hub *serve.Hub` on `run.Deps`. A specified writers on `Request` and a hub in `internal/serve`, not the Deps field. Compatible: Execute builds Request writers from an optional broadcast sink. `design.md` keeps the sink optional and nil-safe so `internal/run` is not required to import `internal/serve`; composition root is `serveDispatch` (`cmd/lucind-ai/cli.go:674-725`).

- **`ListRunEvents` name (B).** A said Model DTOs over `Events`. B named the method. Kept as B's surface; does not fight A.

- **Rollback ownership (C).** A did not own rollback. C's `git revert` / schema-stays-5 / additive-route story is the rollback section. No divergence from A's architecture.

- **C open question on threat-matrix source.** Process note (`~/.claude/skills/sdd-design/references/threat-matrix.md`), not an architecture fork. Omitted from `design.md` Open Questions.

None of these reverse Candidate 2. All three design lenses converged on files + SSE.
