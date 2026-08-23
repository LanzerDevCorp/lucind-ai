package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cursorToolCallSuffix is the suffix every tool-call variant key carries in
// cursor-agent's stream. The installed CLI (2026.08.11-e8db854) ships at least
// shell, read, grep, ls, glob, edit, delete, semSearch, webSearch, mcp,
// updateTodos, readLints, task, createPlan and extract; the suffix is stripped
// to leave the bare tool name for the progress line.
const cursorToolCallSuffix = "ToolCall"

// cursorEditingTools are the variants that mutate a file on disk, and so are
// reported under the "Edit" kind rather than the generic "Tool" kind -- the
// same distinction agy's and claude's decoders already draw, so one timeline
// reads consistently across all four executors.
var cursorEditingTools = map[string]bool{"edit": true, "delete": true}

// cursorToolPayload is the per-variant body of a tool_call record. Only the
// fields that carry progress are decoded: the path being operated on, and the
// error a completed call failed with. Everything else a variant reports is
// deliberately ignored so a new field cannot affect decoding.
type cursorToolPayload struct {
	Args struct {
		Path string `json:"path"`
	} `json:"args"`
	Result struct {
		Error *struct {
			Error             string `json:"error"`
			ModelVisibleError string `json:"modelVisibleError"`
		} `json:"error"`
	} `json:"result"`
}

// cursorToolRecord is a "tool_call" record. The tool_call object carries
// exactly one key, which names the variant. Timestamps arrive as JSON strings
// rather than numbers on this CLI, so they are decoded permissively.
type cursorToolRecord struct {
	CallID      string                     `json:"call_id"`
	ToolCall    map[string]json.RawMessage `json:"tool_call"`
	StartedAtMS json.RawMessage            `json:"startedAtMs"`
	EndedAtMS   json.RawMessage            `json:"completedAtMs"`
}

// variant returns the single key of the tool_call object with its suffix
// stripped, plus its raw payload. It reports false when the record does not
// carry exactly one variant, which is the only shape this decoder understands.
func (r cursorToolRecord) variant() (name string, payload json.RawMessage, ok bool) {
	if len(r.ToolCall) != 1 {
		return "", nil, false
	}
	for key, raw := range r.ToolCall {
		return strings.TrimSuffix(key, cursorToolCallSuffix), raw, true
	}
	return "", nil, false
}

// cursorToolTracker remembers the path a started call named so a completed
// record can be reported against the same file. cursor-agent omits args on the
// completed record when a call fails, and "Edit failed: edit" without the file
// is not something a reader can act on.
//
// Calls are keyed by call_id when the stream supplies one and by variant name
// otherwise. The name fallback assumes calls of the same variant do not
// interleave, which holds for a single lane's sequential tool use; a wrong
// path there would only mislabel a progress line, never affect dispatch.
type cursorToolTracker struct {
	paths map[string]string
}

func newCursorToolTracker() *cursorToolTracker {
	return &cursorToolTracker{paths: map[string]string{}}
}

func (t *cursorToolTracker) key(record cursorToolRecord, name string) string {
	if record.CallID != "" {
		return record.CallID
	}
	return name
}

// decodeCursorToolCall turns one tool_call record into a progress message. It
// reports false for any record it does not recognize, so an unfamiliar shape
// is skipped rather than reported as activity that did not happen.
func (t *cursorToolTracker) decodeCursorToolCall(subtype string, record cursorToolRecord) (string, time.Time, bool) {
	name, raw, ok := record.variant()
	if !ok || name == "" {
		return "", time.Time{}, false
	}

	var payload cursorToolPayload
	if len(raw) > 0 {
		// A variant body that does not decode still names a real tool, so the
		// lifecycle line is worth emitting without its path detail.
		_ = json.Unmarshal(raw, &payload)
	}

	key := t.key(record, name)
	path := payload.Args.Path
	switch subtype {
	case "started":
		if path != "" {
			t.paths[key] = path
		}
	case "completed":
		if path == "" {
			path = t.paths[key]
		}
		delete(t.paths, key)
	default:
		return "", time.Time{}, false
	}

	kind := "Tool"
	if cursorEditingTools[name] {
		kind = "Edit"
	}
	detail := name
	if path != "" {
		detail += " (" + path + ")"
	}

	at := cursorToolTimestamp(record, subtype)
	if subtype == "started" {
		return fmt.Sprintf("%s started: %s", kind, detail), at, true
	}
	if payload.Result.Error != nil {
		reason := payload.Result.Error.ModelVisibleError
		if reason == "" {
			reason = payload.Result.Error.Error
		}
		if reason != "" {
			return fmt.Sprintf("%s failed: %s: %s", kind, detail, reason), at, true
		}
		return fmt.Sprintf("%s failed: %s", kind, detail), at, true
	}
	return fmt.Sprintf("%s finished: %s", kind, detail), at, true
}

func cursorToolTimestamp(record cursorToolRecord, subtype string) time.Time {
	raw := record.StartedAtMS
	if subtype == "completed" && len(record.EndedAtMS) > 0 {
		raw = record.EndedAtMS
	}
	if ms, ok := parseCursorMillis(raw); ok {
		return time.UnixMilli(ms)
	}
	return time.Time{}
}

// parseCursorMillis accepts a millisecond timestamp encoded either as a JSON
// number or as a JSON string. cursor-agent quotes these fields on tool_call
// records while leaving timestamp_ms unquoted elsewhere in the same stream,
// so both spellings have to be tolerated.
func parseCursorMillis(raw json.RawMessage) (int64, bool) {
	trimmed := bytes.Trim(bytes.TrimSpace(raw), `"`)
	if len(trimmed) == 0 {
		return 0, false
	}
	ms, err := strconv.ParseInt(string(trimmed), 10, 64)
	if err != nil {
		return 0, false
	}
	return ms, true
}
