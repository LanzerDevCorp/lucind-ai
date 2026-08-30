# Acceptance and Promotion

Load this module when judging Lane completion, batch integration, or final Change delivery.

## Lane and batch outcomes

- Each Lane has an independent timeout. One terminal failure does not cancel siblings.
- The barrier releases only after every Lane reaches a terminal state.
- Exit 0 requires every Lane to reach `done` and no ID in `reverted_ids`.
- `integrated_ids` and `reverted_ids` are stdout summary lines, not a separate report format.
- A `done` status does not prove Acceptance: post-execution checks may bisect or revert it. After schema validation, any hard stop with `fired: true` demotes the lane to blocked regardless of the envelope's claimed top-level status.
- Completed, blocked, failed, deviated, and reverted worktrees can be preserved for evidence and recovery. Cleanup may leave the `lucind/<id>` branch.

## Acceptance protocol & checklist

Accept a Lane only after independently confirming packet scope, result schema, done criteria, hard stops, changed paths, commit evidence, terminal consumers, and the applicable checks.

To make acceptance repeatable, execute the canonical 10-step sequence after every lane completes:

1. **Mechanical acceptance automation**: Run `lucind-ai accept --run <run-id> --lane <lane-id>`, using the run and lane identifiers from the dispatch (`lucind-ai run` output / ledger). It loads the frozen done-candidate for that run and lane from the ledger — not the live branch — re-confirms the exact binding (packet digest, base and candidate commit/tree, `allowed_paths`), fails closed if any hard stop fired or a done criterion is unmet, then runs the repository checks (`lucind-checks.sh`) inside a verifier-owned detached worktree at the candidate commit and tears it down. On success it persists an immutable acceptance receipt and prints the receipt id, binding hash, and candidate commit; a missing candidate or failing checks exit nonzero with no receipt and no ref changes. The receipt is mechanical evidence only — never Promotion/CAS and never qualitative approval. Run `lucind-ai accept` with no flags for live usage rather than trusting cached syntax.
2. **Confirm lane tip & base**: Verify `git rev-parse refs/heads/lucind/<id>` against packet `base_sha`.
3. **Verify diffstat & scope**: Confirm actual changes stayed strictly inside declared `allowed_paths`.
4. **Line-by-line diff review (Irreducible)**: Read the full diff (`git diff <base_sha>..<new_tip>`). Summaries are claims, not proof.
5. **Verify result envelope**: Inspect `.lucind/results/<packet-id>.json` — verify `done_criteria` and ensure `hard_stops` have `fired: false`.
6. **Assert genuine test semantics (Irreducible)**: Review changed/added test files to ensure they assert real behavior and error conditions, rather than superficial passes.
7. **Isolated worktree execution**: Always verify in a detached worktree, never the primary checkout.
8. **Full-repo suite pass**: Ensure mechanical checks cover the whole repository, not only the touched package.
9. **Clean worktree teardown**: Remove temporary worktrees after verification (`lucind-ai worktree cleanup --lane <id> [--force]`).
10. **Persist verification memory**: Record acceptance reasoning (`mem_save`) before reporting acceptance to the user or merging.

## Acceptance subagent delegation

To protect the Orchestrator's context window across multi-wave sessions, the Orchestrator may delegate steps 1–9 to an ephemeral Acceptance Subagent:
- Prompt consists strictly of the acceptance checklist.
- Tools are restricted to `Read`, `Grep`, and `Bash` within a scoped worktree.
- Subagent returns structured evidence (diffstat, test semantics, envelope audit, check logs) without inflating the Orchestrator's transcript.

## Dual-Judge acceptance for Tier A Changes

For Tier A Changes (core engine, ledger, security boundaries, and promotion paths):
- Single-model evaluation is insufficient: run two independent qualitative judgments with differing executor/model architectures (e.g. `agy` and `cursor-agent` / `claude`) over the same frozen candidate.
- If judges disagree on compliance or safety, treat disagreements as blockers until independently audited.

## Promotion gate

Promotion is distinct: it is the human-confirmed integration of the completed Change into its declared Integration Target. Verify the target explicitly.

Feature-targeted Promotion uses a fenced attempt, checks, overlap evaluation, and compare-and-swap on `parent_ref`. Exclusive/`legacy_main` Promotion fast-forwards the current checked-out branch and therefore depends on the operator having selected the intended target checkout.

Report accepted IDs, reverted IDs, check evidence, remaining blockers, preserved worktrees, target ref and SHA, and the human Promotion decision.
