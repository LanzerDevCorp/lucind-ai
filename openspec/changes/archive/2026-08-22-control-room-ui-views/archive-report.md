# Archive Report: Control Room UI Views

## Verdict

PASSED (two rounds). Final mechanical verification passed on commit `fea7459f7ab222243aa0ba51b7d39a430056fdef` (60.79s, exit 0, all packages green under `CGO_ENABLED=0 go build ./... && go test ./... -race -count=1`), recorded in `openspec/changes/control-room-ui-views/verify.md` and frozen in `verify-mechanical.log`. Round 1 qualitative verification raised no blockers, but the orchestrator's task-completion gate surfaced that `(*serve.Model).ListBatchLanes` lacked a caller. Round 2 remediation lane (`api-gaps`) implemented the missing routes and test cases in `fea7459`, promoting cleanly. No CRITICAL issues remain.

## What Shipped

Five capabilities updated and merged into `openspec/specs/`:

1. **`approvals-web-ui`** (Modified):
   - Merged modified requirement `Individual Decisions Without Bulk Approval` (4 scenarios: Fresh load starts unselected, Unselected item cannot be decided, Bulk request rejected, Single-item decision recorded) reflecting multi-panel dashboard integration and HTTP 400 rejection for array/multi-item bodies.
   - Capability total: 4 requirements, 9 scenarios.

2. **`batch-wave-view`** (Added):
   - Created live specification with 1 requirement (`Batch and DAG Wave Inspection`) and 2 scenarios (`Wave grouping and lane lifecycle inspection`, `Barrier release with mixed terminal statuses`).

3. **`feature-lease-monitor`** (Added):
   - Created live specification with 1 requirement (`Shell-Free Feature and Lease Monitoring`) and 3 scenarios (`Active lease and attempt inspection`, `On-demand overlap evidence display`, `Expired lease queried without shell subprocess`).

4. **`lane-envelope-inspector`** (Added):
   - Created live specification with 1 requirement (`Lane Demotion Diagnosis`) and 3 scenarios (`Deviated lane displays offending paths and preserved worktree`, `Multiple out-of-scope paths formatted in diagnosis note`, `Preserved worktree inspected without direct disk read`).

5. **`reconciliation-workspace`** (Added):
   - Created live specification with 1 requirement (`Reconciliation Candidate Inspection`) and 2 scenarios (`Read-only reconciliation request and check inspection`, `Failed CAS promotion displays failure reason`).

Total delta requirements shipped across 5 capabilities: 5 requirements (1 modified, 4 added) and 14 scenarios.

## Dispatch Record

All lanes executed with `gemini-3.7-flash-high` across planning, apply, verify, remediation, and archive:

- **Planning** (24 lanes):
  - 18 lens exploration/proposal/spec/design/task lanes dispatched to `agy` (`gemini-3.7-flash-high`).
  - 6 synthesis lanes executed on `cursor-agent`.
- **Apply** (4 lanes):
  - `ui-features`: `agy` (`gemini-3.7-flash-high`), `read_only: false`, promoted on first attempt. Result envelope preserved.
  - `ui-timeline`: `agy` (`gemini-3.7-flash-high`), `read_only: false`, promoted on first attempt. Result envelope preserved.
  - `ui-approvals`: `agy` (`gemini-3.7-flash-high`), `read_only: false`, marked `deviated` due to uncreated separate file paths in `allowed_paths`, hand-integrated.
  - `cli-wiring`: `agy` (`gemini-3.7-flash-high`), `read_only: false`, marked `deviated` due to uncreated separate file paths in `allowed_paths`, hand-integrated.
- **Verify** (1 lane):
  - `verify-control-room-ui-views`: `agy` (`gemini-3.7-flash-high`), `read_only: true`, qualitative judgment lane.
- **Remediation** (1 lane):
  - `api-gaps`: `agy` (`gemini-3.7-flash-high`), `read_only: false`, closed HTTP route and testing gaps, promoted in `fea7459`. Result envelope preserved.
- **Archive** (1 lane):
  - `archive-control-room-ui-views`: `agy` (`gemini-3.7-flash-high`), `read_only: false`, mechanical archival and spec sync.

Preserved dispatch artifacts:
- 7 packets preserved under `openspec/changes/control-room-ui-views/packets/` (`api-gaps.md`, `archive.md`, `cli-wiring.md`, `ui-approvals.md`, `ui-features.md`, `ui-timeline.md`, `verify.md`). Sourced from `openspec/plans/control-room/control-room-ui-views/packets/` because `.lucind/packets/` is absent in this repository.
- 3 result envelopes preserved under `openspec/changes/control-room-ui-views/envelopes/` (`api-gaps.json`, `ui-features.json`, `ui-timeline.json`). Envelopes for `ui-approvals`, `cli-wiring`, and `verify-control-room-ui-views` were not written to `.lucind/results/` as their integration attempts did not promote.

## Follow-ups

1. **Reconciliation UI POST surface** (`tasks.md:89`): Decide whether UI should support POST mutations (`approve`, `decline`, `cancel`, `renew`, `resolve`) or retain the copy-paste CLI surface (`cmd/lucind-ai/cli.go:1044-1065`).
2. **Lease countdown source** (`tasks.md:90`): Choose between Model `remaining_seconds` calculation vs `expires_at` plus server-side timestamp comparison.
3. **Overlap evidence rendering** (`tasks.md:91`): Evaluate `<pre>` + `escapeHtml` display versus an inline diff tokenizer for `evidence_json`.
4. **`NewHandler` Model parameter** (`tasks.md:92`): Refactor `NewHandler` to accept `*Model` directly rather than constructing `NewModel(l)` internally.
5. **Apply DAG sidecar paths** (`verify.md:40`): Update future sidecars to target consolidated files (`app.js`, `index.html`, `static_test.go`, `cli_test.go`) instead of legacy separate-view file names.
6. **Dual verification dispatch** (`verify.md:41`): Use dual qualitative dispatch or explicit checklist verification to catch dead code and missing consumers earlier.

## Gaps and Contradictions

- **Task 4.1 Superseded**: Task 4.1 (`tasks.md:53`) is marked `[~] SUPERSEDED`. It specified five per-panel 2-second client polls. The implementation converged on one composed `/api/state` endpoint plus an SSE stream with 2-second polling fallback, which provides superior efficiency and consistency. Authorized by the orchestrator.
- **Checkbox Remediation**: Implementation tasks 1.1, 1.2, 2.1, 2.2, 3.1, and 5.1 were reconciled after being completed and verified by remediation lane `api-gaps` in commit `fea7459`.
- **Packet Directory Location**: `.lucind/packets/` was absent; packets were preserved from `openspec/plans/control-room/control-room-ui-views/packets/`.
- **Result Envelopes**: Envelopes for `ui-approvals`, `cli-wiring`, and `verify-control-room-ui-views` were absent from `.lucind/results/` due to unpromoted integration attempts.
