package modes_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/modes"
)

func TestPath(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempHome)

	path, err := modes.Path()
	if err != nil {
		t.Fatalf("Path() returned unexpected error: %v", err)
	}

	expected := filepath.Join(tempHome, "lucind", "modes.json")
	if path != expected {
		t.Errorf("Path() = %q, want %q", path, expected)
	}
}

func TestRoles(t *testing.T) {
	expected := []string{modes.RoleExplorer, modes.RoleWriter, modes.RoleVerifier}
	got := modes.Roles()
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Roles() = %v, want %v", got, expected)
	}

	// Verify mutating returned slice does not mutate subsequent calls
	got[0] = "mutated"
	gotAgain := modes.Roles()
	if gotAgain[0] != modes.RoleExplorer {
		t.Errorf("Roles() allowed internal state mutation: %v", gotAgain)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent", "modes.json")
	_, err := modes.Load(missingPath)
	if err == nil {
		t.Fatal("Load() with missing file expected error, got nil")
	}
	if !errors.Is(err, modes.ErrNoConfig) {
		t.Errorf("Load() error does not wrap ErrNoConfig: %v", err)
	}
}

func TestLoad_Validation(t *testing.T) {
	tests := []struct {
		name          string
		jsonContent   string
		wantErr       bool
		errSubstrings []string
	}{
		{
			name:        "invalid json syntax",
			jsonContent: `{"active": "test", bad json`,
			wantErr:     true,
			errSubstrings: []string{
				"parse config",
			},
		},
		{
			name: "unknown role",
			jsonContent: `{
				"active": "custom",
				"modes": {
					"custom": {
						"reviewer": {
							"executor": "native",
							"model": "haiku-5.5"
						}
					}
				}
			}`,
			wantErr: true,
			errSubstrings: []string{
				"custom",
				"reviewer",
				"unknown role",
			},
		},
		{
			name: "unknown executor",
			jsonContent: `{
				"active": "custom",
				"modes": {
					"custom": {
						"explorer": {
							"executor": "docker",
							"model": "haiku-5.5"
						}
					}
				}
			}`,
			wantErr: true,
			errSubstrings: []string{
				"custom",
				"explorer",
				"unknown executor",
				"docker",
			},
		},
		{
			name: "reserved executor cursor",
			jsonContent: `{
				"active": "custom",
				"modes": {
					"custom": {
						"writer": {
							"executor": "cursor",
							"model": "sonnet-5.5"
						}
					}
				}
			}`,
			wantErr: true,
			errSubstrings: []string{
				"custom",
				"writer",
				"cursor",
				"not implemented yet",
			},
		},
		{
			name: "empty model",
			jsonContent: `{
				"active": "custom",
				"modes": {
					"custom": {
						"verifier": {
							"executor": "lane",
							"model": ""
						}
					}
				}
			}`,
			wantErr: true,
			errSubstrings: []string{
				"custom",
				"verifier",
				"empty model",
			},
		},
		{
			name: "whitespace only model",
			jsonContent: `{
				"active": "custom",
				"modes": {
					"custom": {
						"verifier": {
							"executor": "lane",
							"model": "   "
						}
					}
				}
			}`,
			wantErr: true,
			errSubstrings: []string{
				"custom",
				"verifier",
				"empty model",
			},
		},
		{
			name: "active mode does not exist",
			jsonContent: `{
				"active": "nonexistent-mode",
				"modes": {
					"claude-full": {
						"explorer": {
							"executor": "native",
							"model": "haiku-5.5"
						}
					}
				}
			}`,
			wantErr: true,
			errSubstrings: []string{
				"nonexistent-mode",
			},
		},
		{
			name: "valid with empty active and partial roles",
			jsonContent: `{
				"active": "",
				"modes": {
					"partial": {
						"writer": {
							"executor": "lane",
							"model": "gemini-3.8-flash"
						}
					}
				}
			}`,
			wantErr: false,
		},
		{
			name: "valid full configuration",
			jsonContent: `{
				"active": "claude-full",
				"modes": {
					"claude-full": {
						"explorer": {"executor": "native", "model": "haiku-5.5"},
						"writer": {"executor": "native", "model": "sonnet-5.5"},
						"verifier": {"executor": "native", "model": "haiku-5.5"}
					}
				}
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, "modes.json")
			if err := os.WriteFile(filePath, []byte(tt.jsonContent), 0o600); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			cfg, err := modes.Load(filePath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() expected error, got nil")
				}
				for _, substr := range tt.errSubstrings {
					if !strings.Contains(err.Error(), substr) {
						t.Errorf("error %q does not contain required substring %q", err.Error(), substr)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Load() unexpected error: %v", err)
				}
				if cfg == nil {
					t.Fatal("Load() returned nil config without error")
				}
			}
		})
	}
}

func TestActiveName(t *testing.T) {
	const validJSON = `{
		"active": "claude-full",
		"modes": {
			"claude-full": {
				"explorer": {"executor": "native", "model": "haiku-5.5"},
				"writer": {"executor": "native", "model": "sonnet-5.5"},
				"verifier": {"executor": "native", "model": "haiku-5.5"}
			},
			"lane-mode": {
				"writer": {"executor": "lane", "model": "gemini-3.8"}
			}
		}
	}`

	t.Run("no project dir returns config active", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "modes.json")
		if err := os.WriteFile(filePath, []byte(validJSON), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := modes.Load(filePath)
		if err != nil {
			t.Fatal(err)
		}

		name, fromProject, err := cfg.ActiveName("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "claude-full" || fromProject {
			t.Errorf("ActiveName() = (%q, %v), want (%q, false)", name, fromProject, "claude-full")
		}
	})

	t.Run("missing override file returns config active", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "modes.json")
		if err := os.WriteFile(filePath, []byte(validJSON), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := modes.Load(filePath)
		if err != nil {
			t.Fatal(err)
		}

		projDir := t.TempDir()
		name, fromProject, err := cfg.ActiveName(projDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "claude-full" || fromProject {
			t.Errorf("ActiveName() = (%q, %v), want (%q, false)", name, fromProject, "claude-full")
		}
	})

	t.Run("project override wins over config active", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "modes.json")
		if err := os.WriteFile(filePath, []byte(validJSON), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := modes.Load(filePath)
		if err != nil {
			t.Fatal(err)
		}

		projDir := t.TempDir()
		lucindDir := filepath.Join(projDir, ".lucind")
		if err := os.MkdirAll(lucindDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lucindDir, "mode"), []byte("  lane-mode\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		name, fromProject, err := cfg.ActiveName(projDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "lane-mode" || !fromProject {
			t.Errorf("ActiveName() = (%q, %v), want (%q, true)", name, fromProject, "lane-mode")
		}
	})

	t.Run("unknown project override returns error", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "modes.json")
		if err := os.WriteFile(filePath, []byte(validJSON), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := modes.Load(filePath)
		if err != nil {
			t.Fatal(err)
		}

		projDir := t.TempDir()
		lucindDir := filepath.Join(projDir, ".lucind")
		if err := os.MkdirAll(lucindDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lucindDir, "mode"), []byte("nonexistent-mode\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, _, err = cfg.ActiveName(projDir)
		if err == nil {
			t.Fatal("ActiveName() with unknown override expected error, got nil")
		}
		if !strings.Contains(err.Error(), "nonexistent-mode") {
			t.Errorf("error %q should name unknown mode", err.Error())
		}
	})

	t.Run("neither active nor override set returns error", func(t *testing.T) {
		const noActiveJSON = `{
			"active": "",
			"modes": {
				"claude-full": {
					"writer": {"executor": "native", "model": "sonnet-5.5"}
				}
			}
		}`

		dir := t.TempDir()
		filePath := filepath.Join(dir, "modes.json")
		if err := os.WriteFile(filePath, []byte(noActiveJSON), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := modes.Load(filePath)
		if err != nil {
			t.Fatal(err)
		}

		projDir := t.TempDir()
		_, _, err = cfg.ActiveName(projDir)
		if err == nil {
			t.Fatal("ActiveName() with no active mode expected error, got nil")
		}
	})
}

func TestResolve(t *testing.T) {
	const jsonContent = `{
		"active": "claude-full",
		"modes": {
			"claude-full": {
				"explorer": {"executor": "native", "model": "haiku-5.5"},
				"writer": {"executor": "native", "model": "sonnet-5.5"}
			},
			"lane-mode": {
				"writer": {"executor": "lane", "model": "gemini-3.8-flash"}
			}
		}
	}`

	dir := t.TempDir()
	filePath := filepath.Join(dir, "modes.json")
	if err := os.WriteFile(filePath, []byte(jsonContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := modes.Load(filePath)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("resolve success", func(t *testing.T) {
		route, err := cfg.Resolve("", modes.RoleExplorer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := modes.Route{
			Mode:     "claude-full",
			Role:     modes.RoleExplorer,
			Executor: modes.ExecutorNative,
			Model:    "haiku-5.5",
		}
		if route != want {
			t.Errorf("Resolve() = %+v, want %+v", route, want)
		}
	})

	t.Run("resolve success with project override", func(t *testing.T) {
		projDir := t.TempDir()
		lucindDir := filepath.Join(projDir, ".lucind")
		if err := os.MkdirAll(lucindDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lucindDir, "mode"), []byte("lane-mode\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		route, err := cfg.Resolve(projDir, modes.RoleWriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := modes.Route{
			Mode:     "lane-mode",
			Role:     modes.RoleWriter,
			Executor: modes.ExecutorLane,
			Model:    "gemini-3.8-flash",
		}
		if route != want {
			t.Errorf("Resolve() = %+v, want %+v", route, want)
		}
	})

	t.Run("missing role in active mode errors with mode and role", func(t *testing.T) {
		// verifier is not defined in claude-full
		_, err := cfg.Resolve("", modes.RoleVerifier)
		if err == nil {
			t.Fatal("expected error for undefined role, got nil")
		}
		if !strings.Contains(err.Error(), "claude-full") || !strings.Contains(err.Error(), modes.RoleVerifier) {
			t.Errorf("error %q should name mode and role", err.Error())
		}
	})

	t.Run("unknown role errors with mode and role", func(t *testing.T) {
		_, err := cfg.Resolve("", "unknown-role")
		if err == nil {
			t.Fatal("expected error for unknown role, got nil")
		}
		if !strings.Contains(err.Error(), "unknown-role") || !strings.Contains(err.Error(), "claude-full") {
			t.Errorf("error %q should name role and mode", err.Error())
		}
	})
}

func TestModes(t *testing.T) {
	const jsonContent = `{
		"active": "zebra",
		"modes": {
			"zebra": {"writer": {"executor": "native", "model": "m"}},
			"alpha": {"writer": {"executor": "native", "model": "m"}},
			"middle": {"writer": {"executor": "native", "model": "m"}}
		}
	}`

	dir := t.TempDir()
	filePath := filepath.Join(dir, "modes.json")
	if err := os.WriteFile(filePath, []byte(jsonContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := modes.Load(filePath)
	if err != nil {
		t.Fatal(err)
	}

	got := cfg.Modes()
	want := []string{"alpha", "middle", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Modes() = %v, want %v", got, want)
	}
}

func TestSetActive(t *testing.T) {
	const initialJSON = `{
  "active": "claude-full",
  "modes": {
    "claude-full": {
      "writer": {
        "executor": "native",
        "model": "sonnet-5.5"
      }
    },
    "lane-mode": {
      "writer": {
        "executor": "lane",
        "model": "gemini-3.8-flash"
      }
    }
  }
}`

	t.Run("round trip preserves other modes and sets active with 0o600 and 2-space indent", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "modes.json")
		if err := os.WriteFile(filePath, []byte(initialJSON), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := modes.SetActive(filePath, "lane-mode"); err != nil {
			t.Fatalf("SetActive() unexpected error: %v", err)
		}

		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatal(err)
		}
		contentStr := string(content)
		if !strings.Contains(contentStr, "  \"active\": \"lane-mode\"") {
			t.Errorf("expected 2-space indent active field in file, got:\n%s", contentStr)
		}

		loaded, err := modes.Load(filePath)
		if err != nil {
			t.Fatalf("Load() after SetActive error: %v", err)
		}
		if loaded.Active != "lane-mode" {
			t.Errorf("loaded.Active = %q, want %q", loaded.Active, "lane-mode")
		}

		// Verify both modes and roles are preserved
		modesList := loaded.Modes()
		if !reflect.DeepEqual(modesList, []string{"claude-full", "lane-mode"}) {
			t.Errorf("loaded modes = %v, want %v", modesList, []string{"claude-full", "lane-mode"})
		}

		route, err := loaded.Resolve("", modes.RoleWriter)
		if err != nil {
			t.Fatalf("Resolve writer error: %v", err)
		}
		if route.Mode != "lane-mode" || route.Executor != modes.ExecutorLane || route.Model != "gemini-3.8-flash" {
			t.Errorf("unexpected resolved route: %+v", route)
		}
	})

	t.Run("failed SetActive leaves file untouched", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "modes.json")
		if err := os.WriteFile(filePath, []byte(initialJSON), 0o600); err != nil {
			t.Fatal(err)
		}

		err := modes.SetActive(filePath, "unknown-mode")
		if err == nil {
			t.Fatal("SetActive() with unknown mode expected error, got nil")
		}

		afterContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(afterContent) != initialJSON {
			t.Errorf("file content was modified despite error:\ngot:\n%s\nwant:\n%s", string(afterContent), initialJSON)
		}
	})

	t.Run("failed SetActive on missing file returns ErrNoConfig", func(t *testing.T) {
		dir := t.TempDir()
		missingPath := filepath.Join(dir, "missing.json")

		err := modes.SetActive(missingPath, "any-mode")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, modes.ErrNoConfig) {
			t.Errorf("expected error wrapping ErrNoConfig, got: %v", err)
		}
	})
}

func TestMode(t *testing.T) {
	const jsonContent = `{
		"active": "claude-full",
		"modes": {
			"claude-full": {
				"explorer": {"executor": "native", "model": "haiku-5.5"}
			}
		}
	}`

	dir := t.TempDir()
	filePath := filepath.Join(dir, "modes.json")
	if err := os.WriteFile(filePath, []byte(jsonContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := modes.Load(filePath)
	if err != nil {
		t.Fatal(err)
	}

	roles, ok := cfg.Mode("claude-full")
	if !ok {
		t.Fatal("Mode(claude-full) expected ok = true")
	}
	if roles[modes.RoleExplorer].Model != "haiku-5.5" {
		t.Errorf("unexpected explorer model: %q", roles[modes.RoleExplorer].Model)
	}

	// Verify mutating returned map does not mutate internal state
	roles[modes.RoleExplorer] = modes.RoleConfig{Executor: "mutated", Model: "mutated"}
	rolesAgain, _ := cfg.Mode("claude-full")
	if rolesAgain[modes.RoleExplorer].Model != "haiku-5.5" {
		t.Errorf("Mode() allowed internal map mutation")
	}

	_, ok = cfg.Mode("nonexistent")
	if ok {
		t.Errorf("Mode(nonexistent) expected ok = false, got true")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	const raw = `{"active":"claude-full","modes":{"claude-full":{"explorer":{"executor":"native","model":"haiku-5.5"}}}}`

	var cfg modes.Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if cfg.Active != "claude-full" {
		t.Errorf("cfg.Active = %q, want %q", cfg.Active, "claude-full")
	}

	marshaled, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if string(marshaled) != raw {
		t.Errorf("marshaled = %s, want %s", string(marshaled), raw)
	}
}
