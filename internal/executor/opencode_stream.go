package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const opencodeDecodeFallbackMessage = "Opencode progress decoding disabled; continuing with raw JSON result"

type opencodeStreamRecord struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Part      json.RawMessage `json:"part"`
}

type opencodeStreamPart struct {
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// decodeOpencodeRecord normalizes the JSON-line records observed from
// opencode 1.18.21. Unknown record types are deliberately ignored so a new
// telemetry record cannot affect dispatch success or final-result capture.
func decodeOpencodeRecord(data []byte) (ProgressEvent, bool, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return ProgressEvent{}, false, nil
	}

	var record opencodeStreamRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return ProgressEvent{}, false, fmt.Errorf("decode opencode progress record: %w", err)
	}

	event := ProgressEvent{At: time.UnixMilli(record.Timestamp)}
	switch record.Type {
	case "step_start":
		event.Message = "Step started"
	case "text":
		var part opencodeStreamPart
		if err := json.Unmarshal(record.Part, &part); err != nil {
			return ProgressEvent{}, false, fmt.Errorf("decode opencode text part: %w", err)
		}
		event.Message = part.Text
	case "step_finish":
		var part opencodeStreamPart
		if err := json.Unmarshal(record.Part, &part); err != nil {
			return ProgressEvent{}, false, fmt.Errorf("decode opencode finish part: %w", err)
		}
		event.Message = "Step finished"
		if part.Reason != "" {
			event.Message += ": " + part.Reason
		}
	default:
		return ProgressEvent{}, false, nil
	}

	return event, true, nil
}

// opencodeStreamDecoder receives stdout writes from os/exec, splits JSON-line
// records across arbitrary write boundaries, and emits best-effort progress.
// A malformed record disables only telemetry decoding; Write keeps accepting
// all bytes so the caller's raw Outcome.Stdout capture remains authoritative.
type opencodeStreamDecoder struct {
	progress chan<- ProgressEvent
	pending  []byte
	disabled bool
}

func newOpencodeStreamDecoder(progress chan<- ProgressEvent) *opencodeStreamDecoder {
	return &opencodeStreamDecoder{progress: progress}
}

func (d *opencodeStreamDecoder) Write(p []byte) (int, error) {
	if d.disabled {
		return len(p), nil
	}

	d.pending = append(d.pending, p...)
	for {
		newline := bytes.IndexByte(d.pending, '\n')
		if newline < 0 {
			break
		}
		d.consume(d.pending[:newline])
		d.pending = d.pending[newline+1:]
		if d.disabled {
			d.pending = nil
			break
		}
	}
	return len(p), nil
}

func (d *opencodeStreamDecoder) finish() {
	if d.disabled || len(bytes.TrimSpace(d.pending)) == 0 {
		return
	}
	d.consume(d.pending)
	d.pending = nil
}

func (d *opencodeStreamDecoder) consume(record []byte) {
	event, ok, err := decodeOpencodeRecord(record)
	if err != nil {
		d.emit(ProgressEvent{Message: opencodeDecodeFallbackMessage, At: time.Now()})
		d.disabled = true
		return
	}
	if !ok {
		return
	}

	d.emit(event)
}

func (d *opencodeStreamDecoder) emit(event ProgressEvent) {
	select {
	case d.progress <- event:
	default:
		// Progress is best-effort. A slow consumer must never backpressure the
		// child process or make telemetry part of lane success.
	}
}
