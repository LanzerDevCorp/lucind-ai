package lane

import "testing"

func TestStatusTerminalAndValid(t *testing.T) {
	tests := []struct {
		name         string
		status       Status
		wantTerminal bool
		wantValid    bool
	}{
		{"running is non-terminal and valid", StatusRunning, false, true},
		{"done is terminal and valid", StatusDone, true, true},
		{"failed is terminal and valid", StatusFailed, true, true},
		{"timeout is terminal and valid", StatusTimeout, true, true},
		{"accepted is terminal and valid", StatusAccepted, true, true},
		{"rejected is terminal and valid", StatusRejected, true, true},
		{"invalid status is neither valid nor terminal", Status("bogus"), false, false},
		{"empty status is neither valid nor terminal", Status(""), false, false},
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
