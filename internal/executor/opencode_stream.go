package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// opencodeStreamDegradedMessage is emitted once, on the first stdout line
// that does not decode, so a degraded timeline is visibly degraded instead
// of merely sparse. Decoding is not switched off afterwards: the raw
// stream is always what Outcome.Stdout falls back to, and one unreadable
// line must never blind the rest of a lane's timeline -- this decoder's
// tool events are now load-bearing for measuring how much of a fan-out
// synthesis lane's wall clock is tool latency versus silent reasoning (see
// the package-level profiling context in cursor_agent_stream.go), so
// disabling decoding entirely on the first bad line would defeat the
// purpose it exists for. claude_stream.go's claudeStreamDecoder.note()
// made the identical choice for the identical reason; cursor-agent's
// cursorStreamCapture.decode() never disables at all either, so this
// brings opencode in line with the other three executors rather than
// leaving it the one outlier.
const opencodeStreamDegradedMessage = "Opencode progress decoding degraded; raw stream retained as stdout"

// opencodeEditingTools are the tool names reported under the "Edit" kind
// rather than the generic "Tool" kind, matching the distinction
// cursor_agent_stream.go's cursorEditingTools and agy_stream.go already
// draw. Verified against the installed CLI (1.18.21):
//   - "edit" and "write" are exercised directly by the primary agent loop's
//     path-extraction helper, which special-cases exactly
//     `case "read": case "edit": case "write": return path(input.filePath ??
//     input.filepath)` -- read is excluded here because it does not mutate
//     anything.
//   - "patch" is confirmed as a real, independently configurable tool name
//     (not merely the unrelated "patch" *part* type used for git-snapshot
//     diff summaries) by opencode's own permission-config normalizer:
//     `if (name === "write" || name === "edit" || name === "patch")
//     merged.edit = value`, which folds all three into one edit permission.
//   - "apply_patch" is the same operation's name on opencode's OpenAI/Codex
//     compatibility path and is included defensively for the same reason.
var opencodeEditingTools = map[string]bool{"edit": true, "write": true, "patch": true, "apply_patch": true}

// opencodeStreamRecord is one top-level line of `opencode run --format
// json`. Verified against the installed CLI (1.18.21) by reading its
// bundle: every line the CLI's own print loop writes has the shape
// `{"type":<name>,"timestamp":<ms>,"sessionID":<id>,...extra}`, produced by
// a single internal helper (`Z(name, extra)`) that every event funnels
// through. `extra` is `{part: J}` for "step_start", "step_finish", "text",
// "reasoning" and "tool_use"; a distinct "error" type carries `{error:
// ...}` instead of "part" and is out of scope for this decoder (it reports
// session-level failures, not per-record progress) and simply falls
// through the default case below like any other unrecognized type.
type opencodeStreamRecord struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Part      json.RawMessage `json:"part"`
}

// opencodeToolTime is the start/end pair opencode attaches to a tool
// part's terminal state. Both fields are plain JSON numbers (unlike
// cursor-agent's quoted millisecond timestamps), matching this record's
// own top-level "timestamp" field.
type opencodeToolTime struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// opencodeToolState is a "tool" part's "state" object. Verified against
// the installed CLI (1.18.21): a tool call's state progresses
// pending -> running -> completed|error, but only the terminal two states
// (status "completed" or "error") are ever forwarded to the `--format
// json` stream -- the CLI's own print loop guards the "tool_use" emission
// on exactly `state.status==="completed"||state.status==="error"`, and
// the one path that does print a "running" tool (the "task" tool, for
// TUI display) explicitly excludes `format==="json"`. By the time this
// record arrives, both state.time.start and state.time.end already exist.
//
// FilePath and Filepath decode the two spellings the real CLI's own
// path-extraction helper tolerates for "read"/"edit"/"write" tool input
// (`input.filePath ?? input.filepath`); only the first is expected in
// practice but both are decoded defensively.
type opencodeToolState struct {
	Status string `json:"status"`
	Input  struct {
		FilePath string `json:"filePath"`
		Filepath string `json:"filepath"`
	} `json:"input"`
	// Error is decoded as a plain string because every real construction
	// site observed in the installed CLI assigns a string here (a
	// message, or a stringified exception) rather than a structured
	// object. If some future build serializes a structured error instead,
	// this field fails to decode, the whole part unmarshal fails, and the
	// record falls back to a degraded-note-and-continue -- never a hard
	// dispatch failure -- so this is a safe, not a silent, assumption.
	Error string           `json:"error"`
	Time  opencodeToolTime `json:"time"`
}

