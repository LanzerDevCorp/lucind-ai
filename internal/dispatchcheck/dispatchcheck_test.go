package dispatchcheck_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatchcheck"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

func TestComputeSignals(t *testing.T) {
	alwaysExists := func(path string) (bool, error) { return true, nil }
	neverExists := func(path string) (bool, error) { return false, nil }
	errorExists := func(path string) (bool, error) { return false, errors.New("git failure") }

	tests := []struct {
		name        string
		packet      packet.Packet
		exists      dispatchcheck.PathExists
		wantCount   int
		wantNew     bool
		wantTier    risk.Tier
		wantReasons []string
	}{
		{
			name: "read-only packet zeroes count and sets new file false but keeps tier",
			packet: packet.Packet{
				ReadOnly:     true,
				AllowedPaths: []string{"docs/a.md", "docs/b.md"},
			},
			exists:      neverExists,
			wantCount:   0,
			wantNew:     false,
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "read-only packet with high risk path keeps high tier and reason",
			packet: packet.Packet{
				ReadOnly:     true,
				AllowedPaths: []string{"internal/auth/login.go"},
			},
			exists:      alwaysExists,
			wantCount:   0,
			wantNew:     false,
			wantTier:    risk.TierHigh,
			wantReasons: []string{"risk_tier:high"},
		},
		{
			name: "write packet with multiple existing files",
			packet: packet.Packet{
				AllowedPaths: []string{"pkg/a.go", "pkg/b.go"},
			},
			exists:      alwaysExists,
			wantCount:   2,
			wantNew:     false,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"allowed_paths:2"},
		},
		{
			name: "write packet with missing file according to exists",
			packet: packet.Packet{
				AllowedPaths: []string{"pkg/new.go"},
			},
			exists:      neverExists,
			wantCount:   1,
			wantNew:     true,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"new_file:pkg/new.go"},
		},
		{
			name: "write packet with directory prefix ends with slash",
			packet: packet.Packet{
				AllowedPaths: []string{"internal/sub/"},
			},
			exists:      alwaysExists,
			wantCount:   1,
			wantNew:     true,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"new_file:internal/sub/"},
		},
		{
			name: "write packet with glob star",
			packet: packet.Packet{
				AllowedPaths: []string{"docs/*.md"},
			},
			exists:      alwaysExists,
			wantCount:   1,
			wantNew:     true,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"new_file:docs/*.md"},
		},
		{
			name: "write packet with glob question mark",
			packet: packet.Packet{
				AllowedPaths: []string{"docs/test?.md"},
			},
			exists:      alwaysExists,
			wantCount:   1,
			wantNew:     true,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"new_file:docs/test?.md"},
		},
		{
			name: "write packet with glob square bracket",
			packet: packet.Packet{
				AllowedPaths: []string{"docs/test[0-9].md"},
			},
			exists:      alwaysExists,
			wantCount:   1,
			wantNew:     true,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"new_file:docs/test[0-9].md"},
		},
		{
			name: "nil exists treats path as new file",
			packet: packet.Packet{
				AllowedPaths: []string{"pkg/existing.go"},
			},
			exists:      nil,
			wantCount:   1,
			wantNew:     true,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"new_file:pkg/existing.go"},
		},
		{
			name: "exists error treats path as new file",
			packet: packet.Packet{
				AllowedPaths: []string{"pkg/existing.go"},
			},
			exists:      errorExists,
			wantCount:   1,
			wantNew:     true,
			wantTier:    risk.TierMedium,
			wantReasons: []string{"new_file:pkg/existing.go"},
		},
		{
			name: "multiple hard signals all list in reasons",
			packet: packet.Packet{
				AllowedPaths: []string{"internal/auth/login.go", "pkg/new.go"},
			},
			exists: func(p string) (bool, error) {
				if p == "internal/auth/login.go" {
					return true, nil
				}
				return false, nil
			},
			wantCount:   2,
			wantNew:     true,
			wantTier:    risk.TierHigh,
			wantReasons: []string{"allowed_paths:2", "new_file:pkg/new.go", "risk_tier:high"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dispatchcheck.ComputeSignals(tt.packet, tt.exists)
			if got.AllowedPathCount != tt.wantCount {
				t.Errorf("AllowedPathCount = %d, want %d", got.AllowedPathCount, tt.wantCount)
			}
			if got.NewFile != tt.wantNew {
				t.Errorf("NewFile = %t, want %t", got.NewFile, tt.wantNew)
			}
			if got.Tier != tt.wantTier {
				t.Errorf("Tier = %v, want %v", got.Tier, tt.wantTier)
			}
			if !reflect.DeepEqual(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %#v, want %#v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	alwaysExists := func(path string) (bool, error) { return true, nil }
	neverExists := func(path string) (bool, error) { return false, nil }

	tests := []struct {
		name        string
		packet      packet.Packet
		exists      dispatchcheck.PathExists
		wantAction  dispatchcheck.Action
		wantRoute   string
		wantReasons []string
	}{
		// 1. No route declared
		{
			name: "no route declared accepts untouched",
			packet: packet.Packet{
				Route:        "",
				AllowedPaths: []string{"pkg/a.go", "pkg/b.go"},
			},
			exists:      neverExists,
			wantAction:  dispatchcheck.ActionAccept,
			wantRoute:   "",
			wantReasons: []string{"no route declared"},
		},

		// 2. Route inline
		{
			name: "inline with 2 paths upgrades to worker",
			packet: packet.Packet{
				Route:        "inline",
				AllowedPaths: []string{"pkg/a.go", "pkg/b.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionUpgrade,
			wantRoute:   "worker",
			wantReasons: []string{"allowed_paths:2"},
		},
		{
			name: "inline with new file upgrades to worker",
			packet: packet.Packet{
				Route:        "inline",
				AllowedPaths: []string{"pkg/new.go"},
			},
			exists:      neverExists,
			wantAction:  dispatchcheck.ActionUpgrade,
			wantRoute:   "worker",
			wantReasons: []string{"new_file:pkg/new.go"},
		},
		{
			name: "inline with directory prefix upgrades to worker",
			packet: packet.Packet{
				Route:        "inline",
				AllowedPaths: []string{"pkg/sub/"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionUpgrade,
			wantRoute:   "worker",
			wantReasons: []string{"new_file:pkg/sub/"},
		},
		{
			name: "inline with high risk path upgrades to worker",
			packet: packet.Packet{
				Route:        "inline",
				AllowedPaths: []string{"internal/auth/x.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionUpgrade,
			wantRoute:   "worker",
			wantReasons: []string{"risk_tier:high"},
		},
		{
			name: "inline with nil exists upgrades to worker",
			packet: packet.Packet{
				Route:        "inline",
				AllowedPaths: []string{"main.go"},
			},
			exists:      nil,
			wantAction:  dispatchcheck.ActionUpgrade,
			wantRoute:   "worker",
			wantReasons: []string{"new_file:main.go"},
		},
		{
			name: "inline with 1 existing plain file accepts",
			packet: packet.Packet{
				Route:        "inline",
				AllowedPaths: []string{"pkg/plain.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionAccept,
			wantRoute:   "inline",
			wantReasons: []string{"below dispatch threshold"},
		},

		// 3. Route worker
		{
			name: "worker with 1 file and evidence accepts",
			packet: packet.Packet{
				Route:         "worker",
				RouteEvidence: "needed dedicated worker lane",
				AllowedPaths:  []string{"pkg/plain.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionAccept,
			wantRoute:   "worker",
			wantReasons: []string{"below dispatch threshold, accepted"},
		},
		{
			name: "worker with 2 files and evidence accepts with signals",
			packet: packet.Packet{
				Route:         "worker",
				RouteEvidence: "touches 2 files",
				AllowedPaths:  []string{"pkg/a.go", "pkg/b.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionAccept,
			wantRoute:   "worker",
			wantReasons: []string{"allowed_paths:2"},
		},
		{
			name: "worker without evidence rejects",
			packet: packet.Packet{
				Route:         "worker",
				RouteEvidence: "",
				AllowedPaths:  []string{"pkg/plain.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionReject,
			wantRoute:   "worker",
			wantReasons: []string{"route_evidence is required when route is worker or fanout"},
		},
		{
			name: "worker with whitespace-only evidence rejects",
			packet: packet.Packet{
				Route:         "worker",
				RouteEvidence: "   \t\n  ",
				AllowedPaths:  []string{"pkg/plain.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionReject,
			wantRoute:   "worker",
			wantReasons: []string{"route_evidence is required when route is worker or fanout"},
		},

		// 4. Route fanout
		{
			name: "fanout read-only accepts",
			packet: packet.Packet{
				Route:         "fanout",
				RouteEvidence: "explorer fanout across 3 lenses",
				ReadOnly:      true,
				AllowedPaths:  []string{"docs/research.md"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionAccept,
			wantRoute:   "fanout",
			wantReasons: []string{"below dispatch threshold, accepted"},
		},
		{
			name: "fanout write with 2 paths accepts",
			packet: packet.Packet{
				Route:         "fanout",
				RouteEvidence: "independent tasks",
				ReadOnly:      false,
				AllowedPaths:  []string{"pkg/a.go", "pkg/b.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionAccept,
			wantRoute:   "fanout",
			wantReasons: []string{"allowed_paths:2"},
		},
		{
			name: "fanout write with 1 path rejects",
			packet: packet.Packet{
				Route:         "fanout",
				RouteEvidence: "single file write",
				ReadOnly:      false,
				AllowedPaths:  []string{"pkg/a.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionReject,
			wantRoute:   "fanout",
			wantReasons: []string{"fanout requires a read-only lane or at least 2 allowed paths"},
		},
		{
			name: "fanout without evidence rejects",
			packet: packet.Packet{
				Route:         "fanout",
				RouteEvidence: "",
				ReadOnly:      true,
				AllowedPaths:  []string{"docs/research.md"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionReject,
			wantRoute:   "fanout",
			wantReasons: []string{"route_evidence is required when route is worker or fanout"},
		},

		// 5. Unknown route
		{
			name: "unknown route rejects",
			packet: packet.Packet{
				Route:        "telepathic",
				AllowedPaths: []string{"pkg/plain.go"},
			},
			exists:      alwaysExists,
			wantAction:  dispatchcheck.ActionReject,
			wantRoute:   "telepathic",
			wantReasons: []string{"unknown route: telepathic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dispatchcheck.Check(tt.packet, tt.exists)
			if got.Action != tt.wantAction {
				t.Errorf("Check() Action = %v, want %v", got.Action, tt.wantAction)
			}
			if got.Route != tt.wantRoute {
				t.Errorf("Check() Route = %v, want %v", got.Route, tt.wantRoute)
			}
			if !reflect.DeepEqual(got.Reasons, tt.wantReasons) {
				t.Errorf("Check() Reasons = %#v, want %#v", got.Reasons, tt.wantReasons)
			}
		})
	}
}
