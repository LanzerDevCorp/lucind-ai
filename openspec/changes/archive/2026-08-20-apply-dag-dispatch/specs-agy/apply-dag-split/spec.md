# Apply DAG Split Specification

## Purpose

Define the sidecar DAG declaration format and the `lucind-ai split` validation and packet generation tool to decompose tasks into dependency-ordered waves with non-overlapping path scopes.

## Requirements

### Requirement: DAG Sidecar Schema Validation

The `lucind-ai split` command MUST parse `apply-dag.yaml` containing a top-level `change` name and a list of `packets`. Each packet MUST declare a unique `id`, a non-empty `allowed_paths` list, a list of `depends_on` packet IDs, an `executor`, a `routed_by` rationale, an optional `model`, and a valid `body_path` referencing an existing Markdown file. (Trace: Decision 1 — DAG artifact and exact format)

#### Scenario: Valid DAG schema parsing
- GIVEN a valid `apply-dag.yaml` with required fields and existing `body_path` files
- WHEN `lucind-ai split` is executed
- THEN it MUST parse all packet definitions successfully

#### Scenario: Missing or empty allowed paths rejected
- GIVEN an `apply-dag.yaml` containing a packet with an empty or omitted `allowed_paths` list
- WHEN `lucind-ai split` validates the file
- THEN it MUST reject the configuration with a validation error

#### Scenario: Duplicate packet ID rejected
- GIVEN an `apply-dag.yaml` containing two packets with the same `id`
- WHEN `lucind-ai split` validates the file
- THEN it MUST reject the configuration with a duplicate ID error

#### Scenario: Missing body path rejected
- GIVEN an `apply-dag.yaml` where a packet specifies a `body_path` that does not exist on disk
- WHEN `lucind-ai split` validates the file
- THEN it MUST fail with a missing file error

### Requirement: Acyclic Dependency Graph Validation

The `lucind-ai split` command MUST validate that packet dependencies form a directed acyclic graph using topological sorting (Kahn's algorithm) and MUST reject cycles with a diagnostic error. (Trace: Decision 1 — DAG artifact and exact format)

#### Scenario: Acyclic dependencies partitioned into waves
- GIVEN a valid set of packets with acyclic `depends_on` relationships
- WHEN `lucind-ai split` processes the graph
- THEN it MUST assign packets to dependency waves where each wave depends only on prior waves

#### Scenario: Dependency cycle rejected
- GIVEN a set of packets with circular `depends_on` references (e.g. A -> B -> A)
- WHEN `lucind-ai split` evaluates the graph
- THEN it MUST reject the graph and report the detected cycle

### Requirement: Path Scope Disjointness and Dependency Enforcement

The `lucind-ai split` command MUST enforce that packets assigned to the same wave have pairwise disjoint `allowed_paths` using component-boundary prefix matching without glob expansion. Packets with overlapping `allowed_paths` MUST belong to different waves with an explicit `depends_on` edge between them; overlapping scopes without a dependency edge MUST be rejected. (Trace: Decision 1 — DAG artifact and exact format)

#### Scenario: Same-wave disjoint paths accepted
- GIVEN packets in the same wave declaring `internal/ledger/` and `internal/serve/`
- WHEN `lucind-ai split` validates path disjointness
- THEN it MUST accept the wave partition

#### Scenario: Same-wave overlapping paths rejected
- GIVEN packets in the same wave declaring `internal/ledger/` and `internal/ledger/store.go`
- WHEN `lucind-ai split` validates path disjointness
- THEN it MUST reject the configuration due to same-wave path overlap

#### Scenario: Component-boundary prefix matching
- GIVEN packets declaring `internal/ledger` and `internal/ledgertest`
- WHEN `lucind-ai split` evaluates prefix overlap
- THEN it MUST treat them as disjoint because the boundary is not component-aligned

#### Scenario: Cross-wave overlap with dependency edge accepted
- GIVEN packet B depends on packet A, and both declare `internal/ledger/`
- WHEN `lucind-ai split` validates the graph
- THEN it MUST accept the overlap because an explicit dependency edge separates their waves

#### Scenario: Cross-wave overlap without dependency edge rejected
- GIVEN two packets declaring overlapping paths without a `depends_on` edge between them
- WHEN `lucind-ai split` validates the graph
- THEN it MUST reject the configuration requiring a dependency edge or scope reduction

### Requirement: Packet Generation and Frontmatter Formatting

The `lucind-ai split` command MUST write individual packet Markdown files to the output directory specified by `--out`. Each generated packet file MUST contain frontmatter with `id`, `executor`, `routed_by`, optional `model`, and `allowed_paths` formatted as a single-line JSON array, followed by the verbatim content of `body_path`. (Trace: Decision 1 — DAG artifact and exact format, Decision 2 — Packet.AllowedPaths and its terminal consumers)

#### Scenario: Generate packet files with JSON array allowed_paths
- GIVEN a valid `apply-dag.yaml` and output directory `packets/`
- WHEN `lucind-ai split --dag apply-dag.yaml --out packets/` is executed
- THEN it MUST write `packets/<id>.md` for each packet with frontmatter containing `allowed_paths: ["..."]`

#### Scenario: Concatenate body content unchanged
- GIVEN a packet body file containing task markdown prose and instructions
- WHEN the packet file is generated
- THEN the body content MUST follow the frontmatter delimiters verbatim without alteration

### Requirement: Sequential Wave Plan Emission

The `lucind-ai split` command MUST output ready-to-execute `lucind-ai run` CLI invocations to stdout, ordered sequentially wave-by-wave, with all packets in a wave passed as repeatable `--packet` flags. (Trace: Decision 1 — DAG artifact and exact format, Decision 3 — sequential lucind-ai run per wave)

#### Scenario: Stdout emission of wave commands
- GIVEN a DAG resolved into wave 1 (`apply-ledger`) and wave 2 (`apply-serve`, `apply-run`)
- WHEN `lucind-ai split` completes
- THEN stdout MUST contain `lucind-ai run --packet packets/apply-ledger.md` followed by `lucind-ai run --packet packets/apply-serve.md --packet packets/apply-run.md`

#### Scenario: Independent packets in single wave
- GIVEN multiple independent packets with no dependencies and mutually disjoint paths
- WHEN `lucind-ai split` runs
- THEN it MUST emit a single `lucind-ai run` command combining all packets in wave 1
