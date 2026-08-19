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
