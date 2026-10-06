package lane_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

const (
	lifecycleValidDoneJSON = `{
		"lane_id": "test-lane",
		"status": "done",
		"summary": "Completed successfully",
		"hard_stops": []
	}`

	lifecycleValidFailedJSON = `{
		"lane_id": "test-lane",
		"status": "failed",
		"summary": "Execution failed",
		"hard_stops": []
	}`

	lifecycleSchemaInvalidJSON = `{
		"lane_id": "test-lane",
		"status": "done"
	}`
)

func createLifecycleLane(t *testing.T, dir string, status lane.Status) lane.Lane {
	t.Helper()
	now := time.Now().UTC().Add(-5 * time.Minute)
	l := lane.Lane{
		Version:   1,
		ID:        "20261006-120000-abcd",
		Cwd:       dir,
		BaseTree:  "test-base-tree",
		Allow:     []string{"initial.txt"},
		Checks:    []string{"echo ok"},
		Model:     "test-model",
		Status:    status,
		Turn:      1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := l.Save(dir); err != nil {
		t.Fatalf("save initial test lane: %v", err)
	}
	return l
}

func writeLifecycleResult(t *testing.T, dir string, l lane.Lane, content string) {
	t.Helper()
	resPath := lane.ResultFilePath(dir, l)
	if err := os.MkdirAll(filepath.Dir(resPath), 0755); err != nil {
		t.Fatalf("mkdir for result file: %v", err)
	}
	if err := os.WriteFile(resPath, []byte(content), 0644); err != nil {
		t.Fatalf("write result file: %v", err)
	}
}

func TestLane_BeginTurn(t *testing.T) {
	t.Run("legal and illegal starting statuses", func(t *testing.T) {
		tests := []struct {
			name        string
			startStatus lane.Status
			wantErr     bool
		}{
			{"legal running", lane.StatusRunning, false},
			{"legal done", lane.StatusDone, false},
			{"legal failed", lane.StatusFailed, false},
			{"legal timeout", lane.StatusTimeout, false},
			{"illegal accepted", lane.StatusAccepted, true},
			{"illegal rejected", lane.StatusRejected, true},
			{"illegal bogus", lane.Status("bogus"), true},
			{"illegal empty", lane.Status(""), true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				l := createLifecycleLane(t, dir, tt.startStatus)

				err := l.BeginTurn(dir, nil, nil)
				if (err != nil) != tt.wantErr {
					t.Fatalf("BeginTurn() error = %v, wantErr %v", err, tt.wantErr)
				}

				loaded, loadErr := lane.Load(dir, l.ID)
				if loadErr != nil {
					t.Fatalf("Load lane: %v", loadErr)
				}

				if tt.wantErr {
					if l.Status != tt.startStatus {
						t.Errorf("in-memory Status = %v, want unmodified %v", l.Status, tt.startStatus)
					}
					if loaded.Status != tt.startStatus {
						t.Errorf("on-disk Status = %v, want unmodified %v", loaded.Status, tt.startStatus)
					}
				} else {
					if l.Status != lane.StatusRunning {
						t.Errorf("in-memory Status = %v, want %v", l.Status, lane.StatusRunning)
					}
					if loaded.Status != lane.StatusRunning {
						t.Errorf("on-disk Status = %v, want %v", loaded.Status, lane.StatusRunning)
					}
				}
			})
		}
	})

	t.Run("legacy turn 0 handling with result.json", func(t *testing.T) {
		dir := t.TempDir()
		l := createLifecycleLane(t, dir, lane.StatusDone)
		l.Turn = 0
		if err := l.Save(dir); err != nil {
			t.Fatalf("save lane with Turn 0: %v", err)
		}

		laneDir := lane.LaneDir(dir, l.ID)
		resultPath := filepath.Join(laneDir, "result.json")
		prevPath := filepath.Join(laneDir, "result.prev.json")
		if err := os.WriteFile(resultPath, []byte(lifecycleValidDoneJSON), 0644); err != nil {
			t.Fatalf("write result.json: %v", err)
		}

		if err := l.BeginTurn(dir, nil, nil); err != nil {
			t.Fatalf("BeginTurn() error = %v, want nil", err)
		}

		if _, err := os.Stat(resultPath); !os.IsNotExist(err) {
			t.Errorf("expected result.json to no longer exist after rename, stat err = %v", err)
		}

		prevContent, err := os.ReadFile(prevPath)
		if err != nil {
			t.Fatalf("read result.prev.json: %v", err)
		}
		if string(prevContent) != lifecycleValidDoneJSON {
			t.Errorf("result.prev.json content = %s, want %s", string(prevContent), lifecycleValidDoneJSON)
		}

		if l.Turn != 1 {
			t.Errorf("l.Turn = %d, want 1", l.Turn)
		}
		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Turn != 1 {
			t.Errorf("loaded.Turn = %d, want 1", loaded.Turn)
		}
	})

	t.Run("legacy turn 0 handling without result.json", func(t *testing.T) {
		dir := t.TempDir()
		l := createLifecycleLane(t, dir, lane.StatusDone)
		l.Turn = 0
		if err := l.Save(dir); err != nil {
			t.Fatalf("save lane with Turn 0: %v", err)
		}

		if err := l.BeginTurn(dir, nil, nil); err != nil {
			t.Fatalf("BeginTurn() error = %v, want nil", err)
		}

		if l.Turn != 1 {
			t.Errorf("l.Turn = %d, want 1", l.Turn)
		}
		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Turn != 1 {
			t.Errorf("loaded.Turn = %d, want 1", loaded.Turn)
		}
	})

	t.Run("non-zero turn increments", func(t *testing.T) {
		tests := []struct {
			startTurn int
			wantTurn  int
		}{
			{1, 2},
			{2, 3},
			{5, 6},
		}

		for _, tt := range tests {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusDone)
			l.Turn = tt.startTurn
			if err := l.Save(dir); err != nil {
				t.Fatalf("save lane: %v", err)
			}

			if err := l.BeginTurn(dir, nil, nil); err != nil {
				t.Fatalf("BeginTurn() error = %v, want nil", err)
			}

			if l.Turn != tt.wantTurn {
				t.Errorf("l.Turn = %d, want %d", l.Turn, tt.wantTurn)
			}
			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Turn != tt.wantTurn {
				t.Errorf("loaded.Turn = %d, want %d", loaded.Turn, tt.wantTurn)
			}
		}
	})

	t.Run("counter resets", func(t *testing.T) {
		dir := t.TempDir()
		l := createLifecycleLane(t, dir, lane.StatusFailed)
		stopTime := time.Now().UTC().Add(-2 * time.Minute)
		l.Retries = 2
		l.Continues = 5
		l.LastStopAt = &stopTime
		if err := l.Save(dir); err != nil {
			t.Fatalf("save lane: %v", err)
		}

		if err := l.BeginTurn(dir, nil, nil); err != nil {
			t.Fatalf("BeginTurn() error = %v, want nil", err)
		}

		if l.Retries != 0 {
			t.Errorf("l.Retries = %d, want 0", l.Retries)
		}
		if l.Continues != 0 {
			t.Errorf("l.Continues = %d, want 0", l.Continues)
		}
		if l.LastStopAt != nil {
			t.Errorf("l.LastStopAt = %v, want nil", l.LastStopAt)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Retries != 0 {
			t.Errorf("loaded.Retries = %d, want 0", loaded.Retries)
		}
		if loaded.Continues != 0 {
			t.Errorf("loaded.Continues = %d, want 0", loaded.Continues)
		}
		if loaded.LastStopAt != nil {
			t.Errorf("loaded.LastStopAt = %v, want nil", loaded.LastStopAt)
		}
	})

	t.Run("allow and checks updating", func(t *testing.T) {
		tests := []struct {
			name        string
			newAllow    []string
			newChecks   []string
			wantAllow   []string
			wantChecks  []string
		}{
			{
				name:       "updates when provided",
				newAllow:   []string{"updated.txt", "more.txt"},
				newChecks:  []string{"go test ./..."},
				wantAllow:  []string{"updated.txt", "more.txt"},
				wantChecks: []string{"go test ./..."},
			},
			{
				name:       "preserves existing when nil",
				newAllow:   nil,
				newChecks:  nil,
				wantAllow:  []string{"initial.txt"},
				wantChecks: []string{"echo ok"},
			},
			{
				name:       "preserves existing when empty slice",
				newAllow:   []string{},
				newChecks:  []string{},
				wantAllow:  []string{"initial.txt"},
				wantChecks: []string{"echo ok"},
			},
			{
				name:       "updates allow and preserves checks when checks is nil",
				newAllow:   []string{"only-allow.txt"},
				newChecks:  nil,
				wantAllow:  []string{"only-allow.txt"},
				wantChecks: []string{"echo ok"},
			},
			{
				name:       "preserves allow and updates checks when allow is nil",
				newAllow:   nil,
				newChecks:  []string{"make lint"},
				wantAllow:  []string{"initial.txt"},
				wantChecks: []string{"make lint"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				l := createLifecycleLane(t, dir, lane.StatusDone)

				if err := l.BeginTurn(dir, tt.newAllow, tt.newChecks); err != nil {
					t.Fatalf("BeginTurn() error = %v, want nil", err)
				}

				if len(l.Allow) != len(tt.wantAllow) {
					t.Fatalf("l.Allow length = %d, want %d", len(l.Allow), len(tt.wantAllow))
				}
				for i := range tt.wantAllow {
					if l.Allow[i] != tt.wantAllow[i] {
						t.Errorf("l.Allow[%d] = %q, want %q", i, l.Allow[i], tt.wantAllow[i])
					}
				}

				if len(l.Checks) != len(tt.wantChecks) {
					t.Fatalf("l.Checks length = %d, want %d", len(l.Checks), len(tt.wantChecks))
				}
				for i := range tt.wantChecks {
					if l.Checks[i] != tt.wantChecks[i] {
						t.Errorf("l.Checks[%d] = %q, want %q", i, l.Checks[i], tt.wantChecks[i])
					}
				}

				loaded, err := lane.Load(dir, l.ID)
				if err != nil {
					t.Fatalf("Load lane: %v", err)
				}
				if len(loaded.Allow) != len(tt.wantAllow) {
					t.Fatalf("loaded.Allow length = %d, want %d", len(loaded.Allow), len(tt.wantAllow))
				}
				if len(loaded.Checks) != len(tt.wantChecks) {
					t.Fatalf("loaded.Checks length = %d, want %d", len(loaded.Checks), len(tt.wantChecks))
				}
			})
		}
	})

	t.Run("status transitions to running and updatedAt refreshed and saved", func(t *testing.T) {
		dir := t.TempDir()
		l := createLifecycleLane(t, dir, lane.StatusDone)
		priorUpdatedAt := l.UpdatedAt

		time.Sleep(10 * time.Millisecond)
		if err := l.BeginTurn(dir, nil, nil); err != nil {
			t.Fatalf("BeginTurn() error = %v, want nil", err)
		}

		if l.Status != lane.StatusRunning {
			t.Errorf("l.Status = %v, want %v", l.Status, lane.StatusRunning)
		}
		if !l.UpdatedAt.After(priorUpdatedAt) {
			t.Errorf("l.UpdatedAt (%v) was not refreshed after priorUpdatedAt (%v)", l.UpdatedAt, priorUpdatedAt)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusRunning {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusRunning)
		}
		if !loaded.UpdatedAt.After(priorUpdatedAt) {
			t.Errorf("loaded.UpdatedAt (%v) was not refreshed", loaded.UpdatedAt)
		}
	})

	t.Run("CanContinue", func(t *testing.T) {
		tests := []struct {
			status      lane.Status
			wantCanCont bool
		}{
			{lane.StatusRunning, true},
			{lane.StatusDone, true},
			{lane.StatusFailed, true},
			{lane.StatusTimeout, true},
			{lane.StatusAccepted, false},
			{lane.StatusRejected, false},
			{lane.Status("bogus"), false},
			{lane.Status(""), false},
		}

		for _, tt := range tests {
			t.Run(string(tt.status), func(t *testing.T) {
				if got := tt.status.CanContinue(); got != tt.wantCanCont {
					t.Errorf("Status(%q).CanContinue() = %v, want %v", tt.status, got, tt.wantCanCont)
				}

				l := lane.Lane{Status: tt.status}
				if got := l.CanContinue(); got != tt.wantCanCont {
					t.Errorf("Lane{Status: %q}.CanContinue() = %v, want %v", tt.status, got, tt.wantCanCont)
				}
			})
		}
	})
}

