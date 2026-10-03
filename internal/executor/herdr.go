package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type herdrCmd func(ctx context.Context, args ...string) ([]byte, error)
type gitCommonDirFunc func(ctx context.Context, worktree string) (string, error)

// HerdrAgy dispatches the agy CLI headlessly inside a herdr pane.
type HerdrAgy struct {
	// Binary is the executable to run. Defaults to "agy" when empty.
	Binary string

	// PollInterval bounds how frequently out.ndjson is polled for stream-json
	// progress events. When zero, defaults to 1 second.
	PollInterval time.Duration

	// Unexported seams for test isolation
	cmd          herdrCmd
	gitCommonDir gitCommonDirFunc
}

// DefaultModel delegates to Agy.
func (h HerdrAgy) DefaultModel() string {
	return Agy{}.DefaultModel()
}

// KnownModels delegates to Agy.
func (h HerdrAgy) KnownModels() []string {
	return Agy{}.KnownModels()
}

func (h HerdrAgy) execHerdr(ctx context.Context, args ...string) ([]byte, error) {
	if h.cmd != nil {
		return h.cmd(ctx, args...)
	}
	cmd := exec.CommandContext(ctx, "herdr", args...)
	return cmd.CombinedOutput()
}

func (h HerdrAgy) getGitCommonDir(ctx context.Context, worktree string) (string, error) {
	if h.gitCommonDir != nil {
		return h.gitCommonDir(ctx, worktree)
	}
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func deriveRepoRoot(commonDir string) string {
	clean := filepath.Clean(commonDir)
	if strings.HasSuffix(clean, string(filepath.Separator)+".git") || clean == ".git" {
		return filepath.Dir(clean)
	}
	return clean
}

type herdrWorktreeOpenResult struct {
	Result struct {
		AlreadyOpen bool `json:"already_open"`
		Workspace   struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"workspace"`
		RootPane struct {
			PaneID string `json:"pane_id"`
		} `json:"root_pane"`
	} `json:"result"`
	Error json.RawMessage `json:"error"`
}

// herdrErrorPayload is the failure envelope herdr prints, for example
// {"error":{"code":"timeout","message":"..."},"id":"cli:pane:wait-output"}.
// A bare string in "error" is also accepted.
type herdrErrorPayload struct {
	Error json.RawMessage `json:"error"`
}

// decodeHerdrError extracts the machine code and the human message from a herdr
// failure envelope. Both are empty when data is not an error envelope.
func decodeHerdrError(data []byte) (code, message string) {
	var payload herdrErrorPayload
	if json.Unmarshal(data, &payload) != nil || len(payload.Error) == 0 || string(payload.Error) == "null" {
		return "", ""
	}
	var obj struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(payload.Error, &obj) == nil && (obj.Code != "" || obj.Message != "") {
		return obj.Code, obj.Message
	}
	var text string
	if json.Unmarshal(payload.Error, &text) == nil {
		return text, ""
	}
	return "", ""
}

func parseHerdrError(data []byte, fallbackErr error) error {
	if code, message := decodeHerdrError(data); code != "" || message != "" {
		if code != "" && message != "" {
			return fmt.Errorf("%s: %s", code, message)
		}
		return errors.New(code + message)
	}
	if fallbackErr != nil {
		if len(data) > 0 {
			return fmt.Errorf("%w: %s", fallbackErr, strings.TrimSpace(string(data)))
		}
		return fallbackErr
	}
	return errors.New(strings.TrimSpace(string(data)))
}

// Run execs agy inside a herdr pane in the specified worktree.
func (h HerdrAgy) Run(ctx context.Context, req Request) (outcome Outcome, err error) {
	// 1. Resolve state directory outside the worktree
	stateRoot := os.Getenv("XDG_STATE_HOME")
	if stateRoot == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return Outcome{}, fmt.Errorf("resolve home dir: %w", homeErr)
		}
		stateRoot = filepath.Join(home, ".local", "state", "lucind-ai", "herdr")
	} else {
		stateRoot = filepath.Join(stateRoot, "lucind-ai", "herdr")
	}

	if err := os.MkdirAll(stateRoot, 0700); err != nil {
		return Outcome{}, fmt.Errorf("create state root %q: %w", stateRoot, err)
	}

	stateDir, err := os.MkdirTemp(stateRoot, "run-*")
	if err != nil {
		return Outcome{}, fmt.Errorf("create temp state dir: %w", err)
	}

	// 8. Cleanup state directory on success
	defer func() {
		if outcome.ExitCode == 0 && !outcome.TimedOut && err == nil {
			_ = os.RemoveAll(stateDir)
		}
	}()

	promptPath := filepath.Join(stateDir, "prompt.md")
	runShPath := filepath.Join(stateDir, "run.sh")
	outPath := filepath.Join(stateDir, "out.ndjson")
	errPath := filepath.Join(stateDir, "err.log")
	exitCodePath := filepath.Join(stateDir, "exit.code")

	if err := os.WriteFile(promptPath, []byte(req.Prompt), 0600); err != nil {
		return Outcome{}, fmt.Errorf("write prompt.md: %w", err)
	}

	// 2. Format: stream-json when req.Progress != nil, otherwise json
	format := "json"
	if req.Progress != nil {
		format = "stream-json"
	}

	// 3. Generate run.sh
	binary := h.Binary
	if binary == "" {
		binary = "agy"
	}

	agyArgs := []string{
		shQuote(binary),
		"--print", fmt.Sprintf("\"$(cat %s)\"", shQuote(promptPath)),
		"--output-format", shQuote(format),
		"--mode", shQuote("accept-edits"),
		"--dangerously-skip-permissions",
	}
	if req.Model != "" {
		agyArgs = append(agyArgs, "--model", shQuote(req.Model))
	}
	if req.SchemaPath != "" {
		agyArgs = append(agyArgs, "--json-schema", shQuote(req.SchemaPath))
	}
	if req.WorktreePath != "" {
		agyArgs = append(agyArgs, "--add-dir", shQuote(req.WorktreePath))
	}
	if pt, ok := printTimeoutFor(ctx); ok {
		agyArgs = append(agyArgs, "--print-timeout", shQuote(pt.String()))
	}

	// Generate 16 random hex characters for nonce
	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		return Outcome{}, fmt.Errorf("generate random nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	// Filter requestEnv for LUCIND_READ_ONLY_PATHS and LUCIND_REQUIRED_SKILLS only
	var exports []string
	for _, envVar := range requestEnv(req) {
		if strings.HasPrefix(envVar, readOnlyPathsEnv+"=") {
			val := strings.TrimPrefix(envVar, readOnlyPathsEnv+"=")
			exports = append(exports, fmt.Sprintf("export %s=%s", readOnlyPathsEnv, shQuote(val)))
		}
		if strings.HasPrefix(envVar, requiredSkillsEnv+"=") {
			val := strings.TrimPrefix(envVar, requiredSkillsEnv+"=")
			exports = append(exports, fmt.Sprintf("export %s=%s", requiredSkillsEnv, shQuote(val)))
		}
	}

	// A failed cd must never fall through to running agy in the pane's default
	// directory: skip agy, report status 1, and still emit the sentinel below.
	cmdLine := fmt.Sprintf("%s > %s 2> %s", strings.Join(agyArgs, " "), shQuote(outPath), shQuote(errPath))
	var scriptLines []string
	scriptLines = append(scriptLines, "#!/bin/sh")
	if req.WorktreePath != "" {
		scriptLines = append(scriptLines, fmt.Sprintf("if cd -- %s; then", shQuote(req.WorktreePath)))
	} else {
		scriptLines = append(scriptLines, "if true; then")
	}
	for _, e := range exports {
		scriptLines = append(scriptLines, "  "+e)
	}
	scriptLines = append(scriptLines, "  "+cmdLine)
	scriptLines = append(scriptLines, "  status=$?")
	scriptLines = append(scriptLines, "else")
	scriptLines = append(scriptLines, "  status=1")
	scriptLines = append(scriptLines, fmt.Sprintf("  echo 'herdr-agy: cannot enter worktree' > %s", shQuote(errPath)))
	scriptLines = append(scriptLines, "fi")
	scriptLines = append(scriptLines, fmt.Sprintf("echo \"$status\" > %s", shQuote(exitCodePath)))
	scriptLines = append(scriptLines, fmt.Sprintf("echo \"LUCIND_EXIT_%s=$status\"", nonce))

	scriptContent := strings.Join(scriptLines, "\n") + "\n"
	if err := os.WriteFile(runShPath, []byte(scriptContent), 0700); err != nil {
		return Outcome{}, fmt.Errorf("write run.sh: %w", err)
	}

	// 4. Open lane worktree in herdr
	commonDir, err := h.getGitCommonDir(ctx, req.WorktreePath)
	if err != nil {
		return Outcome{}, fmt.Errorf("derive git common dir: %w", err)
	}
	repoRoot := deriveRepoRoot(commonDir)

	openOut, openErr := h.execHerdr(ctx, "worktree", "open", "--cwd", repoRoot, "--path", req.WorktreePath)
	if openErr != nil {
		return Outcome{}, fmt.Errorf("herdr worktree open: %w", parseHerdrError(openOut, openErr))
	}

	var openResp herdrWorktreeOpenResult
	if err := json.Unmarshal(openOut, &openResp); err != nil {
		return Outcome{}, fmt.Errorf("parse herdr worktree open response: %w", err)
	}
	if code, message := decodeHerdrError(openOut); code != "" || message != "" {
		return Outcome{}, fmt.Errorf("herdr worktree open: %s: %s", code, message)
	}
	paneID := openResp.Result.RootPane.PaneID
	if paneID == "" {
		return Outcome{}, errors.New("herdr worktree open: missing root pane id")
	}

	// Run the script in the root pane
	typedCmd := fmt.Sprintf("sh %s", shQuote(runShPath))
	runOut, runErr := h.execHerdr(ctx, "pane", "run", paneID, typedCmd)
	if runErr != nil {
		return Outcome{}, fmt.Errorf("herdr pane run: %w", parseHerdrError(runOut, runErr))
	}

	// Setup progress polling if stream-json
	var decoder *agyStreamDecoder
	stopPolling := make(chan struct{})
	var stopOnce sync.Once
	stopPoll := func() { stopOnce.Do(func() { close(stopPolling) }) }
	var pollWG sync.WaitGroup
	var offset int64
	var offsetMu sync.Mutex

	if req.Progress != nil {
		decoder = newAgyStreamDecoder(req.Progress)
		pollInterval := h.PollInterval
		if pollInterval <= 0 {
			pollInterval = 1 * time.Second
		}
		pollWG.Add(1)
		go func() {
			defer pollWG.Done()
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stopPolling:
					return
				case <-ticker.C:
					f, err := os.Open(outPath)
					if err != nil {
						continue
					}
					fi, err := f.Stat()
					if err == nil {
						offsetMu.Lock()
						currentOffset := offset
						if fi.Size() > currentOffset {
							buf := make([]byte, fi.Size()-currentOffset)
							n, _ := f.ReadAt(buf, currentOffset)
							if n > 0 {
								offset += int64(n)
								_, _ = decoder.Write(buf[:n])
							}
						}
						offsetMu.Unlock()
					}
					_ = f.Close()
				}
			}
		}()
	}

	// 5. Wait for sentinel
	var waitTimeout time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < 0 {
			remaining = 0
		}
		waitTimeout = remaining + 30*time.Second
	} else {
		waitTimeout = 35 * time.Minute
	}
	timeoutMs := waitTimeout.Milliseconds()
	if timeoutMs < 0 {
		timeoutMs = 0
	}

	sentinelRegex := fmt.Sprintf("LUCIND_EXIT_%s=[0-9]+", nonce)
	type waitResult struct {
		out []byte
		err error
	}
	waitDone := make(chan waitResult, 1)
	go func() {
		out, waitErr := h.execHerdr(ctx, "pane", "wait-output", paneID, "--regex", sentinelRegex, "--timeout", strconv.FormatInt(timeoutMs, 10))
		waitDone <- waitResult{out: out, err: waitErr}
	}()

	var isTimedOut bool
	select {
	case <-ctx.Done():
		isTimedOut = true
	case res := <-waitDone:
		if res.err != nil {
			// Only a herdr "timeout" is a timeout. Any other failure (pane gone,
			// herdr unreachable) is an execution error, not a timed-out lane.
			if code, _ := decodeHerdrError(res.out); code == "timeout" || ctx.Err() != nil {
				isTimedOut = true
			} else {
				stopPoll()
				pollWG.Wait()
				return Outcome{}, fmt.Errorf("herdr pane wait-output: %w", parseHerdrError(res.out, res.err))
			}
		}
	}

	// 6. Handle timeout or cancellation
	if isTimedOut {
		ctrlCtx, ctrlCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = h.execHerdr(ctrlCtx, "pane", "send-keys", paneID, "C-c")
		ctrlCancel()

		graceCtx, graceCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = h.execHerdr(graceCtx, "pane", "wait-output", paneID, "--regex", sentinelRegex, "--timeout", "10000")
		graceCancel()

		outcome.TimedOut = true
	}

	// Stop progress polling and drain remaining output
	if req.Progress != nil {
		stopPoll()
		pollWG.Wait()

		f, err := os.Open(outPath)
		if err == nil {
			fi, err := f.Stat()
			if err == nil {
				offsetMu.Lock()
				currentOffset := offset
				if fi.Size() > currentOffset {
					buf := make([]byte, fi.Size()-currentOffset)
					n, _ := f.ReadAt(buf, currentOffset)
					if n > 0 {
						_, _ = decoder.Write(buf[:n])
					}
				}
				offsetMu.Unlock()
			}
			_ = f.Close()
		}
		decoder.finish()
	}

	// 7. Read outputs and build Outcome
	errBytes, _ := os.ReadFile(errPath)
	outcome.Stderr = string(errBytes)

	outBytes, _ := os.ReadFile(outPath)
	if req.Progress != nil {
		if decoder.terminal {
			outcome.Stdout = string(decoder.result)
		} else {
			outcome.Stdout = string(outBytes)
			if !outcome.TimedOut {
				outcome.OutputTruncated = true
			}
		}
	} else {
		outcome.Stdout = string(outBytes)
	}

	exitBytes, exitErr := os.ReadFile(exitCodePath)
	if exitErr == nil {
		code, convErr := strconv.Atoi(strings.TrimSpace(string(exitBytes)))
		if convErr != nil {
			// Never report an unreadable status as success.
			return Outcome{}, fmt.Errorf("parse exit.code %q: %w", strings.TrimSpace(string(exitBytes)), convErr)
		}
		outcome.ExitCode = code
	} else if !outcome.TimedOut {
		return Outcome{}, fmt.Errorf("read exit.code: %w", exitErr)
	}

	return outcome, nil
}
