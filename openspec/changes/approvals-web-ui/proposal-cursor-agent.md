# Proposal: Approvals Web UI

## Intent
Make “I always say yes” a visible rate. `lucind-ai serve` is the localhost-only surface: per-item approval with inline evidence, blocking wait with timeout, and a merged batch ready for review with the exact `opencode` command. Rule 4 (who, when, later defect) is the one that matters.

## Scope
### In Scope
- `lucind-ai serve`: `embed` HTML/CSS/JS, stdlib `net/http`, same SQLite, localhost only
- Blocking wait with timeout before a lane goes terminal
- Four hard rules: no approve-all; unselected per-item decisions; evidence as output or `file:line`; who/when/later-defect plus wrong-approval rate
- Additive schema v3 `approvals` table

### Out of Scope
- Go, migrations, `internal/` here; canonical `proposal.md`; design/spec/tasks; npm; 7th `lane.Status`; executor `human`

## Capabilities
### New Capabilities
- `approvals-web-ui`: localhost UI, per-item decisions, inline evidence, cycle control, wait+timeout
- `approval-ledger`: who/when/decision/later-defect records and wrong-approval rate

### Modified Capabilities
- None (`openspec/specs/` does not exist)

## Approach
Keep six `lane.Status` values. Add schema v3 `approvals` (`run_id`, `lane_id`, `approver`, `decision`, `at`, `defect_surfaced_later`).

**Lane-lifecycle hook:** `internal/run/run.go` `Execute`, between `decideStatus` (~L315/~L407) and `deps.Ledger.SetStatus` (~L338). Wait MUST finish before `internal/run/batch.go` `b.Observe` (~L144). Column is `approver`, never `human`.

**Open (design — do not pick):** linking a later packet defect to an approval row — new event type, foreign key, or later manual flag.

## Affected Areas
| Area | Impact | Description |
|------|--------|-------------|
| `cmd/lucind-ai` | New | `serve`; localhost bind |
| `internal/run/run.go` | Modified | Wait between `decideStatus` and `SetStatus` |
| `internal/run/batch.go` | Modified | `Observe` only after wait |
| `internal/ledger/schema.go` | Modified | Schema v3 `approvals` |
| embedded static UI | New | HTML/CSS/JS via `embed` |

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Wait after `Observe` | High if misplaced | Hook before L338 and `Observe` |
| `human` executor collision | Med | `approver` column only |
| Defect-link unset | Med | Design decides; rate UI waits |
| No in-repo `net/http`/`embed` | Med | Design owns bind/embed |
| 7th status enum | Low | Rejected: switches + CHECK |

## Rollback Plan
No runtime effect. Later: revert `serve` and the Execute wait; leave v3 `approvals` empty. Implementation PR reverts cleanly.

## Dependencies
- Ledger schema v2; existing `Execute` / `Observe`
- PRD §8.3; explore `sdd/approvals-web-ui/explore`

## Success Criteria
- [ ] `serve` binds localhost only and reads the live ledger
- [ ] No approve-all; items start unselected; decided one by one
- [ ] Evidence is output or `file:line`, never a claim
- [ ] Ledger stores who/when/decision; UI shows later-defect rate
- [ ] Wait between `decideStatus` and `SetStatus`, before `b.Observe`
- [ ] Timeout ends an infinite wait
