# lucind-ai: roadmap

What exists is in [`product.md`](product.md).

## Done

- **agy-only contract** (`feature/agy-only-contract`): single provider, `dispatch`/`wait`,
  per-lane JSON state, embedded agy plugin (PreToolUse/Stop hooks), `accept` with allowed-path and
  attestation checks, Claude skill `lucind`. Orchestration, ledger and other providers removed.
  Delivered as a chain of PRs (#5–#11) merged through #12.
- **Leftover cleanup** (`lucind-cleanup`, `chore/remove-packet-leftovers`, #13): static agy model
  list and the other non-blocking leftovers of the contract; packet-era rules, skills, backups,
  stray files and stale cursor-agent proposals removed.
- **Per-lane checks and `lucind-ai install`** (`feature/lane-checks-and-install`, #14): the
  orchestrator picks the attested checks per lane with `--check` (the hardcoded
  `lucind-checks.sh` is deprecated); one flagless `lucind-ai install` sets up the agy plugin and
  the Claude skill.
- **Stop-hook retries** (`feature/lane-stop-retries`): hook payload logging, `wait` revalidates
  the result before reporting `failed`, the retry budget resets after a quiet window and is
  capped, and the stop counters reset on lane continuation. Verified with real probe lanes.
- **Superseded:** the multi-provider herdr work (`herdr-agent-factory`, `herdr-interactive-agents`)
  predates the agy-only contract; its interactive-pane and Stop-hook ideas survive in it.

## Next

1. **Real-lane stability trials.** Run several real lanes end to end (Claude, dispatch, agy, attest,
   accept) across tasks and models; record failures of the Stop-retry loop, timeouts and
   allowed-path denials.
2. **Shell-write escape.** PreToolUse sees file-write tools; shell writes are caught only at
   `accept`. Measure how often it matters before adding anything.
3. **Stale agy trust entries after a crash** in `~/.gemini/antigravity-cli/settings.json`.
4. **Idle detection.** herdr `agent` idle detection is unreliable with agy; the Stop hook is the
   completion signal today.
5. **RTK support.** Install RTK as part of the lucind-ai setup (today it is wired by hand in the
   global Claude config: `@RTK.md` include plus the `rtk hook claude` PreToolUse hook).
6. **Research gentle-ai reviews in depth.** Understand how receipt-driven development (RDD) works
   end to end: review lifecycle, receipts and lineage, consent, correction, and how it interacts
   with lucind-ai lanes and `accept`.
7. **Inject the `lucind:dispatch` block into the global `~/.claude/CLAUDE.md`.** `lucind-ai install`
   should write it (idempotent, between its own markers, outside the gentle-ai ones) so the
   dispatch precedence rules stop being hand-maintained.
8. **Adopt golangci-lint with a minimal profile.** Enable only `errcheck`, `staticcheck`, `govet`,
   `unused` and `ineffassign` in `.golangci.yml`. Run it once first to see how many findings it
   reports (today `go vet` and `gofmt` are clean); if the first run is clean or small, add it as
   `--check 'golangci-lint run'` for lanes and to `lucind-checks.sh`. If it is noisy, trim the
   linter list before requiring it.

## Only if needed

- Cross-machine attestations (today the HMAC key and attestations are local).
- A second provider, only when a real need and a verified hook surface exist.
