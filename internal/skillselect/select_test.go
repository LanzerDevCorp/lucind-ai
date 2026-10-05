package skillselect_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

func TestSelect_ZeroSkills(t *testing.T) {
	// A server that fails if called
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be contacted when skills slice is empty")
	}))
	defer srv.Close()

	client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	in := skillselect.Input{
		Brief: "Some task",
		Allow: []string{"internal/**"},
	}

	res, err := skillselect.Select(context.Background(), client, []skillselect.Skill{}, in, 0.7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Threshold != 0.7 {
		t.Errorf("res.Threshold = %v, want 0.7", res.Threshold)
	}
	if len(res.Decisions) != 0 {
		t.Errorf("len(res.Decisions) = %d, want 0", len(res.Decisions))
	}
}

func TestSelect_RequestShapeAndState(t *testing.T) {
	const secretKey = "typesafe-secret-key-xyz"
	var (
		gotMethod      string
		gotAuthHeader  string
		gotContentType string
		gotBodyBytes   []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuthHeader = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		var err error
		gotBodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"skill_000": { "type": "noul", "noul": 0.8 },
				"skill_001": { "type": "noul", "noul": 0.2 }
			},
			"usage": { "input_tokens": 100, "output_tokens": 20 }
		}`))
	}))
	defer srv.Close()

	client, err := skillselect.NewClient(secretKey, skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	skills := []skillselect.Skill{
		{
			Name:        "golang-testing",
			Description: "Go testing conventions",
			Scope:       "repo",
			Path:        "/path/to/golang-testing/SKILL.md",
		},
		{
			Name:        "branch-pr",
			Description: "PR creation",
			Scope:       "user",
			Path:        "/path/to/branch-pr/SKILL.md",
		},
	}

	in := skillselect.Input{
		Brief: "Implement skillselect package",
		Allow: []string{
			"internal/skillselect/**",
			"cmd/lucind-ai/*.go",
			"odd/tasks/*.md",
			"internal/skillselect/*.go",
			"Makefile",
		},
	}

	res, err := skillselect.Select(context.Background(), client, skills, in, 0.7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify HTTP headers
	if gotMethod != http.MethodPost {
		t.Errorf("gotMethod = %q, want POST", gotMethod)
	}
	if gotAuthHeader != "Bearer "+secretKey {
		t.Errorf("gotAuthHeader = %q, want 'Bearer %s'", gotAuthHeader, secretKey)
	}
	if gotContentType != "application/json" {
		t.Errorf("gotContentType = %q, want 'application/json'", gotContentType)
	}

	// Parse and verify request payload
	var reqMap map[string]any
	if err := json.Unmarshal(gotBodyBytes, &reqMap); err != nil {
		t.Fatalf("unmarshal request body failed: %v", err)
	}

	if reqMap["model"] != "jev-latest" {
		t.Errorf("model = %v, want 'jev-latest'", reqMap["model"])
	}

	// Verify state
	state, ok := reqMap["state"].(map[string]any)
	if !ok {
		t.Fatalf("state is not a JSON object: %T", reqMap["state"])
	}
	if state["task_brief"] != in.Brief {
		t.Errorf("state.task_brief = %v, want %q", state["task_brief"], in.Brief)
	}

	allowed, ok := state["allowed_edit_surfaces"].([]any)
	if !ok || len(allowed) != len(in.Allow) {
		t.Fatalf("state.allowed_edit_surfaces = %v, want %v", state["allowed_edit_surfaces"], in.Allow)
	}

	exts, ok := state["file_extensions"].([]any)
	if !ok {
		t.Fatalf("state.file_extensions = %v, want []any", state["file_extensions"])
	}
	// Expected sorted unique extensions: [".go", ".md"]
	if len(exts) != 2 || exts[0] != ".go" || exts[1] != ".md" {
		t.Errorf("state.file_extensions = %v, want ['.go', '.md']", exts)
	}

	// Verify questions
	questions, ok := reqMap["questions"].(map[string]any)
	if !ok {
		t.Fatalf("questions is not an object: %T", reqMap["questions"])
	}
	if len(questions) != 2 {
		t.Fatalf("len(questions) = %d, want 2", len(questions))
	}

	for i, qid := range []string{"skill_000", "skill_001"} {
		q, ok := questions[qid].(map[string]any)
		if !ok {
			t.Fatalf("questions[%s] is not an object: %v", qid, questions[qid])
		}
		if q["type"] != "noul" {
			t.Errorf("questions[%s].type = %v, want 'noul'", qid, q["type"])
		}

		instr, ok := q["instructions"].(map[string]any)
		if !ok {
			t.Fatalf("questions[%s].instructions is not an object", qid)
		}
		if !strings.Contains(instr["executor"].(string), "A coding subagent will implement `task_brief`") {
			t.Errorf("unexpected executor instruction: %v", instr["executor"])
		}
		if instr["question"] != "Must this subagent read the skill below before it starts changing files?" {
			t.Errorf("unexpected question instruction: %v", instr["question"])
		}
		sk, ok := instr["skill"].(map[string]any)
		if !ok {
			t.Fatalf("questions[%s].instructions.skill is not an object", qid)
		}
		if sk["name"] != skills[i].Name {
			t.Errorf("skill.name = %v, want %q", sk["name"], skills[i].Name)
		}
		if sk["description"] != skills[i].Description {
			t.Errorf("skill.description = %v, want %q", sk["description"], skills[i].Description)
		}

		crit, ok := q["criteria"].(map[string]any)
		if !ok {
			t.Fatalf("questions[%s].criteria is not an object", qid)
		}
		if !strings.Contains(crit["true"].(string), "The skill's guidance directly shapes code, tests or documentation") {
			t.Errorf("unexpected criteria.true: %v", crit["true"])
		}
		if !strings.Contains(crit["false"].(string), "The skill is unrelated to the task") {
			t.Errorf("unexpected criteria.false: %v", crit["false"])
		}
	}

	// Verify Result usage
	if res.Usage.InputTokens != 100 || res.Usage.OutputTokens != 20 {
		t.Errorf("res.Usage = %+v, want {100, 20}", res.Usage)
	}
}

func TestSelect_ProbabilityMappingAndSorting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"skill_000": { "type": "noul", "noul": 0.60 },
				"skill_001": { "type": "noul", "noul": 0.95 },
				"skill_002": { "type": "noul", "noul": 0.95 },
				"skill_003": { "type": "noul", "noul": 0.40 }
			},
			"usage": { "input_tokens": 50, "output_tokens": 10 }
		}`))
	}))
	defer srv.Close()

	client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	skills := []skillselect.Skill{
		{Name: "zeta", Path: "/path/zeta"},
		{Name: "beta", Path: "/path/beta"},
		{Name: "alpha", Path: "/path/alpha"},
		{Name: "gamma", Path: "/path/gamma"},
	}

	res, err := skillselect.Select(context.Background(), client, skills, skillselect.Input{Brief: "test"}, 0.70)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Decisions) != 4 {
		t.Fatalf("got %d decisions, want 4", len(res.Decisions))
	}

	// Order should be:
	// 1. alpha (prob 0.95, selected true) - tie broken by name ascending
	// 2. beta  (prob 0.95, selected true)
	// 3. zeta  (prob 0.60, selected false)
	// 4. gamma (prob 0.40, selected false)
	expected := []struct {
		name     string
		prob     float64
		selected bool
	}{
		{"alpha", 0.95, true},
		{"beta", 0.95, true},
		{"zeta", 0.60, false},
		{"gamma", 0.40, false},
	}

	for i, exp := range expected {
		d := res.Decisions[i]
		if d.Name != exp.name {
			t.Errorf("decision[%d].Name = %q, want %q", i, d.Name, exp.name)
		}
		if d.Probability != exp.prob {
			t.Errorf("decision[%d].Probability = %v, want %v", i, d.Probability, exp.prob)
		}
		if d.Selected != exp.selected {
			t.Errorf("decision[%d].Selected = %v, want %v", i, d.Selected, exp.selected)
		}
	}
}

