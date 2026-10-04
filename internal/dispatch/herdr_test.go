package dispatch_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
)

type fakeHerdrRunner struct {
	mu       sync.Mutex
	calls    [][]string
	handlers map[string]func(args []string) ([]byte, error)
	fallback func(args []string) ([]byte, error)
}

func newFakeHerdrRunner() *fakeHerdrRunner {
	return &fakeHerdrRunner{
		handlers: make(map[string]func(args []string) ([]byte, error)),
	}
}

func (f *fakeHerdrRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, args)
	f.mu.Unlock()

	key := ""
	if len(args) >= 2 {
		key = args[0] + " " + args[1]
	} else if len(args) == 1 {
		key = args[0]
	}

	if h, ok := f.handlers[key]; ok {
		return h(args)
	}
	if f.fallback != nil {
		return f.fallback(args)
	}
	return nil, nil
}

func (f *fakeHerdrRunner) Calls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([][]string, len(f.calls))
	copy(res, f.calls)
	return res
}

func TestDetermineSplitDirection_WidePaneRight(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "w1:p1")
	runner := newFakeHerdrRunner()
	runner.handlers["pane layout"] = func(args []string) ([]byte, error) {
		return []byte(`{
			"result": {
				"layout": {
					"panes": [
						{"pane_id": "w1:p1", "rect": {"width": 100, "height": 50}}
					]
				}
			}
		}`), nil
	}

	dir := dispatch.DetermineSplitDirection(context.Background(), runner)
	if dir != "right" {
		t.Errorf("DetermineSplitDirection() = %q, want \"right\"", dir)
	}
}

func TestDetermineSplitDirection_NarrowPaneDown(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "w1:p1")
	runner := newFakeHerdrRunner()
	runner.handlers["pane layout"] = func(args []string) ([]byte, error) {
		return []byte(`{
			"result": {
				"layout": {
					"panes": [
						{"pane_id": "w1:p1", "rect": {"width": 90, "height": 50}}
					]
				}
			}
		}`), nil
	}

	dir := dispatch.DetermineSplitDirection(context.Background(), runner)
	if dir != "down" {
		t.Errorf("DetermineSplitDirection() = %q, want \"down\"", dir)
	}
}

func TestDetermineSplitDirection_FocusedFallback(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "")
	runner := newFakeHerdrRunner()
	runner.handlers["pane layout"] = func(args []string) ([]byte, error) {
		return []byte(`{
			"result": {
				"layout": {
					"focused_pane_id": "w1:p2",
					"panes": [
						{"pane_id": "w1:p1", "focused": false, "rect": {"width": 200, "height": 50}},
						{"pane_id": "w1:p2", "focused": true, "rect": {"width": 60, "height": 50}}
					]
				}
			}
		}`), nil
	}

	dir := dispatch.DetermineSplitDirection(context.Background(), runner)
	if dir != "down" {
		t.Errorf("DetermineSplitDirection() = %q, want \"down\" (from focused pane w1:p2)", dir)
	}
}

func TestDetermineSplitDirection_LayoutErrorFallback(t *testing.T) {
	runner := newFakeHerdrRunner()
	runner.handlers["pane layout"] = func(args []string) ([]byte, error) {
		return nil, errors.New("pane layout failed")
	}

	dir := dispatch.DetermineSplitDirection(context.Background(), runner)
	if dir != "right" {
		t.Errorf("DetermineSplitDirection() = %q, want \"right\" fallback on error", dir)
	}
}

func TestDetermineSplitDirection_PaneNotFoundFallback(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "nonexistent")
	runner := newFakeHerdrRunner()
	runner.handlers["pane layout"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"layout": {"panes": []}}}`), nil
	}

	dir := dispatch.DetermineSplitDirection(context.Background(), runner)
	if dir != "right" {
		t.Errorf("DetermineSplitDirection() = %q, want \"right\" fallback when pane not found", dir)
	}
}

func TestSplitPane_Success(t *testing.T) {
	runner := newFakeHerdrRunner()
	runner.handlers["pane split"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"pane": {"pane_id": "w1:p_new"}}}`), nil
	}

	paneID, err := dispatch.SplitPane(context.Background(), runner, "right", "/some/cwd", "20261003-120000-abcd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paneID != "w1:p_new" {
		t.Errorf("SplitPane() = %q, want \"w1:p_new\"", paneID)
	}

	wantArgs := []string{
		"pane", "split",
		"--current",
		"--direction", "right",
		"--cwd", "/some/cwd",
		"--env", "LUCIND_LANE=20261003-120000-abcd",
		"--no-focus",
	}
	calls := runner.Calls()
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], wantArgs) {
		t.Errorf("SplitPane calls = %v, want %v", calls, [][]string{wantArgs})
	}
}

