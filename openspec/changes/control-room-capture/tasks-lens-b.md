# Tasks Lens B — Partition & Dispatch Shape: Control Room Capture

## Assumed decomposition

The implementation is partitioned into 4 standalone units across 3 layers: path validation, executor streaming, run integration, and HTTP tailing/serving. Unit 1 (`ledgerpath`) and Unit 2 (`executor`) provide foundational primitives for Unit 3 (`run`), which wires execution spooling into the primary repository. Unit 4 (`serve` & CLI) consumes the log path convention to expose SSE streaming, log downloads, and Model JSON. The critical path is Unit 1 + Unit 2 → Unit 3 → Unit 4.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Add primary-root log path resolution (`ResolveLog`) and worktree-rejection validation | `internal/ledgerpath/ledgerpath.go`<br>`internal/ledgerpath/ledgerpath_test.go` | `agy` | Restores `ledgerpath` to `Resolve` only; removes `ResolveLog` and path-traversal validation |
| 2 | Add destination `io.Writer` fields to `Request` and `MultiWriter` teeing across executors | `internal/executor/executor.go`<br>`internal/executor/agy.go`<br>`internal/executor/cursor_agent.go`<br>`internal/executor/opencode.go`<br>`internal/executor/agy_test.go`<br>`internal/executor/cursor_agent_test.go`<br>`internal/executor/opencode_test.go` | `agy` | Restores `Request` and executors to in-memory buffer capture only |
| 3 | Open append-only primary logs in `Execute`, pass destination writers, set `Report.LogPath`, and bound failure notes | `internal/run/run.go`<br>`internal/run/run_test.go`<br>`internal/run/batch_test.go`<br>`internal/run/integrate_test.go` | `cursor-agent` | Restores `Execute` to direct unspooled execution and in-memory diagnosis notes |
| 4 | Add SSE live tailing, log download, and Model JSON routes in `serve.NewHandler` with CLI wiring | `internal/serve/handlers.go`<br>`internal/serve/handlers_test.go` (new file)<br>`internal/serve/server_test.go`<br>`cmd/lucind-ai/cli.go`<br>`cmd/lucind-ai/cli_test.go` | `cursor-agent` | Restores approvals-only HTTP mux and 3-argument `NewHandler` signature in `cli.go` |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1, Unit 2 | Yes | Yes: `ResolveLog` and `Request.Stdout`/`Stderr` fields are additive with nil defaults, preserving existing execution and passing `lucind-checks.sh` |
| 2 | Unit 3, Unit 4 | Yes | Yes: Unit 3 wires spooling to Wave 1 primitives and Unit 4 exposes HTTP endpoints via `ledgerpath.ResolveLog`; both compile and pass all tests independently and combined |

## Disjointness Check

- **Wave 1 (Unit 1 vs Unit 2)**: Path set 1 (`internal/ledgerpath/*`) vs Path set 2 (`internal/executor/*`). Under `packet.PathInScope`, neither path set prefixes the other across component boundaries. Verdict: Disjoint.
- **Wave 2 (Unit 3 vs Unit 4)**: Path set 3 (`internal/run/*`) vs Path set 4 (`internal/serve/*`, `cmd/lucind-ai/*`). Under `packet.PathInScope`, neither path set prefixes the other across component boundaries. Verdict: Disjoint.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**: The entire change is ~400–500 lines across 8 production files and ~5 test files. Units 1 and 2 are small mechanical additions (~20–40 lines each) that do not justify the overhead of `apply-dag.yaml`, `lucind-ai split`, multi-lane worktrees, and per-wave bisection orchestration. Following the precedent in `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md`, changes under review budget with small early units are safer and faster dispatched sequentially as work-unit commits in a single packet. Furthermore, multi-wave apply DAGs introduce failure risks at the `Integrate` gate (`internal/run/integrate.go:50-59`), as documented in `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml`.

## Open Questions

- [ ] Skill contract drift: `~/.claude/skills/sdd-tasks/SKILL.md` describes a single monolithic `tasks.md` generation, which is superseded here by the 3-lens parallel task breakdown.
- [ ] Log path layout: Confirm `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log` and single interleaved file vs `.stdout.log`/`.stderr.log` before implementing Units 3 and 4.
