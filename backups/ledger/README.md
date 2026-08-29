# Ledger backups

Point-in-time snapshots of the lucind-ai ledger (`.lucind/lucind.db`), which is
gitignored runtime state and therefore never travels with a clone.

Each file is produced with SQLite's online backup API, not a file copy:

```
sqlite3 "$PWD/.lucind/lucind.db" ".backup '$PWD/backups/ledger/lucind-ledger-<UTC timestamp>.db'"
```

The database runs in WAL mode, so a plain `cp` of the `.db` alone can miss
transactions still living in the `-wal` file. `.backup` takes a consistent
snapshot instead.

## Restoring on another machine

```
mkdir -p .lucind
cp backups/ledger/lucind-ledger-<stamp>.db .lucind/lucind.db
sqlite3 "file:$PWD/.lucind/lucind.db?mode=ro" "PRAGMA integrity_check;"
```

The ledger is machine-local coordination state: lanes, runs, acceptance
receipts, authoring evidence, features, and defect records. Restoring it makes
this repository's run history and defect records readable again; it does not
restore lane worktrees, which are reclaimed on successful integration.
