# Synthesis Notes: Control Room UI Shell

## Unresolved Contradictions

None.

## Coverage Gaps

None. All nine proposal-spine items appear in at least one lens draft. No propose draft listed a Dependencies heading; the canonical doc names only seams already cited (`internal/serve`, schema v5, `approvals-web-ui`). No propose draft listed Success Criteria as a heading; the canonical checklist restates verified invariants from A and C plus explore.md, not new product claims.

## Dropped Citations

1. **Lens A: `internal/serve/model.go:95-99` as fence counters.** Those lines are `CASResult` (`Outcome`, `CandidateSHA`, `FailureReason`). Fence lives on `Attempt` (`model.go:44`) and `Lease` (`model.go:55`) and in schema (`internal/ledger/schema.go:114,125`). Fence claim kept with `model.go:44,55` and `schema.go:106-170`; `95-99` dropped.

2. **Lens C: `internal/feature/feature.go:89-135` as CLI lease/git guards.** That span is `Service`, `NewService`, `ValidateParentRef`, and the start of `Create`. `AcquireLease` / `RenewLease` / `ReleaseLease` start at line 283. Citation dropped. Read-only web surface kept via `internal/serve/handlers.go:87-115` (decide and defect under `/approvals/`).

3. **Lens C: `internal/reconcile/reconcile.go:116-168` as reconcile mutations.** That span is `Service` fields and `NewService`. `CreateRequest` is at 213; `Approve` is at 406. Citation dropped.

4. **Lens C: `.lucind/result.schema.json:1-160` as the result-envelope contract.** That path does not exist in this worktree. The authoritative schema is `internal/result/result.schema.json`, embedded by `internal/result/schema.go:1-68`. Envelope-untouched claim kept via `schema.go:1-68`; the `.lucind/` path dropped.

5. **Lens C: `internal/worktree/worktree.go:1-35` as serve's linked-worktree refusal.** Those lines are the package comment and error vars. `IsLinkedWorktree` is at line 278. The serve refusal is `cmd/lucind-ai/cli.go:702-705`. Citation dropped; CLI lifecycle coverage kept via `cli.go:674-725`.

Partial non-drops (claim narrowed, citation kept for the part that matches):

- **Connection health at `handlers.go:130-141` / `index.html:145-148`.** Those seams are approver identity and wrong-approval rate only. Connection/freshness is proposed client chrome, not an existing `ServerState` field. Requirement kept; citation used only for approver/rate.
- **Host-header validation as if it existed.** `serve` has no `Host` check (listen-address `IsLoopback` only). Listed as residual DNS-rebinding risk, not as current code.

## Scope Divergence

Lens A's Candidate 1 (vanilla ES-module SPA, hash router, shared store, read-only REST over `serve.Model`, embed-only, loopback, individual decide) is the canonical approach.

**Lens B** assumed that approach and wrote capability rows plus delta specs against it. It did not re-litigate Candidates 2/3. Divergence: B's routing scenario names `#/features` as a required mount target, while A leaves whether a Features **view** ships in this change as an open question (A still exposes `ListFeatures`/`GetFeature` over HTTP). The Features-view MUST did not enter `proposal.md`; Model GET routes did. B also asked whether views subscribe to store slices or full snapshots — kept as an open question. Process-drift notes (parallel lenses vs monolithic `sdd-propose`) were not copied.

**Lens C** assumed the same vanilla embed SPA and a read-only web surface except existing `/approvals/` writes. It never named A's three candidates; its Node/framework ban independently kills Candidate 3 (same arbitration as explore). Divergence: C listed hash vs History as an open choice; A selected hash for this change and left History as a future catch-all. Canonical doc follows A. C proposed "validate Host header" as a mitigation; that is not in A's approach and is not a delta requirement. C's tab-scoped queries and request deadlines are compatible with A's granular Model GETs and were kept under Risks.

**Independent convergence:** all three treat today's UI as an approvals-only inbox; `serve.Model` as unused by HTTP; `embed.FS` + no npm as non-negotiable; loopback and per-item decide as invariants; polling of `/api/state` as the live transport for *this* change; SSE and a frozen six-view catalog as out of scope or open.
