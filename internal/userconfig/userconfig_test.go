package userconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/userconfig"
)

func TestDirAndEnvFile_WithXDG(t *testing.T) {
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)

	dir, err := userconfig.Dir()
	if err != nil {
		t.Fatalf("Dir() error: %v", err)
	}
	wantDir := filepath.Join(tempXDG, "lucind")
	if dir != wantDir {
		t.Errorf("Dir() = %q, want %q", dir, wantDir)
	}

	envFile, err := userconfig.EnvFile()
	if err != nil {
		t.Fatalf("EnvFile() error: %v", err)
	}
	wantEnv := filepath.Join(wantDir, "env")
	if envFile != wantEnv {
		t.Errorf("EnvFile() = %q, want %q", envFile, wantEnv)
	}
}

func TestDirAndEnvFile_DefaultHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir, err := userconfig.Dir()
	if err != nil {
		t.Fatalf("Dir() error: %v", err)
	}
	wantDir := filepath.Join(fakeHome, ".config", "lucind")
	if dir != wantDir {
		t.Errorf("Dir() = %q, want %q", dir, wantDir)
	}

	envFile, err := userconfig.EnvFile()
	if err != nil {
		t.Fatalf("EnvFile() error: %v", err)
	}
	wantEnv := filepath.Join(wantDir, "env")
	if envFile != wantEnv {
		t.Errorf("EnvFile() = %q, want %q", envFile, wantEnv)
	}
}

func TestReadKey_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	val, err := userconfig.ReadKey("TYPESAFE_API_KEY")
	if err != nil {
		t.Fatalf("ReadKey on missing file should return nil error, got: %v", err)
	}
	if val != "" {
		t.Errorf("ReadKey = %q, want empty string", val)
	}
}

func TestReadKey_ParsingTolerances(t *testing.T) {
	tests := []struct {
		name    string
		content string
		keyName string
		wantVal string
		wantErr bool
	}{
		{
			name:    "simple key value",
			content: "TYPESAFE_API_KEY=my-secret-key-1\n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "my-secret-key-1",
		},
		{
			name:    "blank lines and comments",
			content: "\n\n# This is a comment\n   # Indented comment\nTYPESAFE_API_KEY=my-secret-key-2\n\n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "my-secret-key-2",
		},
		{
			name:    "leading export",
			content: "export TYPESAFE_API_KEY=my-secret-key-3\n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "my-secret-key-3",
		},
		{
			name:    "double quotes around value",
			content: "TYPESAFE_API_KEY=\"my-secret-key-4\"\n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "my-secret-key-4",
		},
		{
			name:    "single quotes around value",
			content: "TYPESAFE_API_KEY='my-secret-key-5'\n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "my-secret-key-5",
		},
		{
			name:    "whitespace trimmed around key and value",
			content: "   export   TYPESAFE_API_KEY   =   \"my-secret-key-6\"   \n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "my-secret-key-6",
		},
		{
			name:    "unrelated keys ignored and key not found",
			content: "OTHER_KEY=something\nANOTHER_KEY=\"val\"\n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "",
		},
		{
			name:    "multiple keys with target key in middle",
			content: "FOO=bar\nTYPESAFE_API_KEY=found-key\nBAZ=qux\n",
			keyName: "TYPESAFE_API_KEY",
			wantVal: "found-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempXDG := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", tempXDG)

			dir := filepath.Join(tempXDG, "lucind")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatalf("MkdirAll failed: %v", err)
			}
			envPath := filepath.Join(dir, "env")
			if err := os.WriteFile(envPath, []byte(tt.content), 0600); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}

			gotVal, err := userconfig.ReadKey(tt.keyName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadKey() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotVal != tt.wantVal {
				t.Errorf("ReadKey() = %q, want %q", gotVal, tt.wantVal)
			}
		})
	}
}

func TestWriteKey_CreateNewFileAndPermissions(t *testing.T) {
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)

	const keyName = "TYPESAFE_API_KEY"
	const keyVal = "new-secret-key-123"

	err := userconfig.WriteKey(keyName, keyVal)
	if err != nil {
		t.Fatalf("WriteKey() error: %v", err)
	}

	dir := filepath.Join(tempXDG, "lucind")
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat dir failed: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0700 {
		t.Errorf("dir mode = %#o, want 0700", perm)
	}

	envFile := filepath.Join(dir, "env")
	fileInfo, err := os.Stat(envFile)
	if err != nil {
		t.Fatalf("Stat envFile failed: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0600 {
		t.Errorf("file mode = %#o, want 0600", perm)
	}

	got, err := userconfig.ReadKey(keyName)
	if err != nil {
		t.Fatalf("ReadKey() error: %v", err)
	}
	if got != keyVal {
		t.Errorf("ReadKey() = %q, want %q", got, keyVal)
	}
}

func TestWriteKey_PreservesEveryOtherLineByteForByte(t *testing.T) {
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)

	dir := filepath.Join(tempXDG, "lucind")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	envPath := filepath.Join(dir, "env")

	initialContent := `# Header comment with spaces and special chars: !@#$%^&*()
export EXISTING_FOO="bar"
   # Indented comment
UNTOUCHED=exact_bytes_here

TYPESAFE_API_KEY=old-key-to-replace
TRAILING_VAR=keep_me
`
	if err := os.WriteFile(envPath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	newKeyVal := "super-updated-key"
	if err := userconfig.WriteKey("TYPESAFE_API_KEY", newKeyVal); err != nil {
		t.Fatalf("WriteKey failed: %v", err)
	}

	updatedBytes, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	updatedStr := string(updatedBytes)

	wantExpected := `# Header comment with spaces and special chars: !@#$%^&*()
export EXISTING_FOO="bar"
   # Indented comment
UNTOUCHED=exact_bytes_here

TYPESAFE_API_KEY=super-updated-key
TRAILING_VAR=keep_me
`
	if updatedStr != wantExpected {
		t.Errorf("updated content mismatch:\ngot:\n%s\nwant:\n%s", updatedStr, wantExpected)
	}
}

func TestWriteKey_AppendsWhenNotFound(t *testing.T) {
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)

	dir := filepath.Join(tempXDG, "lucind")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	envPath := filepath.Join(dir, "env")

	initialContent := "FOO=bar\nBAZ=qux\n"
	if err := os.WriteFile(envPath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := userconfig.WriteKey("TYPESAFE_API_KEY", "appended-key"); err != nil {
		t.Fatalf("WriteKey failed: %v", err)
	}

	updatedBytes, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	want := "FOO=bar\nBAZ=qux\nTYPESAFE_API_KEY=appended-key\n"
	if string(updatedBytes) != want {
		t.Errorf("got:\n%s\nwant:\n%s", string(updatedBytes), want)
	}
}

func TestWriteKey_KeyNeverLeaksInErrors(t *testing.T) {
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)

	// Make the lucind config path a read-only file instead of directory so WriteKey fails
	conflictPath := filepath.Join(tempXDG, "lucind")
	if err := os.WriteFile(conflictPath, []byte("blocking-file"), 0400); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	const secretKey = "super-sensitive-secret-token"
	err := userconfig.WriteKey("TYPESAFE_API_KEY", secretKey)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if strings.Contains(err.Error(), secretKey) {
		t.Fatalf("SECRET KEY LEAKED IN ERROR: %s", err.Error())
	}
}