func TestSelect_ThresholdFiltering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"skill_000": { "type": "noul", "noul": 0.70 },
				"skill_001": { "type": "noul", "noul": 0.699 }
			}
		}`))
	}))
	defer srv.Close()

	client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	skills := []skillselect.Skill{
		{Name: "exact-threshold", Path: "/path/exact"},
		{Name: "just-below", Path: "/path/below"},
	}

	res, err := skillselect.Select(context.Background(), client, skills, skillselect.Input{Brief: "test"}, 0.70)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, d := range res.Decisions {
		if d.Name == "exact-threshold" && !d.Selected {
			t.Errorf("expected 0.70 to be selected at threshold 0.70")
		}
		if d.Name == "just-below" && d.Selected {
			t.Errorf("expected 0.699 to NOT be selected at threshold 0.70")
		}
	}
}

func TestSelect_MissingAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Missing answer for skill_001
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"skill_000": { "type": "noul", "noul": 0.85 }
			}
		}`))
	}))
	defer srv.Close()

	client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	skills := []skillselect.Skill{
		{Name: "skill-0", Path: "/p0"},
		{Name: "skill-1", Path: "/p1"},
	}

	_, err = skillselect.Select(context.Background(), client, skills, skillselect.Input{Brief: "test"}, 0.7)
	if err == nil {
		t.Fatal("expected error for missing answer, got nil")
	}

	var missingErr *skillselect.MissingAnswerError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected *skillselect.MissingAnswerError, got %T: %v", err, err)
	}
	if missingErr.QuestionID != "skill_001" {
		t.Errorf("missingErr.QuestionID = %q, want 'skill_001'", missingErr.QuestionID)
	}
}

