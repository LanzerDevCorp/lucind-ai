# Proposal: Feature Parent Integration

## Intent

Safely advance features on parent branches instead of the implicitly promoted checkout.

## Scope

### In Scope
- Non-rewritten parent refs with packet/run parent and expected SHA.
- Integration, evidence, proposed localhost approval, and bounded candidates.
- Required reconciliation blocks promotion, not admission/dispatch.

### Out of Scope
- Closure/integration to `main`, delivery, remote approval, and Sonnet semantic authority.

## User Workflow

1. Lucind creates a parent ref, records its SHA, and starts lanes there.
2. Lanes combine and promote only their parent under its lease.
3. Evidence is informational, warning, or required; only required pauses promotions.
4. The **proposed** UI displays evidence and records one source → target direction; expiry needs fresh evidence.
5. Direction permits one Sonnet candidate and automatic CAS only after checks, markers, and SHA validation. Failures block; closure is external.

## Capabilities

### New Capabilities
- `parent-feature-integration`: Parent lifecycle, recovery, and CAS.
- `reconciliation-approval`: Evidence, expiring approval, candidates, and audit state.

### Modified Capabilities
- None; `openspec/specs/` is empty.

## Approach and Authority

Use explicit parent refs/SHAs and a durable per-feature lease for checks, candidates, and CAS. Replays return a result or resume inputs; stale leases recover deterministically.

Display base/tips, rename-aware paths/sizes, hunk proximity, hotspots, special-file labels, checks, and candidate diff. Predicted conflicts, nearby overlap, or hotspots can require reconciliation; disjoint shared hunks may warn. Structural evidence is best-effort; no opaque score decides.

Choose a reconciliation-specific durable record, linked to—not generalized from—the unimplemented lane-only approval design: its subject is cross-feature direction, not a lane. Humans choose direction; Sonnet cannot choose it or invent semantics. Failed checks, stale refs, over-bound conflicts, timeout, or unresolved markers block and preserve evidence.

## Compatibility and Migration

Use an additive ledger migration. Specs must choose: declared single-parent legacy default or clear failure. Silent checked-out-branch promotion is prohibited; single-`main` needs opt-in/migration.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `cmd/lucind-ai/cli.go`, `internal/run/*` | Modified | Parent lifecycle/recovery. |
| `internal/packet/*`, `internal/worktree/*`, `internal/integrate/*` | Modified | Parent/SHA, starts, CAS. |
| `internal/ledger/*`, proposed `internal/serve/*`, `internal/resolve/*` | Modified/New | State, UI, resolver. |

## Risks and Rollback

| Risk | Mitigation |
|---|---|
| Stale refs/lease loss | Expected-SHA CAS; preserved evidence. |
| False overlap | Visible evidence; warnings do not block. |
| Unsafe candidate | Direction-bound resolver and mandatory checks. |

Disable parent integration; retain evidence and restore only through explicit operator Git action—never history rewriting.

## Success Criteria

- [ ] Parents advance independently without changing `main` or the primary checkout.
- [ ] Required reconciliation needs fresh direction, checks, matching SHAs, and CAS.
- [ ] Retry, expiry, stale refs, failed checks, and unresolved markers leave recoverable evidence.
