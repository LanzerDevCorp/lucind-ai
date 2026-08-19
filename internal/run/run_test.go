package run_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// fakeExecutor is a hand-written test double: it records the last Request
// it received and returns a pre-programmed Outcome, so tests can drive
// every branch of Execute's dispatch-decision logic without spawning a
// real process.
type fakeExecutor struct {
	outcome executor.Outcome
	err     error
	gotReq  executor.Request
	// beforeReturn, when set, runs just before Run returns. Tests use it
	// to inject a side effect (e.g. closing the ledger) at the exact
	// point in Execute's flow where the dispatch would normally succeed,
	// to reach failure paths that only exist after RegisterLane and the
	// dispatch have both already gone through.
	beforeReturn func()
}

func (f *fakeExecutor) Run(_ context.Context, req executor.Request) (executor.Outcome, error) {
	f.gotReq = req
	if f.beforeReturn != nil {
		f.beforeReturn()
	}
	return f.outcome, f.err
}

// doneEnvelopeJSON is a minimal envelope that satisfies result.schema.json
// with status "done".
const doneEnvelopeJSON = `{
	"packet_id": "lane-a",
	"status": "done",
	"summary": "did the thing",
	"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
}`

// testPacket returns a valid Packet for lane "lane-a" with executor "agy",
// which matches the ledger schema's CHECK constraint on the executor
// column.
func testPacket() packet.Packet {
	return packet.Packet{ID: "lane-a", Executor: "agy", RoutedBy: "touches config, Tier A audit mandatory", Body: "do the thing"}
}

// newTestDeps builds a run.Deps wired to a real on-disk ledger (never
// faked — the point of this package's tests is proving the real ledger is
// wired), a stubbed CreateWorktree that never touches git, and a pinned
// clock. wtPath is a real temp directory standing in for the lane's
// worktree, since Execute writes the result schema to real disk there.
func newTestDeps(t *testing.T, wtPath string, fsys func(string) fs.FS, exec executor.Executor) run.Deps {
	t.Helper()

	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	return run.Deps{
		RunID:       "run-1",
		PrimaryRoot: "/primary",
		Ledger:      l,
		Executor:    exec,
		CreateWorktree: func(_ context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: wtPath, Branch: "lucind/" + laneID}, nil
		},
		WorktreeFS: fsys,
		Now:        func() time.Time { return now },
	}
}

func TestExecuteHappyPathEnvelopeDoneReachesLaneDone(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}
	if report.Envelope == nil {
		t.Fatal("report.Envelope = nil, want a populated envelope")
	}

	// The ledger, not the Report, is the source of truth: query it back
	// directly rather than trusting what Execute returned.
	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].LaneID != "lane-a" || states[0].Status != lane.Done {
		t.Errorf("LaneStates() = %+v, want one lane-a=done", states)
	}
}

// blockedEnvelopeJSON is a minimal envelope satisfying result.schema.json
// with status "blocked": a fired hard stop, and the required note on it.
const blockedEnvelopeJSON = `{
	"packet_id": "lane-a",
	"status": "blocked",
	"summary": "hit a hard stop",
	"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": true, "note": "would have had to edit it"}]
}`

func TestExecuteEnvelopeBlockedReachesLaneBlocked(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(blockedEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Blocked {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Blocked {
		t.Errorf("LaneStates() = %+v, want one lane-a=blocked", states)
	}
}

// readCountingFS wraps an fs.FS and records how many times Open was
// called, so tests can prove an envelope was never even attempted to be
// read, not merely that its content was discarded.
type readCountingFS struct {
	fs.FS
	opens *int
}

func (r readCountingFS) Open(name string) (fs.File, error) {
	*r.opens++
	return r.FS.Open(name)
}

func TestExecuteTimedOutDispatchNeverReadsEnvelope(t *testing.T) {
	wtPath := t.TempDir()
	// The fake executor is timed out but still carries a "done" envelope
	// in its backing worktree fs, to prove Execute ignores it entirely
	// rather than merely happening not to use it.
	fe := &fakeExecutor{outcome: executor.Outcome{TimedOut: true}}
	opens := 0
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return readCountingFS{
			FS:    fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}},
			opens: &opens,
		}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Blocked {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}
	if report.Envelope != nil {
		t.Errorf("report.Envelope = %+v, want nil", report.Envelope)
	}
	if opens != 0 {
		t.Errorf("envelope filesystem was opened %d time(s), want 0 — a timed-out dispatch must never read the envelope", opens)
	}
}

