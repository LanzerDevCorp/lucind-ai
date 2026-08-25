# Synthesis Notes: Native Stability Campaign

## Unresolved Contradictions

None

## Coverage Gaps

None

## Dropped Citations

None

## Architecture Divergence

None — all three converged. All three design lenses independently converged on Candidate 2 (Modular Subpackage Decomposition under `internal/stability/` with subpackages `store`, `fixture`, `process`, `evidence`, and `reconcile`), Linux process-group isolation (`Setpgid: true`) on `internal/executor/agy.go:193-205`, and dedicated common-dir SQLite/WAL authority at `<git-common-dir>/lucind-ai/stability/v1/stability.db` with the primary Run ledger at `<primaryRoot>/.lucind/lucind.db` remaining untouched. Lens A's authoritative resolutions of the four open questions from `proposal.md` Section 9 (optional process-group control via `executor.Request.Setpgid`, dynamic common-dir path resolution via `worktree.GitRunner`, independent stability lifecycle reconciliation in `internal/stability/reconcile`, and full forensic Trial Record emission in `stability status --json`) superseded the restatements of those items as open questions in Lens B and Lens C.
