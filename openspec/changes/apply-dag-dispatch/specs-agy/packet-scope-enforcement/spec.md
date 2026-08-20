# Packet Scope Enforcement Specification

## Purpose

Enforce packet path scope boundaries upfront before worktree creation and post-execution against actual git diffs in `decideStatus`.

## Requirements

### Requirement: Packet AllowedPaths Parsing

The packet parser MUST parse single-line JSON array `allowed_paths` frontmatter into `Packet.AllowedPaths` (`[]string`). If the `allowed_paths` key is omitted or empty, `Packet.AllowedPaths` MUST be empty. If `allowed_paths` is present but contains invalid JSON, parsing MUST fail with an error. (Trace: Decision 2 — Packet.AllowedPaths and its terminal consumers, Decision 5 — rollback)

#### Scenario: Parse valid JSON array allowed_paths
- GIVEN a packet file with frontmatter `allowed_paths: ["internal/ledger/", "cmd/lucind-ai/cli.go"]`
- WHEN `packet.Parse` parses the packet
- THEN `Packet.AllowedPaths` MUST contain `["internal/ledger/", "cmd/lucind-ai/cli.go"]`

#### Scenario: Omitted allowed_paths yields empty slice
- GIVEN a packet file without an `allowed_paths` frontmatter key
- WHEN `packet.Parse` parses the packet
- THEN `Packet.AllowedPaths` MUST be empty or nil without error

#### Scenario: Malformed JSON allowed_paths rejected
- GIVEN a packet file with frontmatter `allowed_paths: [unquoted-path]`
- WHEN `packet.Parse` parses the packet
- THEN parsing MUST fail with a syntax error

### Requirement: Upfront Batch Path Disjointness Check

Before creating any lane worktrees, the CLI dispatch layer MUST check all packets in the batch for pairwise disjoint `AllowedPaths` using component-boundary prefix matching. If any two packets declare overlapping paths, dispatch MUST halt with an error before `ExecuteBatch` is invoked. (Trace: Decision 2 — Packet.AllowedPaths and its terminal consumers)

#### Scenario: Disjoint batch paths proceed to execution
- GIVEN a batch of packets declaring `["internal/ledger/"]` and `["internal/serve/"]`
- WHEN the upfront disjointness check runs
- THEN it MUST pass and proceed to worktree creation and execution

#### Scenario: Overlapping batch paths rejected before worktree creation
- GIVEN a batch of packets where two packets declare overlapping path scopes (e.g. `internal/ledger/` and `internal/ledger/store.go`)
- WHEN the upfront disjointness check runs
- THEN it MUST reject the batch with an error before any worktree is created

#### Scenario: Empty allowed_paths bypassed in upfront check
- GIVEN a batch of packets where packets omit `allowed_paths`
- WHEN the upfront disjointness check runs
- THEN it MUST bypass disjointness validation for those packets

### Requirement: Post-Execution Base-SHA Git Diff Scope Verification

In `decideStatus`, when a lane envelope has valid status `done` and `AllowedPaths` is non-empty, the system MUST compute the actual modified paths as the union of: (1) committed changes against primary repository base HEAD (`git diff --name-only --diff-filter=ACDMRT <base> HEAD`), (2) unstaged changes (`git diff --name-only --diff-filter=ACDMRT`), and (3) untracked files respecting `.gitignore` (`git ls-files -o --exclude-standard`), excluding any `.lucind/` paths. (Trace: Decision 2 — Packet.AllowedPaths and its terminal consumers)

#### Scenario: Multi-commit in-scope changes verified
- GIVEN a lane with two or more commits touching only files within `AllowedPaths`
- WHEN `decideStatus` computes the diff union against the base SHA
- THEN all committed changes across all commits MUST be evaluated and verified as in-scope

#### Scenario: Unstaged and untracked in-scope files included
- GIVEN a lane with unstaged modifications or untracked new files within `AllowedPaths`
- WHEN `decideStatus` computes the diff union
- THEN those paths MUST be included in the union and verified as in-scope

#### Scenario: Gitignored files and dot-lucind paths excluded
- GIVEN a lane worktree containing `.lucind/result.json` and gitignored build artifacts
- WHEN `decideStatus` computes the diff union
- THEN `.lucind/` paths and gitignored files MUST be excluded from scope validation

### Requirement: Scope Violation Status Demotion

If any path in the computed git diff union is outside the declared `AllowedPaths` (neither an exact match nor a component-boundary subpath), `decideStatus` MUST demote a `done` status to `deviated` and record an `EventLaneNote` listing the offending paths. Envelopes with `blocked` or `failed` status MUST NOT be changed to `deviated`. (Trace: Decision 2 — Packet.AllowedPaths and its terminal consumers)

#### Scenario: Out-of-scope change demoted from done to deviated
- GIVEN a lane whose envelope reports `done`, but whose diff union includes a file outside `AllowedPaths`
- WHEN `decideStatus` evaluates the lane
- THEN the resolved lane status MUST be `deviated`

#### Scenario: Offending paths recorded in lane note
- GIVEN a lane demoted to `deviated` due to scope violations
- WHEN `decideStatus` records the decision
- THEN an `EventLaneNote` naming the out-of-scope paths MUST be written to the ledger

#### Scenario: Blocked envelope retained despite scope violation
- GIVEN a lane whose envelope reports `blocked` and touches an out-of-scope path
- WHEN `decideStatus` evaluates the lane
- THEN the resolved lane status MUST remain `blocked`

#### Scenario: Failed envelope retained despite scope violation
- GIVEN a lane whose envelope reports `failed` and touches an out-of-scope path
- WHEN `decideStatus` evaluates the lane
- THEN the resolved lane status MUST remain `failed`

### Requirement: Git Inspection Failure Safety

If any git inspection command fails during scope verification in `decideStatus`, the lane status MUST resolve to `blocked` with a diagnostic note, never guessing or defaulting to `done`. (Trace: Decision 2 — Packet.AllowedPaths and its terminal consumers)

#### Scenario: Git inspection error yields blocked status
- GIVEN a git command failure during diff computation (e.g. invalid base SHA or corrupted git tree)
- WHEN `decideStatus` runs scope verification
- THEN the lane status MUST resolve to `blocked` with a diagnostic explanation

### Requirement: Backward Compatibility for Undeclared Scopes

Packets that omit `allowed_paths` or have an empty `AllowedPaths` slice MUST bypass the upfront disjointness check and the post-execution diff check, preserving existing behavior and allowing propose/design dual-dispatch packets to reach `done` unmodified. (Trace: Decision 2 — Packet.AllowedPaths and its terminal consumers, Decision 5 — rollback)

#### Scenario: Packet without allowed_paths reaches done
- GIVEN a packet without `allowed_paths` whose envelope is valid and status is `done`
- WHEN `decideStatus` evaluates the lane
- THEN it MUST resolve to `done` without running the git diff scope check

#### Scenario: Undeclared scope skipped in batch disjointness check
- GIVEN a batch of propose or design packets without `allowed_paths`
- WHEN CLI pre-flight validation runs
- THEN the upfront disjointness check MUST skip them and proceed to execution
