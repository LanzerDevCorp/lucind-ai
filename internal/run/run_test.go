package run_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	outcome      executor.Outcome
	err          error
	gotReq       executor.Request
	defaultModel string
	// beforeReturn, when set, runs just before Run returns. Tests use it
	// to inject a side effect (e.g. closing the ledger) at the exact
	// point in Execute's flow where the dispatch would normally succeed,
	// to reach failure paths that only exist after RegisterLane and the
	// dispatch have both already gone through.
	beforeReturn func()
}

type progressExecutor struct {
	run func(context.Context, executor.Request) (executor.Outcome, error)
}

func (e progressExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	return e.run(ctx, req)
}

func (progressExecutor) DefaultModel() string  { return "test-model" }
func (progressExecutor) KnownModels() []string { return []string{"test-model"} }

func (f *fakeExecutor) Run(_ context.Context, req executor.Request) (executor.Outcome, error) {
	f.gotReq = req
	if f.beforeReturn != nil {
		f.beforeReturn()
	}
	return f.outcome, f.err
}

func (f *fakeExecutor) DefaultModel() string {
	return f.defaultModel
}

func (f *fakeExecutor) KnownModels() []string {
	return []string{f.defaultModel}
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
	return packet.Packet{
		ID:                "lane-a",
		Executor:          "agy",
		RoutedBy:          "touches config, Tier A audit mandatory",
		Feature:           "feat-lane-a",
		ParentRef:         "refs/heads/main",
		BaseSHA:           "b000000000000000000000000000000000000000",
		ExpectedParentSHA: "b000000000000000000000000000000000000000",
		Body:              "do the thing",
		Model:             "test-model",
		Agent:             "test-agent",
		SDDPhase:          "apply",
		FanoutGroup:       "ledger",
		Skill:             "lucind-apply",
		Path:              ".lucind/packets/lane-a.md",
	}
}

// TestExecuteUpdatesLaneMetadataAfterRegisterLane proves Execute snapshots
// packet dispatch context (including Skill and PacketPath) via
// UpdateLaneMetadata immediately after RegisterLane.
func TestExecuteUpdatesLaneMetadataAfterRegisterLane(t *testing.T) {
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		return executor.Outcome{ExitCode: 0}, nil
	}}
	wtPath := t.TempDir()
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
	}, exec)

	p := testPacket()
	p.AllowedPaths = []string{"internal/run/run.go"}
	if _, err := run.Execute(context.Background(), deps, p); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	got, err := deps.Ledger.GetLaneMetadata(context.Background(), deps.RunID, p.ID)
	if err != nil {
		t.Fatalf("GetLaneMetadata() error = %v", err)
	}
	if got.Model != p.Model || got.Agent != p.Agent || got.Feature != p.Feature {
		t.Fatalf("metadata model/agent/feature = (%q,%q,%q), want (%q,%q,%q)",
			got.Model, got.Agent, got.Feature, p.Model, p.Agent, p.Feature)
	}
	if got.SDDPhase != p.SDDPhase || got.FanoutGroup != p.FanoutGroup {
		t.Fatalf("metadata sdd_phase/fanout_group = (%q,%q), want (%q,%q)",
			got.SDDPhase, got.FanoutGroup, p.SDDPhase, p.FanoutGroup)
	}
	if got.Skill != p.Skill || got.PacketPath != p.Path {
		t.Fatalf("metadata skill/packet_path = (%q,%q), want (%q,%q)",
			got.Skill, got.PacketPath, p.Skill, p.Path)
	}
	if !reflect.DeepEqual(got.AllowedPaths, p.AllowedPaths) {
		t.Fatalf("metadata AllowedPaths = %v, want %v", got.AllowedPaths, p.AllowedPaths)
	}
}

// newTestDeps builds a run.Deps wired to a real on-disk ledger (never
// faked — the point of this package's tests is proving the real ledger is
// wired), a stubbed CreateWorktree that never touches git, and a pinned
// clock. wtPath is a real temp directory standing in for the lane's
// worktree, since Execute writes the result schema to real disk there.
func newTestDeps(t *testing.T, wtPath string, fsys func(string) fs.FS, exec executor.Executor, baseSHA ...string) run.Deps {
	t.Helper()

	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	sha := ""
	if len(baseSHA) > 0 {
		sha = baseSHA[0]
	}

	return run.Deps{
		RunID:       "run-1",
		PrimaryRoot: "/primary",
		Ledger:      l,
		LookupExecutor: func(string) (executor.Executor, error) {
			return exec, nil
		},
		CreateWorktree: func(_ context.Context, primaryRoot, laneID, _, _ string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: wtPath, Branch: "lucind/" + laneID, BaseSHA: sha}, nil
		},
		WorktreeFS: fsys,
		Now:        func() time.Time { return now },
		HasUniqueLaneCommits: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
		PorcelainEmpty: func(context.Context, string) (bool, error) {
			return true, nil
		},
		PersistEnvelope: func(context.Context, string, string, *result.Envelope) error {
			return nil
		},
	}
}