func TestLane_RecordStop(t *testing.T) {
	t.Run("starting statuses", func(t *testing.T) {
		tests := []struct {
			name        string
			startStatus lane.Status
			wantErr     bool
		}{
			{"legal running", lane.StatusRunning, false},
			{"illegal done", lane.StatusDone, true},
			{"illegal failed", lane.StatusFailed, true},
			{"illegal timeout", lane.StatusTimeout, true},
			{"illegal accepted", lane.StatusAccepted, true},
			{"illegal rejected", lane.StatusRejected, true},
			{"illegal bogus", lane.Status("bogus"), true},
			{"illegal empty", lane.Status(""), true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				l := createLifecycleLane(t, dir, tt.startStatus)
				writeLifecycleResult(t, dir, l, lifecycleValidDoneJSON)

				now := time.Now().UTC()
				_, err := l.RecordStop(dir, now)
				if (err != nil) != tt.wantErr {
					t.Fatalf("RecordStop() error = %v, wantErr %v", err, tt.wantErr)
				}

				loaded, loadErr := lane.Load(dir, l.ID)
				if loadErr != nil {
					t.Fatalf("Load lane: %v", loadErr)
				}

				if tt.wantErr {
					if l.Status != tt.startStatus {
						t.Errorf("in-memory Status = %v, want %v", l.Status, tt.startStatus)
					}
					if loaded.Status != tt.startStatus {
						t.Errorf("on-disk Status = %v, want %v", loaded.Status, tt.startStatus)
					}
				}
			})
		}
	})

	t.Run("result evaluation and retry budget edges", func(t *testing.T) {
		t.Run("legal stop with valid done result", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRunning)
			writeLifecycleResult(t, dir, l, lifecycleValidDoneJSON)

			now := time.Now().UTC()
			dec, err := l.RecordStop(dir, now)
			if err != nil {
				t.Fatalf("RecordStop error: %v", err)
			}
			if dec.Action != lane.StopActionStop {
				t.Errorf("Action = %v, want %v", dec.Action, lane.StopActionStop)
			}
			if l.Status != lane.StatusDone {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusDone)
			}

			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Status != lane.StatusDone {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusDone)
			}
		})

		t.Run("legal stop with valid failed result", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRunning)
			writeLifecycleResult(t, dir, l, lifecycleValidFailedJSON)

			now := time.Now().UTC()
			dec, err := l.RecordStop(dir, now)
			if err != nil {
				t.Fatalf("RecordStop error: %v", err)
			}
			if dec.Action != lane.StopActionStop {
				t.Errorf("Action = %v, want %v", dec.Action, lane.StopActionStop)
			}
			if l.Status != lane.StatusFailed {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusFailed)
			}

			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Status != lane.StatusFailed {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
			}
		})

		t.Run("missing result with budget available", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRunning)
			l.Retries = 0
			l.Continues = 0
			l.LastStopAt = nil
			if err := l.Save(dir); err != nil {
				t.Fatalf("Save lane: %v", err)
			}

			now := time.Now().UTC()
			dec, err := l.RecordStop(dir, now)
			if err != nil {
				t.Fatalf("RecordStop error: %v", err)
			}
			if dec.Action != lane.StopActionContinue {
				t.Errorf("Action = %v, want %v", dec.Action, lane.StopActionContinue)
			}
			if l.Retries != 1 {
				t.Errorf("Retries = %d, want 1", l.Retries)
			}
			if l.Continues != 1 {
				t.Errorf("Continues = %d, want 1", l.Continues)
			}
			if l.LastStopAt == nil || !l.LastStopAt.Equal(now) {
				t.Errorf("LastStopAt = %v, want %v", l.LastStopAt, now)
			}
			if l.Status != lane.StatusRunning {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusRunning)
			}

			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Retries != 1 {
				t.Errorf("loaded.Retries = %d, want 1", loaded.Retries)
			}
			if loaded.Continues != 1 {
				t.Errorf("loaded.Continues = %d, want 1", loaded.Continues)
			}
			if loaded.LastStopAt == nil || !loaded.LastStopAt.Equal(now) {
				t.Errorf("loaded.LastStopAt = %v, want %v", loaded.LastStopAt, now)
			}
			if loaded.Status != lane.StatusRunning {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusRunning)
			}
		})

		t.Run("schema-invalid result with budget available", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRunning)
			l.Retries = 1
			l.Continues = 3
			pastStop := time.Now().UTC().Add(-10 * time.Second)
			l.LastStopAt = &pastStop
			if err := l.Save(dir); err != nil {
				t.Fatalf("Save lane: %v", err)
			}
			writeLifecycleResult(t, dir, l, lifecycleSchemaInvalidJSON)

			now := time.Now().UTC()
			dec, err := l.RecordStop(dir, now)
			if err != nil {
				t.Fatalf("RecordStop error: %v", err)
			}
			if dec.Action != lane.StopActionContinue {
				t.Errorf("Action = %v, want %v", dec.Action, lane.StopActionContinue)
			}
			if l.Retries != 2 {
				t.Errorf("Retries = %d, want 2", l.Retries)
			}
			if l.Continues != 4 {
				t.Errorf("Continues = %d, want 4", l.Continues)
			}
			if l.Status != lane.StatusRunning {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusRunning)
			}

			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Retries != 2 {
				t.Errorf("loaded.Retries = %d, want 2", loaded.Retries)
			}
			if loaded.Continues != 4 {
				t.Errorf("loaded.Continues = %d, want 4", loaded.Continues)
			}
			if loaded.Status != lane.StatusRunning {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusRunning)
			}
		})

		t.Run("missing result after RetryQuietWindow", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRunning)
			l.Retries = 1
			l.Continues = 3
			now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
			pastStop := now.Add(-lane.RetryQuietWindow - 5*time.Second) // >= 60s
			l.LastStopAt = &pastStop
			if err := l.Save(dir); err != nil {
				t.Fatalf("Save lane: %v", err)
			}

			dec, err := l.RecordStop(dir, now)
			if err != nil {
				t.Fatalf("RecordStop error: %v", err)
			}
			if dec.Action != lane.StopActionContinue {
				t.Errorf("Action = %v, want %v", dec.Action, lane.StopActionContinue)
			}
			if !dec.QuietWindowReset {
				t.Errorf("QuietWindowReset = %v, want true", dec.QuietWindowReset)
			}
			// Retries reset to 0 then incremented to 1
			if l.Retries != 1 {
				t.Errorf("Retries = %d, want 1", l.Retries)
			}
			// Continues incremented to 4
			if l.Continues != 4 {
				t.Errorf("Continues = %d, want 4", l.Continues)
			}
			if l.Status != lane.StatusRunning {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusRunning)
			}

			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Retries != 1 {
				t.Errorf("loaded.Retries = %d, want 1", loaded.Retries)
			}
			if loaded.Continues != 4 {
				t.Errorf("loaded.Continues = %d, want 4", loaded.Continues)
			}
			if loaded.Status != lane.StatusRunning {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusRunning)
			}
		})

		t.Run("missing result when Retries exhausted", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRunning)
			l.Retries = 2
			l.Continues = 2
			now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
			pastStop := now.Add(-10 * time.Second) // within quiet window (< 60s)
			l.LastStopAt = &pastStop
			if err := l.Save(dir); err != nil {
				t.Fatalf("Save lane: %v", err)
			}

			dec, err := l.RecordStop(dir, now)
			if err != nil {
				t.Fatalf("RecordStop error: %v", err)
			}
			if dec.Action != lane.StopActionStop {
				t.Errorf("Action = %v, want %v", dec.Action, lane.StopActionStop)
			}
			if dec.LimitExhausted != "retries" {
				t.Errorf("LimitExhausted = %q, want %q", dec.LimitExhausted, "retries")
			}
			if l.Status != lane.StatusFailed {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusFailed)
			}

			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Status != lane.StatusFailed {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
			}
		})

		t.Run("missing result when Continues exhausted", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRunning)
			l.Retries = 0
			l.Continues = 10
			now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
			pastStop := now.Add(-10 * time.Second)
			l.LastStopAt = &pastStop
			if err := l.Save(dir); err != nil {
				t.Fatalf("Save lane: %v", err)
			}

			dec, err := l.RecordStop(dir, now)
			if err != nil {
				t.Fatalf("RecordStop error: %v", err)
			}
			if dec.Action != lane.StopActionStop {
				t.Errorf("Action = %v, want %v", dec.Action, lane.StopActionStop)
			}
			if dec.LimitExhausted != "total continues" {
				t.Errorf("LimitExhausted = %q, want %q", dec.LimitExhausted, "total continues")
			}
			if l.Status != lane.StatusFailed {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusFailed)
			}

			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Status != lane.StatusFailed {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
			}
		})
	})
}

