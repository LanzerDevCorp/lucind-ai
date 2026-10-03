package router

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestJevLive calls the real Jev API once. It runs only when LUCIND_JEV_LIVE=1
// and LUCIND_JEV_API_KEY is set, so the normal suite never touches the network.
func TestJevLive(t *testing.T) {
	key := os.Getenv("LUCIND_JEV_API_KEY")
	if os.Getenv("LUCIND_JEV_LIVE") != "1" || key == "" {
		t.Skip("set LUCIND_JEV_LIVE=1 and LUCIND_JEV_API_KEY to run")
	}
	j, err := NewJev(DefaultJevURL, key)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cases := []Signals{
		{AllowedPathCount: 1},
		{AllowedPathCount: 4, NewFile: true, RiskTierLevel: 2},
	}
	for _, s := range cases {
		d, err := j.Route(ctx, s)
		if err != nil {
			t.Fatalf("route %+v: %v", s, err)
		}
		t.Logf("signals=%+v -> route=%s confidence=%.2f", s, d.Route, d.Confidence)
	}
}
