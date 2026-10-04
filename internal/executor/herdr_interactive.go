package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyhooks"
	"github.com/LanzerDevCorp/lucind-ai/internal/agytrust"
)

// resultEnvelopeRelPath mirrors the dispatcher's contract (internal/run: .lucind/result.json).
const resultEnvelopeRelPath = ".lucind/result.json"

// runInteractive runs agy as a visible interactive session in the lane pane.
//
// agy -i never exits by itself, so the end of the work is signalled by the Antigravity Stop hook
// (internal/agyhooks), which writes done.json once the agent is fully idle. The result envelope
// stays the contract: the hook validates it and lets the agent repair it a bounded number of
// times. After the hook reports, the session is ended with two C-c and the pane keeps showing
// what happened. Nothing is redirected to files, so the owner watches the agent work.
func (h HerdrAgy) runInteractive(ctx context.Context, req Request) (outcome Outcome, err error) {
	stateRoot, err := herdrStateRoot()
	if err != nil {
		return Outcome{}, err
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return Outcome{}, fmt.Errorf("create state root %q: %w", stateRoot, err)
	}
	reapOldRunDirs(stateRoot, runDirRetention, time.Now())
	stateDir, err := os.MkdirTemp(stateRoot, "run-*")
	if err != nil {
		return Outcome{}, fmt.Errorf("create temp state dir: %w", err)
	}
	defer func() {
		if outcome.ExitCode == 0 && !outcome.TimedOut && err == nil {
			_ = os.RemoveAll(stateDir)
		}
	}()

	if req.WorktreePath == "" {
		return Outcome{}, errors.New("herdr-agy interactive: worktree path is required")
	}

	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		return Outcome{}, fmt.Errorf("generate random nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	sentinelRegex := fmt.Sprintf("LUCIND_EXIT_%s=[0-9]+", nonce)

	promptPath := filepath.Join(stateDir, "prompt.md")
	runShPath := filepath.Join(stateDir, "run.sh")
	exitCodePath := filepath.Join(stateDir, "exit.code")
	if err := os.WriteFile(promptPath, []byte(req.Prompt), 0o600); err != nil {
		return Outcome{}, fmt.Errorf("write prompt.md: %w", err)
	}
	if err := os.WriteFile(runShPath, []byte(h.interactiveScript(req, promptPath, exitCodePath, nonce)), 0o700); err != nil {
		return Outcome{}, fmt.Errorf("write run.sh: %w", err)
	}

	// Trust the lane worktree (agy blocks on a trust prompt for unseen folders) and install the
	// Stop hook; both are undone when the lane ends.
	settings := h.TrustSettingsPath
	if settings == "" {
		if settings, err = agytrust.DefaultSettingsPath(); err != nil {
			return Outcome{}, err
		}
	}
	if err := agytrust.Add(settings, req.WorktreePath); err != nil {
		return Outcome{}, fmt.Errorf("trust lane worktree: %w", err)
	}
	defer func() { _ = agytrust.Remove(settings, req.WorktreePath) }()

	hookBin := h.HookBinary
	if hookBin == "" {
		if hookBin, err = os.Executable(); err != nil {
			return Outcome{}, fmt.Errorf("resolve lucind-ai executable for the Stop hook: %w", err)
		}
	}
	if err := agyhooks.Install(ctx, req.WorktreePath, agyhooks.Options{
		Binary:     hookBin,
		StateDir:   stateDir,
		ResultPath: filepath.Join(req.WorktreePath, filepath.FromSlash(resultEnvelopeRelPath)),
	}); err != nil {
		return Outcome{}, fmt.Errorf("install Stop hook: %w", err)
	}
	defer removeLaneHooks(req.WorktreePath)

	commonDir, err := h.getGitCommonDir(ctx, req.WorktreePath)
	if err != nil {
		return Outcome{}, fmt.Errorf("derive git common dir: %w", err)
	}
	paneID, ownedWorkspace, err := h.openLanePane(ctx, deriveRepoRoot(commonDir), req.WorktreePath)
	if err != nil {
		return Outcome{}, err
	}

	if runOut, runErr := h.execHerdr(ctx, "pane", "run", paneID, "sh "+shQuote(runShPath)); runErr != nil {
		return Outcome{}, fmt.Errorf("herdr pane run: %w", parseHerdrError(runOut, runErr))
	}

	// Wait for whichever comes first: the Stop hook's done.json, the pane sentinel (agy exited on
	// its own, e.g. crashed), or the caller's deadline.
	grace := h.Grace
	if grace <= 0 {
		grace = 10 * time.Second
	}
	pollEvery := h.PollInterval
	if pollEvery <= 0 {
		pollEvery = 500 * time.Millisecond
	}
	waitTimeout := 35 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		waitTimeout = max(time.Until(deadline), 0) + 30*time.Second
	}

	stop := make(chan struct{})
	defer close(stop)
	doneCh := make(chan agyhooks.Done, 1)
	go func() {
		ticker := time.NewTicker(pollEvery)
		defer ticker.Stop()
		for {
			if d, ok, err := agyhooks.ReadDone(stateDir); err == nil && ok {
				doneCh <- d
				return
			}
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	type sentinelResult struct {
		out []byte
		err error
	}
	sentinelCh := make(chan sentinelResult, 1)
	waitCtx, cancelWait := context.WithCancel(ctx)
	defer cancelWait()
	go func() {
		out, waitErr := h.execHerdr(waitCtx, "pane", "wait-output", paneID, "--regex", sentinelRegex, "--timeout", strconv.FormatInt(waitTimeout.Milliseconds(), 10))
		sentinelCh <- sentinelResult{out: out, err: waitErr}
	}()

	select {
	case d := <-doneCh:
		cancelWait()
		h.stopSession(paneID, ownedWorkspace, sentinelRegex, grace)
		return interactiveOutcome(d), nil
	case res := <-sentinelCh:
		if res.err != nil {
			if code, _ := decodeHerdrError(res.out); code == "timeout" || ctx.Err() != nil {
				break
			}
			return Outcome{}, fmt.Errorf("herdr pane wait-output: %w", parseHerdrError(res.out, res.err))
		}
		// agy left on its own. A done.json written just before the exit still wins.
		if d, ok, _ := agyhooks.ReadDone(stateDir); ok {
			return interactiveOutcome(d), nil
		}
		return readInteractiveExit(exitCodePath)
	case <-ctx.Done():
	}

	// Deadline: interrupt, then stop the pane for good.
	cancelWait()
	h.stopSession(paneID, ownedWorkspace, sentinelRegex, grace)
	outcome.TimedOut = true
	if o, exitErr := readInteractiveExit(exitCodePath); exitErr == nil {
		outcome.ExitCode = o.ExitCode
	}
	return outcome, nil
}

// interactiveScript is what the pane runs. agy's output is NOT redirected: it is the live view.
func (h HerdrAgy) interactiveScript(req Request, promptPath, exitCodePath, nonce string) string {
	binary := h.Binary
	if binary == "" {
		binary = "agy"
	}
	args := []string{
		shQuote(binary),
		"-i", fmt.Sprintf("\"$(cat %s)\"", shQuote(promptPath)),
		"--mode", shQuote("accept-edits"),
		"--dangerously-skip-permissions",
	}
	if req.Model != "" {
		args = append(args, "--model", shQuote(req.Model))
	}
	args = append(args, "--add-dir", shQuote(req.WorktreePath))

	var lines []string
	lines = append(lines, "#!/bin/sh", fmt.Sprintf("if cd -- %s; then", shQuote(req.WorktreePath)))
	for _, envVar := range requestEnv(req) {
		for _, name := range []string{readOnlyPathsEnv, requiredSkillsEnv} {
			if strings.HasPrefix(envVar, name+"=") {
				lines = append(lines, fmt.Sprintf("  export %s=%s", name, shQuote(strings.TrimPrefix(envVar, name+"="))))
			}
		}
	}
	lines = append(lines,
		"  "+strings.Join(args, " "),
		"  status=$?",
		"else",
		"  status=1",
		"fi",
		fmt.Sprintf("echo \"$status\" > %s", shQuote(exitCodePath)),
		fmt.Sprintf("echo \"LUCIND_EXIT_%s=$status\"", nonce),
	)
	return strings.Join(lines, "\n") + "\n"
}

// openLanePane opens the lane worktree in herdr and returns its root pane. ownedWorkspace is the
// workspace id only when this call created it; a reused workspace is never closed by us.
func (h HerdrAgy) openLanePane(ctx context.Context, repoRoot, worktree string) (paneID, ownedWorkspace string, err error) {
	openOut, openErr := h.execHerdr(ctx, "worktree", "open", "--cwd", repoRoot, "--path", worktree)
	if openErr != nil {
		return "", "", fmt.Errorf("herdr worktree open: %w", parseHerdrError(openOut, openErr))
	}
	var resp herdrWorktreeOpenResult
	if err := json.Unmarshal(openOut, &resp); err != nil {
		return "", "", fmt.Errorf("parse herdr worktree open response: %w", err)
	}
	if code, message := decodeHerdrError(openOut); code != "" || message != "" {
		return "", "", fmt.Errorf("herdr worktree open: %s: %s", code, message)
	}
	paneID = resp.Result.RootPane.PaneID
	if paneID == "" {
		return "", "", errors.New("herdr worktree open: missing root pane id")
	}
	if !resp.Result.AlreadyOpen {
		ownedWorkspace = resp.Result.Workspace.WorkspaceID
	}
	return paneID, ownedWorkspace, nil
}

// stopSession ends the agy session: C-c twice (the first clears input, the second exits), then, if
// the pane sentinel still has not appeared, one more C-c and, only for a workspace this executor
// opened, closing it.
func (h HerdrAgy) stopSession(paneID, ownedWorkspace, sentinelRegex string, grace time.Duration) {
	sendCtrlC := func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = h.execHerdr(c, "pane", "send-keys", paneID, "C-c")
	}
	exited := func(wait time.Duration) bool {
		c, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		_, err := h.execHerdr(c, "pane", "wait-output", paneID, "--regex", sentinelRegex, "--timeout", strconv.FormatInt(wait.Milliseconds(), 10))
		return err == nil
	}

	sendCtrlC()
	time.Sleep(min(grace/4, time.Second))
	sendCtrlC()
	if exited(grace) {
		return
	}
	sendCtrlC()
	if !exited(grace/2) && ownedWorkspace != "" {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = h.execHerdr(c, "workspace", "close", ownedWorkspace)
	}
}

func interactiveOutcome(d agyhooks.Done) Outcome {
	switch d.Status {
	case "valid":
		return Outcome{}
	case "invalid":
		// The dispatcher reads the envelope and decides; the executor ran fine.
		return Outcome{Stderr: "result envelope invalid after the allowed repairs: " + d.Error}
	default:
		msg := d.Error
		if msg == "" {
			msg = "status " + d.Status
		}
		return Outcome{ExitCode: 1, Stderr: "stop hook error: " + msg}
	}
}

func readInteractiveExit(exitCodePath string) (Outcome, error) {
	b, err := os.ReadFile(exitCodePath)
	if err != nil {
		return Outcome{}, fmt.Errorf("read exit.code: %w", err)
	}
	code, convErr := strconv.Atoi(strings.TrimSpace(string(b)))
	if convErr != nil {
		return Outcome{}, fmt.Errorf("parse exit.code %q: %w", strings.TrimSpace(string(b)), convErr)
	}
	return Outcome{ExitCode: code}, nil
}

// removeLaneHooks deletes the hooks file this lane installed (and .agents when that leaves it empty).
func removeLaneHooks(worktree string) {
	dir := filepath.Join(worktree, ".agents")
	_ = os.Remove(filepath.Join(dir, "hooks.json"))
	_ = os.Remove(dir) // only succeeds when empty
}

func herdrStateRoot() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lucind-ai", "herdr"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "lucind-ai", "herdr"), nil
}
