# agy-pool / agyr — Antigravity/Gemini multi-account OAuth rotation

The `agy` CLI (Antigravity/Gemini) hits per-account quota. Auth is standard OAuth2: the token
lives in two local JSON files (`~/.gemini/oauth_creds.json` with the refresh token, and
`~/.gemini/google_accounts.json` with the active account). Rotating account = swapping those two
files for another account's saved copy — no browser session juggling or cookie scraping needed.

## Scripts

- **`agy-pool`** — profile manager. Subcommands: `init` (seed `~/.gemini/profiles/pool.conf` with
  the default 5-account pool), `add <email>`, `save <email> [--force]` (copy the currently-active
  creds into that account's saved profile; refuses a mismatched active account unless `--force`),
  `use <email>` (atomic swap back into `~/.gemini/`), `next` (rotate cyclically per
  `~/.gemini/profiles/.state`), `current` / `list` / `count`. Takes an `flock` on the state file so
  concurrent invocations don't race.
- **`agyr`** — wrapper that runs the real `agy` (absolute path, not shadowed, to avoid recursion),
  tees its output, and on `429|RESOURCE_EXHAUSTED|quota exceeded|rate limit exceeded` calls
  `agy-pool next` and retries the same command, up to once per pool account.

Both are meant to be installed on `PATH` (e.g. symlinked or copied into `~/.local/bin/`) — they are
not invoked from within this repo or by any Go code here.

## Pool accounts (5, all @gmail.com)

`lanzerdev20`, `ponenofeik5`, `lucindadsanchez`, `corp.systems.lanzer`, `corp.systems.lucinda`

## Status as of 2026-08-23

Both scripts are written and executable but **have never been run or tested**. `agy-pool init` has
not been run; no account has a saved profile under `~/.gemini/profiles/` yet.

## Known limitations

- Only works for single-turn CLI invocations. It cannot rotate an already-open interactive `agy`
  TUI session — `tee` breaks the terminal's raw mode.
- No equivalent hook exists for the Antigravity graphical IDE.
- `agy` has no `login`/`logout` subcommand. Logging into a different account means deleting
  `~/.gemini/oauth_creds.json` and running `agy` so it triggers the browser OAuth flow again — not
  yet confirmed/tested end to end.

## Remaining work

Log into and `agy-pool save` the 4 accounts other than the currently active one.

## Do not

- Do not run or test these scripts as part of routine repo work — they mutate
  `~/.gemini/oauth_creds.json` and `~/.gemini/google_accounts.json` on the machine that runs them.
- Do not touch `~/.gemini/google_accounts.json` outside of these scripts' own logic — a live
  monitoring session elsewhere may depend on its current contents.
