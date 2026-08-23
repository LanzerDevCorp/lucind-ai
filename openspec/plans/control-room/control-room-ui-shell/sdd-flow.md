# SDD Flow — control-room-ui-shell

All five planning phases run with three agy lenses plus cursor-agent synthesis. No phase is skipped. Materialize the staging target `feature: control-room-planning`, `parent_ref: refs/heads/control-room/planning`, live SHA fields, and each bullet's final path as its complete `allowed_paths`. The design must retain the project's no-build/go:embed constraint; do not invent an npm build step.

## Explore

- `explore-control-room-ui-shell-lens-a` — agy — problem and candidates lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/explore-lens-a.md`
- `explore-control-room-ui-shell-lens-b` — agy — capabilities and scenarios lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/explore-lens-b.md`
- `explore-control-room-ui-shell-lens-c` — agy — risks, trade-offs, and spikes lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/explore-lens-c.md`
- `explore-control-room-ui-shell-synthesis` — cursor-agent — synthesis of three parallel explore lenses into one canonical explore document — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-shell/explore.md`, `openspec/changes/control-room-ui-shell/explore-synthesis-notes.md`

## Propose

- `propose-control-room-ui-shell-lens-a` — agy — candidate and approach lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/propose-lens-a.md`
- `propose-control-room-ui-shell-lens-b` — agy — capability impact and delta specs lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/propose-lens-b.md`
- `propose-control-room-ui-shell-lens-c` — agy — risks, rollback, and test impact lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/propose-lens-c.md`
- `propose-control-room-ui-shell-synthesis` — cursor-agent — synthesis of three parallel propose lenses into one canonical proposal document — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-shell/proposal.md`, `openspec/changes/control-room-ui-shell/proposal-synthesis-notes.md`

## Design

- `design-control-room-ui-shell-lens-a` — agy — decisions lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/design-lens-a.md`
- `design-control-room-ui-shell-lens-b` — agy — surface-and-flow lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/design-lens-b.md`
- `design-control-room-ui-shell-lens-c` — agy — failure-test-rollback lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/design-lens-c.md`
- `design-control-room-ui-shell-synthesis` — cursor-agent — synthesis of three parallel design lenses into one canonical design — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-shell/design.md`, `openspec/changes/control-room-ui-shell/design-synthesis-notes.md`

## Spec

- `spec-control-room-ui-shell-lens-a` — agy — capabilities and requirements lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/spec-lens-a.md`
- `spec-control-room-ui-shell-lens-b` — agy — scenarios and coverage lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/spec-lens-b.md`
- `spec-control-room-ui-shell-lens-c` — agy — live-spec conflict and migration lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/spec-lens-c.md`
- `spec-control-room-ui-shell-synthesis` — cursor-agent — synthesis of three parallel spec lenses into the canonical delta spec tree — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-shell/specs/`, `openspec/changes/control-room-ui-shell/spec-synthesis-notes.md`

## Tasks

- `tasks-control-room-ui-shell-lens-a` — agy — decomposition and ordering lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/tasks-lens-a.md`
- `tasks-control-room-ui-shell-lens-b` — agy — partition and dispatch-shape lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/tasks-lens-b.md`
- `tasks-control-room-ui-shell-lens-c` — agy — proof and review-burden lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ui-shell/tasks-lens-c.md`
- `tasks-control-room-ui-shell-synthesis` — cursor-agent — synthesis of three parallel tasks lenses into one canonical tasks.md — `cursor-grok-4.6-high` — `openspec/changes/control-room-ui-shell/tasks.md`, `openspec/changes/control-room-ui-shell/tasks-synthesis-notes.md`

## Target and post-planning phases

Planning packets use the staging target above and exact output scopes. Apply uses `feature: control-room-ui-shell`, `parent_ref: refs/heads/control-room/control-room-ui-shell`, and refreshed SHA fields; verify is boolean `read_only: true`; archive is one agy lane. No phase is skipped.
