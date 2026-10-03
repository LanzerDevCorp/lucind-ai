#!/usr/bin/env bash
set -uo pipefail

# Warn if any orphaned agy background processes remain
ORPHANS=$(ps -eo pid,ppid,args 2>/dev/null | grep -E "(^| )agy( |$)|/agy( |$)" | grep -v grep | grep -v "lucind-ai" | grep -v "check-orphaned" || true)

if [[ -n "$ORPHANS" ]]; then
  echo "⚠️  [lucind-ai] Notice: Potential active/orphan agy processes found:" >&2
  echo "$ORPHANS" >&2
fi

exit 0