func TestLane_MarkTimeout(t *testing.T) {
	tests := []struct {
		name        string
		startStatus lane.Status
		wantErr     bool
	}{
		{"legal running", lane.StatusRunning, false},
		{"legal failed", lane.StatusFailed, false},
		{"illegal done", lane.StatusDone, true},
		{"illegal accepted", lane.StatusAccepted, true},
		{"illegal rejected", lane.StatusRejected, true},
		{"illegal timeout", lane.StatusTimeout, true},
		{"illegal bogus", lane.Status("bogus"), true},
		{"illegal empty", lane.Status(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, tt.startStatus)
			priorUpdatedAt := l.UpdatedAt

			time.Sleep(10 * time.Millisecond)
			err := l.MarkTimeout(dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MarkTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}

			loaded, loadErr := lane.Load(dir, l.ID)
			if loadErr != nil {
				t.Fatalf("Load lane: %v", loadErr)
			}

			if tt.wantErr {
				if l.Status != tt.startStatus {
					t.Errorf("in-memory Status = %v, want %v", l.Status, tt.startStatus)
				}
				if loaded.Status != tt.startStatus {
					t.Errorf("on-disk Status = %v, want %v", loaded.Status, tt.startStatus)
				}
			} else {
				if l.Status != lane.StatusTimeout {
					t.Errorf("in-memory Status = %v, want %v", l.Status, lane.StatusTimeout)
				}
				if !l.UpdatedAt.After(priorUpdatedAt) {
					t.Errorf("in-memory UpdatedAt (%v) not after priorUpdatedAt (%v)", l.UpdatedAt, priorUpdatedAt)
				}
				if loaded.Status != lane.StatusTimeout {
					t.Errorf("on-disk Status = %v, want %v", loaded.Status, lane.StatusTimeout)
				}
				if !loaded.UpdatedAt.After(priorUpdatedAt) {
					t.Errorf("on-disk UpdatedAt (%v) not after priorUpdatedAt (%v)", loaded.UpdatedAt, priorUpdatedAt)
				}
			}
		})
	}
}

