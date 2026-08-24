# Synthesis Notes: Lane Status Observability

## Unresolved Contradictions

None

## Coverage Gaps

None. All nine spine items are in `proposal.md`. The sdd-propose skill's 450-word cap and Intent/Scope template are superseded by this packet's 1800-word budget and spine; that is packet precedence, not a missing spine item. Dedicated "Dependencies" and "Success Criteria" headings from the skill template are folded into Approach and Delta Specifications rather than omitted.

## Dropped Citations

1. **Lens A — `internal/ledger/schema.go:310-330` as the place to introduce `migrateV6ToV7DDL`.** Those lines are the comment and start of `func migrate` (`:310-313`). There is no v7 DDL. The v7 *pattern* is kept via `schema.go:182-219,221-308`. Existing `migrate` transactional/idempotent behavior is kept at `schema.go:310-409` (Lens C's `:313-409` claim, which does resolve). Dropped: `:310-330` as a v7 DDL seam.

2. **Lens B — `internal/serve/handlers.go:33-60` as the packet-body HTTP route and as the orphan-sweep seam.** Those lines are `ServerState` fields (`Approver`, `Runs`, `Lanes`, `Events`, `LaneProgress`). Route registration starts at `NewHandlerWithConfig` (`handlers.go:190`). Packet-body and sweep claims are explore ground truth and are kept **without** this citation. `handlers.go:190` is used in `proposal.md` as the existing mux (newly verified; not a retarget of `:33-60` to mean "the packet route exists").

3. **Lens C — `internal/serve/handlers.go:30-120` as an additive packet-body endpoint.** `:23-91` is `ServerState`; `:93-130` is `/api/state` pagination bounds. Same drop as (2). Additive-routes claim kept without this citation.

4. **Lens B/C — `internal/serve/server.go:1-60` and `:19-53` as orphan sweep, PID liveness, or ticker.** Those lines are `ListenAndServe` (loopback bind, `http.Server`, shutdown). No sweep, PID, or ticker exists. Serve-side sweep remains explore ground truth and is kept **without** this citation.

5. **Lens C — `internal/ledger/lanes.go:35-50` as `SetStatus` for false-positive sweep mitigation.** File does not exist. `SetStatus` is `internal/ledger/ledger.go:452`. Dropped; not retargeted. Running-transition seam kept at `internal/run/run.go:355`.

6. **Lens C — `internal/serve/server_test.go:42-93` as orphan-sweep tests.** `:47-93` is `TestBulkRequestBodyReturns400` (bulk approval rejected). Dropped as sweep-coverage evidence.

7. **Lens B — `internal/ledger/schema.go:298-308` as the `runs.pid` v7 seam** (orphan-reconciliation table row). `:298-307` is `CREATE TABLE lane_progress` (message/seq only). `runs` without `pid` is `schema.go:226-234`. Dropped as a `runs.pid` citation; kept for `lane_progress` telemetry columns.

8. **Lens C — `internal/ledger/runs.go:103-137` as "no historical v7 backfill".** That range is `RunIDsByRecentEvent` (recover run IDs when `runs` has no row). Related folklore, not a v7 backfill policy. No-backfill kept as packet/explore ground truth **without** this citation.

9. **Lens C — `internal/run/run_test.go:25-60` as proof `Execute` already calls `UpdateLaneMetadata`.** Those lines define `fakeExecutor`. Dropped as existing-coverage evidence; dispatch tests remain a required new layer.

10. **Lens C — `internal/run/batch_test.go:170-210` as `ensureLaneFailed` metadata coverage.** Those lines are `newBatchTestDeps`. Dropped the same way.

11. **Lens B — `openspec/specs/lane-envelope-inspector` as the packet-body capability.** That spec is demotion diagnosis (deviated lanes, offending paths, preserved worktree), not serving dispatched markdown. Capability remapped to new `dispatched-packet-body` plus `batch-wave-view`. Not a file:line miss; recorded here so the orchestrator does not send specs at the wrong file.

## Scope Divergence

Lens A's Candidate 1 (one PR, `size:exception`, six items) is authoritative. Lens B did not propose a competing candidate; its impact table and delta specs assume the same six items (metadata wiring, frontmatter, packet body, structured telemetry, v7, PID sweep). Lens C independently treated the two-PR split as a rejected alternative and listed `size:exception` as an **accepted** review-budget risk (`explore.md:60-66`), corroborating A.

Independent convergence across all three: static `skill:` frontmatter only (no live Skill telemetry); no historical backfill; `cursor-agent` usage left empty; STRICT v7 create-copy-drop-rename combining `runs.pid` and `lane_progress` usage columns; `UpdateLaneMetadata` after `RegisterLane` at `run.go:334` and `batch.go:184`; sweeper lives in `serve`, not in `lucind-ai run`.

Content from B or C that did **not** enter `proposal.md` because it contradicted Lens A, closed a still-open explore question, or was process-meta:

- **Lens B** required `input_tokens`/`output_tokens`/`total_tokens`. Lens A (and `app.js:542-544`) specify `total_tokens`, `cost_usd`, and tool-call metrics. Synthesis follows A; per-decoder input/output remain an implementation detail.
- **Lens B** mapped packet-body serving onto `lane-envelope-inspector` (see Dropped Citations 11).
- **Lens C** proposed a 15–30s ticker and required verified `ESRCH` before marking failed. Those close Open Questions 3 and 4. Put back as open; C's false-positive *risk* is kept without locking interval or syscall.
- **Lens C** listed DAG-wave `Node`/`EmitPacketContent` changes as deferred to a follow-up, and listed cross-platform PID interrogation beyond Linux/POSIX as excluded. Both are Open Questions 5 and 4; the resolutions do not survive. Out of scope in `proposal.md` is the packet's conditional form (not in unless the question brings them in / not beyond what the question settles).
- **Lens A/B/C** each added a process open question that three-lens fan-out takes precedence over `~/.claude/skills/sdd-propose/SKILL.md:92-158`. Not a product question for this change; omitted from `proposal.md`.
