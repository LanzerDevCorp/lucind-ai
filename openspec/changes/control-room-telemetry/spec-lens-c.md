# Spec Lens C — Live-Spec Conflicts & Migration: Control Room Telemetry

## Assumed requirements

This change introduces two new capabilities (`lane-telemetry-streaming`, `shell-free-telemetry-query`) and touches three modified live capabilities (`approvals-web-ui`, `lane-execution`, `parent-feature-integration`). We assume four core delta requirements asserted by `proposal.md`: (1) `Worktree-local log teeing` (`proposal.md:59-74`), asserting executor dispatch concurrently streams child stdout/stderr to a worktree-local log and in-memory hub via `io.MultiWriter`, while honoring `cmd.WaitDelay` and flagging `Outcome.OutputTruncated` on unclosed pipes; (2) `Loopback SSE stream` (`proposal.md:75-84`), asserting `internal/serve` exposes an SSE route (`/api/telemetry/events`) over loopback using stdlib `net/http` and `http.Flusher` without bypassing individual approval controls; (3) `Ledger isolation` (`proposal.md:85-100`), asserting high-frequency stream chunks stay off SQLite `events` and terminal failure `lane_note` rows remain capped at 4096 bytes per stream; and (4) `Shell-free queries and status invariants` (`proposal.md:101-116`), asserting `serve.Model` queries lifecycle events without `os/exec`, preserving the six-value `lane.Status` enum and ensuring the batch barrier releases only after terminal status persistence.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `approvals-web-ui` | `openspec/specs/approvals-web-ui/spec.md:1-83` | 4 | 9 | Extended with loopback SSE endpoint (`/api/telemetry/events`) on existing mux; existing `Loopback Binding` (`:10-25`), `Individual Decisions Without Bulk Approval` (`:26-48`), `Inline Evidence and Batch Review Command` (`:49-66`), and `Approver Wrong-Approval Rate` (`:67-83`) remain unchanged. |
| `lane-execution` | `openspec/specs/lane-execution/spec.md:1-62` | 3 | 6 | Extended with concurrent worktree-local log and in-memory hub teeing during dispatch; existing `Gate Placement in the Lifecycle` (`:10-26`), `Resolve Before Barrier Observation` (`:27-43`), and `Additive Schema, Unchanged Enum` (`:44-62`) invariants (six-value `lane.Status` enum and barrier release only after terminal status persistence) remain unchanged. |
| `parent-feature-integration` | `openspec/specs/parent-feature-integration/spec.md:1-65` | 4 | 9 | Preserved/isolated; integration attempt phases and check results continue to persist through `WriteWithAudit` into `integration_events` without high-frequency stream chunks entering SQLite; existing `Explicit Feature Target` (`:5-18`), `Managed Parent Lifecycle` (`:19-32`), `Immutable Starts and Serialized Promotion` (`:33-46`), and `Recoverable Idempotent Attempts` (`:47-65`) remain unchanged. |

## Conflicts

None. This change extends the live specifications additively and contradicts no live requirement or scenario:
- In `approvals-web-ui` (`openspec/specs/approvals-web-ui/spec.md:1-83`): The new SSE streaming endpoint (`/api/telemetry/events`) strictly adheres to loopback-only binding via `serve.IsLoopback` (`Loopback Binding`, `:10-25`), does not permit or introduce bulk approval mechanisms (`Individual Decisions Without Bulk Approval`, `:26-48`), preserves inline evidence requirements (`Inline Evidence and Batch Review Command`, `:49-66`), and does not alter approver defect rate tracking (`Approver Wrong-Approval Rate`, `:67-83`).
- In `lane-execution` (`openspec/specs/lane-execution/spec.md:1-62`): Hooking stdout/stderr teeing into `run.Execute` preserves approval wait sequencing before terminal ledger persistence (`Gate Placement in the Lifecycle`, `:10-26`), enforces bounded flush (<500ms) without allowing premature barrier release before terminal status persistence (`Resolve Before Barrier Observation`, `:27-43`), and preserves the six-value `lane.Status` enum without adding stream states (`Additive Schema, Unchanged Enum`, `:44-62`).
- In `parent-feature-integration` (`openspec/specs/parent-feature-integration/spec.md:1-65`): Feature integration attempts and check results continue to use `WriteWithAudit` and `integration_events`, preventing high-frequency log chunk pollution while maintaining explicit feature targeting (`:5-18`), managed parent lifecycles (`:19-32`), serialized promotion leases (`:33-46`), and idempotent recovery (`:47-65`).

Because all proposed changes introduce new requirements or additively extend capabilities without invalidating existing behavior, there are no live conflicts (a conflict is a MODIFIED requirement, not an ADDED one).

## MODIFIED Full Blocks

### Requirement: Loopback Binding

**Source**: `openspec/specs/approvals-web-ui/spec.md:10` — 2 scenarios

The server MUST bind only to `127.0.0.1` and MUST reject a non-loopback `--addr`.

#### Scenario: Loopback listen

- GIVEN loopback address `127.0.0.1:7433`
- WHEN starting `lucind-ai serve`
- THEN the server MUST listen on loopback.

#### Scenario: Non-loopback rejected

- GIVEN non-loopback address `0.0.0.0:7433`
- WHEN starting `lucind-ai serve --addr 0.0.0.0:7433`
- THEN the server MUST reject binding and exit with an error.

### Requirement: Individual Decisions Without Bulk Approval

**Source**: `openspec/specs/approvals-web-ui/spec.md:26` — 3 scenarios

Items MUST start unselected. Every decision MUST be made individually; the UI MUST NOT provide a
bulk/"approve all" control, and the server MUST reject a multi-item request.

