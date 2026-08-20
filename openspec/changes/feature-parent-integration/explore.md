## Exploration: feature-parent-integration

### Current State
`runDispatch` resolves one primary repository root and opens its single SQLite ledger. `worktree.Create` invokes `git worktree add -b` without a start revision, so lane and integration branches inherit the primary checkout's `HEAD`. `integrate.Combine` starts the temporary combined worktree the same way; after checks and bisection, `Promote` fast-forwards whichever branch is checked out in `primaryRoot`. Therefore a normal `main` checkout is an implicit and unsafe integration target for concurrent product features.

The ledger is schema v2 with lanes and append-only events; it has no feature, parent revision, integration attempt, lock, or approval entity. The planned approvals-web-ui change has not been implemented: its design proposes a schema-v3, lane-keyed approvals table and `internal/serve`, but neither exists in current code.

The resolver is bounded but not a semantic authority. It runs `claude -p --model sonnet` (not a Sonnet 5 pin), limits conflict-marker blocks to 400 lines, allows five minutes, stages only paths Git marks unmerged, and creates the merge commit. Unit tests exercise a fake invoker, size bound, partial failure, prompt, staging, and commit behavior; no real Claude invocation or end-to-end parent-feature reconciliation is proven.

### Affected Areas
- `internal/packet/packet.go` and `internal/dag/*.go` — require an explicit feature/parent target and expected parent revision in every dispatchable unit.
- `internal/worktree/worktree.go` — must create lane and combine worktrees from an explicit commit, with feature-scoped names that cannot collide.
- `internal/run/run.go`, `internal/run/batch.go`, and `internal/run/integrate.go` — must group a batch by one parent feature, serialize parent advancement, retain stale/retry evidence, and promote only that parent.
- `internal/integrate/integrate.go` — must combine from the expected parent revision and use a compare-and-swap update of the named parent ref rather than the primary checkout.
- `internal/ledger/schema.go` and `internal/ledger/ledger.go` — need additive durable records for parent features, integration attempts/expected revisions, locks or leases, overlap evidence, and reconciliation approvals.
- `internal/packet/disjoint.go` — current allowed-path disjointness is useful admission evidence but cannot be the cross-feature overlap policy.
- `internal/resolve/resolve.go` — may be reused only after a human approves a source-to-target direction and an explicitly bounded candidate is created.
- Planned `internal/serve` / approvals schema — need a new reconciliation subject or a generalized approval model; the planned lane-only key cannot represent a cross-feature decision safely.

### Approaches
1. **Explicit parent ref with feature-scoped CAS promotion** — Store a durable parent branch and expected immutable tip for each feature. Create every lane and combine worktree at that tip. Under a per-feature lease, verify the parent still equals the expected tip, then atomically update only `refs/heads/<parent>` from expected to verified integration tip (`git update-ref <ref> <new> <expected>`). The primary checkout is never switched or merged.
   - Pros: explicit target; concurrent unrelated parents advance independently; CAS detects stale tips; deterministic retries can reuse the same attempt identity; no mutation of `main`.
   - Cons: requires a new feature/attempt state model, ref validation, recovery, and tests around Git ref races.
   - Effort: High.

2. **Dedicated checked-out parent worktree per feature** — Give every parent branch a durable linked worktree and run `git merge --ff-only` in that worktree under a feature lock. Lanes and combination trees start from that parent's captured tip.
   - Pros: resembles the existing `Promote` flow and makes the target visible on disk.
   - Cons: a checkout is an unnecessary mutable coordination surface; interruption leaves more worktrees; lock correctness still needs expected-tip validation; it is easier to accidentally reuse the wrong worktree.
   - Effort: Medium-High.

### Recommendation
Adopt **explicit parent refs with feature-scoped CAS promotion**, optionally retaining an inspect-only parent worktree later. Treat the feature branch name and expected SHA as required packet/run data, never infer either from `primaryRoot`. One feature lease covers combine, checks, bisection, candidate resolution, and CAS promotion; different feature leases may run concurrently. An interrupted lease expires only after recovery verifies its recorded expected/current refs and preserves lane/combine worktrees for diagnosis. Replaying the same integration-attempt ID must either report the prior terminal result or safely retry from recorded inputs, never produce a second promotion.