func TestExecuteExitZeroMissingEnvelopeIsBlockedNotDone(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{} // no .lucind/result.json at all
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil (a missing envelope is a successful Execute call ending blocked)", err)
	}

	if report.Status != lane.Blocked {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}
	if report.Envelope != nil {
		t.Errorf("report.Envelope = %+v, want nil", report.Envelope)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Blocked {
		t.Errorf("LaneStates() = %+v, want one lane-a=blocked", states)
	}

	// The unreadable-envelope reason must survive somewhere durable, since
	// Report has no field for it: check the ledger events.
	events, err := deps.Ledger.Events(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	found := false
	for _, e := range events {
		if e.Type == ledger.EventLaneNote && strings.Contains(e.Detail, "result envelope could not be read") {
			found = true
		}
	}
	if !found {
		t.Errorf("no ledger event recorded the unreadable-envelope reason; events = %+v", events)
	}
}

func TestExecuteExitZeroSchemaInvalidEnvelopeIsBlockedNotDone(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	// "status" is outside the schema's enum ("succeeded" is not one of
	// done/blocked/deviated/failed), so this fails schema validation
	// despite being well-formed JSON.
	invalid := `{"packet_id": "lane-a", "status": "succeeded", "summary": "x", "hard_stops": []}`
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(invalid)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil (schema-invalid envelope is a successful Execute call ending blocked)", err)
	}

	if report.Status != lane.Blocked {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}
	if report.Envelope != nil {
		t.Errorf("report.Envelope = %+v, want nil", report.Envelope)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Blocked {
		t.Errorf("LaneStates() = %+v, want one lane-a=blocked", states)
	}
}

func TestExecuteWritesResultSchemaIntoWorktree(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	got, err := os.ReadFile(filepath.Join(wtPath, ".lucind", "result.schema.json"))
	if err != nil {
		t.Fatalf("reading .lucind/result.schema.json: %v", err)
	}
	if want := result.SchemaJSON(); !bytes.Equal(got, want) {
		t.Errorf("schema bytes written to worktree do not match result.SchemaJSON()\ngot:  %s\nwant: %s", got, want)
	}
}

func TestExecuteRequestCarriesPacketBodyAndWorktreeAndSchemaPaths(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	p := testPacket()
	if _, err := run.Execute(context.Background(), deps, p); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if fe.gotReq.Prompt != p.Body {
		t.Errorf("gotReq.Prompt = %q, want %q", fe.gotReq.Prompt, p.Body)
	}
	if fe.gotReq.WorktreePath != wtPath {
		t.Errorf("gotReq.WorktreePath = %q, want %q", fe.gotReq.WorktreePath, wtPath)
	}
	wantSchemaPath := filepath.Join(wtPath, ".lucind", "result.schema.json")
	if fe.gotReq.SchemaPath != wantSchemaPath {
		t.Errorf("gotReq.SchemaPath = %q, want %q", fe.gotReq.SchemaPath, wantSchemaPath)
	}
}

// TestExecuteAppendsLifecycleLedgerEvents proves Execute's own lifecycle
// events land in the ledger. It does not check for a barrier_released
// event: Execute does not own a barrier at all (see ExecuteBatch, in
// batch.go) -- that event is ExecuteBatch's responsibility now, not
// Execute's.
func TestExecuteAppendsLifecycleLedgerEvents(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	events, err := deps.Ledger.Events(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}

	var sawRegistered, sawStatusChanged bool
	for _, e := range events {
		if e.LaneID != "lane-a" {
			t.Errorf("event %+v carries LaneID %q, want lane-a for every event this run produces", e, e.LaneID)
		}
		switch e.Type {
		case ledger.EventLaneRegistered:
			sawRegistered = true
		case ledger.EventLaneStatusChanged:
			sawStatusChanged = true
		}
	}
	if !sawRegistered {
		t.Errorf("no %s event recorded; events = %+v", ledger.EventLaneRegistered, events)
	}
	if !sawStatusChanged {
		t.Errorf("no %s event recorded; events = %+v", ledger.EventLaneStatusChanged, events)
	}
}

// TestExecuteDispatchErrorLeavesLaneFailedInLedger proves the defect found
// by the binary's first real end-to-end dispatch: once RegisterLane has
// succeeded, an error from the executor (a real infrastructure failure --
// the process never ran at all, per executor.Agy.Run's contract) must not
// leave the lane stuck at lane.Running forever. It must be persisted as
// lane.Failed in the real ledger before Execute returns its error, checked
// here by reading the ledger back rather than trusting the returned error.
func TestExecuteDispatchErrorLeavesLaneFailedInLedger(t *testing.T) {
	wtPath := t.TempDir()
	wantErr := errors.New("exec: \"agy-stub\": executable file not found in $PATH")
	fe := &fakeExecutor{err: wantErr}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	_, err := run.Execute(context.Background(), deps, testPacket())
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil when the executor fails to dispatch")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want it to wrap %v", err, wantErr)
	}

	states, statesErr := deps.Ledger.LaneStates(context.Background(), "run-1")
	if statesErr != nil {
		t.Fatalf("LaneStates() error = %v", statesErr)
	}
	if len(states) != 1 || states[0].LaneID != "lane-a" || states[0].Status != lane.Failed {
		t.Fatalf("LaneStates() = %+v, want one lane-a=failed", states)
	}

	events, eventsErr := deps.Ledger.Events(context.Background(), "run-1")
	if eventsErr != nil {
		t.Fatalf("Events() error = %v", eventsErr)
	}
	found := false
	for _, e := range events {
		if e.Type == ledger.EventLaneNote && strings.Contains(e.Detail, wantErr.Error()) {
			found = true
		}
	}
	if !found {
		t.Errorf("no ledger event recorded the dispatch failure reason %q; events = %+v", wantErr.Error(), events)
	}
}

