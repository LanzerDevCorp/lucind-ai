# lucind-ai: product

Single source of truth for what `lucind-ai` does today (branch `dev`). Only implemented behavior lives here; unimplemented work is in [`ROADMAP.md`](ROADMAP.md). Where this file and the code disagree, the code wins.

Companion docs (kept separate, not repeated here): [`attestation.md`](attestation.md) (HMAC test attestation, dispatcher commit), [`router.md`](router.md) (route router and Jev shadow mode), [`agent-rules.md`](agent-rules.md) (`lucind-rules.md` generation), [`explore.md`](explore.md) (explorer fan-out), [`usage-log.md`](usage-log.md) (usage JSONL and report), [`feature-parent-integration.md`](feature-parent-integration.md) (feature anchors, leases, overlap, reconcile).

## 1. Purpose and boundary

`lucind-ai` is the **delegated-execution layer for work paid by subscription**. One Go binary routes execution work to CLI agents already covered by a subscription, isolates each in its own git worktree, and refuses to believe what comes back until it passes a schema and a runtime enforcement ladder.

It owns **parallel execution and the integrity of what returns**. It does not own review, delivery or lifecycle.

| Question | Answered by |
|---|---|
| "Did the executor do what I asked, and did it declare where it stopped?" (at the moment of return) | `lucind-ai`: envelope + enforcement ladder |
| "Is this diff any good?" (post-hoc, over a frozen candidate) | `gentle-ai` RDD |

Different objects, not redundant. The founding defect: a packet came back `done` with every criterion green and still violated an explicit hard stop. RDD arrives too late to see that.

**Boundary with `gentle-ai`.**
- `gentle-ai` is never patched from here; everything works through surfaces it already exposes. Its skills are read from the trees named in `.lucind/skill-roots.yaml`.
- The automated flow ends at `accept` (mechanical receipt). The binary does not trigger RDD: the review lifecycle is a negotiated transition machine that launches reviewers as host subagents and relays consent to a human. The human drives RDD from an `opencode` session (reviewer on a different model family than the orchestrator).
- `judgment-day` is a by-hand alternative to RDD, never part of the flow (it issues no receipt and carries no delivery authority).

**Hard constraints.** Subscriptions only (never a metered per-token API key); single user (no distribution or compat matrix); strict TDD for the binary; `gentle-ai` is not ours to patch.

## 2. Roster and executors

The unit of parallelism is the **subscription**, not the CLI: two CLIs on one subscription add no capacity. Execution capacity is the Antigravity and Cursor subscriptions; Anthropic orchestrates and ChatGPT reviews.

`supportedExecutors` (`cmd/lucind-ai`): an unlisted executor is a routing error, never a silent fallback. Each executor has a closed model list (`Executor.KnownModels`); a packet naming a model outside it is rejected at admission.

| Executor | Subscription | Dispatch | Models (default first) |
|---|---|---|---|
| `agy` | Google Antigravity | Inside herdr (`HERDR_ENV=1`, `LUCIND_HERDR_VISIBLE!=off`): visible interactive `agy -i` pane (see below). Otherwise headless `agy --print`. | `gemini-3.7-flash-high`, `gemini-3.8-flash-high`, `gemini-3.8-flash-medium`, `gemini-3.1-pro-high`, `claude-opus-4-6-thinking` |
| `herdr-agy` | Google Antigravity | Types `sh run.sh` into a herdr pane and waits on a nonce exit sentinel (`herdr pane wait-output --regex`). Interactive unless opted out; same models as `agy`. | as `agy` |
| `cursor-agent` | Cursor | Headless `--print --output-format json --trust --force --approve-mcps` | `cursor-grok-4.6-high` (only) |
| `claude` | Anthropic | Headless `claude --print`, edits enabled | `claude-opus-5` (only) |
| `opencode` | ChatGPT / OpenAI (OAuth) | Headless `opencode run --format json --auto`; the `agent` packet field is valid only here | `openai/gpt-5.6-sol`, `openai/gpt-5.6-luna` |

Excluded: `codex` (same ChatGPT subscription as `opencode`, no extra capacity), `opencode-go` (metered per token), `gemini` CLI (dead; `agy` succeeded it).

