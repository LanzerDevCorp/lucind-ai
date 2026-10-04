package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// HerdrRunner executes herdr commands.
type HerdrRunner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// DefaultHerdrRunner executes herdr via os/exec.
type DefaultHerdrRunner struct{}

func (d DefaultHerdrRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "herdr", args...)
	return cmd.CombinedOutput()
}

type paneRect struct {
	Height int `json:"height"`
	Width  int `json:"width"`
	X      int `json:"x"`
	Y      int `json:"y"`
}

type paneInfo struct {
	Focused bool     `json:"focused"`
	PaneID  string   `json:"pane_id"`
	Rect    paneRect `json:"rect"`
}

type layoutResponse struct {
	Result struct {
		Layout struct {
			FocusedPaneID string     `json:"focused_pane_id"`
			Panes         []paneInfo `json:"panes"`
		} `json:"layout"`
	} `json:"result"`
}

type splitResponse struct {
	Result struct {
		Pane struct {
			PaneID string `json:"pane_id"`
		} `json:"pane"`
	} `json:"result"`
}

// DetermineSplitDirection determines whether to split "right" or "down".
// It queries `herdr pane layout`, checks the rect of $HERDR_PANE_ID (or the focused pane),
// and returns "right" if width >= 2 * height, else "down".
// If layout fails or pane is not found, it falls back to "right".
func DetermineSplitDirection(ctx context.Context, runner HerdrRunner) string {
	paneEnv := os.Getenv("HERDR_PANE_ID")
	var args []string
	if paneEnv != "" {
		args = []string{"pane", "layout", "--pane", paneEnv}
	} else {
		args = []string{"pane", "layout"}
	}

	out, err := runner.Run(ctx, args...)
	if err != nil {
		return "right"
	}

	var resp layoutResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "right"
	}

	panes := resp.Result.Layout.Panes
	var matched *paneInfo
	if paneEnv != "" {
		for i := range panes {
			if panes[i].PaneID == paneEnv {
				matched = &panes[i]
				break
			}
		}
	}

	if matched == nil {
		focusedID := resp.Result.Layout.FocusedPaneID
		for i := range panes {
			if panes[i].Focused || (focusedID != "" && panes[i].PaneID == focusedID) {
				matched = &panes[i]
				break
			}
		}
	}

	if matched == nil {
		return "right"
	}

	if matched.Rect.Width >= 2*matched.Rect.Height {
		return "right"
	}
	return "down"
}

// SplitPane splits the current pane in the specified direction and returns the new pane ID.
func SplitPane(ctx context.Context, runner HerdrRunner, direction, cwd, laneID string) (string, error) {
	args := []string{
		"pane", "split",
		"--current",
		"--direction", direction,
		"--cwd", cwd,
		"--env", "LUCIND_LANE=" + laneID,
		"--no-focus",
	}

	out, err := runner.Run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("herdr pane split: %w", err)
	}

	var resp splitResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("parse herdr pane split response: %w", err)
	}

	paneID := resp.Result.Pane.PaneID
	if paneID == "" {
		return "", fmt.Errorf("pane split did not return a pane_id: %s", string(out))
	}

	return paneID, nil
}

type paneGetResponse struct {
	Result struct {
		Pane struct {
			Cwd string `json:"cwd"`
		} `json:"pane"`
	} `json:"result"`
}

// GetPaneCwd queries herdr for the working directory of a pane.
func GetPaneCwd(ctx context.Context, runner HerdrRunner, paneID string) (string, error) {
	out, err := runner.Run(ctx, "pane", "get", paneID)
	if err != nil {
		return "", fmt.Errorf("herdr pane get: %w (output: %s)", err, string(out))
	}

	var resp paneGetResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("parse herdr pane get response: %w", err)
	}

	if resp.Result.Pane.Cwd == "" {
		return "", fmt.Errorf("pane get did not return a cwd: %s", string(out))
	}

	return resp.Result.Pane.Cwd, nil
}

// ClosePane closes a pane in herdr.
func ClosePane(ctx context.Context, runner HerdrRunner, paneID string) error {
	out, err := runner.Run(ctx, "pane", "close", paneID)
	if err != nil {
		return fmt.Errorf("herdr pane close: %w (output: %s)", err, string(out))
	}
	return nil
}

