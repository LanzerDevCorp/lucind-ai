package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const agyStreamFallbackMessage = "Agy stream unavailable; retrying blocking JSON"

type agyUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ThinkingTokens  int `json:"thinking_tokens"`
	CacheReadTokens int `json:"cache_read_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

type agyStreamRecord struct {
	Event      string          `json:"event"`
	Init       json.RawMessage `json:"init"`
	StepUpdate json.RawMessage `json:"step_update"`
	Result     json.RawMessage `json:"result"`
}

type agyStepUpdate struct {
	State     string    `json:"state"`
	StepType  string    `json:"step_type"`
	TextDelta string    `json:"text_delta"`
	ToolName  string    `json:"tool_name"`
	Usage     *agyUsage `json:"usage"`
	ToolInfo  struct {
		Name       string `json:"name"`
		Parameters struct {
			TargetFile string `json:"TargetFile"`
		} `json:"parameters"`
	} `json:"tool_info"`
}

type agyResult struct {
	Status string    `json:"status"`
	Usage  *agyUsage `json:"usage"`
}

// agyStreamDecoder accepts arbitrary stdout write boundaries while retaining
// the terminal result separately from incremental progress records.
type agyStreamDecoder struct {
	progress  chan<- ProgressEvent
	pending   []byte
	terminal  bool
	result    []byte
	toolCalls int64
}

func newAgyStreamDecoder(progress chan<- ProgressEvent) *agyStreamDecoder {
	return &agyStreamDecoder{progress: progress}
}

func (d *agyStreamDecoder) Write(p []byte) (int, error) {
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

func (d *agyStreamDecoder) finish() {
	if len(bytes.TrimSpace(d.pending)) > 0 {
		d.consume(d.pending)
	}
	d.pending = nil
}

func (d *agyStreamDecoder) consume(line []byte) {
	if d.terminal || len(bytes.TrimSpace(line)) == 0 {
		return
	}

	var record agyStreamRecord
	if err := json.Unmarshal(line, &record); err != nil || record.Event == "" {
		return
	}

	switch record.Event {
	case "init":
	case "step_update":
		d.consumeStep(record.StepUpdate)
	case "result":
		d.consumeResult(record.Result)
	default:
		// New top-level telemetry records are valid protocol extensions. They do
		// not affect final-result capture and are intentionally ignored.
	}
}

func (d *agyStreamDecoder) consumeStep(raw json.RawMessage) {
	var step agyStepUpdate
	if !jsonObject(raw) || json.Unmarshal(raw, &step) != nil || step.StepType == "" || step.State == "" {
		return
	}

	switch step.StepType {
	case "agent_response":
		if step.TextDelta != "" {
			d.emit(step.TextDelta, 0, 0)
		}
		if step.Usage != nil {
			d.emit(formatAgyUsage(*step.Usage), int64(step.Usage.TotalTokens), 0)
		}
	case "tool":
		name := step.ToolName
		if name == "" {
			name = step.ToolInfo.Name
		}
		if name == "" {
			return
		}
		kind := "Tool"
		suffix := ""
		if name == "write_to_file" || name == "replace_file_content" {
			kind = "Edit"
			if step.ToolInfo.Parameters.TargetFile != "" {
				suffix = " (" + step.ToolInfo.Parameters.TargetFile + ")"
			}
		}
		action := map[string]string{"ACTIVE": "started", "DONE": "finished", "ERROR": "failed"}[step.State]
		if action != "" {
			if step.State == "ACTIVE" {
				d.toolCalls++
			}
			d.emit(fmt.Sprintf("%s %s: %s%s", kind, action, name, suffix), 0, 0)
		}
	}
}

func (d *agyStreamDecoder) consumeResult(raw json.RawMessage) {
	var result agyResult
	if !jsonObject(raw) || json.Unmarshal(raw, &result) != nil || result.Status == "" {
		return
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return
	}
	d.result = append(compact.Bytes(), '\n')
	d.terminal = true
	if result.Usage != nil {
		d.emit(formatAgyUsage(*result.Usage), int64(result.Usage.TotalTokens), 0)
	}
}

func jsonObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}

func formatAgyUsage(usage agyUsage) string {
	return fmt.Sprintf("Usage: %d input, %d output, %d thinking, %d cache read, %d total tokens",
		usage.InputTokens, usage.OutputTokens, usage.ThinkingTokens, usage.CacheReadTokens, usage.TotalTokens)
}

func (d *agyStreamDecoder) emit(message string, totalTokens int64, costUSD float64) {
	select {
	case d.progress <- ProgressEvent{
		Message:     message,
		At:          time.Now(),
		TotalTokens: totalTokens,
		CostUSD:     costUSD,
		ToolCalls:   d.toolCalls,
	}:
	default:
		// Progress is best-effort and must never backpressure the child process.
	}
}

func emitAgyProgress(progress chan<- ProgressEvent, message string) {
	select {
	case progress <- ProgressEvent{Message: message, At: time.Now()}:
	default:
	}
}
