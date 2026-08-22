# Tasks Synthesis Notes: Control Room Serve

## Unresolved Contradictions

None

## Coverage Gaps

- Wave merge (step 5): Lens A's phases 2–5 cannot be separate Integrate waves. Changing `NewHandler` at `internal/serve/handlers.go:36` without `cmd/lucind-ai/cli.go:715` and `internal/serve/server_test.go:70,114,155,215` fails `go build ./...` (`lucind-checks.sh`). Merged into Unit 2 / Wave 2. Lens B already had this grouping; independently confirmed. Wave 1 (Unit 1) stays separate: additive ledger methods are green alone. Did not split 1.3 into a prior RED wave (would fail Integrate).
- Spec `Feature Listing and Inspection Endpoint` requires attempt history, active leases, and overlap evidence on `GET /api/v1/features`. `design.md:139` leaves that HTTP shape open (nested JSON, extra GETs, or deferred). No lens tasked it. Not invented. 2.3 uses `ListFeatures` / `GetFeature` (`model.go:128,152`), which return the `Feature` row only.
- `sdd-tasks` artifact cap is 530 words; this packet sets 1800. Packet wins. Canonical `tasks.md` uses the skill's Work Units columns (Likely PR, Focused test command, Runtime harness) plus lens B's executor and `allowed_paths`. No draft named a runtime harness; both units are `N/A` with reasons taken from lens C's "does not prove" column (no bounded `lucind-ai serve` harness; browser EventSource deferred to `control-room-ui-views`).
- Cross-process WAL between separate `run` and `serve` binaries, and browser `EventSource` reconnection, are lens C verification gaps. In-process `-race` in 5.4 is what the design testing strategy asked for; the rest is not tasked.

## Dropped Citations

- Lens A 1.1 `internal/ledger/ledger.go:285` as the site of `Runs`. That line is `Lanes` (`WHERE run_id = ? ORDER BY lane_id`). `Runs` does not exist (`design.md:75`). Task kept as a new neighbor of `Lanes`; existence-at-285 dropped.
- Lens A 1.2 `ledger.go:490,892` as `EventsSince` / `IntegrationEventsSince`. 490 is `Events` (`WHERE run_id = ?`); 892 is `IntegrationEvents` (`WHERE feature_id = ?`). Neither is `id > lastID`. Methods kept as new neighbors.
- Lens A 1.3 `ledger_test.go:432` as cursor-query tests. That line starts `TestAppendEventStoresRunScopedEventWithNullLaneID`. New tests kept; this location dropped.
- Lens A 3.1 `handlers.go:87` as the SSE handler. That line registers `POST /approvals/`. SSE kept as a new mux route on `NewHandler` (`handlers.go:36-118`).
- Lens A 4.2 / lens C `cli_test.go:1908` as `TestServeLinkedWorktreeRefusal`. 1908 is `TestServeNonLoopbackAddrRejectedAtCLI` (`0.0.0.0`). Production worktree guard is `cli.go:702-707`; the test is new. 1908–1917 kept as the existing loopback CLI test.
- Lens C `cli_test.go:1919` as a second proving test for CLI wiring. That line is `TestServeFlagsAndSubcommandRecognized` (invalid flag). Dropped.
- Lens C proving `-run 'TestLedgerRuns|TestLedgerEventsSince|TestLedgerIntegrationEventsSince'` derived from `ledger_test.go:432,490`. Those names do not exist (490 is `TestSetWorktreePreservedOnUnknownLaneErrors`). `go test -run` with zero matches exits 0. Names kept as tests to write in 1.3; the Unit 1 proving command is `go test ./internal/ledger`.
- Lens C `-run 'TestV1RESTEndpoints|TestUnmatchedAPIJSON404'` derived from `server_test.go:42,136`. 42 is `TestBulkRequestBodyReturns400`; 136 is `TestSingleApprovalAndDefectEndpoints`. Dropped as current commands. Coverage kept as new 5.2 tests; package proving command is `go test ./internal/serve`.
- Lens C `-run 'TestEventsStreamSSE'` derived from `server_test.go:17,42`. 17 is `TestNonLoopbackListenFails`; 42 is bulk 400. Dropped as a current command. Coverage kept as new 5.3.
- Lens C `-run 'TestServeLinkedWorktreeRefusal|TestServeNonLoopbackAddrRejectedAtCLI'` as proof that `serveDispatch` already passes `*Model`. The loopback test returns before `ledger.Open` and does not construct `NewHandler`. Second name does not exist. Wiring is 4.1 plus `go build`; worktree behavior is new 4.2.
- Lens C `-run 'TestConcurrentServeReadsAndBatchWrite'` derived from `ledger_test.go:367` and `server_test.go:42`. 367 is `TestConcurrentRegisterAndSetStatusAcrossDistinctLanes` (ledger pool, not serve REST/SSE vs `ExecuteBatch`). Dropped as a current command. Coverage kept as new 5.4.

## Decomposition Divergence

Lens A (authoritative): five sequential phases, 14 tasks, ledger queries → REST mux → SSE → CLI → tests. Critical path and the `NewHandler` coupling of `handlers.go`, `cli.go`, and `server_test.go` match the code.

Lens B assumed two sequential work units (ledger, then the whole HTTP/CLI/test surface) rather than five Integrate waves. Nothing from B mapped to a task A did not have. Cost: B did not enumerate A's 14 checklist lines; those come from A. Independent convergence: B's Unit 1 is A's phase 1; B's Unit 2 is A's phases 2–5; B rejected a handlers/CLI split for the same `NewHandler` compile break A named.

Lens C assumed three core units (ledger, handlers REST/SSE/404, CLI). The CLI-only unit maps to A's 4.1–4.2 but cannot be its own Integrate wave (same signature break). C's Review Workload Forecast PR split already folded handlers+CLI into PR 2, so the forecast converges with B even though the assumed-decomposition paragraph does not. Acceptance rows 1–5 map onto A's 1.x, 2.x, 3.1, 4.x, and 5.4. Invented `-run` names were not kept as proving commands (Dropped Citations).

Independent convergence across all three: ledger cursor/run queries first; `*Model` into `NewHandler`; `/api/v1/*` plus SSE; JSON 404 under `/api/`; linked-worktree refusal already in production; no threat-matrix RED tests.
