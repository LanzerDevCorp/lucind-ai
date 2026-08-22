# Spec Synthesis Notes: Control Room Serve

## Unresolved Contradictions

None

## Coverage Gaps

- Format drift versus `~/.claude/skills/sdd-spec/SKILL.md`: that skill's "For NEW Specs" template uses `# {Domain} Specification` / `## Purpose` / `## Requirements`. This packet required the skill's delta format at `openspec/changes/control-room-serve/specs/<capability>/spec.md`. New capabilities use `## ADDED Requirements`. Archive-time copy-full-then-edit applies only to `Loopback Binding`.
- Lens B omitted under budget and no draft invented replacements: error states for Run Listing, Reconciliation Candidates, Approval Queue, and SSE; edge cases for Lane Listing (unknown `run_id`), Feature Listing (empty list), Individual Decisions, and Static Assets (unknown non-API path). Happy path plus one empty-or-error scenario remains for every shipped requirement.
- No draft specified that `GET /api/state` stays unchanged. The proposal keeps it; the live spec never named it. Omission leaves live behavior unspecified rather than deleted.
- No draft specified whether `GET /api/v1/reconciliations` is unfiltered or requires `feature_id`. `ListReconciliationRequests` is per-feature (`internal/serve/model.go:277-292`); `design.md` already uses `?feature_id=`. A's statement and B's scenarios name the unfiltered path. Not invented here.
- Open questions not turned into requirements: SSE cursor vs IPC (design already chose SQLite `id > lastID`); optional `--dev-static-dir`; HTTP reconcile mutations beyond `POST /approvals/{runID}/{laneID}` (proposal out of scope).
- A's SSE statement names `http.Flusher`. Spine item 8 forbids HOW; A's statement is authoritative, so it shipped. Tasks should treat Flusher as the stdlib flush seam, not a second protocol.

## Dropped Citations

None

## Requirement Divergence

All three independently named the same five capabilities (`control-room-api-runs`, `control-room-api-features`, `control-room-api-reconcile`, `control-room-events-stream`, `approvals-web-ui`) and the same Loopback Binding modification. B's other seven requirement names match A's ADDED set. Independent convergence.

Lens B also specified **Individual Decisions Without Bulk Approval** (single POST recorded; bulk array/composite 400) with coverage at `internal/serve/handlers.go:87-115,148-211` and `internal/serve/server_test.go:42-135`. Lens A did not name it. Those two scenarios are not in the delta.

Lens C copied that live block under MODIFIED Full Blocks but Conflicts said None — preserved, not changed. No ADDED→MODIFIED correction. Live `Individual Decisions Without Bulk Approval`, `Inline Evidence and Batch Review Command`, and `Approver Wrong-Approval Rate` stay as they are. C recorded no removals or renames.
