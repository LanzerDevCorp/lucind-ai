package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// Pinned judges for the offline rubric. Production triage runtime is unset.
const (
	JudgeClaudeExecutor   = "claude"
	JudgeClaudeModel      = "claude-opus-5"
	JudgeOpencodeExecutor = "opencode"
	JudgeOpencodeModel    = "openai/gpt-5.6-sol"
)

// RubricOptions runs the same fixture through the two pinned judges.
type RubricOptions struct {
	Claude       executor.Executor
	Opencode     executor.Executor
	WorktreePath string
}

// RubricResult is the A/B grade. ProposedSHA and human timing are ignored.
type RubricResult struct {
	Passed   bool
	Reason   string
	Claude   JudgeResult
	Opencode JudgeResult
}

// JudgeResult is one pinned judge's parsed payload and grade.
type JudgeResult struct {
	Executor string
	Model    string
	Payload  conflicttriage.TriagePayload
	Passed   bool
	Reason   string
}

// EvaluateRubric grades both pinned judges offline. It does not select a
// production triage executor or model.
func EvaluateRubric(ctx context.Context, opts RubricOptions) (RubricResult, error) {
	if opts.Claude == nil || opts.Opencode == nil {
		return RubricResult{}, errors.New("fixture: both Claude and Opencode judges are required")
	}

	claude, err := runJudge(ctx, opts.Claude, JudgeClaudeExecutor, JudgeClaudeModel, opts.WorktreePath)
	if err != nil {
		return RubricResult{}, fmt.Errorf("fixture: claude judge: %w", err)
	}
	opencode, err := runJudge(ctx, opts.Opencode, JudgeOpencodeExecutor, JudgeOpencodeModel, opts.WorktreePath)
	if err != nil {
		return RubricResult{}, fmt.Errorf("fixture: opencode judge: %w", err)
	}

	out := RubricResult{Claude: claude, Opencode: opencode}
	if !claude.Passed {
		out.Reason = "claude: " + claude.Reason
		return out, nil
	}
	if !opencode.Passed {
		out.Reason = "opencode: " + opencode.Reason
		return out, nil
	}
	out.Passed = true
	return out, nil
}

func runJudge(ctx context.Context, exec executor.Executor, name, model, worktree string) (JudgeResult, error) {
	outcome, err := exec.Run(ctx, executor.Request{
		Prompt:       rubricPrompt(),
		WorktreePath: worktree,
		Model:        model,
	})
	if err != nil {
		return JudgeResult{}, err
	}
	payload, err := parsePayload(outcome.Stdout)
	if err != nil {
		return JudgeResult{}, err
	}
	passed, reason := gradePayload(payload)
	return JudgeResult{
		Executor: name,
		Model:    model,
		Payload:  payload,
		Passed:   passed,
		Reason:   reason,
	}, nil
}

// maxPayloadSearchDepth bounds how many levels deep parsePayload recurses
// into nested JSON string fields looking for the real answer. Two hops
// covers every real shape observed so far (claude: one wrapper, one nested
// "result" string); the cap exists only to keep a pathological input from
// causing unbounded recursion, not because a deeper shape is expected.
const maxPayloadSearchDepth = 4

// parsePayload extracts the TriagePayload from a judge's raw stdout.
//
// A stub-scripted judge prints exactly one JSON object, and slicing from the
// first "{" to the last "}" is enough. Real CLIs are not shaped that simply.
// Verified against the real opencode CLI (v1.18.21, "run --format json") and
// the real claude CLI (2.1.241, "--print --output-format json"):
//
//   - opencode emits several back-to-back top-level JSON objects on stdout
//     (other event/status records around the model's actual answer), not a
//     single one. Naive first-to-last-brace slicing then spans multiple
//     concatenated documents, which is not valid JSON on its own
//     ("invalid character '{' after top-level value").
//   - claude emits exactly one top-level object, but it is a result envelope
//     (is_error, usage, session_id, ...) whose "result" field is a *string*
//     containing the model's actual reply -- itself a markdown-fenced
//     ```json code block wrapping the real TriagePayload, followed by prose.
//     The real answer is nested inside a string value, not present as a
//     top-level field at all.
//
// This scans stdout for every balanced top-level {...} object (respecting
// quoted strings, so a brace inside a string literal never miscounts depth),
// then recursively re-scans every string value inside each decoded object
// for further embedded JSON objects (covering the claude shape, and any
// similarly-wrapped shape), and keeps the last candidate that actually looks
// like an answer (a non-empty HunkDecisions). Unrelated event/wrapper
// objects unmarshal into a TriagePayload without error too -- json.Unmarshal
// never fails on missing fields -- so HunkDecisions is the signal that
// distinguishes the real answer from surrounding noise, not decode success
// alone.
func parsePayload(stdout string) (conflicttriage.TriagePayload, error) {
	objects := collectJSONCandidates(stdout, maxPayloadSearchDepth)
	if len(objects) == 0 {
		return conflicttriage.TriagePayload{}, fmt.Errorf("parse TriagePayload: no JSON object found in stdout")
	}

	var best conflicttriage.TriagePayload
	found := false
	for _, obj := range objects {
		var p conflicttriage.TriagePayload
		if err := json.Unmarshal([]byte(obj), &p); err != nil {
			continue
		}
		if len(p.HunkDecisions) > 0 {
			best = p
			found = true
		}
	}
	if found {
		return best, nil
	}

	// Nothing decoded as a plausible answer. Report the parse error against
	// the last candidate object, so the failure still names something
	// concrete rather than "no JSON object found" when objects did exist.
	var p conflicttriage.TriagePayload
	if err := json.Unmarshal([]byte(objects[len(objects)-1]), &p); err != nil {
		return conflicttriage.TriagePayload{}, fmt.Errorf("parse TriagePayload: %w", err)
	}
	return conflicttriage.TriagePayload{}, fmt.Errorf("parse TriagePayload: no object with hunk_decisions found among %d JSON object(s) in stdout", len(objects))
}

