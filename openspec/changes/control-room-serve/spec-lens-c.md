# Spec Lens C — Live-Spec Conflicts & Migration: Control Room Serve

## Assumed requirements

This change modifies `approvals-web-ui` and introduces four new capabilities (`control-room-api-runs`, `control-room-api-features`, `control-room-api-reconcile`, `control-room-events-stream`). For `approvals-web-ui`, it modifies the loopback and boundary requirement to enforce linked-worktree refusal before ledger initialization while preserving loopback binding and anti-bulk single-decision semantics. For new capabilities, it adds granular read-only JSON REST endpoints under `/api/v1/` for runs, lanes, features, and reconciliations, alongside an SSE event stream (`GET /api/v1/events/stream`) tailing ledger events, and requires unmatched `/api/*` paths to return JSON 404s rather than HTML fallbacks.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `approvals-web-ui` | `openspec/specs/approvals-web-ui/spec.md:1-83` | 4 | 9 | `Loopback Binding` (modified to add linked-worktree refusal); `Individual Decisions Without Bulk Approval` (preserved/touched for path-bound validation) |

## Conflicts

None. The change extends `approvals-web-ui` and introduces new `/api/v1/` routes without contradicting or invalidating any existing guarantees of the live specification. `Loopback Binding` is extended with a pre-flight linked-worktree refusal check (`cmd/lucind-ai/cli.go:702-707`), while single-decision validation (`POST /approvals/{runID}/{laneID}`), inline evidence rendering, approver defect rate calculations, and existing `/api/state` polling remain intact and fully compatible.

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

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| None | N/A | No requirements are removed or renamed. | N/A | None |

## Open Questions

- [ ] SSE cadence across `run` and `serve`: whether the event push loop on `id > lastID` should use a fixed polling ticker or an adaptive backoff.
- [ ] HTTP mutation scope beyond single-lane decisions (`POST /approvals/{runID}/{laneID}`): whether reconciliation approvals (e.g., wrapping `reconcile.Service.Approve` at `cmd/lucind-ai/cli.go:1166-1176`) should eventually be exposed via HTTP by default, gated behind `--enable-dispatch`, or remain CLI-only on loopback.
- [ ] Execution divergence from `sdd-spec` skill: this sub-agent executes a parallel lens writing only `spec-lens-c.md` (skipping `specs/` directory creation, Engram persistence, and summary block) as instructed by packet precedence over the standalone `sdd-spec` skill.
