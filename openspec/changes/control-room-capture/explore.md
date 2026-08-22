# Explore: Control Room Capture

File-backed stream spooling on the primary repository’s `.lucind/` directory, with compact ledger milestones and loopback HTTP access, is the viable shape. SQLite chunk storage and a required streaming daemon are not.

## Problem statement and background

`lucind-ai` is a single Go binary that routes execution to subscription-backed CLIs (`agy`, `cursor-agent`, `opencode`) in isolated git worktrees (`docs/prd.md:12-18,61-70`; `internal/worktree/worktree.go:150-171,184-237`). Three gaps block a Control Room from seeing what those lanes actually wrote.

**In-flight blindness.** Each executor runs the child via `exec.CommandContext` and captures stdio into unbounded in-memory `bytes.Buffer`s (`internal/executor/agy.go:165-171`; `internal/executor/cursor_agent.go:87-93`; `internal/executor/opencode.go:126-132`). The default per-lane budget is `20 * time.Minute` (`cmd/lucind-ai/cli.go:42`). Nothing tails those buffers while the child runs.

**Success-path discard.** `executor.Outcome` holds `Stdout`/`Stderr` only as diagnosis fields (`internal/executor/executor.go:42-63`). `Execute` writes a ledger note solely when `decideStatus` returns a non-empty reason (`internal/run/run.go:422-435,549-572`). A clean envelope maps to `lane.Done` with empty reason (`internal/result/result.go:117-135`), so the `Report` keeps `Envelope` and an empty `Diagnosis` — not the streams (`internal/run/run.go:501-508`). The buffers then die with the stack.

**Clipped failure notes.** On timeout or non-zero exit (`internal/run/run.go:549-555`; `internal/executor/status.go:14-21`), `formatStreamDetail` keeps a 4096-byte tail per stream (`internal/run/run.go:89,132-144`) as `EventLaneNote` in SQLite `events` (`internal/ledger/schema.go:34-43`). That is a compact diagnosis, not a transcript.

**Approvals-only serve.** `lucind-ai serve` is a loopback HTTP UI (`cmd/lucind-ai/cli.go:112-113,674-725`; `internal/serve/server.go:19-53`). `ServerState` exposes pending `ledger.Approval` rows (`internal/serve/handlers.go:15-21`; `internal/ledger/schema.go:45-56`). `NewHandler` routes `/`, `/api/state`, and `/approvals/` (`internal/serve/handlers.go:36-118`); `serveDispatch` constructs that handler and never `NewModel` (`cmd/lucind-ai/cli.go:715`). `serve.Model` already queries features, attempts, leases, overlap, reconciliation, and audit events (`internal/serve/model.go:14-125,128-343`) and is used by `lucind-ai feature status` (`cmd/lucind-ai/cli.go:852`), not by the HTTP UI.

**Affected areas:** `internal/executor/*` (capture), `internal/run/run.go` and `batch.go` (lifecycle), `internal/run/integrate.go` (worktree teardown), `internal/serve/handlers.go` (HTTP surface), `internal/ledger` (milestones only), `plugin/claude-code/skills/lucind-ai/SKILL.md` (archive of `.lucind/` artifacts).

## Candidate approaches

