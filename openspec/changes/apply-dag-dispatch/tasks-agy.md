# Tasks: Apply-Phase DAG Dispatch

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1700–2400 lines (code: ~750–1050, tests: ~850–1200, docs/templates: ~100–150) |
| 2000-line budget risk | High / Elevated (borders and plausibly exceeds the 2000-line change budget) |
| Chained PRs recommended | Yes (or strictly sequenced 3-unit single-PR delivery) |
| Suggested split | 3 PRs: (1) Packet & Scope Enforcement, (2) DAG Engine & Splitter, (3) CLI Wiring, Docs & E2E |
| Delivery strategy | single-pr (with fallback to 3-part chain if review budget enforcement requires) |
| Chain strategy | pending |

### Review Budget Risk Assessment

This change is the largest and highest-uncertainty change among the three dual-executor-dispatch siblings. The implementation spans two new capabilities (`apply-dag-dispatch` and `allowed-paths-enforcement`) and shifts the `sdd-apply` lifecycle:
1. **`internal/packet`**: Frontmatter JSON-array parsing and component-prefix disjointness check (~200–300 lines).
2. **`internal/run`**: Base-SHA 3-way diff union computation (`git diff`, unstaged diff, untracked `ls-files`), exclusion of `.lucind/`, and `decideStatus` demotion logic (~400–600 lines with git integration tests).
3. **`internal/dag`**: New package for YAML parsing, graph validation, Kahn's algorithm wave grouping, and packet emission (~600–850 lines).
4. **`cmd/lucind-ai`**: CLI `split` subcommand, upfront disjointness check wiring, and `integrated_ids`/`reverted_ids` stdout printing (~250–400 lines).
5. **Docs & Templates & E2E**: `SKILL.md`, packet templates, and end-to-end integration tests (~250–400 lines).

Because total changed lines plausibly reach 1700–2400 lines, review-budget risk is **High**. If delivered as a single PR, test fixtures and helpers must remain focused to stay under the 2000-line ceiling. If the implementation expands beyond 2000 lines, it should be split along the 3 suggested work units.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Packet frontmatter `AllowedPaths`, `DisjointAllowedPaths` check, and `internal/run` scope check via base-SHA diff union | PR 1 | `go test ./internal/packet ./internal/run -race -count=1` | Temp git repo + fakeExecutor / real ledger | `internal/packet`, `internal/run` |
| 2 | `internal/dag` package: YAML parsing, Kahn's algorithm waves, cycle/disjointness validation, and packet file emission | PR 2 | `go test ./internal/dag -race -count=1` | Unit tests / temp directory fixtures | `internal/dag` |
| 3 | CLI `split` subcommand, `DisjointAllowedPaths` CLI wiring, `integrated_ids`/`reverted_ids` printing, docs, templates, and E2E verification | PR 3 | `go test ./cmd/lucind-ai ./... -race -count=1` | `lucind-ai split` + `lucind-ai run` E2E wave dispatch | `cmd/lucind-ai`, `plugin/claude-code/skills/lucind-ai/` |

---

## Phase 1: Packet Frontmatter & Upfront Disjointness Check (`internal/packet`)

- [ ] 1.1 RED: Write unit tests in `internal/packet/packet_test.go` covering `AllowedPaths []string` field parsing from a single-line JSON array in frontmatter (`allowed_paths: ["internal/ledger/"]`), verifying omitted `allowed_paths` stays nil/empty, empty JSON array `[]` is parsed as empty, and invalid JSON returns a parse error (`packet.ErrInvalidAllowedPaths`). Trace: `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`, `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`.
- [ ] 1.2 GREEN: Add `AllowedPaths []string` to `packet.Packet` struct and update `packet.Parse` in `internal/packet/packet.go` to parse single-line JSON array strings, keeping omitted keys empty and returning clear parse errors on invalid JSON. Trace: `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`.
- [ ] 1.3 RED: Write unit tests in `internal/packet/disjoint_test.go` for `DisjointAllowedPaths(packets []Packet) error`: component-boundary prefix overlap matches (`internal/ledger` matches `internal/ledger/foo.go`), non-component prefixes do not match (`internal/led` does not match `internal/ledger/foo.go`), sibling directories are accepted as disjoint (`internal/foo/` vs `internal/bar/`), and multi-packet batches with overlapping declared paths return an error. Trace: `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`, `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`.
- [ ] 1.4 GREEN: Implement `DisjointAllowedPaths` and path prefix matching helpers in `internal/packet/disjoint.go` using component-boundary prefix rules without glob expansion. Trace: `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`, `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`.

