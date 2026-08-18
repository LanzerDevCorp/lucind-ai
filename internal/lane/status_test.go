package lane

import "testing"

func TestStatusTerminalAndValid(t *testing.T) {
	tests := []struct {
		name         string
		status       Status
		wantTerminal bool
		wantValid    bool
	}{
		{"pending is non-terminal", Pending, false, true},
		{"running is non-terminal", Running, false, true},
		{"done is terminal", Done, true, true},
		{"blocked is terminal", Blocked, true, true},
		{"deviated is terminal", Deviated, true, true},
		{"failed is terminal", Failed, true, true},
		{"invalid status is neither valid nor terminal", Status("bogus"), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Terminal(); got != tt.wantTerminal {
				t.Errorf("Status(%q).Terminal() = %v, want %v", tt.status, got, tt.wantTerminal)
			}
			if got := tt.status.Valid(); got != tt.wantValid {
				t.Errorf("Status(%q).Valid() = %v, want %v", tt.status, got, tt.wantValid)
			}
		})
	}
}
