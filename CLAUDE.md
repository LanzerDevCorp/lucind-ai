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