func TestExecuteProgressWriterFlushesAtBatchSize(t *testing.T) {
	const eventCount = 32
	flushed := make(chan []ledger.LaneProgress, 1)
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		for i := 0; i < eventCount; i++ {
			req.Progress <- executor.ProgressEvent{Message: fmt.Sprintf("event-%02d", i), At: time.Unix(int64(i+1), 0)}
		}
		select {
		case batch := <-flushed:
			if len(batch) != eventCount {
				t.Errorf("first progress batch length = %d, want %d", len(batch), eventCount)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("progress batch did not flush at 32 events while the executor was still running")
		}
		return executor.Outcome{ExitCode: 0}, nil
	}}

	wtPath := t.TempDir()
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
	}, exec)
	deps.AppendProgressBatch = func(ctx context.Context, batch []ledger.LaneProgress) error {
		copyOfBatch := append([]ledger.LaneProgress(nil), batch...)
		flushed <- copyOfBatch
		return deps.Ledger.AppendProgressBatch(ctx, batch)
	}

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	got, err := deps.Ledger.GetProgressAfter(context.Background(), deps.RunID, "lane-a", 0)
	if err != nil {
		t.Fatalf("GetProgressAfter() error = %v", err)
	}
	if len(got) != eventCount {
		t.Fatalf("persisted progress count = %d, want %d", len(got), eventCount)
	}
}

// TestExecuteProgressWriterForwardsTelemetryFields proves writeLaneProgress
// copies TotalTokens/CostUSD/ToolCalls from ProgressEvent onto LaneProgress
// so decoder telemetry survives the ledger round-trip.
func TestExecuteProgressWriterForwardsTelemetryFields(t *testing.T) {
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		req.Progress <- executor.ProgressEvent{
			Message: "usage", At: time.Unix(1, 0),
			TotalTokens: 23459, CostUSD: 0.12, ToolCalls: 3,
		}
		return executor.Outcome{ExitCode: 0}, nil
	}}

	wtPath := t.TempDir()
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
	}, exec)

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	got, err := deps.Ledger.GetProgressAfter(context.Background(), deps.RunID, "lane-a", 0)
	if err != nil {
		t.Fatalf("GetProgressAfter() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("persisted progress count = %d, want 1", len(got))
	}
	if got[0].TotalTokens != 23459 || got[0].CostUSD != 0.12 || got[0].ToolCalls != 3 {
		t.Fatalf("telemetry = tokens %d cost %v tools %d, want 23459 / 0.12 / 3",
			got[0].TotalTokens, got[0].CostUSD, got[0].ToolCalls)
	}
}

func TestExecuteProgressWriterFlushesOnInterval(t *testing.T) {
	flushed := make(chan []ledger.LaneProgress, 1)
	started := time.Now()
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		req.Progress <- executor.ProgressEvent{Message: "waiting", At: time.Unix(1, 0)}
		select {
		case batch := <-flushed:
			if len(batch) != 1 || batch[0].Message != "waiting" {
				t.Errorf("interval batch = %+v, want one waiting event", batch)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("progress batch did not flush on the 250 ms interval while the executor was still running")
		}
		return executor.Outcome{ExitCode: 0}, nil
	}}

	wtPath := t.TempDir()
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
	}, exec)
	deps.AppendProgressBatch = func(ctx context.Context, batch []ledger.LaneProgress) error {
		flushed <- append([]ledger.LaneProgress(nil), batch...)
		return deps.Ledger.AppendProgressBatch(ctx, batch)
	}

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if elapsed := time.Since(started); elapsed < 200*time.Millisecond {
		t.Fatalf("interval flush completed after %v, want approximately 250 ms rather than an immediate flush", elapsed)
	}
}

func TestExecuteProgressWriterFlushesOnShutdown(t *testing.T) {
	const eventCount = 3
	flushed := make(chan []ledger.LaneProgress, 1)
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		for i := 0; i < eventCount; i++ {
			req.Progress <- executor.ProgressEvent{Message: fmt.Sprintf("final-%d", i), At: time.Unix(int64(i+1), 0)}
		}
		return executor.Outcome{ExitCode: 0}, nil
	}}

	wtPath := t.TempDir()
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
	}, exec)
	deps.AppendProgressBatch = func(ctx context.Context, batch []ledger.LaneProgress) error {
		flushed <- append([]ledger.LaneProgress(nil), batch...)
		return deps.Ledger.AppendProgressBatch(ctx, batch)
	}

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	select {
	case batch := <-flushed:
		if len(batch) != eventCount {
			t.Fatalf("shutdown batch length = %d, want %d", len(batch), eventCount)
		}
	default:
		t.Fatal("executor shutdown did not flush the partial progress batch")
	}
}

func TestExecuteBatchProgressWritersKeepConcurrentLanesIsolated(t *testing.T) {
	const eventsPerLane = 40
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		laneID := filepath.Base(req.WorktreePath)
		for i := 0; i < eventsPerLane; i++ {
			req.Progress <- executor.ProgressEvent{Message: fmt.Sprintf("%s-%02d", laneID, i), At: time.Unix(int64(i+1), 0)}
		}
		return executor.Outcome{ExitCode: 0}, nil
	}}

	root := t.TempDir()
	deps := newTestDeps(t, filepath.Join(root, "unused"), func(path string) fs.FS {
		laneID := filepath.Base(path)
		envelope := fmt.Sprintf(`{"packet_id":%q,"status":"done","summary":"done","hard_stops":[]}`, laneID)
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(envelope)}}
	}, exec)
	deps.CreateWorktree = func(_ context.Context, _, laneID, _, _ string) (worktree.Worktree, error) {
		path := filepath.Join(root, laneID)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return worktree.Worktree{}, err
		}
		return worktree.Worktree{Path: path, Branch: "lucind/" + laneID}, nil
	}

	pA := testPacket()
	pB := testPacket()
	pB.ID = "lane-b"
	pB.Feature = "feat-lane-b"
	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{pA, pB})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v", err)
	}
	if !report.Released {
		t.Fatalf("ExecuteBatch().Released = false, want true; reports = %+v", report.Lanes)
	}
	for _, laneID := range []string{"lane-a", "lane-b"} {
		got, err := deps.Ledger.GetProgressAfter(context.Background(), deps.RunID, laneID, 0)
		if err != nil {
			t.Fatalf("GetProgressAfter(%q) error = %v", laneID, err)
		}
		if len(got) != eventsPerLane {
			t.Fatalf("progress count for %s = %d, want %d", laneID, len(got), eventsPerLane)
		}
		for _, event := range got {
			if !strings.HasPrefix(event.Message, laneID+"-") {
				t.Fatalf("progress for %s contains another lane's message %q", laneID, event.Message)
			}
		}
	}
}