| Approach | Pros | Cons | Feasibility |
|---|---|---|---|
| **1. File spool + MultiWriter + SSE** — `executor.Request` gains an output dest (`internal/executor/executor.go:14-37`); `Execute` opens log files and tees child stdio with `io.MultiWriter` at the three `cmd.Stdout`/`cmd.Stderr` assignments; serve tails files via `http.Flusher`. | No SQLite blob growth; crash-resilient append; stdlib-only; `run` and `serve` decouple through the filesystem. | Path lifecycle and retention; tailer must tolerate rewrite if a lane retries. | High |
| **2. SQLite `lane_stream_chunks` + long-poll** — additive schema beyond `schemaVersion = 5` (`internal/ledger/schema.go:10`); chunk writer into the ledger; poll `?since=`. | One store; FK cleanup. | Multi-megabyte streams vs `_pragma=busy_timeout(5000)` (`internal/ledger/ledger.go:126-128,162-163`) while lanes already write events and statuses (`internal/run/run.go:425-434`; `internal/run/batch.go:147-152,157-214`) against a 4-conn pool (`internal/ledger/ledger.go:180-184`). | Medium — viable, wrong store |
| **3. Required serve daemon, Unix socket, in-memory rings, WebSockets** | Low latency; bounded rings; interactive stdin later. | Breaks the daemonless CLI (`cmd/lucind-ai/cli.go:99-127`; `docs/prd.md:188-193`); WebSockets are non-stdlib, which `lucind-ai serve` already forbids (`openspec/changes/archive/2026-08-20-approvals-web-ui/proposal.md:16,25,83`). | Low — reject |
| **4. Hybrid (1 + ledger milestones + Model queries)** — same file tee; discrete lifecycle notes in `events`; JSON endpoints from existing `Model` methods plus file tail/download. | Heavy bytes on disk, queryable milestones in SQLite; `serve` works before, during, or after `run`; reuses `Model`. | Archive today copies `.lucind/packets/` and `.lucind/results/` only (`plugin/claude-code/skills/lucind-ai/SKILL.md:280-285`); run logs would need an explicit preserve-or-prune rule. Logs must land on the **primary root**, not the lane worktree: `completeIntegration` persists the envelope then removes the worktree (`internal/run/integrate.go:159-163`; `cmd/lucind-ai/cli.go:647-660`). | High — recommended |

**Recommendation.** Candidate 4. Candidate 2 is the same observability goal in the wrong medium. Candidate 3 fails project constraints. Candidate 1 is 4 without activating `Model` or milestone events.

Directory prefix (`.lucind/runs/` vs `.lucind/logs/`) and whether this change or `control-room-serve` registers the HTTP routes are unresolved — see synthesis notes. Split vs interleaved stdout/stderr is an open question, not a candidate fork.

## User and capability impact

Operators today cannot watch a dispatch and cannot retrieve a successful lane’s full stdio. This change would:

1. **Spool** child stdout/stderr to durable files under the primary `.lucind/` for every executor, including `lane.Done`.
2. **Keep** SQLite notes bounded (`internal/run/run.go:89,132-144`).
3. **Expose** live tail and post-mortem download on the existing loopback server (`internal/serve/server.go:19-53`; `cmd/lucind-ai/cli.go:691-694`), without a daemon — `run` and `serve` already start independently (`cmd/lucind-ai/cli.go:99-113`).
4. **Preserve** WaitDelay drain semantics so grandchild-held pipes still set `Outcome.OutputTruncated` / `Report.OutputCaptureIncomplete` (`internal/executor/agy.go:15-39,182-197`; `internal/executor/cursor_agent.go:104-118`; `internal/executor/opencode.go:143-160`; `internal/run/run.go:232-242`).

`.lucind/` is gitignored (`.gitignore:2`), so logs are local working state unless archive or a cleanup command copies them.

## Scenarios and use cases

1. **Live spool during a batch.** `lucind-ai run --packet ...` (`cmd/lucind-ai/cli.go:129-173`) runs lanes concurrently (`internal/run/batch.go:66-89`). Each `Executor.Run` (`internal/executor/executor.go:66-67`) tees stdio to a primary-root log while still filling `Outcome`.
2. **Done lane keeps a transcript.** Envelope maps to `lane.Done` (`internal/result/result.go:117-135`); `SetStatus` persists terminal state (`internal/run/run.go:480`; `internal/ledger/ledger.go:452-475`); the file remains.
3. **Unclipped failure.** Timeout or non-zero exit (`internal/run/run.go:549-555`) still writes a 4096-byte `EventLaneNote`; the file has the rest.
4. **Live Control Room tail.** `serve` on `127.0.0.1:7433` (`cmd/lucind-ai/cli.go:683`) streams file appends to a subscriber and closes when the lane ends.
5. **Uniform executors.** Same dest on `Request`; agy/cursor-agent/opencode keep their flags (`internal/executor/agy.go:141-155`; `internal/executor/cursor_agent.go:68-80`; `internal/executor/opencode.go:105-120`) and WaitDelay paths.
6. **Post-mortem after `run` exits.** A later `serve` reads the file (200) or 404s if absent.

## Technical risks and trade-offs

