package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/barrier"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledgerpath"
)

// openTestLedger opens a Ledger against a fresh temp-dir primary root and
// registers its cleanup. No test in this package touches a real .lucind/.
func openTestLedger(t *testing.T) *Ledger {
	t.Helper()
	root := t.TempDir()
	l, err := Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

// TestOpenPlacesDatabaseUnderPrimaryRootLucindDir proves the "Single ledger
// location" requirement's first scenario with real code: Open derives the
// database path from the primary repository root via ledgerpath.Resolve
// instead of accepting a caller-supplied path, so the database file always
// lands at "<primaryRoot>/.lucind/lucind.db". There is no parameter through
// which a caller could point Open at any other location — pointing the
// ledger at an arbitrary path is not a discipline question here, it is
// structurally impossible through this API.
func TestOpenPlacesDatabaseUnderPrimaryRootLucindDir(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	l, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	wantPath := ledgerpath.Resolve(root)
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("database file not found at ledgerpath.Resolve(root) = %s: %v", wantPath, err)
	}
}

func TestOpenCreatesLucindDirAndSucceeds(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	l, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	if _, err := os.Stat(filepath.Join(root, ".lucind")); err != nil {
		t.Fatalf(".lucind directory was not created under root: %v", err)
	}
}

func TestOpenIsIdempotentOnSamePrimaryRoot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	l1, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := l1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	l2, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("second Open on the same primary root: %v", err)
	}
	defer l2.Close()
}

func TestOpenFailsWhenPragmaCannotApply(t *testing.T) {
	ctx := context.Background()

	// An in-memory database can never honor journal_mode=WAL (SQLite keeps
	// it as "memory"). This proves the pragma read-back actually asserts
	// the applied value rather than trusting the DSN syntax alone. It
	// exercises the unexported openAtPath directly since the public Open
	// no longer accepts an arbitrary dbPath (it only accepts a primary
	// repository root and derives the path itself).
	_, err := openAtPath(ctx, ":memory:")
	if !errors.Is(err, ErrPragmaNotApplied) {
		t.Fatalf("openAtPath(\":memory:\") error = %v, want ErrPragmaNotApplied", err)
	}
}

func TestRegisterLaneInsertsRetrievableRow(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()

	in := Lane{
		RunID:            "run-1",
		LaneID:           "lane-a",
		PacketID:         "packet-1",
		Executor:         "agy",
		RoutingCondition: "diff touches internal/lane",
		Status:           lane.Pending,
	}
	if err := l.RegisterLane(ctx, in); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}

	lanes, err := l.Lanes(ctx, "run-1")
	if err != nil {
		t.Fatalf("Lanes: %v", err)
	}
	if len(lanes) != 1 {
		t.Fatalf("Lanes returned %d rows, want 1", len(lanes))
	}

	got := lanes[0]
	if got.RunID != in.RunID || got.LaneID != in.LaneID {
		t.Fatalf("identity = (%q,%q), want (%q,%q)", got.RunID, got.LaneID, in.RunID, in.LaneID)
	}
	if got.RoutingCondition != in.RoutingCondition {
		t.Fatalf("RoutingCondition = %q, want %q", got.RoutingCondition, in.RoutingCondition)
	}
	if got.Status != lane.Pending {
		t.Fatalf("Status = %v, want %v", got.Status, lane.Pending)
	}
	if got.Attempt != 1 {
		t.Fatalf("Attempt = %d, want default 1", got.Attempt)
	}
}

func TestRegisterLaneRejectsMissingRoutingCondition(t *testing.T) {
	tests := []struct {
		name             string
		routingCondition string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := openTestLedger(t)
			in := Lane{
				RunID:            "run-1",
				LaneID:           "lane-a",
				PacketID:         "packet-1",
				Executor:         "agy",
				RoutingCondition: tt.routingCondition,
				Status:           lane.Pending,
			}
			err := l.RegisterLane(context.Background(), in)
			if !errors.Is(err, ErrRoutingConditionMissing) {
				t.Fatalf("RegisterLane error = %v, want ErrRoutingConditionMissing", err)
			}
		})
	}
}

func TestRegisterLaneRejectsInvalidStatus(t *testing.T) {
	l := openTestLedger(t)
	in := Lane{
		RunID:            "run-1",
		LaneID:           "lane-a",
		PacketID:         "packet-1",
		Executor:         "agy",
		RoutingCondition: "cond",
		Status:           lane.Status("bogus"),
	}
	if err := l.RegisterLane(context.Background(), in); err == nil {
		t.Fatal("RegisterLane with invalid status = nil error, want an error")
	}
}

func TestSetStatusUpdatesLaneAndAppendsEventsInOrder(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()

	in := Lane{
		RunID:            "run-1",
		LaneID:           "lane-a",
		PacketID:         "p",
		Executor:         "agy",
		RoutingCondition: "cond",
		Status:           lane.Pending,
	}
	if err := l.RegisterLane(ctx, in); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}

	t1 := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 18, 10, 5, 0, 0, time.UTC)

	if err := l.SetStatus(ctx, "run-1", "lane-a", lane.Running, t1); err != nil {
		t.Fatalf("SetStatus(running): %v", err)
	}
	if err := l.SetStatus(ctx, "run-1", "lane-a", lane.Done, t2); err != nil {
		t.Fatalf("SetStatus(done): %v", err)
	}

	lanes, err := l.Lanes(ctx, "run-1")
	if err != nil {
		t.Fatalf("Lanes: %v", err)
	}
	if len(lanes) != 1 || lanes[0].Status != lane.Done {
		t.Fatalf("lanes = %+v, want a single lane-a row with status done", lanes)
	}

	events, err := l.Events(ctx, "run-1")
	if err != nil {
		t.Fatalf("Events: %v", err)
	}

	var transitions []string
	for _, e := range events {
		if e.Type == "lane_status_changed" {
			transitions = append(transitions, e.Detail)
		}
	}
	want := []string{string(lane.Running), string(lane.Done)}
	if len(transitions) != len(want) || transitions[0] != want[0] || transitions[1] != want[1] {
		t.Fatalf("lane_status_changed event details in order = %v, want %v", transitions, want)
	}
}

