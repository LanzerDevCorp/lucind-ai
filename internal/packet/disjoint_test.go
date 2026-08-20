package packet_test

import (
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

func TestPathInScope(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		allowed []string
		want    bool
	}{
		{
			name:    "prefix match without trailing slash in allowed",
			path:    "internal/ledger/foo.go",
			allowed: []string{"internal/ledger"},
			want:    true,
		},
		{
			name:    "prefix match with trailing slash in allowed",
			path:    "internal/ledger/foo.go",
			allowed: []string{"internal/ledger/"},
			want:    true,
		},
		{
			name:    "non-component prefix is rejected",
			path:    "internal/ledger/foo.go",
			allowed: []string{"internal/led"},
			want:    false,
		},
		{
			name:    "exact file match is true",
			path:    "cmd/lucind-ai/cli.go",
			allowed: []string{"cmd/lucind-ai/cli.go"},
			want:    true,
		},
		{
			name:    "exact directory match with trailing slashes normalized",
			path:    "internal/ledger/",
			allowed: []string{"internal/ledger"},
			want:    true,
		},
		{
			name:    "exact directory match reverse slash normalization",
			path:    "internal/ledger",
			allowed: []string{"internal/ledger/"},
			want:    true,
		},
		{
			name:    "nested subdirectory matches prefix",
			path:    "internal/ledger/sub/pkg/file.go",
			allowed: []string{"internal/ledger"},
			want:    true,
		},
		{
			name:    "multiple allowed paths with one matching",
			path:    "internal/ledger/foo.go",
			allowed: []string{"cmd/lucind-ai/cli.go", "internal/led", "internal/ledger"},
			want:    true,
		},
		{
			name:    "empty allowed list returns false",
			path:    "internal/ledger/foo.go",
			allowed: []string{},
			want:    false,
		},
		{
			name:    "nil allowed list returns false",
			path:    "internal/ledger/foo.go",
			allowed: nil,
			want:    false,
		},
		{
			name:    "unrelated path returns false",
			path:    "cmd/lucind-ai/cli.go",
			allowed: []string{"internal/ledger"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packet.PathInScope(tt.path, tt.allowed)
			if got != tt.want {
				t.Errorf("PathInScope(%q, %v) = %v, want %v", tt.path, tt.allowed, got, tt.want)
			}
		})
	}
}

func TestDisjointAllowedPaths(t *testing.T) {
	tests := []struct {
		name        string
		packets     []packet.Packet
		wantErr     bool
		wantIDsInErr []string
	}{
		{
			name: "prefix overlap returns error naming both packet IDs",
			packets: []packet.Packet{
				{ID: "pkt-dir", AllowedPaths: []string{"internal/foo/"}},
				{ID: "pkt-file", AllowedPaths: []string{"internal/foo/bar.go"}},
			},
			wantErr:      true,
			wantIDsInErr: []string{"pkt-dir", "pkt-file"},
		},
		{
			name: "reverse order prefix overlap returns error naming both packet IDs",
			packets: []packet.Packet{
				{ID: "pkt-file", AllowedPaths: []string{"internal/foo/bar.go"}},
				{ID: "pkt-dir", AllowedPaths: []string{"internal/foo/"}},
			},
			wantErr:      true,
			wantIDsInErr: []string{"pkt-file", "pkt-dir"},
		},
		{
			name: "exact same path in both packets returns error",
			packets: []packet.Packet{
				{ID: "pkt-a", AllowedPaths: []string{"internal/ledger/foo.go"}},
				{ID: "pkt-b", AllowedPaths: []string{"internal/ledger/foo.go"}},
			},
			wantErr:      true,
			wantIDsInErr: []string{"pkt-a", "pkt-b"},
		},
		{
			name: "sibling directories are disjoint and return nil",
			packets: []packet.Packet{
				{ID: "pkt-foo", AllowedPaths: []string{"internal/foo/"}},
				{ID: "pkt-bar", AllowedPaths: []string{"internal/bar/"}},
			},
			wantErr: false,
		},
		{
			name: "non-component prefix is disjoint and returns nil",
			packets: []packet.Packet{
				{ID: "pkt-led", AllowedPaths: []string{"internal/led"}},
				{ID: "pkt-ledger", AllowedPaths: []string{"internal/ledger/foo.go"}},
			},
			wantErr: false,
		},
		{
			name: "packet with empty AllowedPaths is skipped as undeclared",
			packets: []packet.Packet{
				{ID: "pkt-undeclared", AllowedPaths: []string{}},
				{ID: "pkt-declared", AllowedPaths: []string{"internal/foo/bar.go"}},
			},
			wantErr: false,
		},
		{
			name: "packet with nil AllowedPaths is skipped as undeclared",
			packets: []packet.Packet{
				{ID: "pkt-undeclared", AllowedPaths: nil},
				{ID: "pkt-declared", AllowedPaths: []string{"internal/foo/bar.go"}},
			},
			wantErr: false,
		},
		{
			name: "single packet returns nil",
			packets: []packet.Packet{
				{ID: "pkt-single", AllowedPaths: []string{"internal/foo/"}},
			},
			wantErr: false,
		},
		{
			name: "all undeclared returns nil",
			packets: []packet.Packet{
				{ID: "pkt-1", AllowedPaths: nil},
				{ID: "pkt-2", AllowedPaths: []string{}},
			},
			wantErr: false,
		},
		{
			name:    "empty packet slice returns nil",
			packets: []packet.Packet{},
			wantErr: false,
		},
		{
			name:    "nil packet slice returns nil",
			packets: nil,
			wantErr: false,
		},
		{
			name: "three packets where first and third overlap",
			packets: []packet.Packet{
				{ID: "pkt-1", AllowedPaths: []string{"internal/foo/"}},
				{ID: "pkt-2", AllowedPaths: []string{"internal/bar/"}},
				{ID: "pkt-3", AllowedPaths: []string{"internal/foo/baz.go"}},
			},
			wantErr:      true,
			wantIDsInErr: []string{"pkt-1", "pkt-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := packet.DisjointAllowedPaths(tt.packets)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DisjointAllowedPaths() error = nil, want error")
				}
				for _, id := range tt.wantIDsInErr {
					if !strings.Contains(err.Error(), id) {
						t.Errorf("DisjointAllowedPaths() error = %q, want it to contain %q", err.Error(), id)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("DisjointAllowedPaths() error = %v, want nil", err)
				}
			}
		})
	}
}
