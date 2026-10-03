package usagelog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RouterSignals holds the numeric/boolean signals for a router event.
type RouterSignals struct {
	AllowedPathCount int  `json:"allowed_path_count"`
	NewFile          bool `json:"new_file"`
	RiskTierLevel    int  `json:"risk_tier_level"`
	ReadOnly         bool `json:"read_only"`
}

// RouterEvent is a shadow router log entry written to usage.jsonl.
type RouterEvent struct {
	TS             time.Time      `json:"ts"`
	Kind           string         `json:"kind"`
	PrimaryRoute   string         `json:"primary_route,omitempty"`
	CandidateRoute string         `json:"candidate_route,omitempty"`
	Confidence     float64        `json:"confidence,omitempty"`
	ErrorKind      string         `json:"error_kind,omitempty"`
	Signals        *RouterSignals `json:"signals,omitempty"`
}

// Record is one executor attempt log entry in usage.jsonl.
type Record struct {
	Kind            string         `json:"kind,omitempty"`
	PrimaryRoute    string         `json:"primary_route,omitempty"`
	CandidateRoute  string         `json:"candidate_route,omitempty"`
	Confidence      float64        `json:"confidence,omitempty"`
	ErrorKind       string         `json:"error_kind,omitempty"`
	Signals         *RouterSignals `json:"signals,omitempty"`
	TS              time.Time      `json:"ts"`
	RunID           string         `json:"run_id"`
	LaneID          string         `json:"lane_id"`
	Attempt         int            `json:"attempt"`
	Executor        string         `json:"executor"`
	Provider        string         `json:"provider"`
	Model           string         `json:"model"`
	LaneRole        string         `json:"lane_role"`
	InputTokens     int64          `json:"input_tokens"`
	OutputTokens    int64          `json:"output_tokens"`
	ThinkingTokens  int64          `json:"thinking_tokens"`
	CacheReadTokens int64          `json:"cache_read_tokens"`
	TotalTokens     int64          `json:"total_tokens"`
	CostUSD         float64        `json:"cost_usd"`
	TokensKnown     bool           `json:"tokens_known"`
	DurationMS      int64          `json:"duration_ms"`
	ExitCode        int            `json:"exit_code"`
	TimedOut        bool           `json:"timed_out"`
	Status          string         `json:"status"`
}

// Usage holds parsed token and cost metrics extracted from an executor's stdout.
type Usage struct {
	InputTokens     int64
	OutputTokens    int64
	ThinkingTokens  int64
	CacheReadTokens int64
	TotalTokens     int64
	CostUSD         float64
}

// DefaultPath returns the default location of the usage log:
// $XDG_STATE_HOME/lucind-ai/usage.jsonl, fallback ~/.local/state/lucind-ai/usage.jsonl.
func DefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lucind-ai", "usage.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "lucind-ai", "usage.jsonl"), nil
}

// Provider maps an executor name to its canonical provider identifier:
// "agy" and "herdr-agy" => "agy", "cursor-agent" => "cursor", "claude" => "claude",
// "opencode" => "opencode", anything else => executor name.
func Provider(executor string) string {
	switch executor {
	case "agy", "herdr-agy":
		return "agy"
	case "cursor-agent":
		return "cursor"
	case "claude":
		return "claude"
	case "opencode":
		return "opencode"
	default:
		return executor
	}
}

type rawPayload struct {
	TotalCostUSD *float64  `json:"total_cost_usd"`
	CostUSD      *float64  `json:"cost_usd"`
	Usage        *rawUsage `json:"usage"`
}

type rawUsage struct {
	InputTokens              *int64   `json:"input_tokens"`
	OutputTokens             *int64   `json:"output_tokens"`
	ThinkingTokens           *int64   `json:"thinking_tokens"`
	CacheReadTokens          *int64   `json:"cache_read_tokens"`
	CacheReadInputTokens     *int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens *int64   `json:"cache_creation_input_tokens"`
	TotalTokens              *int64   `json:"total_tokens"`
	TotalCostUSD             *float64 `json:"total_cost_usd"`
	CostUSD                  *float64 `json:"cost_usd"`
}

