# Apply DAG Dispatch Specification

## RENAMED Requirements

### Requirement: Cross-Wave Overlap Requires a Dependency Edge
RENAMED TO: Cross-Wave Overlap Requires Transitive Dependency Ordering

## MODIFIED Requirements

### Requirement: Cross-Wave Overlap Requires Transitive Dependency Ordering

Cross-wave path overlap MUST be allowed when a directed `depends_on` path (direct or transitive) in either direction orders the two packets. Any pair of packets whose `allowed_paths` overlap under component-boundary prefix match without a directed `depends_on` path between them MUST be rejected by `lucind-ai split` before any packet files are emitted, requiring the author to add a dependency or shrink scope. (Design: Global Overlap Validation; design.md Architecture Decisions; explore.md Gap 1, Item 3; internal/dag/waves.go:64-74; internal/dag/split.go:24-30.)

#### Scenario: Overlap with an edge splits into two waves
- GIVEN packet B `depends_on: [A]` and both declare overlapping `allowed_paths`
- WHEN `lucind-ai split` succeeds
- THEN A and B MUST appear on different wave commands, A before B

#### Scenario: Overlap without an edge rejected
- GIVEN two packets with overlapping `allowed_paths` and no directed `depends_on` path (direct or transitive) in either direction
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail

#### Scenario: Transitive dependency ordering authorizes overlap
- GIVEN packet C depends on B, packet B depends on A, and packets A and C declare overlapping `allowed_paths`
- WHEN `lucind-ai split` validates and splits the DAG
- THEN it MUST accept the DAG and place A and C in different waves in dependency order

#### Scenario: Unordered cross-wave overlap rejected
- GIVEN three packets A, B, and C where A and C declare overlapping `allowed_paths`, packet C `depends_on: [B]`, and packet B does not depend on A (such that C lands in a later wave than A for an unrelated reason without a dependency path between A and C)
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST reject the DAG before emitting any packet files
