# Synthesis Notes: Control Room Capture

## Unresolved Contradictions

1. **Log directory prefix and stream split.** All three propose drafts leave directory naming open: `.lucind/runs/<run_id>/lanes/<lane_id>.log` versus `.lucind/logs/<run_id>/<lane_id>.log`, and one interleaved file versus `.stdout.log` / `.stderr.log`. Lens B’s capability table and lens C’s worktree-deletion risk illustrate the `runs/…/<lane_id>.log` shape; lens A lists both prefixes as options and does not pick. No such directories exist in the tree. The code only settles primary root versus worktree (`internal/run/integrate.go:159-163` deletes the worktree after `PersistEnvelope`). Prefix and split vs interleaved are unpicked.

2. **Which change registers HTTP log and Model routes.** Lens A’s selected Candidate 4 adds SSE tail, transcript download, and `serve.NewModel` JSON to `lucind-ai serve`, then asks whether registration happens in this change (`internal/serve/handlers.go:36-118`) or in `control-room-serve`. Lens B’s delta makes those HTTP endpoints a MUST for this change, and asks the same question. Lens C’s out-of-scope assigns “global route registration ownership” to `control-room-serve`, while still listing SSE handler tests under this change. The current mux is approvals-only (`internal/serve/handlers.go:36-118`; `cmd/lucind-ai/cli.go:715`). That does not decide which change may add routes. This notes file does not pick.

## Coverage Gaps

None. The three drafts together cover the nine-item proposal spine. No draft specified a default retention TTL, a max log bytes per lane, or redaction of secrets in captured stdio; those are product choices, not missing spine items. The `sdd-propose` skill’s interactive proposal-question round and 450-word budget are not in the drafts; the packet sets the 1800-word budget and three-lens workflow.

## Dropped Citations

- **Lens A Candidate 3: `openspec/changes/archive/2026-08-20-apply-dag-dispatch/proposal.md` as the project constraint forbidding non-stdlib WebSockets.** That file is Apply-Phase DAG Dispatch (packets, waves, `allowed_paths`). It does not mention WebSockets or a stdlib-only HTTP rule. Candidate 3 remains rejected on daemonless CLI grounds (`cmd/lucind-ai/cli.go:99-127`; `docs/prd.md:188-193`) and because serve today is stdlib `net/http` (`internal/serve/server.go:19-53`). The stdlib/no-npm constraint for serve lives in `openspec/changes/archive/2026-08-20-approvals-web-ui/proposal.md` (out of scope / dependencies), which this draft did not cite.

- **Lens A: `cmd/lucind-ai/cli.go:647-660` as the site that “deletes lane worktrees.”** Those lines are `PersistEnvelope` writing `<primaryRoot>/.lucind/results/<laneID>.json`. Worktree deletion is wired at `cli.go:641-646` (`RemoveLaneWorktree`) and called from `internal/run/integrate.go:159-163`. Deletion claim dropped at 647-660; persist-to-primary-root kept via lens B’s `cli.go:641-660` plus `integrate.go:159-163`.

- **Lens C rollback: `internal/run/run.go:169-175` as “execution pipelines” restored by revert.** Those lines document `Deps.LaneTimeout`. They are unrelated to stream capture. Rollback restated against executor `bytes.Buffer` sites and `internal/run/run.go:422-435` (diagnosis notes).

- **Lens C additivity: `internal/ledger/schema.go:190-219` as the additive rebuild pattern for `events.type`.** That span is `migrateV4ToV5DDL`, which rebuilds `lanes` to admit executor `opencode`. Events-table rebuild for the type CHECK is `schema.go:59-78`. Claim restated from 38-39 and 59-78 only.

- **Lens C test seam: `internal/run/batch_test.go:66-100` as ExecuteBatch concurrency / isolated stream-file coverage.** Those lines are helpers (`batchPacket`, `laneEnvelopeJSON`). Concurrent execution is `TestExecuteBatchRunsLanesConcurrentlyNotSequentially` at `batch_test.go:530`. Seam citation dropped; proposal points at line 530.

- **Lens C test seam: `internal/run/integrate_test.go:20-80` as completeIntegration log-preservation coverage.** Those lines are `integrateRecorder` and the start of `newIntegrateTestDeps`. Envelope persist after integrate is `TestCompleteIntegrationPersistsEnvelopeForEveryIntegratedLane` at `integrate_test.go:392`. Seam citation dropped; proposal points at line 392.

- **Lens C test seam: `cmd/lucind-ai/cli_test.go:37-80` as end-to-end dispatch creating `.lucind/` log files.** `TestRunNoArgsPrintsUsageAndFails` starts at 37; the file header (lines 27-30) states these tests fail before `Execute`. No log-file assertion there. Seam citation dropped; proposal calls for new CLI tests.

## Scope Divergence

Lens A is authoritative: Candidate 4 (file spool + ledger milestones + Model query routing), reject Candidates 1–3.

Lens B did not re-derive candidates. It independently assumed file-backed primary-root spooling, WaitDelay preservation, 4096-byte notes, daemonless SSE, and wrote the impact table plus delta specs against that shape. Corroboration: success-path discard, clipped notes, approvals-only HTTP, unrouted `Model`. Cost: no independent feasibility take on SQLite chunks or a daemon. Divergence: named a Modified capability `ledger-events`, which does not exist under `openspec/specs/`. Canonical proposal folds that bound into `lane-execution` / `control-room-capture`. B also illustrated `.lucind/runs/…/<lane_id>.log` as if chosen; that remains an open question (contradiction 1).

Lens C did not name A’s four candidates. It independently assumed primary-root file logs, stdlib SSE, WaitDelay as the dangerous seam, and worktree teardown as the location constraint. Corroboration: grandchild pipes, unbounded buffers, Done-path discard, ANSI-in-notes, gitignored `.lucind/`, schema v5 additivity, no envelope-schema change. Divergence: out-of-scope assigns global route registration to `control-room-serve`, which is stricter than A’s Candidate 4 (and B’s HTTP MUST) — recorded as contradiction 2, not as a decided out-of-scope in `proposal.md`. C’s UI / ledger-migration / telemetry exclusions match A’s sibling split and were kept. Illustrative `.lucind/runs/` path matches B, not a second candidate.

**Convergence (all three, independently):** in-flight blindness; success-path stream loss; clipped SQLite notes; approvals-only HTTP UI; file-backed logs rather than SQLite blobs; stdlib HTTP/SSE; no required daemon; WaitDelay/grandchild drain; logs must land on the primary root so they survive `completeIntegration`.
