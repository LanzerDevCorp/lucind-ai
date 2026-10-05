package skillselect_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

func TestNewClient_URLSafetyAndKey(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		key     string
		wantErr bool
		isKey   bool
	}{
		{
			name:    "valid default https",
			baseURL: "",
			key:     "valid-key-123",
			wantErr: false,
		},
		{
			name:    "valid custom https",
			baseURL: "https://custom.api.typesafe.ai/v1/systemone",
			key:     "valid-key-123",
			wantErr: false,
		},
		{
			name:    "valid loopback 127.0.0.1 http",
			baseURL: "http://127.0.0.1:8080/v1/systemone",
			key:     "valid-key-123",
			wantErr: false,
		},
		{
			name:    "valid loopback localhost http",
			baseURL: "http://localhost:9090",
			key:     "valid-key-123",
			wantErr: false,
		},
		{
			name:    "valid loopback ipv6 http",
			baseURL: "http://[::1]:8080",
			key:     "valid-key-123",
			wantErr: false,
		},
		{
			name:    "rejected non-loopback http hostname",
			baseURL: "http://api.typesafe.ai/v1/systemone",
			key:     "valid-key-123",
			wantErr: true,
		},
		{
			name:    "rejected non-loopback http ip",
			baseURL: "http://192.168.1.100:8080",
			key:     "valid-key-123",
			wantErr: true,
		},
		{
			name:    "rejected empty key",
			baseURL: "https://api.typesafe.ai/v1/systemone",
			key:     "",
			wantErr: true,
			isKey:   true,
		},
		{
			name:    "rejected whitespace key",
			baseURL: "https://api.typesafe.ai/v1/systemone",
			key:     "   \t  ",
			wantErr: true,
			isKey:   true,
		},
		{
			name:    "rejected malformed url",
			baseURL: "://malformed",
			key:     "valid-key-123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []skillselect.Option{}
			if tt.baseURL != "" {
				opts = append(opts, skillselect.WithBaseURL(tt.baseURL))
			}
			client, err := skillselect.NewClient(tt.key, opts...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil client: %+v", client)
				}
				if tt.isKey && !errors.Is(err, skillselect.ErrMissingAPIKey) {
					t.Errorf("expected ErrMissingAPIKey, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if client == nil {
					t.Fatal("expected non-nil client")
				}
			}
		})
	}
}

func TestClient_KeyNeverLeaksInErrors(t *testing.T) {
	const secretKey = "super-secret-key-that-must-not-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		// Server maliciously echoes back the secret key in the error body
		_, _ = w.Write([]byte(`{"error":"unauthorized with token: ` + secretKey + `"}`))
	}))
	defer srv.Close()

	client, err := skillselect.NewClient(secretKey, skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, postErr := client.Post(context.Background(), map[string]string{"foo": "bar"})
	if postErr == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := postErr.Error()
	if strings.Contains(errStr, secretKey) {
		t.Fatalf("SECRET KEY LEAKED in error message: %q", errStr)
	}
}

func TestClient_RedirectNotFollowed(t *testing.T) {
	var targetHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&targetHits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	const secretKey = "do-not-send-to-target"
	client, err := skillselect.NewClient(secretKey, skillselect.WithBaseURL(redirector.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, postErr := client.Post(context.Background(), map[string]string{"test": "payload"})
	if postErr == nil {
		t.Fatal("expected error on redirect, got nil")
	}

	var httpErr *skillselect.HTTPError
	if !errors.As(postErr, &httpErr) {
		t.Fatalf("expected *skillselect.HTTPError, got %T: %v", postErr, postErr)
	}
	if httpErr.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("got status %d, want %d", httpErr.StatusCode, http.StatusTemporaryRedirect)
	}

	if got := atomic.LoadInt32(&targetHits); got != 0 {
		t.Fatalf("target was contacted %d times; redirects must never be followed", got)
	}
	if strings.Contains(postErr.Error(), secretKey) {
		t.Fatal("secret key leaked in error message")
	}
}

func TestClient_OversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Send 1 MiB + 10 bytes
		padding := make([]byte, skillselect.MaxResponseBodyBytes+10)
		for i := range padding {
			padding[i] = 'a'
		}
		_, _ = w.Write(padding)
	}))
	defer srv.Close()

	client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, postErr := client.Post(context.Background(), map[string]string{"test": "data"})
	if postErr == nil {
		t.Fatal("expected error for oversized body, got nil")
	}

	if !errors.Is(postErr, skillselect.ErrOversizedBody) && !errors.Is(postErr, skillselect.ErrMalformedResponse) {
		t.Errorf("expected ErrOversizedBody or ErrMalformedResponse, got %v", postErr)
	}
}

func TestClient_Non2xxStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":"invalid_api_key"}`,
		},
		{
			name:       "422 unprocessable entity",
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"error":"invalid_questions"}`,
		},
		{
			name:       "429 rate limited",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"rate_limited"}`,
		},
		{
			name:       "500 internal server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":"server_error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client, err := skillselect.NewClient("test-key", skillselect.WithBaseURL(srv.URL))
			if err != nil {
				t.Fatalf("NewClient error: %v", err)
			}

			_, postErr := client.Post(context.Background(), nil)
			if postErr == nil {
				t.Fatal("expected error, got nil")
			}

			var httpErr *skillselect.HTTPError
			if !errors.As(postErr, &httpErr) {
				t.Fatalf("expected *skillselect.HTTPError, got %T: %v", postErr, postErr)
			}
			if httpErr.StatusCode != tt.statusCode {
				t.Errorf("got status %d, want %d", httpErr.StatusCode, tt.statusCode)
			}
			if !strings.Contains(httpErr.Body, tt.body) {
				t.Errorf("httpErr.Body = %q, want it to contain %q", httpErr.Body, tt.body)
			}
		})
	}
}

func TestKeyFromEnv(t *testing.T) {
	orig := os.Getenv("TYPESAFE_API_KEY")
	defer func() {
		_ = os.Setenv("TYPESAFE_API_KEY", orig)
	}()

	_ = os.Setenv("TYPESAFE_API_KEY", "env-test-key-999")
	if got := skillselect.KeyFromEnv(); got != "env-test-key-999" {
		t.Errorf("KeyFromEnv() = %q, want 'env-test-key-999'", got)
	}

	_ = os.Unsetenv("TYPESAFE_API_KEY")
	if got := skillselect.KeyFromEnv(); got != "" {
		t.Errorf("KeyFromEnv() = %q, want empty string", got)
	}
}