// TestExecuteTerminalStatusWriteFailureStillReturnsOriginalCause covers a
// failure at a different point after registration than the dispatch error
// above: here the dispatch itself succeeds and decideStatus picks a real
// terminal status (lane.Done), but the ledger is closed (simulating a
// broken connection) at the exact moment Execute is about to persist that
// status. recordLaneFailure's own fallback SetStatus call fails too, since
// the ledger is still closed -- proving the original cause survives rather
// than being replaced by the secondary write failure, per the hard
// constraint that the original error must never be lost.
func TestExecuteTerminalStatusWriteFailureStillReturnsOriginalCause(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	// Close the ledger as a side effect of the dispatch succeeding, so
	// RegisterLane and the running-status write have already gone
	// through, but the terminal SetStatus call that follows decideStatus
	// hits a closed database -- a failure point strictly after
	// registration and strictly different from the executor returning an
	// error.
	fe.beforeReturn = func() { deps.Ledger.Close() }

	_, err := run.Execute(context.Background(), deps, testPacket())
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil when persisting the terminal status fails")
	}
	if !strings.Contains(err.Error(), "terminal status") {
		t.Errorf("Execute() error = %v, want it to mention the terminal-status write that actually failed", err)
	}
}

// TestExecuteReturnsErrorWhenWorktreeCreationFails proves the distinction
// the whole flow rests on: a lane that ran and ended blocked/deviated/
// failed is a *successful* Execute call with a Report (see the tests
// above), but a failure that stops the flow before a lane state even
// exists — worktree creation failing — is a real Execute error, and no
// Report is meaningful for it.
func TestExecuteReturnsErrorWhenWorktreeCreationFails(t *testing.T) {
	wantErr := errors.New("git worktree add: boom")
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, t.TempDir(), func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.CreateWorktree = func(context.Context, string, string) (worktree.Worktree, error) {
		return worktree.Worktree{}, wantErr
	}

	_, err := run.Execute(context.Background(), deps, testPacket())
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil when worktree creation fails")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want it to wrap %v", err, wantErr)
	}

	// Worktree creation never happened, so no lane row should exist at all.
	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 0 {
		t.Errorf("LaneStates() = %+v, want none", states)
	}
}

