# agy-only-contract

## Objective

Shrink lucind-ai to a deterministic contract layer for a single provider (Antigravity `agy`,
driven through herdr). Claude orchestrates (plan, criteria, merges, review); agy implements;
lucind-ai only enforces what is verifiable without judgment.

## Problem / why

lucind-ai mixes orchestration with contract: `integrate` (~1.3k LOC: deterministic merge,
bisection, lane revert, CAS promote), a worktree per lane always, a SQLite ledger with
features/leases/reconcile/defects, five executors, judges on cursor-agent and conflict
resolution on `claude -p`. All of that is judgment the orchestrator does better and cheaper.

## Boundary rule

lucind-ai owns only what is reproducible without judgment: result-contract validation,
tree-hash attestation, pass/fail checks, allowed-path enforcement. Everything else
(when to parallelize, merge, resolve conflicts, retry, review, pick a model) is Claude's.

## Target design

### CLI (all that remains)

- `dispatch --cwd <dir> --allow <glob>... --brief <file|-> [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota N]`
  - packetless; brief is free Markdown.
  - writes `.lucind/lanes/<id>/lane.json` with base tree hash (incl. uncommitted, same
    algorithm as attest), allowed globs, model, pane id; passes `LUCIND_LANE=<id>` to agy.
  - opens the agy pane via herdr; blocks until the Stop hook marks the lane done (or `--detach`
    returns `{lane, pane_id, cwd}` immediately; `wait <lane>` blocks later).
  - never closes or kills the pane. On timeout: status `timeout`, distinct exit code, pane alive.
  - `--lane <id>` continues an existing lane in the same pane (new brief, re-armed wait).
- `wait <lane> [--timeout D]` — block + validate result.
- `attest run -- <cmd>` / `attest verify --command <cmd>` — unchanged (HMAC, tree hash).
- `check` — runs `lucind-checks.sh` (scrubbed env, timeout, process-group kill). Unchanged.
- `accept --lane <id>` — writes `receipt.json` iff: result valid against schema; changed files
  vs base tree ⊆ allowed globs; a valid attestation matches the final tree (else runs `check`).
- `hook <event>` — handlers called by the agy plugin.
- `plugin install` — writes the embedded agy plugin to `~/.gemini/config/plugins/lucind/`.
  `make install` chains it so binary and plugin versions never drift.
- `--version`.

### State

Per-lane JSON files in `.lucind/lanes/<id>/{lane,result,receipt}.json`. No SQLite.
Attestations stay where they are (`$XDG_STATE_HOME/lucind-ai/attestations/...`).

### agy plugin (global, embedded in the binary)

- All hooks are pass-through when `LUCIND_LANE` is absent (free/manual agy unaffected).
- PreToolUse: deny writes outside the lane's allowed globs, except
  `.lucind/lanes/<id>/result.json`; deny reading the HMAC key and writing the attestations dir.
- Stop: validate `result.json` against the schema; if missing/invalid return
  `decision: "continue"` with the schema error as reason, max 2 retries, then lane `failed`.
- Rules + skills describing the result contract and `lucind-ai attest run` (replaces
  `rules init|generate`).

### Claude side

One Claude Code skill `lucind` (in `plugin/claude-code`): when to dispatch vs. use free agy via
herdr (read-only explore/research needs no lucind-ai), dispatch→wait→accept loop, when to
create a worktree (only parallel writers; one writer per tree), merging with plain git,
model guidance.

### Deleted

Executors `cursor-agent`, `opencode`, `claude` (and the Executor interface — agy is a concrete
adapter); `judges`; `resolve`; `integrate`/`run` (combine, bisect, revert, CAS promote,
batch, `--max-parallel`, `integrate retry`); `explore`; `split`/`dag`; `feature`, leases,
`reconcile`; `defect`; `worktree cleanup` and per-lane worktrees; `usage`/usagelog; SQLite
ledger + migrations; packets; `rules init|generate`; `plugin/opencode`.

## Constraints

- Strict TDD where a runnable deterministic test exists (RED → GREEN → refactor).
- Generated artifacts (code, comments, docs, commits) in English.
- Conventional commits; no AI attribution lines.
- Use rg/fd/bat/eza/sd, never grep/find/cat/sed/ls.
- Each task ends with ≥1 work-unit commit on this branch; push/PR/merge are the user's call.
- ~400 authored changed lines per task is a planning heuristic, not a cap (deletions excluded
  from concern — this feature is deletion-heavy).

## Roles

- Orchestrator / planner / validator: Claude (pane w1:p9). Owns this document.
- Implementer: agy (herdr pane w1:pT), may use its own subagents to keep context small.
- Validation: Claude re-runs one reported check per task and reads the diff before checking off.

## Tasks

Route for every task: delegated to agy via herdr (multi-file, deletion-heavy work).

- [x] **T0 — Spike: env inheritance in agy hooks.** Prove whether a command hook spawned by
  agy sees an env var set on the agy process (`LUCIND_LANE=spike agy ...`). Throwaway
  workspace hook writing `env` + stdin to a file. Record result + evidence here. If not
  inherited, document plan B (bind `conversationId` on first `PreInvocation`).
  Acceptance: a yes/no answer with captured evidence. No source changes.
