package dag

import (
	"testing"
)

func TestReaches(t *testing.T) {
	dependents := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"D": {},
	}

	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{
			name: "direct edge",
			from: "A",
			to:   "B",
			want: true,
		},
		{
			name: "transitive chain",
			from: "A",
			to:   "C",
			want: true,
		},
		{
			name: "reverse direction",
			from: "C",
			to:   "A",
			want: false,
		},
		{
			name: "disjoint nodes",
			from: "A",
			to:   "D",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reaches(dependents, tt.from, tt.to)
			if got != tt.want {
				t.Errorf("reaches(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
