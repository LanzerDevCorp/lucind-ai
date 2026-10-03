package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/router"
)

func TestNewJev_URLSafetyAndKey(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid https default",
			baseURL: "",
			apiKey:  "test-secret-key",
			wantErr: false,
		},
		{
			name:    "valid https custom url",
			baseURL: "https://custom.typesafe.ai/v1/systemone",
			apiKey:  "test-secret-key",
			wantErr: false,
		},
		{
			name:    "valid loopback 127.0.0.1 http",
			baseURL: "http://127.0.0.1:8080/v1/systemone",
			apiKey:  "test-secret-key",
			wantErr: false,
		},
		{
			name:    "valid loopback localhost http",
			baseURL: "http://localhost:9090",
			apiKey:  "test-secret-key",
			wantErr: false,
		},
		{
			name:    "valid loopback ipv6 http",
			baseURL: "http://[::1]:8080",
			apiKey:  "test-secret-key",
			wantErr: false,
		},
		{
			name:    "rejected non-loopback http",
			baseURL: "http://example.com/v1/systemone",
			apiKey:  "test-secret-key",
			wantErr: true,
		},
		{
			name:    "rejected non-loopback ip http",
			baseURL: "http://192.168.1.1:8080",
			apiKey:  "test-secret-key",
			wantErr: true,
		},
		{
			name:    "rejected empty api key",
			baseURL: "https://api.typesafe.ai/v1/systemone",
			apiKey:  "",
			wantErr: true,
		},
		{
			name:    "rejected whitespace api key",
			baseURL: "https://api.typesafe.ai/v1/systemone",
			apiKey:  "   ",
			wantErr: true,
		},
		{
			name:    "rejected invalid url",
			baseURL: "://invalid-url",
			apiKey:  "test-secret-key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j, err := router.NewJev(tt.baseURL, tt.apiKey)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewJev(%q, %q) expected error, got nil", tt.baseURL, tt.apiKey)
				}
				if j != nil {
					t.Fatalf("expected nil *Jev on error, got %+v", j)
				}
			} else {
				if err != nil {
					t.Fatalf("NewJev(%q, %q) unexpected error: %v", tt.baseURL, tt.apiKey, err)
				}
				if j == nil {
					t.Fatal("expected non-nil *Jev, got nil")
				}
			}
		})
	}
}

