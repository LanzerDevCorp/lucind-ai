# Spec Synthesis Notes: Control Room Telemetry

## Unresolved Contradictions

None

## Coverage Gaps

- Feature Attempt Audit Preservation (lens A; `parent-feature-integration`) has RFC 2119 SHALL but no lens-B scenario. Not invented. Archive would land a scenario-less requirement unless a later lane adds one.
- Spec drafts left three product questions unspecified, so they are not requirements: (1) archive worktree logs before `RemoveLaneWorktree` / `worktree cleanup`; (2) SSE payload framing (raw chunks vs multiplexed JSON); (3) coarse milestones (in-memory only vs schema v6). Canonical `design.md` chose archive beside `PersistEnvelope` as `.lucind/results/<lane-id>.log` and schema v5; SSE framing remains open there too.
- New capabilities are change-folder deltas (`## ADDED Requirements`), not live-shaped `# Specification` + `## Purpose` + `## Requirements`. Archive is expected to rewrite that shape.
- Skill `sdd-spec` 650-word artifact cap was not applied; this packet's 1800-word authored budget governs.

## Dropped Citations

- Lens B coverage table claimed ledger isolation is testable through `internal/ledger/ledger_test.go:366-381`. That range is `TestConcurrentRegisterAndSetStatusAcrossDistinctLanes` (concurrent `RegisterLane`/`SetStatus`), not CHECK rejection of stream event types. Isolation scenarios were kept: they cite `internal/ledger/schema.go:38-39` and `internal/run/run.go:71-100,422-435`, which match. The coverage-table claim is not in the delta.

## Requirement Divergence

Lens A's set is authoritative: five ADDED requirements, no MODIFIED/REMOVED/RENAMED. Lens C found no live-spec conflicts (`openspec/specs/approvals-web-ui/spec.md:1-83`, `lane-execution/spec.md:1-62`, `parent-feature-integration/spec.md:1-65`); classification stays ADDED. Lens C still copied five unchanged live blocks under `## MODIFIED Full Blocks`; they match `openspec/specs/approvals-web-ui/spec.md:10-48` and `lane-execution/spec.md:10-62` verbatim and are not shipped as MODIFIED.

Lens B and C independently used the proposal's four names (`Worktree-local log teeing`, `Loopback SSE stream`, `Ledger isolation`, `Shell-free queries and status invariants`) and omitted A's fifth. Scenarios were joined by that mapping, not exact title match:

| Lens A | Lens B scenarios used |
|---|---|
| Worktree-Local Log Teeing and Process Invariants | Worktree-local log teeing (3) |
| Loopback Server-Sent Events Telemetry Stream | Loopback SSE stream (3) |
| High-Frequency SQLite Ledger Isolation | Ledger isolation (3) |
| Shell-Free Run Lifecycle Query | Shell-free queries and status invariants (3) |
| Feature Attempt Audit Preservation | none |

No lens-B scenario was keyed to a name outside that map, so none were dropped for namelessness. Lens C's extra open question (3-lens vs single `sdd-spec` sub-agent) is process, not a product requirement.

Independent convergence: B and C matched A's first four on substance (tee + WaitDelay, loopback SSE without weakening decide, chunks off `events` with 4096-byte `lane_note`, shell-free model + six-value status + barrier after persist). A's fifth is the only split.
