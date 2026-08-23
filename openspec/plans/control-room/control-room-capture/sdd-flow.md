# SDD Flow — control-room-capture

All five planning phases run with three agy lenses and cursor-agent synthesis. No phase is skipped. Materialize the staging target `feature: control-room-planning`, `parent_ref: refs/heads/control-room/planning`, live SHA fields, and each bullet's final path as its complete `allowed_paths`. The stream-format evidence gap is carried into every relevant design/tasks lens and must not be filled by assumption.

## Explore

- `explore-control-room-capture-lens-a` — agy — problem and candidates lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/explore-lens-a.md`
- `explore-control-room-capture-lens-b` — agy — capabilities and scenarios lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/explore-lens-b.md`
- `explore-control-room-capture-lens-c` — agy — risks, trade-offs, and spikes lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/explore-lens-c.md`
- `explore-control-room-capture-synthesis` — cursor-agent — synthesis of three parallel explore lenses into one canonical explore document — `cursor-grok-4.6-high` — `openspec/changes/control-room-capture/explore.md`, `openspec/changes/control-room-capture/explore-synthesis-notes.md`

## Propose

- `propose-control-room-capture-lens-a` — agy — candidate and approach lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/propose-lens-a.md`
- `propose-control-room-capture-lens-b` — agy — capability impact and delta specs lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/propose-lens-b.md`
- `propose-control-room-capture-lens-c` — agy — risks, rollback, and test impact lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/propose-lens-c.md`
- `propose-control-room-capture-synthesis` — cursor-agent — synthesis of three parallel propose lenses into one canonical proposal document — `cursor-grok-4.6-high` — `openspec/changes/control-room-capture/proposal.md`, `openspec/changes/control-room-capture/proposal-synthesis-notes.md`

## Design

- `design-control-room-capture-lens-a` — agy — decisions lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/design-lens-a.md`
- `design-control-room-capture-lens-b` — agy — surface-and-flow lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/design-lens-b.md`
- `design-control-room-capture-lens-c` — agy — failure-test-rollback lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/design-lens-c.md`
- `design-control-room-capture-synthesis` — cursor-agent — synthesis of three parallel design lenses into one canonical design — `cursor-grok-4.6-high` — `openspec/changes/control-room-capture/design.md`, `openspec/changes/control-room-capture/design-synthesis-notes.md`

## Spec

- `spec-control-room-capture-lens-a` — agy — capabilities and requirements lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/spec-lens-a.md`
- `spec-control-room-capture-lens-b` — agy — scenarios and coverage lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/spec-lens-b.md`
- `spec-control-room-capture-lens-c` — agy — live-spec conflict and migration lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/spec-lens-c.md`
- `spec-control-room-capture-synthesis` — cursor-agent — synthesis of three parallel spec lenses into the canonical delta spec tree — `cursor-grok-4.6-high` — `openspec/changes/control-room-capture/specs/`, `openspec/changes/control-room-capture/spec-synthesis-notes.md`

## Tasks

- `tasks-control-room-capture-lens-a` — agy — decomposition and ordering lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/tasks-lens-a.md`
- `tasks-control-room-capture-lens-b` — agy — partition and dispatch-shape lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/tasks-lens-b.md`
- `tasks-control-room-capture-lens-c` — agy — proof and review-burden lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-capture/tasks-lens-c.md`
- `tasks-control-room-capture-synthesis` — cursor-agent — synthesis of three parallel tasks lenses into one canonical tasks.md — `cursor-grok-4.6-high` — `openspec/changes/control-room-capture/tasks.md`, `openspec/changes/control-room-capture/tasks-synthesis-notes.md`

## Target and post-planning phases

Planning packets use the staging target above and exact lens/synthesis output scopes. Apply uses `feature: control-room-capture`, `parent_ref: refs/heads/control-room/control-room-capture`, and refreshed SHA fields; verify is boolean `read_only: true`; archive is one agy lane. No phase is skipped.
