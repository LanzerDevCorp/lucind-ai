# SDD Flow — control-room-ledger

All five planning phases run with the verified three agy lenses plus cursor-agent synthesis. No phase is skipped. Materialize `feature: control-room-planning`, `parent_ref: refs/heads/control-room/planning`, live SHA fields, and each bullet's final path as its complete `allowed_paths` before dispatch because current admission requires target fields.

## Explore

- `explore-control-room-ledger-lens-a` — agy — problem and candidates lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/explore-lens-a.md`
- `explore-control-room-ledger-lens-b` — agy — capabilities and scenarios lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/explore-lens-b.md`
- `explore-control-room-ledger-lens-c` — agy — risks, trade-offs, and spikes lens of the three-lens explore fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/explore-lens-c.md`
- `explore-control-room-ledger-synthesis` — cursor-agent — synthesis of three parallel explore lenses into one canonical explore document — `cursor-grok-4.6-high` — `openspec/changes/control-room-ledger/explore.md`, `openspec/changes/control-room-ledger/explore-synthesis-notes.md`

## Propose

- `propose-control-room-ledger-lens-a` — agy — candidate and approach lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/propose-lens-a.md`
- `propose-control-room-ledger-lens-b` — agy — capability impact and delta specs lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/propose-lens-b.md`
- `propose-control-room-ledger-lens-c` — agy — risks, rollback, and test impact lens of the three-lens propose fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/propose-lens-c.md`
- `propose-control-room-ledger-synthesis` — cursor-agent — synthesis of three parallel propose lenses into one canonical proposal document — `cursor-grok-4.6-high` — `openspec/changes/control-room-ledger/proposal.md`, `openspec/changes/control-room-ledger/proposal-synthesis-notes.md`

## Design

- `design-control-room-ledger-lens-a` — agy — decisions lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/design-lens-a.md`
- `design-control-room-ledger-lens-b` — agy — surface-and-flow lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/design-lens-b.md`
- `design-control-room-ledger-lens-c` — agy — failure-test-rollback lens of the three-lens design fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/design-lens-c.md`
- `design-control-room-ledger-synthesis` — cursor-agent — synthesis of three parallel design lenses into one canonical design — `cursor-grok-4.6-high` — `openspec/changes/control-room-ledger/design.md`, `openspec/changes/control-room-ledger/design-synthesis-notes.md`

## Spec

- `spec-control-room-ledger-lens-a` — agy — capabilities and requirements lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/spec-lens-a.md`
- `spec-control-room-ledger-lens-b` — agy — scenarios and coverage lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/spec-lens-b.md`
- `spec-control-room-ledger-lens-c` — agy — live-spec conflict and migration lens of the three-lens spec fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/spec-lens-c.md`
- `spec-control-room-ledger-synthesis` — cursor-agent — synthesis of three parallel spec lenses into the canonical delta spec tree — `cursor-grok-4.6-high` — `openspec/changes/control-room-ledger/specs/`, `openspec/changes/control-room-ledger/spec-synthesis-notes.md`

## Tasks

- `tasks-control-room-ledger-lens-a` — agy — decomposition and ordering lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/tasks-lens-a.md`
- `tasks-control-room-ledger-lens-b` — agy — partition and dispatch-shape lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/tasks-lens-b.md`
- `tasks-control-room-ledger-lens-c` — agy — proof and review-burden lens of the three-lens tasks fan-out — `gemini-3.7-flash-high` — `openspec/changes/control-room-ledger/tasks-lens-c.md`
- `tasks-control-room-ledger-synthesis` — cursor-agent — synthesis of three parallel tasks lenses into one canonical tasks.md — `cursor-grok-4.6-high` — `openspec/changes/control-room-ledger/tasks.md`, `openspec/changes/control-room-ledger/tasks-synthesis-notes.md`

## Target and post-planning phases

Every planning packet uses the staging target above and only its listed output path. Apply uses `feature: control-room-ledger`, `parent_ref: refs/heads/control-room/control-room-ledger`, and refreshed SHA fields; verify is boolean `read_only: true`; archive is one agy lane. No phase is skipped.
