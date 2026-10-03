# Packet and result contracts

Load this module whenever authoring a packet or judging a Lane result. `../../assets/packet-template.md` is the compatible base asset.

## Frontmatter

Every packet starts with YAML frontmatter and a non-empty prompt body.

| Key | Requirement |
|---|---|
| `id` | Required unique Lane ID; names `lucind/<id>` and its worktree. |
| `executor` | Required supported Execution Route runtime; no fallback. |
| `routed_by` | Required triggering condition, never merely the executor name. |
| `model` | Optional model from that executor's closed allow-list. |
| `agent` | Optional opencode-only primary agent profile. |
| `read_only` | Optional strict boolean; omitted means write. |
| `allowed_paths` | Optional single-line JSON array; YAML lists are invalid. Empty currently disables overlap and post-run scope enforcement for the packet. |
| `read_only_paths` | Apply-DAG JSON array owned by transitive dependencies and forbidden for this node to write. It must not overlap the node's `allowed_paths`. |
| `feature`, `parent_ref`, `base_sha`, `expected_parent_sha` | Omit from reusable templates (no live target SHAs or feature names). At wave dispatch the orchestrator writes all four onto the packet copies it passes to `lucind-ai run`. Required together on a dispatched feature-targeted batch; admission stays fail-closed on unbound or mixed targets. |
| `legacy_main` | Runtime boolean mapping for Exclusive Mode. Dispatching with `--legacy-main` **requires** an expected parent SHA from one of exactly two sources: the batch-wide `--expected-parent-sha <sha>` flag, or this key in every packet's frontmatter. It is an optimistic-concurrency guard on the parent ref, so the binary will not derive it for you — deriving it from `HEAD` would assert the check against itself. Omitting both is refused in pre-dispatch validation (`cmd/lucind-ai/cli.go:211-214`) before any worktree or quota is consumed; the error names only the first packet, but the flag satisfies the whole batch. |
| `route` | Optional execution route tier: `inline`, `worker`, or `fanout`. Empty allowed; any other value is a parse error. |
| `route_evidence` | Optional free-string explanation for the routing decision. |
| `understood` | Optional `true`/`false`: whether the fix is understood. `false` upgrades an `inline` packet to `worker`. |
| `open_design` | Optional `true`/`false`: a design question is still open. `true` upgrades an `inline` packet to `worker`. |
| `estimated_lookups` | Optional integer 0-1000: expected sequential lookups. More than 5 upgrades an `inline` packet to `worker`. |
| `named_skills_only` | Optional strict boolean; when `true`, derives only explicitly named stack and ad-hoc skills plus `lucind-executor` (no lane-role and no `sdd-*` skills). |
| `verification` | Optional single-line JSON array of exact foreground verification command strings to execute. |
| `known_environmental_failures` | Optional single-line JSON array of baseline test failure names or commands that do not block acceptance. |
| `commit_message` | Optional single-line Conventional Commit message string (<= 100 chars, matching `^(feat|fix|docs|refactor|test|chore|perf|build|ci|style|revert)(\([a-z0-9._/-]+\))?!?: \S.*$`). Requires non-empty `verification`. When declared, the worker does NOT commit; after the worker reports done, the dispatcher executes verification under attestation and commits with this message. Forbidden from containing `co-authored-by` or `generated with`. |
| `max_iterations` | Optional integer 1..4 (absent means 1). Number of write/test/fix iterations per rung before escalating or stopping. Values > 1 require non-empty `verification` and a `commit_message` (the dispatcher commits looped work). |
| `escalation` | Optional single-line JSON array of up to 3 rung objects `[{"executor":"<name>","model":"<model or empty>"}]` without unknown keys. Declares the escalation ladder when verification fails. Requires non-empty `verification`. Requires non-empty `verification` and a `commit_message`. |

## Body structure

Include Goal, Why safe now, Preconditions, Allowed paths, allowed outside-repository paths with revert commands, Out of scope, Context with grounded citations, objective Done criteria, and explicit Hard stops.

Mandatory criterion 1: every introduced indirection names and proves a terminal consumer.

*Mandatory criterion 2*: write work is committed conventionally with no AI attribution, `git status --porcelain` empty, and `git log --oneline -1` evidence. For `read_only: true`, replace commit evidence with clean status and `HEAD` equal to `git merge-base HEAD <primary HEAD>`. For packets declaring `commit_message` (obligation: `dispatcher`), the worker must NOT commit (`commit: ""` in envelope, `HEAD` equal to base SHA); the dispatcher executes verification under attestation, verifies valid attestation on the current tree hash, and commits with `commit_message` without AI attribution trailers.

Every hard stop must be evaluated in the result whether or not it fired. A fired stop returns `blocked`; the Agent does not guess.

## Result envelope

The packet body must explicitly tell the Agent to write `.lucind/result.json` and validate it against `.lucind/result.schema.json`. The binary never synthesizes this file from executor stdout. A correct commit without the file cannot complete the Lane.

The envelope carries status, summary, done-criterion evidence, hard-stop evaluations, changed paths, commits, blocker details, and optional interaction requirements. Treat it as the Agent's structured claim, not independent proof. Verify cited `file:line`, changed paths, git status, commit, checks, and terminal consumers before Acceptance.

Valid statuses: `done`, `blocked`, `deviated`, `failed`, and `interaction_required`. When the worker needs a human or orchestrator answer to proceed and has stopped, it reports `status: "interaction_required"` and includes the mandatory top-level `interaction` payload (`question`, `reason`, `unblock_response`, and optional `options`). The `interaction` object is required when status is `interaction_required` and forbidden for all other statuses. In the binary's terminal lane status vocabulary, `interaction_required` maps to `lane.Blocked` with the question captured in the failure reason.

Read-only packets may inspect paths outside their worktree only when those paths are explicitly granted. Ignored packets from the primary checkout are not automatically visible in Lane worktrees.

## Optional shadow authoring

Shadow authoring is observational and opt-in. The specialist receives typed,
target-free authoring facts and may return typed contract data only. The
manual artifact remains the only artifact selected and dispatched, regardless
of validity, semantic equivalence, digest stability, latency, review cost, or
any operator metric. Shadow timeout, invalid JSON/schema, unavailable route,
compiler rejection, and fallback-agent detection are warning-only observations.

Each attempt is compared with the same late target binding and records
field-level normalized differences, validity, semantic equivalence, digest
equality, replay stability, latency, review cost, and failure class. Persistence
is isolated per attempt: a failed evidence transaction must not roll back or
block later attempts. Disabling shadow invocation requires no conversion of
stored manual packets and never enables automatic cutover.
