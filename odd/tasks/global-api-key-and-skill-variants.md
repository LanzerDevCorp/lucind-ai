# global-api-key-and-skill-variants

Branch: `feature/global-api-key-and-skill-variants`
Engram mirror: `odd/global-api-key-and-skill-variants/tasks`
Delivery strategy: `ask-on-risk` (forecast ~500 authored lines; one lane, then split commits by task).

## Objective

`TYPESAFE_API_KEY` stops being a per-shell, per-invocation chore. `lucind-ai` finds it in the
environment or in `~/.config/lucind/env`, `lucind-ai install` stores it once, and the installed
Claude skill matches what is configured: with a key the orchestrator only passes `--auto-skills`;
without one it keeps resolving the skills section by hand.

## Problem

Today the key is read only with `os.Getenv`, so every dispatch needs it exported by the caller.
With an empty key dispatch only warns and continues without a skills section, which silently loses
the skill loading that ROADMAP item 4 just fixed. The orchestrator also carries the cognitive load
of resolving skill paths even when Jev could do it.

## Decisions

- D1 (owner): lookup order is the environment variable, then `~/.config/lucind/env`
  (`$XDG_CONFIG_HOME/lucind/env` when `XDG_CONFIG_HOME` is set).
- D2 (owner): `lucind-ai install` asks for nothing when the key already resolves (variable or
  file); otherwise it asks for the key and stores it.
- D3 (owner): two skill variants. With a key: the orchestrator is not obliged to write the skills
  section and the skill only tells it to pass `--auto-skills`. Without a key: the orchestrator
  resolves the skills section by hand as the fallback.
- D4: file format is dotenv-style `TYPESAFE_API_KEY=value` (tolerates `export `, quotes, comments,
  blank lines), written with mode 0600 in a 0700 directory. The environment variable wins over
  the file. Install never overwrites an existing key and never copies a key from the environment
  to disk by itself.
- D5: the prompt needs a terminal. Without one (for example `make install` in CI or from an agent),
  install does not block: it installs the manual variant and prints how to configure the key.
  An empty answer at the prompt skips it and also installs the manual variant.
- D6: no new dependencies. The key is read without echo through `stty -echo` on the terminal; if
  `stty` is unavailable the prompt says the input will be visible.
- D7: one source `SKILL.md` with `<!-- lucind:variant manual -->` and `<!-- lucind:variant auto -->`
  blocks. The installer keeps the selected blocks and drops the other ones; unmarked text is common.
- D8: the skill variant is chosen at install time. Adding the key later needs a re-run of
  `lucind-ai install` to switch the variant; the skill and the README say so.

## Scope

In: new `internal/userconfig` package, `skillselect` key resolution, error messages in `dispatch`
and `skills select`, `claudeplugin` variants, `lucind-ai install` flow, the skill source, docs, tests.
Out: validating the key against Jev, `lucind-ai` flags to pass the key, other secrets.

## Tasks

- [ ] T1 userconfig: path resolution, tolerant reader, 0600 writer, and `skillselect` using env then
      file (rename `KeyFromEnv` to `ResolveKey`); update the missing-key error text.
- [ ] T2 skill variants: markers in `SKILL.md`, a render step in `claudeplugin`, both variants tested.
- [ ] T3 install flow: resolve key, prompt when absent and a terminal exists, store it, pick the
      variant, report which one was installed.
- [ ] T4 docs: README, product, skill-selection, ROADMAP (Done entry, remove the `.env` note).

## Acceptance criteria

- With `TYPESAFE_API_KEY` unset and a valid `~/.config/lucind/env`, `dispatch --auto-skills` works.
- The environment variable overrides the file.
- `lucind-ai install` with a resolvable key asks nothing and installs the auto variant.
- `lucind-ai install` with no key and no terminal installs the manual variant and exits 0.
- With a terminal and no key, install prompts, writes the file with mode 0600, installs the auto variant.
- No test reads or writes the real home directory (every test isolates `XDG_CONFIG_HOME`).
- The key is never printed, logged or put in an error message.

## Checks

- `golangci-lint run`
- `CGO_ENABLED=0 go build ./...`
- `go test ./... -race -count=1` (`TestLaneIDFormat` is a known flake)

## Progress

Branch created. No task started.

## Next step

Dispatch one lane (single writer, one tree) with `--auto-skills`, review the diff against this
document, then `accept`.
