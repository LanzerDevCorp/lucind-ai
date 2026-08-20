# Apply DAG Dispatch Specification

## Purpose

Declare an apply-phase DAG in a sidecar YAML file, validate and split it into packet files whose same-wave `allowed_paths` do not overlap, and dispatch each wave as one `lucind-ai run` so wave N+1 only ever sees wave N's promoted code.

## Requirements

### Requirement: Sidecar DAG Artifact

An apply DAG MUST be declared at `openspec/changes/<change-id>/apply-dag.yaml`, with a top-level `change` name and a `packets` list. `tasks.md` MUST remain an unstructured human checklist and MUST NOT be the parse source. (Design Decision 1.)

#### Scenario: Sidecar is the parse source
- GIVEN an SDD change with both `tasks.md` and `apply-dag.yaml`
- WHEN `lucind-ai split` reads the DAG
- THEN it MUST parse `apply-dag.yaml` and MUST NOT parse wave structure, `depends_on`, or `allowed_paths` out of `tasks.md`

#### Scenario: Sidecar schema
- GIVEN a valid `apply-dag.yaml` whose `packets` entries carry `id`, `executor`, `routed_by`, `allowed_paths`, `depends_on`, `body_path`, and optionally `model`
- WHEN `lucind-ai split` runs
- THEN it MUST accept the file and emit one packet per entry

#### Scenario: Missing body path rejected
- GIVEN an `apply-dag.yaml` where a packet's `body_path` does not exist on disk
- WHEN `lucind-ai split` validates the file
- THEN it MUST fail with a missing-file error

### Requirement: Split Is the Mechanical Consumer

`lucind-ai split --dag <apply-dag.yaml> --out <dir>` MUST write one packet file per DAG node by concatenating generated frontmatter — including a single-line JSON-array `allowed_paths:` key — with the Markdown at `body_path`, verbatim and unaltered. It MUST print copy-pasteable wave commands to stdout, in dependency order, as the wave plan; it MUST NOT write a `waves.json` or any other file whose only purpose is to restate that plan. Split MUST NOT invent Goal, Context, or done-criteria text. (Design Decision 1, Decision 2.)

#### Scenario: Two-wave stdout plan
- GIVEN a DAG where `apply-serve` and `apply-run` both `depends_on: [apply-ledger]` and are same-wave-disjoint
- WHEN `lucind-ai split` succeeds
- THEN stdout MUST contain a first line `lucind-ai run --packet .../apply-ledger.md` and a later line passing both remaining packets on one `lucind-ai run`

#### Scenario: Body text is copied, not invented
- GIVEN a `body_path` whose Markdown already contains the packet Goal and done criteria
- WHEN `lucind-ai split` writes the packet file
- THEN the packet body MUST be that Markdown unchanged, preceded only by generated frontmatter

### Requirement: Unique Packet IDs

`lucind-ai split` MUST reject a DAG in which two packets share the same `id`. (Design Decision 1.)

#### Scenario: Duplicate id rejected
- GIVEN two packets both named `apply-ledger`
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail and MUST NOT write packet files for that DAG

### Requirement: Non-Empty Allowed Paths at Split

`lucind-ai split` MUST reject a packet whose `allowed_paths` is omitted or empty — an empty list belongs to the sibling `read-only-packet-dispatch` capability, not this one. Split MUST always emit a non-empty `allowed_paths` list on every packet it writes. (Design Decision 1, Decision 2.)

#### Scenario: Empty allowed_paths rejected at split time
- GIVEN a packet declared with `allowed_paths: []`
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail

### Requirement: Acyclic Depends-On, Grouped by Kahn's Algorithm

`lucind-ai split` MUST reject a DAG whose `depends_on` graph contains a cycle, and MUST group acyclic packets into waves using Kahn's algorithm, where each wave depends only on prior waves. (Design Decision 1.)

#### Scenario: Cycle rejected
- GIVEN packet A depends on B and packet B depends on A
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail and report the detected cycle

#### Scenario: Valid DAG grouped into waves
- GIVEN packet B depends on A, packet C depends on neither, and A's paths are disjoint from C's
- WHEN `lucind-ai split` succeeds
- THEN A and C MUST appear on the first wave command, and B MUST appear on a later wave command than A

### Requirement: Same-Wave Paths Pairwise Disjoint

