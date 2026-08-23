# SDD Flow — control-room-telemetry

All five planning phases run. Lenses are three concurrent agy packets; synthesis is a cursor-agent packet after the three terminal envelopes. `apply`, `verify`, and `archive` are single-lane follow-ons; none is skipped. Materialize `feature: control-room-planning`, `parent_ref: refs/heads/control-room/planning`, live SHA fields, and each bullet's final path as its complete `allowed_paths` because current admission requires target fields (`internal/run/run.go:267-285`).

## Explore

- `explore-control-room-telemetry-lens-a` — agy — condition: problem and candidates lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/explore-lens-a.md`
- `explore-control-room-telemetry-lens-b` — agy — condition: capabilities and scenarios lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/explore-lens-b.md`
- `explore-control-room-telemetry-lens-c` — agy — condition: risks, trade-offs, and spikes lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/explore-lens-c.md`
- `explore-control-room-telemetry-synthesis` — cursor-agent — condition: synthesis of three parallel explore lenses into one canonical explore document — `cursor-grok-4.6-high` — `openspec/changes/control-room-telemetry/explore.md`, `openspec/changes/control-room-telemetry/explore-synthesis-notes.md`

## Propose

- `propose-control-room-telemetry-lens-a` — agy — condition: candidate and approach lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/propose-lens-a.md`
- `propose-control-room-telemetry-lens-b` — agy — condition: capability impact and delta specs lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/propose-lens-b.md`
- `propose-control-room-telemetry-lens-c` — agy — condition: risks, rollback, and test impact lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/propose-lens-c.md`
- `propose-control-room-telemetry-synthesis` — cursor-agent — condition: synthesis of three parallel propose lenses into one canonical proposal document — `cursor-grok-4.6-high` — `openspec/changes/control-room-telemetry/proposal.md`, `openspec/changes/control-room-telemetry/proposal-synthesis-notes.md`

## Design

- `design-control-room-telemetry-lens-a` — agy — condition: decisions lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/design-lens-a.md`
- `design-control-room-telemetry-lens-b` — agy — condition: surface-and-flow lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/design-lens-b.md`
- `design-control-room-telemetry-lens-c` — agy — condition: failure-test-rollback lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/design-lens-c.md`
- `design-control-room-telemetry-synthesis` — cursor-agent — condition: synthesis of three parallel design lenses into one canonical design — `cursor-grok-4.6-high` — `openspec/changes/control-room-telemetry/design.md`, `openspec/changes/control-room-telemetry/design-synthesis-notes.md`

## Spec

- `spec-control-room-telemetry-lens-a` — agy — condition: capabilities and requirements lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/spec-lens-a.md`
- `spec-control-room-telemetry-lens-b` — agy — condition: scenarios and coverage lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/spec-lens-b.md`
- `spec-control-room-telemetry-lens-c` — agy — condition: live-spec conflict and migration lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/spec-lens-c.md`
- `spec-control-room-telemetry-synthesis` — cursor-agent — condition: synthesis of three parallel spec lenses into the canonical delta spec tree — `cursor-grok-4.6-high` — `openspec/changes/control-room-telemetry/specs/`, `openspec/changes/control-room-telemetry/spec-synthesis-notes.md`

## Tasks

- `tasks-control-room-telemetry-lens-a` — agy — condition: decomposition and ordering lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/tasks-lens-a.md`
- `tasks-control-room-telemetry-lens-b` — agy — condition: partition and dispatch-shape lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/tasks-lens-b.md`
- `tasks-control-room-telemetry-lens-c` — agy — condition: proof and review-burden lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-telemetry/tasks-lens-c.md`
- `tasks-control-room-telemetry-synthesis` — cursor-agent — condition: synthesis of three parallel tasks lenses into one canonical tasks.md — `cursor-grok-4.6-high` — `openspec/changes/control-room-telemetry/tasks.md`, `openspec/changes/control-room-telemetry/tasks-synthesis-notes.md`

## Target and post-planning phases

Every planning packet uses the staging target above and an `allowed_paths` set containing only its listed output. Apply uses `feature: control-room-telemetry`, `parent_ref: refs/heads/control-room/control-room-telemetry`, and refreshed SHA fields; verify uses boolean `read_only: true`; archive uses the archive template. No phase is skipped.