// TestExecuteRegistersLaneWithRoutedByAsRoutingCondition proves the ledger
// records the packet's actual routing condition, not the executor's name
// standing in for one — recording the executor as its own routing reason
// is the implicit routing the packet format now forbids by requiring
// routed_by. It reads the fact back from the real ledger via Lanes(...)
// rather than trusting the struct Execute was called with.
func TestExecuteRegistersLaneWithRoutedByAsRoutingCondition(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	p := testPacket()
	if _, err := run.Execute(context.Background(), deps, p); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	lanes, err := deps.Ledger.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	if len(lanes) != 1 {
		t.Fatalf("Lanes() = %+v, want exactly one lane", lanes)
	}
	if lanes[0].RoutingCondition != p.RoutedBy {
		t.Errorf("lanes[0].RoutingCondition = %q, want %q (the packet's routed_by, not its executor)", lanes[0].RoutingCondition, p.RoutedBy)
	}
	if lanes[0].RoutingCondition == p.Executor {
		t.Errorf("lanes[0].RoutingCondition = %q equals the executor name %q — that is implicit routing", lanes[0].RoutingCondition, p.Executor)
	}
}

// TestExecuteRequestCarriesExplicitModel proves a packet naming a model
// gets that exact model on the dispatched executor.Request.
func TestExecuteRequestCarriesExplicitModel(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	p := testPacket()
	p.Model = "gemini-3.7-flash-lite"
	if _, err := run.Execute(context.Background(), deps, p); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if fe.gotReq.Model != "gemini-3.7-flash-lite" {
		t.Errorf("gotReq.Model = %q, want %q", fe.gotReq.Model, "gemini-3.7-flash-lite")
	}
}

// TestExecuteRequestFallsBackToDefaultModelWhenPacketOmitsIt proves a
// packet that names no model still dispatches with the project default,
// applied here at the composition root rather than inside packet.Parse.
func TestExecuteRequestFallsBackToDefaultModelWhenPacketOmitsIt(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	p := testPacket()
	p.Model = ""
	if _, err := run.Execute(context.Background(), deps, p); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if fe.gotReq.Model != run.DefaultModel {
		t.Errorf("gotReq.Model = %q, want run.DefaultModel = %q", fe.gotReq.Model, run.DefaultModel)
	}
}

// outputTruncatedDetailSubstring matches on the ledger event detail text
// run.go records for a truncated outcome, without depending on the
// unexported constant that holds it (this is an external run_test
// package). It is deliberately a fragment of the sentence, not the whole
// thing, so the test does not become a brittle exact-string match on
// wording.
const outputTruncatedDetailSubstring = "captured stdout/stderr may be incomplete"

// TestExecuteTruncatedOutcomeStillYieldsDoneAndReportsCapture proves
// truncation is purely a diagnosis concern: a dispatch outcome with
// executor.Outcome.OutputTruncated set, paired with a valid "done"
// envelope, still reaches lane.Done -- truncation never changes the
// lane's status -- and the Report carries the truncation flag through so
// a caller downstream of Execute can act on it.
func TestExecuteTruncatedOutcomeStillYieldsDoneAndReportsCapture(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0, OutputTruncated: true}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v (truncation must not change lane status)", report.Status, lane.Done)
	}
	if !report.OutputCaptureIncomplete {
		t.Errorf("report.OutputCaptureIncomplete = false, want true")
	}
}

// TestExecuteTruncatedOutcomeAppendsLedgerEvent proves the truncation fact
// survives in the ledger's own record, not only in the Report the caller
// happened to receive once. It reads the events back from the real ledger
// rather than trusting Report.
func TestExecuteTruncatedOutcomeAppendsLedgerEvent(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0, OutputTruncated: true}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	events, err := deps.Ledger.Events(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	found := false
	for _, e := range events {
		if e.Type == ledger.EventLaneNote && strings.Contains(e.Detail, outputTruncatedDetailSubstring) {
			found = true
		}
	}
	if !found {
		t.Errorf("no ledger event recorded the output-truncated fact; events = %+v", events)
	}
}

// TestExecuteNonTruncatedOutcomeAppendsNoTruncationEvent proves the
// truncation event is conditional: an ordinary, fully-drained outcome must
// not leave a stray truncation note in the ledger.
func TestExecuteNonTruncatedOutcomeAppendsNoTruncationEvent(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0, OutputTruncated: false}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.OutputCaptureIncomplete {
		t.Errorf("report.OutputCaptureIncomplete = true, want false")
	}

	events, err := deps.Ledger.Events(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	for _, e := range events {
		if strings.Contains(e.Detail, outputTruncatedDetailSubstring) {
			t.Errorf("unexpected output-truncated event for a non-truncated outcome; events = %+v", events)
		}
	}
}

