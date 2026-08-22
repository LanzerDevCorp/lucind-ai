# Synthesis Notes: Control Room Capture

## Unresolved Contradictions

1. **Log directory prefix.** Lens A candidate 4 and lens B scenarios/success criteria place files at `.lucind/runs/<run-id>/lanes/<lane-id>.log` (A candidate 1 also uses `.lucind/runs/` but split `{stdout,stderr}.log` names). Lens C’s critical risk, trade-off matrix, and spikes place files at `<primaryRoot>/.lucind/logs/<run_id>/<lane_id>.log` (spikes use split `{stdout,stderr}.log`). No such directories exist in the tree; the code only settles *primary root vs worktree* (worktrees are deleted at `internal/run/integrate.go:159-163`). Prefix `runs` vs `logs` is unpicked.

2. **Who registers HTTP log routes.** Lens A candidates 1 and 4 and lens B scenarios 4/6 plus success criteria add `/api/runs/{runID}/lanes/{laneID}/stream` and `/logs` on `internal/serve/handlers.go:36-118`. Lens C out of scope assigns “HTTP server multiplexing, routing registration, and loopback listener lifecycle” to `control-room-serve`. The current mux is approvals-only (`internal/serve/handlers.go:36-118`; `cmd/lucind-ai/cli.go:715`); that does not decide which change may add routes.

## Coverage Gaps

None. The three drafts together cover the eight-item exploration spine. No draft specified default retention TTL, max log bytes per lane, or redaction of secrets in captured stdio; those are product choices for proposal, not missing spine items.

## Dropped Citations

Claims below were removed or restated. `explore.md` uses only lines opened in this worktree that actually support the surviving sentence.

- **Lens A: `internal/executor/agy.go:133-138`, `cursor_agent.go:102-106`, `opencode.go:117-121` as `exec.CommandContext` + `bytes.Buffer`.** Those spans are worktree-boundary comments (agy), `ErrWaitDelay` handling (cursor-agent), and `--dir`/WaitDelay setup (opencode). Capture is `agy.go:165-171`, `cursor_agent.go:87-93`, `opencode.go:126-132`. Claim restated from those sites. Candidate 1’s “use `io.MultiWriter` at agy.go:133-138” is the same miss; `io.MultiWriter` does not exist in the repo.

- **Lens A/B: `internal/run/run.go:408-415` as discarding stdout/stderr on `lane.Done`.** Those lines are `enforceAllowedPaths` / `enforceCompletionMode`. Discard is implicit: diagnosis is appended only when `reason != ""` (`run.go:422-435`) and `Report` has no stream fields (`run.go:501-508`). Claim restated from those sites.

- **Lens A: “non-terminal failure or timeout.”** Timeout and non-zero exit are terminal `lane.Blocked` (`internal/executor/status.go:14-21`; `run.go:549-555`). Wording dropped.

- **Lens A: `internal/ledger/ledger.go:67-75` as `_busy_timeout=5000`.** Those lines are `ErrOverlapEvidenceNotFound` and `ExecutorNotAdmittedError`. DSN/pragma is `ledger.go:126-128,162-163`. Claim restated.

- **Lens A: `docs/prd.md:50` as stdlib-only.** Line 50 is “Subscriptions only. Never a metered per-token API key.” Stdlib-only for serve is `openspec/changes/archive/2026-08-20-approvals-web-ui/proposal.md:16,25,83`. `prd.md:50` dropped.

- **Lens A candidate 2: `internal/ledger/ledger.go:180-210` as a chunking writer into `Ledger`.** Those lines set `SetMaxOpenConns(4)` and implement `checkPragmas`. No chunk API exists. Candidate 2 kept as a design option; that seam citation dropped.

- **Lens A: “parallel 4-lane dispatches” at `internal/run/batch.go:66-147`.** `ExecuteBatch` is N-way (`batch.go:66-89`); 4 is the SQLite pool size (`ledger.go:180-184`), not a dispatch width. Softened to concurrent lanes.

- **Lens A candidate 4: milestone types “heartbeat” and “envelope parse” in `events`.** Current CHECK admits `run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended` (`schema.go:34-43`). Those two names are not in schema; not claimed as existing.

- **Lens B: `internal/run/run.go:390-403` as non-zero exit or timeout.** That branch is the never-ran `exec.Run` error path. Timeouts/exits go through `decideStatus` (`run.go:549-555`). Claim restated.

- **Lens B: `cmd/lucind-ai/cli.go:99-107` as “`lucind-ai run` has exited.”** Those lines print usage on empty args and start the subcommand switch. Post-mortem-without-daemon restated from `cli.go:99-113`.

- **Lens B open question: `lucind-ai cleanup` at `cli.go:118-119`.** Line 118 is `case "worktree":`. Usage lists `lucind-ai worktree cleanup --lane <id>` (`cli.go:56`). No `lucind-ai cleanup` command. Question restated against that usage string.

- **Lens C: `internal/run/run.go:71-89` as unbounded in-memory buffering.** That span is `streamDetailCap = 4096` and its comment — the ledger *bound*. Unbounded capture restated from the executor `bytes.Buffer` assignments.

- **Lens C mitigation implying `Setpgid` is present.** No `SysProcAttr` / `Setpgid` in `internal/executor`. WaitDelay-only is current; `Setpgid` not described as existing.

- **Lens B `result.go:43-125` as “evaluates to `lane.Done`.”** That range is mostly struct types; mapping is `Envelope.LaneStatus` at `result.go:117-135`. Tightened.

Citations that resolved and support their claims were kept (with line corrections only where listed above). `internal/serve/static/app.js` has no `file:line` in the drafts; the file exists and polls `/api/state` only — mentioned in notes here, not used as a load-bearing citation in `explore.md`.

## Approach Divergence

Lens A is primary: four candidates, reject daemon/WebSockets, recommend hybrid file spool + ledger milestones + `Model` query routing.

Lens B did not re-derive candidates. It assumed A’s candidate 4 path shape (`.lucind/runs/<run-id>/lanes/<lane-id>.log`, one file) and wrote impact, six scenarios, and success criteria against that shape. Cost: no independent feasibility take on SQLite chunks or a daemon; those rest entirely on A (with C’s lock-contention and stdlib evidence). B independently corroborated the problem (success discard, 4096-byte notes, approvals-only `ServerState`, unrouted `Model`) and the daemonless `run`/`serve` split.

Lens C did not name A’s four candidates. It assumed `.lucind/logs/` and, in spikes, split stdout/stderr files. It placed HTTP routing, schema migrations, UI, and telemetry in sibling changes — so its “out of scope” is stricter than A/B, which put log/SSE routes on `handlers.go`. Cost: no candidate table; routing ownership left as contradiction (1) above. C independently corroborated file-backed primary-root storage, stdlib pipes, SSE over WebSockets/PTY, and the critical worktree-teardown constraint (`PersistEnvelope` then `RemoveLaneWorktree`), which A/B implied by writing `.lucind/` but did not pin to the primary root.

**Convergence (all three, independently):** in-flight blindness; success-path stream loss; clipped SQLite notes; approvals-only HTTP UI; file-backed logs rather than SQLite blobs; stdlib HTTP/SSE; no required daemon; WaitDelay/grandchild drain is the dangerous executor seam.