---

## Phase 2: Post-Execution Scope Enforcement (`internal/run`)

- [ ] 2.1 RED: Write tests in `internal/run/scope_test.go` and `internal/run/run_test.go` using real temporary git repositories to test base-SHA three-way diff union computation and `decideStatus` scope check:
  - Base-SHA diff union: captured from primary `HEAD`, combining committed diff (`git diff --name-only --diff-filter=ACDMRT <base> HEAD`), unstaged changes (`git diff --name-only --diff-filter=ACDMRT`), and untracked files (`git ls-files -o --exclude-standard`).
  - Zero commits on lane: untracked in-scope files evaluate correctly without relying on `HEAD~1` resolution.
  - Multiple commits on lane: early out-of-scope commits are caught in the base-SHA union even if the last commit touched only in-scope files.
  - In-scope-only changes remain `lane.Done`.
  - Out-of-scope tracked or untracked changes demote `Done` to `lane.Deviated` with an `EventLaneNote` naming the offending paths.
  - `.lucind/` directory (e.g. `.lucind/result.json`) is always excluded from scope comparison and never causes demotion.
  - Envelopes with `status: blocked` or `status: failed` are never rewritten to `lane.Deviated` even with out-of-scope modifications.
  - Git inspection command failure demotes `Done` to `lane.Blocked` with a diagnostic note.
  - Packets omitting `AllowedPaths` bypass the scope check completely and reach `lane.Done` unmodified (regression test).
  - Trace: `specs/allowed-paths-enforcement/spec.md#Base-SHA Three-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`, `specs/allowed-paths-enforcement/spec.md#Blocked and Failed Are Never Rewritten to Deviated`, `specs/allowed-paths-enforcement/spec.md#.lucind/ Is Always Excluded From Scope Comparison`, `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`, `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`, `specs/sdd-apply/spec.md#Additive Rollback, No Ledger Migration`.
- [ ] 2.2 GREEN: Implement diff-union calculation in `internal/run/scope.go` and wire the scope check into `decideStatus` in `internal/run/run.go`: capture primary `HEAD` base SHA, execute the three-way git union, exclude `.lucind/`, compare against `p.AllowedPaths` via component-boundary prefix matching, demote `Done` to `lane.Deviated` with diagnostic notes on violations, and return `lane.Blocked` on git inspection errors. Trace: `specs/allowed-paths-enforcement/spec.md#Base-SHA Three-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`, `specs/allowed-paths-enforcement/spec.md#Blocked and Failed Are Never Rewritten to Deviated`, `specs/allowed-paths-enforcement/spec.md#.lucind/ Is Always Excluded From Scope Comparison`, `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.

---

## Phase 3: DAG Engine & Splitter (`internal/dag`)

- [ ] 3.1 RED: Write unit tests in `internal/dag/parse_test.go` and `internal/dag/validate_test.go` for `apply-dag.yaml` parsing and validation:
  - Valid `apply-dag.yaml` sidecar with `change` and `packets` (`id`, `executor`, `routed_by`, `allowed_paths`, `depends_on`, `body_path`, optional `model`).
  - Reject duplicate packet `id` entries.
  - Reject omitted or empty `allowed_paths` (`[]`).
  - Reject missing `body_path` files on disk.
  - Reject cyclic dependencies in `depends_on` (cycle detection via Kahn's algorithm).
  - Reject same-wave path overlap when no `depends_on` edge exists.
  - Accept same-wave disjoint paths (e.g. `internal/foo/` and `internal/bar/`).
  - Accept cross-wave path overlap when an explicit `depends_on` edge orders the packets.
  - Trace: `specs/apply-dag-dispatch/spec.md#Sidecar DAG Artifact`, `specs/apply-dag-dispatch/spec.md#Unique Packet IDs`, `specs/apply-dag-dispatch/spec.md#Non-Empty Allowed Paths at Split`, `specs/apply-dag-dispatch/spec.md#Acyclic Depends-On, Grouped by Kahn's Algorithm`, `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`, `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires a Dependency Edge`.
