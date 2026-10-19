package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
)

func setupTestModesConfig(t *testing.T) (string, string) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	lucindDir := filepath.Join(configDir, "lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatalf("mkdir lucind config: %v", err)
	}
	content := `{
  "active": "standard",
  "modes": {
    "fast": {
      "explorer": {"executor": "native", "model": "haiku-5.5"},
      "writer": {"executor": "lane", "model": "sonnet-5.5"},
      "verifier": {"executor": "native", "model": "haiku-5.5"}
    },
    "standard": {
      "explorer": {"executor": "native", "model": "sonnet-5.5"},
      "writer": {"executor": "lane", "model": "sonnet-5.5"},
      "verifier": {"executor": "lane", "model": "sonnet-5.5"}
    }
  }
}`
	cfgPath := filepath.Join(lucindDir, "modes.json")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write modes.json: %v", err)
	}
	return configDir, cfgPath
}

func runCLI(ctx context.Context, args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := run(ctx, args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestModeList(t *testing.T) {
	setupTestModesConfig(t)
	stdout, stderr, code := runCLI(context.Background(), "mode", "list")
	if code != 0 {
		t.Fatalf("mode list code = %d, want 0, stderr: %s", code, stderr)
	}
	want := "  fast\n* standard\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestModeShow(t *testing.T) {
	setupTestModesConfig(t)
	stdout, stderr, code := runCLI(context.Background(), "mode", "show")
	if code != 0 {
		t.Fatalf("mode show code = %d, want 0, stderr: %s", code, stderr)
	}
	want := "mode: standard (global)\nexplorer: native sonnet-5.5\nwriter: lane sonnet-5.5\nverifier: lane sonnet-5.5\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestModeUse_Global(t *testing.T) {
	_, cfgPath := setupTestModesConfig(t)
	stdout, stderr, code := runCLI(context.Background(), "mode", "use", "fast")
	if code != 0 {
		t.Fatalf("mode use code = %d, want 0, stderr: %s", code, stderr)
	}
	if want := "active mode: fast (global)\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), `"active": "fast"`) {
		t.Errorf("config active not updated to fast: %s", string(data))
	}
}

func TestModeUse_ProjectAndClear(t *testing.T) {
	setupTestModesConfig(t)
	ctx := context.Background()
	projDir := t.TempDir()

	// 1. mode use with --project
	stdout, stderr, code := runCLI(ctx, "mode", "use", "fast", "--project", "--cwd", projDir)
	if code != 0 {
		t.Fatalf("mode use --project code = %d, want 0, stderr: %s", code, stderr)
	}
	wantPath := filepath.Join(projDir, ".lucind", "mode")
	if want := "active mode: fast (project: " + wantPath + ")\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	data, err := os.ReadFile(wantPath)
	if err != nil || string(data) != "fast\n" {
		t.Errorf("override content = %q, err = %v, want \"fast\\n\"", string(data), err)
	}

	// 2. Precedence: mode show reflects project override
	stdout, stderr, code = runCLI(ctx, "mode", "show", "--cwd", projDir)
	if code != 0 {
		t.Fatalf("mode show --cwd code = %d, want 0, stderr: %s", code, stderr)
	}
	wantShow := "mode: fast (project override)\nexplorer: native haiku-5.5\nwriter: lane sonnet-5.5\nverifier: native haiku-5.5\n"
	if stdout != wantShow {
		t.Errorf("stdout = %q, want %q", stdout, wantShow)
	}

	// 3. Precedence: mode list shows * for project override
	stdout, stderr, code = runCLI(ctx, "mode", "list", "--cwd", projDir)
	if code != 0 {
		t.Fatalf("mode list --cwd code = %d, want 0, stderr: %s", code, stderr)
	}
	if wantList := "* fast\n  standard\n"; stdout != wantList {
		t.Errorf("stdout = %q, want %q", stdout, wantList)
	}

	// 4. mode clear removes override
	stdout, stderr, code = runCLI(ctx, "mode", "clear", "--cwd", projDir)
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("mode clear expected silent success, code=%d, stdout=%q, stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed", wantPath)
	}

	// 5. mode clear succeeds silently when file does not exist
	stdout, stderr, code = runCLI(ctx, "mode", "clear", "--cwd", projDir)
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("mode clear second call expected silent success, code=%d", code)
	}

	// 6. After clear, global mode is effective again
	stdout, _, code = runCLI(ctx, "mode", "show", "--cwd", projDir)
	if code != 0 || !strings.HasPrefix(stdout, "mode: standard (global)\n") {
		t.Errorf("stdout = %q, want global standard mode", stdout)
	}
}

