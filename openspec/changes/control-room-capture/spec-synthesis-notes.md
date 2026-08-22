# Spec Synthesis Notes: Control Room Capture

## Unresolved Contradictions

None

## Coverage Gaps

- **Unreadable-envelope bound.** Lens A's Bounded SQLite diagnostics statement names failed, timed-out, and unreadable-envelope dispatches. Lens B supplied scenarios for clean Done, failed (50KiB clip), and timeout. No draft supplied a dedicated unreadable-envelope scenario. Not invented.
- **Retention, directory prefix, split vs interleaved.** All three lenses left these as open questions. Specs require files under `<primaryRoot>/.lucind/` and do not choose `runs/` vs `logs/`, one file vs `.stdout.log`/`.stderr.log`, or archive/prune policy. Proposal success criterion "archive/retention rule is explicit" is therefore unmet in the delta.
- **Model JSON routes.** Canonical design Decision 6 registers `/api/model/...` on `NewHandler` in this change. No spec lens named Model JSON as a requirement. A's Loopback HTTP stream access covers SSE tail and transcript download only. Not invented.
- **Route registration phase.** All three lenses asked whether log SSE/download register here or in `control-room-serve`. The requirement states that `lucind-ai serve` MUST expose them; it does not say which change's patch adds the mux entries.
- **Format drift vs `~/.claude/skills/sdd-spec/SKILL.md`.** That skill's "For NEW Specs" template uses `# {Domain} Specification` / `## Purpose` / `## Requirements`. This packet (and the archive merge contract) requires `# Delta for <capability>` with `## ADDED Requirements` under `openspec/changes/control-room-capture/specs/`. Canonical files follow the packet. Archive converts a new-capability delta into a live spec (title, Purpose, `## Requirements`) and must not `cp` the delta verbatim. Skill size budget is 650 words; this packet's authored ceiling is 1800 excluding copied MODIFIED blocks. There are no MODIFIED blocks. Authored delta-tree word count is 788 (control-room-capture 402, lane-execution 181, approvals-web-ui 205), over the skill's 650 and under the packet's 1800.
- **No REMOVED/RENAMED migration.** None were classified. Not a missing spine item.

## Dropped Citations

- **Lens B coverage table: `internal/run/run_test.go:820` as WaitDelay drain.** Line 820 is `TestExecuteUnreadableEnvelopeLedgerNoteCarriesStderr` (unreadable envelope ledger note), not WaitDelay. WaitDelay Execute coverage is `run_test.go:645` (`TestExecuteTruncatedOutcomeStillYieldsDoneAndReportsCapture`). Grandchild-pipe coverage is `internal/executor/agy_test.go:158`. WaitDelay scenarios kept.
- **Lens B coverage table: `internal/run/run_test.go:645` as Bounded SQLite diagnostics.** Line 645 is the truncation/WaitDelay Execute test, not the 4096-byte cap. Cap tests start at `run_test.go:856` (`TestExecuteOversizedStderrLedgerNoteIsBoundedAndKeepsTail`). `internal/run/run.go:132` (`formatStreamDetail`) does clip at 4096. SQLite scenarios kept.
- **Lens B coverage table: `internal/executor/agy_test.go:28` as spooling coverage.** Line 28 is `TestRunExitZero` (exit code only; in-memory buffers). Not current spooling coverage. Spooling scenarios kept; they rest on executor stdio assignment (`agy.go:169-173` and siblings) and worktree removal after integrate (`internal/run/integrate.go:159-163`).
- **Lens B coverage table: `internal/run/integrate_test.go:392` as log survival.** `TestCompleteIntegrationPersistsEnvelopeForEveryIntegratedLane` asserts envelope persist, not primary-root logs. Worktree removal is `integrate.go:159-163`. Log-survival scenarios kept as new behavior, not as existing test coverage.

## Requirement Divergence

All three lenses independently named the same four requirements: Continuous primary-root stream spooling, Non-interfering WaitDelay drain, Bounded SQLite diagnostics, Loopback HTTP stream access. Independent convergence is corroboration.

Lens A's assignment is authoritative and is what shipped: spooling and Bounded SQLite on new `control-room-capture`; WaitDelay on existing `lane-execution`; HTTP on existing `approvals-web-ui`. Lens B did not assign capabilities. Lens C described the same four as extending `lane-execution` and `approvals-web-ui` while establishing `control-room-capture`, without a conflicting split.

Lens C opened both live specs (`openspec/specs/lane-execution/spec.md`, 3 requirements / 6 scenarios; `openspec/specs/approvals-web-ui/spec.md`, 4 requirements / 9 scenarios) and found no conflicts. Verified: live requirements (gate placement, barrier observation, additive schema, loopback binding, individual decisions, inline evidence, approver rate) remain true. No ADDED-to-MODIFIED correction. No REMOVED or RENAMED.

Lens B content that did not enter the delta:

- **Non-loopback bind address rejected** (keyed to Loopback HTTP stream access). Restates unmodified live Loopback Binding (`openspec/specs/approvals-web-ui/spec.md` scenarios Loopback listen / Non-loopback rejected; `internal/serve/server.go:12-22`; `server_test.go:17`). Omitted; citation itself is valid.
- **Log path scenario identifiers.** B named `ledgerpath.Validate` and `ErrLedgerOutsidePrimaryRepo`. Those exist (`internal/ledgerpath/ledgerpath.go:23-27,40-58`) and `Validate` remains unwired (`ledgerpath.go:7-14`). Restated as WHAT (refuse a worktree-shaped destination). Not dropped.
- **Scenario HOW.** B's "tee'd" / `io.MultiWriter`, `exec.ErrWaitDelay`, `outputTruncatedDetail`, `text/event-stream`, and "leaking goroutines" were restated without those identifiers. Requirement statements from A, including `Outcome.OutputTruncated` / `Report.OutputCaptureIncomplete`, were kept.
