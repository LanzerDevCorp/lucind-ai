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
| `feature`, `parent_ref`, `base_sha`, `expected_parent_sha` | Required together for a feature-targeted batch. |
| `legacy_main` | Runtime boolean mapping for Exclusive Mode. |

## Body structure

Include Goal, Why safe now, Preconditions, Allowed paths, allowed outside-repository paths with revert commands, Out of scope, Context with grounded citations, objective Done criteria, and explicit Hard stops.

Mandatory criterion 1: every introduced indirection names and proves a terminal consumer.

*Mandatory criterion 2*: write work is committed conventionally with no AI attribution, `git status --porcelain` empty, and `git log --oneline -1` evidence. For `read_only: true`, replace commit evidence with clean status and `HEAD` equal to `git merge-base HEAD <primary HEAD>`.

Every hard stop must be evaluated in the result whether or not it fired. A fired stop returns `blocked`; the Agent does not guess.

## Result envelope

The packet body must explicitly tell the Agent to write `.lucind/result.json` and validate it against `.lucind/result.schema.json`. The binary never synthesizes this file from executor stdout. A correct commit without the file cannot complete the Lane.

The envelope carries status, summary, done-criterion evidence, hard-stop evaluations, changed paths, commits, and blocker details. Treat it as the Agent's structured claim, not independent proof. Verify cited `file:line`, changed paths, git status, commit, checks, and terminal consumers before Acceptance.

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
