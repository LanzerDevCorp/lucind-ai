package ledgerpath

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		primaryRoot string
		want        string
	}{
		{
			name:        "absolute primary root",
			primaryRoot: "/home/user/repo",
			want:        filepath.Join("/home/user/repo", ".lucind", "lucind.db"),
		},
		{
			name:        "relative primary root",
			primaryRoot: "repo",
			want:        filepath.Join("repo", ".lucind", "lucind.db"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve(tt.primaryRoot)
			if got != tt.want {
				t.Fatalf("Resolve(%q) = %q, want %q", tt.primaryRoot, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	const primaryRoot = "/home/user/repo"

	tests := []struct {
		name      string
		candidate string
		wantErr   error
	}{
		{
			name:      "candidate directly under primary repo .lucind is accepted",
			candidate: filepath.Join(primaryRoot, ".lucind", "lucind.db"),
			wantErr:   nil,
		},
		{
			name:      "candidate nested under primary repo .lucind is accepted",
			candidate: filepath.Join(primaryRoot, ".lucind", "sub", "lucind.db"),
			wantErr:   nil,
		},
		{
			name:      "worktree-shaped candidate outside the primary repo is rejected",
			candidate: filepath.Join(primaryRoot+"-worktrees", "lane1", ".lucind", "lucind.db"),
			wantErr:   ErrLedgerOutsidePrimaryRepo,
		},
		{
			name:      "sibling directory sharing a name prefix is rejected",
			candidate: filepath.Join(primaryRoot+"-other", ".lucind", "lucind.db"),
			wantErr:   ErrLedgerOutsidePrimaryRepo,
		},
		{
			name:      "primary repo root itself without .lucind segment is rejected",
			candidate: filepath.Join(primaryRoot, "lucind.db"),
			wantErr:   ErrLedgerOutsidePrimaryRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.candidate, primaryRoot)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Validate(%q, %q) = %v, want nil", tt.candidate, primaryRoot, err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate(%q, %q) = %v, want %v", tt.candidate, primaryRoot, err, tt.wantErr)
			}
		})
	}
}
