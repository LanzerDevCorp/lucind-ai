package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// claudeStreamDegradedMessage is emitted once, on the first stdout line that
// does not decode, so a degraded timeline is visibly degraded instead of
// merely sparse. Decoding is not switched off afterwards: the raw stream is
// always what Outcome.Stdout falls back to, and one unreadable line must not
// blind the rest of a dispatch's timeline.
const claudeStreamDegradedMessage = "Claude progress decoding degraded; raw stream retained as stdout"

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// claudeStreamRecord is one top-level line of `--output-format stream-json`.
// The record types claude emits are "system", "assistant", "user",
// "stream_event" and "result"; only the middle two carry incremental
// activity and only "result" is terminal. Every other type -- including
// types added later -- is ignored on purpose, so a new telemetry record can
// never affect dispatch success or final-result capture.
type claudeStreamRecord struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Message json.RawMessage `json:"message"`
	Usage   *claudeUsage    `json:"usage"`
	CostUSD float64         `json:"total_cost_usd"`
}

// claudeMessage is the Anthropic message envelope carried by "assistant" and
// "user" records. Content is kept raw because a "user" record may legally
// carry a plain string instead of a block array; that shape carries no
// progress and is skipped rather than treated as a decode failure.
type claudeMessage struct {
	Content json.RawMessage `json:"content"`
}

// claudeContentBlock is one content block. The fields are the union of the
// block shapes that carry progress: "text" (assistant prose), "tool_use"
// (a tool starting) and "tool_result" (that same tool finishing). Blocks of
// any other type, "thinking" included, are skipped -- private reasoning is
// not dispatch progress and does not belong in a shared ledger.
type claudeContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	ToolUseID string `json:"tool_use_id"`
	IsError   bool   `json:"is_error"`
	Input     struct {
		FilePath string `json:"file_path"`
	} `json:"input"`
}

// claudeToolCall is what a started tool needs to be named again when its
// result arrives: a tool_result block carries only the tool_use_id, never
// the tool's name, so the pairing has to be remembered on this side.
type claudeToolCall struct {
	kind   string
	detail string
}

// claudeStreamDecoder accepts arbitrary stdout write boundaries while
// retaining the terminal result record separately from incremental progress.
type claudeStreamDecoder struct {
	progress chan<- ProgressEvent
	pending  []byte
	terminal bool
	result   []byte
	noted    bool
	tools    map[string]claudeToolCall
}

func newClaudeStreamDecoder(progress chan<- ProgressEvent) *claudeStreamDecoder {
	return &claudeStreamDecoder{progress: progress, tools: map[string]claudeToolCall{}}
}

func (d *claudeStreamDecoder) Write(p []byte) (int, error) {
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

func (d *claudeStreamDecoder) finish() {
	if len(bytes.TrimSpace(d.pending)) > 0 {
		d.consume(d.pending)
	}
	d.pending = nil
}

func (d *claudeStreamDecoder) consume(line []byte) {
	if d.terminal || len(bytes.TrimSpace(line)) == 0 {
		return
	}

	var record claudeStreamRecord
	if err := json.Unmarshal(line, &record); err != nil {
		d.note()
		return
	}

	switch record.Type {
	case "assistant":
		d.consumeBlocks(record.Message, d.consumeAssistantBlock)
	case "user":
		d.consumeBlocks(record.Message, d.consumeUserBlock)
	case "result":
		d.consumeResult(line, record)
	default:
		// "system", "stream_event", and any record type added later.
	}
}

func (d *claudeStreamDecoder) consumeBlocks(raw json.RawMessage, consume func(claudeContentBlock)) {
	var message claudeMessage
	if !jsonObject(raw) || json.Unmarshal(raw, &message) != nil {
		return
	}
	if trimmed := bytes.TrimSpace(message.Content); len(trimmed) == 0 || trimmed[0] != '[' {
		// A string content body carries no per-block progress.
		return
	}

	var blocks []claudeContentBlock
	if json.Unmarshal(message.Content, &blocks) != nil {
		return
	}
	for _, block := range blocks {
		consume(block)
	}
}

func (d *claudeStreamDecoder) consumeAssistantBlock(block claudeContentBlock) {
	switch block.Type {
	case "text":
		if block.Text != "" {
			d.emit(block.Text)
		}
	case "tool_use":
		if block.Name == "" {
			return
		}
		call := claudeToolCall{kind: "Tool", detail: block.Name}
		if block.Input.FilePath != "" {
			call.kind = "Edit"
			call.detail += " (" + block.Input.FilePath + ")"
		}
		if block.ID != "" {
			d.tools[block.ID] = call
		}
		d.emit(fmt.Sprintf("%s started: %s", call.kind, call.detail))
	}
}

func (d *claudeStreamDecoder) consumeUserBlock(block claudeContentBlock) {
	if block.Type != "tool_result" {
		return
	}
	call, ok := d.tools[block.ToolUseID]
	if !ok {
		// The matching tool_use never arrived (a resumed session, or a
		// dropped line). Nothing truthful can be said about which tool
		// this was, so say nothing rather than guess a name.
		return
	}
	delete(d.tools, block.ToolUseID)

	action := "finished"
	if block.IsError {
		action = "failed"
	}
	d.emit(fmt.Sprintf("%s %s: %s", call.kind, action, call.detail))
}

func (d *claudeStreamDecoder) consumeResult(line []byte, record claudeStreamRecord) {
	if record.Subtype == "" {
		return
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, line); err != nil {
		return
	}
	d.result = append(compact.Bytes(), '\n')
	d.terminal = true

	if record.Subtype != "success" {
		// claude exits 0 on error_max_turns and error_during_execution, so
		// a run that stopped short of finishing its work is indistinguishable
		// from a completed one unless the subtype is said out loud.
		d.emit("Result: " + record.Subtype)
	}
	if record.Usage != nil {
		d.emit(formatClaudeUsage(*record.Usage, record.CostUSD))
	}
}

func formatClaudeUsage(usage claudeUsage, costUSD float64) string {
	message := fmt.Sprintf("Usage: %d input, %d output, %d cache read, %d cache write tokens",
		usage.InputTokens, usage.OutputTokens, usage.CacheReadInputTokens, usage.CacheCreationInputTokens)
	if costUSD > 0 {
		message += fmt.Sprintf(", $%.4f", costUSD)
	}
	return message
}

func (d *claudeStreamDecoder) note() {
	if d.noted {
		return
	}
	d.noted = true
	d.emit(claudeStreamDegradedMessage)
}

func (d *claudeStreamDecoder) emit(message string) {
	select {
	case d.progress <- ProgressEvent{Message: message, At: time.Now()}:
	default:
		// Progress is best-effort and must never backpressure the child process.
	}
}
