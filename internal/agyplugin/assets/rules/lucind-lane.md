---
trigger: model_decision
description: Only when working inside a lucind-ai lane (the LUCIND_LANE environment variable is set and the task brief lives in .lucind/lanes/<id>/brief.md).
---

# lucind-ai lane contract

You are running inside a lucind-ai lane. Read `.lucind/lanes/$LUCIND_LANE/brief.md` first.

- Edit only files matching the lane's allowed paths (`allow` in `.lucind/lanes/$LUCIND_LANE/lane.json`).
  A hook denies file writes outside them; shell writes are caught later by `lucind-ai accept`.
- Never read the attest key or touch the attestations directory; the hook denies it.
- Before stopping, write your result envelope to `.lucind/lanes/$LUCIND_LANE/result.json`
  (see the `lucind-result` skill for the exact shape). Stopping without a valid envelope
  sends you back to fix it, at most twice, after which the lane is marked failed.
- Finish with `lucind-ai attest run -- sh lucind-checks.sh` on the final tree so `accept`
  can reuse the attestation.
