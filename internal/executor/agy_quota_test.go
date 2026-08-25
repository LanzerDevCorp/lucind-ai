package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// realUsageResponse is a captured `agy --print "/usage" --output-format
// json` response, verified against the real Antigravity CLI (agy 1.1.20).
const realUsageResponse = `{"conversation_id":"","status":"SUCCESS","response":"","duration_seconds":0,"num_turns":0,"usage":{"input_tokens":0,"output_tokens":0,"thinking_tokens":0,"cache_read_tokens":0,"total_tokens":0},"command":{"name":"usage","data":{"groups":[{"name":"Gemini Models","buckets":[{"id":"gemini-weekly","window":"weekly","remaining_fraction":0.7205095291137695},{"id":"gemini-5h","window":"5h","remaining_fraction":0.620489776134491}]},{"name":"Claude and GPT models","buckets":[{"id":"3p-weekly","window":"weekly","remaining_fraction":0.9897966384887695},{"id":"3p-5h","window":"5h","remaining_fraction":0.9830784201622009}]}]}}}`

func writeQuotaStub(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
	return path
}

func TestAgyQuotaEnsureSkipsRotationWhenAboveThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	agyStub := writeQuotaStub(t, "agy-stub.sh", "#!/bin/sh\ncat <<'EOF'\n"+realUsageResponse+"\nEOF\n")
	poolCallLog := filepath.Join(t.TempDir(), "pool-calls.log")
	poolStub := writeQuotaStub(t, "agy-pool-stub.sh", "#!/bin/sh\necho \"$@\" >> "+poolCallLog+"\nexit 0\n")

	q := executor.AgyQuota{AgyBinary: agyStub, AgyPoolBinary: poolStub}
	// realUsageResponse's gemini-5h fraction is ~0.62, well above a 0.10 minimum.
	if err := q.Ensure(context.Background(), 0.10); err != nil {
		t.Fatalf("Ensure() error = %v, want nil", err)
	}

	if _, err := os.Stat(poolCallLog); err == nil {
		got, _ := os.ReadFile(poolCallLog)
		t.Errorf("agy-pool was invoked (%q) but should not be when the active account clears the threshold", got)
	}
}

func TestAgyQuotaEnsureRotatesWhenBelowThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	lowUsage := `{"command":{"data":{"groups":[{"name":"Gemini Models","buckets":[{"id":"gemini-5h","window":"5h","remaining_fraction":0.05}]}]}}}`
	agyStub := writeQuotaStub(t, "agy-stub.sh", "#!/bin/sh\ncat <<'EOF'\n"+lowUsage+"\nEOF\n")

	poolCallLog := filepath.Join(t.TempDir(), "pool-calls.log")
	poolStub := writeQuotaStub(t, "agy-pool-stub.sh", `#!/bin/sh
echo "$@" >> `+poolCallLog+`
case "$1" in
  best) echo "backup@example.com" ;;
  use) exit 0 ;;
esac
`)

	q := executor.AgyQuota{AgyBinary: agyStub, AgyPoolBinary: poolStub}
	if err := q.Ensure(context.Background(), 0.10); err != nil {
		t.Fatalf("Ensure() error = %v, want nil", err)
	}

	calls, err := os.ReadFile(poolCallLog)
	if err != nil {
		t.Fatalf("read pool call log: %v", err)
	}
	got := string(calls)
	if !strings.Contains(got, "best 0.1") {
		t.Errorf("agy-pool calls = %q, want a %q call", got, "best 0.1")
	}
	if !strings.Contains(got, "use backup@example.com") {
		t.Errorf("agy-pool calls = %q, want %q", got, "use backup@example.com")
	}
}

func TestAgyQuotaEnsureErrorsWhenNoPooledAccountClears(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	lowUsage := `{"command":{"data":{"groups":[{"name":"Gemini Models","buckets":[{"id":"gemini-5h","window":"5h","remaining_fraction":0.02}]}]}}}`
	agyStub := writeQuotaStub(t, "agy-stub.sh", "#!/bin/sh\ncat <<'EOF'\n"+lowUsage+"\nEOF\n")
	poolStub := writeQuotaStub(t, "agy-pool-stub.sh", "#!/bin/sh\necho 'agy-pool: ninguna cuenta supera el minimo' 1>&2\nexit 1\n")

	q := executor.AgyQuota{AgyBinary: agyStub, AgyPoolBinary: poolStub}
	err := q.Ensure(context.Background(), 0.10)
	if err == nil {
		t.Fatal("Ensure() error = nil, want an error blocking the wave")
	}
}

func TestAgyQuotaEnsureErrorsWhenAgyFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	agyStub := writeQuotaStub(t, "agy-stub.sh", "#!/bin/sh\necho 'boom' 1>&2\nexit 1\n")
	poolStub := writeQuotaStub(t, "agy-pool-stub.sh", "#!/bin/sh\nexit 0\n")

	q := executor.AgyQuota{AgyBinary: agyStub, AgyPoolBinary: poolStub}
	err := q.Ensure(context.Background(), 0.10)
	if err == nil {
		t.Fatal("Ensure() error = nil, want an error when the active account's usage can't be read")
	}
}

func TestParseGeminiFiveHourFraction(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    float64
		wantErr bool
	}{
		{
			name: "real response",
			raw:  realUsageResponse,
			want: 0.620489776134491,
		},
		{
			name:    "missing bucket",
			raw:     `{"command":{"data":{"groups":[{"name":"Claude and GPT models","buckets":[{"id":"3p-5h","remaining_fraction":0.9}]}]}}}`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			raw:     `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executor.ParseGeminiFiveHourFraction([]byte(tt.raw))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseGeminiFiveHourFraction() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseGeminiFiveHourFraction() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("ParseGeminiFiveHourFraction() = %v, want %v", got, tt.want)
			}
		})
	}
}