**Aptitude map** (routing intent, corrected when envelope data says otherwise): `agy` gets sweeps and volume (4+ files, broad mechanical change, repetitive refactors); `cursor-agent` gets single-piece precision and verification (blind judges, fast checks); Claude orchestrates and resolves bounded conflicts.

**Quota.** When a subscription runs dry the lane dies (`blocked`); it is never re-routed to another executor by quota. Before a wave that contains `agy` lanes, `AgyQuota.Ensure` checks the active account's 5-hour Gemini bucket against `--min-quota` (default `0.10`), rotating to the pooled account with most quota via `scripts/agy-pool` when needed. A packet may declare an explicit `escalation` ladder (up to 3 rungs, alternate executor/model) that the binary executes after failed verification; that is a declared policy, not a quota fallback.

### Agent factory and interactive lanes

Inside herdr every `agy` lane runs as a visible interactive session in its own pane:

- **Trust.** Interactive agy blocks on a per-path trust prompt. `internal/agytrust` adds the lane worktree to `trustedWorkspaces` in `~/.gemini/antigravity-cli/settings.json` (parse-preserving, atomic, locked; never rewrites JSON it cannot parse) and removes it when the lane ends.
- **End of work and repair.** A per-lane `.agents/hooks.json` installs an Antigravity `Stop` hook running `lucind-ai hook stop`. On `fullyIdle` it validates the envelope and writes `done.json`; an invalid or missing envelope makes the hook answer `continue` with the validation error (at most `--max-continues`, default 2) so the agent repairs it. The executor then ends the session with two C-c; the hard stop (and closing only a workspace it opened) still applies.
- **Rules.** `.agents/rules/lucind-worker.md` and `lucind-lane-scope.md` (always on) and `lucind-result-envelope.md` (on demand, schema inlined) are rendered from `lucind-rules.md` (see [`agent-rules.md`](agent-rules.md)). Hooks and rules are excluded via git `info/exclude` and removed after the lane. A repo that already tracks `.agents/hooks.json` or `lucind-*.md` rules makes the lane fail instead of overwriting.
- **Signals.** Interactive runs give no usage JSON and no process exit code; the hook, `done.json`, the pane sentinel and the deadline are the signals. Usage is logged with `tokens_known:false`.
- **herdr facts that shaped the design.** `worktree open` needs `--cwd <repo>`; `worktree remove` kills panes and ignores unique commits (lucind-ai owns worktree creation and removal, herdr only opens them); `agent wait`/`agent prompt --wait` idle detection is unreliable with agy, so completion never relies on it; `--sandbox` is not a usable guard, so the guard is the post-run `allowed_paths` diff.

Worker model: workers never commit. After verification (and any judges), the **dispatcher** makes the Conventional Commit from the packet's `commit_message` (`internal/run/commit_step.go`; see [`attestation.md`](attestation.md)). Fan-out width is capped at 3 (`--max-parallel`).

## 3. Isolation model

- One worktree per lane at `<repo-parent>/<repo-name>-worktrees/<lane>`, branch `lucind/<lane>`, `base_sha` pinned. Never a temp dir; never the main tree. Each worktree needs its own `.codegraph/` index (never copied or shared).
- `run` refuses to start from inside a linked worktree.
- A lane's `.lucind/` holds only that lane's `result.json`; the single ledger lives in the primary repo's `.lucind/`. Dispatcher commits exclude `.lucind`.
- **Bounds, all owned by the binary:** wall clock per lane (`context.WithTimeout` and child kill; `--timeout`), attempts per packet (no silent retry). Turn caps do not exist in any executor. `agy --print-timeout` is always set strictly above the binary's own deadline so the binary decides. A lane killed on its ceiling returns `blocked` with the worktree preserved (the loop evidence is inside it).
- A failed lane never blocks a good one, but nothing merges unverified (section 4, combined tree).
- **Human lane** (`lane_role: human`): runs serially outside the parallel batch; no agent ever generates, enters or writes a credential value.

## 4. Dispatch pipeline

