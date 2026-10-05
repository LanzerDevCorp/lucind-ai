---
name: lucind
description: Orchestrate Antigravity (agy) from Claude through herdr, with lucind-ai as the deterministic contract for code-changing lanes. Use when delegating implementation, e2e tests, visual validation, exploration or research to agy, or when the user mentions lucind, lanes, dispatch, accept or attest. Requires HERDR_ENV=1.
---

# lucind — Claude orchestrates, agy executes, lucind-ai enforces

Split of work: Claude owns criteria (plan, architecture, when to parallelize, merges, conflicts,
review, model choice). agy does the heavy lifting (implementation, exploration, e2e, visual
checks). `lucind-ai` only enforces what needs no judgment: the result contract, allowed paths,
and test attestation.

Load the `herdr` skill too; this skill assumes its pane/agent commands.

## Choose the path

| Work | Path |
|---|---|
| Read-only: explore, research, fan-out of explorers | **Free agy via herdr** — no lucind-ai |
| Changes code | **Lane**: `lucind-ai dispatch` → `wait` → `accept` |

### Free agy (read-only)

1. `herdr pane split --current --direction right|down --cwd "$PWD" --no-focus` → pane id.
2. `herdr agent start <name> --kind agy --pane <id> -- --dangerously-skip-permissions`.
3. `herdr agent prompt <pane-id> "<task>. Do not edit any file. ..."` — always say *do not edit*;
   nothing enforces it outside a lane.
4. Short answers: `herdr pane read <pane-id> --source recent-unwrapped --lines 120`.
   Long research: ask agy to write Markdown into your scratchpad and reply with the path.
5. Fan-out = repeat per explorer. Close panes you created when done.

### Lane (code changes)

```bash
lucind-ai dispatch --cwd "$PWD" [--allow '<glob>']... --brief <file|-> \
  [--auto-skills] [--check '<cmd>']... [--model M] [--timeout 60m] [--detach] [--min-quota 0.1]
lucind-ai wait <lane>            # only after --detach
lucind-ai accept --lane <id>     # run from the lane's repo
```

- `dispatch` creates `.lucind/lanes/<id>/`, records the base tree, opens a new pane with
  `LUCIND_LANE=<id>`, starts agy and sends it the brief plus a contract footer.
