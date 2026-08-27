# Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

None

## Coverage Gaps

None

## Dropped Citations

None

## Approach Divergence

Lens A's problem formulation and candidate analysis served as primary authority: recommending Candidate 1 (fail-closed worktree cleanup with direct CLI banner anchoring) while rejecting Candidate 2 (automatic stash/commit on teardown with prompt injection due to ref pollution, clean-tree guarantee violations in `internal/run/run.go:906-940`, and unpredictable token bloat) and Candidate 3 (interactive TTY prompting with ledger quarantine due to headless batch incompatibility in `cmd/lucind-ai/cli.go:37-44` and unnecessary ledger schema overhead).

Lens B did not propose competing architectural approaches; its scenarios, impact analysis, and success criteria assumed Candidate 1, focusing on concrete operator loops (dirty vs forced worktree cleanup, TDD WIP rescue on timeout, `integrate retry` recovery without redispatch, qualitative acceptance checklist gates, and multi-wave DAG split reminders).

Lens C independently corroborated Candidate 1 and expanded on technical seams and operational risks: safeguarding automated internal teardown callers (`internal/integrate/integrate.go:121-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`, `cmd/lucind-ai/cli.go:858-869`) against dirty check failures, isolating diagnostic guidance banners from machine-readable stdout streams (`cmd/lucind-ai/cli_test.go:4515-4530`), validating `PorcelainEmpty` behavior against `.gitignore` rules (`internal/worktree/worktree_test.go:536-595`), and warning on multi-wave `base_sha` staleness.

Independent convergence across all three drafts:
1. Fail-closed worktree cleanup requiring `--force` (`-f`) on `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) and checking `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`).
2. Static CLI terminal banners anchoring critical failure and milestone points to authority documentation (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-49`, `references/operations/troubleshooting.md:7-18`, `references/coordination/recovery-reconciliation.md:25-35`, `references/contracts/acceptance-promotion.md:18-30`).
3. Prescriptive operator/orchestrator-driven TDD WIP rescue protocol over automatic commits.
4. Strict out-of-scope boundaries: zero alterations to `.lucind/result.json` schema (`internal/result/result.go:10-45`), no background garbage collection daemons, and no modifications to feature leases or CAS integration mechanics (`internal/feature/feature.go:300-350`, `internal/accept/accept.go:120-130`).
5. Open questions merged cleanly around internal teardown API shapes, banner stream routing (`stderr` vs `stdout`), and exporting `worktree.ErrWorktreeDirty`.