func TestSetStatusOnUnknownLaneErrors(t *testing.T) {
	l := openTestLedger(t)
	err := l.SetStatus(context.Background(), "run-1", "missing-lane", lane.Done, time.Now())
	if !errors.Is(err, ErrLaneUnknown) {
		t.Fatalf("SetStatus on unknown lane error = %v, want ErrLaneUnknown", err)
	}
}

// TestConcurrentRegisterAndSetStatusAcrossDistinctLanes exercises the pool
// of 4 (never SetMaxOpenConns(1) — that would make this pass by
// construction). 8 goroutines each register and finish a distinct lane
// while one more goroutine repeatedly reads LaneStates concurrently. It
// asserts zero errors (including zero SQLITE_BUSY, which busy_timeout
// converts into a wait instead of a failure) and that every row landed
// correctly — the assertion is about real write contention, not
// last-writer-wins on a shared lane.
func TestConcurrentRegisterAndSetStatusAcrossDistinctLanes(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	const runID = "run-concurrent"
	const n = 8

	var wg sync.WaitGroup
	errs := make(chan error, n+1)

	for i := 0; i < n; i++ {
		laneID := fmt.Sprintf("lane-%d", i)
		wg.Add(1)
		go func(laneID string) {
			defer wg.Done()
			in := Lane{
				RunID:            runID,
				LaneID:           laneID,
				PacketID:         "p",
				Executor:         "agy",
				RoutingCondition: "cond",
				Status:           lane.Pending,
			}
			if err := l.RegisterLane(ctx, in); err != nil {
				errs <- fmt.Errorf("RegisterLane(%s): %w", laneID, err)
				return
			}
			if err := l.SetStatus(ctx, runID, laneID, lane.Done, time.Now()); err != nil {
				errs <- fmt.Errorf("SetStatus(%s): %w", laneID, err)
			}
		}(laneID)
	}

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < 50; i++ {
			if _, err := l.LaneStates(ctx, runID); err != nil {
				errs <- fmt.Errorf("LaneStates: %w", err)
				return
			}
		}
	}()

	wg.Wait()
	<-readerDone
	close(errs)

	for err := range errs {
		t.Errorf("concurrent operation error: %v", err)
	}

	lanes, err := l.Lanes(ctx, runID)
	if err != nil {
		t.Fatalf("Lanes: %v", err)
	}
	if len(lanes) != n {
		t.Fatalf("Lanes returned %d rows, want %d", len(lanes), n)
	}
	for _, ln := range lanes {
		if ln.Status != lane.Done {
			t.Fatalf("lane %s status = %v, want done", ln.LaneID, ln.Status)
		}
	}
}

func TestAppendEventStoresRunScopedEventWithNullLaneID(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()

	at := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	e := Event{RunID: "run-1", Type: EventRunStarted, Detail: "batch of 3 lanes", At: at}
	if err := l.AppendEvent(ctx, e); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events, err := l.Events(ctx, "run-1")
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Events returned %d rows, want 1", len(events))
	}
	got := events[0]
	if got.Type != EventRunStarted || got.Detail != e.Detail {
		t.Fatalf("event = %+v, want type %q detail %q", got, EventRunStarted, e.Detail)
	}
	if got.LaneID != "" {
		t.Fatalf("LaneID = %q, want empty (run-scoped/NULL)", got.LaneID)
	}
	if !got.At.Equal(at) {
		t.Fatalf("At = %v, want %v", got.At, at)
	}
}

func TestSetWorktreePreservedUpdatesFlag(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()

	in := Lane{
		RunID:            "run-1",
		LaneID:           "lane-a",
		PacketID:         "p",
		Executor:         "agy",
		RoutingCondition: "cond",
		Status:           lane.Blocked,
	}
	if err := l.RegisterLane(ctx, in); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}

	if err := l.SetWorktreePreserved(ctx, "run-1", "lane-a", true); err != nil {
		t.Fatalf("SetWorktreePreserved: %v", err)
	}

	lanes, err := l.Lanes(ctx, "run-1")
	if err != nil {
		t.Fatalf("Lanes: %v", err)
	}
	if len(lanes) != 1 || !lanes[0].WorktreePreserved {
		t.Fatalf("lanes = %+v, want a single lane-a row with WorktreePreserved=true", lanes)
	}
}

func TestSetWorktreePreservedOnUnknownLaneErrors(t *testing.T) {
	l := openTestLedger(t)
	err := l.SetWorktreePreserved(context.Background(), "run-1", "missing-lane", true)
	if !errors.Is(err, ErrLaneUnknown) {
		t.Fatalf("SetWorktreePreserved on unknown lane error = %v, want ErrLaneUnknown", err)
	}
}

// TestLaneStatesFeedsBarrierEvaluate is the seam test: it proves the
// ledger -> barrier boundary type (lane.State) round-trips through a real
// database read directly into barrier.Evaluate and produces the expected
// Outcome. internal/barrier itself never imports internal/ledger — only
// this test file, in the ledger package's test binary, does.
func TestLaneStatesFeedsBarrierEvaluate(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()

	laneIDs := []string{"a", "b", "c"}
	statuses := []lane.Status{lane.Done, lane.Done, lane.Blocked}
	for i, id := range laneIDs {
		in := Lane{
			RunID:            "run-1",
			LaneID:           id,
			PacketID:         "p",
			Executor:         "agy",
			RoutingCondition: "cond",
			Status:           statuses[i],
		}
		if err := l.RegisterLane(ctx, in); err != nil {
			t.Fatalf("RegisterLane(%s): %v", id, err)
		}
	}

	states, err := l.LaneStates(ctx, "run-1")
	if err != nil {
		t.Fatalf("LaneStates: %v", err)
	}

	outcome := barrier.Evaluate(laneIDs, states)
	if !outcome.Released {
		t.Fatal("Evaluate(states from LaneStates) did not release with every lane terminal")
	}

	wantIntegrate := map[string]bool{"a": true, "b": true}
	if len(outcome.Integrate) != len(wantIntegrate) {
		t.Fatalf("Integrate = %v, want exactly %v", outcome.Integrate, wantIntegrate)
	}
	for _, id := range outcome.Integrate {
		if !wantIntegrate[id] {
			t.Fatalf("Integrate contains unexpected lane %q", id)
		}
	}
	if len(outcome.Preserve) != 1 || outcome.Preserve[0] != "c" {
		t.Fatalf("Preserve = %v, want [c]", outcome.Preserve)
	}
}

