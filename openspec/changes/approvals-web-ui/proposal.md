# Proposal: Approvals Web UI (`lucind-ai serve`)

## Intent

Make "I always say yes" a visible rate, not an invisible habit. `lucind-ai serve` is the
localhost-only control surface for the whole cycle: per-item approval decisions with inline
evidence, a blocking wait (with timeout) that keeps a batch alive while a lane needs a human, and
a merged-batch view showing the exact `opencode` RDD command to run. Rule 4 — the ledger records
who approved, when, and whether a defect later surfaced in that same packet, and the UI shows the
user's own wrong-approval rate — is the one that matters; the other three rules are friction
people route around without it.

## Scope

### In Scope
- `lucind-ai serve`: `embed` HTML/CSS/JS, stdlib `net/http`, same SQLite ledger, bound to
  `127.0.0.1` only.
- A blocking approval wait (configurable timeout) before a lane's status is persisted as terminal.
- Four hard rules: no "approve all"; every item starts unselected, decided individually; evidence
  is command output or `file:line`, never a bare claim; approval outcomes and later defects are
  tracked and surfaced as a per-user accuracy rate.
- Additive SQLite schema v3 (`approvals` table).

### Out of Scope
- Frontend build tooling, npm, or any non-stdlib dependency.
- Remote binding, authentication, or credentials — localhost-only for v1.
- Extending the 6-value `lane.Status` enum, or reusing/colliding with the reserved `human`
  executor value.
- design/spec/tasks content — this proposal states intent and scope only.

## Capabilities

### New Capabilities
- `approvals-web-ui`: localhost UI — per-item decisions starting unselected, inline evidence,
  merged-batch review with the `opencode` command, approval-accuracy rate.
- `lane-approval-wait`: a blocking gate, with timeout, sitting between a lane's status being
  computed and being persisted as terminal.

### Modified Capabilities
- `lane-execution`: hooks the approval gate into `internal/run/run.go` `Execute`, between
  `decideStatus` and `deps.Ledger.SetStatus`, and must resolve before `internal/run/batch.go`'s
  `b.Observe` for that lane — or the shared batch barrier misfires.

## Approach

Add a new `internal/server` package (stdlib `net/http` + `embed`) exposing the approval UI and API,
reading the existing ledger. Keep the 6-value `lane.Status` enum untouched; add an additive schema
v3 `approvals` table (approver, decision, timestamp, later-defect flag) instead — lower blast
radius than touching every exhaustive switch on `lane.Status`. The approver identity must not reuse
the schema's existing `human` executor value, which is an unrelated mechanism.

**Deferred to design, not decided here:** the exact `approvals` schema/columns, how a later defect
gets linked back to its approval record (event type vs. foreign key vs. manual flag), and where
`serve`'s bind-address/timeout flags default.

## Affected Areas

| Area | Impact | Description |
|------|--------|--------------|
| `cmd/lucind-ai` | New | Registers `serve` subcommand. |
| `internal/server` | New | Localhost HTTP handlers + embedded UI assets. |
| `internal/run/run.go` | Modified | Approval-wait gate between `decideStatus` and `SetStatus`. |
| `internal/run/batch.go` | Modified | `Observe` only fires after the wait resolves. |
| `internal/ledger/schema.go` | Modified | Additive schema v3, `approvals` table. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Approval-wait placed after `b.Observe` | High if misplaced | Hook strictly before `SetStatus`/`Observe`; design names the exact line. |
| Lane hangs on an unanswered approval | Medium | Mandatory configurable timeout with a defined fallback status. |
| Approver naming collides with `human` executor | Low | Reserve distinct column/vocabulary, never reuse `human`. |
| Extending `lane.Status` instead of an additive table | Low (rejected) | Additive `approvals` table keeps every existing exhaustive switch untouched. |

## Rollback Plan

No runtime effect until `serve` is invoked. Reverting is two independent, clean steps: drop the
`Execute`/`Observe` wait hook (lanes go terminal exactly as today), and leave the additive `approvals`
table unused — schema v3 never breaks v2 readers. Both revert in one PR with no data migration back.

## Dependencies

- Go stdlib only: `net/http`, `embed`, `database/sql`.
- Existing `internal/ledger` (schema v2) and `internal/run` lane lifecycle.
- PRD §8.3; this session's exploration (`sdd/approvals-web-ui/explore`).

## Success Criteria

- [ ] `lucind-ai serve` binds `127.0.0.1` only and reads the live ledger.
- [ ] No "approve all" anywhere; every item starts unselected, decided individually.
- [ ] Evidence shown is command output or `file:line`, never a bare claim.
- [ ] Ledger records who approved, when, and later-defect status; UI shows the approver's own
      wrong-approval rate.
- [ ] Approval-wait resolves strictly before `b.Observe`, and a timeout ends an unanswered wait.
- [ ] Schema v3 is additive; `lane.Status` stays at 6 values.
