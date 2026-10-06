# invalid-key-guidance

Branch: `fix/invalid-key-guidance`
Engram mirror: `odd/invalid-key-guidance/tasks`
Route: direct inline by the orchestrator, on the owner's explicit request. This document was written
after the code, not before the first write as the protocol asks; the tasks below are the work done.

## Objective

When the stored `TYPESAFE_API_KEY` exists but the server rejects it, tell the user and the
orchestrator how to fix it, and give `lucind-ai install` a way to replace the key.

## Problem

A headless orchestrator test (lane `20261005-205855-e207`) showed that for a rejected key (HTTP 401)
the skill and the exit 5 message pointed to `lucind-ai install`. Plain install never replaces a key
that already resolves (`determineVariantAndSetupKey` returns on any non-empty key without validating
it), so the advice was wrong: the orchestrator told the user that install would restore auto-skills.

## Decisions

- D1 (owner): fix the guidance text and add `install --reset-key`.
- D2: exit 5 prints a different hint when the server answers 401 or 403 (`IsKeyRejected`), and keeps
  the missing-key hint only for a missing key. Other failures (500, 429, network) blame neither.
- D3: `--reset-key` needs a terminal and fails with exit 1 before installing anything when there is
  none. An empty answer keeps the current key. A warning is printed when the environment variable is
  set because it overrides the file. The key value never appears in any output.
- D4: `--help` stays valid only on its own, as the existing test requires.

## Tasks

- [x] T1 `AutoSkillsUnavailableError.IsKeyRejected` and the rejected-key stderr hint.
- [x] T2 `install --reset-key`, combinable with `--no-claude-md` in any order.
- [x] T3 skill text, README, product, skill-selection and ROADMAP.

## Progress and evidence

RED observed first (the new tests did not compile). GREEN: `TestAutoSkillsUnavailableError_IsKeyRejected`,
`TestInstall_ResetKey_*`, `TestInstall_PlainInstallNeverReplacesAnExistingKey`,
`TestInstall_FlagsCombineInAnyOrderAndUnknownOnesFail`, `TestDispatch_AutoSkillsUnavailable_RejectedKeyHint`.
The full suite caught a regression of mine: my first flag loop accepted `--help extra`; fixed to keep help
valid only alone, and the four hardcoded usage strings in `install_test.go` were updated.
Final: `gofmt -l` clean, build OK, `golangci-lint` 0 issues, `go test ./... -race -count=1` green (14 packages).
Real terminal check with a pty and a temporary probe test (removed): the hidden read does not echo, Ctrl-D
skips with an empty answer and no error, and Ctrl-C restores terminal echo. This closes the earlier
"no automated test for the terminal path" caveat, although the pty check itself is not part of the suite.
RDD: off for this clone (clone-local), so no native review.

## Next step

Merge to `dev` and `make install`. Then run `lucind-ai install --reset-key` in a terminal only if the
stored key is ever rejected again.
