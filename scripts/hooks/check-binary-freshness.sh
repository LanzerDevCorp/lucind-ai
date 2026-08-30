#!/usr/bin/env bash
set -uo pipefail

if ! command -v lucind-ai >/dev/null 2>&1; then
  exit 0
fi

BINARY_SHA=$(lucind-ai -v 2>/dev/null | grep -oE '[0-9a-f]{7,40}' | head -1 || true)
HEAD_SHA=$(git rev-parse --short HEAD 2>/dev/null || true)

if [[ -n "$BINARY_SHA" && -n "$HEAD_SHA" ]]; then
  if [[ "$BINARY_SHA" != "$HEAD_SHA"* && "$HEAD_SHA" != "$BINARY_SHA"* ]]; then
    echo "⚠️  [lucind-ai] Installed binary ($BINARY_SHA) may be stale vs git HEAD ($HEAD_SHA). Run 'make install' if recent code was modified." >&2
  fi
fi

echo '{"decision": "allow"}'
exit 0
