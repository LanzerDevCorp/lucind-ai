# Synthesis Notes: Control Room Serve

## Unresolved Contradictions

None

## Coverage Gaps

- `openspec/changes/control-room-serve/specs/` is absent. The packet named accepted specs as a precondition; this worktree has proposal + explore artifacts only. No spec ids were available to reference. Specs were not invented.
- HTTP paths for attempts, leases, and overlap evidence. Proposal capability `control-room-api-features` names Model-backed JSON reads; Lens A says those `serve.Model` methods (`internal/serve/model.go:166-275`) are used; Lens B listed no routes. Recorded as an open question in `design.md`; no routes were invented.
- Skill/packet heading drift (not missing content): `sdd-design` names Data Flow, Interfaces / Contracts, and Migration / Rollout, and an 800-word ceiling. Canonical `design.md` maps those to Flow and Invariants, Interfaces / Contracts, and Rollback and Additivity. The packet's 1800-word ceiling is used, matching archived designs.

## Dropped Citations

- Lens A: `*ledger.Ledger` type at `internal/ledger/ledger.go:191-192`. Those lines are `return &Ledger{db: db}, nil` closing `openAtPath`. The type is `internal/ledger/ledger.go:132-134`. Architecture (pass `*Ledger` into `NewHandler`) kept; this location dropped. `design.md` cites 132-134 and `internal/serve/handlers.go:36`.
- Lens B hop 1: unmatched `/api/*` already returns JSON 404 at `internal/serve/handlers.go:39-77` (and surface row `39-55`). Those lines are the `/` catch-all: static lookup, then stdlib `http.NotFound` at line 53 (plain text). Unmatched `/api/foo` hits this, not a JSON 404. JSON 404 kept as new work; citation dropped as current behavior.
- Lens B hop 3: mutations execute atomic UPDATE + `AppendEvent` at `internal/ledger/ledger.go:448-486,615-640`. `SetStatus` (448-486) does UPDATE + inline event INSERT in one tx, not a call to `AppendEvent`. `Decide` (615-640) is a pending-row UPDATE with no `events` write. Claim dropped; `design.md` states the two behaviors separately.
- Lens B/C “today” / new-seam citations of `Ledger.Runs` at `internal/ledger/ledger.go:284-330` (and C at 285-330), `EventsSince` at 488-525 / 490-525, `IntegrationEventsSince` at 892-928 / 892-925. Those ranges are `Lanes`, `Events` (`WHERE run_id = ?`), and `IntegrationEvents` (`WHERE feature_id = ?`). None is an `id > lastID` cursor or a distinct-run listing. Methods kept as new work adjacent to those functions; existence claims dropped.
- Lens C: `internal/serve/static_test.go:11-103` verifies MIME types, `index.html` fallback, and JSON API 404. Those tests cover absent bulk-approve controls, opencode/evidence copy, unselected items, and evidence validation. MIME/JSON-404 tests kept as required new coverage.
- Lens C: `cmd/lucind-ai/cli_test.go:1908-1930` is a linked-worktree `serve` test (exit 1, no ledger). Those lines are `TestServeNonLoopbackAddrRejectedAtCLI` (`0.0.0.0`), `TestServeFlagsAndSubcommandRecognized`, and the start of `TestDefaultApproverNotEmpty`. No linked-worktree test exists under `cmd/lucind-ai`. Production check at `cmd/lucind-ai/cli.go:702-707` kept; this test citation dropped.
- Lens C: AST audit of `internal/serve/*.go` at `internal/serve/model_test.go:595-628`. `TestModelSourceDoesNotShellOut` reads and parses only `model.go`. The file ends at line 627 (Lens A/B `1-628` overshoots). Claim narrowed to `model.go` at `595-627`.
- Lens C: result envelope unchanged at `.lucind/result.schema.json`. `.lucind/` is gitignored; the tracked schema is `internal/result/result.schema.json`. Claim restated with that path.

## Architecture Divergence

All three assumed the same architecture: extend `internal/serve` and `internal/ledger` (no new packages); freeze `schemaVersion` at 5; pass `*serve.Model` plus `*ledger.Ledger` into `NewHandler`; additive `/api/v1/` GETs and `GET /api/v1/events/stream`; loopback bind; linked-worktree refusal before `ledger.Open`; preserve `GET /api/state` and path-bound POST. Proposal Candidate 1 matches this. Nothing from B or C was dropped for failing Lens A's architecture.

Independent convergence: B and C named `Runs`, `EventsSince`, and `IntegrationEventsSince` for the listing and cursor work Lens A specified without those identifiers. C independently restated omitted `WriteTimeout`, 3s `Shutdown`, and `git revert` additivity.

Compatible elaboration (not divergence): Lens B specified query params (`?run_id=`, `?feature_id=`) and `GET /api/v1/features/{id}`; Lens A left wire shape to the surface lens. Kept.

Lens B hop 1 stating JSON 404 as current behavior was a citation failure (above), not a rival architecture.
