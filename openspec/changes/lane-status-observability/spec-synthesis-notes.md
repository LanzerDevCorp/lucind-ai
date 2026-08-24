# Spec Synthesis Notes: Lane Status Observability

## Unresolved Contradictions

None. The three Assumed requirements blocks partition the six capabilities without overlap on requirement text. Lens A and Lens C independently classified `lane-execution` and `read-only-packet-schema` as ADDED; live specs confirm that classification. Lens B owns the two new observability capabilities; Lens C owns orphan reconciliation and `batch-wave-view`. No draft asserted a fact the others contradicted.

## Coverage Gaps

- **Open questions left open (quiet-decision corrections).** Lens A scenarios used working frontmatter names `sdd_phase`, `fanout_group`, and `skill`; the delta requires optional SDD-phase, fanout-group, and skill keys without locking names (proposal OQ 1). Lens B required preserving the packet-path association without choosing JSON field vs `lanes` column (OQ 2 stays open). Lens B also hardcoded GET `/api/runs/{run_id}/lanes/{lane_id}/packet` and `Content-Type: text/markdown`; those are not among the five named open questions, but they are HOW, so the delta keeps “packet body endpoint / verbatim markdown.” Sweep ticker interval, PID-liveness syscall, and DAG `Node` / `EmitPacketContent` scope (OQ 3–5) are not answered.
- **Skill vs packet (format vs execution).** `sdd-spec` Step 2 would write new domains to `openspec/specs/`; this packet writes the change-folder tree. Skill Step 4 agrees with the change folder. Skill 650-word cap vs packet 1800-word authored budget: packet execution wins (authored tree 1289 words, 91 copied MODIFIED-scenario words excluded). Skill Step 5 Engram persistence of the spec artifact and Step 6 return block are superseded by this packet.
- **`batch-wave-view` has no independent second opinion.** Lens C is the only draft. The synthesizer verified the live requirement `Batch and DAG Wave Inspection` at `openspec/specs/batch-wave-view/spec.md:9-29` and copied both live scenarios verbatim before adding two new ones.
- **Not invented.** No draft specified DAG-wave packet-body routing, JSON names for tool-call counts beyond the dashboard’s existing `tool_rate` consumer, or a serve-side sweeper call site (none exists yet; `handlers.go:190` is only the mux registration seam).

## Dropped Citations

**Counts.** 80 citation-manifest rows (A 17, B 18, C 45) opened in this worktree. Unique ranges: 65 (63 verified, 1 dropped, 1 retargeted). Known-wrong propose-phase citations (`schema.go:310-330` as v7 DDL, `handlers.go:33-60`/`30-120` as packet-route/sweep, `server.go:1-60`/`19-53` as sweep/PID/ticker, `lanes.go:35-50`, `server_test.go:42-93` as sweep coverage, `schema.go:298-308` as `runs.pid`, `runs.go:103-137` as backfill policy, `lane-envelope-inspector` as packet-body capability) did not resurface.

**Dropped.** `internal/serve/static/app.js:200-249` (Lens B: “dashboard lane card packet link”). Those lines are approval-card header rendering (`normalizeApproval` / `createApprovalCard` packet id). Fleet lane cards that show model/phase/fanout live at `createFleetCard` (~564–595). The packet-body HTTP requirement is kept on verified CLI mapping (`cli.go:160-174`) and mux registration (`handlers.go:190`); the UI packet *link* is required only under `batch-wave-view`, as new behavior.

**Retargeted.** `internal/packet/packet_test.go:15-67` (Lens A: delimiter splitting, field extraction, *and* absent-key handling). Lines 15–67 cover delimiter splitting and present-key extraction; absent-key behavior is `TestParseModelAbsentLeavesFieldEmpty` at `:69+`. Requirement kept.

**Verified (unique ranges, claims held).**
- Ledger metadata: `lanes_meta.go:12,20-32,39,67-77,89,39-83,89-100`; `lanes_meta_test.go:15-70`.
- Packet parse: `packet.go:33-75,78-167,94-138`.
- Dispatch seams without `UpdateLaneMetadata`: `run.go:334,334-344,355`; `batch.go:184,184-193`.
- Serve DTOs/list: `model.go:163-184,187-193,288-301`; `model_test.go:599-670`; `handlers.go:190`.
- CLI: `cli.go:160-174` (path mapping), `:314-321` (`RegisterRun` without PID).
- Progress/executors: `executor.go:17-21`; `progress.go:15-20`; `progress_test.go:14-49`; `agy_stream.go:12-39,160-162`; `claude_stream.go:17-36,212-218`; `opencode_stream.go:100-125,226-228`; `cursor_agent.go:1-60`; `cursor_agent_stream.go:1-50`.
- Schema (correct `lane_progress` use of 298–308, not `runs.pid`): `schema.go:10,182-219,221-308,226-234,298-305,298-308`.
- Runs/status/notes: `runs.go:16-24,29-40`; `ledger.go:443,452-475`.
- UI telemetry/unavailable: `app.js:532-538,542-544`.
- DAG (OQ 5 evidence only): `parse.go:21-37`; `emit.go:11-60`.
- Live specs: `lane-execution/spec.md:1,10,27,44`; `read-only-packet-schema/spec.md:1,9,28,47,61,75`; `batch-wave-view/spec.md:1,9`.
- Proposal: `proposal.md:1,37-46,138-151`.

## Requirement Divergence

- **Classification.** Lens C’s live-spec audit **confirmed** Lens A’s ADDED calls. Live `lane-execution` is three approval-wait requirements (`Gate Placement in the Lifecycle`, `Resolve Before Barrier Observation`, `Additive Schema, Unchanged Enum`); none mention `UpdateLaneMetadata`. Live `read-only-packet-schema` is five `read_only` requirements; none mention SDD-phase / fanout-group / skill keys. No classification correction. Independent corroboration: A and C both ADDED; B and C both cited `schema.go:298+` as `lane_progress` (not `runs.pid`) and `handlers.go:190` as mux registration.
- **`batch-wave-view`.** Sole author Lens C. Synthesizer compared C’s MODIFIED block to `openspec/specs/batch-wave-view/spec.md:9-29`. Requirement name `Batch and DAG Wave Inspection` matches. Both live scenarios copied verbatim. Requirement text extended with metadata, packet-body link, telemetry, and swept-orphan notes; Previously line added; two new scenarios added. Full block shipped (not partial).
- **Schema v7.** One migration, stated under `orphan-lane-reconciliation` (Lens C described both STRICT rebuilds). `lane-progress-telemetry` references that same v7 migration rather than inventing a second story.