Packets placed in the same wave MUST have pairwise-disjoint `allowed_paths`, decided by component-boundary prefix match, never glob expansion: `internal/ledger` matches `internal/ledger/foo.go`; `internal/led` does not match `internal/ledger/foo.go`. (Design Decision 1; glob support is explicitly out of scope.)

#### Scenario: Same-wave overlap rejected
- GIVEN two packets with no `depends_on` edge, one declaring `internal/foo/` and the other `internal/foo/bar.go`
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST fail

#### Scenario: Sibling directories accepted in the same wave
- GIVEN two packets with no `depends_on` edge, one declaring `internal/foo/` and the other `internal/bar/`
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST place both packets on the same wave command

#### Scenario: Non-component prefix is not overlap
- GIVEN one packet declaring `internal/led` and another declaring `internal/ledger/foo.go`, with no `depends_on` edge
- WHEN `lucind-ai split` validates the DAG
- THEN it MUST treat the paths as disjoint

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

### Requirement: Sequential Run Per Wave

The orchestrator MUST execute the wave commands `lucind-ai split` printed, one at a time, stopping on any non-zero exit. `ExecuteBatch` MUST remain a flat concurrent batch with no DAG or wave concept added to it. Wave N+1's worktrees MUST be created from primary `HEAD` only after wave N has integrated and promoted. (Design Decision 3.)

#### Scenario: Next wave sees promoted code
- GIVEN wave N's `lucind-ai run` exits 0 after Integrate promotes its done lanes
- WHEN wave N+1's `lucind-ai run` creates worktrees
- THEN those worktrees MUST branch from the already-promoted primary `HEAD`

#### Scenario: Failed wave halts the remaining DAG
- GIVEN wave N's `lucind-ai run` exits non-zero because a lane is `blocked`, `deviated`, `failed`, or reverted during bisection
- WHEN the orchestrator would dispatch wave N+1
- THEN it MUST NOT dispatch any remaining wave

#### Scenario: ExecuteBatch stays flat
- GIVEN a wave of two independent packets
- WHEN `lucind-ai run --packet p1 --packet p2` executes
- THEN `ExecuteBatch` MUST run them as today's concurrent batch — one goroutine per packet, one barrier, then Integrate — with no wave scheduler added inside it

### Requirement: Per-Wave Integrate Reuses Combine, Resolve, and Bisect Unmodified

Each wave's integration MUST call the existing Combine → `resolve.Resolve` (400-line `claude -p` cap) → Check → bisect → Promote path exactly as it exists. This capability MUST NOT reimplement or redesign those steps. (Design Decision 3; proposal: What stays untouched.)

#### Scenario: Wave integration is the existing path
- GIVEN a wave whose lanes reach the barrier
- WHEN Integrate runs
- THEN it MUST use the existing combine, resolver, bisection, and promote implementation, not a DAG-specific copy

### Requirement: One Run Per Wave on the Ledger

Each `lucind-ai run` that dispatches a wave MUST be its own ledger `run_id`, individually inspectable. The binary MUST NOT introduce a nested or whole-DAG run identifier. (Design Decision 3.)

#### Scenario: Two waves, two run IDs
- GIVEN a two-wave DAG whose first wave exits 0
- WHEN the second wave is dispatched
- THEN the ledger MUST record a distinct `run_id` for each wave

### Requirement: Integrated and Reverted Lane IDs on Stdout

`printReport` MUST print the list of integrated lane IDs (`integrated_ids`) and reverted lane IDs (`reverted_ids`) on stdout, alongside the existing `integrate:` counts and `reason`. It MUST NOT require a `--json` flag or a `.lucind/runs/<run_id>.json` file to learn those IDs — the six existing ledger event types remain sufficient and no new event type is introduced. (Design Decision 4.)

#### Scenario: Bisected-out lane ID is printed
- GIVEN a wave where one lane printed `status: done` at dispatch time and `revertLanes` later bisected it out
- WHEN `printReport` writes stdout
- THEN `reverted_ids` MUST list that lane's ID and `integrated_ids` MUST NOT

#### Scenario: Empty reverted_ids when nothing reverted
- GIVEN a wave execution where every lane is successfully integrated
- WHEN `printReport` writes stdout
- THEN stdout MUST include the integrated lane IDs and an explicitly empty `reverted_ids:` list