// laneNoteDetails returns the Detail of every EventLaneNote event recorded
// for runID, in the real ledger -- the tests below assert against the
// ledger's own record, never against Report, per this package's
// established style of never trusting the in-memory struct on its own.
func laneNoteDetails(t *testing.T, l *ledger.Ledger, runID string) []string {
	t.Helper()

	events, err := l.Events(context.Background(), runID)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}

	var details []string
	for _, e := range events {
		if e.Type == ledger.EventLaneNote {
			details = append(details, e.Detail)
		}
	}
	return details
}

// anyContains reports whether any of details contains substr.
func anyContains(details []string, substr string) bool {
	for _, d := range details {
		if strings.Contains(d, substr) {
			return true
		}
	}
	return false
}

// TestExecuteNonZeroExitLedgerNoteCarriesExitCodeAndStderr proves the
// long-standing undiagnosable-failure gap is closed: a lane that exits
// non-zero must have its captured stderr land in the same ledger note as
// the exit code, not just "dispatch exited %d" on its own -- the exit
// code alone was exactly the gap that made run a7f7b87f-96af-454f-9e68-
// 5ce8f0932fc8's readme-stale lane failure undiagnosable.
func TestExecuteNonZeroExitLedgerNoteCarriesExitCodeAndStderr(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 1, Stderr: "panic: something went wrong at the very end"}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Blocked {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "dispatch exited 1") {
		t.Errorf("ledger notes = %+v, want one containing %q", details, "dispatch exited 1")
	}
	if !anyContains(details, "panic: something went wrong at the very end") {
		t.Errorf("ledger notes = %+v, want one containing the captured stderr", details)
	}
}

// TestExecuteTimedOutLedgerNoteCarriesStderr mirrors the non-zero-exit
// case above for the timeout path: a dispatch killed on its ceiling still
// has whatever it managed to write to stderr before it was killed, and
// that must reach the ledger note too, not just "dispatch timed out" on
// its own.
func TestExecuteTimedOutLedgerNoteCarriesStderr(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{TimedOut: true, Stderr: "still working on step 3 of 9 when killed"}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Blocked {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "dispatch timed out") {
		t.Errorf("ledger notes = %+v, want one containing %q", details, "dispatch timed out")
	}
	if !anyContains(details, "still working on step 3 of 9 when killed") {
		t.Errorf("ledger notes = %+v, want one containing the captured stderr", details)
	}
}

// TestExecuteUnreadableEnvelopeLedgerNoteCarriesStderr covers the third
// failing path: exit 0, but the envelope could not be read at all. The
// dispatch still ran a real process that may have written useful stderr
// before giving up on writing a valid envelope, so that stderr must reach
// the ledger note alongside the ErrEnvelopeUnreadable reason.
func TestExecuteUnreadableEnvelopeLedgerNoteCarriesStderr(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0, Stderr: "error: refused to write result.json, disk full"}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{} // no .lucind/result.json at all
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Blocked {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "result envelope could not be read") {
		t.Errorf("ledger notes = %+v, want one containing the envelope-unreadable reason", details)
	}
	if !anyContains(details, "error: refused to write result.json, disk full") {
		t.Errorf("ledger notes = %+v, want one containing the captured stderr", details)
	}
}

// TestExecuteOversizedStderrLedgerNoteIsBoundedAndKeepsTail proves the
// cap: a dispatch that wrote megabytes to stderr must not blow up the
// ledger row, and what survives must be the TAIL of the capture, not the
// head -- a process explains why it died in its last output, not its
// first. headMarker sits at the very start of the fake stderr and must be
// dropped; tailMarker sits at the very end and must survive, alongside a
// visible truncation notice.
func TestExecuteOversizedStderrLedgerNoteIsBoundedAndKeepsTail(t *testing.T) {
	const headMarker = "HEAD-MARKER-MUST-BE-DROPPED"
	const tailMarker = "TAIL-MARKER-MUST-SURVIVE"

	filler := strings.Repeat("x", 8192)
	stderr := headMarker + filler + tailMarker

	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 1, Stderr: stderr}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	var note string
	for _, d := range details {
		if strings.Contains(d, "dispatch exited 1") {
			note = d
		}
	}
	if note == "" {
		t.Fatalf("ledger notes = %+v, want one containing %q", details, "dispatch exited 1")
	}

	if len(note) >= len(stderr) {
		t.Errorf("recorded note length = %d, want it bounded well below the raw stderr length %d", len(note), len(stderr))
	}
	if strings.Contains(note, headMarker) {
		t.Errorf("recorded note = %q, want the head of stderr dropped, not kept", note)
	}
	if !strings.Contains(note, tailMarker) {
		t.Errorf("recorded note = %q, want the tail of stderr kept", note)
	}
	if !strings.Contains(strings.ToLower(note), "truncat") {
		t.Errorf("recorded note = %q, want it visibly marked as truncated", note)
	}
}