| Risk | Severity | What the code shows | Mitigation |
|---|---|---|---|
| Grandchild MCP processes hold stdio past child exit | High | WaitDelay + `exec.ErrWaitDelay` already exist; no `Setpgid` today | Keep WaitDelay; prove drain under tee (spike 2) |
| Unbounded in-memory capture × concurrent lanes | High | Full `bytes.Buffer` per stream (`agy.go:169-171` and siblings); batch is N goroutines (`batch.go:80-89`) | Disk tee; bound any UI replay buffer |
| Worktree-local logs vanish on integrate | Critical | Envelope is copied to `<primaryRoot>/.lucind/results/` then the worktree is removed (`cli.go:647-660`; `integrate.go:159-163`) | Write logs on the primary root; `ledgerpath.Validate` exists but is unwired (`internal/ledgerpath/ledgerpath.go:7-14,23-58`) |
| Slow SSE client backpressures the child | High | No fan-out today (`handlers.go:36-118`) | File (or non-blocking subscribers) between child and HTTP |
| OS-pipe block buffering vs TTY | Medium | All three CLIs already request JSON streaming over pipes | Measure before adding a PTY |
| ANSI in ledger notes | Medium | `formatStreamDetail` truncates only (`run.go:132-144`) | Raw bytes on disk; strip before notes |

| Dimension | Choice | Why |
|---|---|---|
| Storage | Disk append vs SQLite chunks | Avoids WAL/busy-timeout write amplification |
| Transport | `os.Pipe` vs PTY | Stdlib, no CGO; confirm JSON flags unbuffer enough |
| Location | Primary `.lucind/` vs worktree | Worktrees are deleted after integrate |
| Delivery | SSE vs short-poll | Stdlib `http.Flusher`; one connection per lane |
| Layout | Split files vs interleaved | Isolation vs exact stdout/stderr ordering — open |

## Potential spikes

1. **MultiWriter + bounded replay.** Tee to disk and a ~1MB ring; assert a stalled HTTP consumer does not stall `cmd.Run` across `ExecuteBatch` (`internal/run/batch.go:80-89`). Seams: `agy.go:165-171`, `cursor_agent.go:87-93`, `opencode.go:126-132`.
2. **Grandchild / WaitDelay.** Mock child that forks a grandchild holding pipes; confirm `OutputTruncated` without losing the parent exit (`agy.go:15-39,182-197` and siblings; `run.go:232-242`).
3. **Pipe vs TTY granularity.** Time JSON event arrival for `--output-format json` / `--format json` (`agy.go:141-143`; `cursor_agent.go:68-70`; `opencode.go:105-109`).
4. **Primary-root lifecycle.** Write under `<primaryRoot>/.lucind/…/<run_id>/`; survive `completeIntegration`; path-check like `ledgerpath.Validate`.

## Success criteria

- [ ] All three executors spool stdout and stderr continuously to primary-root files for `Done`, `Blocked`, `Failed`, and `Deviated`.
- [ ] `events.detail` stays bounded (`run.go:89,132-144`); no unbounded blobs in SQLite.
- [ ] Loopback-only access (`server.go:19-53`; `cli.go:691-694`) can download a finished transcript and tail an in-flight file, including after `run` has exited (`cli.go:99-113`).
- [ ] WaitDelay / `context.DeadlineExceeded` behavior is unchanged under spooling (`agy.go:177-197` and siblings).
- [ ] Archive/retention rule for run logs is explicit relative to today’s packets/envelopes copy (`SKILL.md:280-285`).

## Out of scope and open questions

**Out of scope (sibling changes):** UI chrome and xterm views (`control-room-ui-shell`, `control-room-ui-views`); listener lifecycle and mux ownership (`control-room-serve` — disputed with this change’s HTTP seams); SQLite schema migrations (`control-room-ledger`); token/timeline telemetry (`control-room-telemetry`); `gentle-ai` gates and admission.

**Open questions**

- [ ] Retain logs only under gitignored `.lucind/`, copy them at archive (`SKILL.md:280-285`), and/or prune via `lucind-ai worktree cleanup` / a dedicated cleanup command (`cmd/lucind-ai/cli.go:56` usage)?
- [ ] One interleaved file, or `.stdout.log` / `.stderr.log`?

**Ready for proposal:** Yes, once the two escalations in `explore-synthesis-notes.md` (path prefix; which change registers routes) and the two open questions are decided. No Go is required to choose them.
