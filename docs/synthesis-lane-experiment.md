# Synthesis lane experiment

What the fan-out synthesis lane actually spends its wall clock on, and what does
and does not make it faster.

Measured on the `conflict-triage-fixture` change, August 2026.

## The question

The multi-lens SDD fan-out dispatches three `agy` lens lanes over disjoint slices
of a phase, then one synthesis lane that arbitrates them into a canonical
artifact. Lens lanes finish in 2-4 minutes. Synthesis lanes took 17-19.

Profiling five real lanes found no answer: `cursor-agent`, the executor every
synthesis lane ran on, had emitted **zero** tool events across 17115
`lane_progress` rows. The ledger held narrated thinking text and nothing else, so
43-71% of each lane's wall clock sat in silences that could not be attributed to
tool latency or to reasoning.

An earlier experiment had removed 56% of the synthesizer's citation-verification
work and saved no measurable time. Without attribution there was no way to know
why.

## Step 1: make the lane measurable

Only `agy` emitted a full progress vocabulary. Decoders were built for the other
three so one timeline reads consistently across all four executors:

| Executor | Model | Was | Now |
|---|---|---|---|
| `agy` | gemini-3.7-flash-high | full vocabulary | unchanged |
| `claude` | claude-opus-5 | no progress at all | tool lifecycle, usage, non-success result subtype |
| `cursor-agent` | cursor-grok-4.6-high | thinking text only | tool lifecycle with real timestamps |
| `opencode` | openai/gpt-5.6-sol | step boundaries only | tool lifecycle, per-step token usage |

Two CLI-specific traps are worth recording:

- **`claude` refuses `--output-format stream-json` without `--verbose`.** It
  prints an error and exits 1 without dispatching. Same failure family as
  `--print-timeout`, opposite direction: there a flag must never be sent, here one
  must never be omitted.
- **`opencode` never emits a tool-*started* record.** Its print loop forwards a
  `tool_use` line only once `state.status` is already `completed` or `error`. The
  terminal record carries both `state.time.start` and `state.time.end`, so one
  JSON line must be expanded into two events at two different real timestamps.
  Reporting only the terminal event at its arrival time would erase the tool
  latency the decoder exists to measure.

### The defect that hid inside a green test suite

The first `cursor-agent` decoder shipped with passing tests and produced nothing
in production: 4500 progress rows, zero tool events. Two bugs, one root cause.

`variant()` required the `tool_call` object to carry exactly one key. Real
records carry four or five -- the variant key alongside `hookAdditionalContexts`,
`toolCallId`, `startedAtMs`, and on completion `completedAtMs`. Every tool call
the CLI ever emitted was skipped.

Worse, the timestamps live *inside* that object, and a `tool_call` record carries
no top-level `timestamp_ms` at all. Reading them from the top level yielded zero
and fell back to `time.Now()`, so every call would have been stamped at parse
time rather than run time -- silently wrong rather than visibly missing.

The tests passed because their fixtures were hand-written. They encoded the
assumption, not the CLI. **A stream decoder's fixtures must be captured from the
real binary.** The fix was verified by putting real-shaped fixtures in front of
the old decoder first and confirming it reproduced production exactly.

Capture recipe, roughly 30 seconds of quota:

```sh
cursor-agent --print "<prompt>" --output-format stream-json \
  --stream-partial-output --trust --force --approve-mcps \
  --model cursor-grok-4.6-high > raw.jsonl
```

## Step 2: four packet-template changes, trialled on one phase

Trialled together on the `tasks` phase before propagating to the 20 shipped
templates:

1. **Double commit** -- ship the canonical artifact as its own commit before the
   notes file is started, so a lane that dies mid-notes still delivers the artifact.
2. **Citation manifest** -- each lens emits a `## Citation Manifest` the
   synthesizer can verify against, instead of re-deriving every claim.
3. **Mechanical self-check** -- `lucind-lane-check.sh` replaces narrating word
   counts, section presence, and tree cleanliness in prose.
4. **Citation existence verification** -- each lens confirms its own citations
   resolve before committing.

### Result: they cost essentially nothing on the lens side

| Phase | Lens durations (min) | Mean |
|---|---|---|
| spec (none active) | 2.2 / 2.0 / 3.1 | 2.4 |
| design (none active) | 3.5 / 3.7 / 3.1 | 3.4 |
| **tasks (all four active)** | **3.0 / 2.7 / 5.3** | **3.7** |

+0.3 min against design. All three `tasks` drafts passed the mechanical check --
budget, required sections, and every citation resolving. This was the first wave
where lenses self-verified before committing; for contrast, `spec-lens-c` shipped
at 1002 words against a 1000-word budget with nobody noticing.

## Step 3: two synthesizers, same inputs

Both arms read the same three lens drafts and answered the same question. Output
paths were deliberately disjoint so the two lanes would not collide at
integration -- a `required` overlap in our own delivery would have been an
accident, not the fixture's deliberate one.

| Lane | Wall | Progress rows | Tool events | Inside tool calls |
|---|---|---|---|---|
| cursor-grok (broken decoder) | 17.9 min | 4500 | 0 | — |
| opencode/gpt-5.6-luna | 13.0 min | 99 | 60 | 10.4 min (80%) |
| cursor-grok (fixed decoder) | 8.9 min | 2721 | 194 | **0.7 min (7%)** |

## What this settles

**Tool latency is ~7% of the synthesizer's wall clock.** 194 calls totalled 42
seconds. The remaining 93% is model generation. Caching file reads, trimming what
the synthesizer must open, or any other tool-side engineering will not move the
number.

This retroactively explains the failed citation-verification experiment:
verification was never the cost.

**`luna`'s 80%-inside-tools is not tool latency either.** It is one 10.4-minute
`task` call delegating to an `sdd-tasks` sub-agent -- nested model time wearing a
tool's clothes. It also means that arm did not synthesize directly, which
confounds it as a model-versus-model comparison.

So in both synthesizers essentially all wall clock is generation. The only real
levers are **generate less** (tighter budgets, lenses that leave less to compose)
or **use a faster model**. The packet-template work is the right shape of lever.

### What this does not settle

The 17.9 -> 8.9 min drop between the two `cursor-grok` runs is **not** evidence
that anything got faster. A stdout decoder cannot affect model speed. That is
run-to-run variance plus probable prompt-cache warmth on identical lens drafts,
at n=2. No template effect can be claimed from this sample either.

## Which draft won

| Draft | Words | Boxes | Verdict |
|---|---|---|---|
| cursor A | 990 | 13 | Good, but labels regression tests as RED and packs `allowed_paths` into table cells. |
| opencode/luna | 801 | 15 | Finer decomposition, but `./lucind-checks.sh` as every unit's "focused test command" defeats the purpose, and the Executor column holds dispatch shape rather than an executor. |
| **cursor B** | **1024** | **13** | **Promoted.** Enumerates `allowed_paths` per unit, calls out the directory-versus-file prefix trap, keeps RED and GREEN in one unit, and notes the data dependency that blocks Unit 2/Unit 4 parallelism. |

## Open follow-ups

- Propagate the four template changes to the 20 shipped templates under
  `plugin/claude-code/skills/lucind-ai/assets/`.
- `lanes.started_at`, `lanes.ended_at` and `lanes.model` are NULL for every row.
  Lane duration must be derived from `lane_progress`, and the ledger cannot say
  which model ran a lane after the fact -- which is exactly what an A/B needs.
- `Model.GetOverview` shares the empty-runs-table blind spot already fixed in
  `/api/state`.
- `worktree cleanup` deletes the worktree but leaves the lane branch behind.