// TestExecuteBothStreamsEmptyLedgerNoteSaysSoNotTruncated proves an empty
// capture on BOTH streams is reported plainly, per stream, rather than
// accidentally matching the truncation wording -- a reader must never
// mistake "nothing was captured" for "something was captured and then
// clipped." This is exactly the shape a caller cannot distinguish from a
// truncation bug unless the two are worded unambiguously differently.
func TestExecuteBothStreamsEmptyLedgerNoteSaysSoNotTruncated(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 1, Stderr: "", Stdout: ""}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	var note string
	for _, d := range details {
		if strings.Contains(d, "dispatch exited 1") {
			note = d
		}
	}
	if note == "" {
		t.Fatalf("ledger notes = %+v, want one containing %q", details, "dispatch exited 1")
	}
	if !strings.Contains(note, "stderr: ") || !strings.Contains(note, "stdout: ") {
		t.Errorf("recorded note = %q, want both streams labelled distinctly", note)
	}
	if strings.Contains(strings.ToLower(note), "truncat") {
		t.Errorf("recorded note = %q, want an empty capture on both streams never mistaken for a truncated one", note)
	}
}

// TestExecuteStderrEmptyStdoutCarriesFailureJSON is THE real case: it
// reproduces the incident that motivated this whole change. Running two
// real agy processes concurrently with the production flag set showed
// stderr at 0 bytes on both, while the failure was reported as complete
// JSON on stdout (e.g. {"status":"ERROR","error":"timeout waiting for
// response",...}). A diagnosis that only ever looked at stderr would have
// recorded nothing at all for this exact incident -- this test is the one
// that proves the gap is actually closed, not just stderr's plumbing.
func TestExecuteStderrEmptyStdoutCarriesFailureJSON(t *testing.T) {
	const failureJSON = `{"status":"ERROR","error":"timeout waiting for response","response":"","duration_seconds":84.5}`

	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 1, Stderr: "", Stdout: failureJSON}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "timeout waiting for response") {
		t.Errorf("ledger notes = %+v, want one containing the failure JSON captured on stdout", details)
	}
	if !strings.Contains(report.Diagnosis, "timeout waiting for response") {
		t.Errorf("report.Diagnosis = %q, want it to contain the failure JSON captured on stdout", report.Diagnosis)
	}
}

// TestExecuteBothStreamsPopulatedAreDistinctlyLabelled proves that when
// both stderr and stdout carry content, both survive into the ledger
// note, each under its own label, rather than one silently overwriting or
// being concatenated indistinguishably with the other.
func TestExecuteBothStreamsPopulatedAreDistinctlyLabelled(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{
		ExitCode: 1,
		Stderr:   "STDERR-ONLY-CONTENT",
		Stdout:   "STDOUT-ONLY-CONTENT",
	}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	var note string
	for _, d := range details {
		if strings.Contains(d, "dispatch exited 1") {
			note = d
		}
	}
	if note == "" {
		t.Fatalf("ledger notes = %+v, want one containing %q", details, "dispatch exited 1")
	}
	if !strings.Contains(note, "STDERR-ONLY-CONTENT") {
		t.Errorf("recorded note = %q, want it to contain the stderr content", note)
	}
	if !strings.Contains(note, "STDOUT-ONLY-CONTENT") {
		t.Errorf("recorded note = %q, want it to contain the stdout content", note)
	}
	stderrLabelIdx := strings.Index(note, "stderr: STDERR-ONLY-CONTENT")
	stdoutLabelIdx := strings.Index(note, "stdout: STDOUT-ONLY-CONTENT")
	if stderrLabelIdx == -1 || stdoutLabelIdx == -1 {
		t.Errorf("recorded note = %q, want each stream's content directly attached to its own label", note)
	}
}