func TestExecuteProgressProducerDropsWritesWhenChannelIsFull(t *testing.T) {
	const overflowEvents = 100
	appendStarted := make(chan struct{})
	releaseAppend := make(chan struct{})
	accepted, dropped := 0, 0
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		for i := 0; i < 32; i++ {
			req.Progress <- executor.ProgressEvent{Message: fmt.Sprintf("initial-%02d", i), At: time.Unix(int64(i+1), 0)}
		}
		select {
		case <-appendStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("progress writer did not begin the threshold flush")
		}
		for i := 0; i < overflowEvents; i++ {
			event := executor.ProgressEvent{Message: fmt.Sprintf("overflow-%03d", i), At: time.Unix(int64(i+100), 0)}
			select {
			case req.Progress <- event:
				accepted++
			default:
				dropped++
			}
		}
		close(releaseAppend)
		return executor.Outcome{ExitCode: 0}, nil
	}}

	wtPath := t.TempDir()
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
	}, exec)
	firstAppend := true
	deps.AppendProgressBatch = func(ctx context.Context, batch []ledger.LaneProgress) error {
		if firstAppend {
			firstAppend = false
			close(appendStarted)
			<-releaseAppend
		}
		return deps.Ledger.AppendProgressBatch(ctx, batch)
	}

	if _, err := run.Execute(context.Background(), deps, testPacket()); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if accepted != 32 || dropped != overflowEvents-32 {
		t.Fatalf("non-blocking overflow writes accepted=%d dropped=%d, want accepted=32 dropped=%d", accepted, dropped, overflowEvents-32)
	}
}

func TestExecuteProgressInsertErrorIsObservableWithoutChangingLaneStatus(t *testing.T) {
	wantErr := errors.New("forced progress insert failure")
	exec := progressExecutor{run: func(_ context.Context, req executor.Request) (executor.Outcome, error) {
		req.Progress <- executor.ProgressEvent{Message: "will fail", At: time.Unix(1, 0)}
		return executor.Outcome{ExitCode: 0}, nil
	}}

	wtPath := t.TempDir()
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
	}, exec)
	deps.AppendProgressBatch = func(context.Context, []ledger.LaneProgress) error { return wantErr }

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Done {
		t.Fatalf("report.Status = %v, want %v despite progress insert failure", report.Status, lane.Done)
	}
	if !strings.Contains(report.Diagnosis, wantErr.Error()) {
		t.Errorf("report.Diagnosis = %q, want progress insert error %q", report.Diagnosis, wantErr)
	}
	if details := laneNoteDetails(t, deps.Ledger, deps.RunID); !anyContains(details, wantErr.Error()) {
		t.Errorf("lane notes = %+v, want progress insert error %q", details, wantErr)
	}
	states, err := deps.Ledger.LaneStates(context.Background(), deps.RunID)
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Done {
		t.Fatalf("LaneStates() = %+v, want lane-a=done", states)
	}
}

func resultEnvelopePathForTest() string { return ".lucind/result.json" }

func TestExecuteHappyPathEnvelopeDoneReachesLaneDone(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	p := testPacket()
	if len(p.AllowedPaths) != 0 {
		t.Fatalf("testPacket().AllowedPaths = %v, want empty (omitted) allowed_paths", p.AllowedPaths)
	}

	report, err := run.Execute(context.Background(), deps, p)
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

// TestExecuteLookupExecutorErrorLeavesLaneFailedInLedger proves that when
// LookupExecutor returns an error (e.g. unknown executor name), Execute
// records that lane as lane.Failed in the ledger with a diagnostic note and
// does not panic or leave the lane unregistered or stuck in running.
func TestExecuteLookupExecutorErrorLeavesLaneFailedInLedger(t *testing.T) {
	wtPath := t.TempDir()
	wantErr := errors.New("unsupported executor: bogus")
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{}
	}, &fakeExecutor{})
	deps.LookupExecutor = func(name string) (executor.Executor, error) {
		return nil, wantErr
	}

	_, err := run.Execute(context.Background(), deps, testPacket())
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil when LookupExecutor returns an error")
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
			break
		}
	}
	if !found {
		t.Errorf("Events() = %+v, want a lane_note containing %q", events, wantErr.Error())
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
	deps.CreateWorktree = func(context.Context, string, string, string, string) (worktree.Worktree, error) {
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
// packet that names no model still dispatches with the resolved executor's
// own default, applied here at the composition root rather than inside
// packet.Parse.
func TestExecuteRequestFallsBackToDefaultModelWhenPacketOmitsIt(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{
		outcome:      executor.Outcome{ExitCode: 0},
		defaultModel: "stub-executor-default",
	}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)

	p := testPacket()
	p.Model = ""
	if _, err := run.Execute(context.Background(), deps, p); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if fe.gotReq.Model != "stub-executor-default" {
		t.Errorf("gotReq.Model = %q, want %q", fe.gotReq.Model, "stub-executor-default")
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

func TestExecuteWriteDoneWithoutUniqueCommitsFails(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	doneWithCommitJSON := `{
		"packet_id": "lane-a",
		"status": "done",
		"summary": "claimed done with commit",
		"commit": "deadbeef12345678",
		"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
	}`
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneWithCommitJSON)}}
	}, fe)
	deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
		return false, nil
	}

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Failed {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Failed)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].LaneID != "lane-a" || states[0].Status != lane.Failed {
		t.Errorf("LaneStates() = %+v, want one lane-a=failed", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "commit") {
		t.Errorf("ledger notes = %+v, want a note naming the missing commit", details)
	}
}

