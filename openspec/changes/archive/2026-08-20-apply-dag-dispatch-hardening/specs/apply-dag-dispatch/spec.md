# Apply DAG Dispatch Delta

Only **Cross-Wave Overlap Requires a Dependency Edge** is renamed and modified. The other ten requirements in `openspec/specs/apply-dag-dispatch/spec.md` are unchanged.

## RENAMED Requirements

### Requirement: Cross-Wave Overlap Requires a Dependency Edge
RENAMED TO: Cross-Wave Overlap Requires Transitive Dependency Ordering

## MODIFIED Requirements

### Requirement: Cross-Wave Overlap Requires Transitive Dependency Ordering

Cross-wave path overlap MUST be allowed when a directed `depends_on` path -- direct or transitive -- orders the two packets in either direction. Any pair of packets whose `allowed_paths` overlap under component-boundary prefix match, with no directed `depends_on` path between them in either direction, MUST be rejected by `lucind-ai split` before any packet files are emitted, requiring the author to add a dependency or shrink scope. (Design: Global Overlap Validation; explore.md Gap 1, Item 3; `internal/dag/waves.go:64-74`; `internal/dag/split.go:24-30`.)

#### Scenario: Overlap with an edge splits into two waves
- GIVEN packet B `depends_on: [A]` and both declare overlapping `allowed_paths`
- WHEN `lucind-ai split` succeeds
- THEN A and B MUST appear on different wave commands, A before B

#### Scenario: Transitive dependency ordering authorizes overlap
- GIVEN packet C depends on B, packet B depends on A, and packets A and C declare overlapping `allowed_paths` with no direct edge between A and C
- WHEN `lucind-ai split` validates and splits the DAG
- THEN it MUST accept the DAG and place A and C in different waves in dependency order

#### Scenario: Overlap without any path rejected
- GIVEN two packets with overlapping `allowed_paths` and no `depends_on` path at all -- neither a direct edge nor a transitive path -- in either direction
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail

#### Scenario: Unordered overlap across unrelated waves rejected
- GIVEN three packets A, B, and C where A and C declare overlapping `allowed_paths`, C `depends_on: [B]`, and B does not depend on A -- so there is no `depends_on` path between A and C in either direction, and C lands in a later wave than A only because of its dependency on the unrelated packet B
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST reject the DAG before emitting any packet files
