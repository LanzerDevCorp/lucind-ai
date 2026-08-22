# Proposal: Control Room UI Views

## Intent

The approvals UI is still an approvals-only page. `lucind-ai serve` binds loopback HTTP (`internal/serve/server.go:19-28`) and `NewHandler` serves `/`, `GET /api/state`, and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:36-118`). `ServerState` is approver identity, wrong-approval rate, the opencode command, and pending approvals (`internal/serve/handlers.go:16-21, 120-146`). `app.js` polls `/api/state` every 2s and rebuilds approval cards (`internal/serve/static/app.js:22-70, 96-97`).

Feature-parent state already has a shell-free query surface (`internal/serve/model.go:14-24`) over schema v4 tables (`internal/ledger/schema.go:96-179`; `schemaVersion` is 5 at line 10): features, attempts, leases, overlap evidence, reconciliation, and audit events. `NewHandler` takes `*ledger.Ledger`, not `*Model` (`internal/serve/handlers.go:36`). Operators already run `reconcile approve|decline|cancel|renew|resolve` from the CLI (`cmd/lucind-ai/cli.go:1044-1065`). The browser cannot show feature lifecycles, leases, overlap, reconciliation, batch waves, or lane demotion notes.

This change is that dashboard: modular REST over `serve.Model`, lazy vanilla JS panels, zero npm, still localhost-only (`docs/prd.md:217-221`).

## Selected candidate and approach

**Candidate 2: Modular REST endpoints with lazy-loaded vanilla JS panels** (`openspec/changes/control-room-ui-views/explore.md:3, 20-23`). Keep `net/http` + `embed.FS` (`internal/serve/static.go:8-19`). Rejected: monolithic `/api/state` (retransmits `evidence_json` and candidate `output` every 2s — `internal/serve/model.go:68, 109-110`); `html/template` postbacks (tears down POST-then-`fetchState` — `internal/serve/static/app.js:72-89` — and the bulk-400 tests at `internal/serve/server_test.go:42-93`); React/Vue/npm (PRD §8.3).

**Lane-lifecycle call site.** This change does not intercept `ExecuteBatch` (`internal/run/batch.go:66`) or `enforceAllowedPaths` (`internal/run/run.go:409`). It hooks `serve.NewHandler` (`internal/serve/handlers.go:36`): new GET routes go through `*Model`. Lane status is read from `lanes` (`internal/ledger/schema.go:18-32`) after `SetStatus` (`internal/run/run.go:480`); demotion text is the existing `lane_note` event (`internal/run/run.go:423-430`). Approvals stay on `handleDecide` (`internal/serve/handlers.go:148-211`).

**Server.** `NewHandler` accepts `*Model` (or builds one from the ledger). Feature-parent queries use the typed methods already on Model. New read methods cover batch/lane rows (today Model only has feature-parent DTOs — `internal/serve/model.go:26-125`). Proposed routes:

| Route | Backing |
|---|---|
| `GET /api/approvals` | Current `ServerState` fields (`internal/serve/handlers.go:16-21, 120-146`) |
| `GET /api/features` | `ListFeatures` (`internal/serve/model.go:128-149`) |
| `GET /api/features/{id}/attempts` | `ListAttempts` (`internal/serve/model.go:167-188`) |
| `GET /api/leases` | `ListLeases` (`internal/serve/model.go:206-227`) |
| `GET /api/overlap/{feature_id}` | `ListOverlapEvidence` (`internal/serve/model.go:245-266`) |
| `GET /api/reconcile/requests` | Per-feature `ListReconciliationRequests` / `assembleRequest` (`internal/serve/model.go:277-301, 345-350`); a collection URI composes those |
| `GET /api/batch/lanes` | New Model reads over `lanes` plus `BatchReport` (`internal/run/batch.go:19-27`), per-lane deadlines (`40-43`), preserved worktrees (`50-52`), statuses (`internal/lane/status.go:10-16`), barrier `Outcome` (`internal/barrier/barrier.go:21-60`), and `lane_note` diagnosis |

`GET /api/state` and `POST /approvals/...` remain.

**Client.** Reorganize `internal/serve/static/app.js:1-98` into five panels: Batch/Wave, Approvals, Feature/Lease, Reconciliation, Lane envelope. Restrict the 2s poll (`96-97`) to `/api/approvals`, `/api/leases`, and `/api/batch/lanes`. Fetch `evidence_json`, candidate `output`/`checks`, and audit on card expand (`internal/serve/model.go:68, 109-110, 118-125`). Keep `escapeHtml` / `textContent` (`internal/serve/static/app.js:51-55, 91-94`). Keep per-card Approve/Reject (`63-66`); backend still 400s bulk bodies (`internal/serve/handlers.go:161-176`; `docs/prd.md:229-241`).

## Conceptual changes

1. **Model on the HTTP path.** Handlers query Model instead of calling ledger methods only from `serveStateJSON`.
2. **Batch/lane DTOs on Model.** Needed for waves (`internal/dag/waves.go:41-70`) and barrier release. There is no `runs` table in this schema.
3. **Hot poll vs on-demand.** Do not grow `ServerState` into a full-dashboard blob.
4. **Read-only browser.** Model does not run git or shell (`internal/serve/model.go:14-16`). Mutations stay individual ledger transactions (`internal/serve/handlers.go:148-211`). Reconciliation POST vs copy-paste CLI is unresolved.

## Capabilities

### New Capabilities
- `batch-wave-view`: concurrent lanes, DAG waves, deadlines, worktrees, barrier release
- `feature-lease-monitor`: feature rows, lease owner/fence, attempts, overlap payloads
- `reconciliation-workspace`: requests, candidates, checks, CAS, audit (read)
- `lane-envelope-inspector`: demotion diagnosis and preserved worktree for `deviated` lanes

### Modified Capabilities
- `approvals-web-ui`: same anti-bulk / inline-evidence / wrong-rate / opencode rules inside the multi-panel shell (`openspec/specs/approvals-web-ui/spec.md:26-83`)

| Capability | Impact | Existing seam |
|---|---|---|
| `approvals-web-ui` | Modified | `internal/serve/handlers.go:36-118, 161-176`; `internal/serve/static/app.js:12-20, 22-70` |
| `batch-wave-view` | Added | `internal/run/batch.go:19-27, 40-43`; `internal/dag/waves.go:41-70`; `internal/lane/status.go:10-28`; `internal/barrier/barrier.go:21-60`; `internal/worktree/worktree.go:168-238`; `internal/ledger/schema.go:18-32` |
| `feature-lease-monitor` | Added | `internal/ledger/schema.go:96-139`; `internal/serve/model.go:27-70, 128-266`; `internal/feature/feature.go:48-93, 293-360` |
| `reconciliation-workspace` | Added | `internal/ledger/schema.go:141-179`; `internal/serve/model.go:74-125, 278-343`; `cmd/lucind-ai/cli.go:1044-1065` |
| `lane-envelope-inspector` | Added | `internal/run/run.go:423-430, 576-654`; `internal/run/batch.go:50-52`; `internal/result/result.schema.json:21-43` |

## Delta specifications

### Requirement: Anti-rubber-stamping in the multi-view shell

The approvals surface MUST keep individual selection and one-item POST. The server MUST 400 bulk/multi-item bodies (`internal/serve/handlers.go:161-176`). Evidence renders only as command output or `file:line` (`internal/serve/static/app.js:12-20`); otherwise a fallback note (`51-55`). Show the operator's own wrong-approval rate and the opencode command (`internal/serve/handlers.go:130-140`).

#### Scenario: Bulk approval payload rejected
- GIVEN pending approvals (`internal/ledger/schema.go:45-56`)
- WHEN a client POSTs an array or multi-item body to `/approvals/` (`internal/serve/handlers.go:161-176`)
- THEN HTTP 400 and no ledger decision

#### Scenario: Unsupported claim withheld
- GIVEN evidence that is neither command output nor `file:line` (`internal/serve/static/app.js:12-20`)
- WHEN the card renders (`51-55`)
- THEN the string MUST NOT appear as evidence; a fallback note MUST

### Requirement: Batch and DAG wave inspection

The UI MUST show batch status, wave grouping (`internal/dag/waves.go:41-70`), lane status `pending`/`running`/`done`/`blocked`/`deviated`/`failed` (`internal/lane/status.go:10-16`), executor (`internal/ledger/schema.go:22`), worktree path (`internal/worktree/worktree.go:212-238`), per-lane deadline (`internal/run/batch.go:40-43`), and barrier `Released` / Integrate / Preserve (`internal/barrier/barrier.go:21-60`).

#### Scenario: Wave and lane status
- GIVEN an active batch (`internal/run/batch.go:19-27`)
- WHEN the operator opens the batch inspector
- THEN each lane shows status, executor, worktree path, and whether the barrier released (`internal/barrier/barrier.go:21-29`)

#### Scenario: Barrier release with mixed terminals
- GIVEN one lane `done` and another `failed` or `deviated` (`internal/lane/status.go:21-28`)
- WHEN `Evaluate` runs (`internal/barrier/barrier.go:36-60`)
- THEN the UI shows Released, integrate-eligible done lanes, and preserved non-done worktrees (`internal/run/batch.go:50-52`)

### Requirement: Shell-free feature and lease monitoring

Feature and lease views MUST use `serve.Model` only (`internal/serve/model.go:14-24`). Show feature state (`27-35`), lease owner and fence (`52-59`), latest attempt (`37-50`), and overlap class/payload (`62-70`, `internal/ledger/schema.go:131-139`).

#### Scenario: Active lease and attempt
- GIVEN a feature with a lease and attempts (`internal/ledger/schema.go:96-129`)
- WHEN the operator selects it
- THEN the UI shows owner, fence, attempt status, and candidate SHA (`internal/serve/model.go:128-227`)

### Requirement: Reconciliation candidate inspection

The UI MUST show requests, candidates, checks, and `CASResult` (`internal/serve/model.go:74-115, 278-323`). Mutation (POST vs copy-paste CLI) is out of this requirement.

#### Scenario: Candidate display
- GIVEN a request with a candidate (`internal/ledger/schema.go:141-169`)
- WHEN the operator inspects it
- THEN the UI shows status, allowed paths, checks, and `output` (`internal/serve/model.go:101-115`)

### Requirement: Lane demotion diagnosis

When `enforceAllowedPaths` demotes Done→Deviated (`internal/run/run.go:643-653`), the UI MUST show `deviated`, the offending-path text from the `lane_note` (`423-430, 651`), and the preserved worktree (`internal/run/batch.go:50-52`). Envelope `hard_stops` (`internal/result/result.schema.json:21-43`) are displayed only if a Model DTO can supply them without `os`/`git` (`internal/serve/model_test.go:610-618`).

#### Scenario: Deviated lane
- GIVEN a lane demoted for paths outside `allowed_paths` (`internal/run/run.go:650-652`)
- WHEN the operator opens the envelope inspector
- THEN status is `deviated`, offending paths are listed, and the worktree path is shown

## Technical risks and failure modes

| Risk | Impact | Mitigation | Seam |
|---|---|---|---|
| 2s `innerHTML = ''` drops focus, open cards, scroll | High | Keyed in-place DOM patch | `internal/serve/static/app.js:45-70, 96-97` |
| XSS via candidate `output` / evidence | High | `escapeHtml` / `textContent` on every inserted string | `internal/serve/static/app.js:51-55, 91-94`; `internal/serve/model.go:109-110` |
| Bulk / "approve all" in new panels | High | Per-card actions; keep HTTP 400 | `internal/serve/handlers.go:161-176`; `internal/serve/static/app.js:63-66`; `internal/serve/static_test.go:11-39` |
| Polling `evidence_json` /  long `output` freezes the tab | Medium | Omit from hot polls; fetch on expand | `internal/serve/model.go:68, 109, 245-266` |
| Browser clock vs `ExpiresAt` | Medium | Server `remaining_seconds` or `server_time` (open question) | `internal/serve/model.go:56, 84, 354-357`; `internal/feature/feature.go:294-297` |
| Batch views bypass Model and shell out | Medium | New `ListBatchLanes` / diagnosis DTOs; keep `TestModelSourceDoesNotShellOut` | `internal/serve/model.go:14-24, 26-125`; `internal/serve/model_test.go:595-627` |

## Rollback and additivity

`git revert` the commit. New work is GET routes, static JS/HTML, and Model read methods. No schema migration (`schemaVersion` stays 5 at `internal/ledger/schema.go:10`). Loopback only (`internal/serve/server.go:19-28`). Keep `GET /api/state` (`internal/serve/handlers.go:79-85, 120-146`) and `POST /approvals/{runID}/{laneID}` (`87-115, 148-211`). New `/api/*` view routes are additive.

## Test and validation impact

| Layer | Coverage | Seam |
|---|---|---|
| Model | New batch/lane DTOs and optional countdown helpers; `TestModelSourceDoesNotShellOut` must still forbid `os`/`os/exec`/`git` | `internal/serve/model_test.go:74-347, 595-627` |
| HTTP | New GET routes; keep 400 bulk and 409 duplicate decide | `internal/serve/server_test.go:42-93, 196-236` |
| Static | No "approve all" / "bulk-approve"; `isValidEvidence` still rejects bare prose; cards start unselected | `internal/serve/static_test.go:11-102` |
| Observation | Read-only checks that UI DTOs match barrier/batch facts without calling `ExecuteBatch` | `internal/barrier/barrier_test.go`; `internal/run/batch.go:19-89` |

## Out of scope

- Loopback bind, TLS, listen address — `control-room-serve` (`internal/serve/server.go:16-73`)
- Shell chrome, navigation frame, CSS tokens — `control-room-ui-shell` (`internal/serve/static/index.html:8-17`)
- Schema migrations and ledger writes — `control-room-ledger` (`internal/ledger/schema.go:10, 18-180`)
- Live logs / pty / telemetry — `control-room-capture`, `control-room-telemetry`
- npm, TypeScript, React/Vue/Svelte (`docs/prd.md:217-221`)
- Reconciliation mutation in the UI (open question)
- Reading `.lucind/result.json` from the worktree filesystem (Model must not import `os`)

## Open questions

- [ ] May the UI POST reconcile `approve`/`decline`/`cancel`/`renew`/`resolve`, or only show copy-paste CLI matching `cmd/lucind-ai/cli.go:1044-1065`? Handlers have no reconcile routes (`internal/serve/handlers.go:36-118`). Escalated in `openspec/changes/control-room-ui-views/explore.md:84`.
- [ ] Render overlap `evidence_json` (`internal/serve/model.go:68`) as `<pre>` + `escapeHtml` (`internal/serve/static/app.js:53, 91-94`), or a zero-dependency inline diff tokenizer?
- [ ] Lease/reconcile countdowns: Model `remaining_seconds`, or `expires_at` plus a server timestamp (`internal/serve/model.go:56, 84, 354-357`)?
