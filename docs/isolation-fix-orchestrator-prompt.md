# Follow-up Prompt: Integration Target Isolation Fix

You are the sole Orchestrator for the `integration-target-isolation` Change in the `lucind-ai` repository.

## Product context

Read the repository-root `CONTEXT.md` before making decisions. The relevant agreed guarantees are:

- A Change remains isolated until an explicit integration decision.
- Every Change declares its Integration Target; the current checkout is never an inferred target.
- Isolated Mode is the default.
- One Orchestrator owns this Change.
- Promotion requires the exact declared target and human confirmation.

The product goal is to let multiple Orchestrators run independent Changes concurrently in one local Git repository without candidate contamination.

## Approved execution strategy

Use a **direct strategy** with one writer. Do not create or run an SDD cycle for this Change. The human has already approved this strategy.

Create a dedicated branch and linked worktree for this Change before editing. Keep the worktree under the user's home repository area, preferably a sibling path such as:

```text
/home/lanzerdev/git_root/lucind-ai-worktrees/integration-target-isolation
```

Do not work in the primary checkout and do not use `/tmp`. Initialize a separate CodeGraph index inside the worktree if CodeGraph is needed there.

## Problem to fix

Feature-targeted integration has leases, durable attempts, overlap classification, recovery, and CAS promotion wired into production. However, candidate construction still passes through the ordinary integration combiner, which creates the combined worktree from the mutable primary checkout `HEAD`.

Prior verified flow:

```text
lucind-ai run
  → IntegrateFeature
  → ExecuteAttempt
  → integrate.Combine / CombineTree
  → worktree.Create(primaryRoot, "integrate-" + runID)
```

The combined candidate must instead derive solely from the immutable parent revision recorded for the Change. An unrelated branch currently checked out in the primary workspace must never enter the candidate.

## Required outcome

Implement and prove all of the following:

1. A feature-targeted integration candidate starts from the Change's declared immutable parent revision, not the primary checkout `HEAD`.
2. The lane branches are combined onto that exact parent.
3. Promotion still advances only the declared Integration Target through the existing CAS/lease authority.
4. Two Changes with different parent revisions can construct candidates concurrently without inheriting commits from each other or from an unrelated checked-out branch.
5. Base movement, stale expected-parent state, merge conflict, check failure, and CAS failure preserve the existing safe blocked/recovery behavior.
6. Exclusive/legacy integration behavior is not changed accidentally. If shared combine primitives must change, make the base revision explicit at their boundary and cover both callers.
7. Candidate construction never checks out or mutates the primary workspace.

## Test-first requirements

Use focused Go tests before implementation. At minimum, add behavior-level regression coverage proving:

- the primary checkout points at unrelated Change C;
- Change A declares parent A;
- Change A's combined candidate contains parent A plus A's accepted lane commits;
- the candidate excludes C-only commits;
- concurrent or sequential candidates for A and B remain based on their own parents;
- a stale target still fails through the existing CAS path.

Run focused tests during development, then run the repository's full applicable checks. `go test ./...` must pass. If the Change touches the installed binary, follow `CLAUDE.md` and run `make install`; report the resulting `lucind-ai -v` value.

## Write Scope

Primary allowed scope:

```text
internal/run/**
internal/integrate/**
internal/worktree/**
cmd/lucind-ai/**          # only when the production wiring requires it
```

Relevant Go test files in those areas are included.

Do not modify:

```text
CONTEXT.md
plugin/claude-code/skills/lucind-ai/**
docs/isolation-fix-orchestrator-prompt.md
```

Another Orchestrator owns the concurrent `skill-modularization` Change. If the fix genuinely requires crossing the forbidden scope, stop and request a cross-Change decision rather than editing it.

## Skills to load before work

Resolve the current skill registry and load the exact installed paths for:

- `tdd`
- `go-testing`
- `work-unit-commits`

Use CodeGraph before broad repository exploration. Verify current source rather than trusting the historical symbol locations in this prompt.

## Boundaries

- Do not implement issue #4 in this Change.
- Do not modularize or rewrite the lucind-ai skill.
- Do not add distributed or cross-machine coordination.
- Do not change provider routing, SDD behavior, or Agent Teams integration.
- Do not weaken existing lease, overlap, recovery, check, or CAS guarantees.
- Do not commit, push, or create a PR unless the human explicitly requests it.

## Completion report

Return:

1. Root cause verified against current source.
2. Exact behavioral change.
3. Files changed.
4. Tests added and commands run.
5. Evidence that unrelated primary `HEAD` commits cannot contaminate the candidate.
6. Any residual concurrency or recovery risks.
7. Confirmation that the concurrent skill-modularization Write Scope was untouched.

Save important discoveries and the completed bug fix to Engram under project `lucind-ai` before returning.
