# Tasks: Delegated Packet Authoring

## Delivery Constraints

Apply MUST use GPT-5.6 Sol Fast, strict RED→GREEN→REFACTOR, `go test ./... -race -count=1`, and `./lucind-checks.sh`; this phase MUST NOT launch apply. Automatic cutover and manual-path removal remain out of scope.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated authored additions + deletions | 2,800–3,400; generated goldens excluded |
| Active budget / risk | 3,000 / High |
| Chained PRs recommended | Yes |
| Delivery strategy / chain strategy | single-pr / pending maintainer-approved `size:exception` |
| Proposed commits | WU1 → WU2 → WU3 → WU4 → WU5 → WU6 |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High
3000-line budget risk: High

## Phase 1: Compiler Contract

- [x] 1.1 **WU1 — `feat(packetauthor): compile deterministic contracts`.** Files/symbols: `internal/packetauthor/{contract,compile,manual,diagnostic}_test.go` then production, `testdata/compatibility/*`; RED grammar bytes, diagnostic order, duplicate fields, replay, target authority/staleness; GREEN `Compile`, `AdmitManual`, `AdmitBatch`, late binding. Command: `go test ./internal/packetauthor -race -count=1`; harness: N/A—pure compiler/fixtures; rollback: new module/fixtures; depends: none; evidence: byte-identical legacy bodies, stable v1 digest, ordered diagnostics.

## Phase 2: Canonical Evidence

- [x] 2.1 **WU2 — `feat(evidence): freeze canonical candidate evidence`.** Files/symbols: RED `internal/candidatechange/*_test.go`, `internal/{result,accept,ledger}/*_test.go`; GREEN `candidatechange.Collect`, `result.FileChange`, `accept.validateResultAndScope`, `LaneCandidate`/`AcceptanceBinding`, `schemaVersion` v9→v10. Command: `go test ./internal/candidatechange ./internal/result ./internal/accept ./internal/ledger -race -count=1`; harness: `t.TempDir` Git+SQLite; rollback: candidatechange and additive v10 columns/tables; depends: WU1; evidence: exact criteria/stops/commit/copy correspondence and legacy/v1 reads. RED threats: relative/absolute/symlink roots; staged, `commit -a`, empty-index, rename, copy; GREEN uses canonical roots and argv-only `git -C` four-way union.

## Phase 3: Dispatch Integration

- [x] 3.1 **WU3 — `feat(dispatch): admit every batch before allocation`.** Files/symbols: RED `cmd/lucind-ai/cli_test.go`, `internal/{packet,run,executor}/*_test.go`; GREEN `runDispatch`, `Packet.ReadOnlyPaths`/`Parse`, `executor.Request`, `run.ExecuteBatch` wiring. Command: `go test ./cmd/lucind-ai ./internal/packet ./internal/run ./internal/executor -race -count=1`; harness: in-process fake deps proving zero worktree/quota and executor-visible inputs; rollback: admission adapter/fields; depends: WU1–2; evidence: mixed batch admission, stale rejection, unchanged body, read-only scope never grants writes.

## Phase 4: Specialist Adapter

- [x] 4.1 **WU4 — `feat(author): add bounded typed specialist`.** Files/symbols: RED `internal/packetauthor/specialist_test.go`, `cmd/lucind-ai/packet_authoring_test.go`; GREEN typed validator/adapter plus `.opencode/agent/lucind-packet-author.md`. Command: `go test ./internal/packetauthor ./cmd/lucind-ai -race -count=1`; harness: fake executor identity, no external account; rollback: specialist adapter/profile; depends: WU1,3; evidence: target/dispatch/unknown fields rejected and only trusted compiler renders.

## Phase 5: Shadow Rollout

- [x] 5.1 **WU5 — `feat(shadow): persist non-authoritative comparisons`.** Files/symbols: RED `internal/packetauthor/compare_test.go`, `internal/ledger/shadow_test.go`, CLI tests; GREEN `Compare`, transactional shadow stores, metrics, disable/fallback warnings; update `packets-results.md` and packet/templates. Command: `go test ./internal/packetauthor ./internal/ledger ./cmd/lucind-ai -race -count=1`; harness: fake timeout/fallback plus SQLite begin/insert/commit failures; rollback: disable flag then shadow code/tables/docs; depends: WU2–4; evidence: isolated rollback, sorted sanitized warnings, manual artifact always selected.

## Phase 6: Integration Proof

- [x] 6.1 **WU6 — `test(packet-authoring): prove end-to-end contracts`.** Files: `cmd/lucind-ai/cli_test.go`, cross-package fixtures/goldens; RED mutation checks for admission/evidence/correspondence, then GREEN/refactor. Command: `go test ./... -race -count=1`; harness: `./lucind-checks.sh`; rollback: integration tests/goldens only; depends: WU1–5; evidence: full suite and harness logs, deterministic goldens rerun without `-update`, no automatic cutover/manual removal.