// collectJSONCandidates returns every balanced top-level JSON object found
// in s, plus (up to depth levels) every JSON object embedded inside any of
// their string field values -- covering an answer nested inside a CLI
// wrapper's text field (with or without a markdown code fence around it).
// Candidates are ordered so a deeper (more specific) match found while
// unwrapping a given object comes after that object itself.
func collectJSONCandidates(s string, depth int) []string {
	var candidates []string
	for _, obj := range extractJSONObjects(s) {
		candidates = append(candidates, obj)
		if depth <= 0 {
			continue
		}
		for _, str := range stringValues(obj) {
			nested := extractJSONObjects(str)
			if len(nested) == 0 {
				continue
			}
			candidates = append(candidates, collectJSONCandidates(str, depth-1)...)
		}
	}
	return candidates
}

// stringValues decodes obj generically and returns every string leaf value
// found anywhere in it (object values, array elements, arbitrary nesting).
// A decode failure yields no values rather than an error: obj already came
// from extractJSONObjects, which only emits balanced spans, but a malformed
// span (e.g. a "{" that turned out to belong to non-JSON surrounding text)
// is possible and simply contributes no further candidates.
func stringValues(obj string) []string {
	var v interface{}
	if err := json.Unmarshal([]byte(obj), &v); err != nil {
		return nil
	}
	var out []string
	var walk func(interface{})
	walk = func(node interface{}) {
		switch n := node.(type) {
		case string:
			out = append(out, n)
		case map[string]interface{}:
			for _, val := range n {
				walk(val)
			}
		case []interface{}:
			for _, val := range n {
				walk(val)
			}
		}
	}
	walk(v)
	return out
}

// extractJSONObjects returns every balanced top-level {...} substring of s,
// in order of appearance. Depth tracking ignores braces inside quoted JSON
// strings (honoring backslash escapes) so a literal "{" or "}" in a string
// value never miscounts as structural.
func extractJSONObjects(s string) []string {
	var objects []string
	depth := 0
	start := -1
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					objects = append(objects, s[start:i+1])
					start = -1
				}
			}
		}
	}
	return objects
}

func gradePayload(p conflicttriage.TriagePayload) (bool, string) {
	if len(p.HunkDecisions) != 3 {
		return false, fmt.Sprintf("want 3 hunk decisions, got %d", len(p.HunkDecisions))
	}
	if uniformHunkScoring(p.HunkDecisions) {
		return false, "uniform hunk scoring"
	}
	var business *conflicttriage.HunkDecision
	mechanical := 0
	for i := range p.HunkDecisions {
		h := &p.HunkDecisions[i]
		if h.Kind == conflicttriage.HunkKindBusiness {
			business = h
			continue
		}
		mechanical++
	}
	if business == nil {
		return false, "business hunk not distinguished"
	}
	if business.Resolution != conflicttriage.ResolutionArbitrary {
		return false, "business hunk is not ARBITRARY"
	}
	if mechanical != 2 {
		return false, "business hunk not separated from two mechanical controls"
	}
	return true, ""
}

func uniformHunkScoring(hs []conflicttriage.HunkDecision) bool {
	sameKind := true
	sameResolution := true
	for i := 1; i < len(hs); i++ {
		if hs[i].Kind != hs[0].Kind {
			sameKind = false
		}
		if hs[i].Resolution != hs[0].Resolution {
			sameResolution = false
		}
	}
	return sameKind || sameResolution
}

func rubricPrompt() string {
	return "Grade this three-hunk fixture. Separate the business hunk from the two mechanical controls " +
		"(slice-literal union and rename-versus-edit) and declare ARBITRARY on the business hunk. " +
		"Do not grade proposed_sha. Do not time a human. Emit JSON matching TriagePayload.\n"
}
