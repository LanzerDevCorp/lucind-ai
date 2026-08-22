# Synthesis Notes: Control Room Capture

## Unresolved Contradictions

1. **Destination field identifiers on `executor.Request`.** Lens B’s surface table names `StdoutDest` / `StderrDest`. Lens C’s new-seam list names `StdoutWriter` / `StderrWriter`. Lens A only says “stream destinations.” `Request` today has no dest fields (`internal/executor/executor.go:15-37`). The code does not settle the identifier. `design.md` describes destination `io.Writer` fields without locking a name.

## Coverage Gaps

- **Change-local specs are absent.** `openspec/changes/control-room-capture/specs/` does not exist in this worktree. The skill asks the design to reference specs; technical approach maps to proposal requirements and live `lane-execution` / `approvals-web-ui` capabilities instead. No spec IDs were invented.
- **Skill heading drift, not a missing spine item.** `~/.claude/skills/sdd-design/SKILL.md` names the section `Migration / Rollout` and budgets 800 words. This repository’s archived designs and this packet use `Rollback and Additivity` and a 1800-word ceiling. Substance is present under the repo heading. The skill’s Step 4 Engram persist and Step 5 return block are superseded by the packet.
- No draft specified a max log-file byte cap, secret redaction in captured stdio, or a default retention TTL. Those are product choices, not missing spine items.

## Dropped Citations

- **Lens A Decision 4: `internal/barrier/barrier.go:75-80` as SQLite coordinating barriers.** Those lines are in-memory `barrier.New` (empty-list rejection). SQLite records `barrier_released` later (`internal/run/batch.go:99-104`). Dropped “SQLite coordinates barriers”; kept `SetStatus` (`internal/ledger/ledger.go:452-475`) and `AppendEvent` (`internal/run/run.go:425-434`).
- **Lens A Decision 1: `internal/run/batch.go:147-152` as SQLite status writes contending with stream blobs.** Those lines are `barrier.Observe` (in-memory). Concurrent ledger writes are `batch.go:80-89` plus `Execute` itself. Citation dropped; `80-89` kept.
- **Lens A Decision 5: `docs/prd.md:188-193` as `run`/`serve` independence.** Those lines are PRD §8.1 (“one command: lucind-ai” / what the binary owns). They do not mention `serve`. Independence kept via `cmd/lucind-ai/cli.go:99-127,674-725`. Serve-as-binary is `docs/prd.md:219`.
- **Lens A Decision 4: `lucind-ai status` as the CLI diagnosis consumer.** No such subcommand exists (`cli.go:56` usage lists `run`/`split`/`check`/`serve`/`feature`/`reconcile`/`worktree`). Consumer is `printReport` (`cli.go:512-536`).
- **Lens C assumed architecture: SSE/download “via `serve.NewModel`” (`internal/serve/model.go:21-24`).** `NewModel` is `return &Model{ledger: l}` — SQL query mapping, not file tailing. Lens A already split disk-file HTTP (Decision 5) from Model JSON (Decision 6); C’s “via” claim was dropped.
- **Lens B Hop 1: `internal/ledgerpath/ledgerpath.go:44-59` as the log-file open site.** That span is `Validate`, which does not open files. Open site kept as `Execute` (`internal/run/run.go:368-374`); `Validate` kept as the path-boundary check (`:40-58`).
- **Lens B/C: `ledgerpath.Resolve` (`:34-38`) as the log-path function.** `Resolve` returns `<primaryRoot>/.lucind/lucind.db`. Threat-matrix response restated onto a new `ResolveLog` plus existing `Validate`.
- **Lens B file-changes: `cmd/lucind-ai/cli.go:129-173` as the executor `cmd.Run` consumer.** Those lines parse `--packet` files. `ExecuteBatch` is `cli.go:304`. Terminal consumer restated.
- **Lens C testing: `internal/run/run_test.go:675-710` as the 4096-byte `streamDetailCap` seam.** That span tests `OutputTruncated` ledger events, not the byte cap. Cap tests are `run_test.go:856-896` (oversize stderr) and `:1106-1128` (`Done` writes no stream note).
- **Lens C testing: `handlers.go:36-118` currently tests SSE streams and disconnects.** The mux serves `/`, `/api/state`, `/approvals/` only. SSE tests are new. `server_test.go:17-40` is loopback `ErrNonLoopback`, kept as the HTTP harness.
- **Lens C E2E: `cmd/lucind-ai/cli_test.go:37-80,1058-1150` as stub-child log creation.** `:37-48` is usage-only (file header `:27-30` says these fail before `Execute`). `:1058-1150` is `TestRunSequentialInvocationsProduceDistinctRunIDs` with `testDoneExecutor`, no log assertion. E2E restated as new CLI tests.
- **Lens C testing: `batch_test.go:66-100` as concurrent isolated-log coverage.** Those lines are `batchFakeExecutor` helpers. Concurrent ExecuteBatch is `:530-573`. Helper kept as a seam, not as existing log coverage.
- **Lens C testing: `integrate_test.go:20-80` as log preservation after integrate.** Those lines are `integrateRecorder`. Envelope persist + worktree remove is `TestCompleteIntegrationPersistsEnvelopeForEveryIntegratedLane` (`:392-435`).
- **Lens C rollback: `internal/result/result.go:43-135` as the envelope parser.** `:43-101` are supporting types; `Envelope`/`LaneStatus` are `:102-135`; `Read` starts at `:137`. Unchanged-contract claim kept on `:102-135` and `schema.go:10-28`.

## Architecture Divergence

All three assumed Candidate 4: hybrid primary-root file spooling, diagnosis-only `Outcome`, 4096-byte notes, loopback SSE/download, no daemon. Independent convergence on that shape is corroboration.

- **Lens C** conflated SSE/download with `serve.NewModel`. Dropped; A splits disk-file HTTP (Decision 5) from Model JSON (Decision 6).
- **Lens C** put “global server multiplexer / route ownership” in `control-room-serve` out of scope. **Lens B** left Model-route registration as an open question. **Lens A Decision 6** registers `/api/model/...` on `NewHandler` in this change. A owns architecture; B/C deferral did not enter `design.md`. Log SSE/download routes from B (`/api/runs/{runID}/lanes/{laneID}/tail` and `/log`) survive because B owns surface and A did not name those paths.
- **Lens C** named Request fields `StdoutWriter`/`StderrWriter`; **lens B** named `StdoutDest`/`StderrDest`. See Unresolved Contradictions.
- **Lens C** recommended interleaved `.log` with stream markers and gitignored-default retention. A left prefix, split, and retention open; those stay Open Questions, not decisions.
- Process-only open questions (skill 800-word / Engram / three-lens precedence) were omitted from `design.md`.
