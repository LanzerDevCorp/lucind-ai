# Synthesis Notes: Lane Status Observability

## Unresolved Contradictions

None.

All three `## Assumed architecture` blocks describe complementary slices of one system (metadata + packet GET; telemetry + v7 DDL; PID capture + serve-side sweep). Proposal Candidate 1 and the six committed specs already chose that system. Open Questions 1–5 were each closed by the owning lens with a concrete choice; none of the five numbered proposal questions was carried forward as still-open. Lens B’s leftover “notice to C” about `pid = 0` matches Lens C Decision 5 rather than contradicting it. Lens A’s `LaneMetadata.Skill` / `PacketPath` additions do not collide with Lens B’s `ProgressEvent` / `LaneProgress` numeric fields or Lens C’s `Run.PID`.

## Coverage Gaps

- **`tool_rate` elapsed-time origin.** Lens B specified `float64(tool_calls) / max(elapsed_minutes, 1.0/60.0)` at `GetLaneProgress` but did not name the start instant (lane `StartedAt` vs first/last progress `At`). The canonical design used lane `StartedAt` → progress `At` so a cumulative count becomes a lifetime rate. That origin is an elaboration, not a lens sentence. Tasks should treat the formula and the 1s floor as closed, and the start instant as still implementer-specified unless they adopt `StartedAt`.
- Skill `sdd-design` Step 3 lists an 800-word budget, Engram persistence (Step 4), and a return summary block (Step 5). This packet’s 1800-word budget (DDL excluded), two-file output, and `.lucind/result.json` envelope supersede those on execution. Not a missing spine item: Technical Approach, Architecture Decisions (Choice / Alternatives / Rationale), Flow and Invariants, File Changes with terminal consumers, Interfaces / Contracts (including v7 DDL), Testing Strategy, Threat Matrix (every row Applicable or N/A: reason), Rollback and Additivity, and Open Questions / Out of Scope are all present. The skill’s published threat-matrix *table* has five rows; process integration is a trigger to include the matrix, not a sixth published row. The Process integration row is Lens C content assigned by this packet.

## Dropped Citations

Manifest union was the verification worklist. Unique ranges were opened once, batched by file. Outcomes below are drops and retargets; verified rows shipped (or informed a shipped sentence) and are not repeated.

1. **Lens B — `internal/executor/claude_stream_test.go:50-60` as “Test seam for claude stream-json telemetry decoding.”** Those lines are `TestClaudeRunSendsVerboseWithStreamJSON`: they assert `--output-format stream-json` and `--verbose` on argv (`:61-66`). They do not decode usage. Usage fixtures exist later in the same file (`:134`, `:160`, `:184`). Dropped as a telemetry-decoding seam. Canonical testing table points at claude usage fixtures after that argv test, not at `:50-60`.

2. **Lens C — `internal/serve/hub.go:213-235` as terminal consumer of Sweeper.Run (ticker, probe, `pid<=0` guard).** That range is `Hub.Run`: one immediate `poll` then a ticker (`:212-235`). No sweeper exists. Manifest claim “Hub.Run executes immediate poll then ticker” is verified and was kept as the *pattern*. Decision 3/4/5 terminal-consumer identification of this range as Sweeper.Run is dropped. C’s File Changes already create `internal/serve/sweeper.go`; that is the canonical location.

3. **Lens B — `internal/executor/cursor_agent_stream.go:1-218`.** File is 217 lines (`wc -l`). Retargeted to `1-217`. Tools-only decoder claim holds (`cursor_agent.go:239-270` emits `ProgressEvent{Message, At}` only).

4. **Lens A Decision 1 prose — `app.js:534-536` as currently rendering `skill`.** Those lines read `sdd_phase`, `fanout_group`, and `feature`, each falling back to `"Unavailable"`. They do not read `skill`. Manifest row (“reads sdd_phase, fanout_group, and feature”) is verified. Skill display is a planned delta at that site, not current behavior.

5. **Lens A Decision 3 prose — `handlers.go:190-357` as the packet-body route.** That range is `NewHandlerWithConfig` mux setup. No `/api/packets/` route exists. Manifest row (“sets up HTTP mux routes”) is verified. The packet GET is new work on that mux, not an existing handler.

Known-wrong citations from `proposal-synthesis-notes.md` / `spec-synthesis-notes.md` did not resurface as authority (`schema.go:310-330` as v7 DDL, `handlers.go:33-60` as packet route, `server.go` as sweep, `lanes.go`, `server_test.go:42-93` as sweep tests, `schema.go:298-308` as `runs.pid`, `runs.go:103-137` as no-backfill policy, `run_test.go:25-60` as existing `UpdateLaneMetadata` coverage, `lane-envelope-inspector` as packet-body capability, `app.js:200-249` as packet link).

`server_test.go:47-100` is `TestBulkRequestBodyReturns400` (HTTP 400 on bulk approval). Kept only as an httptest status/error pattern, explicitly not packet-GET coverage.

## Architecture Divergence

None — all three converged.

Independent agreement, arrived at without seeing sibling drafts: one STRICT v7 rebuild of `runs` (add `pid`) and `lane_progress` (add usage columns); no historical backfill; sweeper lives in `serve`, not in `lucind-ai run`; static `skill:` frontmatter only; cursor-agent zeros token/cost fields. Lens B’s `pid INTEGER NOT NULL DEFAULT 0 CHECK (pid >= 0)` and Lens C’s “skip `pid <= 0`” are the same untracked-row rule. Lens A’s `PacketPath` on `LaneMetadata` JSON matches the existing v6 split (queryable `lanes` columns vs snapshot JSON) that B and C did not reopen.

The only architecture-shaped mismatch was Lens C citing `Hub.Run` as if it were `Sweeper.Run` (Dropped Citations 2). That cost C the terminal-consumer sentences on Decisions 3–5, not the sweep design itself: File Changes still named `internal/serve/sweeper.go`, and the 10s interval / `kill(pid,0)` / zero-PID skip stand.
