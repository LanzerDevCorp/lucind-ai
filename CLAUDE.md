# lucind-ai — project notes

## Keeping the `lucind-ai` binary current

`lucind-ai -v` (or `--version`) prints the exact build (`git describe`) baked in at compile time.
Check it before dispatching if the installed binary was built more than a few commits ago — a
stale binary silently lacks recent features. This bit once: a session hit `unsupported executor
"cursor-agent"` from `$GOPATH/bin/lucind-ai`, built before that executor landed, with no way to
tell it was stale.

Run `make install` after any change touching the binary. It installs to `$GOPATH/bin` (already
on `PATH`) with a real version string, so `lucind-ai -v` always reflects what was actually built.
Never build to an ad-hoc temp path and dispatch from there instead — that's exactly how the
staleness above went unnoticed for a whole session.

## Checks for this repo

Pass the ones that fit the change to `lucind-ai dispatch` with `--check '<cmd>'` (repeatable).
`accept` requires an attestation of each one on the final tree and runs the missing ones.

| Check | Use it when |
|---|---|
| `golangci-lint run` | Any Go change. Minimal profile in `.golangci.yml` (`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`); the repo is clean, so any finding is new. |
| `CGO_ENABLED=0 go build ./...` | Any Go change. Catches compile errors in packages the change did not touch. |
| `go test ./... -race -count=1` | Logic changes. Skip it for lint-only or docs-only work. |

`lucind-checks.sh` runs all three in order.
Keep the two lists in sync. A narrower check (`go test ./internal/dispatch -race -count=1`) is
fine when the change is confined to one package.
`TestLaneIDFormat` can fail once in a full run (known flaky, see `docs/ROADMAP.md`); repeat it
before blaming the change.