1. **Orchestrator writes packets** (Markdown + frontmatter) and decides the lane split. This is prose and judgment, not code.
2. **Admission, zero side effects** (`dispatchcheck` plus checks in `cmd/lucind-ai`): supported executor; model inside the executor's closed list; `agent` only on `opencode`; disjoint `allowed_paths` across the batch; complete and uniform feature target; route validated against computable signals (`allowed_paths` count, new-file flag, risk tier) with declared signals (`understood`, `open_design`, `estimated_lookups`) able to upgrade `inline` to `worker`; sufficient `agy` quota for the wave; not inside a linked worktree; byte-identical Claude/OpenCode skill trees with a fresh embedded schema. Any failure exits non-zero before a worktree or ledger row exists.
3. **Worktree per lane**, then **parallel dispatch** (capped, headless or herdr pane). Optional write/test/fix loop with the declared escalation ladder.
4. **Envelope.** The agent writes `.lucind/result.json` in its worktree; the binary validates it against the embedded `internal/result/result.schema.json`. Validation is identical for every executor. For non-cooperative CLIs the schema is injected into the prompt (raw JSON); `agy --json-schema` stays on as a belt.
5. **Enforcement ladder.** The envelope is a claim, not a verdict; its `status` only enters the ladder:

   | Rung | Demotes to |
   |---|---|
   | non-zero exit or expired clock | `blocked` |
   | envelope unreadable or schema-invalid (`ErrEnvelopeUnreadable`) | `blocked` |
   | any declared hard stop with `fired: true` | `blocked` |
   | four-way git diff against `base_sha` leaves `allowed_paths` | `deviated` |
   | `skills_loaded` missing a required skill | `deviated` |
   | git state contradicts the packet mode: write packet with no unique commit or a dirty tree; read-only packet that committed or is dirty | `failed` |

   Only a lane clearing every rung freezes a candidate. Every hard stop listed in the packet must appear in the envelope whether or not it fired; an envelope omitting one is invalid. Optional verification commands and the dispatcher commit run before the completion check.
6. **Barrier** (`internal/barrier`, pure join over `lane.State`): releases when **every** lane is terminal (`done | blocked | deviated | failed`), never on `done` alone.
7. **Integration** (only `done` lanes enter; others keep their worktrees and their questions are relayed verbatim):
   - **Feature target** (default): `internal/run` `IntegrateFeature`/attempt: fenced exclusive lease, overlap gate, combined tree, mandatory checks, re-validation of refs, and compare-and-swap promotion that never touches the primary checkout. The attempt is durable and recoverable (`feature recover`).
   - **Legacy** (`--legacy-main --expected-parent-sha`): combined tree via `internal/integrate`; **green** integrates the batch, **red** bisects to isolate the broken lane, integrates the rest and returns the isolated lane to `blocked` with its worktree preserved.
   - **Conflicts** are resolved by `claude -p --model sonnet` (`internal/resolve`), bounded to 400 conflict lines; past that, or if it cannot close, it escalates to the human with both versions intact.
8. **Cleanup.** Only worktrees of lanes that integrated are removed; the `lucind/<id>` branch stays.
9. **Exit code.** 0 only if every lane reached `done` and none appears in `reverted_ids`. Blocked outcomes exit non-zero; there is no approval pause inside a dispatch.
10. **Stop.** The human runs `accept`, reads the diff, then RDD (section 1). Promotion stays a human decision, distinct from lane acceptance.

### Acceptance (`accept`)

Re-verifies a **frozen candidate out of the ledger** (not the live branch): packet digest, base and candidate commit/tree, `allowed_paths`; fails closed on a fired hard stop or unmet criterion; reuses valid attestations for every declared `verification` command (fail closed); runs repository checks in a verifier-owned detached worktree. Emits an immutable receipt, never touches a ref, is never a semantic approval. Mechanical checks run unless the lane is read-only or has a non-writing role (`lens`, `synthesis`, `verify`, `archive`, `human`); an unknown role fails closed to "checks required".

### Side tracks reusing the lane machinery

- **`explore`**: three read-only lenses (structural via CodeGraph, textual via `rg`, historical via git/Engram) plus one synthesizer producing about 2k tokens of `path:line` evidence. See [`explore.md`](explore.md).
- **Blind judges** (`internal/judges`): pre-commit gate by risk tier (`internal/risk`: passive = readback; medium = attestation + 1 judge; high = attestation + 2 judges from different families); only findings both judges confirm are fixed, at most two fix rounds. Opt-in with `LUCIND_JUDGES=on`; default models `claude-sonnet-5-thinking-high` and `gpt-5.6-sol-high` (`LUCIND_JUDGE_MODELS`), run through `cursor-agent`.
- **Router** (`internal/router`): deterministic baseline plus an advisory Jev shadow runner (`LUCIND_JEV_SHADOW=on`, numbers and booleans only, no authority). See [`router.md`](router.md).