func TestJev_RequestShapeAndResponseParsing(t *testing.T) {
	const secretKey = "test-secret-key-12345"
	var (
		gotMethod      string
		gotAuthHeader  string
		gotContentType string
		gotRawBody     []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuthHeader = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		var err error
		gotRawBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"route": {
					"type": "choice",
					"choice": "worker",
					"confidence": 0.85
				}
			},
			"usage": {
				"total_tokens": 120
			}
		}`))
	}))
	defer srv.Close()

	jev, err := router.NewJev(srv.URL, secretKey)
	if err != nil {
		t.Fatalf("NewJev error: %v", err)
	}

	sig := router.Signals{
		AllowedPathCount: 3,
		NewFile:          true,
		RiskTierLevel:    2,
		ReadOnly:         false,
	}

	dec, err := jev.Route(context.Background(), sig)
	if err != nil {
		t.Fatalf("Route unexpected error: %v", err)
	}

	// Verify Decision
	if dec.Route != "worker" {
		t.Errorf("dec.Route = %q; want 'worker'", dec.Route)
	}
	if dec.Source != "jev" {
		t.Errorf("dec.Source = %q; want 'jev'", dec.Source)
	}
	if dec.Confidence != 0.85 {
		t.Errorf("dec.Confidence = %v; want 0.85", dec.Confidence)
	}

	// Verify HTTP Request shape
	if gotMethod != http.MethodPost {
		t.Errorf("gotMethod = %q; want POST", gotMethod)
	}
	if gotAuthHeader != "Bearer "+secretKey {
		t.Errorf("gotAuthHeader = %q; want 'Bearer %s'", gotAuthHeader, secretKey)
	}
	if gotContentType != "application/json" {
		t.Errorf("gotContentType = %q; want 'application/json'", gotContentType)
	}

	// Verify request JSON body structure
	var reqMap map[string]interface{}
	if err := json.Unmarshal(gotRawBody, &reqMap); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	if reqMap["model"] != "jev-latest" {
		t.Errorf("reqMap[model] = %v; want 'jev-latest'", reqMap["model"])
	}

	// Verify state contains only numeric/boolean fields
	state, ok := reqMap["state"].(map[string]interface{})
	if !ok {
		t.Fatalf("state is not a JSON object: %v", reqMap["state"])
	}
	for k, v := range state {
		switch v.(type) {
		case float64, bool:
			// valid JSON numbers unmarshal as float64, booleans as bool
		default:
			t.Errorf("state field %q has type %T (%v); must be only number or bool", k, v, v)
		}
	}

	// Verify questions.route structure
	questions, ok := reqMap["questions"].(map[string]interface{})
	if !ok {
		t.Fatalf("questions is not a JSON object: %v", reqMap["questions"])
	}
	routeQ, ok := questions["route"].(map[string]interface{})
	if !ok {
		t.Fatalf("questions.route is not a JSON object: %v", questions["route"])
	}
	if routeQ["type"] != "choice" {
		t.Errorf("questions.route.type = %v; want 'choice'", routeQ["type"])
	}
	if _, ok := routeQ["instructions"].(string); !ok {
		t.Errorf("questions.route.instructions is missing or not a string: %v", routeQ["instructions"])
	}
	criteria, ok := routeQ["criteria"].(map[string]interface{})
	if !ok {
		t.Fatalf("questions.route.criteria is not an object: %v", routeQ["criteria"])
	}
	for _, opt := range []string{"inline", "worker", "fanout"} {
		if desc, ok := criteria[opt].(string); !ok || strings.TrimSpace(desc) == "" {
			t.Errorf("criteria[%q] missing or empty: %v", opt, criteria[opt])
		}
	}

	// Assert NO field in raw body contains a path-like string (like "/" or ".go" or ".md")
	// besides the static criteria/instructions
	bodyStr := string(gotRawBody)
	for _, forbidden := range []string{".go", ".md", "/home", "cmd/", "internal/", "docs/"} {
		if strings.Contains(bodyStr, forbidden) {
			t.Errorf("request body contains forbidden substring %q: %s", forbidden, bodyStr)
		}
	}
}

func TestJev_ErrorHandlingAndKinds(t *testing.T) {
	const secretKey = "super-secret-key-to-not-leak"

	tests := []struct {
		name         string
		handler      http.HandlerFunc
		closeEarly   bool
		wantKind     string
		wantStatus   int
		overrideTime time.Duration
	}{
		{
			name: "401 unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			},
			wantKind:   "unauthorized",
			wantStatus: 401,
		},
		{
			name: "422 invalid request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"error":"unprocessable"}`))
			},
			wantKind:   "invalid_request",
			wantStatus: 422,
		},
		{
			name: "429 rate limited",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			},
			wantKind:   "rate_limited",
			wantStatus: 429,
		},
		{
			name: "529 overloaded",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(529)
				_, _ = w.Write([]byte(`{"error":"overloaded"}`))
			},
			wantKind:   "overloaded",
			wantStatus: 529,
		},
		{
			name: "500 internal server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal error"}`))
			},
			wantKind:   "http",
			wantStatus: 500,
		},
		{
			name: "malformed JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`not json at all`))
			},
			wantKind: "malformed",
		},
		{
			name: "missing answer",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"model":"jev-latest","answers":{}}`))
			},
			wantKind: "malformed",
		},
		{
			name: "unknown choice",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"model":"jev-latest","answers":{"route":{"choice":"unknown-choice","confidence":0.9}}}`))
			},
			wantKind: "malformed",
		},
		{
			name: "oversized body over 1MiB",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				// write 1 MiB + 10 bytes of spaces and json
				prefix := `{"model":"jev-latest","answers":{"route":{"choice":"worker","confidence":0.8}},"padding":"`
				_, _ = w.Write([]byte(prefix))
				padding := make([]byte, 1024*1024+100)
				for i := range padding {
					padding[i] = 'a'
				}
				_, _ = w.Write(padding)
				_, _ = w.Write([]byte(`"}`))
			},
			wantKind: "malformed",
		},
		{
			name: "server timeout via slow handler",
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(150 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			},
			wantKind:     "transport",
			overrideTime: 50 * time.Millisecond,
		},
		{
			name:       "closed server",
			closeEarly: true,
			wantKind:   "transport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			if tt.handler != nil {
				srv = httptest.NewServer(tt.handler)
			} else {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			}
			url := srv.URL
			if tt.closeEarly {
				srv.Close()
			} else {
				defer srv.Close()
			}

			jev, err := router.NewJev(url, secretKey)
			if err != nil {
				t.Fatalf("NewJev error: %v", err)
			}
			if tt.overrideTime > 0 {
				jev.Timeout = tt.overrideTime
			}

			_, routeErr := jev.Route(context.Background(), router.Signals{})
			if routeErr == nil {
				t.Fatal("expected Route error, got nil")
			}

			var jevErr *router.JevError
			if !errors.As(routeErr, &jevErr) {
				t.Fatalf("expected *router.JevError, got %T: %v", routeErr, routeErr)
			}
			if jevErr.Kind != tt.wantKind {
				t.Errorf("jevErr.Kind = %q; want %q", jevErr.Kind, tt.wantKind)
			}
			if tt.wantStatus != 0 && jevErr.Status != tt.wantStatus {
				t.Errorf("jevErr.Status = %d; want %d", jevErr.Status, tt.wantStatus)
			}

			// Assert secretKey is NEVER present in the error string or representations
			errStr := routeErr.Error()
			if strings.Contains(errStr, secretKey) {
				t.Fatalf("SECRET KEY LEAKED in error message: %q", errStr)
			}
		})
	}
}

func TestJevDoesNotFollowRedirectsAndNeverLeaksTheKey(t *testing.T) {
	var targetHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&targetHits, 1)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	jev, err := router.NewJev(redirector.URL, "secret-key-123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = jev.Route(context.Background(), router.Signals{AllowedPathCount: 1})
	var jerr *router.JevError
	if !errors.As(err, &jerr) || jerr.Kind != "http" || jerr.Status != http.StatusTemporaryRedirect {
		t.Fatalf("err = %v, want JevError http 307", err)
	}
	if got := atomic.LoadInt32(&targetHits); got != 0 {
		t.Fatalf("redirect target was contacted %d times; redirects must never be followed", got)
	}
	if strings.Contains(err.Error(), "secret-key-123") {
		t.Fatal("API key leaked into the error")
	}
}