func TestSplitPane_Errors(t *testing.T) {
	t.Run("runner error", func(t *testing.T) {
		runner := newFakeHerdrRunner()
		runner.fallback = func(args []string) ([]byte, error) {
			return nil, errors.New("command failed")
		}
		_, err := dispatch.SplitPane(context.Background(), runner, "right", ".", "123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		runner := newFakeHerdrRunner()
		runner.fallback = func(args []string) ([]byte, error) {
			return []byte("not-json"), nil
		}
		_, err := dispatch.SplitPane(context.Background(), runner, "right", ".", "123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty pane id", func(t *testing.T) {
		runner := newFakeHerdrRunner()
		runner.fallback = func(args []string) ([]byte, error) {
			return []byte(`{"result":{"pane":{"pane_id":""}}}`), nil
		}
		_, err := dispatch.SplitPane(context.Background(), runner, "right", ".", "123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetPaneCwd_Success(t *testing.T) {
	runner := newFakeHerdrRunner()
	runner.handlers["pane get"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"pane": {"pane_id": "w1:p1", "cwd": "/workspace/my-project"}}}`), nil
	}

	cwd, err := dispatch.GetPaneCwd(context.Background(), runner, "w1:p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cwd != "/workspace/my-project" {
		t.Errorf("GetPaneCwd() = %q, want %q", cwd, "/workspace/my-project")
	}

	wantArgs := []string{"pane", "get", "w1:p1"}
	calls := runner.Calls()
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], wantArgs) {
		t.Errorf("GetPaneCwd calls = %v, want %v", calls, [][]string{wantArgs})
	}
}

func TestGetPaneCwd_Errors(t *testing.T) {
	t.Run("runner error", func(t *testing.T) {
		runner := newFakeHerdrRunner()
		runner.handlers["pane get"] = func(args []string) ([]byte, error) {
			return []byte("pane not found"), errors.New("exit 1")
		}
		_, err := dispatch.GetPaneCwd(context.Background(), runner, "w1:p1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "herdr pane get:") || !strings.Contains(err.Error(), "pane not found") {
			t.Errorf("unexpected error format: %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		runner := newFakeHerdrRunner()
		runner.handlers["pane get"] = func(args []string) ([]byte, error) {
			return []byte("invalid json output"), nil
		}
		_, err := dispatch.GetPaneCwd(context.Background(), runner, "w1:p1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "parse herdr pane get response:") {
			t.Errorf("unexpected error format: %v", err)
		}
	})

	t.Run("empty cwd", func(t *testing.T) {
		runner := newFakeHerdrRunner()
		runner.handlers["pane get"] = func(args []string) ([]byte, error) {
			return []byte(`{"result": {"pane": {"pane_id": "w1:p1", "cwd": ""}}}`), nil
		}
		_, err := dispatch.GetPaneCwd(context.Background(), runner, "w1:p1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "pane get did not return a cwd:") {
			t.Errorf("unexpected error format: %v", err)
		}
	})

	t.Run("missing cwd field", func(t *testing.T) {
		runner := newFakeHerdrRunner()
		runner.handlers["pane get"] = func(args []string) ([]byte, error) {
			return []byte(`{"result": {"pane": {"pane_id": "w1:p1"}}}`), nil
		}
		_, err := dispatch.GetPaneCwd(context.Background(), runner, "w1:p1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "pane get did not return a cwd:") {
			t.Errorf("unexpected error format: %v", err)
		}
	})
}

func TestClosePane_Success(t *testing.T) {
	runner := newFakeHerdrRunner()
	runner.handlers["pane close"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"closed": true}}`), nil
	}

	err := dispatch.ClosePane(context.Background(), runner, "w1:p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantArgs := []string{"pane", "close", "w1:p1"}
	calls := runner.Calls()
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], wantArgs) {
		t.Errorf("ClosePane calls = %v, want %v", calls, [][]string{wantArgs})
	}
}

func TestClosePane_Errors(t *testing.T) {
	runner := newFakeHerdrRunner()
	runner.handlers["pane close"] = func(args []string) ([]byte, error) {
		return []byte("pane already closed"), errors.New("exit 1")
	}

	err := dispatch.ClosePane(context.Background(), runner, "w1:p1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "herdr pane close:") || !strings.Contains(err.Error(), "pane already closed") {
		t.Errorf("unexpected error format: %v", err)
	}
}