Use layered, deterministic overlap evidence before reconciliation:

1. Require a common merge base and compare each active parent range from that base using rename-aware Git name/status and numstat output. Report changed-path intersection, change-size ratios, and whether either range is empty.
2. For shared text files, compare zero-context diff hunk intervals and insertion anchors. Same file with disjoint hunks is weaker evidence than intersecting or adjacent hunks; binary, rename/delete, generated, and mode-only changes are separately labelled rather than silently scored.
3. Weight repeated paths across active parent ranges as deterministic hotspots, normalized by each feature's changed-file and changed-line totals. This avoids treating one shared generated file like a small feature's entire change.
4. Optionally add structural evidence from CodeGraph (same exported symbol, call path, schema, or package boundary), clearly marked best-effort. It improves semantic recall but must not be the sole gate because generated code, dynamic behavior, and stale/unavailable indexes create false negatives.

The UI must show evidence, not a single score: base/tip SHAs, path/rename status, hunk excerpts and ratios, hotspot history, optional structural matches, check results, and the candidate diff. Shared files can be false positives (disjoint edits, formatting, lockfiles, generated output); distinct files can be false negatives (shared schema/API or runtime contract). Policy should therefore classify evidence as informational, warning, or reconciliation-required rather than claiming conflict prediction.

For reconciliation-required overlap, create a durable record with states `detected`, `awaiting_direction`, `approved`, `declined`, `expired`, `candidate_running`, `candidate_ready`, `integrated`, and `failed`. The localhost UI must permit exactly one per-record user decision: approve `source -> target`, decline/defer, or explicitly expire/cancel; it must not offer approve-all, infer direction, or authorize bidirectional merging. A pending or expired decision blocks the affected promotion; whether it also blocks new lane dispatch remains a product decision. Record actor, timestamps, evidence snapshot/version, selected direction, timeout outcome, candidate SHA, resolver output reference, checks, and final CAS result in the ledger audit trail.

After an approved direction, the Sonnet resolver may repair only the bounded candidate in the selected target context. It must receive the approved source/target and allowed conflicted paths, cannot alter direction, and cannot claim business-semantic resolution without checks and human-visible evidence. Failed checks, a changed expected parent, unresolved markers, timeout, or an over-bound conflict return the record to a blocked/reviewable state; they never auto-promote.

### Risks
- Parent names, revisions, and feature identity added only to packets but not the ledger would make recovery and auditing non-deterministic.
- A process-local mutex alone fails across processes; use durable feature leases plus ref CAS and explicit stale-lease recovery.
- Blocking all execution on a warning may waste capacity; allowing promotion through reconciliation-required overlap defeats the safety boundary. Policy scope needs product approval.
- Existing approvals-web-ui design is lane-specific and unimplemented; coupling reconciliation to it without a generalized approval subject would create an invalid audit model.
- Git/hunk evidence predicts textual overlap, not product semantics; optional structural evidence reduces but cannot eliminate false negatives.
- Resolver unit coverage does not prove real Sonnet execution, cross-feature direction handling, or end-to-end recovery.

### Ready for Proposal
Yes, after the user answers these product questions:
1. Does a reconciliation-required overlap block only promotion (recommended) or also admission/dispatch of new lanes for either feature?
2. What policy thresholds/classification promote deterministic overlap evidence from warning to reconciliation-required, and may a user override a warning without a direction decision?
3. Who may approve direction in the localhost UI, what identity is authoritative, and what is the timeout behavior: remain blocked, expire/cancel, or require an explicit renewed request?
4. Is the approval model generalized for lane and reconciliation subjects, or is reconciliation a separate audited entity linked to the planned lane approvals table?
5. May the resolver create a candidate automatically after direction approval, or must the user approve candidate creation separately; what exact checks are mandatory before its CAS promotion?
6. What is the parent-branch lifecycle: who creates/closes it, may it be rebased, and how is its eventual human-directed merge into `main` performed outside `lucind-ai`?