- [ ] 3.2 GREEN: Implement `apply-dag.yaml` schema structs, YAML parser, and graph validation logic in `internal/dag/parse.go` and `internal/dag/validate.go`. Trace: `specs/apply-dag-dispatch/spec.md#Sidecar DAG Artifact`, `specs/apply-dag-dispatch/spec.md#Unique Packet IDs`, `specs/apply-dag-dispatch/spec.md#Non-Empty Allowed Paths at Split`, `specs/apply-dag-dispatch/spec.md#Acyclic Depends-On, Grouped by Kahn's Algorithm`, `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`, `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires a Dependency Edge`.
- [ ] 3.3 RED: Write unit tests in `internal/dag/split_test.go` for wave grouping and packet file generation:
  - Kahn's algorithm wave grouping: topological sort into sequential waves where each wave depends only on prior waves.
  - Packet file emission: write `<out>/<id>.md` containing generated YAML frontmatter with single-line JSON array `allowed_paths: ["..."]` concatenated with verbatim Markdown from `body_path` (Goal, Context, criteria unaltered).
  - Wave plan formatting: generate copy-pasteable `lucind-ai run --packet ...` stdout command strings in wave order, without writing unnecessary `waves.json` files.
  - Trace: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`, `specs/apply-dag-dispatch/spec.md#Acyclic Depends-On, Grouped by Kahn's Algorithm`, `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`.
- [ ] 3.4 GREEN: Implement wave grouping, packet emission, and command formatting in `internal/dag/split.go` and `internal/dag/wave.go`. Trace: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`, `specs/apply-dag-dispatch/spec.md#Acyclic Depends-On, Grouped by Kahn's Algorithm`.

---

## Phase 4: CLI Subcommand, Output Wiring & Exit Contract (`cmd/lucind-ai`)

- [ ] 4.1 RED: Write tests in `cmd/lucind-ai/cli_test.go` for the `split` subcommand: parsing `--dag <apply-dag.yaml>` and `--out <dir>` flags, validating arguments, calling `dag.Split`, printing wave commands to stdout on success, and returning non-zero with error diagnostics on invalid DAGs. Trace: `specs/apply-dag-dispatch/spec.md#Sidecar DAG Artifact`, `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`.
- [ ] 4.2 GREEN: Register `case "split":` subcommand in `cmd/lucind-ai/cli.go` and connect it to `internal/dag.Split`. Trace: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`.
- [ ] 4.3 RED: Write tests in `cmd/lucind-ai/cli_test.go` verifying that `runDispatch` calls `packet.DisjointAllowedPaths` before `ExecuteBatch` (and before any worktree creation), exiting with code 1 when overlapping `--packet` flags are passed. Trace: `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`.
- [ ] 4.4 GREEN: Wire `packet.DisjointAllowedPaths(ps)` check into `runDispatch` in `cmd/lucind-ai/cli.go` before `lucindrun.ExecuteBatch`. Trace: `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`.
- [ ] 4.5 RED: Write tests in `cmd/lucind-ai/cli_test.go` verifying that `printReport` prints `integrated_ids: <id...>` and `reverted_ids: <id...>` on stdout:
  - All lanes integrated: `integrated_ids` lists all lanes, `reverted_ids:` is printed empty.
  - Bisection reverts a lane: `reverted_ids` lists the bisected lane ID, `integrated_ids` lists the surviving lanes.
  - Process exits non-zero if any lane is reverted or not `done`.
  - Trace: `specs/apply-dag-dispatch/spec.md#Integrated and Reverted Lane IDs on Stdout`, `specs/sdd-apply/spec.md#Orchestrator Reads Stdout, Not a New Report Format`, `specs/sdd-apply/spec.md#Orchestrator Advances Only on a Passing Wave`.
- [ ] 4.6 GREEN: Update `printReport` in `cmd/lucind-ai/cli.go` to print `integrated_ids` and `reverted_ids` from `IntegrateReport`. Trace: `specs/apply-dag-dispatch/spec.md#Integrated and Reverted Lane IDs on Stdout`, `specs/sdd-apply/spec.md#Orchestrator Reads Stdout, Not a New Report Format`.

---

## Phase 5: Documentation & Template Updates

