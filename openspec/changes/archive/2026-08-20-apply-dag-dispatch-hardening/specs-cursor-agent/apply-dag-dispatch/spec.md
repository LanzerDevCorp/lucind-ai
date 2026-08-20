# Apply DAG Dispatch Delta

Only **Cross-Wave Overlap Requires a Dependency Edge** is renamed and modified. The other ten requirements are unchanged.

## RENAMED Requirements

### Requirement: Cross-Wave Overlap Requires a Dependency Edge
RENAMED TO: Cross-Wave Overlap Requires Transitive Dependency Ordering

## MODIFIED Requirements

### Requirement: Cross-Wave Overlap Requires Transitive Dependency Ordering

Cross-wave path overlap MUST be allowed when a directed `depends_on` path (direct or transitive) orders the two packets in either direction. An overlap with no `depends_on` path between the two packets — neither a direct edge nor a transitive path in either direction — MUST be rejected, requiring the author to add a dependency or shrink scope. `lucind-ai split` is the terminal consumer: it MUST fail before writing any packet file. (design.md § Global Overlap Validation; explore.md:12-19 citing `internal/dag/waves.go:64-74`.)

#### Scenario: Overlap with an edge splits into two waves
- GIVEN packet B `depends_on: [A]` and both declare overlapping `allowed_paths`
- WHEN `lucind-ai split` succeeds
- THEN A and B MUST appear on different wave commands, A before B

#### Scenario: Overlap without an edge rejected
- GIVEN two packets with overlapping `allowed_paths` and no `depends_on` path at all, direct or transitive, in either direction
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail

#### Scenario: Unordered overlap across unrelated waves rejected
- GIVEN three packets A, B, and C where A and C declare overlapping `allowed_paths`, C `depends_on: [B]`, and B does not depend on A — so there is no `depends_on` path between A and C in either direction, and C lands in a later wave than A only because of B
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail
