# herdr agent factory: design decisions and handoff

Status: design agreed with the owner on 2026-10-03; implementation in progress on `feature/herdr-agent-factory`.
Tracking: `odd/tasks/herdr-agent-factory.md` (tasks T1-T15, authoritative checklist) and its Engram mirror `odd/herdr-agent-factory/tasks` (project `lucind-ai`).
This document consolidates every decision taken in the design session so implementation can continue in this repository without the original conversation.

## 1. North star

Turn lucind-ai into a multi-provider agent factory:

- **Claude** orchestrates: design, task definition, product and technical decisions, the human layer, and final judgment. Target share of work: about 25%.
- **Antigravity CLI (`agy`)** does the heavy lanes: explorers, writers, fixes, tests, e2e, UAT, screenshots. Target: about 60% or more, using fan-out and loops.
- **Cursor CLI (`cursor-agent`)** does verification: blind judges from different model families and fast grep-based checks. Target: about 15%.
- **herdr** panes are the principal executor. lucind-ai keeps owning worktrees, packets, envelopes, scope enforcement, the ledger and acceptance.
- **No SDD.** The workflow is ODD (Organic Driven Development): a feature document plus task packets. Not every task needs fan-out: a tiny, understood fix is done inline by the orchestrator.
- **Deterministic where it can be, probabilistic only where it must be.** Test results are attested by a signed log, risk is classified by rules, routing is validated by rules; a System One model (Jev) only advises on fuzzy signals, in shadow mode first.

## 2. Architecture summary

```text
Claude (orchestrator, 25%)
   |  writes ODD feature doc + task packet (Markdown + frontmatter)
   v
lucind-ai dispatcher (Go)
   |  creates worktree (base_sha pinned), validates route vs signals, derives risk tier
   |  HerdrExecutor: opens worktree in herdr, runs headless CLI in a pane,
   |  captures stream-json + exit sentinel, validates the JSON result envelope
   v
herdr panes
   |-- agy headless workers (60%)     -> write code, never commit
   |-- cursor-agent judges (15%)      -> blind review by risk tier
   v
attestation (HMAC over test result + git tree hash) -> dispatcher commits -> integrate to feature branch
```

Key properties:

- A worker never commits. After a green attestation and the tier's judges, the dispatcher makes the Conventional Commit from the packet.
- The attestation binds the exact working tree (including uncommitted files), so the commit that follows is the tree that was tested, and nobody re-runs tests unless the tree changed.
- Escalation is a ladder declared by Claude in the packet and executed deterministically by the binary: retry in the same lane, then a stronger model in that lane, then Claude, then the owner.

## 3. Decisions

### 3.1 Scope and structure

| Decision | Choice |
|---|---|
| gentle-ai fork | No. Extend lucind-ai in place (canonical repo). `lucind-ai-deterministic-orchestrator` is a stale ancestor; do not touch it. |
| Coexistence with gentle-ai orchestrator | A workspace `CLAUDE.md` states that delegation goes through the dispatcher. If friction appears: edit `~/.claude/CLAUDE.md` by hand and stop running `gentle-ai sync` (review major updates manually). `sync` only rewrites marker-delimited sections, so rules placed outside the markers survive it (reported, not verified). |
| Execution contract | ODD feature doc written by Claude plus a task packet per task; worker returns a schema-validated JSON envelope. |
| Packet format | Keep Markdown with frontmatter; add machine fields via `packetauthor`. |
| Worker modes | Headless by default (`agy -p`, `cursor-agent -p`) inside herdr panes; interactive only for UAT and exploratory debugging. |
| Rules distribution | One source of truth generating `CLAUDE.md`, `GEMINI.md` and `AGENTS.md` per workspace (T15). |
| How Claude talks to the dispatcher | CLI via Bash (`dispatch run <packet>` style), bounded output. No MCP server. |

### 3.2 Accounts and quota

- Only one `agy` account can be active at a time (the owner tested it; credentials live in the OS keyring). Rotation uses the owner's `agy-pool` script (`scripts/agy-pool`).
- Claude supports two accounts in parallel via `CLAUDE_CONFIG_DIR`.
- Real parallelism therefore comes from git worktrees, not from accounts. Fan-out width cap: 3 workers.
- Usage target 60/15/25 is measured, not assumed: per-call JSONL log (provider, model, tokens, lane) and a report command (T13).

### 3.3 Verification and risk

