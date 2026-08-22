# Spec Lens C — Live-Spec Conflicts & Migration: Control Room Capture

## Assumed requirements

The control-room-capture change introduces continuous file-backed stream spooling for lane execution under `<primaryRoot>/.lucind/`, bounded SQLite ledger diagnostics capped at 4096 bytes per stream, and loopback HTTP stream tail and download endpoints on `lucind-ai serve`. These capabilities extend `lane-execution` and `approvals-web-ui` while establishing the new `control-room-capture` domain. We assume four core requirements: (1) `Continuous primary-root stream spooling` asserting all three executors tee stdout/stderr to durable primary-root logs across all terminal statuses; (2) `Non-interfering WaitDelay drain` asserting child stdio drainage delays preserve exit codes and do not fail valid `lane.Done` runs; (3) `Bounded SQLite diagnostics` asserting ledger event notes cap stream tails at 4096 bytes without unclipped blobs; and (4) `Loopback HTTP stream access` asserting read-only loopback SSE tail and transcript download without child backpressure.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `lane-execution` | `openspec/specs/lane-execution/spec.md:1` | 3 | 6 | Yes |
| `approvals-web-ui` | `openspec/specs/approvals-web-ui/spec.md:1` | 4 | 9 | Yes |

## Conflicts

None. This change is strictly additive. All four proposed requirements (`Continuous primary-root stream spooling`, `Non-interfering WaitDelay drain`, `Bounded SQLite diagnostics`, and `Loopback HTTP stream access`) extend existing system behavior without contradicting, modifying, or invalidating any live requirement in `openspec/specs/lane-execution/spec.md` or `openspec/specs/approvals-web-ui/spec.md`.

Specifically:
- In `lane-execution`: The existing requirements (`Gate Placement in the Lifecycle`, `Resolve Before Barrier Observation`, and `Additive Schema, Unchanged Enum`) govern the placement of approval waits in the lane lifecycle, batch barrier observation ordering, and schema isolation. Primary-root stream spooling and WaitDelay handling execute around subprocess spawn and drainage, leaving gate placement, barrier synchronization, and the six-value `lane.Status` enum intact.
- In `approvals-web-ui`: The existing requirements (`Loopback Binding`, `Individual Decisions Without Bulk Approval`, `Inline Evidence and Batch Review Command`, and `Approver Wrong-Approval Rate`) govern server network binding and approval decision mechanics. Stream tailing and transcript download endpoints run under the existing loopback constraint and do not alter the anti-bulk approval rules or evidence validation.

Because no existing requirement guarantees are made untrue or superseded, there are no live conflicts. A conflict is a MODIFIED requirement, not an ADDED one.

## MODIFIED Full Blocks

None. No existing live requirements in `openspec/specs/lane-execution/spec.md` or `openspec/specs/approvals-web-ui/spec.md` are modified or replaced by this change. The proposed delta requirements are all ADDED requirements that extend capability behavior without altering existing requirement contracts.

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|

None. No requirements are removed or renamed by this change. All existing live requirements in `openspec/specs/lane-execution/spec.md` and `openspec/specs/approvals-web-ui/spec.md` remain active and unchanged, so no migration or consumer updates are required.

## Open Questions

- [ ] Directory layout: Should log files standardize on `.lucind/runs/<run_id>/lanes/<lane_id>.log` or `.lucind/logs/<run_id>/<lane_id>.log`, and should stdout/stderr be recorded in an interleaved file or split files (`.stdout.log` / `.stderr.log`)?
- [ ] Route ownership: Should log SSE and download endpoints be registered under `internal/serve/handlers.go` in this change or deferred to `control-room-serve`?
- [ ] Retention & cleanup: How should log files under `.lucind/` be pruned or retained during `lucind-ai archive` or run expiration?
- [ ] Parallel spec lane synthesis: `~/.claude/skills/sdd-spec/SKILL.md` assumes a single-agent workflow producing delta specs directly under `openspec/changes/<change-name>/specs/`, whereas this change executes a 3-lens parallel partition where Lens C produces conflict analysis and verbatim blocks as feedstock for the synthesizer lane.