func TestLane_RecordVerdict(t *testing.T) {
	t.Run("legal starting statuses and verdicts", func(t *testing.T) {
		legalStatuses := []lane.Status{
			lane.StatusRunning,
			lane.StatusDone,
			lane.StatusFailed,
			lane.StatusTimeout,
			lane.StatusAccepted,
			lane.StatusRejected,
		}

		verdicts := []struct {
			verdict    string
			wantStatus lane.Status
		}{
			{lane.VerdictAccepted, lane.StatusAccepted},
			{lane.VerdictRejected, lane.StatusRejected},
		}

		for _, st := range legalStatuses {
			for _, v := range verdicts {
				t.Run(string(st)+" with "+v.verdict, func(t *testing.T) {
					dir := t.TempDir()
					l := createLifecycleLane(t, dir, st)
					priorUpdatedAt := l.UpdatedAt

					time.Sleep(10 * time.Millisecond)
					err := l.RecordVerdict(dir, v.verdict)
					if err != nil {
						t.Fatalf("RecordVerdict(%q) error = %v, want nil", v.verdict, err)
					}

					if l.Status != v.wantStatus {
						t.Errorf("in-memory Status = %v, want %v", l.Status, v.wantStatus)
					}
					if !l.UpdatedAt.After(priorUpdatedAt) {
						t.Errorf("in-memory UpdatedAt (%v) not after priorUpdatedAt (%v)", l.UpdatedAt, priorUpdatedAt)
					}

					loaded, err := lane.Load(dir, l.ID)
					if err != nil {
						t.Fatalf("Load lane: %v", err)
					}
					if loaded.Status != v.wantStatus {
						t.Errorf("on-disk Status = %v, want %v", loaded.Status, v.wantStatus)
					}
					if !loaded.UpdatedAt.After(priorUpdatedAt) {
						t.Errorf("on-disk UpdatedAt (%v) not after priorUpdatedAt (%v)", loaded.UpdatedAt, priorUpdatedAt)
					}
				})
			}
		}
	})

	t.Run("re-review transitions", func(t *testing.T) {
		t.Run("rejected to accepted", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusRejected)
			if err := l.RecordVerdict(dir, lane.VerdictAccepted); err != nil {
				t.Fatalf("RecordVerdict(VerdictAccepted) error = %v, want nil", err)
			}
			if l.Status != lane.StatusAccepted {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusAccepted)
			}
			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Status != lane.StatusAccepted {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusAccepted)
			}
		})

		t.Run("accepted to rejected", func(t *testing.T) {
			dir := t.TempDir()
			l := createLifecycleLane(t, dir, lane.StatusAccepted)
			if err := l.RecordVerdict(dir, lane.VerdictRejected); err != nil {
				t.Fatalf("RecordVerdict(VerdictRejected) error = %v, want nil", err)
			}
			if l.Status != lane.StatusRejected {
				t.Errorf("Status = %v, want %v", l.Status, lane.StatusRejected)
			}
			loaded, err := lane.Load(dir, l.ID)
			if err != nil {
				t.Fatalf("Load lane: %v", err)
			}
			if loaded.Status != lane.StatusRejected {
				t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusRejected)
			}
		})
	})

	t.Run("illegal verdicts", func(t *testing.T) {
		illegalVerdicts := []string{"bogus", "", "pending"}
		for _, v := range illegalVerdicts {
			t.Run("verdict "+v, func(t *testing.T) {
				dir := t.TempDir()
				l := createLifecycleLane(t, dir, lane.StatusDone)

				err := l.RecordVerdict(dir, v)
				if err == nil {
					t.Fatalf("RecordVerdict(%q) error = nil, want error", v)
				}

				if l.Status != lane.StatusDone {
					t.Errorf("in-memory Status = %v, want %v", l.Status, lane.StatusDone)
				}
				loaded, loadErr := lane.Load(dir, l.ID)
				if loadErr != nil {
					t.Fatalf("Load lane: %v", loadErr)
				}
				if loaded.Status != lane.StatusDone {
					t.Errorf("on-disk Status = %v, want %v", loaded.Status, lane.StatusDone)
				}
			})
		}
	})

	t.Run("illegal starting statuses", func(t *testing.T) {
		illegalStatuses := []lane.Status{
			lane.Status("bogus"),
			lane.Status(""),
		}

		for _, st := range illegalStatuses {
			t.Run(string(st), func(t *testing.T) {
				dir := t.TempDir()
				l := createLifecycleLane(t, dir, st)

				err := l.RecordVerdict(dir, lane.VerdictAccepted)
				if err == nil {
					t.Fatalf("RecordVerdict() error = nil, want error for status %v", st)
				}

				if l.Status != st {
					t.Errorf("in-memory Status = %v, want %v", l.Status, st)
				}
				loaded, loadErr := lane.Load(dir, l.ID)
				if loadErr != nil {
					t.Fatalf("Load lane: %v", loadErr)
				}
				if loaded.Status != st {
					t.Errorf("on-disk Status = %v, want %v", loaded.Status, st)
				}
			})
		}
	})
}

