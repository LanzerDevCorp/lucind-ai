package ledger

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledgerpath"
)

func TestMigrateV5ToV6PreservesRowsAndAddsSchema(t *testing.T) {
	ctx := context.Background()
	root := createV5SchemaFixture(t, ctx)

	l, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("Open v5 database: %v", err)
	}
	defer l.Close()

	var versionCount int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 6`).Scan(&versionCount); err != nil {
		t.Fatalf("query v6 migration: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("version 6 migration count = %d, want 1", versionCount)
	}

	assertTableColumns(t, l.db, "runs", []string{
		"run_id", "feature_id", "status", "target_ref", "lane_count", "started_at", "ended_at",
	})
	assertTableColumns(t, l.db, "lane_progress", []string{
		"run_id", "lane_id", "seq", "message", "at",
	})
	assertTableColumns(t, l.db, "lanes", []string{
		"run_id", "lane_id", "packet_id", "executor", "routing_condition", "status",
		"worktree_path", "worktree_preserved", "attempt", "started_at", "ended_at",
		"model", "agent", "feature",
	})

	for _, table := range []string{"runs", "lane_progress", "lanes", "events"} {
		var strict int
		if err := l.db.QueryRowContext(ctx, `SELECT strict FROM pragma_table_list WHERE name = ?`, table).Scan(&strict); err != nil {
			t.Fatalf("query %s strict flag: %v", table, err)
		}
		if strict != 1 {
			t.Errorf("%s strict = %d, want 1", table, strict)
		}
	}

	var (
		packetID, executor, routingCondition, status, worktreePath string
		worktreePreserved, attempt                                 int
		startedAt, endedAt                                         string
		model, agent, feature                                      sql.NullString
	)
	if err := l.db.QueryRowContext(ctx, `
		SELECT packet_id, executor, routing_condition, status, worktree_path,
		       worktree_preserved, attempt, started_at, ended_at, model, agent, feature
		FROM lanes WHERE run_id = 'run-v5' AND lane_id = 'lane-v5'
	`).Scan(
		&packetID, &executor, &routingCondition, &status, &worktreePath,
		&worktreePreserved, &attempt, &startedAt, &endedAt, &model, &agent, &feature,
	); err != nil {
		t.Fatalf("query preserved lane: %v", err)
	}
	if packetID != "packet-v5" || executor != "opencode" || routingCondition != "route-v5" ||
		status != "done" || worktreePath != "/tmp/lane-v5" || worktreePreserved != 1 ||
		attempt != 3 || startedAt != "2026-08-22T01:00:00Z" || endedAt != "2026-08-22T01:05:00Z" {
		t.Errorf("v5 lane changed during migration: packet=%q executor=%q route=%q status=%q path=%q preserved=%d attempt=%d started=%q ended=%q",
			packetID, executor, routingCondition, status, worktreePath, worktreePreserved, attempt, startedAt, endedAt)
	}
	if model.Valid || agent.Valid || feature.Valid {
		t.Errorf("new metadata = model:%v agent:%v feature:%v, want NULL values", model, agent, feature)
	}

	var eventType, detail, eventAt string
	if err := l.db.QueryRowContext(ctx, `SELECT type, detail, at FROM events WHERE id = 41`).Scan(&eventType, &detail, &eventAt); err != nil {
		t.Fatalf("query preserved event: %v", err)
	}
	if eventType != "lane_note" || detail != "v5 detail" || eventAt != "2026-08-22T01:04:00Z" {
		t.Errorf("v5 event changed during migration: type=%q detail=%q at=%q", eventType, detail, eventAt)
	}

	var decision, evidence string
	if err := l.db.QueryRowContext(ctx, `
		SELECT decision, evidence FROM approvals WHERE run_id = 'run-v5' AND lane_id = 'lane-v5'
	`).Scan(&decision, &evidence); err != nil {
		t.Fatalf("query preserved approval: %v", err)
	}
	if decision != "approved" || evidence != "v5 evidence" {
		t.Errorf("v5 approval changed during migration: decision=%q evidence=%q", decision, evidence)
	}
}

func TestSchemaV6ConstraintsAndIndexes(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO runs (run_id, feature_id, status, target_ref, lane_count, started_at)
		VALUES ('run-1', 'feature-1', 'running', 'refs/heads/main', 2, '2026-08-22T02:00:00Z')
	`); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO runs (run_id, feature_id, status, target_ref, lane_count, started_at)
		VALUES ('run-1', 'feature-2', 'running', 'refs/heads/dev', 1, '2026-08-22T02:01:00Z')
	`); err == nil {
		t.Fatal("duplicate run_id insert succeeded, want primary-key error")
	}

	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO lane_progress (run_id, lane_id, seq, message, at)
		VALUES ('run-1', 'lane-1', 1, 'started', '2026-08-22T02:02:00Z')
	`); err != nil {
		t.Fatalf("insert lane progress: %v", err)
	}
	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO lane_progress (run_id, lane_id, seq, message, at)
		VALUES ('run-1', 'lane-1', 1, 'duplicate', '2026-08-22T02:03:00Z')
	`); err == nil {
		t.Fatal("duplicate progress sequence succeeded, want primary-key error")
	}

	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO events (run_id, type, detail, at)
		VALUES ('run-1', 'run_status_changed', 'done', '2026-08-22T02:04:00Z')
	`); err != nil {
		t.Fatalf("insert run_status_changed event: %v", err)
	}
	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO events (run_id, type, detail, at)
		VALUES ('run-1', 'not_a_real_event', '', '2026-08-22T02:05:00Z')
	`); err == nil {
		t.Fatal("unknown event type succeeded, want CHECK constraint error")
	}

	assertIndexColumns(t, l.db, "idx_lane_progress_run_lane_seq", []string{"run_id", "lane_id", "seq"})
	assertIndexColumns(t, l.db, "idx_events_run", []string{"run_id", "id"})
}

func TestSchemaV6ReopenIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root := createV5SchemaFixture(t, ctx)

	l1, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := l1.db.ExecContext(ctx, `
		INSERT INTO runs (run_id, feature_id, status, target_ref, lane_count, started_at)
		VALUES ('run-reopen', 'feature-reopen', 'running', 'main', 1, '2026-08-22T03:00:00Z')
	`); err != nil {
		t.Fatalf("insert run before reopen: %v", err)
	}
	if _, err := l1.db.ExecContext(ctx, `
		INSERT INTO lane_progress (run_id, lane_id, seq, message, at)
		VALUES ('run-reopen', 'lane-reopen', 1, 'persist me', '2026-08-22T03:01:00Z')
	`); err != nil {
		t.Fatalf("insert progress before reopen: %v", err)
	}
	var firstAppliedAt string
	if err := l1.db.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 6`).Scan(&firstAppliedAt); err != nil {
		t.Fatalf("query first v6 applied_at: %v", err)
	}
	if err := l1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	l2, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer l2.Close()

	var migrationCount int
	var secondAppliedAt string
	if err := l2.db.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(applied_at) FROM schema_migrations WHERE version = 6
	`).Scan(&migrationCount, &secondAppliedAt); err != nil {
		t.Fatalf("query v6 migration after reopen: %v", err)
	}
	if migrationCount != 1 || secondAppliedAt != firstAppliedAt {
		t.Errorf("v6 migration after reopen = count %d applied_at %q, want count 1 applied_at %q", migrationCount, secondAppliedAt, firstAppliedAt)
	}

	var runCount, progressCount int
	if err := l2.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE run_id = 'run-reopen'`).Scan(&runCount); err != nil {
		t.Fatalf("query run after reopen: %v", err)
	}
	if err := l2.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lane_progress WHERE message = 'persist me'`).Scan(&progressCount); err != nil {
		t.Fatalf("query progress after reopen: %v", err)
	}
	if runCount != 1 || progressCount != 1 {
		t.Errorf("preserved rows after reopen = runs %d progress %d, want 1 and 1", runCount, progressCount)
	}
}

func createV5SchemaFixture(t *testing.T, ctx context.Context) string {
	t.Helper()
	root := t.TempDir()
	dbPath := ledgerpath.Resolve(root)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open v5 fixture: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, schemaMigrationsDDL+schemaDDL); err != nil {
		t.Fatalf("apply v5 fixture schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, applied_at) VALUES
		(1, '2026-08-22T00:00:00Z'), (2, '2026-08-22T00:00:00Z'),
		(3, '2026-08-22T00:00:00Z'), (4, '2026-08-22T00:00:00Z'),
		(5, '2026-08-22T00:00:00Z');
		INSERT INTO lanes (
		  run_id, lane_id, packet_id, executor, routing_condition, status,
		  worktree_path, worktree_preserved, attempt, started_at, ended_at
		) VALUES (
		  'run-v5', 'lane-v5', 'packet-v5', 'opencode', 'route-v5', 'done',
		  '/tmp/lane-v5', 1, 3, '2026-08-22T01:00:00Z', '2026-08-22T01:05:00Z'
		);
		INSERT INTO events (id, run_id, lane_id, type, detail, at)
		VALUES (41, 'run-v5', 'lane-v5', 'lane_note', 'v5 detail', '2026-08-22T01:04:00Z');
		INSERT INTO approvals (
		  run_id, lane_id, packet_id, approver, decision, evidence,
		  defect_surfaced_later, requested_at, decided_at
		) VALUES (
		  'run-v5', 'lane-v5', 'packet-v5', 'maintainer', 'approved', 'v5 evidence',
		  0, '2026-08-22T01:02:00Z', '2026-08-22T01:03:00Z'
		);
	`); err != nil {
		t.Fatalf("seed v5 fixture: %v", err)
	}
	return root
}

func assertTableColumns(t *testing.T, db *sql.DB, table string, want []string) {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatalf("query %s columns: %v", table, err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan %s column: %v", table, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns: %v", table, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s columns = %v, want %v", table, got, want)
	}
}

func assertIndexColumns(t *testing.T, db *sql.DB, index string, want []string) {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_index_info(?) ORDER BY seqno`, index)
	if err != nil {
		t.Fatalf("query %s columns: %v", index, err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan %s column: %v", index, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns: %v", index, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s columns = %v, want %v", index, got, want)
	}
}