// TestExecuteOversizedStdoutLedgerNoteIsBoundedAndKeepsTail mirrors
// TestExecuteOversizedStderrLedgerNoteIsBoundedAndKeepsTail for stdout,
// since agy is the observed case where the diagnosis lives on stdout, not
// stderr -- the cap and tail-keeping rule must apply to it too, not only
// to stderr.
func TestExecuteOversizedStdoutLedgerNoteIsBoundedAndKeepsTail(t *testing.T) {
	const headMarker = "STDOUT-HEAD-MARKER-MUST-BE-DROPPED"
	const tailMarker = "STDOUT-TAIL-MARKER-MUST-SURVIVE"

	filler := strings.Repeat("y", 8192)
	stdout := headMarker + filler + tailMarker

	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 1, Stdout: stdout}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	var note string
	for _, d := range details {
		if strings.Contains(d, "dispatch exited 1") {
			note = d
		}
	}
	if note == "" {
		t.Fatalf("ledger notes = %+v, want one containing %q", details, "dispatch exited 1")
	}

	if len(note) >= len(stdout) {
		t.Errorf("recorded note length = %d, want it bounded well below the raw stdout length %d", len(note), len(stdout))
	}
	if strings.Contains(note, headMarker) {
		t.Errorf("recorded note = %q, want the head of stdout dropped, not kept", note)
	}
	if !strings.Contains(note, tailMarker) {
		t.Errorf("recorded note = %q, want the tail of stdout kept", note)
	}
	if !strings.Contains(strings.ToLower(note), "truncat") {
		t.Errorf("recorded note = %q, want it visibly marked as truncated", note)
	}
}

// TestExecuteReportCarriesDiagnosisForEachFailingPath proves Report.
// Diagnosis is populated for all three non-success dispatch paths, so a
// caller that never touches the ledger (e.g. cmd/lucind-ai's printReport)
// can still see why a lane failed.
func TestExecuteReportCarriesDiagnosisForEachFailingPath(t *testing.T) {
	tests := []struct {
		name    string
		outcome executor.Outcome
		fsys    func(string) fs.FS
		want    string
	}{
		{
			name:    "non-zero exit",
			outcome: executor.Outcome{ExitCode: 7, Stderr: "boom-nonzero"},
			fsys:    func(string) fs.FS { return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}} },
			want:    "boom-nonzero",
		},
		{
			name:    "timed out",
			outcome: executor.Outcome{TimedOut: true, Stderr: "boom-timeout"},
			fsys:    func(string) fs.FS { return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}} },
			want:    "boom-timeout",
		},
		{
			name:    "unreadable envelope",
			outcome: executor.Outcome{ExitCode: 0, Stderr: "boom-envelope"},
			fsys:    func(string) fs.FS { return fstest.MapFS{} },
			want:    "boom-envelope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtPath := t.TempDir()
			fe := &fakeExecutor{outcome: tt.outcome}
			deps := newTestDeps(t, wtPath, tt.fsys, fe)

			report, err := run.Execute(context.Background(), deps, testPacket())
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}
			if !strings.Contains(report.Diagnosis, tt.want) {
				t.Errorf("report.Diagnosis = %q, want it to contain %q", report.Diagnosis, tt.want)
			}
		})
	}
}

// TestExecuteSuccessfulLaneReportsNoDiagnosisOrLedgerNote proves the
// opposite side of the contract: a lane that reaches a terminal status
// from a readable envelope must not carry a stray diagnosis, in the
// Report or in the ledger -- diagnosis is reserved for the three
// non-success dispatch paths, never a normal envelope-decided outcome.
func TestExecuteSuccessfulLaneReportsNoDiagnosisOrLedgerNote(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0, Stderr: "some incidental stderr chatter"}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Done {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Done)
	}
	if report.Diagnosis != "" {
		t.Errorf("report.Diagnosis = %q, want empty for a successful lane", report.Diagnosis)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if anyContains(details, "some incidental stderr chatter") {
		t.Errorf("ledger notes = %+v, want no note carrying the incidental stderr for a successful lane", details)
	}
}
