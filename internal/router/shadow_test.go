package router_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/router"
)

type fakeRouter struct {
	dec router.Decision
	err error
	pan any
}

func (f fakeRouter) Route(ctx context.Context, s router.Signals) (router.Decision, error) {
	if f.pan != nil {
		panic(f.pan)
	}
	return f.dec, f.err
}

func TestShadow_Route(t *testing.T) {
	sig := router.Signals{
		AllowedPathCount: 1,
		NewFile:          false,
		RiskTierLevel:    0,
		ReadOnly:         true,
	}

	primaryDecision := router.Decision{
		Route:      "inline",
		Source:     "deterministic",
		Confidence: 1.0,
		Reasons:    nil,
	}

	t.Run("agreement logs a router_agreement event", func(t *testing.T) {
		var events []router.Event
		s := router.Shadow{
			Primary: fakeRouter{dec: primaryDecision},
			Candidate: fakeRouter{dec: router.Decision{
				Route:      "inline",
				Source:     "jev",
				Confidence: 0.9,
			}},
			Log: func(ev router.Event) {
				events = append(events, ev)
			},
		}

		dec, err := s.Route(context.Background(), sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Route != "inline" || dec.Source != "deterministic" {
			t.Errorf("got decision %+v; want %+v", dec, primaryDecision)
		}
		if len(events) != 1 || events[0].Kind != "router_agreement" || events[0].PrimaryRoute != "inline" || events[0].CandidateRoute != "inline" || events[0].Confidence != 0.9 {
			t.Fatalf("expected one router_agreement event, got %+v", events)
		}
	})

	t.Run("disagreement logs router_disagreement event", func(t *testing.T) {
		var events []router.Event
		s := router.Shadow{
			Primary: fakeRouter{dec: primaryDecision},
			Candidate: fakeRouter{dec: router.Decision{
				Route:      "worker",
				Source:     "jev",
				Confidence: 0.75,
			}},
			Log: func(ev router.Event) {
				events = append(events, ev)
			},
		}

		dec, err := s.Route(context.Background(), sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Primary is still returned unchanged
		if dec.Route != "inline" || dec.Source != "deterministic" {
			t.Errorf("got decision %+v; want %+v", dec, primaryDecision)
		}

		if len(events) != 1 {
			t.Fatalf("expected 1 event on disagreement, got %d", len(events))
		}
		ev := events[0]
		if ev.Kind != "router_disagreement" {
			t.Errorf("ev.Kind = %q; want 'router_disagreement'", ev.Kind)
		}
		if ev.PrimaryRoute != "inline" {
			t.Errorf("ev.PrimaryRoute = %q; want 'inline'", ev.PrimaryRoute)
		}
		if ev.CandidateRoute != "worker" {
			t.Errorf("ev.CandidateRoute = %q; want 'worker'", ev.CandidateRoute)
		}
		if ev.Confidence != 0.75 {
			t.Errorf("ev.Confidence = %v; want 0.75", ev.Confidence)
		}
		if ev.Signals != sig {
			t.Errorf("ev.Signals = %+v; want %+v", ev.Signals, sig)
		}
		if ev.TS.IsZero() {
			t.Error("ev.TS is zero")
		}
	})

	t.Run("candidate JevError logs router_error with kind only", func(t *testing.T) {
		var events []router.Event
		s := router.Shadow{
			Primary: fakeRouter{dec: primaryDecision},
			Candidate: fakeRouter{err: &router.JevError{
				Kind:   "rate_limited",
				Status: 429,
			}},
			Log: func(ev router.Event) {
				events = append(events, ev)
			},
		}

		dec, err := s.Route(context.Background(), sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Route != "inline" {
			t.Errorf("dec.Route = %q; want 'inline'", dec.Route)
		}

		if len(events) != 1 {
			t.Fatalf("expected 1 event on candidate error, got %d", len(events))
		}
		ev := events[0]
		if ev.Kind != "router_error" {
			t.Errorf("ev.Kind = %q; want 'router_error'", ev.Kind)
		}
		if ev.ErrorKind != "rate_limited" {
			t.Errorf("ev.ErrorKind = %q; want 'rate_limited'", ev.ErrorKind)
		}
		if ev.PrimaryRoute != "" || ev.CandidateRoute != "" {
			t.Errorf("expected empty routes on error event, got primary=%q, candidate=%q", ev.PrimaryRoute, ev.CandidateRoute)
		}
	})

	t.Run("candidate generic error logs router_error with unknown", func(t *testing.T) {
		var events []router.Event
		s := router.Shadow{
			Primary:   fakeRouter{dec: primaryDecision},
			Candidate: fakeRouter{err: errors.New("something went wrong")},
			Log: func(ev router.Event) {
				events = append(events, ev)
			},
		}

		dec, err := s.Route(context.Background(), sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Route != "inline" {
			t.Errorf("dec.Route = %q; want 'inline'", dec.Route)
		}

		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		ev := events[0]
		if ev.Kind != "router_error" {
			t.Errorf("ev.Kind = %q; want 'router_error'", ev.Kind)
		}
		if ev.ErrorKind != "unknown" {
			t.Errorf("ev.ErrorKind = %q; want 'unknown'", ev.ErrorKind)
		}
	})

	t.Run("candidate panic recovered and logged as panic error", func(t *testing.T) {
		var events []router.Event
		s := router.Shadow{
			Primary:   fakeRouter{dec: primaryDecision},
			Candidate: fakeRouter{pan: "unexpected nil pointer dereference"},
			Log: func(ev router.Event) {
				events = append(events, ev)
			},
		}

		dec, err := s.Route(context.Background(), sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Route != "inline" {
			t.Errorf("dec.Route = %q; want 'inline'", dec.Route)
		}

		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		ev := events[0]
		if ev.Kind != "router_error" {
			t.Errorf("ev.Kind = %q; want 'router_error'", ev.Kind)
		}
		if ev.ErrorKind != "panic" {
			t.Errorf("ev.ErrorKind = %q; want 'panic'", ev.ErrorKind)
		}
	})

	t.Run("nil candidate is no-op", func(t *testing.T) {
		var logged bool
		s := router.Shadow{
			Primary:   fakeRouter{dec: primaryDecision},
			Candidate: nil,
			Log: func(ev router.Event) {
				logged = true
			},
		}

		dec, err := s.Route(context.Background(), sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(dec, primaryDecision) {
			t.Errorf("dec = %+v; want %+v", dec, primaryDecision)
		}
		if logged {
			t.Error("logged called for nil candidate")
		}
	})

	t.Run("nil log is no-op", func(t *testing.T) {
		s := router.Shadow{
			Primary: fakeRouter{dec: primaryDecision},
			Candidate: fakeRouter{dec: router.Decision{
				Route:      "worker",
				Source:     "jev",
				Confidence: 0.9,
			}},
			Log: nil,
		}

		dec, err := s.Route(context.Background(), sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(dec, primaryDecision) {
			t.Errorf("dec = %+v; want %+v", dec, primaryDecision)
		}
	})

	t.Run("primary error returned unchanged", func(t *testing.T) {
		primErr := errors.New("primary failed")
		var events []router.Event
		s := router.Shadow{
			Primary: fakeRouter{dec: primaryDecision, err: primErr},
			Candidate: fakeRouter{dec: router.Decision{
				Route:      "worker",
				Source:     "jev",
				Confidence: 0.9,
			}},
			Log: func(ev router.Event) {
				events = append(events, ev)
			},
		}

		dec, err := s.Route(context.Background(), sig)
		if !errors.Is(err, primErr) {
			t.Fatalf("expected primary error %v, got %v", primErr, err)
		}
		if !reflect.DeepEqual(dec, primaryDecision) {
			t.Errorf("dec = %+v; want %+v", dec, primaryDecision)
		}
	})
}
