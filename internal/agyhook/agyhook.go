// Package agyhook implements the handlers behind the agy plugin hooks
// (`lucind-ai hook pre-tool-use` and `lucind-ai hook stop`).
//
// Every handler is pass-through when no lane id is supplied (LUCIND_LANE is
// unset), so free or manual agy sessions are never affected. Inside a lane,
// internal errors fail closed for PreToolUse (deny) and end the session for
// Stop, and are logged to .lucind/lanes/<id>/hook.log.
package agyhook

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

// MaxRetries is how many times Stop re-enters the agent loop for a missing
// or schema-invalid result.json before the lane is marked failed.
const (
	MaxRetries        = 2
	RetryQuietWindow  = 60 * time.Second
	MaxTotalContinues = 10
)

var now = time.Now

type toolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type payload struct {
	WorkspacePaths    []string  `json:"workspacePaths"`
	ToolCall          *toolCall `json:"toolCall"`
	ExecutionNum      int       `json:"executionNum"`
	TerminationReason string    `json:"terminationReason"`
	FullyIdle         *bool     `json:"fullyIdle"`
	Error             string    `json:"error"`
	ConversationID    string    `json:"conversationId"`
	TranscriptPath    string    `json:"transcriptPath"`
}

type role string

const (
	roleUnknown role = "unknown"
	roleMain    role = "main"
	roleWorker  role = "worker"
)

type transcriptStep struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

// fileWriteTool matches agy tools that modify a file. Observed names:
// write_to_file, replace_file_content, multi_replace_file_content. The
// pattern is anchored so unrelated tools (browser_move_mouse, run_command)
// are not caught.
var fileWriteTool = regexp.MustCompile(`^(write|edit|replace|multi_replace|delete|remove|rename|patch|apply)[a-z_]*$|^(create|move)_(file|dir)[a-z_]*$`)

// pathKeys are the argument names agy file tools use for their target.
var pathKeys = []string{"TargetFile", "target_file", "AbsolutePath", "FilePath", "file_path", "Path", "path", "File"}

func allow() []byte { return []byte(`{"decision":"allow"}`) }

func deny(reason string) []byte {
	b, _ := json.Marshal(map[string]string{"decision": "deny", "reason": reason})
	return b
}

func end() []byte { return []byte(`{}`) }

func cont(reason string) []byte {
	b, _ := json.Marshal(map[string]string{"decision": "continue", "reason": reason})
	return b
}

