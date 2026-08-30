---
id: remediate-phasespec-dispatch-wiring
executor: agy
routed_by: fix confirmed and corroborated verify.md findings 2, 3, 4, 5, 8 — phase specialist production wiring
model: gemini-3.7-flash-high
---

# Packet remediate-phasespec-dispatch-wiring

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-phasespec-dispatch-wiring  ·  **Branch:** lucind/remediate-phasespec-dispatch-wiring

## Goal

Fix five related verify.md findings, all rooted in the phase specialist not actually being wired end-to-end on the production `lucind-ai phase <name>` CLI path, even though unit tests pass by exercising the specialist's internal API directly:

1. **(CONFIRMED) Finding 2** — `cmd/lucind-ai/cli.go`'s `phaseDispatch` (around lines 2452-2460) calls `adapter.Synthesize(ctx, phasespec.SynthesizeRequest{ChangeName, Phase, Force})` with no `LensStates` and no `Content`. It never calls `admitDispatchBatch`/`runDispatch`. Only the already-complete no-op path is reachable from the CLI; an incomplete phase cannot actually get a synthesis lane dispatched.
2. **Finding 3** — `phasespec.Status` has no lens fields, so lens accepted/merged state from `gentle-ai sdd-status` JSON is never ingested into `Synthesize`. Only test code constructs `LensStates` directly.
3. **Finding 4** — Canonical artifact filenames in `phasespec.go` (`proposal.md`, `apply-progress.md`, `verify-report.md`, `archive-report.md`) diverge from the names the spec scenarios use (`propose.md` etc. — see `openspec/changes/skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md:7,13`).
4. **Finding 5** — `admitDispatchBatch` → `skillset.Derive` fail-closes on any `sdd_phase` outside the closed set, even when `lane_role` is omitted (a legacy packet that `packet.Parse` otherwise still accepts per its own backward-compatibility scenario). This is a regression against `read-only-packet-schema`'s "omitted lane_role preserves backward compatibility" scenario.
5. **Finding 8** — `phasespec.isPhaseComplete` treats a `done` status-JSON token as sufficient without checking that the canonical artifact file actually exists on disk, contrary to the unchanged-phase scenario's stated precondition.

## Preconditions

- Read `openspec/changes/skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md` in full — it is the authority for correct filenames, the lens-gating scenario, and the unchanged-phase scenario.
- Read `openspec/changes/skill-provisioning-and-phase-specialist/specs/read-only-packet-schema/spec.md` for the backward-compatibility scenario (finding 5).
- Read `internal/phasespec/phasespec.go` in full (the `Status`, `SynthesizeRequest`, `Synthesize`, `isPhaseComplete` functions and everything between).
- Read `internal/phasespec/phasespec_test.go` and `internal/phasespec/specialist_test.go` to see exactly how `LensStates`/`Content` are constructed today (test-only, per the finding) versus how they need to come from parsed `sdd-status` JSON in production.
- Read `cmd/lucind-ai/cli.go` around `phaseDispatch` (`:2389-2465` roughly) and `cmd/lucind-ai/packet_authoring.go`'s `admitDispatchBatch` (finding 5's fail-closed call site) and `internal/skillset/skillset.go`'s `Derive` (what makes it reject).
- Read `internal/packet/packet.go:208-215` and `packet_test.go:281-297` for the exact backward-compatibility scenario that finding 5 currently breaks.

## Allowed paths

- `internal/phasespec/`
- `cmd/lucind-ai/cli.go`
- `cmd/lucind-ai/packet_authoring.go`

## Read-only inputs

- `internal/skillset/`
- `internal/skillroots/`
- `internal/lucindconfig/`
- `internal/packet/`
- `internal/packetauthor/`

## Out of scope

Do not touch `internal/run`, `internal/accept`, or any plugin/asset files — those are separate remediation units dispatching in the same wave. Do not wrap or replace `gentle-ai`; do not add a `cmd` import into `internal/phasespec` (design.md explicitly forbids this — parse `sdd-status --json` output as data in `phasespec`, do any CLI-only wiring in `cmd/lucind-ai`). Do not change `phasespec`'s directory-traversal protection.

## Done criteria

- [ ] `phaseDispatch` parses lens accepted/merged state from the already-fetched `sdd-status --json` output and passes it into `Synthesize` (via an extended `phasespec.Status`/`SynthesizeRequest`, or equivalent) instead of calling `Synthesize` with empty `LensStates`/`Content`.
- [ ] For an incomplete phase with all required lenses merged, `Synthesize` actually triggers dispatch of the synthesis lane (via `admitDispatchBatch`/`runDispatch` — the existing dispatch machinery in `cmd/lucind-ai`, not new machinery), matching the spec's "dispatching the propose synthesis lane" scenario. `internal/phasespec` itself still does no `cmd` import; the dispatch call happens in `cmd/lucind-ai`.
- [ ] Canonical artifact filenames match the spec's named files exactly (`openspec/changes/<change>/<phase>.md` per the spec's own naming — confirm the exact expected name per phase from the spec text itself, do not guess).
- [ ] `admitDispatchBatch` no longer fail-closes a legacy packet that omits `lane_role` but declares a non-closed `sdd_phase` — `skillset.Derive` (or its call site) must be skipped or made permissive when `lane_role` is absent, restecting `packet.Parse`'s own backward-compatibility contract.
- [ ] `isPhaseComplete` checks that the canonical artifact file exists on disk in addition to the status-JSON `done` token.
- [ ] Existing tests `TestPhaseSubcommandGatesPrematureSynthesis`, `TestConsumesStatusAndWritesCanonicalArtifact`, `TestPhaseAlreadyCompleteNoRedundantDispatch`, and the legacy-omission test in `packet_test.go` are updated to assert the corrected behavior (not just left passing by coincidence) or new tests are added that would have caught each of the five findings above by exercising the real CLI/production path, not just the internal API.
- [ ] `go build ./...` and `go test ./... -race -count=1` are green.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops

- Stop `blocked` if the spec's exact expected canonical filename per phase is genuinely ambiguous after reading `phase-specialist-dispatch/spec.md` closely — quote the ambiguous text instead of guessing.
- Stop `blocked` if fixing finding 5 (backward compatibility) would require changing `packet.Parse`'s own contract rather than `admitDispatchBatch`/`skillset.Derive`'s handling of the already-parsed packet.
- Stop `blocked` if wiring real dispatch into `Synthesize`/`phaseDispatch` would require `internal/phasespec` to import `cmd` — that is explicitly forbidden; put that logic in `cmd/lucind-ai` instead.
- Stop `blocked` if any of these five findings turns out, on closer reading, not to reproduce — say so with evidence rather than fixing something that isn't broken.

## Return

Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