func TestRoute(t *testing.T) {
	setupTestModesConfig(t)
	ctx := context.Background()

	// Plain output
	stdout, stderr, code := runCLI(ctx, "route", "writer")
	if code != 0 {
		t.Fatalf("route writer code = %d, want 0, stderr: %s", code, stderr)
	}
	if stdout != "lane sonnet-5.5\n" {
		t.Errorf("stdout = %q, want %q", stdout, "lane sonnet-5.5\n")
	}

	// JSON output
	stdout, stderr, code = runCLI(ctx, "route", "explorer", "--json")
	if code != 0 {
		t.Fatalf("route explorer --json code = %d, want 0, stderr: %s", code, stderr)
	}
	var res struct {
		Mode     string `json:"mode"`
		Role     string `json:"role"`
		Executor string `json:"executor"`
		Model    string `json:"model"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json unmarshal %q error: %v", stdout, err)
	}
	if res.Mode != "standard" || res.Role != "explorer" || res.Executor != "native" || res.Model != "sonnet-5.5" {
		t.Errorf("unexpected json result: %+v", res)
	}
}

func TestRoute_UnknownRole(t *testing.T) {
	setupTestModesConfig(t)
	_, stderr, code := runCLI(context.Background(), "route", "invalidrole")
	if code != 1 {
		t.Fatalf("route invalidrole code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "usage: lucind-ai route") {
		t.Errorf("stderr = %q, want route usage", stderr)
	}
}

func TestMode_UnknownMode(t *testing.T) {
	setupTestModesConfig(t)
	_, stderr, code := runCLI(context.Background(), "mode", "use", "nonexistent")
	if code != 1 {
		t.Fatalf("mode use nonexistent code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "lucind-ai: mode use:") || !strings.Contains(stderr, "nonexistent") {
		t.Errorf("stderr = %q, want error message about nonexistent mode", stderr)
	}
}

func TestMode_MissingConfig(t *testing.T) {
	emptyHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", emptyHome)
	_, stderr, code := runCLI(context.Background(), "mode", "list")
	if code != 1 {
		t.Fatalf("mode list with missing config code = %d, want 1", code)
	}
	expectedPath := filepath.Join(emptyHome, "lucind", "modes.json")
	if !strings.Contains(stderr, expectedPath) {
		t.Errorf("stderr = %q, want it to contain path %q", stderr, expectedPath)
	}
	if !strings.Contains(stderr, "modes.json") || !strings.Contains(stderr, "active") {
		t.Errorf("stderr = %q, want minimal example config", stderr)
	}
}

func createDispatchFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	return dir, promptPath
}

func TestDispatch_Role(t *testing.T) {
	ctx := context.Background()
	repoDir, promptPath := createDispatchFixture(t)

	// 1. Role resolves to a lane model and reaches Options.Model
	t.Run("resolves_lane_model", func(t *testing.T) {
		setupTestModesConfig(t)
		origDispatchRun := dispatchRun
		defer func() { dispatchRun = origDispatchRun }()
		called := false
		var capturedOpts dispatch.Options
		dispatchRun = func(_ context.Context, opts dispatch.Options, _ dispatch.HerdrRunner) (dispatch.Output, int, error) {
			called = true
			capturedOpts = opts
			return dispatch.Output{}, 0, nil
		}

		_, stderr, code := runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--role", "writer")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %s", code, stderr)
		}
		if !called {
			t.Fatal("expected dispatchRun to be called")
		}
		if capturedOpts.Model != "sonnet-5.5" {
			t.Errorf("captured Model = %q, want %q", capturedOpts.Model, "sonnet-5.5")
		}
	})

	// 2. --model wins over --role
	t.Run("model_wins_over_role", func(t *testing.T) {
		setupTestModesConfig(t)
		origDispatchRun := dispatchRun
		defer func() { dispatchRun = origDispatchRun }()
		called := false
		var capturedOpts dispatch.Options
		dispatchRun = func(_ context.Context, opts dispatch.Options, _ dispatch.HerdrRunner) (dispatch.Output, int, error) {
			called = true
			capturedOpts = opts
			return dispatch.Output{}, 0, nil
		}

		_, stderr, code := runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--role", "writer", "--model", "custom-model")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %s", code, stderr)
		}
		if !called {
			t.Fatal("expected dispatchRun to be called")
		}
		if capturedOpts.Model != "custom-model" {
			t.Errorf("captured Model = %q, want %q", capturedOpts.Model, "custom-model")
		}

		// Also check that a native role with --model is allowed and executor is not checked
		called = false
		_, stderr, code = runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--role", "explorer", "--model", "custom-model-2")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %s", code, stderr)
		}
		if !called {
			t.Fatal("expected dispatchRun to be called for native role when --model is set")
		}
		if capturedOpts.Model != "custom-model-2" {
			t.Errorf("captured Model = %q, want %q", capturedOpts.Model, "custom-model-2")
		}
	})

	// 3. Project override changes the model
	t.Run("project_override", func(t *testing.T) {
		configDir, _ := setupTestModesConfig(t)
		customJSON := `{
  "active": "standard",
  "modes": {
    "standard": {
      "writer": {"executor": "lane", "model": "sonnet-5.5"}
    },
    "custom": {
      "writer": {"executor": "lane", "model": "opus-4.5"}
    }
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "lucind", "modes.json"), []byte(customJSON), 0o600); err != nil {
			t.Fatalf("write custom modes.json: %v", err)
		}

		overrideDir, overridePrompt := createDispatchFixture(t)
		lucindDir := filepath.Join(overrideDir, ".lucind")
		if err := os.MkdirAll(lucindDir, 0o755); err != nil {
			t.Fatalf("mkdir .lucind: %v", err)
		}
		if err := os.WriteFile(filepath.Join(lucindDir, "mode"), []byte("custom\n"), 0o644); err != nil {
			t.Fatalf("write override: %v", err)
		}

		origDispatchRun := dispatchRun
		defer func() { dispatchRun = origDispatchRun }()
		called := false
		var capturedOpts dispatch.Options
		dispatchRun = func(_ context.Context, opts dispatch.Options, _ dispatch.HerdrRunner) (dispatch.Output, int, error) {
			called = true
			capturedOpts = opts
			return dispatch.Output{}, 0, nil
		}

		_, stderr, code := runCLI(ctx, "dispatch", "--cwd", overrideDir, "--allow", "src/**", "--prompt", overridePrompt, "--role", "writer")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %s", code, stderr)
		}
		if !called {
			t.Fatal("expected dispatchRun to be called")
		}
		if capturedOpts.Model != "opus-4.5" {
			t.Errorf("captured Model = %q, want %q", capturedOpts.Model, "opus-4.5")
		}
	})

	// 4. Native executor rejected without calling dispatchRun
	t.Run("native_executor_rejected", func(t *testing.T) {
		setupTestModesConfig(t)
		origDispatchRun := dispatchRun
		defer func() { dispatchRun = origDispatchRun }()
		called := false
		dispatchRun = func(_ context.Context, _ dispatch.Options, _ dispatch.HerdrRunner) (dispatch.Output, int, error) {
			called = true
			return dispatch.Output{}, 0, nil
		}

		_, stderr, code := runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--role", "explorer")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if called {
			t.Fatal("dispatchRun must not be called when executor is native")
		}
		wantMsg := `lucind-ai: dispatch: role "explorer" is native in mode "standard"; launch it natively with model sonnet-5.5 instead of a lane`
		if !strings.Contains(stderr, wantMsg) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, wantMsg)
		}
	})

	// 5. Unknown role
	t.Run("unknown_role", func(t *testing.T) {
		setupTestModesConfig(t)
		origDispatchRun := dispatchRun
		defer func() { dispatchRun = origDispatchRun }()
		called := false
		dispatchRun = func(_ context.Context, _ dispatch.Options, _ dispatch.HerdrRunner) (dispatch.Output, int, error) {
			called = true
			return dispatch.Output{}, 0, nil
		}

		_, stderr, code := runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--role", "nonexistent")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if called {
			t.Fatal("dispatchRun must not be called for unknown role")
		}
		if !strings.Contains(stderr, "unknown role") || !strings.Contains(stderr, "nonexistent") {
			t.Errorf("stderr = %q, want unknown role error", stderr)
		}

		// Also verify unknown role with --model still fails and does not call dispatchRun
		called = false
		_, stderr, code = runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--role", "nonexistent", "--model", "custom-model")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if called {
			t.Fatal("dispatchRun must not be called for unknown role even with --model")
		}
		if !strings.Contains(stderr, "unknown role") || !strings.Contains(stderr, "nonexistent") {
			t.Errorf("stderr = %q, want unknown role error", stderr)
		}
	})

	// 6. Missing config
	t.Run("missing_config", func(t *testing.T) {
		emptyHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", emptyHome)
		origDispatchRun := dispatchRun
		defer func() { dispatchRun = origDispatchRun }()
		called := false
		dispatchRun = func(_ context.Context, _ dispatch.Options, _ dispatch.HerdrRunner) (dispatch.Output, int, error) {
			called = true
			return dispatch.Output{}, 0, nil
		}

		_, stderr, code := runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--role", "writer")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if called {
			t.Fatal("dispatchRun must not be called when config is missing")
		}
		expectedPath := filepath.Join(emptyHome, "lucind", "modes.json")
		if !strings.Contains(stderr, expectedPath) {
			t.Errorf("stderr = %q, want path %q", stderr, expectedPath)
		}
		if !strings.Contains(stderr, "modes.json") || !strings.Contains(stderr, "active") {
			t.Errorf("stderr = %q, want minimal configuration example", stderr)
		}
	})

	// 7. Dispatch without --role still passes --model and leaves Options.Model empty when omitted
	t.Run("dispatch_without_role", func(t *testing.T) {
		origDispatchRun := dispatchRun
		defer func() { dispatchRun = origDispatchRun }()
		var capturedOpts dispatch.Options
		dispatchRun = func(_ context.Context, opts dispatch.Options, _ dispatch.HerdrRunner) (dispatch.Output, int, error) {
			capturedOpts = opts
			return dispatch.Output{}, 0, nil
		}

		// With --model
		_, stderr, code := runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath, "--model", "explicit-model")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %s", code, stderr)
		}
		if capturedOpts.Model != "explicit-model" {
			t.Errorf("captured Model = %q, want %q", capturedOpts.Model, "explicit-model")
		}

		// Without --model
		_, stderr, code = runCLI(ctx, "dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptPath)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %s", code, stderr)
		}
		if capturedOpts.Model != "" {
			t.Errorf("captured Model = %q, want empty string", capturedOpts.Model)
		}
	})
}
