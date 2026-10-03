package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultJevURL is the production endpoint for SystemOne.
	DefaultJevURL = "https://api.typesafe.ai/v1/systemone"
	// JevModel is the model identifier used for routing decisions.
	JevModel = "jev-latest"
	// JevInstructions provides the fixed choice prompt instructions.
	JevInstructions = "Select the execution route for the task based on complexity and risk."
	// MaxJevResponseBodyBytes bounds response reading to 1 MiB.
	MaxJevResponseBodyBytes = 1024 * 1024
	// DefaultJevTimeout is the default request timeout.
	DefaultJevTimeout = 2 * time.Second
)

var jevCriteria = map[string]string{
	"inline": "A small change that one agent can do alone.",
	"worker": "Delegated work requiring a separate worker lane.",
	"fanout": "Parallel exploration with multiple lenses.",
}

// JevError is a typed error for failures encountered during Jev API calls.
// The API key never appears in its string representation.
type JevError struct {
	Kind   string // unauthorized, invalid_request, rate_limited, overloaded, http, transport, malformed
	Status int    // HTTP status code if available
}

func (e *JevError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("jev error: %s (status %d)", e.Kind, e.Status)
	}
	return fmt.Sprintf("jev error: %s", e.Kind)
}

// Jev is an HTTP adapter for the TypeSafe Jev routing API.
// It implements Router in shadow mode.
type Jev struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewJev creates and validates a new Jev adapter.
// BaseURL defaults to https://api.typesafe.ai/v1/systemone if empty.
// BaseURL must use https, except plain http is permitted only for loopback hosts.
func NewJev(baseURL, apiKey string) (*Jev, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("jev api key is required")
	}

	targetURL := strings.TrimSpace(baseURL)
	if targetURL == "" {
		targetURL = DefaultJevURL
	}

	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid jev url: %w", err)
	}

	switch u.Scheme {
	case "https":
		// HTTPS is always permitted
	case "http":
		hostname := u.Hostname()
		if !isLoopbackHost(hostname) {
			return nil, fmt.Errorf("http scheme is only allowed for loopback addresses (127.0.0.1, ::1, localhost); got %q", hostname)
		}
	default:
		return nil, fmt.Errorf("unsupported jev url scheme: %q", u.Scheme)
	}

	return &Jev{
		BaseURL: targetURL,
		APIKey:  apiKey,
		Timeout: DefaultJevTimeout,
		// Never follow redirects: the bearer key must only ever go to the validated URL.
		HTTPClient: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}, nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type jevChoiceQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type jevQuestions struct {
	Route jevChoiceQuestion `json:"route"`
}

type jevRequestBody struct {
	State     Signals      `json:"state"`
	Model     string       `json:"model"`
	Questions jevQuestions `json:"questions"`
}

type jevRouteAnswer struct {
	Type       string  `json:"type"`
	Choice     string  `json:"choice"`
	Confidence float64 `json:"confidence"`
}

type jevResponseBody struct {
	Model   string                    `json:"model"`
	Answers map[string]jevRouteAnswer `json:"answers"`
}

// Route sends numeric/boolean signals to Jev and returns the suggested Decision.
// Never retries; bounded by Timeout and 1 MiB response limit.
func (j *Jev) Route(ctx context.Context, s Signals) (Decision, error) {
	timeout := j.Timeout
	if timeout <= 0 {
		timeout = DefaultJevTimeout
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := jevRequestBody{
		State: s,
		Model: JevModel,
		Questions: jevQuestions{
			Route: jevChoiceQuestion{
				Type:         "choice",
				Instructions: JevInstructions,
				Criteria:     jevCriteria,
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return Decision{}, &JevError{Kind: "malformed"}
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, j.BaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return Decision{}, &JevError{Kind: "transport"}
	}
	req.Header.Set("Authorization", "Bearer "+j.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := j.HTTPClient
	if client == nil {
		client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Decision{}, &JevError{Kind: "transport"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		kind := "http"
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			kind = "unauthorized"
		case http.StatusUnprocessableEntity:
			kind = "invalid_request"
		case http.StatusTooManyRequests:
			kind = "rate_limited"
		case 529:
			kind = "overloaded"
		}
		// Drain a bounded amount of error body without logging or surfacing
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return Decision{}, &JevError{Kind: kind, Status: resp.StatusCode}
	}

	// Bounded response read (1 MiB + 1 byte to detect overflow)
	limited := io.LimitReader(resp.Body, int64(MaxJevResponseBodyBytes)+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return Decision{}, &JevError{Kind: "transport"}
	}
	if len(respBody) > MaxJevResponseBodyBytes {
		return Decision{}, &JevError{Kind: "malformed"}
	}

	var parsed jevResponseBody
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Decision{}, &JevError{Kind: "malformed"}
	}

	routeAns, ok := parsed.Answers["route"]
	if !ok || routeAns.Choice == "" {
		return Decision{}, &JevError{Kind: "malformed"}
	}

	switch routeAns.Choice {
	case "inline", "worker", "fanout":
		// Valid choices
	default:
		return Decision{}, &JevError{Kind: "malformed"}
	}

	return Decision{
		Route:      routeAns.Choice,
		Source:     "jev",
		Confidence: routeAns.Confidence,
	}, nil
}