- Blocks until agy's Stop hook marks the lane `done`/`failed`; prints one JSON object
  `{lane, pane_id, cwd, status, result_path}` (`result_path` printed by `dispatch` and `wait` is
  the current turn's result file, `.lucind/lanes/<id>/result-<turn>.json`). Exit codes: 0 done,
  1 error, 3 failed, 4 timeout.
- The pane is **never** closed or killed by lucind-ai. Talk to agy freely with
  `herdr agent prompt <pane_id> ...`; the contract is checked once, at `accept`, on the final tree.
  For a formal follow-up turn with wait + validation: `dispatch --lane <id> --brief ...`. For
  follow-up turns `dispatch --lane <id> --brief ...`, `--allow` is optional: when omitted, the
  lane's stored globs apply; when given, they replace the stored globs.
- On `timeout`: inspect the pane, nudge, or close it yourself. lucind-ai does nothing more.
- `accept` writes `receipt.json` and accepts only when: the result envelope is valid; every file
  changed since the base tree matches an `--allow` glob; and each check is verified.
  `--check '<cmd>'` is optional and repeatable: the orchestrator picks the checks that fit the
  change (a lint, one test type, e2e, or the whole suite); with none, `accept` requires no
  attestation. `accept` reuses a valid attestation per check on the final tree and runs only
  the missing ones.
  **Never re-run tests yourself after an accepted receipt** — the attestation is the proof.
- **Review before `accept`.** `status: done` only means agy wrote a valid envelope, not that the
  brief was fully delivered. Compare `git status` and `git diff --stat` and the envelope's
  `done_criteria` with the brief's scope items; if something is missing, send a follow-up turn to
  the same lane (`dispatch --lane <id> --brief ...`, including the full original brief) instead of
  accepting. Results use per-turn files (`result-<turn>.json`), and only the current turn's result
  file counts (the Stop hook, `wait`, and `accept` ignore valid results from other turns). Note the
  known limit: a Stop from a previous turn that is still finishing cannot be told apart from a
  current one, and can spend a retry and inject the "write the envelope" nudge into the new turn
  (the retry budget resets after 60 s without a counted stop).
- **After `accept`, decide what to do with the pane** (lucind-ai never closes it). Default: close
  it with `herdr pane close <pane_id>`. If reviewing the diff left a doubt about the
  implementation, leave it open and ask agy with `herdr agent prompt <pane_id> "..."`, reusing the
  implementer's fresh context; close it once the question is settled.

## Pane layout (preferred)

Keep your own pane big on the left and put every agy pane in one column on the right, stacked.
This is the user's preferred layout; use it for lanes, probes and read-only agy panes alike.

```
┌──────────────────────────┬────────────┐
│                          │  lane 1    │
│   You (orchestrator)     ├────────────┤
│   about 70% of the width │  lane 2    │
│                          ├────────────┤
│                          │  lane 3    │
└──────────────────────────┴────────────┘
```

How to get it:

1. **First lane:** run `lucind-ai dispatch ...` from your pane as usual. It splits the calling pane
   and goes **right** when the pane is at least twice as wide as it is tall (a full-width pane is).
2. **Every further lane:** run the dispatch with `HERDR_PANE_ID` set to the **previous lane's pane**,
   for example `HERDR_PANE_ID=w1:p1E lucind-ai dispatch ... --detach`. herdr resolves `--current`
   from that variable, so the new pane is split from the lane column. A narrow pane (width under
   twice its height) splits **down**, which stacks the lanes. Without this, dispatch would split
   your own pane downward and shrink it.
3. **Make yours bigger:** once the panes exist, run
   `herdr pane resize --direction right --amount 0.2 --pane "$HERDR_PANE_ID"` (on a 120-column area
   the split goes from 60/60 to 84/36). Check it with `herdr pane layout --pane "$HERDR_PANE_ID"`.
4. **Clean up:** close each lane pane after its `accept` (see above); the column reflows by itself.

Parallel lanes need `--detach`, and parallel writers still need one worktree each. With three or
more stacked panes the heights get small; prefer fewer parallel lanes.

## Writing a brief

Free Markdown, not validated by lucind-ai. Include: goal, scope, acceptance criteria, constraints
(TDD, style), and what to report in the result envelope. The contract footer added by `dispatch` already
covers lane id, allowed globs, result path, and the final verification commands (each
`lucind-ai attest run -- sh -c '<check>'`, or noting none when zero checks).
Keep `--allow` as narrow as the task: it is enforced by agy's PreToolUse hook and again by `accept`.

### Brief sections

Each one is its own Markdown heading. A section that ends at the next heading must contain only
what is described here (no explanatory prose), so put prose under a following heading.

- `## Allowed edit surfaces`: the same set as `--allow`. Exact repo-relative paths or narrow
  globs, one per line; never `.`, a bare repository root, or an absolute path; paths containing
  whitespace go in whole-entry backticks. List pre-existing untracked targets agy may write and
  the directories where new files are authorized. Nothing beyond the task: a surface wider than
  the task is the same defect as no surface at all.
- `## Skills to load before work`: one exact `SKILL.md` path per line, absolute. With
  `--auto-skills`, the orchestrator omits the section and does not read the skill registry:
  lucind-ai asks Jev using `.atl/skill-registry.md` and `TYPESAFE_API_KEY`, records
  `skills-<turn>.json`, and a hand-written section always wins. Without the flag, resolve them
  yourself and pass the real file path (resolve symlinks; for example
  `.agents/skills/<name>/SKILL.md`), since lane workers are not Claude; agy reads those files
  before touching code and does not rediscover skills. Omit when no skill applies.
- `## Hard stops`: one line per condition that must stop agy. The envelope requires one
  `hard_stops` entry per hard stop in the brief (`[]` when none), so list them here.
- `## Verification`: the exact commands agy must run, each reported as
  `<command>: <observed result>` in `done_criteria[].evidence`. The final verification
  command(s) from the contract footer (`lucind-ai attest run -- sh -c '<check>'`, if any)
  still apply.
- `## Known environmental failures` (optional): exact test names or command lines already
  failing on the base. Any other failing required command means the lane is not `done`.
- `## Test-first policy`: when a relevant runnable deterministic test and a clear expected outcome
  exist, observe RED before implementing, then GREEN, then refactor while tests stay green. Tests
  or frameworks being present alone do not establish applicability. For passive documentation,
  unavailable runners, or no meaningful runnable RED, state the exception and run proportionate
  functional or structural checks. Name the applicable runner and the evidence or exception; never
  invent RED/GREEN evidence or a runner.
- `## Feature document`: the repo-relative locator `odd/tasks/<feature-name>.md`. Read the actual
  file (and reconcile it with its Engram mirror `odd/<feature-name>/tasks`) before delegating, pass
  the relevant context in the brief, and tell agy to read the document before editing. Omit for
  small work with no feature document.
- `## Language contract`: generated technical artifacts (code, comments, tests, fixtures, UI
  copy, docs) default to English regardless of conversation language. If another language is
  explicitly requested for an artifact, use a neutral/professional register unless a specific tone
  or regional variant is requested.
- `## Size heuristic`: about 400 authored changed lines per task (additions plus deletions) is a
  planning heuristic only, not an acceptance criterion, hard cap, or automatic stop. If the
  correct, clear solution naturally exceeds it, say why and continue. Never delete spaces, blank
  lines or comments to save lines, omit tests, minify, add gratuitous abstractions, or split
  artificially to fit it.
- `## Remote scope`: only when remote work is authorized. State the exact destination, operation
  and credential/session; delegation cannot expand it. Omit otherwise, and agy stays local.
- `## Key Learnings` as the closing instruction: after writing its normal result envelope, agy
  closes its final response text with a `## Key Learnings` block of 1-5 numbered items, each a
  standalone factual sentence of at least 20 characters and at least 4 words, omitting the block
  when there is genuinely no reusable learning.

Example of the skills section:

```markdown
## Skills to load before work
/abs/path/to/skill-a/SKILL.md
/abs/path/to/skill-b/SKILL.md
```

## Parallelism and worktrees

- Default: one lane in the main checkout, on a branch you create. lucind-ai never creates
  branches or worktrees.
- Parallel writers: one writer per tree. Create worktrees yourself
  (`git worktree add <repo-parent>/<repo>-worktrees/<name> -b <branch>`), dispatch one lane per
  worktree (`--cwd` = worktree), then merge with plain git and resolve conflicts yourself.
- Read-only agents can share any tree.

## Models

`--model` > `LUCIND_AGY_MODEL` > agy default. Fast/cheap model for exploration and validation;
strong model for implementation and e2e.

## Quota

`--min-quota F` refuses to dispatch below a remaining 5h fraction. When agy runs out, switch
accounts with `agy-pool use <email>` (`agy-pool usage --refresh`, `agy-pool best`) and start a
fresh agy session; an open session does not pick up new credentials reliably.

## Setup

`lucind-ai install` installs the Claude skill into `~/.claude/skills/lucind`, the `lucind` agy plugin, and the `lucind-roles` agy plugin. `make install` builds the binary and runs `lucind-ai install`. Check the build with `lucind-ai -v` before dispatching.