## 5. Packet contract

Markdown with frontmatter. Notable fields: `executor`, `model`, `agent` (opencode only), `allowed_paths`, `read_only`, `read_only_paths`, `lane_role` (`lens | synthesis | apply | verify | archive | ultrafixer | human`), `feature`/`parent_ref`/`base_sha`/`expected_parent_sha`, `route`/`route_evidence`, `understood`/`open_design`/`estimated_lookups`, `verification`, `known_environmental_failures`, `commit_message`, `max_iterations` (write/test/fix loop), `escalation`, `skill`/`adhoc_skills`, `named_skills_only`, `fanout_group`. A legacy `sdd_phase` is parsed but carries no authority; mechanical-check gating is decided by `lane_role`/`read_only`.

**Envelope** (`result.schema.json`): `packet_id`, `status` (`done | blocked | deviated | failed | interaction_required`), `summary`, `hard_stops[{hard_stop, fired, note}]` required; plus `done_criteria`, `files_changed[{path, change}]`, `skills_loaded`, and the `interaction` payload for `interaction_required` (mapped to `blocked`). Lane statuses (`internal/lane`): `pending | running | done | blocked | deviated | failed`.

Packet-path results are also persisted under `<primary>/.lucind/results/`.

## 6. Ledger

SQLite (pure-Go driver, no cgo) at `<primary-repo>/.lucind/lucind.db` (`internal/ledgerpath`); exactly one per repository, never in a worktree. Schema **v11**, forward-only migrations (never rewritten). Tables: runs, lanes, events, lane_progress, lane_candidates, acceptance_receipts, approvals (vestigial; kept only so the migration chain stays intact), features, feature_leases, integration_attempts, integration_events, overlap_evidence, reconciliation_requests, reconciliation_candidates, defect_records, and two packet-author shadow tables. `busy_timeout` is 30 s.

It stays small: a ledger, not a narrative (that goes to engram). A routing decision is stored with the condition that triggered it; there is no implicit routing. Diagnosis notes are capped per stream and keep the **tail**.

Other state:

| What | Where |
|---|---|
| Usage log | `$XDG_STATE_HOME/lucind-ai/usage.jsonl` (fallback `~/.local/state/...`), mode 0600; `LUCIND_USAGE_LOG=off` disables |
| Attestation | key `~/.config/lucind-ai/attest.key` (0600); entries under `$XDG_STATE_HOME/lucind-ai/attestations/<repo_id>/` (0444) |
| herdr executor state | `$XDG_STATE_HOME/lucind-ai/herdr/`, removed on success; `run-*` dirs older than 7 days reaped |
| Rules source | `<workspace>/lucind-rules.md` (checked in) |
| Per-lane in worktree | `.lucind/result.json`, `.agents/hooks.json`, `.agents/rules/lucind-*.md` |

## 7. CLI reference

Fourteen subcommands plus `--version | -v` (prints the `git describe` build baked at compile time; install with `make install`, never an ad-hoc `go build`).