func TestExecuteWriteDoneWithDirtyWorktreeFails(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
		return true, nil
	}
	deps.PorcelainEmpty = func(context.Context, string) (bool, error) {
		return false, nil
	}

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Failed {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Failed)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].LaneID != "lane-a" || states[0].Status != lane.Failed {
		t.Errorf("LaneStates() = %+v, want one lane-a=failed", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "uncommitted") && !anyContains(details, "dirty") {
		t.Errorf("ledger notes = %+v, want a note naming uncommitted/dirty changes", details)
	}
}

func TestExecuteReadOnlyDoneWithoutCommitsAndCleanTreeReachesDone(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
		return false, nil
	}
	deps.PorcelainEmpty = func(context.Context, string) (bool, error) {
		return true, nil
	}

	p := testPacket()
	p.ReadOnly = true

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].LaneID != "lane-a" || states[0].Status != lane.Done {
		t.Errorf("LaneStates() = %+v, want one lane-a=done", states)
	}
	if report.Diagnosis != "" {
		t.Errorf("report.Diagnosis = %q, want empty", report.Diagnosis)
	}
}

func TestExecuteReadOnlyDoneWithUniqueCommitsFails(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
		return true, nil
	}
	deps.PorcelainEmpty = func(context.Context, string) (bool, error) {
		return true, nil
	}

	p := testPacket()
	p.ReadOnly = true

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Failed {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Failed)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].LaneID != "lane-a" || states[0].Status != lane.Failed {
		t.Errorf("LaneStates() = %+v, want one lane-a=failed", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "commit") {
		t.Errorf("ledger notes = %+v, want a note naming unique commits", details)
	}
}

func TestExecuteReadOnlyDoneWithDirtyWorktreeFails(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
		return false, nil
	}
	deps.PorcelainEmpty = func(context.Context, string) (bool, error) {
		return false, nil
	}

	p := testPacket()
	p.ReadOnly = true

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Failed {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Failed)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].LaneID != "lane-a" || states[0].Status != lane.Failed {
		t.Errorf("LaneStates() = %+v, want one lane-a=failed", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "uncommitted") && !anyContains(details, "dirty") {
		t.Errorf("ledger notes = %+v, want a note naming uncommitted/dirty changes", details)
	}
}

func TestExecuteNonDoneStatusBypassesGitInspection(t *testing.T) {
	deviatedJSON := `{
		"packet_id": "lane-a",
		"status": "deviated",
		"summary": "deviated from plan",
		"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}],
		"deviations": [{"expected": "plan a", "actual": "plan b", "reason": "needed"}]
	}`
	failedJSON := `{
		"packet_id": "lane-a",
		"status": "failed",
		"summary": "technical failure",
		"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
	}`

	tests := []struct {
		name       string
		outcome    executor.Outcome
		envelope   string
		wantStatus lane.Status
	}{
		{
			name:       "blocked envelope",
			outcome:    executor.Outcome{ExitCode: 0},
			envelope:   blockedEnvelopeJSON,
			wantStatus: lane.Blocked,
		},
		{
			name:       "deviated envelope",
			outcome:    executor.Outcome{ExitCode: 0},
			envelope:   deviatedJSON,
			wantStatus: lane.Deviated,
		},
		{
			name:       "failed envelope",
			outcome:    executor.Outcome{ExitCode: 0},
			envelope:   failedJSON,
			wantStatus: lane.Failed,
		},
		{
			name:       "non-zero exit",
			outcome:    executor.Outcome{ExitCode: 1, Stderr: "syntax error"},
			envelope:   "",
			wantStatus: lane.Blocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtPath := t.TempDir()
			fe := &fakeExecutor{outcome: tt.outcome}
			var commitsCalls, porcelainCalls int
			deps := newTestDeps(t, wtPath, func(string) fs.FS {
				if tt.envelope == "" {
					return fstest.MapFS{}
				}
				return fstest.MapFS{".lucind/result.json": {Data: []byte(tt.envelope)}}
			}, fe)
			deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
				commitsCalls++
				return true, nil
			}
			deps.PorcelainEmpty = func(context.Context, string) (bool, error) {
				porcelainCalls++
				return true, nil
			}

			report, err := run.Execute(context.Background(), deps, testPacket())
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}

			if report.Status != tt.wantStatus {
				t.Errorf("report.Status = %v, want %v", report.Status, tt.wantStatus)
			}
			if commitsCalls != 0 {
				t.Errorf("HasUniqueLaneCommits called %d times, want 0", commitsCalls)
			}
			if porcelainCalls != 0 {
				t.Errorf("PorcelainEmpty called %d times, want 0", porcelainCalls)
			}
		})
	}
}

