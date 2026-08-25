# Repository safety

Load this module for any repository write, dispatch boundary, or safety decision.

## Boundaries

- Declare one Change, one Orchestrator, one Integration Target, and explicit Write Scopes before work starts.
- A Lane may modify only its packet's `allowed_paths` and declared paths outside the repository. Supply exact revert commands for outside paths.
- Concurrent Lanes must be path-disjoint. `internal/dag.Validate` rejects unordered overlap; add a `depends_on` path and execute sequentially when scopes overlap.
- Run from the primary repository root. The binary refuses dispatch from a linked worktree.
- Put all packet files under `.lucind/packets/`. A tracked packet dirties the primary root and can make integration fail with `integrate: primary root has uncommitted changes`.
- Keep `.lucind/` ignored. It contains packets, ledger state, result schema, and result envelopes. The changed-path union intentionally excludes it.
- Keep canonical Change artifacts under `openspec/changes/<id>/`. Archive must preserve packet bodies and envelopes there before moving the Change directory.

## Dispatch safety

- Confirm the intended base is green before `feature create`, not after. Run `lucind-ai check` on the exact commit you are about to declare as `base_sha` and require it to pass first. Every wave's integration candidate is built starting AT `base_sha` (`internal/run/attempt.go:398` passes it into `CombineTree`, which resolves in `internal/worktree/worktree.go:221`'s `git worktree add -b <branch> <path> <resolvedSHA>`), so a red base poisons every wave's checks regardless of what any Lane did — and once registered, an active feature's `base_sha` is immutable (see `../modes/isolated.md`), so the only way out is `feature disable` and re-create against a corrected base.
- Validate preconditions before dispatch. A precondition that depends on a later packet step means the packet is misordered and must block.
- Preserve sibling execution on one Lane failure; Lanes have independent clocks and do not cancel siblings.
- Treat `allowed_paths` omitted or empty truthfully: current runtime performs neither cross-batch overlap checks nor post-run diff checks for that packet.
- A `read_only: true` Lane must create no unique commit, leave a clean worktree, and prove `HEAD` equals `git merge-base HEAD <primary HEAD>`.
- Every write Lane must commit with a conventional commit, no AI attribution, clean status, and terminal-consumer evidence for every new indirection.
- Never silently change an Execution Route. Executor, model, and agent profile are explicit Orchestrator-owned choices.
- Passing `--approval-timeout` to `lucind-ai run` (or `integrate retry`) is a real gate: it defaults to `0` (no wait / bypass, `cmd/lucind-ai/cli.go:163`), and any positive value parks every Lane in `WaitDecision` until an approval is recorded or the timeout elapses. This is unrelated to `lucind-ai serve`'s own `--approval-timeout`, whose 30-minute default is informational only and does not gate Lanes (`cmd/lucind-ai/cli.go:748`). Never pass a positive `--approval-timeout` to `run`/`integrate retry` without a live `lucind-ai serve --approver ...` process actually recording decisions — otherwise every Done Lane silently blocks until it times out into `Blocked`.
