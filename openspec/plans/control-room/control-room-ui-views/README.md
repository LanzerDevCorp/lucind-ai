# control-room-ui-views

## Scope

Add Features, Timeline, and Approvals views, then wire the complete application and CLI security flag. This feature owns existing `app.js` and `cmd/lucind-ai/cli.go` so no concurrent lane edits either shared file.

## Non-scope

Do not alter ledger schema/APIs, executor capture, server routes, or the shell/view modules owned by `control-room-ui-shell`. Do not enable dispatch by default.

## Exact allowed paths

- `internal/serve/static/views/features.js`, `internal/serve/features_static_test.go` (new)
- `internal/serve/static/views/timeline.js`, `internal/serve/timeline_static_test.go` (new)
- `internal/serve/static/views/approvals.js`, `internal/serve/approvals_static_test.go` (new)
- `internal/serve/static/app.js` (existing)
- `cmd/lucind-ai/cli.go` (existing)
- `cmd/lucind-ai/cli_control_room_test.go` (new)

## Acceptance criteria

- Features view shows parent/base, lease holder/fence/TTL, attempt state, overlap, and reconciliation evidence.
- Timeline merges events, integration events, and progress with filters and bounded rendering.
- Approvals preserves individual-only decisions and the current evidence block.
- CLI exposes the explicit dispatch gate while the default remains read-only/disabled; embedded app wiring loads all six views.

## Definition of done

The three-view wave and wiring wave exit 0, focused UI/CLI tests and `lucind-ai check` pass, and final consolidation is performed explicitly into `dev`.
