# main-conversation-binding

Branch: `feature/main-conversation-binding` (from `dev`). Mirror: Engram `odd/main-conversation-binding/tasks`.

## Objective

Replace the Stop hook's classification of main vs worker conversations, which parses step 0 of agy's
private transcript, with a binding that lucind-ai owns: the first conversation id seen by an agy
`PreInvocation` hook after a turn starts is the main conversation of that turn.

## Problem and why

Roadmap item 3 ("poor solution, replace it"). `classifyConversation` in `internal/agyhook/agyhook.go`
couples lucind-ai to undocumented transcript internals (`source`, `type`, `<SYSTEM_MESSAGE>`, `sender=`,
the word `worker`) and breaks silently if any of them change.

Measured evidence (two real lanes, throwaway repo `lucind-probes`, 100 hook events):
- `PreInvocation` and `PostInvocation` fire for worker conversations, each with its own `conversationId`.
- The hook payload has only the documented fields: no parent id, no prompt text.
- The first event of a lane is the main conversation (`invocationNum` 0, `initialNumSteps` 1). Workers
  first appear later (`invocationNum` 0, `initialNumSteps` 0) while the main one is mid-run.
- `ANTIGRAVITY_CONVERSATION_ID` in the hook env equals the hook's own conversation id, so it carries no parent.
- herdr docs expose nothing per conversation (one agent per pane).

## Design

- New hook `lucind-ai hook pre-invocation`, registered under `PreInvocation` in the plugin `hooks.json.tmpl`.
  It always prints `{}` and never injects steps.
- Binding is a write-once file per turn, `<laneDir>/main-turn-<turn>`, created with `O_CREATE|O_EXCL`
  and holding the conversation id. Race-free without mutating `lane.json`.
- `Stop` classifies by comparing its `conversationId` with the current turn's marker: equal is main,
  different is worker, marker missing is unknown and keeps the existing `fullyIdle` fallback.
- Remove the transcript parsing (`classifyConversation`, `transcriptStep`, the `conversations/` cache).

## Scope

In: `internal/agyhook/**`, `internal/agyplugin/**`, `cmd/lucind-ai/hook*.go`, `docs/ROADMAP.md`, `docs/product.md`.
Out: `dispatch`, `accept`, lane schema, herdr integration, other providers.

Known limit: a worker of the previous turn still emitting `PreInvocation` before the main conversation
does in the next turn would be bound by mistake. Not measured; documented as accepted risk.

## Constraints

- Test-first with `go test` (RED then GREEN). A hook never breaks agy: always exit 0, always valid JSON.
- English artifacts. About 400 authored changed lines per task is a planning heuristic only.

## Acceptance criteria

- `pre-invocation` binds the first conversation per turn and ignores later ones; a new turn binds again.
- `Stop` ignores worker conversations, decides on the bound main conversation, and falls back to `fullyIdle`
  when no marker exists.
- No code reads the agy transcript for classification any more.
- The rendered plugin `hooks.json` contains `PreInvocation` and stays valid JSON.
- `golangci-lint run`, `CGO_ENABLED=0 go build ./...` and `go test ./... -race -count=1` pass.

## Tasks

- [ ] **T1 — Binding and Stop classification.** `PreInvocation` handler, `hook pre-invocation` subcommand,
  plugin template entry, Stop rewrite, tests. Route: delegated lane (writer trigger: 2+ non-trivial files),
  `claude-sonnet-5-5-medium`.
- [ ] **T2 — Docs.** `docs/ROADMAP.md` (move item 3 to Done, record the measurement) and `docs/product.md`.
  Route: same lane, separate commit.

## Progress and evidence

- Branch created; document and mirror written. No source changes yet.
- Review tier: pending (`gentle-ai review assess` after the work-unit commit).

## Next step

Dispatch the lane for T1 and T2.