- [ ] **T1 — Delete non-agy providers.** Remove `cursor-agent`, `opencode`, `claude`
  executors, the `Executor` interface (agy/herdr-agy concrete), `internal/judges`,
  `internal/resolve` (callers that need them are deleted in T2 — stub/remove call sites so the
  build stays green), packet `agent` field, `plugin/opencode` + its Makefile targets, related
  tests/fixtures. Acceptance: `sh lucind-checks.sh` green; `rg -i 'cursor-agent|opencode|claude -p'`
  only in docs/history.
- [ ] **T2 — Delete orchestration + ledger.** Remove `integrate`, `run` batch/integration,
  `explore`, `split`/`dag`, `feature`, `reconcile`, `defect`, `worktree` (per-lane worktree
  creation + cleanup), `usage`/usagelog, SQLite ledger + migrations, packets,
  `rules init|generate`. Introduce `.lucind/lanes/<id>/` JSON state. Keep `attest`, `check`,
  `accept` (adapted to lane files), `hook`, result schema, agy quota gate.
  Acceptance: checks green; CLI usage lists only surviving commands.
- [ ] **T3 — Packetless `dispatch` / `wait`.** Start from `feature/herdr-direct-dispatch`
  ideas. Base tree hash, `LUCIND_LANE`, blocking/detach, `--lane` continuation, timeout
  semantics, pane never closed. Acceptance: unit tests with a fake herdr; checks green.
- [ ] **T4 — agy plugin + `plugin install`.** Embedded plugin (plugin.json, hooks.json,
  rules, skills); PreToolUse allowed-path + key/attestation denial; Stop validation with
  bounded `continue`; pass-through without `LUCIND_LANE`; `make install` chains install;
  `accept` allowed-path diff vs base tree. Acceptance: hook handler tests on recorded stdin
  payloads; checks green.
- [ ] **T5 — Claude skill + docs.** `plugin/claude-code` reduced to skill `lucind`; rewrite
  `docs/product.md` and `ROADMAP.md` to this design. Acceptance: one real end-to-end lane
  (Claude → dispatch → agy → attest → accept) observed by the orchestrator.

## Progress / evidence

(updated per task: commit SHA, checks run and observed result, route, notes)

- **T0** (route: delegated to agy via herdr; no source changes). Result: **YES** — command
  hooks inherit the agy process env. `LUCIND_LANE=spike agy -p ... --dangerously-skip-permissions
  --model gemini-3.8-flash-low` in a throwaway workspace; hook script read
  `os.environ["LUCIND_LANE"]` and logged `"spike"` on all 4 events (PreToolUse + Stop).
  Orchestrator spot check: hook.py reads env (not hardcoded); 4/4 log entries carry the value.
  Plan B not needed. Facts for T4:
  - PreToolUse entries are grouped `{"matcher": "...", "hooks": [...]}`; Stop entries are flat
    objects in the `Stop` array; top level is a named group (`{"<name>": {...events}}`).
  - PreToolUse must print `{"decision":"allow"}` to not gate; Stop `{}` ends,
    `{"decision":"continue","reason":...}` re-enters the loop.
  - stdin carries `conversationId`, `workspacePaths`, `transcriptPath`, `modelName`,
    `toolCall{name,args}` (PreToolUse; `run_command` args `CommandLine`, `Cwd`), and Stop has
    `executionNum`, `fullyIdle`, `terminationReason`.
  - `~/.gemini/antigravity-cli/settings.json` `trustedWorkspaces` already contains `$HOME`.

## Decisions log (taken autonomously; for user review)

- D1 (T0): lane identity in hooks = `LUCIND_LANE` env var; no conversationId binding.
- D2: lane dir = `<git toplevel of lane cwd>/.lucind/lanes/<id>/`.
- D3: lane id = `YYYYMMDD-HHMMSS-<4 hex>` (UTC), generated by lucind.
- D4: lane files are versioned JSON written atomically; `lane.json` status ∈
  running|done|failed|timeout|accepted|rejected; `receipt.json` records base/final tree,
  changed files, verdict, reasons, evidence (attestation path or check log).
- D5: the agy CLI loads global plugins from `~/.gemini/antigravity-cli/plugins/<name>/`
  (not `~/.gemini/config/plugins`, which is the Antigravity 2.0 app). T4 `plugin install` targets
  the CLI dir and verifies with `agy plugin validate <dir>`.
- D6 (process, user-approved): agy's global `~/.gemini/GEMINI.md` is a delegating orchestrator
  role; its implementation subagents use the `worker` role from a global agy plugin
  `lucind-roles` (`agents/worker.md`, validated: "agents: 1 processed"), fallback
  `define_subagent`. Workers never commit; agy's orchestrator commits after verifying.
  agy sessions run with `--dangerously-skip-permissions` (user request).

## Next step

T1.
