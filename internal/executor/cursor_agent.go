package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
)

// defaultCursorBinary is the name CursorAgent execs when Binary is left empty.
// It relies on PATH lookup, matching how the CLI is normally invoked.
const defaultCursorBinary = "cursor-agent"

// CursorAgent dispatches the cursor-agent CLI headlessly.
type CursorAgent struct {
	// Binary is the executable to run. Defaults to "cursor-agent" when empty,
	// but tests override it to point at a stub script instead of spending
	// real quota against the real CLI.
	Binary string

	// WaitDelay bounds how long Run waits for the child's stdio pipes to
	// drain once the direct child process has exited. Zero (the field's
	// natural default) is replaced with defaultWaitDelay in Run, so
	// production always dispatches with a non-zero value -- see
	// defaultWaitDelay's doc comment for why that value must never be
	// left at exec's own zero-value behavior (wait forever). Tests set
	// this explicitly to something small.
	WaitDelay time.Duration
}

// DefaultModel returns the model CursorAgent runs on when a packet names none.
func (c CursorAgent) DefaultModel() string {
	return "cursor-grok-4.6-high"
}

// KnownModels returns every model a packet may deliberately request for
// CursorAgent. cursor-grok-4.6-high, CursorAgent's own default, is the only
// entry: CursorAgent must run strictly on grok, never escalate to another
// provider's model -- Claude's opus included -- so provider identity and
// billing stay unambiguous. Extend this list only for a model actually
// verified against the real cursor-agent CLI, and never with another
// provider's model -- see internal/executor.Executor.KnownModels's doc
// comment for why this is not free text.
func (c CursorAgent) KnownModels() []string {
	return []string{"cursor-grok-4.6-high"}
}

// Run execs cursor-agent with req.Prompt, in req.WorktreePath, bounded by ctx.
//
// The flag set this method always sends -- --print, --output-format json,
// --trust, --force, and --approve-mcps -- is the minimal non-interactive
// invocation verified empirically across both design proposals.
// --trust and --force (-f) in particular are load-bearing: without them a
// headless run stalls on an interactive workspace trust or tool-execution
// prompt and exits with an error or dies on the wall clock.
//
// Unlike agy, cursor-agent exposes no timeout flag and no --json-schema
// equivalent. Wall-clock bounding is enforced exclusively via ctx, and schema
// enforcement is handled by result envelope validation rather than flag-level
// constraints.
func (c CursorAgent) Run(ctx context.Context, req Request) (Outcome, error) {
	binary := c.Binary
	if binary == "" {
		binary = defaultCursorBinary
	}

	if req.Progress != nil {
		capture := &cursorStreamCapture{ctx: ctx, progress: req.Progress}
		outcome, err := c.run(ctx, req, binary, "stream-json", capture)
		capture.finish()
		if err != nil || outcome.TimedOut || capture.parseable() {
			return outcome, err
		}
	}

	return c.run(ctx, req, binary, "json", &bytes.Buffer{})
}

func (c CursorAgent) run(ctx context.Context, req Request, binary, outputFormat string, stdout cursorOutputCapture) (Outcome, error) {
	args := []string{
		"--print", req.Prompt,
		"--output-format", outputFormat,
		"--trust",
		"--force",
		"--approve-mcps",
	}
	if outputFormat == "stream-json" {
		args = append(args, "--stream-partial-output")
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.WorktreePath != "" {
		args = append(args, "--workspace", req.WorktreePath)
	}

	waitDelay := c.WaitDelay
	if waitDelay == 0 {
		waitDelay = defaultWaitDelay
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = req.WorktreePath
	cmd.WaitDelay = waitDelay

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = stdout

	err := cmd.Run()

	outcome := Outcome{Stderr: stderr.String(), Stdout: stdout.String()}

	if ctx.Err() == context.DeadlineExceeded {
		outcome.TimedOut = true
		return outcome, nil
	}

	if errors.Is(err, exec.ErrWaitDelay) {
		// The process itself ran and exited with a successful status --
		// os/exec.Cmd.WaitDelay's doc comment is explicit that
		// ErrWaitDelay replaces a nil Wait error only when "the command
		// has otherwise exited with a successful status". Only its
		// stdio pipes stayed open past waitDelay afterward (typically
		// because a grandchild process inherited them). cmd.ProcessState
		// is populated once the process itself has exited, so it is the
		// reliable source for the real exit code here, not a guess.
		if cmd.ProcessState != nil {
			outcome.ExitCode = cmd.ProcessState.ExitCode()
		}
		outcome.OutputTruncated = true
		return outcome, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		outcome.ExitCode = exitErr.ExitCode()
		return outcome, nil
	}
	if err != nil {
		// Not an ExitError and not ErrWaitDelay: the process never ran
		// at all (binary missing, permission denied, etc). This is a
		// real error, not an Outcome.
		return Outcome{}, err
	}

	outcome.ExitCode = 0
	return outcome, nil
}

type cursorOutputCapture interface {
	io.Writer
	String() string
}

// cursorStreamCapture decodes only record fields observed from the installed
// cursor-agent. Valid records with unfamiliar shapes are intentionally ignored.
type cursorStreamCapture struct {
	ctx       context.Context
	progress  chan<- ProgressEvent
	raw       bytes.Buffer
	pending   []byte
	records   int
	malformed bool
}

func (c *cursorStreamCapture) Write(p []byte) (int, error) {
	n, err := c.raw.Write(p)
	c.pending = append(c.pending, p...)
	for {
		newline := bytes.IndexByte(c.pending, '\n')
		if newline < 0 {
			break
		}
		c.decode(c.pending[:newline])
		c.pending = c.pending[newline+1:]
	}
	return n, err
}

func (c *cursorStreamCapture) String() string {
	return c.raw.String()
}

func (c *cursorStreamCapture) finish() {
	if len(c.pending) > 0 {
		c.decode(c.pending)
		c.pending = nil
	}
}

func (c *cursorStreamCapture) parseable() bool {
	return c.records > 0 && !c.malformed
}

func (c *cursorStreamCapture) decode(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}

	var record struct {
		Type        string `json:"type"`
		Subtype     string `json:"subtype"`
		Text        string `json:"text"`
		TimestampMS *int64 `json:"timestamp_ms"`
		Message     struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &record); err != nil {
		c.malformed = true
		return
	}
	c.records++

	var messages []string
	if record.Type == "thinking" && record.Subtype == "delta" {
		messages = append(messages, record.Text)
	}
	if record.Type == "assistant" {
		for _, content := range record.Message.Content {
			if content.Type == "text" {
				messages = append(messages, content.Text)
			}
		}
	}

	at := time.Now()
	if record.TimestampMS != nil {
		at = time.UnixMilli(*record.TimestampMS)
	}
	for _, message := range messages {
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}
		select {
		case c.progress <- ProgressEvent{Message: message, At: at}:
		case <-c.ctx.Done():
			return
		}
	}
}