func TestExecuteGitInspectionErrorFailsLaneWithLedgerNote(t *testing.T) {
	t.Run("HasUniqueLaneCommits error", func(t *testing.T) {
		wtPath := t.TempDir()
		fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
		deps := newTestDeps(t, wtPath, func(string) fs.FS {
			return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
		}, fe)
		deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
			return false, errors.New("git merge-base failed: bad object")
		}

		report, err := run.Execute(context.Background(), deps, testPacket())
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}

		if report.Status != lane.Failed {
			t.Errorf("report.Status = %v, want %v (not Blocked)", report.Status, lane.Failed)
		}

		states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("LaneStates() error = %v", err)
		}
		if len(states) != 1 || states[0].Status != lane.Failed {
			t.Errorf("LaneStates() = %+v, want lane.Failed", states)
		}

		details := laneNoteDetails(t, deps.Ledger, "run-1")
		if !anyContains(details, "git merge-base failed: bad object") {
			t.Errorf("ledger notes = %+v, want note with error message", details)
		}
	})

	t.Run("PorcelainEmpty error", func(t *testing.T) {
		wtPath := t.TempDir()
		fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
		deps := newTestDeps(t, wtPath, func(string) fs.FS {
			return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
		}, fe)
		deps.HasUniqueLaneCommits = func(context.Context, string, string) (bool, error) {
			return true, nil
		}
		deps.PorcelainEmpty = func(context.Context, string) (bool, error) {
			return false, errors.New("git status failed: permission denied")
		}

		report, err := run.Execute(context.Background(), deps, testPacket())
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}

		if report.Status != lane.Failed {
			t.Errorf("report.Status = %v, want %v (not Blocked)", report.Status, lane.Failed)
		}

		states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("LaneStates() error = %v", err)
		}
		if len(states) != 1 || states[0].Status != lane.Failed {
			t.Errorf("LaneStates() = %+v, want lane.Failed", states)
		}

		details := laneNoteDetails(t, deps.Ledger, "run-1")
		if !anyContains(details, "git status failed: permission denied") {
			t.Errorf("ledger notes = %+v, want note with error message", details)
		}
	})
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "seed commit")

	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{
		"-c", "user.email=run-test@example.com",
		"-c", "user.name=run-test",
	}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
}

func setupGitWorktree(t *testing.T, primaryRoot, laneID string) (string, string) {
	t.Helper()
	wtPath := t.TempDir()
	runGit(t, primaryRoot, "worktree", "add", "-b", "lucind/"+laneID, wtPath)
	cmd := exec.Command("git", "-C", wtPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD error = %v", err)
	}
	baseSHA := strings.TrimSpace(string(out))
	return wtPath, baseSHA
}

func gitRevParse(t *testing.T, dir, rev string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", rev)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s error = %v", rev, err)
	}
	return strings.TrimSpace(string(out))
}

