# Native Stability Campaigns

Status: accepted

Lucind AI will own stable-release validation through the product command `lucind-ai stability run` rather than an external harness. The command starts one **Stability Campaign** bound to the clean current `HEAD` and exact installed build, then runs three sequential **Stability Trials** using only the `agy` executor with the explicitly pinned `gemini-3.7-flash-high` model. This makes release confidence repeatable, inspectable, and part of the product contract instead of an undocumented operator procedure.

## Considered Options

- **External acceptance harness:** smaller product surface, but release evidence and recovery semantics would live outside the authority being validated.
- **One ordinary Run:** reuses the existing ledger but cannot represent three consecutive trials, cross-Change remediation, candidate-bound certification, or a terminal release receipt without overloading the Run model.
- **Native Stability Campaign:** adds a dedicated lifecycle and storage boundary, but keeps the scenario, recovery, evidence, and verdict under one product-owned contract.

## Consequences

- `lucind-ai stability run` is Linux-only in v1 and requires an interactive terminal. It performs a read-only preflight, requires a clean worktree, verifies that the running binary matches `HEAD`, checks `agy` availability, runs `lucind-ai check`, forecasts 15 model dispatches, and asks for an explicit `yes/no` confirmation with `no` as the default. There is no NIP and no non-interactive bypass.
- A Campaign contains three strictly sequential Trials. Every Trial runs the complete fixture journey from clean operational state; any failure stops the Campaign and resets the consecutive-success count.
- Each Trial uses deterministic embedded fixture packets and real `agy` edits. It coordinates Change A, independent Change B, a separate Fix Change, selective blocking, explicit test-actor approvals, distinct Integration Targets, ancestry isolation, and resumption of A after the fix.
- The canonical crash abruptly terminates B's `agy` process after its Lane result is persisted and before Acceptance. A ten-second native Ownership Lease must expire before an explicitly recorded reclaim dispatches B's replacement. Surviving descendant processes fail the Trial.
- Five `agy` dispatches are allowed per Trial: initial A, initial B, replacement B, Fix, and resumed A. Automatic retries are forbidden. Budgets are ten minutes per dispatch, 45 minutes per Trial, and 135 minutes per Campaign.
- Mutable Campaign and Trial state uses SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/`. Immutable terminal receipts are content-addressed JSON in the same authority. Ordinary Run IDs remain in `<primary-root>/.lucind/lucind.db` and are linked from Trial Records rather than migrated.
- `lucind-ai stability status` is read-only and supports `--json`. `resume` and `abort` are explicit, interactive recovery operations. Ambiguous recovery fails closed; unresolved residue leaves the Campaign in `blocked_cleanup`; `abort` idempotently retries cleanup without redispatching work.
- Evidence is preserved before fixture refs and worktrees are removed. Long-lived records contain bounded sanitized logs and raw-payload hashes, never credentials, environment values, usernames, or absolute paths. Historical records are retained indefinitely in v1.
- A passed receipt binds the candidate commit, Lucind AI build, fixture digest, all three Trial Records, `agy` and model versions, Linux environment, and final verdict. The latest terminal Campaign for a commit determines current certification. Passing does not tag, version, or release the product.
- The Campaign runs `lucind-ai check` before mutation and again after the third Trial and complete cleanup. Deterministic fake-executor tests cover the state machine, but only a real three-Trial `agy` Campaign is acceptance evidence; that Campaign is not part of `go test ./...`.