func TestLane_MarkStopped(t *testing.T) {
	t.Run("valid starting statuses", func(t *testing.T) {
		tests := []struct {
			name        string
			startStatus lane.Status
			resultJSON  string
			writeResult bool
			wantStatus  lane.Status
		}{
			{
				name:        "running with valid done result marks done",
				startStatus: lane.StatusRunning,
				resultJSON:  lifecycleValidDoneJSON,
				writeResult: true,
				wantStatus:  lane.StatusDone,
			},
			{
				name:        "running with valid failed result marks failed",
				startStatus: lane.StatusRunning,
				resultJSON:  lifecycleValidFailedJSON,
				writeResult: true,
				wantStatus:  lane.StatusFailed,
			},
			{
				name:        "running with missing result marks failed",
				startStatus: lane.StatusRunning,
				writeResult: false,
				wantStatus:  lane.StatusFailed,
			},
			{
				name:        "running with schema invalid result marks failed",
				startStatus: lane.StatusRunning,
				resultJSON:  lifecycleSchemaInvalidJSON,
				writeResult: true,
				wantStatus:  lane.StatusFailed,
			},
			{
				name:        "failed with valid done result marks done",
				startStatus: lane.StatusFailed,
				resultJSON:  lifecycleValidDoneJSON,
				writeResult: true,
				wantStatus:  lane.StatusDone,
			},
			{
				name:        "failed with valid failed result marks failed",
				startStatus: lane.StatusFailed,
				resultJSON:  lifecycleValidFailedJSON,
				writeResult: true,
				wantStatus:  lane.StatusFailed,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				l := createLifecycleLane(t, dir, tt.startStatus)
				if tt.writeResult {
					writeLifecycleResult(t, dir, l, tt.resultJSON)
				}

				st, err := l.MarkStopped(dir)
				if err != nil {
					t.Fatalf("MarkStopped() error = %v, want nil", err)
				}
				if st != tt.wantStatus {
					t.Errorf("returned Status = %v, want %v", st, tt.wantStatus)
				}
				if l.Status != tt.wantStatus {
					t.Errorf("in-memory Status = %v, want %v", l.Status, tt.wantStatus)
				}

				loaded, err := lane.Load(dir, l.ID)
				if err != nil {
					t.Fatalf("Load lane: %v", err)
				}
				if loaded.Status != tt.wantStatus {
					t.Errorf("on-disk Status = %v, want %v", loaded.Status, tt.wantStatus)
				}
			})
		}
	})

	t.Run("invalid starting statuses", func(t *testing.T) {
		tests := []struct {
			name        string
			startStatus lane.Status
		}{
			{"done is invalid", lane.StatusDone},
			{"timeout is invalid", lane.StatusTimeout},
			{"accepted is invalid", lane.StatusAccepted},
			{"rejected is invalid", lane.StatusRejected},
			{"bogus is invalid", lane.Status("bogus")},
			{"empty is invalid", lane.Status("")},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				l := createLifecycleLane(t, dir, tt.startStatus)
				writeLifecycleResult(t, dir, l, lifecycleValidDoneJSON)

				_, err := l.MarkStopped(dir)
				if err == nil {
					t.Fatalf("MarkStopped() error = nil, want error for start status %v", tt.startStatus)
				}

				if l.Status != tt.startStatus {
					t.Errorf("in-memory Status = %v, want %v", l.Status, tt.startStatus)
				}
				loaded, loadErr := lane.Load(dir, l.ID)
				if loadErr != nil {
					t.Fatalf("Load lane: %v", loadErr)
				}
				if loaded.Status != tt.startStatus {
					t.Errorf("on-disk Status = %v, want %v", loaded.Status, tt.startStatus)
				}
			})
		}
	})
}