// opencodeTokens is the per-step token usage opencode reports on a
// "step-finish" part. Verified against the installed CLI (1.18.21)'s own
// schema for that part: `tokens: {input, output, reasoning, cache: {read,
// write}}` (total is also present but redundant with input+output+
// reasoning and is not decoded here).
type opencodeTokens struct {
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

// opencodeStreamPart is the union of every part shape that carries
// progress: "text" (Text), "step-finish" (Reason, Tokens, Cost) and "tool"
// (Tool, State). A part of any other shape simply leaves the fields it
// does not carry at their zero value, the same union-struct approach
// claude_stream.go's claudeContentBlock already takes for the same reason.
type opencodeStreamPart struct {
	Text   string            `json:"text"`
	Reason string            `json:"reason"`
	Cost   float64           `json:"cost"`
	Tokens *opencodeTokens   `json:"tokens"`
	Tool   string            `json:"tool"`
	State  opencodeToolState `json:"state"`
}

// decodeOpencodeToolUse turns one "tool_use" part into the timeline events
// its embedded timestamps allow. Because opencode only ever emits this
// record once a call has already reached a terminal state, a single
// record is expanded into two events -- started (backdated to
// state.time.start) and finished or failed (at state.time.end) -- rather
// than reporting only the terminal event at its own arrival time, which
// would understate how long the call actually ran and erase exactly the
// tool-latency-versus-silence signal this decoder exists to produce.
//
// A part naming no tool, or reporting a status other than "completed" or
// "error" (which the installed CLI never forwards to this stream, but a
// future build might), is skipped rather than guessed at.
func decodeOpencodeToolUse(part opencodeStreamPart) []ProgressEvent {
	name := part.Tool
	if name == "" {
		return nil
	}
	if part.State.Status != "completed" && part.State.Status != "error" {
		return nil
	}

	kind := "Tool"
	if opencodeEditingTools[name] {
		kind = "Edit"
	}
	path := part.State.Input.FilePath
	if path == "" {
		path = part.State.Input.Filepath
	}
	detail := name
	if path != "" {
		detail += " (" + path + ")"
	}

	start := time.UnixMilli(part.State.Time.Start)
	end := time.UnixMilli(part.State.Time.End)
	events := []ProgressEvent{{Message: fmt.Sprintf("%s started: %s", kind, detail), At: start}}

	if part.State.Status == "error" {
		message := fmt.Sprintf("%s failed: %s", kind, detail)
		if part.State.Error != "" {
			message += ": " + part.State.Error
		}
		return append(events, ProgressEvent{Message: message, At: end})
	}
	return append(events, ProgressEvent{Message: fmt.Sprintf("%s finished: %s", kind, detail), At: end})
}

// formatOpencodeUsage mirrors formatClaudeUsage's and formatAgyUsage's
// shape so the usage line reads consistently across executors: token
// breakdown first, then a dollar cost only when opencode reports one.
func formatOpencodeUsage(tokens opencodeTokens, cost float64) string {
	message := fmt.Sprintf("Usage: %d input, %d output, %d reasoning, %d cache read, %d cache write tokens",
		tokens.Input, tokens.Output, tokens.Reasoning, tokens.Cache.Read, tokens.Cache.Write)
	if cost > 0 {
		message += fmt.Sprintf(", $%.4f", cost)
	}
	return message
}

// decodeOpencodeRecord normalizes one JSON-line record observed from
// opencode 1.18.21 into the ordered timeline events it carries. A record
// legitimately expands into more than one event: "tool_use" always yields
// a started and a finished/failed pair (see decodeOpencodeToolUse), and
// "step_finish" yields its step-boundary message plus a usage line
// whenever opencode reports token usage for that step. Unknown record
// types are deliberately ignored so a new telemetry record cannot affect
// dispatch success or final-result capture.
func decodeOpencodeRecord(data []byte) ([]ProgressEvent, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var record opencodeStreamRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode opencode progress record: %w", err)
	}

	at := time.UnixMilli(record.Timestamp)
	switch record.Type {
	case "step_start":
		return []ProgressEvent{{Message: "Step started", At: at}}, nil
	case "text":
		var part opencodeStreamPart
		if err := json.Unmarshal(record.Part, &part); err != nil {
			return nil, fmt.Errorf("decode opencode text part: %w", err)
		}
		return []ProgressEvent{{Message: part.Text, At: at}}, nil
	case "step_finish":
		var part opencodeStreamPart
		if err := json.Unmarshal(record.Part, &part); err != nil {
			return nil, fmt.Errorf("decode opencode finish part: %w", err)
		}
		message := "Step finished"
		if part.Reason != "" {
			message += ": " + part.Reason
		}
		events := []ProgressEvent{{Message: message, At: at}}
		if part.Tokens != nil {
			events = append(events, ProgressEvent{Message: formatOpencodeUsage(*part.Tokens, part.Cost), At: at})
		}
		return events, nil
	case "tool_use":
		var part opencodeStreamPart
		if err := json.Unmarshal(record.Part, &part); err != nil {
			return nil, fmt.Errorf("decode opencode tool part: %w", err)
		}
		return decodeOpencodeToolUse(part), nil
	default:
		return nil, nil
	}
}

