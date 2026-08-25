# Spec Synthesis Notes: Native Stability Campaign

## Unresolved Contradictions

None

## Coverage Gaps

None

## Dropped Citations

- `internal/feature/feature.go:98-113`: In `spec-lens-a.md`, cited in the requirement body for "Out-of-scope defect detection and recording" as the terminal consumer for "MUST persist a Defect Record and halt promotion." Verification against the codebase reveals lines 98-113 define `ValidateParentRef` (which validates parent branch ref strings against reserved namespaces like `main` and `lucind/*`). It contains no logic for Defect Records, promotion halts, or out-of-scope defect detection. `spec-lens-a.md`'s own Citation Manifest accurately described this range as `ValidateParentRef`, confirming the requirement body text used a misplaced citation. Because Defect Record persistence is new behavior introduced by this Change under `internal/stability`, no existing code in `internal/feature/feature.go` serves as its terminal consumer. The invalid citation was dropped from the delta specification and the terminal consumer retargeted to the new `internal/stability` domain.

## Requirement Divergence

- **Lens A vs Lens B structure**: Lens A authored 17 requirements decomposed across 8 capabilities (7 new, 1 existing `lane-execution`), whereas Lens B organized scenarios under 10 product-level requirement areas corresponding to Master Plan items R1–R10. Lens A's 17-requirement decomposition is authoritative. All behavioral scenarios from Lens B were mapped and joined onto Lens A's requirement names across the 8 capability delta specs.
- **Classification convergence**: Lens A classified all 17 requirements as `ADDED`, including the Linux process-group isolation requirement against existing capability `lane-execution`. Lens C independently verified `openspec/specs/lane-execution/spec.md` (checking all four live requirements: Gate Placement in the Lifecycle, Resolve Before Barrier Observation, Additive Schema / Unchanged Enum, and Lane metadata dispatch persistence) and confirmed that no existing requirement is contradicted or modified. Lens C independently corroborated Lens A's classification of `lane-execution` process-group isolation as an `ADDED` requirement within that capability's delta specification.
- **Domain convergence**: All three lenses converged independently on core architectural invariants: 3 sequential Trials with reset-on-failure, 5 dispatches per Trial pinned to `gemini-3.7-flash-high`, 10s lease TTL with monotonic fencing, Test Actor approval for out-of-scope remediation, common-directory SQLite/WAL authority (`<git-common-dir>/lucind-ai/stability/v1/stability.db`), bounded 4096-byte log sanitization, and non-mutating release boundaries with canonical JSON Stability Receipts (RFC 8785).
