---
id: verify-ultrafixer-cursor-agent
executor: cursor-agent
routed_by: qualitative verification of spec compliance, edge cases, and test quality for the ultrafixer change (second, independent judge)
read_only: true
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: 6ca101ba3cc45b3d43bf6561afcd1a5d6207189a
expected_parent_sha: 6ca101ba3cc45b3d43bf6561afcd1a5d6207189a
---

# Packet verify-ultrafixer-cursor-agent

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/verify-ultrafixer-cursor-agent  ·  **Branch:** lucind/verify-ultrafixer-cursor-agent

## Goal

Perform qualitative verification of the candidate implementation for change `ultrafixer` against
its specifications, edge cases, and test quality, evaluating the frozen mechanical check results
below. You are the second, independent judgment lane — do not coordinate with or assume the
findings of the other judge (`verify-ultrafixer-agy`, executor `agy`), dispatched in the same
batch.

## Why this is safe to dispatch now

The candidate implementation is complete (proposal, design, three delta specs, tasks, and apply
all committed on `feature/ultrafixer`), and mechanical checks (`lucind-checks.sh`) have already run
and passed deterministically at commit `13b6295` (log frozen and committed one commit later at
`6ca101b`, this packet's `base_sha`). This judgment lane is read-only and does not mutate
repository state or race with other lanes.

## Preconditions

- Mechanical checks have already executed deterministically and passed (see Context below).
- Frozen mechanical check log is committed to the candidate branch
  (`openspec/changes/ultrafixer/verify-mechanical.log`) and embedded in `## Context`.
- Worktree is created from the candidate branch (`feature/ultrafixer`) at this packet's `base_sha`.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.**
      Verification citations trace to concrete symbols, spec requirements, or tests.
- [ ] **The worktree carries no unique commits and no working-tree changes relative to the lane's
      birth point.** Evidence: `git status --porcelain` empty and `HEAD` equals this packet's
      `base_sha` (`6ca101b`).
- [ ] **Qualitative evaluation completed** (`.lucind/result.json` populated with `status`,
      `summary`, and structured `findings`).

## Allowed paths

None. This is a read-only judgment lane. Do NOT create, modify, or delete any tracked or untracked
files in the worktree, other than `.lucind/result.json`.

## Allowed paths outside the repository

None.

## Out of scope

Do NOT execute `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suite.
Deterministic mechanical checks have already run twice (once flaked on a documented,
isolation-verified pre-existing concurrency issue unrelated to this change —
`TestConcurrentLeaseAcquisition` under `internal/feature`, listed as a known full-suite
timing/concurrency failure in this repo's own troubleshooting reference — and once passed clean);
their frozen output is in `## Context`. Re-running them wastes quota and adds no new signal. Do NOT
modify any source files or commit any changes.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired. An undeclared hard stop invalidates the result.

- Executing mechanical test/build commands when mechanical results are already provided.
- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- Two reasonable interpretations exist for a spec requirement and the specification does not say
  which.
- Satisfying one instruction in this packet would require violating another.

## Tool selection guidance

Perform your qualitative evaluation using read/navigation tools (`Read`, `Glob`, `Grep`,
`codegraph`) and read-only git queries (`git diff`, `git log`, `git show`). Do NOT use shell
execution for build or test runners.

## Evaluation areas

1. **Spec compliance**: Verify that the implementation (`internal/ledger/schema.go`,
   `internal/ledger/ledger.go`, `cmd/lucind-ai/cli.go`,
   `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`,
   `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md`) satisfies
   every MUST/MUST NOT requirement and scenario in
   `openspec/changes/ultrafixer/specs/ultrafixer-dispatch/spec.md`,
   `openspec/changes/ultrafixer/specs/defect-records/spec.md`, and
   `openspec/changes/ultrafixer/specs/dependencies-defects/spec.md`.
2. **Edge cases**: Identify any missing edge-case handling, negative scenarios, boundary
   conditions, or concurrency concerns — particularly around the new `defect_records` schema v8
   migration (idempotency on reopen, CHECK constraint enforcement), the `disposition` state
   machine, and the CLI flag validation for `lucind-ai defect record`/`lucind-ai defect list`.
3. **Test quality**: Evaluate whether the new test cases
   (`internal/ledger/schema_test.go`, `internal/ledger/ledger_test.go`, `cmd/lucind-ai/cli_test.go`,
   `internal/packet/packet_test.go`) assert on real terminal behavior (actual DB rows, actual CLI
   stdout/stderr, actual parsed packet structure) rather than tautologies, mocks, or internal
   implementation details.
4. **Design fidelity**: Confirm the implementation matches `design.md`'s Architecture Decisions —
   in particular, confirm there genuinely is no new Go code under `internal/run/`,
   `internal/executor/`, or `internal/packet/` beyond the one new test file (`packet_test.go`
   only gains a template-contract test; no production code there changes) as
   `design.md`'s "Zero new Go dispatch plumbing" decision claimed.

## Context

### Mechanical check summary

Command: `lucind-ai check --out openspec/changes/ultrafixer/verify-mechanical.log` (wraps
`lucind-checks.sh`). Exit code: 0. Duration: 58.01288831s. Candidate git SHA checked: `13b6295`
(the mechanical-check-log commit itself, `6ca101b`, is one docs-only commit later and is this
packet's `base_sha` — the checked content is identical except for the added log file).

Note for the record: an earlier run of the same check at the same commit failed once on
`TestConcurrentLeaseAcquisition` (`internal/feature/feature_test.go:429`,
`SQLITE_BUSY`/"database is locked") — this exact test is explicitly listed as a known
full-suite timing/concurrency flake in
`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md`'s Verification
traps section. It was reproduced 3/3 clean in isolation
(`go test ./internal/feature/... -run TestConcurrentLeaseAcquisition -count=3 -v`) before being
classified as unrelated flakiness, per that same doc's explicit instruction to reproduce a named
failure repeatedly in isolation before doing so. The full suite then passed clean on retry (the
transcript below).

### Mechanical check transcript

```
=== lucind-ai mechanical check ===
Git Commit SHA: 13b6295ab5a5f5d681b03fd8a26b975168373dd0
Command: lucind-checks.sh
Duration: 58.01288831s
Exit Code: 0
==================================
ok  	github.com/LanzerDevCorp/lucind-ai/cmd/lucind-ai	56.492s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/barrier	1.015s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/buildcheck	1.715s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage	2.316s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage/fixture	2.957s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/dag	1.058s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/executor	16.865s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/feature	4.326s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/integrate	5.071s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/lane	1.011s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/lanecheck	1.174s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/ledger	18.259s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/ledgerpath	1.012s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/overlap	1.601s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/packet	1.037s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/reconcile	6.544s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/resolve	1.783s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/result	1.045s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/run	42.865s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/serve	12.849s
ok  	github.com/LanzerDevCorp/lucind-ai/internal/worktree	1.890s
```

Also on record: the apply Lane's own full-suite run (`go test ./... -race -count=1`, stricter than
`lucind-checks.sh`) passed clean twice — once in the preserved deviated-Lane worktree before the
manual merge, and once again on `feature/ultrafixer` immediately after the merge — both including
`internal/feature` with no failure.

### Relevant specifications and design documents

- `openspec/changes/ultrafixer/proposal.md`
- `openspec/changes/ultrafixer/design.md`
- `openspec/changes/ultrafixer/specs/ultrafixer-dispatch/spec.md`
- `openspec/changes/ultrafixer/specs/defect-records/spec.md`
- `openspec/changes/ultrafixer/specs/dependencies-defects/spec.md`
- `openspec/changes/ultrafixer/tasks.md`

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well
the work went.

Omit the `commit` field (or leave it empty) per read-only envelope convention. Report all
qualitative observations in `findings` with `finding`, `evidence` (`file:line` or command output),
and `affects`.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
