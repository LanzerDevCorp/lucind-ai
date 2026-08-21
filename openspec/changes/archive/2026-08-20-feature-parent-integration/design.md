# Design: Feature Parent Integration

## Technical Approach

Make feature identity, `refs/heads/*` parent, immutable base SHA, and expected parent SHA explicit at admission. `internal/run` coordinates durable attempts; Git plumbing creates worktrees and performs CAS without checking out or mutating the primary tree or `main`. SQLite is the authority for leases, evidence, approvals, recovery, and audit; Git refs remain the promotion authority.

## Architecture Decisions

| Choice | Rejected alternative / rationale |
|---|---|
| Named-ref `git update-ref <ref> <candidate> <expected>` CAS | Checked-out-parent merge adds mutable state and cannot safely detect races. |
| Per-feature expiring lease with monotonic fencing token | Process mutex cannot coordinate multiple processes; token rejects expired owners. |
| Separate reconciliation aggregate linked to future lane approvals | A lane-keyed approval cannot represent cross-feature direction; no UI is assumed. |
| Deterministic Git evidence plus optional CodeGraph evidence | Opaque/structural-only scoring is irreproducible and can miss dynamic or stale relationships. |

## Domain, Flow, and Invariants

`Feature: created → active → disabled`; closure/delivery is external. `Attempt: recorded → leased → combining → checking → cas_pending → promoted`; any nonterminal state may become `blocked|failed|stale`, and terminal replay returns the stored result. `Overlap: current → superseded|resolved`. `Request: awaiting → approved|declined|cancelled|expired`; approval creates exactly one `candidate_running → integrated|failed|stale` candidate. IDs are caller UUIDs; unique `(feature_id,idempotency_key)` and one candidate per request prevent duplication. Required overlap gates both parents' promotion, never dispatch. Source is immutable; approvals bind direction, evidence version/hash, source/target SHAs, actor (`local:<OS-user>`), and expiry.

Lease acquisition/renewal uses short SQLite write transactions: conditionally replace an absent/expired row and increment `fence`; every attempt mutation requires `(owner,fence,unexpired)`. No transaction spans Git or checks. Recovery compares refs: expected means resume; candidate means CAS succeeded and ledger finalization is replayed; any other/missing ref blocks and preserves artifacts. Checked-out managed refs, deleted refs, external rewrites, non-commit refs, multiple/no merge bases, stale SHAs, and lease loss fail closed. Cleanup occurs only after committed promotion; all failure/stale candidates and worktrees remain inspectable.

## Git and Overlap Contracts

Validate parent with `git check-ref-format --branch`, canonicalize to `refs/heads/`, reject `main` and Lucind temporary namespaces, resolve `<sha>^{commit}`, verify ancestry, and inspect `git worktree list --porcelain`; never derive a target from `HEAD`. Create lane/combine branches using `git worktree add -b <feature-scoped-name> <path> <recorded-sha>`.

Evaluation runs after feature creation/parent promotion and before either promotion gate. From the unique merge base, capture `git diff --find-renames --name-status -z`, `--numstat -z`, zero-context hunks, and `git merge-tree --write-tree`; normalize sorted paths and label rename/delete, binary, mode-only, generated, symlink/submodule, and executable changes. Record line/path ratios and hotspot weight (shared changed lines divided by each feature total). Defaults are configurable: required for predicted conflict, rename/delete collision, shared binary, intersecting/≤3-line hunks, or hotspot ≥0.50; warning for shared disjoint paths/hotspot ≥0.20; otherwise informational. Evidence records every signal and threshold. CodeGraph symbol/call/schema matches are best-effort, versioned, and never independently escalate.

## Persistence, Interfaces, and CLI

Schema **v4** adds `features`, `integration_attempts`, `feature_leases`, `overlap_evidence`, `reconciliation_requests`, `reconciliation_candidates`, and `integration_events`; columns cover IDs/states, refs/SHAs, idempotency key, owner/fence/expiry, evidence JSON/version/hash/class, direction/actor/expiry, allowed paths/model/config/output/checks/candidate SHA, timestamps, and failures. State plus audit append occurs atomically; migration is additive and rollback disables commands while retaining v4 data. (`approvals-web-ui` already shipped schema v3's `approvals` table; this change targets the next version, v4, not a re-use of v3.)

Commands: `feature create --id --parent --base-sha --expected-parent-sha`; packets/DAG add `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`; `run`, `feature status`, `feature recover --attempt`; `reconcile approve --request --source --target|decline|cancel`, and `renew`. Legacy packets fail unless `--legacy-main --expected-parent-sha`; this declares `main` only for dispatch compatibility, never promotion. Future `internal/serve` consumes application-service DTOs/list/get/decide methods, not shell commands.

## File Changes and Test Seams

Modify `cmd/lucind-ai/cli.go`, `internal/{packet,dag,worktree,run,integrate,ledger,resolve}/*`; create `internal/feature`, `internal/overlap`, `internal/reconcile`, and future-facing `internal/serve/model.go`. Inject `GitRunner`, ledger store, clock, ID source, resolver, overlap provider, and lease hooks. Resolver receives approved direction/allowed paths, records `sonnet`, config, 400 conflict-line bound and five-minute timeout, detects out-of-scope edits/markers/semantic ambiguity, runs mandatory `lucind-checks.sh`, then CASes only when refs, fence, evidence, and checks remain valid.

Tests are table-driven unit state/policy tests; `t.TempDir()` Git/SQLite integration tests; multi-handle/process lease races; fault injection after combine/check/CAS/ledger commit; CLI/legacy/API DTO tests; and an optional manual subscription-dependent real Sonnet end-to-end probe.

## Threat Matrix

| Boundary | Applicability; safe/failure behavior; planned RED test |
|---|---|
| Documentation-like paths | N/A: no executable-file classification changes. |
| Git repository selection | Applicable: fixed absolute primary root and explicit `git -C`/`Cmd.Dir`; reject relative/foreign roots; RED relative, absolute-foreign, linked-worktree cases. |
| Commit state | Applicable: plumbing ignores primary index and candidate stages only approved paths; reject out-of-scope/staged leakage; RED staged, `commit -a` leakage, empty-index cases. |
| Push state | N/A: no push operation. |
| PR commands | N/A: review/delivery remain external. |

## Open Questions

None.
