# Design: Approvals Web UI

## Technical Approach

`lucind-ai serve`: stdlib `net/http` + `embed` HTML/CSS/JS. `run` and `serve` share `<primaryRoot>/.lucind/lucind.db` (WAL on). Wait in `Execute` after `decideStatus` (`run.go:315`) and before `SetStatus` (`:338`) so `runOneLane` `Observe` (`batch.go:144`) never sees an approval-pending lane as terminal. Accuracy is an `approvals` query, not a seventh `lane.Status`.

## Architecture Decisions

### Decision: Additive `approvals` table (schema v3), not a seventh `lane.Status`

**Choice**: New table. Enum unchanged. Lane stays `running` while waiting. Column `approver` ≠ executor `human`.

**Alternatives considered**: Seventh status (every switch + CHECK + barrier); wait as `blocked` (barrier releases).

**Rationale**: v1→v2 is additive-in-one-tx; a new terminal value misfires `Observe`.

### Decision: HTTP in `internal/serve`; CLI stays a switch

**Choice**: `internal/serve` owns `embed`, mux, loopback listen. `run()` adds `case "serve":` → `serveDispatch` (`cli.go:83-87`).

**Alternatives considered**: Handlers in `main`; server inside `ExecuteBatch`.

**Rationale**: Testable without `main`; WAL allows two `ledger.Open` processes.

### Decision: Wait on `persistCtx` + `Deps.ApprovalTimeout`; zero skips

**Choice**: If `done` after diagnosis note, insert pending and wait on `WithTimeout(persistCtx, ApprovalTimeout)` (`run.go:289-301`). Zero skips (existing tests). Timeout/reject → `blocked`. Never auto-approve.

**Alternatives considered**: Wait on `laneCtx` (dead after a 20m dispatch).

**Rationale**: Worktree stays warm; barrier idle until terminal `SetStatus`.

### Decision: Rule 4 is a column on the approval row

**Choice**: `defect_surfaced_later INTEGER NOT NULL DEFAULT 0`, set after the fact via `MarkDefectSurfaced`. Rate = flagged / approved, per `approver`. No new event type, no defects table, no auto-flag on later `failed`.

**Alternatives considered**: Event type as source of truth; FK to a defects table; infer from later `packet_id` status.

**Rationale**: The rate must be an explicit mark or it will be ignored.

### Decision: Flag defaults

**Choice**: `--addr` `127.0.0.1:7433` (reject non-loopback); `--approver` `user.Current().Username` (no secrets); `--approval-timeout` `30m` (≠ `--timeout` 20m/lane). Same primary-root + refuse linked worktree as `run`.

**Alternatives considered**: Bind `0.0.0.0`; invent a token; reuse `--timeout` for the wait.

**Rationale**: Localhost-only because this UI can approve work. Do not name the flag `human`.

## Data Flow

```
laneCtx → Execute.Run → decideStatus
  !=done ───────────────────────────┤
  done → RequestApproval → WaitDecision(persistCtx)
         serve POST one | timeout → SetStatus(done|blocked)
runOneLane Observe → barrier
serve GET / → pending + accuracy + opencode argv
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/ledger/schema.go` | Modify | v3 `approvals`; `currentVersion<3` |
| `internal/ledger/ledger.go` | Modify | CRUD, poll wait, rate, defect flag |
| `internal/run/run.go` | Modify | `ApprovalTimeout`; wait between `:315` and `:338` |
| `internal/serve/*.go` | Create | Loopback server, `embed` UI, handlers |
| `internal/serve/static/*` | Create | No approve-all; nothing preselected; evidence inline |
| `cmd/lucind-ai/cli.go` | Modify | `case "serve"`; `--approval-timeout` |

## Interfaces / Contracts

```go
type Approval struct {
    RunID, LaneID, PacketID, Approver, Evidence string
    Decision Decision // pending|approved|rejected|timed_out
    DefectSurfacedLater bool
    RequestedAt time.Time
    DecidedAt *time.Time
}
func (l *Ledger) WaitDecision(ctx context.Context, runID, laneID string) (Approval, error)
func ListenAndServe(ctx context.Context, addr string, h http.Handler) error // loopback
// GET /  POST /approvals/{runID}/{laneID}  POST .../defect
```

`RequestApproval`, `Decide` (one row), `MarkDefectSurfaced`, `ApproverRate`.

```go
if deps.ApprovalTimeout > 0 && status == lane.Done {
    waitCtx, cancel := context.WithTimeout(persistCtx, deps.ApprovalTimeout)
    defer cancel()
    dec, err := deps.Ledger.WaitDecision(waitCtx, deps.RunID, p.ID) // timeout/reject → blocked
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Ledger | v3 migrate; one-row Decide; rate | Real SQLite `t.TempDir()` + `Open` |
| Execute wait | Blocks until Decide; timeout → `blocked` | Real ledger; goroutine `Decide`; tests keep timeout 0 |
| HTTP | One-item POST; bulk 400; non-loopback listen fails | `httptest` + real ledger |
| CLI / UI | `serve` flags; no “approve all” in `embed.FS` | stdlib `testing` (no testify); fake HTTP trigger, real ledger+wait |

## Threat Matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths (executable Markdown, README.sh, etc.) | N/A | Ledger + embed only |
| Git repository selection (`git -C`, relative/absolute paths) | N/A | No new git selector |
| Commit state (staged, `commit -a`, empty index) | N/A | No commit automation |
| Push state (tracking branch, first push, explicit refspec) | N/A | No push |
| PR commands (explicit `--head`, env prefix, composed commands) | N/A | No PR argv |
| Unauthenticated loopback HTTP that can release a waiting lane | Applicable | Bind `127.0.0.1`; reject non-loopback `--addr`; POST one `(run_id,lane_id)`. RED: non-loopback listen fails; bulk body 400 |

## Migration / Rollout

Schema **v3**, same tx pattern as v2 (`schema.go:70-114`). Table `approvals`: PK `(run_id, lane_id)`; `packet_id`; `approver DEFAULT ''`; `decision CHECK IN pending,approved,rejected,timed_out`; `evidence`; `defect_surfaced_later 0/1`; `requested_at`; `decided_at`. Fresh insert 1,2,3; v2 takes only v3. Rollback: drop table, delete migration 3. Off when `ApprovalTimeout==0`.

## Open Questions

- [ ] Merged-batch `opencode` argv vs integrate combined-tree path.
- [ ] Poll 250ms vs `sqlite3_update_hook` (v1 = poll).