// logf appends a line to the lane's hook.log, best effort.
func logf(root, id, format string, a ...any) {
	if root == "" || id == "" || !lane.ValidateID(id) {
		return
	}
	dir := lane.LaneDir(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "hook.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, a...))
}

// laneRoot resolves the lane root: the git toplevel of workspacePaths[0].
func laneRoot(ctx context.Context, p payload) (string, error) {
	if len(p.WorkspacePaths) == 0 || p.WorkspacePaths[0] == "" {
		return "", errors.New("payload has no workspacePaths")
	}
	root, err := attest.RepoToplevel(ctx, p.WorkspacePaths[0])
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	return root, nil
}

// resolve cleans path and resolves symlinks through its deepest existing
// ancestor, so a symlink inside the tree cannot smuggle a write outside it.
func resolve(path string) string {
	path = filepath.Clean(path)
	rest := ""
	cur := path
	for {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return path
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

func targetPath(args map[string]any) string {
	for _, k := range pathKeys {
		if s, ok := args[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// protectedMarkers returns the strings whose presence in any tool argument
// denies the call: the attestation HMAC key and the attestations directory.
func protectedMarkers() []string {
	markers := []string{"lucind-ai/attest.key", "lucind-ai/attestations"}
	if keyPath, err := attest.ResolveKeyPath(); err == nil {
		markers = append(markers, keyPath)
	}
	if dir, err := attest.ResolveStateDir("x"); err == nil {
		markers = append(markers, filepath.Dir(dir))
	}
	return markers
}

// PreToolUse decides whether a tool call may proceed. laneID is the value of
// LUCIND_LANE; stdin is the raw hook payload. It returns the hook response.
func PreToolUse(ctx context.Context, laneID string, stdin []byte) []byte {
	if laneID == "" {
		return allow()
	}
	var p payload
	if err := json.Unmarshal(stdin, &p); err != nil || p.ToolCall == nil {
		return deny("lucind-ai: cannot decode the hook payload inside a lane")
	}
	if !lane.ValidateID(laneID) {
		return deny("lucind-ai: LUCIND_LANE is not a valid lane id")
	}
	root, err := laneRoot(ctx, p)
	if err != nil {
		return deny("lucind-ai: cannot resolve the lane root: " + err.Error())
	}
	l, err := lane.Load(root, laneID)
	if err != nil {
		logf(root, laneID, "pre-tool-use: %v", err)
		return deny("lucind-ai: cannot load the lane: " + err.Error())
	}

	if argsJSON, err := json.Marshal(p.ToolCall.Args); err == nil {
		for _, m := range protectedMarkers() {
			if strings.Contains(string(argsJSON), m) {
				return deny("lucind-ai: access to the attest key or attestations is not allowed inside a lane")
			}
		}
	}

	if !fileWriteTool.MatchString(p.ToolCall.Name) {
		return allow()
	}
	target := targetPath(p.ToolCall.Args)
	if target == "" {
		return deny(fmt.Sprintf("lucind-ai: %s call has no recognizable target path", p.ToolCall.Name))
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	target = resolve(target)

	if target == lane.ResultFilePath(root, l) {
		return allow()
	}
	rel, err := filepath.Rel(root, target)
	outside := err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
	rel = filepath.ToSlash(rel)
	if outside || rel == ".lucind" || strings.HasPrefix(rel, ".lucind/") || !lane.MatchAny(l.Allow, rel) {
		return deny(fmt.Sprintf("lucind-ai: %s is outside the lane's allowed paths %v", target, l.Allow))
	}
	return allow()
}

func classifyConversation(laneID, transcriptPath string) (role, error) {
	if transcriptPath == "" {
		return roleUnknown, errors.New("transcript path is empty")
	}
	if strings.HasPrefix(transcriptPath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			transcriptPath = filepath.Join(home, transcriptPath[2:])
		}
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return roleUnknown, fmt.Errorf("open transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	reader := bufio.NewReader(f)
	line, err := reader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return roleUnknown, fmt.Errorf("read transcript: %w", err)
	}
	if len(bytes.TrimSpace(line)) == 0 {
		return roleUnknown, errors.New("transcript is empty")
	}

	var step transcriptStep
	if err := json.Unmarshal(line, &step); err != nil {
		return roleUnknown, fmt.Errorf("decode transcript step: %w", err)
	}

	briefTarget := ".lucind/lanes/" + laneID + "/brief.md"
	isMainSource := step.Source == "" || step.Source == "USER_EXPLICIT"
	isMain := isMainSource &&
		strings.Contains(step.Content, "Read and follow") &&
		strings.Contains(filepath.ToSlash(step.Content), briefTarget)
	if isMain {
		return roleMain, nil
	}

	isWorker := step.Source == "SYSTEM" ||
		step.Type == "SYSTEM_MESSAGE" ||
		strings.Contains(step.Content, "<SYSTEM_MESSAGE>") ||
		strings.Contains(step.Content, "sender=") ||
		strings.Contains(strings.ToLower(step.Content), "worker")
	if isWorker {
		return roleWorker, nil
	}

	return roleUnknown, errors.New("transcript marker not recognized")
}

func sanitizeConvID(id string) string {
	if id == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func getOrClassifyRole(root, laneID string, p payload) (role, error) {
	convDir := filepath.Join(lane.LaneDir(root, laneID), "conversations")
	convID := sanitizeConvID(p.ConversationID)
	if convID != "" {
		cachePath := filepath.Join(convDir, convID)
		if data, err := os.ReadFile(cachePath); err == nil {
			cached := role(strings.TrimSpace(string(data)))
			if cached == roleMain || cached == roleWorker {
				return cached, nil
			}
		}
	}

	r, err := classifyConversation(laneID, p.TranscriptPath)
	if err != nil {
		return roleUnknown, err
	}

	if convID != "" && (r == roleMain || r == roleWorker) {
		if err := os.MkdirAll(convDir, 0o755); err == nil {
			cachePath := filepath.Join(convDir, convID)
			_ = os.WriteFile(cachePath, []byte(r), 0o644)
		}
	}
	return r, nil
}

// Stop evaluates the lane result when agy stops and either ends the session
// or re-enters the agent loop to fix result.json.
func Stop(ctx context.Context, laneID string, stdin []byte) []byte {
	if laneID == "" {
		return end()
	}
	var p payload
	if err := json.Unmarshal(stdin, &p); err != nil {
		return end()
	}
	if !lane.ValidateID(laneID) {
		return end()
	}
	root, err := laneRoot(ctx, p)
	if err != nil {
		return end()
	}
	var compactBuf bytes.Buffer
	raw := stdin
	if err := json.Compact(&compactBuf, stdin); err == nil {
		raw = compactBuf.Bytes()
	}
	const maxPayloadLog = 4096
	var rawPayload string
	if len(raw) > maxPayloadLog {
		rawPayload = strings.ToValidUTF8(string(raw[:maxPayloadLog]), "") + "[truncated]"
	} else {
		rawPayload = string(raw)
	}
	logf(root, laneID, "stop: raw payload %s", rawPayload)

	idle := "unset"
	if p.FullyIdle != nil {
		if *p.FullyIdle {
			idle = "true"
		} else {
			idle = "false"
		}
	}
	logf(root, laneID, "stop: payload executionNum=%d terminationReason=%s fullyIdle=%s error=%q", p.ExecutionNum, p.TerminationReason, idle, p.Error)

	r, err := getOrClassifyRole(root, laneID, p)
	switch r {
	case roleWorker:
		logf(root, laneID, "stop: ignored worker conversation %s", p.ConversationID)
		return end()
	case roleMain:
		if p.FullyIdle != nil && !*p.FullyIdle {
			logf(root, laneID, "stop: main conversation not fully idle")
			return end()
		}
	case roleUnknown:
		logf(root, laneID, "stop: transcript unreadable or marker unknown (%v), falling back to fullyIdle alone", err)
		if p.FullyIdle != nil && !*p.FullyIdle {
			return end()
		}
	}

	l, err := lane.Load(root, laneID)
	if err != nil {
		logf(root, laneID, "stop: %v", err)
		return end()
	}
	if l.Status != lane.StatusRunning {
		return end()
	}

	resultPath := lane.ResultFilePath(root, l)
	_, readErr := result.Read(os.DirFS(lane.LaneDir(root, laneID)), lane.ResultFileName(l.Turn))
	invalid := readErr != nil

	if invalid {
		if l.LastStopAt != nil && now().Sub(*l.LastStopAt) >= RetryQuietWindow {
			dur := now().Sub(*l.LastStopAt)
			l.Retries = 0
			logf(root, laneID, "stop: retry budget reset after %v without a counted stop", dur)
		}

		if l.Retries < MaxRetries && l.Continues < MaxTotalContinues {
			l.Retries++
			l.Continues++
			stopTime := now().UTC()
			l.LastStopAt = &stopTime
			l.UpdatedAt = now().UTC()
			if err := l.Save(root); err != nil {
				logf(root, laneID, "stop: save retries: %v", err)
				return end()
			}
			msg := readErr.Error()
			if len(msg) > 1024 {
				msg = strings.ToValidUTF8(msg[:1024], "")
			}
			if errors.Is(readErr, fs.ErrNotExist) {
				msg = "the file does not exist"
			}
			logf(root, laneID, "stop: retry %d/%d: %s", l.Retries, MaxRetries, msg)
			return cont(fmt.Sprintf("The result envelope at %s is missing or invalid: %s. Write a valid envelope that satisfies the result schema (see the lucind-result skill) to that exact path before stopping.", resultPath, msg))
		}

		status, err := lane.MarkStopped(root, laneID)
		if err != nil {
			logf(root, laneID, "stop: mark stopped: %v", err)
			return end()
		}
		limit := "retries"
		if l.Continues >= MaxTotalContinues {
			limit = "total continues"
		}
		logf(root, laneID, "stop: lane marked %s (%s, retries=%d)", status, limit, l.Retries)
		return end()
	}

	// Valid result (!invalid)
	status, err := lane.MarkStopped(root, laneID)
	if err != nil {
		logf(root, laneID, "stop: mark stopped: %v", err)
		return end()
	}
	logf(root, laneID, "stop: lane marked %s (retries=%d)", status, l.Retries)
	return end()
}
