# Synthesis Notes: Control Room Telemetry

## Unresolved Contradictions

None. Remaining disagreements are open questions (archive path, SSE payload shape, coarse-milestone store) or approach-owned by lens A, not two current-state claims the code cannot break.

## Coverage Gaps

None. All nine proposal-spine items were present across the three drafts. After citation drops, test-impact rows were rewritten to verified test functions rather than left empty. Token/duration-as-ledger-columns were not filled in; they are out of scope or derived at `exec.Run`.

## Dropped Citations

Claims removed from `proposal.md` because the cited lines do not say what the draft claimed. Where the same fact exists elsewhere, the canonical doc uses the verified location instead of the failed citation.

- **SSE client-disconnect cleanup at `internal/serve/server.go:41-53` (lens B).** Those lines are whole-server `ctx.Done()` → `srv.Shutdown` (3s timeout). There is no per-client SSE handler or subscriber unregistration today. The disconnect requirement stays as proposed behavior on the new route, without that seam.

- **`internal/ledger/schema_test.go:15-80` (lens C).** File does not exist. The only ledger tests live in `internal/ledger/ledger_test.go`. Migration idempotency is `TestMigrateIsIdempotent` at `internal/ledger/ledger_test.go:733`. Schema version is `schemaVersion = 5` at `internal/ledger/schema.go:9-10`.

- **WaitDelay / MultiWriter coverage at `internal/executor/cursor_agent_test.go:70-110` (lens C).** Those lines are `TestCursorAgentRunCapturesStderr` and the start of `TestCursorAgentRunCapturesStdout`. Grandchild/`WaitDelay`/`OutputTruncated` coverage is `internal/executor/cursor_agent_test.go:178-205`.

- **WaitDelay / MultiWriter coverage at `internal/executor/opencode_test.go:60-105` (lens C).** Those lines are non-zero exit (code 17) and stderr capture. Grandchild/`WaitDelay` coverage starts at `internal/executor/opencode_test.go:178-205`.

- **`SQLITE_BUSY` / concurrent lease-renewal tests at `internal/ledger/ledger_test.go:120-180` (lens C).** Those lines are `RegisterLane` identity assertions and the start of `TestRegisterLaneRejectsMissingRoutingCondition`. No telemetry-vs-lease contention test exists there.

- **`SQLITE_BUSY` during concurrent renewals at `internal/feature/feature_test.go:210-250` (lens C).** Those lines finish fence increment after expiry and start `TestLeaseValidationAndStaleMutationRejection` (expired `ValidateLease` → `ErrLeaseExpired`). No WAL-contention benchmark. No `Test*RenewLease` exists under `internal/feature`.

- **Parallel live log streaming at `internal/run/run_test.go:45-120` (lens C).** Those lines are `fakeExecutor.KnownModels`, `doneEnvelopeJSON`, `testPacket`, and the start of `newTestDeps`. No streaming tests.

- **Live log streaming at `internal/run/batch_test.go:30-95` (lens C).** Those lines are `batchPacket`, `laneEnvelopeJSON`, and `batchFakeExecutor` field comments. Helpers for concurrency, not log teeing.

- **Live log streaming / barrier observation at `internal/barrier/barrier_test.go:20-60` (lens C).** Those lines are `assertStringSet` and the start of `TestEvaluate` ("all done releases"). Existing Evaluate coverage is real; it does not test telemetry streams. Proposal cites `Evaluate` itself (`internal/barrier/barrier.go:36-59`) as the invariant to preserve.

- **CLI e2e streaming / archiving at `cmd/lucind-ai/cli_test.go:80-160` (lens C).** Those lines are `-v` version printing and missing-`--packet` usage errors. Not log streaming, worktree archive, or report formatting. `printReport` is `cmd/lucind-ai/cli.go:512-540`.

- **Token metadata queries at `internal/serve/model_test.go:595-627` (lens C).** That range is `TestModelSourceDoesNotShellOut` (no `os/exec` / git in `model.go`). It does not query lane telemetry, duration, exit code, or tokens. `executor.Outcome` (`internal/executor/executor.go:42-63`) has `ExitCode`, `TimedOut`, `Stderr`, `Stdout`, `OutputTruncated` — no token fields. Token metadata is out of scope.

- **Execution-duration columns via `internal/ledger/ledger.go:488-526` (lenses A/B).** Those lines are `Ledger.Events`: insertion-ordered lifecycle rows (`id, run_id, lane_id, type, detail, at`). No duration field. Duration is observable at the `exec.Run` seam (`internal/run/run.go:368-375`). Proposal keeps event DTOs over `Events`, not a duration column.

- **Phase duration / check-output recording at `openspec/specs/parent-feature-integration/spec.md:33-45` (lens B).** Those lines are Immutable Starts and Serialized Promotion (CAS). Check output and `WriteWithAudit` are `internal/run/attempt.go:213-214,408-443`, `internal/integrate/integrate.go:90-109`, `internal/ledger/ledger.go:832-873`. The spec citation was dropped; the Go seams were kept.

- **`.lucind/result.schema.json:1-160` (lens C).** File ends at line 159. Claim (envelope schema unchanged) kept at `:1-159`.

- **Existing SSE tests at `internal/serve/handlers.go:36-85` (lens C test table).** Those lines are `NewHandler` registering `/`, `/api/state`, and `/approvals/` — production mux, not tests. Loopback tests are `internal/serve/server_test.go:17-40` (`TestNonLoopbackListenFails`). No SSE tests exist.

## Scope Divergence

Lens A's candidate and approach are canonical: Candidate 2 (worktree-local logs + in-memory SSE hub; SQLite for coarse lifecycle only).

**Independent corroboration.** Lens B's delta specs (tee + WaitDelay, loopback SSE, ledger isolation, shell-free Model, barrier-after-persist) match Candidate 2; they do not revive explore-era SQLite ingest. Lens C's risk table independently rejects high-frequency SQLite, CHECK-constraint event types, WebSockets/OTLP, and non-loopback bind — the same rejections as A's alternatives.

**What B/C assumed that differed from A, and what that cost:**

- **Milestone store (lens C).** C offered `integration_events` (`internal/ledger/schema.go:171-180`) as a place for coarse milestones. A keeps high-frequency data off SQLite and only considers in-memory hub vs optional additive v6 for coarse milestones. `integration_events.type` is unconstrained and feature-scoped (`WriteWithAudit` at `internal/ledger/ledger.go:832-873`), so routing lane streams there would mix attempt audit with telemetry. Proposal does not select `integration_events` for lane milestones; it lists the option under Open Questions as not recommended.

- **Token metadata (lens C).** C required Model tests for token metadata. A did not include JSON token parse in the approach. Code has no token fields on `Outcome`. Cost: token parse stayed out of scope.

- **Executor compatibility (lens C vs A).** A transitions `Request` to streaming writer sinks. C said `Executor.Run` and `Outcome` remain backward-compatible. Not irreconcilable: proposal keeps the `Run` signature and `Outcome` fields and adds optional `io.Writer` sinks on `Request`.

- **Optional schema v6 in C's rollback.** A's selected candidate does not require v6 (milestones are an open question). Rollback in `proposal.md` treats v6 as optional later additivity, not as this change's schema.

- **Archive path wording (lens C).** A and B ask about `.lucind/results/<lane-id>.log` beside `PersistEnvelope`. C also offers `.lucind/logs/<run-id>/`. Merged into one open question; not a candidate split.

None of these reverse Candidate 2. All three propose lenses converged on files + SSE.
