# Prompt for opencode — Control Room SDD master plan

You are planning, not implementing. Produce a master plan document. Write zero
production code.

## Context

Repository: `lucind-ai` (Go, ~37k LOC). Working branch: `dev`. The stable release is
already on `main`; everything on `dev` is disposable and reversible, so bias toward
ambition over caution.

`lucind-ai` is an agent-dispatch orchestrator. It runs "packets" (markdown files with
YAML frontmatter) through executors (`agy`, `cursor-agent`, `opencode`, `human`), each in
its own git worktree, recording everything into a SQLite ledger.

Read these before planning anything. They are ground truth; this prompt is a summary of
them and the files win any disagreement:

1. `docs/control-room-proposal.md` — the approved proposal you are decomposing.
2. `internal/ledger/schema.go` — current schema, at version 5.
3. `internal/serve/` — the surface being replaced (`handlers.go`, `model.go`, `server.go`,
   `static/`).
4. `internal/dag/parse.go` — the `apply-dag.yaml` node schema.
5. `internal/packet/packet.go` — the packet frontmatter schema.
6. `plugin/claude-code/skills/lucind-ai/assets/` — the 24 packet templates.
7. `cmd/lucind-ai/cli.go` — every subcommand and its flags.

## Your task

Decompose the Control Room proposal into **multiple independent SDD features** that can
run their SDD flows **in parallel**, and produce an ultra-detailed master plan that a
separate orchestrator can execute mechanically without making design decisions.

## Machinery constraints — these are hard, verify each against the code

**Packet frontmatter** (`internal/packet`): `id`, `executor`, `routed_by`, `model`,
`agent`, `read_only`, `allowed_paths`, `feature`, `parent_ref`, `base_sha`,
`expected_parent_sha`, `legacy_main`. `routed_by` must state the *condition* that caused
the routing, never the executor's own name. `agent` is only valid for `executor:
opencode`. A `model` must be in that executor's `KnownModels()` or dispatch is rejected
before anything runs.

**Disjointness** (`packet.DisjointAllowedPaths`): every packet inside one
`lucind-ai run` batch must have `allowed_paths` disjoint from every other packet in that
batch. This is checked up front and fails the whole batch. It is the single hardest
constraint on wave design. `internal/ledger/ledger.go` is one 1,400-line file — multiple
packets cannot edit it in the same wave, so new code must land in new files.

**Waves** (`internal/dag/waves.go`, `internal/dag/split.go`): `apply-dag.yaml` declares
packets with `depends_on`; `lucind-ai split` topologically sorts them into waves and
prints one `run --packet ...` line per wave. **There is no automatic wave loop** — the
orchestrator runs each wave and waits for its exit code. Parallelism inside a wave is
real (`internal/run/batch.go` uses goroutines plus a barrier); parallelism across waves
does not exist.

**Fan-out convention**: the 5 planning phases (explore, propose, design, spec, tasks)
each have 3 lens templates (`executor: agy`) plus 1 synthesis template
(`executor: cursor-agent`). This is a template convention, not a CLI feature — the
orchestrator builds N packets from the templates and passes them as N `--packet` flags to
one `lucind-ai run`. `apply` and `verify` and `archive` have single templates.

**Features and integration** (`internal/feature`, `internal/ledger` v4 tables):
`lucind-ai feature create --id --parent --base-sha` registers a feature. Packets naming a
`feature` and `parent_ref` promote their combined tree to that parent branch under a
lease with a fence token. Concurrent features from one clone are supported. **Nothing
auto-merges feature branches into `main`** — plan that step explicitly as human or
sequential work.

**Executor limits** (important for scheduling): `agy` and `cursor-agent` have tight usage
limits. `opencode` is comparatively cheap but should be reserved for apply-DAG work.
Budget the plan accordingly and say where each executor is spent.

## Required deliverables

Write everything under `openspec/plans/control-room/`.

### 1. `MASTER-PLAN.md`

- **Feature decomposition table.** One row per feature: id, one-line goal, the layers of
  the proposal it owns (L1–L5), the exact file paths it may touch, its blocking
  dependencies, and whether its SDD flow can start immediately or must wait.
- **Parallelism matrix.** For every pair of features, state whether their SDD flows can
  run concurrently, and if not, which shared path or artifact forces the ordering. Justify
  each "no" with a concrete file or table name.
- **Execution stages.** Group the features into stages. Stage N contains every feature
  whose SDD flow can run at the same time. For each stage give: the features in it, the
  total concurrent lane count, which executor each lane burns, and the barrier condition
  that must hold before stage N+1 starts.
- **Integration order.** The exact sequence for landing feature branches, including the
  `feature create` invocation per feature (with real `--parent` and `--base-sha` values or
  an explicit note on how to derive them), and the final consolidation into `dev`.
- **Critical path.** Name the longest dependency chain and its length in stages.
- **Risk register.** Carry forward R1–R4 from the proposal, plus any new risk the
  decomposition introduces. Each risk needs a concrete mitigation, not a restatement.

### 2. Per-feature scaffolding

For each feature, a directory `openspec/plans/control-room/<feature-id>/` containing:

- `README.md` — scope, non-scope, the exact `allowed_paths` set, acceptance criteria
  stated as observable behavior, and the definition of done.
- `sdd-flow.md` — the full phase sequence for this feature. For every one of the 5
  planning phases, list the 4 concrete packets (lens-a, lens-b, lens-c, synthesis) with
  their real `id`, `executor`, `routed_by`, `model`, and `allowed_paths` values filled in
  — not the template placeholders. State which phases this feature can skip and why.
- `apply-dag.yaml` — a complete, valid, parseable sidecar for the apply phase, with real
  `id`, `executor`, `routed_by`, `model`, `allowed_paths`, `depends_on`, `body_path`,
  `feature`, and `parent_ref` values. It must satisfy `dag.Validate` and
  `dag.ValidateGlobalOverlap`.
- `packets/` — one stub markdown body per apply-DAG node, each with its real frontmatter
  and a Goal section specific enough that an implementing agent needs no other context.

### 3. `RUNBOOK.md`

The literal command sequence, in order, that executes the entire plan. Every
`lucind-ai feature create`, every `lucind-ai split`, every `lucind-ai run --packet ...`,
every barrier check, every integration step. Annotate each command with what must be true
before it runs and what proves it succeeded. Assume the executing orchestrator has no
memory of this planning session.

## Quality bar

- **Verify, do not assume.** Every flag, field name, table name, and file path you write
  must exist in the code you read. If something is missing, say so explicitly rather than
  inventing a plausible name.
- **Every `allowed_paths` set must be genuinely disjoint** within its wave. Prove it: for
  each wave, show the union and confirm no overlap. This is where plans of this shape
  usually break.
- **Be specific about new files.** Where the proposal says "new code goes in new files to
  keep the wave legal", name each file.
- **No hand-waving on ordering.** "Depends on the schema" is not a dependency; "reads
  `lane_progress.seq`, which migration v6 creates" is.
- **Maximize parallel width** subject to the constraints. If two features could be one,
  say why splitting them is worth the coordination cost — or merge them.
- Prefer more, smaller features over few large ones, as long as their paths stay disjoint.

Write it as if the person executing it will be running unattended overnight and cannot
ask you a single question.