#### Scenario: Fresh load starts unselected

- GIVEN pending items
- WHEN the page loads
- THEN every item MUST be unselected.

#### Scenario: Unselected item cannot be decided

- GIVEN an unselected item
- WHEN a decision is submitted for it
- THEN the system MUST NOT record `approved` or `rejected`.

#### Scenario: Bulk request rejected

- GIVEN multiple pending items
- WHEN a multi-item approval request is posted
- THEN the server MUST reject it; the UI MUST NOT expose a control that could produce one.

### Requirement: Gate Placement in the Lifecycle

**Source**: `openspec/specs/lane-execution/spec.md:10` — 2 scenarios

Approval wait MUST run after status computation and MUST resolve before that status is persisted
to the ledger.

#### Scenario: Approve then persist done

- GIVEN a lane's computed status is `done` and the user approves
- WHEN the wait resolves
- THEN the terminal status MUST be persisted as `done` only after that decision.

#### Scenario: Timeout persists blocked, never done

- GIVEN a lane's computed status is `done` and the wait times out
- WHEN the lane persists
- THEN the terminal status MUST be `blocked`, never `done`.

### Requirement: Resolve Before Barrier Observation

**Source**: `openspec/specs/lane-execution/spec.md:27` — 2 scenarios

Approval wait MUST resolve, and the lane's status MUST be persisted, before the batch barrier
observes that lane.

#### Scenario: Barrier waits for terminal persist

- GIVEN a lane still waiting on an approval decision
- WHEN the batch would otherwise observe that lane
- THEN the barrier MUST NOT treat it as observed until persistence completes.

#### Scenario: Barrier stays idle while one lane waits

- GIVEN one lane waiting on approval and every other lane already terminal
- WHEN the batch barrier is checked
- THEN it MUST NOT release until the waiting lane's status is persisted.

### Requirement: Additive Schema, Unchanged Enum

**Source**: `openspec/specs/lane-execution/spec.md:44` — 2 scenarios

Approval records MUST be stored in a separate, additive table. The six-value `lane.Status` enum
MUST NOT gain a seventh value for this feature.

#### Scenario: Persist approval record

- GIVEN a lane awaiting approval
- WHEN a decision is recorded
- THEN the ledger MUST write it to the `approvals` table without changing `lane.Status`'s valid
  values.

#### Scenario: Mark a defect surfaced later

- GIVEN an approved lane whose packet later surfaces a defect
- WHEN an operator flags it
- THEN the ledger MUST update that approval's `defect_surfaced_later` column without altering the
  lane's historic status.

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| None | None | No requirements are removed or renamed by this change. | N/A | None |

### Reference Inventory of Live Consumers

For synthesis and migration verification, existing consumers of the touched capabilities across the codebase include:
- `internal/serve/server.go:12-22,55-73` (`IsLoopback`, `ErrNonLoopback`, `ListenAndServe` enforcing loopback listen)
- `internal/serve/handlers.go:36-85,148-211` (`NewHandler`, static file serving, `/api/state`, `/approvals/{runID}/{laneID}`, single decision verification, anti-bulk check)
- `internal/serve/server_test.go:17-40,42-93,95-134,136-194` (`TestNonLoopbackListenFails`, `TestBulkRequestBodyReturns400`, `TestUnselectedDecisionReturns400`, `TestSingleApprovalAndDefectEndpoints`)
- `internal/run/run.go:348-351,368-375,422-450` (`Execute`, `SetStatus`, `streamDetailCap`, approval wait gate placement and status persistence)
- `internal/barrier/barrier.go:36-47` (`Evaluate`, barrier release waiting on terminal persisted status)
- `internal/lane/status.go:8-28` (Six-value `lane.Status` enum invariants)
- `internal/executor/agy.go:160-197`, `internal/executor/cursor_agent.go:82-118`, `internal/executor/opencode.go:121-159` (`WaitDelay` drainage, grandchild pipe handling, `Outcome.OutputTruncated`)
- `internal/ledger/schema.go:34-43,171-180` (SQLite schema v5, `events.type` CHECK constraint, `integration_events` table)
- `internal/ledger/ledger.go:366-381,448-485,488-526` (`AppendEvent`, `SetStatus`, `Events` query)
- `internal/run/attempt.go:213-214,408-443`, `internal/integrate/integrate.go:90-109` (`WriteWithAudit`, attempt phase transitions)
- `cmd/lucind-ai/cli.go:674-725` (`serveDispatch` CLI entry point with `--addr`, `--approver`, `--approval-timeout`)

## Open Questions

- [ ] Log archival location: Should worktree logs be archived before worktree removal (`cmd/lucind-ai/cli.go:641-646,1460-1474`) beside envelopes as `.lucind/results/<lane-id>.log` or organized under `.lucind/logs/<run-id>/` (`proposal.md:154-155`)?
- [ ] SSE payload format: Should the SSE stream emit raw stdout/stderr chunk frames or a multiplexed JSON envelope containing lane ID, stream name, timestamp, and chunk bytes (`proposal.md:155`)?
- [ ] Milestone event persistence: If coarse milestone telemetry (e.g. turn index, elapsed duration) is tracked in the future, should it remain purely in-memory in the SSE hub or introduce a schema v6 migration for additive tables in `internal/ledger/schema.go` (`proposal.md:156`)?
- [ ] Parallel spec lane synthesis: `~/.claude/skills/sdd-spec/SKILL.md` assumes a single sub-agent authoring delta specs under `openspec/changes/<change-name>/specs/`, whereas this change executes a 3-lens parallel partition where Lens C produces conflict analysis and verbatim blocks as feedstock for the synthesizer lane.