func TestSelect_InvalidAnswer(t *testing.T) {
	tests := []struct {
		name   string
		answer string
	}{
		{name: "confidence instead of noul", answer: `{ "type": "noul", "confidence": 0.9 }`},
		{name: "no probability field", answer: `{ "type": "noul" }`},
		{name: "not a noul answer", answer: `{ "type": "choice", "noul": 0.9 }`},
		{name: "above one", answer: `{ "type": "noul", "noul": 1.5 }`},
		{name: "below zero", answer: `{ "type": "noul", "noul": -0.1 }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"model": "jev-latest", "answers": {"skill_000": ` + tt.answer + `}}`))
			}))
			defer srv.Close()

			client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
			if err != nil {
				t.Fatalf("NewClient error: %v", err)
			}

			skills := []skillselect.Skill{{Name: "skill-0", Path: "/p0"}}
			_, err = skillselect.Select(context.Background(), client, skills, skillselect.Input{Brief: "test"}, 0.7)

			var invalidErr *skillselect.InvalidAnswerError
			if !errors.As(err, &invalidErr) {
				t.Fatalf("expected *skillselect.InvalidAnswerError, got %T: %v", err, err)
			}
			if invalidErr.QuestionID != "skill_000" {
				t.Errorf("invalidErr.QuestionID = %q, want 'skill_000'", invalidErr.QuestionID)
			}
		})
	}
}

func TestSelect_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json at all`))
	}))
	defer srv.Close()

	client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	skills := []skillselect.Skill{
		{Name: "skill-0", Path: "/p0"},
	}

	_, err = skillselect.Select(context.Background(), client, skills, skillselect.Input{Brief: "test"}, 0.7)
	if err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}

	if !errors.Is(err, skillselect.ErrMalformedJSON) && !errors.Is(err, skillselect.ErrMalformedResponse) {
		t.Errorf("expected ErrMalformedJSON or ErrMalformedResponse, got %v", err)
	}
}

func TestSection(t *testing.T) {
	tests := []struct {
		name      string
		decisions []skillselect.Decision
		want      string
	}{
		{
			name:      "empty decisions",
			decisions: []skillselect.Decision{},
			want:      "",
		},
		{
			name: "none selected",
			decisions: []skillselect.Decision{
				{Name: "skill-a", Path: "/path/a", Selected: false},
				{Name: "skill-b", Path: "/path/b", Selected: false},
			},
			want: "",
		},
		{
			name: "some selected in order",
			decisions: []skillselect.Decision{
				{Name: "skill-a", Path: "/path/a", Selected: true},
				{Name: "skill-b", Path: "/path/b", Selected: false},
				{Name: "skill-c", Path: "/path/c", Selected: true},
			},
			want: "## Skills to load before work\n/path/a\n/path/c\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skillselect.Section(tt.decisions)
			if got != tt.want {
				t.Errorf("Section() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelect_Live(t *testing.T) {
	if os.Getenv("TYPESAFE_LIVE_TEST") != "1" {
		t.Skip("skipping live test; set TYPESAFE_LIVE_TEST=1 to run")
	}
	key := skillselect.KeyFromEnv()
	if key == "" {
		t.Skip("skipping live test; TYPESAFE_API_KEY is not set")
	}

	client, err := skillselect.NewClient(key)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skills := []skillselect.Skill{
		{
			Name:        "golang-testing",
			Description: "Production-ready Golang tests — table-driven tests, testify suites.",
			Scope:       "repo",
			Path:        "/home/lanzerdev/git_root/lucind-ai/.agents/skills/golang-testing/SKILL.md",
		},
		{
			Name:        "branch-pr",
			Description: "Create Gentle AI pull requests with issue-first checks. Trigger: creating, opening, or preparing PRs for review.",
			Scope:       "user",
			Path:        "/home/lanzerdev/.agents/skills/branch-pr/SKILL.md",
		},
	}

	in := skillselect.Input{
		Brief: "Write table-driven tests for Go package",
		Allow: []string{"internal/skillselect/*.go"},
	}

	res, err := skillselect.Select(ctx, client, skills, in, skillselect.DefaultThreshold)
	if err != nil {
		t.Fatalf("Select live failed: %v", err)
	}

	t.Logf("Live select succeeded: model=%s, decisions=%+v, usage=%+v", res.Model, res.Decisions, res.Usage)
	if len(res.Decisions) != 2 {
		t.Errorf("expected 2 decisions, got %d", len(res.Decisions))
	}
}
