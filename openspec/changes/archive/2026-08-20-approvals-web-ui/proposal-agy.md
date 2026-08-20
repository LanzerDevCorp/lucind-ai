# Proposal: Approvals Web UI (`lucind-ai serve`)

## Intent
Provide an embedded localhost web interface (`lucind-ai serve`) for reviewing and approving lane executions, displaying inline evidence, enforcing individual unselected decisions without an "approve all" button, and tracking approver defect rates in SQLite.

## Scope
### In Scope
- Subcommand `lucind-ai serve` on `127.0.0.1` via stdlib `net/http` and `embed`.
- Blocking approval wait with timeout in lane lifecycle.
- Additive SQLite schema v3 (`approvals` table).
- Web UI showing pending approvals, inline evidence, defect metrics, and batch review commands (`opencode`).

### Out of Scope
- Frontend build tools, npm, or non-stdlib dependencies.
- Remote network binding, authentication, or credentials.
- Modifying 6-value `lane.Status` enum or `executor='human'`.

## Capabilities
### New Capabilities
- `approvals-web-ui`: Localhost interface for individual approval decisions, inline evidence inspection, defect metrics, and batch review status.
- `lane-approval-wait`: Blocking gate pausing lane status persistence pending approval or timeout.

### Modified Capabilities
- `lane-execution`: Hooks approval check in `internal/run/run.go` `Execute` between `decideStatus` (~line 407) and `deps.Ledger.SetStatus` (~line 338), before `internal/run/batch.go` `b.Observe`.

## Approach
Implement `lucind-ai serve` using stdlib `net/http` and `embed`. Intercept terminal transitions in `internal/run/run.go` between status calculation and persistence; wait for an approval or timeout before calling `b.Observe`. Additive schema v3 (`approvals` table) records decisions and defect history without changing `lane.Status`.

*Open Question for Design Phase:* Linkage model for defect surfacing back to approval records (ledger event type vs foreign key flag).

## Affected Areas
| Area | Impact | Description |
|------|--------|-------------|
| `cmd/lucind-ai` | New | Registers `serve` command. |
| `internal/server` | New | Localhost HTTP handlers and embedded UI assets. |
| `internal/ledger` | Modified | Schema v3 migration and queries for `approvals`. |
| `internal/run` | Modified | Approval gate between `decideStatus` and `SetStatus`. |

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Lane hangs on unhandled approval | Medium | Mandatory timeout with fallback status. |
| Collision with `executor='human'` | Low | Name fields `approver_name` / `approver_id`. |
| Enum switch churn | High | Keep 6-value `lane.Status`; use additive table. |

## Rollback Plan
Revert `internal/run` lifecycle hook to bypass approval gate. Schema v3 is additive and backwards-compatible.

## Dependencies
- Go standard library (`net/http`, `embed`, `database/sql`, `html/template`).
- Existing `internal/ledger` SQLite database.

## Success Criteria
- [ ] `lucind-ai serve` runs on localhost without frontend build or runtime dependencies.
- [ ] Lane executions pause at lifecycle gate until approved or timed out.
- [ ] UI shows inline evidence, requires individual decisions, and displays approver defect rate.
- [ ] Schema v3 records approvals additively.
