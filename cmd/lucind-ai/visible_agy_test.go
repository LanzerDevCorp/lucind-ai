package main

import (
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

func TestAgyLanesAreVisibleInsideHerdr(t *testing.T) {
	cases := []struct {
		name             string
		herdrEnv, optOut string
		wantAgyVisible   bool
	}{
		{"inside herdr", "1", "", true},
		{"outside herdr", "", "", false},
		{"opt-out inside herdr", "1", "off", false},
		{"herdr env not 1", "0", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HERDR_ENV", tc.herdrEnv)
			t.Setenv("LUCIND_HERDR_VISIBLE", tc.optOut)

			got := supportedExecutors["agy"]()
			h, isHerdr := got.(executor.HerdrAgy)
			if isHerdr != tc.wantAgyVisible {
				t.Fatalf("agy executor = %T, want herdr-backed = %v", got, tc.wantAgyVisible)
			}
			if isHerdr && !h.Interactive {
				t.Error("agy lanes routed through herdr must be interactive")
			}
		})
	}
}

func TestHerdrAgyExecutorIsInteractiveUnlessOptedOut(t *testing.T) {
	t.Setenv("LUCIND_HERDR_VISIBLE", "")
	if h := supportedExecutors["herdr-agy"]().(executor.HerdrAgy); !h.Interactive {
		t.Error("herdr-agy must be interactive by default")
	}
	t.Setenv("LUCIND_HERDR_VISIBLE", "off")
	if h := supportedExecutors["herdr-agy"]().(executor.HerdrAgy); h.Interactive {
		t.Error("LUCIND_HERDR_VISIBLE=off must restore the headless herdr-agy")
	}
}