const v1TestSchemaDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT;

CREATE TABLE IF NOT EXISTS lanes (
  run_id             TEXT    NOT NULL,
  lane_id            TEXT    NOT NULL,
  packet_id          TEXT    NOT NULL,
  executor           TEXT    NOT NULL CHECK (executor IN ('agy','cursor-agent','human')),
  routing_condition  TEXT    NOT NULL CHECK (length(trim(routing_condition)) > 0),
  status             TEXT    NOT NULL CHECK (status IN
                       ('pending','running','done','blocked','deviated','failed')),
  worktree_path      TEXT    NOT NULL DEFAULT '',
  worktree_preserved INTEGER NOT NULL DEFAULT 0 CHECK (worktree_preserved IN (0,1)),
  attempt            INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
  started_at         TEXT,
  ended_at           TEXT,
  PRIMARY KEY (run_id, lane_id)
) STRICT;

CREATE TABLE IF NOT EXISTS events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id  TEXT NOT NULL,
  lane_id TEXT,
  type    TEXT NOT NULL CHECK (type IN ('run_started','lane_registered',
            'lane_status_changed','barrier_released','run_ended')),
  detail  TEXT NOT NULL DEFAULT '',
  at      TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id, id);
`

func TestMigrateV1DatabaseAcceptsLaneNoteAndPreservesRows(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := ledgerpath.Resolve(root)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("create .lucind dir: %v", err)
	}

	dsn := "file:" + dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}

	if _, err := rawDB.ExecContext(ctx, v1TestSchemaDDL); err != nil {
		rawDB.Close()
		t.Fatalf("apply v1 schema DDL: %v", err)
	}

	v1AppliedAt := "2026-08-19T10:00:00Z"
	if _, err := rawDB.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?)`,
		v1AppliedAt,
	); err != nil {
		rawDB.Close()
		t.Fatalf("record v1 migration: %v", err)
	}

	// Insert lanes under v1.
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO lanes (run_id, lane_id, packet_id, executor, routing_condition, status, worktree_path, worktree_preserved, attempt)
		VALUES ('run-1', 'lane-a', 'p-a', 'agy', 'cond-a', 'running', '/path/a', 0, 1),
		       ('run-1', 'lane-b', 'p-b', 'human', 'cond-b', 'done', '/path/b', 1, 2)
	`); err != nil {
		rawDB.Close()
		t.Fatalf("insert v1 lanes: %v", err)
	}

	// Insert events under v1 exercising each of the 5 original event types.
	t1 := "2026-08-19T10:01:00Z"
	t2 := "2026-08-19T10:02:00Z"
	t3 := "2026-08-19T10:03:00Z"
	t4 := "2026-08-19T10:04:00Z"
	t5 := "2026-08-19T10:05:00Z"
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO events (id, run_id, lane_id, type, detail, at) VALUES
		(1, 'run-1', NULL, 'run_started', 'batch started', ?),
		(2, 'run-1', 'lane-a', 'lane_registered', 'cond-a', ?),
		(3, 'run-1', 'lane-a', 'lane_status_changed', 'running', ?),
		(4, 'run-1', 'lane-a', 'barrier_released', 'done', ?),
		(5, 'run-1', NULL, 'run_ended', 'batch ended', ?)
	`, t1, t2, t3, t4, t5); err != nil {
		rawDB.Close()
		t.Fatalf("insert v1 events: %v", err)
	}

	// Verify that inserting a lane_note into this v1 database before migration fails.
	if _, err := rawDB.ExecContext(ctx,
		`INSERT INTO events (run_id, lane_id, type, detail, at) VALUES ('run-1', 'lane-a', 'lane_note', 'note', ?)`,
		t1,
	); err == nil {
		rawDB.Close()
		t.Fatal("v1 events table accepted lane_note event before migration, want CHECK constraint violation")
	}

	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw DB: %v", err)
	}

	// Open the v1 database through the normal path, triggering migration to v2.
	l, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("Open normal path on v1 database: %v", err)
	}
	defer l.Close()

	// Assert no existing lane row is lost or altered.
	lanes, err := l.Lanes(ctx, "run-1")
	if err != nil {
		t.Fatalf("Lanes(run-1): %v", err)
	}
	if len(lanes) != 2 {
		t.Fatalf("Lanes returned %d rows, want 2", len(lanes))
	}
	if lanes[0].LaneID != "lane-a" || lanes[0].Status != lane.Running || lanes[0].WorktreePath != "/path/a" || lanes[0].WorktreePreserved != false || lanes[0].Attempt != 1 {
		t.Fatalf("lane-a altered by migration: %+v", lanes[0])
	}
	if lanes[1].LaneID != "lane-b" || lanes[1].Status != lane.Done || lanes[1].WorktreePath != "/path/b" || lanes[1].WorktreePreserved != true || lanes[1].Attempt != 2 {
		t.Fatalf("lane-b altered by migration: %+v", lanes[1])
	}

	// Assert no existing event row is lost, altered, or reordered.
	events, err := l.Events(ctx, "run-1")
	if err != nil {
		t.Fatalf("Events(run-1): %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("Events returned %d rows, want 5", len(events))
	}
	wantTypes := []string{"run_started", "lane_registered", "lane_status_changed", "barrier_released", "run_ended"}
	wantDetails := []string{"batch started", "cond-a", "running", "done", "batch ended"}
	wantLaneIDs := []string{"", "lane-a", "lane-a", "lane-a", ""}
	for i, e := range events {
		if e.ID != int64(i+1) {
			t.Errorf("event[%d].ID = %d, want %d", i, e.ID, i+1)
		}
		if e.Type != wantTypes[i] {
			t.Errorf("event[%d].Type = %q, want %q", i, e.Type, wantTypes[i])
		}
		if e.Detail != wantDetails[i] {
			t.Errorf("event[%d].Detail = %q, want %q", i, e.Detail, wantDetails[i])
		}
		if e.LaneID != wantLaneIDs[i] {
			t.Errorf("event[%d].LaneID = %q, want %q", i, e.LaneID, wantLaneIDs[i])
		}
	}

	// Now insert a lane_note event into the migrated database.
	t6 := time.Date(2026, 8, 19, 10, 6, 0, 0, time.UTC)
	noteEvent := Event{
		RunID:  "run-1",
		LaneID: "lane-a",
		Type:   EventLaneNote,
		Detail: "lane diagnostic note after migration",
		At:     t6,
	}
	if err := l.AppendEvent(ctx, noteEvent); err != nil {
		t.Fatalf("AppendEvent(lane_note) after migration failed: %v", err)
	}

	// Read back all events and verify lane_note is present and ID sequence continues.
	eventsAfter, err := l.Events(ctx, "run-1")
	if err != nil {
		t.Fatalf("Events after append: %v", err)
	}
	if len(eventsAfter) != 6 {
		t.Fatalf("Events after append returned %d rows, want 6", len(eventsAfter))
	}
	gotNote := eventsAfter[5]
	if gotNote.ID != 6 {
		t.Errorf("gotNote.ID = %d, want 6", gotNote.ID)
	}
	if gotNote.Type != EventLaneNote {
		t.Errorf("gotNote.Type = %q, want %q", gotNote.Type, EventLaneNote)
	}
	if gotNote.Detail != noteEvent.Detail {
		t.Errorf("gotNote.Detail = %q, want %q", gotNote.Detail, noteEvent.Detail)
	}
	if gotNote.LaneID != "lane-a" {
		t.Errorf("gotNote.LaneID = %q, want lane-a", gotNote.LaneID)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	l1, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	t1 := time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC)
	if err := l1.AppendEvent(ctx, Event{RunID: "run-1", LaneID: "lane-a", Type: EventLaneNote, Detail: "note 1", At: t1}); err != nil {
		t.Fatalf("l1 AppendEvent: %v", err)
	}

	var appliedAtV2 string
	if err := l1.db.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 2`).Scan(&appliedAtV2); err != nil {
		t.Fatalf("query version 2 applied_at: %v", err)
	}

	if err := l1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Reopen second time.
	l2, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("second Open on already-migrated database: %v", err)
	}

	var appliedAtV2Second string
	if err := l2.db.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 2`).Scan(&appliedAtV2Second); err != nil {
		t.Fatalf("query version 2 applied_at second time: %v", err)
	}
	if appliedAtV2Second != appliedAtV2 {
		t.Errorf("applied_at for version 2 changed on second Open: got %q, want %q", appliedAtV2Second, appliedAtV2)
	}

	// Append another event.
	t2 := time.Date(2026, 8, 19, 11, 5, 0, 0, time.UTC)
	if err := l2.AppendEvent(ctx, Event{RunID: "run-1", LaneID: "lane-a", Type: EventLaneNote, Detail: "note 2", At: t2}); err != nil {
		t.Fatalf("l2 AppendEvent: %v", err)
	}

	if err := l2.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// Reopen third time.
	l3, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("third Open on already-migrated database: %v", err)
	}
	defer l3.Close()

	events, err := l3.Events(ctx, "run-1")
	if err != nil {
		t.Fatalf("l3 Events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("Events returned %d rows, want 2", len(events))
	}
	if events[0].Detail != "note 1" || events[1].Detail != "note 2" {
		t.Errorf("Events details = [%q, %q], want [note 1, note 2]", events[0].Detail, events[1].Detail)
	}
}

