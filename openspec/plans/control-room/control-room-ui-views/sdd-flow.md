# SDD Flow — control-room-ui-views

All five planning phases run with three agy lenses plus cursor-agent synthesis. No phase is skipped. Materialize the staging target `feature: control-room-planning`, `parent_ref: refs/heads/control-room/planning`, live SHA fields, and each bullet's final path as its complete `allowed_paths`. The final wiring packet owns existing `app.js` and `cmd/lucind-ai/cli.go` so no same-wave packet races either file.

## Explore

- `explore-control-room-ui-views-lens-a` — agy — problem and candidates lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/explore-lens-a.md`
- `explore-control-room-ui-views-lens-b` — agy — capabilities and scenarios lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/explore-lens-b.md`
- `explore-control-room-ui-views-lens-c` — agy — risks, trade-offs, and spikes lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/explore-lens-c.md`
- `explore-control-room-ui-views-synthesis` — cursor-agent — synthesis of three parallel explore lenses into one canonical explore document — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-views/explore.md`, `openspec/changes/control-room-ui-views/explore-synthesis-notes.md`

## Propose

- `propose-control-room-ui-views-lens-a` — agy — candidate and approach lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/propose-lens-a.md`
- `propose-control-room-ui-views-lens-b` — agy — capability impact and delta specs lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/propose-lens-b.md`
- `propose-control-room-ui-views-lens-c` — agy — risks, rollback, and test impact lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/propose-lens-c.md`
- `propose-control-room-ui-views-synthesis` — cursor-agent — synthesis of three parallel propose lenses into one canonical proposal document — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-views/proposal.md`, `openspec/changes/control-room-ui-views/proposal-synthesis-notes.md`

## Design

- `design-control-room-ui-views-lens-a` — agy — decisions lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/design-lens-a.md`
- `design-control-room-ui-views-lens-b` — agy — surface-and-flow lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/design-lens-b.md`
- `design-control-room-ui-views-lens-c` — agy — failure-test-rollback lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/design-lens-c.md`
- `design-control-room-ui-views-synthesis` — cursor-agent — synthesis of three parallel design lenses into one canonical design — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-views/design.md`, `openspec/changes/control-room-ui-views/design-synthesis-notes.md`

## Spec

- `spec-control-room-ui-views-lens-a` — agy — capabilities and requirements lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/spec-lens-a.md`
- `spec-control-room-ui-views-lens-b` — agy — scenarios and coverage lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/spec-lens-b.md`
- `spec-control-room-ui-views-lens-c` — agy — live-spec conflict and migration lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/spec-lens-c.md`
- `spec-control-room-ui-views-synthesis` — cursor-agent — synthesis of three parallel spec lenses into the canonical delta spec tree — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-views/specs/`, `openspec/changes/control-room-ui-views/spec-synthesis-notes.md`

## Tasks

- `tasks-control-room-ui-views-lens-a` — agy — decomposition and ordering lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/tasks-lens-a.md`
- `tasks-control-room-ui-views-lens-b` — agy — partition and dispatch-shape lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/tasks-lens-b.md`
- `tasks-control-room-ui-views-lens-c` — agy — proof and review-burden lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-views/tasks-lens-c.md`
- `tasks-control-room-ui-views-synthesis` — cursor-agent — synthesis of three parallel tasks lenses into one canonical tasks.md — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-views/tasks.md`, `openspec/changes/control-room-ui-views/tasks-synthesis-notes.md`

## Target and post-planning phases

Planning packets use the staging target above and exact output scopes. Apply uses `feature: control-room-ui-views`, `parent_ref: refs/heads/control-room/control-room-ui-views`, and refreshed SHA fields; verify is boolean `read_only: true`; archive is one agy lane. No phase is skipped.
