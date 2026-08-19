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

func TestExecuteHappyPathEnvelopeDoneReleasesBarrierAndIntegrates(t *testing.T) {
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
	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}
	if report.Envelope == nil {
		t.Fatal("report.Envelope = nil, want a populated envelope")
	}
	if got := len(report.Outcome.Integrate); got != 1 || report.Outcome.Integrate[0] != "lane-a" {
		t.Errorf("report.Outcome.Integrate = %v, want [lane-a]", report.Outcome.Integrate)
	}
	if len(report.Outcome.Preserve) != 0 {
		t.Errorf("report.Outcome.Preserve = %v, want empty", report.Outcome.Preserve)
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

func TestExecuteEnvelopeBlockedIsPreservedNotIntegrated(t *testing.T) {
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
	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}
	if len(report.Outcome.Integrate) != 0 {
		t.Errorf("report.Outcome.Integrate = %v, want empty", report.Outcome.Integrate)
	}
	if got := len(report.Outcome.Preserve); got != 1 || report.Outcome.Preserve[0] != "lane-a" {
		t.Errorf("report.Outcome.Preserve = %v, want [lane-a]", report.Outcome.Preserve)
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

	var sawRegistered, sawStatusChanged, sawBarrierReleased bool
	for _, e := range events {
		if e.LaneID != "lane-a" {
			t.Errorf("event %+v carries LaneID %q, want lane-a for every event this run produces", e, e.LaneID)
		}
		switch e.Type {
		case ledger.EventLaneRegistered:
			sawRegistered = true
		case ledger.EventLaneStatusChanged:
			sawStatusChanged = true
		case ledger.EventBarrierReleased:
			sawBarrierReleased = true
		}
	}
	if !sawRegistered {
		t.Errorf("no %s event recorded; events = %+v", ledger.EventLaneRegistered, events)
	}
	if !sawStatusChanged {
		t.Errorf("no %s event recorded; events = %+v", ledger.EventLaneStatusChanged, events)
	}
	if !sawBarrierReleased {
		t.Errorf("no %s event recorded; events = %+v", ledger.EventBarrierReleased, events)
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