- [ ] 5.1 RED/CHECK: Verify existing documentation and templates lack guidance on `apply-dag.yaml`, `lucind-ai split`, sequential wave dispatch loop, `integrated_ids`/`reverted_ids` stdout inspection, and JSON-array `allowed_paths` frontmatter formatting in `plugin/claude-code/skills/lucind-ai/SKILL.md`, `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`, and `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md`. Trace: `specs/sdd-apply/spec.md#Apply Authors Packets, Not Primary Diffs`, `specs/sdd-apply/spec.md#An Absent Sidecar Preserves Hand-Split Apply`, `specs/sdd-apply/spec.md#Orchestrator Advances Only on a Passing Wave`.
- [ ] 5.2 GREEN: Update `plugin/claude-code/skills/lucind-ai/SKILL.md`, `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`, and `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md`:
  - Document the sidecar `openspec/changes/<change-id>/apply-dag.yaml` schema and `lucind-ai split --dag ... --out ...` command.
  - Document the orchestrator loop: dispatching printed wave commands sequentially, checking `integrated_ids`/`reverted_ids` and exit 0 before advancing.
  - Document the frontmatter `allowed_paths: ["..."]` JSON array format and scope enforcement rules.
  - Update `sdd-apply` flow to clarify authoring packets and driving waves rather than making in-process primary edits.
  - Trace: `specs/sdd-apply/spec.md#Apply Authors Packets, Not Primary Diffs`, `specs/sdd-apply/spec.md#An Absent Sidecar Preserves Hand-Split Apply`, `specs/sdd-apply/spec.md#Orchestrator Advances Only on a Passing Wave`, `specs/sdd-apply/spec.md#Orchestrator Reads Stdout, Not a New Report Format`, `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`.

---

## Phase 6: End-to-End Testing & Integration Verification

- [ ] 6.1 RED: Author an end-to-end integration test in `test/e2e/dag_dispatch_test.go` (or `cmd/lucind-ai/e2e_test.go`) creating a real test fixture `apply-dag.yaml` with 2 packets (`apply-p1` and `apply-p2` where `apply-p2` `depends_on: [apply-p1]`, disjoint `allowed_paths`, and valid `body_path` Markdown files). Trace: `specs/apply-dag-dispatch/spec.md#Sidecar DAG Artifact`, `specs/sdd-apply/spec.md#Apply Authors Packets, Not Primary Diffs`.
- [ ] 6.2 GREEN: Run `lucind-ai split` on the fixture, capture generated wave commands from stdout, execute wave 1 via `lucind-ai run`, verify wave 1 promotes to primary `HEAD`, then execute wave 2 via `lucind-ai run` verifying wave 2 branches from the newly-promoted `HEAD`, passes scope check, promotes cleanly, and emits expected `integrated_ids` on stdout. Trace: `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`, `specs/apply-dag-dispatch/spec.md#Per-Wave Integrate Reuses Combine, Resolve, and Bisect Unmodified`, `specs/apply-dag-dispatch/spec.md#One Run Per Wave on the Ledger`, `specs/sdd-apply/spec.md#Orchestrator Advances Only on a Passing Wave`, `specs/sdd-apply/spec.md#Combine, Resolve, and Bisect Stay Untouched`.
- [ ] 6.3 VERIFY: Run full test suite (`go test ./... -race -count=1`) ensuring all unit, integration, and regression tests pass without race conditions, and confirming packets without `allowed_paths` remain fully compatible and unmodified. Trace: `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`, `specs/sdd-apply/spec.md#Additive Rollback, No Ledger Migration`.

---

## Phase 7: Cleanup & Strict Invariance Verification

- [ ] 7.1 Format all Go source files (`gofmt -s -w .`) and verify clean diagnostics via `go vet ./...`.
- [ ] 7.2 Verify strict immutability invariants: confirm that `internal/run/integrate.go`, `internal/resolve/resolve.go`, and `internal/integrate/integrate.go` have 0 edits and zero ledger schema migrations were introduced. Trace: `specs/apply-dag-dispatch/spec.md#Per-Wave Integrate Reuses Combine, Resolve, and Bisect Unmodified`, `specs/sdd-apply/spec.md#Combine, Resolve, and Bisect Stay Untouched`, `specs/sdd-apply/spec.md#Additive Rollback, No Ledger Migration`.