// ExtractUsage performs a best-effort parse of the executor's final JSON on stdout:
// accepts an object with a usage object containing token counts, and cost at top level
// or within usage. Returns ok=false on unparseable input. Never panics on arbitrary input.
func ExtractUsage(stdout string) (u Usage, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			u = Usage{}
			ok = false
		}
	}()

	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return Usage{}, false
	}

	// Try the entire stdout as a single JSON object first
	if parsed, parsedOk := parseUsageJSON([]byte(trimmed)); parsedOk {
		return parsed, true
	}

	// If stdout is NDJSON/stream output, check lines in reverse order for the final result object
	lines := strings.Split(trimmed, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if parsed, parsedOk := parseUsageJSON([]byte(line)); parsedOk {
			return parsed, true
		}
	}

	return Usage{}, false
}

func parseUsageJSON(data []byte) (Usage, bool) {
	var raw rawPayload
	if err := json.Unmarshal(data, &raw); err != nil {
		return Usage{}, false
	}

	if raw.Usage == nil {
		return Usage{}, false
	}

	ru := raw.Usage
	hasTokens := ru.InputTokens != nil ||
		ru.OutputTokens != nil ||
		ru.ThinkingTokens != nil ||
		ru.CacheReadTokens != nil ||
		ru.CacheReadInputTokens != nil ||
		ru.CacheCreationInputTokens != nil ||
		ru.TotalTokens != nil

	if !hasTokens {
		return Usage{}, false
	}

	var u Usage
	if ru.InputTokens != nil {
		u.InputTokens = *ru.InputTokens
	}
	if ru.OutputTokens != nil {
		u.OutputTokens = *ru.OutputTokens
	}
	if ru.ThinkingTokens != nil {
		u.ThinkingTokens = *ru.ThinkingTokens
	}
	if ru.CacheReadTokens != nil {
		u.CacheReadTokens = *ru.CacheReadTokens
	} else if ru.CacheReadInputTokens != nil {
		u.CacheReadTokens = *ru.CacheReadInputTokens
	}
	if ru.CacheCreationInputTokens != nil {
		// fold creation into input
		u.InputTokens += *ru.CacheCreationInputTokens
	}

	if ru.TotalTokens != nil {
		u.TotalTokens = *ru.TotalTokens
	} else {
		// total_tokens absent: compute as sum of parts
		u.TotalTokens = u.InputTokens + u.OutputTokens + u.CacheReadTokens
	}

	if raw.TotalCostUSD != nil {
		u.CostUSD = *raw.TotalCostUSD
	} else if raw.CostUSD != nil {
		u.CostUSD = *raw.CostUSD
	} else if ru.TotalCostUSD != nil {
		u.CostUSD = *ru.TotalCostUSD
	} else if ru.CostUSD != nil {
		u.CostUSD = *ru.CostUSD
	}

	return u, true
}

// Append appends a single Record as a JSON line to path.
// Directory is created with mode 0700 and file with mode 0600.
// The record is written in a single Write call to prevent concurrent line interleaving.
func Append(path string, r Record) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create usage log dir %q: %w", dir, err)
	}

	if r.TS.IsZero() {
		r.TS = time.Now().UTC()
	} else {
		r.TS = r.TS.UTC()
	}

	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal usage record: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open usage log file %q: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write usage log record to %q: %w", path, err)
	}
	return nil
}

// ReadAll reads all records from path line by line.
// Skips blank lines, skips and counts malformed lines.
// A missing file returns nil, 0, nil.
func ReadAll(path string) ([]Record, int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("open usage log %q: %w", path, err)
	}
	defer f.Close()

	var records []Record
	var skipped int

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			skipped++
			continue
		}
		records = append(records, r)
	}

	if err := scanner.Err(); err != nil {
		return records, skipped, fmt.Errorf("scan usage log %q: %w", path, err)
	}
	return records, skipped, nil
}

// AppendRouterEvent appends a single RouterEvent as a JSON line to path.
// Directory is created with mode 0700 and file with mode 0600.
// The event is written in a single Write call to prevent concurrent line interleaving.
func AppendRouterEvent(path string, ev RouterEvent) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create usage log dir %q: %w", dir, err)
	}

	if ev.TS.IsZero() {
		ev.TS = time.Now().UTC()
	} else {
		ev.TS = ev.TS.UTC()
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal router event: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open usage log file %q: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write router event to %q: %w", path, err)
	}
	return nil
}
