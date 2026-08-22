# Spec Lens A — Capabilities & Requirements: Control Room UI Views

## Assumed requirements

This change specifies five capabilities comprising four new full specifications and one existing delta specification. The four new capabilities—`batch-wave-view`, `feature-lease-monitor`, `reconciliation-workspace`, and `lane-envelope-inspector`—each introduce one ADDED requirement targeting full specs at `openspec/specs/<capability>/spec.md`. The existing capability `approvals-web-ui` introduces one MODIFIED requirement targeting a delta spec at `openspec/changes/control-room-ui-views/specs/approvals-web-ui/spec.md` to preserve anti-rubber-stamping invariants within the modular multi-panel dashboard. In total, the requirement set contains five observable requirements: four ADDED and one MODIFIED.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `approvals-web-ui` | Existing | `openspec/changes/control-room-ui-views/specs/approvals-web-ui/spec.md` | `openspec/specs/approvals-web-ui/spec.md:26-83` |
| `batch-wave-view` | New | `openspec/specs/batch-wave-view/spec.md` | |
| `feature-lease-monitor` | New | `openspec/specs/feature-lease-monitor/spec.md` | |
| `reconciliation-workspace` | New | `openspec/specs/reconciliation-workspace/spec.md` | |
| `lane-envelope-inspector` | New | `openspec/specs/lane-envelope-inspector/spec.md` | |

## ADDED Requirements

### Requirement: Batch and DAG Wave Inspection

The dashboard UI MUST display active batch execution status, DAG wave grouping, per-lane lifecycle status (`pending`, `running`, `done`, `blocked`, `deviated`, `failed`), assigned executor, worktree directory path, per-lane execution deadline, and barrier release state (`Released`, integration eligibility for completed lanes, and preservation for non-done worktrees).

**Terminal consumer**: `internal/serve/handlers.go:36-118` (via `GET /api/batch/lanes`), `internal/serve/model.go:26-125` (via `ListBatchLanes`), and `internal/serve/static/app.js:22-70`.

### Requirement: Shell-Free Feature and Lease Monitoring

The UI MUST display feature lifecycle status, active lease owner, monotonic lease fence counter, latest integration attempt status with candidate commit SHA, and classified overlap evidence payloads exclusively through `serve.Model` queries without executing shell or git subprocesses.

**Terminal consumer**: `internal/serve/handlers.go:36-118` (via `GET /api/features`, `GET /api/features/{id}/attempts`, `GET /api/leases`, `GET /api/overlap/{feature_id}`), `internal/serve/model.go:27-70, 128-266`, and `internal/serve/static/app.js:22-70`.

### Requirement: Reconciliation Candidate Inspection

The reconciliation workspace UI MUST display pending reconciliation requests, resolution candidate records, automated check outcomes, CAS promotion evaluation results, and immutable audit event timelines.

**Terminal consumer**: `internal/serve/handlers.go:36-118` (via `GET /api/reconcile/requests`), `internal/serve/model.go:74-125, 278-343`, and `internal/serve/static/app.js:22-70`.

### Requirement: Lane Demotion Diagnosis

When a lane execution is demoted from Done to Deviated due to modifying paths outside declared `allowed_paths`, the UI MUST display status `deviated`, the offending-path diagnosis text recorded in the lane note, and the preserved worktree path location.

**Terminal consumer**: `internal/serve/handlers.go:36-118` (via `GET /api/batch/lanes`), `internal/serve/model.go:26-125` (via `BatchLane.DemotionNote` / `lane_note` event reads), and `internal/serve/static/app.js:22-70`.

## MODIFIED Requirements

### Requirement: Individual Decisions Without Bulk Approval

Approval items in the multi-panel dashboard MUST start unselected and require individual decision submissions via single-item POST requests. The UI MUST NOT provide a bulk or "approve all" control, and the server MUST reject multi-item or array request bodies with HTTP 400 Bad Request.
(Previously: Governed standalone approvals page decisions; updated to enforce single-item decision submission and bulk-request rejection inside the multi-panel dashboard shell.)

**Live block**: `openspec/specs/approvals-web-ui/spec.md:26-48`, 3 scenarios.

## Open Questions

- [ ] Whether reconciliation action execution (`approve`, `decline`, `cancel`, `renew`, `resolve` matching `cmd/lucind-ai/cli.go:1044-1065`) will be supported via HTTP POST endpoints or restricted to copy-paste CLI command rendering (`internal/serve/handlers.go:36-118`).
- [ ] Whether overlap `evidence_json` (`internal/serve/model.go:68`) should be rendered as escaped `<pre>` blocks or parsed by a zero-dependency client-side diff tokenizer (`internal/serve/static/app.js:51-55, 91-94`).
- [ ] Whether lease and reconciliation expiration countdowns should be computed as server-side `remaining_seconds` on `serve.Model` or derived client-side from `expires_at` alongside a server timestamp (`internal/serve/model.go:56, 84, 354-357`).