| Command | Owns |
|---|---|
| `run --packet <p>... [--timeout] [--legacy-main --expected-parent-sha] [--min-quota] [--max-parallel]` | The whole pipeline of section 4 for N packets at once. |
| `explore --objective <text> [--scope <path>...] [--id] [--timeout]` | Three-lens read-only fan-out plus synthesizer. |
| `split --dag <apply-dag.yaml> --out <dir>` | Validated waves, one packet per node, one printed `run` command per wave. Never schedules. |
| `check [--out]` | `lucind-checks.sh` where you stand, optionally freezing a transcript. |
| `accept --run <id> --lane <id>` | Mechanical re-verification and immutable receipt. |
| `feature create \| status \| recover \| renew \| lease release \| lease status \| disable` | Feature anchors (immutable while active, retire-and-recreate via `disable`) and fenced exclusive leases. |
| `reconcile approve \| decline \| cancel \| renew \| resolve` | Cross-feature overlap resolution cycle. |
| `defect record \| list \| resolve \| decline \| defer` | Durable defect records written by the ultrafixer protocol. |
| `worktree cleanup --lane <id> [--force]` | Removes a lane worktree; branch stays. |
| `integrate retry --run <id> [--lane <id>...] [--timeout]` | Rebuilds a reverted batch from the ledger and preserved worktrees, no AI dispatch. |
| `attest run -- <cmd>` / `attest verify --command "<cmd>"` | HMAC-signed test attestation bound to the git tree hash. |
| `usage report [--since] [--file] [--json]` | Aggregates `usage.jsonl`. |
| `rules init \| generate [--root] [--check]` | Generates `CLAUDE.md`, `GEMINI.md`, `AGENTS.md` from `lucind-rules.md`; never overwrites hand-written or symlinked files. |
| `hook stop --state-dir --result [--max-continues]` | agy Stop-hook handler (section 2). |

There is no `phase`, `serve` or approvals UI: the SDD phase gate and `internal/phasespec` were removed, and the approvals web UI was decommissioned.

## 8. Package map

`cmd/lucind-ai` holds the CLI, production wiring (`productionDeps`), `visible_agy.go` (agy factory) and `judges_wiring.go`.

| Area | Packages |
|---|---|
| Contract | `packet`, `packetauthor`, `result` (embedded schema), `lane`, `lanecheck`, `candidatechange` |
| Admission | `dispatchcheck`, `skillset`, `skillroots`, `skillcontent`, `lucindconfig`, `buildcheck` |
| Execution | `executor` (adapters + stream decoders, herdr/interactive variants, `agy_quota`), `agytrust`, `agyhooks`, `worktree`, `rules` |
| Orchestration | `run` (composition root: `Deps`, enforcement ladder, batch, commit step, integration, attempts), `barrier` |
| Integration | `integrate` (combine, checks, CAS), `resolve` (bounded conflict resolver), `conflicttriage` (advisory; production invoker unwired), `feature`, `overlap`, `reconcile`, `dag` |
| Evidence | `ledger`, `ledgerpath`, `accept`, `attest`, `usagelog` |
| Verification and routing | `risk`, `judges`, `router`, `explorefan` |

## 9. Invariants

- Exit 0 only when every lane is `done` and none is reverted. An orchestrator never has to remember to look for a bad outcome.
- Admission failures cause zero side effects.
- The envelope is a claim; only the ladder decides status. Exit 0 alone never means `done`.
- Terminal non-`done` lanes keep their worktrees as evidence; only integrated lanes are cleaned.
- Barrier releases on all-terminal, never on `done` alone; `barrier` imports no ledger.
- Nothing merges unverified: integration goes through a combined tree and checks, and promotion is compare-and-swap.
- Workers never commit; the dispatcher commits after verification.
- No lane runs in the main tree; the ledger is never inside a worktree.
- The binary owns every bound (clock, attempts); executor timeouts are set above it.
- No credential value is ever generated, entered or written by an agent.
- Both skill trees (Claude Code and OpenCode) are byte-identical; `run` verifies it before allocating a worktree. Plugin edits need `make bump-plugin-version`.
- Hard-wired external tools: git, `herdr` (optional), `agy`, `cursor-agent`, `claude`, `opencode`, `codegraph`, `scripts/agy-pool`.
- A new failing test is never presumed flaky: timing-sensitive tests under `internal/feature` and `internal/run` must be reproduced in isolation first.

## 10. Operational hazards

- **Cascade failure loop.** A lane reads another lane's build error as its own defect and thrashes. Worktree isolation removes the shared-scope half; the wall-clock ceiling removes the rest.
- `agy --print-timeout` defaults to 5m; the binary always overrides it above its own deadline.
- `cursor-agent` keeps its own `.cursor/worktrees.json`, which can collide.
- The conflict resolver (`claude -p`) draws on the orchestrator's Anthropic entitlement.
- Trust handling edits the user-global agy `settings.json`; a crash can leave stale `trustedWorkspaces` entries (a `settings.json.lucind.lock` stays beside it).
- A same-family verdict still blocks and raises objections; it cannot approve a merge alone.
