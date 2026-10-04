# lucind-ai: roadmap

What exists is in [`product.md`](product.md).

## Done

- **agy-only contract** (`feature/agy-only-contract`): single provider, `dispatch`/`wait`,
  per-lane JSON state, embedded agy plugin (PreToolUse/Stop hooks), `accept` with allowed-path and
  attestation checks, Claude skill `lucind`. Orchestration, ledger and other providers removed.

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