func TestExecuteScopeCheckInScopeChangesReachesDone(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Create in-scope files (committed, unstaged, and untracked)
	ledgerDir := filepath.Join(wtPath, "internal", "ledger")
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(ledgerDir, "committed.go"), []byte("package ledger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/ledger/committed.go")
	runGit(t, wtPath, "commit", "-m", "add committed.go")

	if err := os.WriteFile(filepath.Join(ledgerDir, "unstaged.go"), []byte("package ledger\n// unstaged\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/ledger/unstaged.go")
	if err := os.WriteFile(filepath.Join(ledgerDir, "unstaged.go"), []byte("package ledger\n// unstaged modified\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(ledgerDir, "untracked.go"), []byte("package ledger\n// untracked\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	wt, err := deps.CreateWorktree(context.Background(), primaryRoot, "lane-a", "", "")
	if err != nil {
		t.Fatalf("CreateWorktree error = %v", err)
	}
	if wt.BaseSHA != birthSHA {
		t.Fatalf("CreateWorktree().BaseSHA = %q, want %q", wt.BaseSHA, birthSHA)
	}

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Done {
		t.Errorf("LaneStates() = %+v, want lane.Done", states)
	}
}

func TestExecuteScopeCheckDemotesOutOfScopeTrackedFileToDeviated(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Create and commit an out-of-scope tracked file
	serveDir := filepath.Join(wtPath, "internal", "serve")
	if err := os.MkdirAll(serveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDir, "server.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/serve/server.go")
	runGit(t, wtPath, "commit", "-m", "add server.go")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Deviated {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Deviated)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Deviated {
		t.Errorf("LaneStates() = %+v, want lane.Deviated", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "internal/serve/server.go") {
		t.Errorf("ledger notes = %+v, want note naming internal/serve/server.go", details)
	}
}

func TestExecuteScopeCheckDemotesOutOfScopeUntrackedFileToDeviated(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Create an untracked out-of-scope file
	serveDir := filepath.Join(wtPath, "internal", "serve")
	if err := os.MkdirAll(serveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDir, "server.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Deviated {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Deviated)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Deviated {
		t.Errorf("LaneStates() = %+v, want lane.Deviated", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "internal/serve/server.go") {
		t.Errorf("ledger notes = %+v, want note naming internal/serve/server.go", details)
	}
}

func TestExecuteScopeCheckZeroCommitsUntrackedInScopeReachesDone(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// 0 unique commits on lane branch; create in-scope untracked file
	ledgerDir := filepath.Join(wtPath, "internal", "ledger")
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(ledgerDir, "untracked.go"), []byte("package ledger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Done {
		t.Errorf("LaneStates() = %+v, want lane.Done", states)
	}
}

func TestExecuteScopeCheckTwoCommitsEarlierOutOfScopeDemotesToDeviated(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Commit 1: touches out-of-scope internal/serve/server.go
	serveDir := filepath.Join(wtPath, "internal", "serve")
	if err := os.MkdirAll(serveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDir, "server.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/serve/server.go")
	runGit(t, wtPath, "commit", "-m", "commit 1 out of scope")

	// Commit 2: touches in-scope internal/ledger/ledger.go
	ledgerDir := filepath.Join(wtPath, "internal", "ledger")
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(ledgerDir, "ledger.go"), []byte("package ledger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/ledger/ledger.go")
	runGit(t, wtPath, "commit", "-m", "commit 2 in scope")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Deviated {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Deviated)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Deviated {
		t.Errorf("LaneStates() = %+v, want lane.Deviated", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "internal/serve/server.go") {
		t.Errorf("ledger notes = %+v, want note naming internal/serve/server.go", details)
	}
}

func TestExecuteScopeCheckMultipleInScopeCommitsReachesDone(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	ledgerDir := filepath.Join(wtPath, "internal", "ledger")
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	// Commit 1: in-scope
	if err := os.WriteFile(filepath.Join(ledgerDir, "first.go"), []byte("package ledger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/ledger/first.go")
	runGit(t, wtPath, "commit", "-m", "commit 1 in scope")

	// Commit 2: in-scope
	if err := os.WriteFile(filepath.Join(ledgerDir, "second.go"), []byte("package ledger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/ledger/second.go")
	runGit(t, wtPath, "commit", "-m", "commit 2 in scope")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Done {
		t.Errorf("LaneStates() = %+v, want lane.Done", states)
	}
}

func TestExecuteScopeCheckBlockedAndFailedEnvelopesNeverRewrittenToDeviated(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	tests := []struct {
		name       string
		envelope   string
		wantStatus lane.Status
	}{
		{
			name:       "blocked envelope",
			envelope:   blockedEnvelopeJSON,
			wantStatus: lane.Blocked,
		},
		{
			name: "failed envelope",
			envelope: `{
				"packet_id": "lane-a",
				"status": "failed",
				"summary": "something crashed",
				"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
			}`,
			wantStatus: lane.Failed,
		},
		{
			name: "deviated envelope",
			envelope: `{
				"packet_id": "lane-a",
				"status": "deviated",
				"summary": "deliberate deviation",
				"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}],
				"deviations": [{"expected": "plan a", "actual": "plan b", "reason": "needed"}]
			}`,
			wantStatus: lane.Deviated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primaryRoot := initGitRepo(t)
			wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

			// Create an out-of-scope tracked file
			serveDir := filepath.Join(wtPath, "internal", "serve")
			if err := os.MkdirAll(serveDir, 0o755); err != nil {
				t.Fatalf("MkdirAll error = %v", err)
			}
			if err := os.WriteFile(filepath.Join(serveDir, "server.go"), []byte("package serve\n"), 0o644); err != nil {
				t.Fatalf("WriteFile error = %v", err)
			}
			runGit(t, wtPath, "add", "internal/serve/server.go")
			runGit(t, wtPath, "commit", "-m", "out-of-scope commit")

			fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
			deps := newTestDeps(t, wtPath, func(string) fs.FS {
				return fstest.MapFS{".lucind/result.json": {Data: []byte(tt.envelope)}}
			}, fe, birthSHA)
			deps.PrimaryRoot = primaryRoot

			p := testPacket()
			p.AllowedPaths = []string{"internal/ledger/"}

			report, err := run.Execute(context.Background(), deps, p)
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}

			if report.Status != tt.wantStatus {
				t.Errorf("report.Status = %v, want %v", report.Status, tt.wantStatus)
			}

			states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
			if err != nil {
				t.Fatalf("LaneStates() error = %v", err)
			}
			if len(states) != 1 || states[0].Status != tt.wantStatus {
				t.Errorf("LaneStates() = %+v, want %v", states, tt.wantStatus)
			}
		})
	}
}

func TestExecuteScopeCheckForceAddedLucindFileExcludedFromUnion(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Force-add and commit .lucind/result.json
	lucindPath := filepath.Join(wtPath, ".lucind")
	if err := os.MkdirAll(lucindPath, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(doneEnvelopeJSON), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "-f", ".lucind/result.json")
	runGit(t, wtPath, "commit", "-m", "force add .lucind/result.json")

	// Also stage-only a .lucind/ note file without committing it
	if err := os.WriteFile(filepath.Join(lucindPath, "extra.json"), []byte(`{"note":"staged"}`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "-f", ".lucind/extra.json")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Done {
		t.Errorf("LaneStates() = %+v, want lane.Done", states)
	}
}

func TestExecuteScopeCheckStagedOnlyPathDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Stage an out-of-scope file and leave it uncommitted, matching the index
	serveDir := filepath.Join(wtPath, "internal", "serve")
	if err := os.MkdirAll(serveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDir, "staged.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/serve/staged.go")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot
	deps.PorcelainEmpty = worktree.PorcelainEmpty

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Deviated {
		t.Errorf("report.Status = %v, want %v (staged out-of-scope must demote to Deviated before completion mode)", report.Status, lane.Deviated)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Deviated {
		t.Errorf("LaneStates() = %+v, want lane.Deviated", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "internal/serve/staged.go") {
		t.Errorf("ledger notes = %+v, want note naming internal/serve/staged.go", details)
	}
}

func TestExecuteScopeCheckStagedOnlyInScopeReachesDone(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Stage an in-scope file and leave it uncommitted, matching the index
	ledgerDir := filepath.Join(wtPath, "internal", "ledger")
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(ledgerDir, "staged.go"), []byte("package ledger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/ledger/staged.go")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot
	// Stub PorcelainEmpty=true so completion mode doesn't fail on uncommitted changes
	deps.PorcelainEmpty = func(context.Context, string) (bool, error) { return true, nil }

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Done {
		t.Errorf("LaneStates() = %+v, want lane.Done", states)
	}
}

func TestExecuteScopeCheckRenameSourceAndDestChecked(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Create and commit an out-of-scope file first
	serveDir := filepath.Join(wtPath, "internal", "serve")
	if err := os.MkdirAll(serveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDir, "old.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/serve/old.go")
	runGit(t, wtPath, "commit", "-m", "add old.go")

	// Rename out-of-scope internal/serve/old.go to in-scope internal/ledger/new.go
	ledgerDir := filepath.Join(wtPath, "internal", "ledger")
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	runGit(t, wtPath, "mv", "internal/serve/old.go", "internal/ledger/new.go")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Deviated {
		t.Errorf("report.Status = %v, want %v (rename source is out-of-scope)", report.Status, lane.Deviated)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 1 || states[0].Status != lane.Deviated {
		t.Errorf("LaneStates() = %+v, want lane.Deviated", states)
	}

	details := laneNoteDetails(t, deps.Ledger, "run-1")
	if !anyContains(details, "internal/serve/old.go") {
		t.Errorf("ledger notes = %+v, want note naming rename source internal/serve/old.go", details)
	}
}

func TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	wtPath, birthSHA := setupGitWorktree(t, primaryRoot, "lane-a")

	// Commit out-of-scope path on the lane
	serveDir := filepath.Join(wtPath, "internal", "serve")
	if err := os.MkdirAll(serveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDir, "server.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, wtPath, "add", "internal/serve/server.go")
	runGit(t, wtPath, "commit", "-m", "lane commit out of scope")

	// Advance primary HEAD with an unrelated commit
	if err := os.WriteFile(filepath.Join(primaryRoot, "primary.txt"), []byte("primary work\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, primaryRoot, "add", "primary.txt")
	runGit(t, primaryRoot, "commit", "-m", "primary advance")

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe, birthSHA)
	deps.PrimaryRoot = primaryRoot

	p := testPacket()
	p.AllowedPaths = []string{"internal/ledger/"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if report.Status != lane.Deviated {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Deviated)
	}
}

func TestExecuteScopeCheckGitFailureResolvesToBlocked(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	t.Run("worktree missing recorded base SHA", func(t *testing.T) {
		primaryRoot := initGitRepo(t)
		wtPath, _ := setupGitWorktree(t, primaryRoot, "lane-a")

		fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
		deps := newTestDeps(t, wtPath, func(string) fs.FS {
			return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
		}, fe, "")
		deps.PrimaryRoot = primaryRoot

		p := testPacket()
		p.AllowedPaths = []string{"internal/ledger/"}

		report, err := run.Execute(context.Background(), deps, p)
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}

		if report.Status != lane.Blocked {
			t.Errorf("report.Status = %v, want %v (must not guess Done or Deviated)", report.Status, lane.Blocked)
		}

		states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("LaneStates() error = %v", err)
		}
		if len(states) != 1 || states[0].Status != lane.Blocked {
			t.Errorf("LaneStates() = %+v, want lane.Blocked", states)
		}

		details := laneNoteDetails(t, deps.Ledger, "run-1")
		if !anyContains(details, "base SHA") && !anyContains(details, "base sha") {
			t.Errorf("ledger notes = %+v, want note diagnosing missing base SHA", details)
		}
	})

	t.Run("worktreePath is not a git repo", func(t *testing.T) {
		primaryRoot := initGitRepo(t)
		invalidWtPath := t.TempDir()

		fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
		deps := newTestDeps(t, invalidWtPath, func(string) fs.FS {
			return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
		}, fe, "dummy-base-sha-1234567890abcdef")
		deps.PrimaryRoot = primaryRoot

		p := testPacket()
		p.AllowedPaths = []string{"internal/ledger/"}

		report, err := run.Execute(context.Background(), deps, p)
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}

		if report.Status != lane.Blocked {
			t.Errorf("report.Status = %v, want %v (must not guess Done or Deviated)", report.Status, lane.Blocked)
		}

		states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("LaneStates() error = %v", err)
		}
		if len(states) != 1 || states[0].Status != lane.Blocked {
			t.Errorf("LaneStates() = %+v, want lane.Blocked", states)
		}

		details := laneNoteDetails(t, deps.Ledger, "run-1")
		if !anyContains(details, "git diff") {
			t.Errorf("ledger notes = %+v, want note diagnosing git diff failure", details)
		}
	})
}

func TestExecuteScopeCheckEmptyAllowedPathsArraySkipsGitAndStaysDone(t *testing.T) {
	cases := []struct {
		name         string
		allowedPaths []string
	}{
		{
			name:         "nil slice (omitted)",
			allowedPaths: nil,
		},
		{
			name:         "empty slice (parsed from JSON [])",
			allowedPaths: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nonGitRoot := "/non-existent-primary-root"
			nonGitWt := t.TempDir()

			fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
			deps := newTestDeps(t, nonGitWt, func(string) fs.FS {
				return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
			}, fe)
			deps.PrimaryRoot = nonGitRoot

			p := testPacket()
			p.AllowedPaths = tc.allowedPaths

			report, err := run.Execute(context.Background(), deps, p)
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}

			if report.Status != lane.Done {
				t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
			}

			states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
			if err != nil {
				t.Fatalf("LaneStates() error = %v", err)
			}
			if len(states) != 1 || states[0].Status != lane.Done {
				t.Errorf("LaneStates() = %+v, want lane.Done", states)
			}
		})
	}
}

func TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []string
	}{
		{
			name:  "rename pair R100",
			input: []byte("R100\x00old path\x00new path\x00"),
			want:  []string{"old path", "new path"},
		},
		{
			name:  "ordinary modified M",
			input: []byte("M\x00dir/file.go\x00"),
			want:  []string{"dir/file.go"},
		},
		{
			name:  "path with embedded newline",
			input: []byte("M\x00dir/\nweird.go\x00"),
			want:  []string{"dir/\nweird.go"},
		},
		{
			name:  "copy pair C100",
			input: []byte("C100\x00src\x00dst\x00"),
			want:  []string{"src", "dst"},
		},
		{
			name:  "multiple mixed entries",
			input: []byte("M\x00a.go\x00R100\x00old.go\x00new.go\x00A\x00added.go\x00"),
			want:  []string{"a.go", "old.go", "new.go", "added.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run.ParseDiffNameStatusZ(tt.input)
			if !slicesEqual(got, tt.want) {
				t.Errorf("ParseDiffNameStatusZ() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLSFilesZ(t *testing.T) {
	input := []byte("a.go\x00file with spaces.txt\x00dir/\nweird.go\x00")
	want := []string{"a.go", "file with spaces.txt", "dir/\nweird.go"}

	got := run.ParseLSFilesZ(input)
	if !slicesEqual(got, want) {
		t.Errorf("ParseLSFilesZ() = %q, want %q", got, want)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExecuteApprovalWaitBlocksUntilDecideApprovePersistsDone(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.ApprovalTimeout = 5 * time.Second

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = deps.Ledger.Decide(context.Background(), deps.RunID, "lane-a", "alice", ledger.DecisionApproved)
	}()

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	lanes, err := deps.Ledger.Lanes(context.Background(), deps.RunID)
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	if len(lanes) != 1 || lanes[0].Status != lane.Done {
		t.Fatalf("lanes = %+v, want lane.Done", lanes)
	}

	app, err := deps.Ledger.Approval(context.Background(), deps.RunID, "lane-a")
	if err != nil {
		t.Fatalf("Approval() error = %v", err)
	}
	if app.Decision != ledger.DecisionApproved || app.Approver != "alice" {
		t.Errorf("app = %+v, want DecisionApproved by alice", app)
	}
}

func TestExecuteApprovalWaitRejectedPersistsBlocked(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.ApprovalTimeout = 5 * time.Second

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = deps.Ledger.Decide(context.Background(), deps.RunID, "lane-a", "bob", ledger.DecisionRejected)
	}()

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Blocked {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}

	lanes, err := deps.Ledger.Lanes(context.Background(), deps.RunID)
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	if len(lanes) != 1 || lanes[0].Status != lane.Blocked {
		t.Fatalf("lanes = %+v, want lane.Blocked", lanes)
	}

	app, err := deps.Ledger.Approval(context.Background(), deps.RunID, "lane-a")
	if err != nil {
		t.Fatalf("Approval() error = %v", err)
	}
	if app.Decision != ledger.DecisionRejected || app.Approver != "bob" {
		t.Errorf("app = %+v, want DecisionRejected by bob", app)
	}
}

func TestExecuteApprovalWaitTimeoutPersistsBlockedNeverDone(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.ApprovalTimeout = 100 * time.Millisecond

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Blocked {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}

	lanes, err := deps.Ledger.Lanes(context.Background(), deps.RunID)
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	if len(lanes) != 1 || lanes[0].Status != lane.Blocked {
		t.Fatalf("lanes = %+v, want lane.Blocked", lanes)
	}

	app, err := deps.Ledger.Approval(context.Background(), deps.RunID, "lane-a")
	if err != nil {
		t.Fatalf("Approval() error = %v", err)
	}
	if app.Decision != ledger.DecisionTimedOut {
		t.Errorf("app.Decision = %v, want DecisionTimedOut", app.Decision)
	}
}

func TestExecuteZeroApprovalTimeoutBypassesGate(t *testing.T) {
	wtPath := t.TempDir()
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtPath, func(string) fs.FS {
		return fstest.MapFS{".lucind/result.json": {Data: []byte(doneEnvelopeJSON)}}
	}, fe)
	deps.ApprovalTimeout = 0

	report, err := run.Execute(context.Background(), deps, testPacket())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if report.Status != lane.Done {
		t.Errorf("report.Status = %v, want %v", report.Status, lane.Done)
	}

	lanes, err := deps.Ledger.Lanes(context.Background(), deps.RunID)
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	if len(lanes) != 1 || lanes[0].Status != lane.Done {
		t.Fatalf("lanes = %+v, want lane.Done", lanes)
	}

	// No approval row created
	_, err = deps.Ledger.Approval(context.Background(), deps.RunID, "lane-a")
	if !errors.Is(err, ledger.ErrLaneUnknown) {
		t.Errorf("Approval() err = %v, want ErrLaneUnknown (no approval row)", err)
	}
}