- **Test attestation (T1).** `lucind-ai attest run -- <cmd>` records `{command, exit_code, tree_hash, timestamps}` signed with HMAC (key in `~/.config/lucind-ai/attest.key`, mode 0600, never in a repo); log files are mode 0444 under `~/.local/state/lucind-ai/attestations/<repo_id>/`. `attest verify --command <cmd>` passes only if the command, exit code 0, current tree hash and MAC all match. Threat model: accidental (an agent claiming tests passed or repeating them), not adversarial; a same-user agent could still forge entries. A dedicated Linux user was considered and rejected as too heavy.
- **Risk tiers (T10).** Port a minimal classifier of gentle-ai's `ClassifyRisk` (it is an `internal` package and cannot be imported): path tokens `auth|update|security|webhook|payments`, risk signals, byte-proven passive content; any failure counts as high. Tiers map to verification: passive means structural readback only; medium means attestation plus one judge; high means attestation plus two blind judges plus Claude. This replaces the earlier "two judges on every work unit".
- **Judges.** Two blind judges from different model families (via `cursor-agent`), review the same frozen target, only findings both confirm are fixed, at most two fix rounds (rules borrowed from gentle-ai's `judgment-day`). Receipt-driven development (RDD) with Claude reviewers is reserved for high risk.
- `cursor-agent` has never been run end to end in lucind-ai; T10 starts by proving it.

### 3.4 Dispatch threshold (T11)

Claude declares a route in the packet; the dispatcher validates it against computable signals and upgrades or rejects mismatches.

- **Inline (Claude does it):** at most one mechanical, already-understood file; no new file; no research or open design; evidence within 3 calls and about 10k tokens; no high-risk path. Run one focused test.
- **Single worker:** two or more non-trivial files, a new file, more than about five lookups, reading that prepares a write, or a suite large enough to flood the orchestrator's context.
- **Fan-out:** only independent read-only lookups (explorers) and independent tasks in separate worktrees. Writes stay single-threaded per worktree.
- Computable signals: number of files in `allowed_paths`, new-file flag, risk tier. Declared signals (set by Claude): already understood, open design, estimated lookups.
- Origin: gentle-ai's ODD constants (`internal/agents/capabilitymanifest/manifest.go:214-236` in the gentle-ai repo). They are prose enforced by model judgment there; here the computable part is enforced in code.

### 3.5 Router with a System One model (T14)

- A `Router` interface in the dispatcher. Implementations: deterministic rules (baseline and permanent fallback) and **Jev** (TypeSafe AI System One model, early access; the owner has access and credits).
- Jev runs in **shadow mode**: it records its decision next to the deterministic one in the usage JSONL, has no authority, and may gain it only after sustained agreement above a confidence threshold.
- Risk tier and hard signals stay deterministic. Jev is for fuzzy signals (is the fix understood, is there open design, is a worker stuck).
- Send only numeric signals and boolean flags until Jev's data-retention terms are read (`https://docs.typesafe.ai/legal.md`; not reviewed yet). Never send diffs or code.
- Jev has Python and JavaScript SDKs only; use plain HTTP from Go (`https://docs.typesafe.ai/api.md`, not read yet). Claims (70-500 ms, $0.042 per million input tokens) are the vendor's and unverified.
- Alternative evaluated: Fastino GLiDE (`POST https://api.fastino.ai/v1/systemone`); it retains data indefinitely and may train on it unless `store: false`, and offers no DPA.

### 3.6 Worker contract

- Port the rules of gentle-pi's `gentle-ai-worker` (`assets/agents/gentle-ai-worker.md`) into `lucind-apply`: exact allowed edit surfaces, tool safety, strict TDD discipline, escalate ambiguity with `interaction_required` instead of guessing, report evidence honestly.
- Add packet fields (T2): `verification` (exact commands), `known_environmental_failures`, `route` and `route_evidence`, injected skills by exact path; and the `interaction_required` status in `internal/result/result.schema.json`.
- Status mapping until T2 lands: `completed` -> `done`, `partial` -> `deviated`, `blocked` and `interaction_required` -> `blocked`.
- Workers load only `lucind-executor` (always), `lucind-apply` (writers) or `lucind-verify` (judges). gentle-ai skills (`branch-pr`, `work-unit-commits`, ...) enter only through `adhoc_skills`. Pocock-style thinking skills (`grilling`, `domain-modeling`) are used only by Claude before ODD.
- Permissions: keep `--dangerously-skip-permissions` with `--mode accept-edits`; the guard is the post-run `allowed_paths` diff check (`enforceAllowedPaths`, `internal/run/run.go`). Whether `agy --sandbox` adds protection is unknown and is part of spike T7.

### 3.7 SDD removal plan (T3-T5)

Facts found by reading the code:

- `sdd_phase` and `fanout_group` live only in the `lane_metadata:v1:` event JSON (`internal/ledger/lanes_meta.go`); there is no ledger column. Removing them needs no schema migration, but readers must keep decoding old events.
- The only behavioral coupling is the gate `SDDPhase == "" || SDDPhase == "apply"` at `internal/accept/accept.go:120` and `internal/run/attempt.go:391` (which lanes run mechanical checks). Neutralize it first onto a lane-role or `read_only` predicate that fails closed.
- `internal/skillset/skillset.go:59-95` hardcodes `sdd-*` skills that are not shipped in this repo (they resolve from `.lucind/skill-roots.yaml`, i.e. from gentle-ai).
- The packet identity digest includes the SDD fields (`internal/run/run.go:682`); dropping them changes digests of existing packets.
- Plugin edits require `make bump-plugin-version`, and the Claude Code and OpenCode skill copies must stay byte-identical (`cmd/plugincontent`).

Order, safest first: (1) retire the 21 SDD packet templates and `references/strategies/sdd.md`, add an `odd.md` strategy; (2) make `sdd-*` derivation optional; (3) neutralize the gates; (4) accept and ignore `sdd_phase`; (5) delete the `phase` subcommand and `internal/phasespec`. **Deferred:** deleting the `SDDPhase` and `FanoutGroup` fields (about 130 test hits).
Keep `lens` and `synthesis` lane roles for the three-lens explorer fan-out (structural via CodeGraph, textual via `rg`, historical via git and Engram, merged by an agy synthesizer into about 2k tokens of path:line evidence) and reuse `archive` as "close task". Keep `feature`, `split` and `integrate`.

### 3.8 Worktrees

- lucind-ai creates and cleans worktrees (`internal/worktree`): lane-derived names, `base_sha` pinning, parent-ref validation, and safety checks before cleanup (`HasUniqueCommits`, `PorcelainEmpty`). herdr only opens them as workspaces (`herdr worktree open --path`). herdr's own `worktree create` does not pin a SHA.
- Worktrees live under `~/git_root/lucind-ai-worktrees/`, never `/tmp` (CodeGraph requirement); each needs its own `.codegraph/` (`gentle-ai codegraph init --cwd <worktree>`).
- Unverified (spike T7): `herdr worktree open` on an externally created worktree, and `herdr worktree remove` with live panes.

### 3.9 Model assignment (starting point, calibrate with logs)

| Lane | Provider | Model |
|---|---|---|
| Orchestration, design, decisions | Claude | Opus (Sonnet for routine) |
| Writer, apply, fix | agy | `gemini-3.8-flash-high` |
| Explorer, research, tests, e2e, UAT | agy | `gemini-3.8-flash-medium` |
| Hard implementation, on demand | agy | `claude-opus-4-6-thinking` |
| Blind judges | Cursor | two different families (for example Sonnet 5 thinking and GPT-5.6 Sol) |
| Fast grep checks | Cursor | Composer 2.5 or a Grok fast model |

Claude models used through Cursor consume Cursor quota, not Anthropic's. Avoid the models marked "NO ZDR" in `cursor-agent --list-models` for sensitive code.

### 3.10 Commits and delivery

- Conventional Commits, one work unit per task, tests and docs with the code.
- Delivery strategy: `feature-branch-chain`. Tasks land on `feature/herdr-agent-factory`; the forecast (about 3,000+ authored lines) exceeds the ~400-line budget, so slices are recorded in the feature doc.
- The owner's global rules apply: no AI attribution trailers in commits, tools `bat`/`rg`/`fd`/`eza` instead of `cat`/`grep`/`find`/`ls`.

## 4. Changes made outside this repository

- `~/.gemini/GEMINI.md` was replaced on 2026-10-03 (578 lines -> 160) with a worker-only role plus the remote-authorization and CodeGraph sections. Backup: `~/.gemini/GEMINI.md.bak-2026-10-03`. **Do not run `gentle-ai sync` for Antigravity**; it would re-inject the orchestrator sections.
- `~/.gemini/system.md` still carries an Engram protocol telling `agy` to save memory proactively. It conflicts with the worker's "do not read persistent memory" rule; whether `agy` loads it is unverified. Undecided.
- A CodeGraph index was initialized in the `gentle-ai` repository, and one is initialized per lucind-ai worktree when it is created.

## 5. Open items and unverified claims

| Item | Status |
|---|---|
| What `agy --sandbox` restricts | Unknown; spike T7 |
| `herdr worktree open` on external worktrees; `remove` with live panes | Unknown; spike T7 |
| Exit code and token capture in a pane | Plan: run the CLI headless with `--output-format stream-json`, tee to a file, write an exit sentinel; `herdr agent wait` / `pane wait-output` to detect the end. Not yet proven end to end in a real executor |
| Jev API format and data-retention terms | Not read; read before T14 |
| Documented lucind-ai rule "no silent provider fallback" | Not found verbatim; the escalation ladder (declared in the packet, executed deterministically) must be reconciled with the written policy (`docs/prd.md:173`, `CONTEXT.md`) |
| gentle-ai `sync` leaving content outside markers untouched | Reported by an explorer, not verified |
| `cursor-agent` end to end | Never run in lucind-ai |

## 6. Task order

T1 HMAC attestation -> T2 packet fields and `interaction_required` -> T3 neutralize SDD gates -> T4 SDD removal (docs, derivation) -> T5 SDD removal (phase command) -> T6 worker and explorer skills -> T7 spike -> T8 `HerdrExecutor` -> T9 dispatcher commit step -> T10 risk classifier and judges -> T11 dispatch-threshold validator -> T12 fan-out and loops -> T13 usage logging -> T14 router with Jev in shadow mode -> T15 rules source and generated files.
Details, estimates, routes and acceptance criteria are in `odd/tasks/herdr-agent-factory.md`.

## 7. How to dispatch a task manually (until HerdrExecutor exists)

This is the recipe used for T1. It is the behavior `HerdrExecutor` (T8) must reproduce.

1. Create the worktree: `git worktree add -b lane/<id> ~/git_root/lucind-ai-worktrees/lane-<id> feature/herdr-agent-factory`, then `gentle-ai codegraph init --cwd <worktree>`.
2. Write the packet as a Markdown file with these headings: `## Skills to load before work`, `## Feature document`, `## Allowed edit surfaces` (exact paths, one per line), `## Objective`, `## Design`, `## Acceptance criteria`, `## Test discipline`, `## Verification`, `## Known environmental failures`, `## Constraints`, `## Return` (ending in a `## Key Learnings` instruction).
3. Create a pane: `herdr pane split --current --direction right --cwd <worktree> --no-focus | jq -r '.result.pane.pane_id'`.
4. Run the worker headless in it with `herdr pane run <pane> "agy --print \"\$(<packet.md)\" --output-format json --mode accept-edits --dangerously-skip-permissions --model gemini-3.8-flash-high --add-dir <worktree> --print-timeout 30m > out.json 2> err.log; echo \$? > exit.code; echo LUCIND_EXIT=\$(<exit.code)"`.
5. Wait with a regex that cannot match the typed command line: `herdr pane wait-output <pane> --regex 'LUCIND_EXIT=[0-9]+' --timeout <ms>`. A literal `--match LUCIND_EXIT=` would match the command echo immediately.
6. Review: read the exit code and `out.json`; check `git diff --name-only` stays inside the allowed surfaces; re-run one reported verification command yourself (spot check); then commit on the lane branch and integrate into the feature branch.

Notes for step 6: the worker leaves changes uncommitted by design; the lane branch is merged or cherry-picked into `feature/herdr-agent-factory` after verification; remove the worktree only after confirming no unique commits remain.

## 8. Resuming in a new session

1. Read this document, then `odd/tasks/herdr-agent-factory.md` for the checklist and per-task progress.
2. Engram: `mem_context`, then `mem_search` for `design/agent-factory-herdr-lucind-ai` (the first three design records are stored under project `gentle-ai`; later records and the ODD mirror under project `lucind-ai`) and `odd/herdr-agent-factory/tasks`.
3. `git status`, `git worktree list` and the branch `lane/*` list show which task was in flight; the Progress section of the feature doc records commit ids per closed task.
4. Check the installed binary matches the source (`lucind-ai -v`; run `make install` after any change touching the binary, see `CLAUDE.md`).
5. Before any work that needs herdr, confirm `HERDR_ENV=1`; use the `herdr` skill rules (parse IDs from JSON, `--no-focus`, never close panes you did not create).