// opencodeStreamDecoder receives stdout writes from os/exec, splits
// JSON-line records across arbitrary write boundaries, and emits
// best-effort progress. A malformed record is noted once (see
// opencodeStreamDegradedMessage) but never disables decoding; Write keeps
// accepting all bytes so the caller's raw Outcome.Stdout capture remains
// authoritative regardless of decoder state.
type opencodeStreamDecoder struct {
	progress chan<- ProgressEvent
	pending  []byte
	noted    bool
}

func newOpencodeStreamDecoder(progress chan<- ProgressEvent) *opencodeStreamDecoder {
	return &opencodeStreamDecoder{progress: progress}
}

func (d *opencodeStreamDecoder) Write(p []byte) (int, error) {
	d.pending = append(d.pending, p...)
	for {
		newline := bytes.IndexByte(d.pending, '\n')
		if newline < 0 {
			break
		}
		d.consume(d.pending[:newline])
		d.pending = d.pending[newline+1:]
	}
	return len(p), nil
}

func (d *opencodeStreamDecoder) finish() {
	if len(bytes.TrimSpace(d.pending)) == 0 {
		return
	}
	d.consume(d.pending)
	d.pending = nil
}

func (d *opencodeStreamDecoder) consume(record []byte) {
	events, err := decodeOpencodeRecord(record)
	if err != nil {
		d.note()
		return
	}
	for _, event := range events {
		d.emit(event)
	}
}

// note emits the degraded-decoding message at most once per dispatch, so a
// lane with several unreadable lines does not flood the timeline with
// repeats of the same notice.
func (d *opencodeStreamDecoder) note() {
	if d.noted {
		return
	}
	d.noted = true
	d.emit(ProgressEvent{Message: opencodeStreamDegradedMessage, At: time.Now()})
}

func (d *opencodeStreamDecoder) emit(event ProgressEvent) {
	select {
	case d.progress <- event:
	default:
		// Progress is best-effort. A slow consumer must never backpressure the
		// child process or make telemetry part of lane success.
	}
}
