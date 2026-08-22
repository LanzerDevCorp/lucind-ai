# Spec Synthesis Notes: Control Room Ledger

## Unresolved Contradictions

None.

## Coverage Gaps

- **Admitted Run Status Event Types** has only Lens B's error scenario (unadmitted type rejected). No draft authored a happy-path scenario that `run_status_changed` inserts succeed, or that the six existing types remain admitted. Not invented.
- No draft specified prune trigger (periodic `lucind-ai serve` vs on-demand CLI), whether run status emits via `AppendEvent` or dedicated helpers, or whether progress `message` is a raw string vs structured JSON. Shared open questions, not contradictions.
- gentle-ai `sdd-spec` 650-word budget is superseded by this packet's 1800 authored-word budget. New-capability files use the skill's full-spec shape (`## Requirements`); existing capabilities use `## ADDED Requirements`. Files live under `openspec/changes/control-room-ledger/specs/` (packet forbids writing `openspec/specs/`). Skill Step 5 Engram persistence and Step 6 return block are superseded by the notes file and `.lucind/result.json`.

## Dropped Citations

Every `file:line` below was opened in this worktree. The requirement it was attached to survived only where other verified evidence remained.

1. **`internal/run/run.go:422-434` as progress ingest** (Lens A Progress Ingest consumer; Lens B coverage). Those lines append a completion `lane_note` after `decideStatus`. Mid-flight ingest is new `lane_progress` behavior; this citation is not that writer.
2. **`internal/run/run.go:425-435` as the `run_status_changed` consumer** (Lens A Admitted Run Status Event Types). Same `AppendEvent` of `EventLaneNote`. Admission is the schema `events.type` CHECK at `internal/ledger/schema.go:38-39` (six literals today; v6 must add `run_status_changed`).
3. **`cmd/lucind-ai/cli.go:282-290` as current run registration** (Lens A First-Class Run Persistence). Those lines mint a UUID and call `ledger.Open`; they do not insert a `runs` row. Dispatch site kept; the gloss that registration already happens there is dropped.
4. **`internal/packet/packet.go:43-54` as covering `feature`** (Lens A Lane Dispatch Metadata). `:43-54` is `Model` and `Agent`. `Feature` is at `:64`.
5. **`internal/ledger/schema.go:38-39` as the Lane Dispatch Metadata test seam** (Lens B coverage table). Those lines are the `events.type` CHECK, not `lanes` metadata columns. The "Unadmitted event type rejected" scenario was re-keyed to Admitted Run Status Event Types.

Verified and kept as support (not copied into the delta): live specs `openspec/specs/lane-execution/spec.md:1-62` (3 requirements, 6 scenarios) and `openspec/specs/approvals-web-ui/spec.md:1-83` (4 requirements, 9 scenarios); `RegisterLane` INSERT omitting metadata at `internal/ledger/ledger.go:255-282`; `PruneIntegrationEvents` analog at `internal/ledger/ledger.go:877-890`; shell-free `serve.Model` at `internal/serve/model.go:14-25`; `/api/state` approvals-only at `internal/serve/handlers.go:79-85`; worktree guards at `cmd/lucind-ai/cli.go:277-280` and `:702-705`; `ledgerpath.Resolve` at `internal/ledgerpath/ledgerpath.go:36-38`. `internal/ledger/runs.go` and `progress.go` do not exist yet; both drafts marked them as new.

## Requirement Divergence

Lens A's set is authoritative: seven ADDED requirements across five capabilities, none MODIFIED / REMOVED / RENAMED.

Lens B named six proposal deltas and collapsed **Admitted Run Status Event Types** into **Lane dispatch metadata**, placing "Unadmitted event type rejected" under the metadata requirement. That scenario is in the delta under A's name. B's other names are case/title variants of A's (`First-class run persistence`, `Isolated progress pruning`, `Shell-free run DTOs`, `Primary-root isolation (preserve)`); scenarios joined on those matches.

Lens C independently named both lane-execution additions (`Lane Dispatch Metadata Persistence` and `Admit Run Status Changed Event Type`) and `Shell-Free Run and Progress Model DTOs`. Live-spec inventory found no conflicts; classification stays ADDED. C's assumed `GetRun` / `ListRuns` / `GetLaneProgress` identifiers are not in A's statement; the delta keeps A's unnamed typed methods (same unresolved naming already escalated in `design-synthesis-notes.md`).

Independent convergence: all three agreed the change is additive on `lane-execution` and `approvals-web-ui`, that the three new capabilities are first-class, and that nothing is removed or renamed. A and C independently split event-type admission from lane metadata; B did not.
