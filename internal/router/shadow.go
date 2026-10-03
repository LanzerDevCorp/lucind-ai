package router

import (
	"context"
	"errors"
	"time"
)

// Event records a shadow routing disagreement or candidate error.
type Event struct {
	Kind           string    `json:"kind"`
	PrimaryRoute   string    `json:"primary_route,omitempty"`
	CandidateRoute string    `json:"candidate_route,omitempty"`
	Confidence     float64   `json:"confidence,omitempty"`
	ErrorKind      string    `json:"error_kind,omitempty"`
	Signals        Signals   `json:"signals,omitempty"`
	TS             time.Time `json:"ts"`
}

// Shadow runs a Candidate Router alongside a Primary Router.
// It ALWAYS returns the Primary decision and error unchanged.
// The Candidate never has decision authority and cannot affect the returned decision.
type Shadow struct {
	Primary   Router
	Candidate Router
	Log       func(Event)
}

// Route executes the Primary router, then executes the Candidate router in the background
// of this call, logging disagreements and errors via Log.
func (s Shadow) Route(ctx context.Context, sig Signals) (Decision, error) {
	var (
		primDec Decision
		primErr error
	)

	if s.Primary != nil {
		primDec, primErr = s.Primary.Route(ctx, sig)
	}

	if s.Candidate != nil && s.Log != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.Log(Event{
						Kind:      "router_error",
						ErrorKind: "panic",
						Signals:   sig,
						TS:        time.Now().UTC(),
					})
				}
			}()

			candDec, candErr := s.Candidate.Route(ctx, sig)
			if candErr != nil {
				errKind := "unknown"
				var jevErr *JevError
				if errors.As(candErr, &jevErr) && jevErr.Kind != "" {
					errKind = jevErr.Kind
				}
				s.Log(Event{
					Kind:      "router_error",
					ErrorKind: errKind,
					Signals:   sig,
					TS:        time.Now().UTC(),
				})
				return
			}

			if candDec.Route != primDec.Route {
				s.Log(Event{
					Kind:           "router_disagreement",
					PrimaryRoute:   primDec.Route,
					CandidateRoute: candDec.Route,
					Confidence:     candDec.Confidence,
					Signals:        sig,
					TS:             time.Now().UTC(),
				})
			}
		}()
	}

	return primDec, primErr
}
