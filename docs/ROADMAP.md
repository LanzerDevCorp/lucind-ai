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

## Only if needed

- Cross-machine attestations (today the HMAC key and attestations are local).
- A second provider, only when a real need and a verified hook surface exist.
