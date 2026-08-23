# control-room-ui-shell

## Scope

Replace the approvals-only embedded shell with the no-build live store shell, shared CSS, and Fleet/DAG/Flows modules. Preserve `go:embed static/*` and the existing polling fallback while adding SSE as the preferred transport.

## Non-scope

Do not add Features, Timeline, or Approvals view modules; do not add CLI flags; do not add server routes. Those are the next feature.

## Exact allowed paths

- `internal/serve/static/index.html` (existing)
- `internal/serve/static/app.css` (new)
- `internal/serve/static/store.js` (new)
- `internal/serve/static/views/fleet.js`, `internal/serve/fleet_static_test.go` (new)
- `internal/serve/static/views/dag.js`, `internal/serve/dag_static_test.go` (new)
- `internal/serve/static/views/flows.js`, `internal/serve/flows_static_test.go` (new)

## Acceptance criteria

- The default Fleet view renders the six-view shell's shared live store and shows executor/model/phase/fanout/feature/worktree/activity fields when present.
- DAG view renders waves, edges, statuses, and overlap errors from API data.
- Flows view renders the five planning phases plus apply/verify/archive and fanout grouping.
- Existing no-build embedding and accessibility/security invariants remain tested.

## Definition of done

Shell wave then three-view wave exit 0, static tests pass, `lucind-ai check` passes, and no npm/build artifact is introduced.