func TestMigrateV2DatabaseCreatesApprovalsTableAndPreservesRows(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := ledgerpath.Resolve(root)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("create .lucind dir: %v", err)
	}

	dsn := "file:" + dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}

	// Apply schema v2 directly
	v2DDL := `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT;

CREATE TABLE IF NOT EXISTS lanes (
  run_id             TEXT    NOT NULL,
  lane_id            TEXT    NOT NULL,
  packet_id          TEXT    NOT NULL,
  executor           TEXT    NOT NULL CHECK (executor IN ('agy','cursor-agent','human')),
  routing_condition  TEXT    NOT NULL CHECK (length(trim(routing_condition)) > 0),
  status             TEXT    NOT NULL CHECK (status IN
                       ('pending','running','done','blocked','deviated','failed')),
  worktree_path      TEXT    NOT NULL DEFAULT '',
  worktree_preserved INTEGER NOT NULL DEFAULT 0 CHECK (worktree_preserved IN (0,1)),
  attempt            INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
  started_at         TEXT,
  ended_at           TEXT,
  PRIMARY KEY (run_id, lane_id)
) STRICT;

CREATE TABLE IF NOT EXISTS events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id  TEXT NOT NULL,
  lane_id TEXT,
  type    TEXT NOT NULL CHECK (type IN ('run_started','lane_registered',
            'lane_status_changed','lane_note','barrier_released','run_ended')),
  detail  TEXT NOT NULL DEFAULT '',
  at      TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id, id);
`
	if _, err := rawDB.ExecContext(ctx, v2DDL); err != nil {
		rawDB.Close()
		t.Fatalf("apply v2 schema DDL: %v", err)
	}

	v2AppliedAt := "2026-08-19T10:00:00Z"
	if _, err := rawDB.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?), (2, ?)`,
		v2AppliedAt, v2AppliedAt,
	); err != nil {
		rawDB.Close()
		t.Fatalf("record v2 migration: %v", err)
	}

	// Insert lane under v2.
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO lanes (run_id, lane_id, packet_id, executor, routing_condition, status, worktree_path, worktree_preserved, attempt)
		VALUES ('run-1', 'lane-a', 'p-a', 'agy', 'cond-a', 'running', '/path/a', 0, 1)
	`); err != nil {
		rawDB.Close()
		t.Fatalf("insert v2 lane: %v", err)
	}

	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw DB: %v", err)
	}

	// Open the v2 database through normal Open, triggering migration to v3.
	l, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("Open on v2 database: %v", err)
	}
	defer l.Close()

	// Verify schema_migrations has version 3
	var v3Count int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 3`).Scan(&v3Count); err != nil {
		t.Fatalf("query version 3: %v", err)
	}
	if v3Count != 1 {
		t.Errorf("version 3 count = %d, want 1", v3Count)
	}

	// Verify approvals table exists and defaults work
	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO approvals (run_id, lane_id, packet_id, decision, requested_at)
		VALUES ('run-1', 'lane-a', 'p-a', 'pending', '2026-08-19T10:00:00Z')
	`); err != nil {
		t.Fatalf("insert into approvals failed: %v", err)
	}

	var defectSurfacedLater int
	var approver string
	if err := l.db.QueryRowContext(ctx, `SELECT approver, defect_surfaced_later FROM approvals WHERE run_id = 'run-1' AND lane_id = 'lane-a'`).Scan(&approver, &defectSurfacedLater); err != nil {
		t.Fatalf("select from approvals: %v", err)
	}
	if approver != "" {
		t.Errorf("approver = %q, want empty string default", approver)
	}
	if defectSurfacedLater != 0 {
		t.Errorf("defect_surfaced_later = %d, want 0 default", defectSurfacedLater)
	}

	// Verify constraint on invalid decision
	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO approvals (run_id, lane_id, packet_id, decision, requested_at)
		VALUES ('run-1', 'lane-b', 'p-b', 'invalid_decision', '2026-08-19T10:00:00Z')
	`); err == nil {
		t.Errorf("expected error inserting invalid decision into approvals table, got nil")
	}

	// Assert lanes were preserved.
	lanes, err := l.Lanes(ctx, "run-1")
	if err != nil {
		t.Fatalf("Lanes(run-1): %v", err)
	}
	if len(lanes) != 1 || lanes[0].LaneID != "lane-a" {
		t.Fatalf("lanes not preserved properly: %+v", lanes)
	}
}

func TestLedgerApprovalLifecycleAndDecision(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	// Register a lane first
	if err := l.RegisterLane(ctx, Lane{
		RunID:            "run-1",
		LaneID:           "lane-1",
		PacketID:         "pkt-1",
		Executor:         "agy",
		RoutingCondition: "cond",
		Status:           lane.Running,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}

	reqAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	app := Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		Evidence:    "file.go:42",
		RequestedAt: reqAt,
	}

	if err := l.RequestApproval(ctx, app); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}

	// Verify read back
	got, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if got.Decision != DecisionPending {
		t.Errorf("got.Decision = %v, want %v", got.Decision, DecisionPending)
	}
	if got.Evidence != "file.go:42" {
		t.Errorf("got.Evidence = %q, want %q", got.Evidence, "file.go:42")
	}

	// Test WaitDecision unblocks on Decide
	doneCh := make(chan struct{})
	var waited Approval
	var waitErr error
	go func() {
		defer close(doneCh)
		waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		waited, waitErr = l.WaitDecision(waitCtx, "run-1", "lane-1")
	}()

	time.Sleep(50 * time.Millisecond)
	if err := l.Decide(ctx, "run-1", "lane-1", "alice", DecisionApproved); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	<-doneCh
	if waitErr != nil {
		t.Fatalf("WaitDecision failed: %v", waitErr)
	}
	if waited.Decision != DecisionApproved {
		t.Errorf("waited.Decision = %v, want %v", waited.Decision, DecisionApproved)
	}
	if waited.Approver != "alice" {
		t.Errorf("waited.Approver = %q, want alice", waited.Approver)
	}
	if waited.DecidedAt == nil {
		t.Errorf("waited.DecidedAt is nil, want timestamp")
	}
}

func TestLedgerWaitDecisionTimesOut(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	if err := l.RequestApproval(ctx, Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	app, err := l.WaitDecision(waitCtx, "run-1", "lane-1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitDecision err = %v, want context.DeadlineExceeded", err)
	}
	if app.Decision != DecisionTimedOut {
		t.Errorf("app.Decision = %v, want %v", app.Decision, DecisionTimedOut)
	}

	// Verify DB was updated to timed_out
	dbApp, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if dbApp.Decision != DecisionTimedOut {
		t.Errorf("dbApp.Decision = %v, want %v", dbApp.Decision, DecisionTimedOut)
	}
}

func TestLedgerApproverRateZeroDefectHistory(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	// User has 3 approvals, none flagged
	for i := 1; i <= 3; i++ {
		laneID := fmt.Sprintf("lane-%d", i)
		if err := l.RequestApproval(ctx, Approval{
			RunID:       "run-1",
			LaneID:      laneID,
			PacketID:    "pkt-1",
			RequestedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("RequestApproval: %v", err)
		}
		if err := l.Decide(ctx, "run-1", laneID, "alice", DecisionApproved); err != nil {
			t.Fatalf("Decide: %v", err)
		}
	}

	rate, err := l.ApproverRate(ctx, "alice")
	if err != nil {
		t.Fatalf("ApproverRate: %v", err)
	}
	if rate != 0.0 {
		t.Errorf("rate = %f, want 0.0", rate)
	}
}

func TestLedgerApproverRateOwnRateOnly(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	// Alice: 2 approvals, 1 defect
	// Bob: 2 approvals, 0 defect
	apps := []struct {
		laneID   string
		approver string
		defect   bool
	}{
		{"lane-a1", "alice", false},
		{"lane-a2", "alice", true},
		{"lane-b1", "bob", false},
		{"lane-b2", "bob", false},
	}

	for _, a := range apps {
		if err := l.RequestApproval(ctx, Approval{
			RunID:       "run-1",
			LaneID:      a.laneID,
			PacketID:    "pkt-1",
			RequestedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("RequestApproval: %v", err)
		}
		if err := l.Decide(ctx, "run-1", a.laneID, a.approver, DecisionApproved); err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if a.defect {
			if err := l.MarkDefectSurfaced(ctx, "run-1", a.laneID, true); err != nil {
				t.Fatalf("MarkDefectSurfaced: %v", err)
			}
		}
	}

	aliceRate, err := l.ApproverRate(ctx, "alice")
	if err != nil {
		t.Fatalf("ApproverRate(alice): %v", err)
	}
	if aliceRate != 0.5 {
		t.Errorf("aliceRate = %f, want 0.5 (50%%)", aliceRate)
	}

	bobRate, err := l.ApproverRate(ctx, "bob")
	if err != nil {
		t.Fatalf("ApproverRate(bob): %v", err)
	}
	if bobRate != 0.0 {
		t.Errorf("bobRate = %f, want 0.0", bobRate)
	}
}

func TestLedgerLaterFailureDoesNotAutoInferDefect(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	// Register lane 1 with packet-x, approve it
	if err := l.RegisterLane(ctx, Lane{
		RunID:            "run-1",
		LaneID:           "lane-1",
		PacketID:         "pkt-x",
		Executor:         "agy",
		RoutingCondition: "cond",
		Status:           lane.Done,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}
	if err := l.RequestApproval(ctx, Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-x",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if err := l.Decide(ctx, "run-1", "lane-1", "alice", DecisionApproved); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	// Later run with same packet fails
	if err := l.RegisterLane(ctx, Lane{
		RunID:            "run-2",
		LaneID:           "lane-2",
		PacketID:         "pkt-x",
		Executor:         "agy",
		RoutingCondition: "cond",
		Status:           lane.Running,
	}); err != nil {
		t.Fatalf("RegisterLane run-2: %v", err)
	}
	if err := l.SetStatus(ctx, "run-2", "lane-2", lane.Failed, time.Now().UTC()); err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}

	// Verify approval for lane-1 is still defect_surfaced_later = false
	app1, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if app1.DefectSurfacedLater {
		t.Errorf("DefectSurfacedLater was auto-inferred to true after later failure, want false")
	}

	// Explicit mark sets it
	if err := l.MarkDefectSurfaced(ctx, "run-1", "lane-1", true); err != nil {
		t.Fatalf("MarkDefectSurfaced: %v", err)
	}
	app1After, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval after mark: %v", err)
	}
	if !app1After.DefectSurfacedLater {
		t.Errorf("DefectSurfacedLater is false after MarkDefectSurfaced, want true")
	}
}

func TestLedgerDecideAlreadyDecidedReturnsError(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	if err := l.RequestApproval(ctx, Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}

	// First decision succeeds
	if err := l.Decide(ctx, "run-1", "lane-1", "alice", DecisionApproved); err != nil {
		t.Fatalf("First Decide failed: %v", err)
	}

	// Second decision must fail with ErrAlreadyDecided and not overwrite the stored decision
	err := l.Decide(ctx, "run-1", "lane-1", "bob", DecisionRejected)
	if !errors.Is(err, ErrAlreadyDecided) {
		t.Fatalf("Second Decide err = %v, want ErrAlreadyDecided", err)
	}

	// Verify the original decision and approver were preserved
	app, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if app.Decision != DecisionApproved {
		t.Errorf("app.Decision = %v, want %v", app.Decision, DecisionApproved)
	}
	if app.Approver != "alice" {
		t.Errorf("app.Approver = %q, want alice", app.Approver)
	}
}

func TestLedgerDecideNonexistentApprovalReturnsErrLaneUnknown(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	err := l.Decide(ctx, "nonexistent-run", "nonexistent-lane", "alice", DecisionApproved)
	if !errors.Is(err, ErrLaneUnknown) {
		t.Fatalf("Decide on nonexistent row err = %v, want ErrLaneUnknown", err)
	}
}

func TestFreshLedgerMigratesToV4WithAllSevenTables(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	var currentVersion int
	if err := l.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&currentVersion); err != nil {
		t.Fatalf("query schema_migrations max version: %v", err)
	}
	if currentVersion != 4 {
		t.Errorf("currentVersion = %d, want 4", currentVersion)
	}

	wantTables := []string{
		"features",
		"integration_attempts",
		"feature_leases",
		"overlap_evidence",
		"reconciliation_requests",
		"reconciliation_candidates",
		"integration_events",
	}

	for _, tbl := range wantTables {
		var name string
		err := l.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %q does not exist in fresh v4 ledger: %v", tbl, err)
		}
	}
}

func TestMigrateV3DatabasePreservesDataAndAdvancesToV4(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := ledgerpath.Resolve(root)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("create .lucind dir: %v", err)
	}

	dsn := "file:" + dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}

	// Apply v3 schema
	v3DDL := schemaMigrationsDDL + schemaDDL
	if _, err := rawDB.ExecContext(ctx, v3DDL); err != nil {
		rawDB.Close()
		t.Fatalf("apply v3 schema DDL: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := rawDB.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?), (2, ?), (3, ?)`,
		now, now, now,
	); err != nil {
		rawDB.Close()
		t.Fatalf("record v3 migrations: %v", err)
	}

	// Insert data into lanes, events, approvals under v3
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO lanes (run_id, lane_id, packet_id, executor, routing_condition, status, worktree_path, worktree_preserved, attempt)
		VALUES ('run-v3', 'lane-v3', 'p-v3', 'agy', 'cond-v3', 'done', '/path/v3', 1, 1)
	`); err != nil {
		rawDB.Close()
		t.Fatalf("insert v3 lane: %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO events (id, run_id, lane_id, type, detail, at)
		VALUES (1, 'run-v3', 'lane-v3', 'lane_status_changed', 'done', ?)
	`, now); err != nil {
		rawDB.Close()
		t.Fatalf("insert v3 event: %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO approvals (run_id, lane_id, packet_id, approver, decision, evidence, defect_surfaced_later, requested_at, decided_at)
		VALUES ('run-v3', 'lane-v3', 'p-v3', 'alice', 'approved', 'evidence.txt', 0, ?, ?)
	`, now, now); err != nil {
		rawDB.Close()
		t.Fatalf("insert v3 approval: %v", err)
	}

	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw DB: %v", err)
	}

	// Open the v3 database through normal Open, triggering migration to v4.
	l, err := Open(ctx, root)
	if err != nil {
		t.Fatalf("Open on v3 database: %v", err)
	}
	defer l.Close()

	// Verify schema_migrations reaches version 4
	var v4Count int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 4`).Scan(&v4Count); err != nil {
		t.Fatalf("query version 4: %v", err)
	}
	if v4Count != 1 {
		t.Errorf("version 4 count = %d, want 1", v4Count)
	}

	// Assert v3 data was preserved
	lanes, err := l.Lanes(ctx, "run-v3")
	if err != nil {
		t.Fatalf("Lanes: %v", err)
	}
	if len(lanes) != 1 || lanes[0].LaneID != "lane-v3" || lanes[0].Status != lane.Done {
		t.Fatalf("lanes altered by migration: %+v", lanes)
	}

	events, err := l.Events(ctx, "run-v3")
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 || events[0].Detail != "done" {
		t.Fatalf("events altered by migration: %+v", events)
	}

	app, err := l.Approval(ctx, "run-v3", "lane-v3")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if app.Decision != DecisionApproved || app.Approver != "alice" {
		t.Fatalf("approvals altered by migration: %+v", app)
	}

	// Verify all 7 new tables exist
	wantTables := []string{
		"features",
		"integration_attempts",
		"feature_leases",
		"overlap_evidence",
		"reconciliation_requests",
		"reconciliation_candidates",
		"integration_events",
	}
	for _, tbl := range wantTables {
		var name string
		err := l.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %q does not exist after migration: %v", tbl, err)
		}
	}

	// Verify idempotency: calling migrate again does nothing and creates no duplicate rows
	if err := migrate(ctx, l.db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var totalMigrations int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&totalMigrations); err != nil {
		t.Fatalf("query total migrations: %v", err)
	}
	if totalMigrations != 4 {
		t.Errorf("total migrations count after second migrate = %d, want 4", totalMigrations)
	}
}

