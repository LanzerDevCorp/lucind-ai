package agyhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Options holds configuration for installing Antigravity lane hooks.
type Options struct {
	Binary       string // lucind-ai executable path, absolute
	StateDir     string // absolute
	ResultPath   string // absolute
	MaxContinues int    // 0 => 2
}

// Done records the terminal outcome written to done.json in the state dir.
type Done struct {
	Status            string `json:"status"`
	TerminationReason string `json:"terminationReason,omitempty"`
	Error             string `json:"error,omitempty"`
	At                string `json:"at,omitempty"`
}

// DonePath returns the path to done.json in stateDir.
func DonePath(stateDir string) string {
	return filepath.Join(stateDir, "done.json")
}

// ReadDone reads and unmarshals done.json from stateDir.
// If the file does not exist, it returns false with no error.
func ReadDone(stateDir string) (Done, bool, error) {
	data, err := os.ReadFile(DonePath(stateDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Done{}, false, nil
		}
		return Done{}, false, fmt.Errorf("read done.json: %w", err)
	}
	var d Done
	if err := json.Unmarshal(data, &d); err != nil {
		return Done{}, false, fmt.Errorf("parse done.json: %w", err)
	}
	return d, true, nil
}

// WriteDone atomically writes done.json into stateDir.
func WriteDone(stateDir string, d Done) error {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if d.At == "" {
		d.At = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal done.json: %w", err)
	}
	data = append(data, '\n')
	return WriteAtomic(DonePath(stateDir), data)
}

// WriteAtomic writes data to targetPath by writing to a temporary file in the
// same directory and renaming it into place.
func WriteAtomic(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "atomic-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename to %s: %w", targetPath, err)
	}
	return nil
}

// shQuote single-quotes a string for POSIX shells, escaping embedded single quotes.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type hookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type hookLane struct {
	Stop []hookHandler `json:"Stop"`
}

type hooksFile struct {
	LucindLane hookLane `json:"lucind-lane"`
}

// Install installs the Stop hook into <worktree>/.agents/hooks.json and adds it
// to the repository's git info/exclude file.
func Install(ctx context.Context, worktree string, o Options) error {
	if !filepath.IsAbs(o.Binary) {
		return errors.New("agyhooks: Binary path must be absolute")
	}
	if !filepath.IsAbs(o.StateDir) {
		return errors.New("agyhooks: StateDir path must be absolute")
	}
	if !filepath.IsAbs(o.ResultPath) {
		return errors.New("agyhooks: ResultPath must be absolute")
	}

	hooksPath := filepath.Join(worktree, ".agents", "hooks.json")
	if _, err := os.Stat(hooksPath); err == nil {
		return fmt.Errorf("agyhooks: %s already exists", hooksPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("agyhooks: stat %s: %w", hooksPath, err)
	}

	maxContinues := o.MaxContinues
	if maxContinues <= 0 {
		maxContinues = 2
	}

	cmd := fmt.Sprintf("%s hook stop --state-dir %s --result %s --max-continues %s",
		shQuote(o.Binary),
		shQuote(o.StateDir),
		shQuote(o.ResultPath),
		shQuote(strconv.Itoa(maxContinues)),
	)

	hf := hooksFile{
		LucindLane: hookLane{
			Stop: []hookHandler{
				{
					Type:    "command",
					Command: cmd,
					Timeout: 30,
				},
			},
		},
	}

	data, err := json.MarshalIndent(hf, "", "  ")
	if err != nil {
		return fmt.Errorf("agyhooks: marshal hooks.json: %w", err)
	}
	data = append(data, '\n')

	agentsDir := filepath.Join(worktree, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("agyhooks: create directory %s: %w", agentsDir, err)
	}

	if err := os.WriteFile(hooksPath, data, 0644); err != nil {
		return fmt.Errorf("agyhooks: write %s: %w", hooksPath, err)
	}
	_ = os.Chmod(hooksPath, 0644)

	if err := updateGitExclude(ctx, worktree); err != nil {
		return fmt.Errorf("agyhooks: update git exclude: %w", err)
	}

	return nil
}

func updateGitExclude(ctx context.Context, worktree string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "rev-parse", "--git-path", "info/exclude")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git rev-parse --git-path info/exclude: %w", err)
	}
	excludePath := strings.TrimSpace(string(out))
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(worktree, excludePath)
	}

	const excludeEntry = "/.agents/hooks.json"

	data, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read exclude file %s: %w", excludePath, err)
	}

	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == excludeEntry {
				return nil
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		return fmt.Errorf("create dir for exclude file: %w", err)
	}

	var buf bytes.Buffer
	if len(data) > 0 {
		buf.Write(data)
		if !bytes.HasSuffix(data, []byte("\n")) {
			buf.WriteByte('\n')
		}
	}
	buf.WriteString(excludeEntry + "\n")

	if err := os.WriteFile(excludePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write exclude file %s: %w", excludePath, err)
	}
	_ = os.Chmod(excludePath, 0644)
	return nil
}