func TestAtomicStateAndAuditAppendSuccessAndRollback(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)

	// 1. Successful atomic state + audit write
	err := l.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO features (id, parent_ref, base_sha, expected_parent_sha, status, created_at, updated_at)
			VALUES ('feat-1', 'refs/heads/feature-1', 'sha1', 'sha1', 'active', ?, ?)`,
			now.Format(time.RFC3339), now.Format(time.RFC3339),
		)
		return err
	}, IntegrationEvent{
		FeatureID: "feat-1",
		Type:      "feature_created",
		Detail:    "created feat-1",
		At:        now,
	})
	if err != nil {
		t.Fatalf("WriteWithAudit failed: %v", err)
	}

	// Verify feature exists
	var featStatus string
	if err := l.db.QueryRowContext(ctx, `SELECT status FROM features WHERE id = 'feat-1'`).Scan(&featStatus); err != nil {
		t.Fatalf("query feature: %v", err)
	}
	if featStatus != "active" {
		t.Errorf("featStatus = %q, want active", featStatus)
	}

	// Verify event exists
	events, err := l.IntegrationEvents(ctx, "feat-1")
	if err != nil {
		t.Fatalf("IntegrationEvents: %v", err)
	}
	if len(events) != 1 || events[0].Type != "feature_created" || events[0].Detail != "created feat-1" {
		t.Fatalf("unexpected events: %+v", events)
	}

	// 2. Failure rollback: inject failure on audit insert (e.g. NULL feature_id violates NOT NULL constraint)
	err = l.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO features (id, parent_ref, base_sha, expected_parent_sha, status, created_at, updated_at)
			VALUES ('feat-failed', 'refs/heads/feature-failed', 'sha2', 'sha2', 'active', ?, ?)`,
			now.Format(time.RFC3339), now.Format(time.RFC3339),
		)
		return err
	}, IntegrationEvent{
		FeatureID: "", // empty FeatureID violates NOT NULL constraint (or will be rejected)
		Type:      "", // empty Type violates NOT NULL / check
		At:        now,
	})
	if err == nil {
		t.Fatal("expected WriteWithAudit to fail on invalid event, got nil")
	}

	// Verify feat-failed was NOT written to features table (transaction was rolled back)
	var count int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM features WHERE id = 'feat-failed'`).Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 0 {
		t.Errorf("features table contains feat-failed row, expected rollback!")
	}
}

func TestPruneIntegrationEventsRetention(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	t1 := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)

	for _, evt := range []IntegrationEvent{
		{FeatureID: "feat-ret", Type: "event1", Detail: "d1", At: t1},
		{FeatureID: "feat-ret", Type: "event2", Detail: "d2", At: t2},
		{FeatureID: "feat-ret", Type: "event3", Detail: "d3", At: t3},
	} {
		if err := l.WriteWithAudit(ctx, nil, evt); err != nil {
			t.Fatalf("insert event: %v", err)
		}
	}

	eventsBefore, err := l.IntegrationEvents(ctx, "feat-ret")
	if err != nil {
		t.Fatalf("IntegrationEvents before prune: %v", err)
	}
	if len(eventsBefore) != 3 {
		t.Fatalf("expected 3 events before prune, got %d", len(eventsBefore))
	}

	// Prune with cutoff at 2026-08-16 (t1 and t2 should be deleted, t3 kept)
	cutoff := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	pruned, err := l.PruneIntegrationEvents(ctx, cutoff)
	if err != nil {
		t.Fatalf("PruneIntegrationEvents: %v", err)
	}
	if pruned != 2 {
		t.Errorf("pruned count = %d, want 2", pruned)
	}

	eventsAfter, err := l.IntegrationEvents(ctx, "feat-ret")
	if err != nil {
		t.Fatalf("IntegrationEvents after prune: %v", err)
	}
	if len(eventsAfter) != 1 {
		t.Fatalf("expected 1 event after prune, got %d", len(eventsAfter))
	}
	if eventsAfter[0].Type != "event3" || !eventsAfter[0].At.Equal(t3) {
		t.Errorf("surviving event = %+v, want event3 at %v", eventsAfter[0], t3)
	}
}

func TestLedgerOverlapEvidenceCRUD(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC)
	row := OverlapEvidenceRow{
		FeatureID:     "feat-1",
		Version:       "v1",
		EvidenceHash:  "hash123",
		EvidenceClass: "required",
		EvidenceJSON:  `{"class":"required"}`,
		CreatedAt:     now,
	}

	id, err := l.InsertOverlapEvidence(ctx, row)
	if err != nil {
		t.Fatalf("InsertOverlapEvidence: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	// Query by featureID and hash
	got, err := l.OverlapEvidence(ctx, "feat-1", "hash123")
	if err != nil {
		t.Fatalf("OverlapEvidence: %v", err)
	}
	if got.EvidenceHash != "hash123" || got.EvidenceClass != "required" || got.EvidenceJSON != row.EvidenceJSON {
		t.Errorf("got = %+v, want %+v", got, row)
	}

	// Query by hash
	gotByHash, err := l.OverlapEvidenceByHash(ctx, "hash123")
	if err != nil {
		t.Fatalf("OverlapEvidenceByHash: %v", err)
	}
	if gotByHash.ID != got.ID {
		t.Errorf("gotByHash.ID = %d, want %d", gotByHash.ID, got.ID)
	}

	// Query nonexistent
	_, err = l.OverlapEvidence(ctx, "feat-1", "nonexistent")
	if !errors.Is(err, ErrOverlapEvidenceNotFound) {
		t.Errorf("expected ErrOverlapEvidenceNotFound, got %v", err)
	}
	_, err = l.OverlapEvidenceByHash(ctx, "nonexistent")
	if !errors.Is(err, ErrOverlapEvidenceNotFound) {
		t.Errorf("expected ErrOverlapEvidenceNotFound, got %v", err)
	}
}

func TestLedgerReconciliationRequestCRUD(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	exp := now.Add(15 * time.Minute)
	req := ReconciliationRequestRow{
		ID:              "req-1",
		FeatureID:       "feat-target",
		Direction:       "feat-source -> feat-target",
		Status:          "awaiting",
		Actor:           "",
		EvidenceVersion: "v1",
		EvidenceHash:    "hash456",
		SourceSHA:       "sha-src",
		TargetSHA:       "sha-tgt",
		ExpiresAt:       &exp,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := l.InsertReconciliationRequest(ctx, req); err != nil {
		t.Fatalf("InsertReconciliationRequest: %v", err)
	}

	// Query by ID
	got, err := l.ReconciliationRequest(ctx, "req-1")
	if err != nil {
		t.Fatalf("ReconciliationRequest: %v", err)
	}
	if got.ID != "req-1" || got.Status != "awaiting" || got.SourceSHA != "sha-src" || got.TargetSHA != "sha-tgt" {
		t.Errorf("got = %+v, want %+v", got, req)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) {
		t.Errorf("got.ExpiresAt = %v, want %v", got.ExpiresAt, exp)
	}

	// List by FeatureID
	list, err := l.ReconciliationRequests(ctx, "feat-target")
	if err != nil {
		t.Fatalf("ReconciliationRequests: %v", err)
	}
	if len(list) != 1 || list[0].ID != "req-1" {
		t.Fatalf("ReconciliationRequests returned %+v, want 1 item", list)
	}

	// Update mutable fields
	got.Status = "approved"
	got.Actor = "local:tester"
	got.UpdatedAt = now.Add(1 * time.Minute)
	if err := l.UpdateReconciliationRequest(ctx, got); err != nil {
		t.Fatalf("UpdateReconciliationRequest: %v", err)
	}

	updated, err := l.ReconciliationRequest(ctx, "req-1")
	if err != nil {
		t.Fatalf("ReconciliationRequest after update: %v", err)
	}
	if updated.Status != "approved" || updated.Actor != "local:tester" {
		t.Errorf("updated = %+v, want approved and actor local:tester", updated)
	}

	// Query nonexistent
	_, err = l.ReconciliationRequest(ctx, "nonexistent")
	if !errors.Is(err, ErrReconciliationRequestNotFound) {
		t.Errorf("expected ErrReconciliationRequestNotFound, got %v", err)
	}

	// Update nonexistent
	err = l.UpdateReconciliationRequest(ctx, ReconciliationRequestRow{ID: "nonexistent"})
	if !errors.Is(err, ErrReconciliationRequestNotFound) {
		t.Errorf("expected ErrReconciliationRequestNotFound, got %v", err)
	}
}

func TestLedgerReconciliationCandidateCRUD(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 13, 0, 0, 0, time.UTC)
	cand := ReconciliationCandidateRow{
		ID:            "cand-1",
		RequestID:     "req-1",
		Status:        "candidate_running",
		AllowedPaths:  "pkg/a.go,pkg/b.go",
		Model:         "sonnet",
		Config:        "",
		Output:        "",
		Checks:        "",
		CandidateSHA:  "",
		FailureReason: "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := l.InsertReconciliationCandidate(ctx, cand); err != nil {
		t.Fatalf("InsertReconciliationCandidate: %v", err)
	}

	// Query by ID
	got, err := l.ReconciliationCandidate(ctx, "cand-1")
	if err != nil {
		t.Fatalf("ReconciliationCandidate: %v", err)
	}
	if got.ID != "cand-1" || got.Status != "candidate_running" || got.Model != "sonnet" {
		t.Errorf("got = %+v, want %+v", got, cand)
	}

	// Query by RequestID
	gotByReq, err := l.ReconciliationCandidateByRequest(ctx, "req-1")
	if err != nil {
		t.Fatalf("ReconciliationCandidateByRequest: %v", err)
	}
	if gotByReq.ID != "cand-1" {
		t.Errorf("gotByReq.ID = %q, want cand-1", gotByReq.ID)
	}

	// List by RequestID
	list, err := l.ReconciliationCandidates(ctx, "req-1")
	if err != nil {
		t.Fatalf("ReconciliationCandidates: %v", err)
	}
	if len(list) != 1 || list[0].ID != "cand-1" {
		t.Fatalf("ReconciliationCandidates returned %+v, want 1 item", list)
	}

	// Update status
	got.Status = "integrated"
	got.CandidateSHA = "sha-integrated"
	got.UpdatedAt = now.Add(2 * time.Minute)
	if err := l.UpdateReconciliationCandidate(ctx, got); err != nil {
		t.Fatalf("UpdateReconciliationCandidate: %v", err)
	}

	updated, err := l.ReconciliationCandidate(ctx, "cand-1")
	if err != nil {
		t.Fatalf("ReconciliationCandidate after update: %v", err)
	}
	if updated.Status != "integrated" || updated.CandidateSHA != "sha-integrated" {
		t.Errorf("updated = %+v, want integrated with sha", updated)
	}

	// Query nonexistent
	_, err = l.ReconciliationCandidate(ctx, "nonexistent")
	if !errors.Is(err, ErrReconciliationCandidateNotFound) {
		t.Errorf("expected ErrReconciliationCandidateNotFound, got %v", err)
	}
	_, err = l.ReconciliationCandidateByRequest(ctx, "nonexistent")
	if !errors.Is(err, ErrReconciliationCandidateNotFound) {
		t.Errorf("expected ErrReconciliationCandidateNotFound, got %v", err)
	}

	// Update nonexistent
	err = l.UpdateReconciliationCandidate(ctx, ReconciliationCandidateRow{ID: "nonexistent"})
	if !errors.Is(err, ErrReconciliationCandidateNotFound) {
		t.Errorf("expected ErrReconciliationCandidateNotFound, got %v", err)
	}
}
