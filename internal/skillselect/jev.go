package skillselect

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
	"os"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/userconfig"
)

const (
	// DefaultJevURL is the production endpoint for SystemOne.
	DefaultJevURL = "https://api.typesafe.ai/v1/systemone"
	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
	// DefaultModel is the default routing/selection model identifier.
	DefaultModel = "jev-latest"
	// MaxResponseBodyBytes bounds response reading to 1 MiB.
	MaxResponseBodyBytes = 1024 * 1024
)

var (
	// ErrMissingAPIKey is returned when the API key is not set.
	ErrMissingAPIKey = errors.New("TYPESAFE_API_KEY is not set; export it or run lucind-ai install to store it in ~/.config/lucind/env")
	// ErrMalformedResponse is returned when the response cannot be understood.
	ErrMalformedResponse = errors.New("malformed response from typesafe api")
	// ErrMalformedJSON is returned when the response is not valid JSON.
	ErrMalformedJSON = errors.New("malformed JSON in response")
	// ErrOversizedBody is returned when the response exceeds MaxResponseBodyBytes.
	ErrOversizedBody = errors.New("response body exceeds 1 MiB limit")
)

// HTTPError represents a non-2xx HTTP response from the API.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("typesafe api http error: status %d: %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("typesafe api http error: status %d", e.StatusCode)
}

// MissingAnswerError represents a missing answer for a requested question.
type MissingAnswerError struct {
	QuestionID string
}

func (e *MissingAnswerError) Error() string {
	return fmt.Sprintf("missing answer for question %q", e.QuestionID)
}

// MalformedJSONError represents a JSON unmarshal failure.
type MalformedJSONError struct {
	Err error
}

func (e *MalformedJSONError) Error() string {
	return fmt.Sprintf("malformed JSON in response: %v", e.Err)
}

func (e *MalformedJSONError) Unwrap() error {
	return ErrMalformedResponse
}

func (e *MalformedJSONError) Is(target error) bool {
	return target == ErrMalformedJSON || target == ErrMalformedResponse
}

// OversizedBodyError represents a response that exceeded the 1 MiB limit.
type OversizedBodyError struct {
	Limit int
}

func (e *OversizedBodyError) Error() string {
	return fmt.Sprintf("response body exceeded limit of %d bytes", e.Limit)
}

func (e *OversizedBodyError) Unwrap() error {
	return ErrMalformedResponse
}

func (e *OversizedBodyError) Is(target error) bool {
	return target == ErrOversizedBody || target == ErrMalformedResponse
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL configures the base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithTimeout configures the request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = d
	}
}

// WithHTTPClient configures the underlying HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithModel configures the model.
func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

// Client is a plain net/http client for the TypeSafe SystemOne / Jev API.
type Client struct {
	baseURL    string
	key        string
	httpClient *http.Client
	timeout    time.Duration
	model      string
}

// NewClient creates a new Client with the provided API key and options.
func NewClient(key string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(key) == "" {
		return nil, ErrMissingAPIKey
	}

	c := &Client{
		baseURL: DefaultJevURL,
		key:     key,
		timeout: DefaultTimeout,
		model:   DefaultModel,
	}

	for _, opt := range opts {
		opt(c)
	}

	targetURL := strings.TrimSpace(c.baseURL)
	if targetURL == "" {
		targetURL = DefaultJevURL
		c.baseURL = targetURL
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

	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	// Never follow redirects: the bearer key must only reach the validated URL.
	c.httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return c, nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Post sends an HTTP POST request to the configured base URL with the given body.
func (c *Client) Post(ctx context.Context, body any) ([]byte, error) {
	timeout := c.timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, c.sanitize(fmt.Errorf("marshaling request: %w", err))
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = http.NoBody
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL, bodyReader)
	if err != nil {
		return nil, c.sanitize(fmt.Errorf("creating request: %w", err))
	}

	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.sanitize(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := readExcerpt(resp.Body, 512)
		if c.key != "" {
			snippet = strings.ReplaceAll(snippet, c.key, "[REDACTED]")
		}
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       snippet,
		}
	}

	limited := io.LimitReader(resp.Body, int64(MaxResponseBodyBytes)+1)
	respBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, c.sanitize(err)
	}

	if len(respBytes) > MaxResponseBodyBytes {
		return nil, &OversizedBodyError{Limit: MaxResponseBodyBytes}
	}

	return respBytes, nil
}

func (c *Client) sanitize(err error) error {
	if err == nil || c.key == "" {
		return err
	}
	if strings.Contains(err.Error(), c.key) {
		redacted := strings.ReplaceAll(err.Error(), c.key, "[REDACTED]")
		return &sanitizedError{cause: err, msg: redacted}
	}
	return err
}

type sanitizedError struct {
	cause error
	msg   string
}

func (s *sanitizedError) Error() string { return s.msg }
func (s *sanitizedError) Unwrap() error { return s.cause }

func readExcerpt(r io.Reader, max int64) string {
	lr := io.LimitReader(r, max)
	b, _ := io.ReadAll(lr)
	_, _ = io.Copy(io.Discard, io.LimitReader(r, 4096))
	return strings.TrimSpace(string(b))
}

// ResolveKey returns TYPESAFE_API_KEY from the environment if non-empty,
// or from the user config env file (~/.config/lucind/env).
func ResolveKey() string {
	if env := os.Getenv("TYPESAFE_API_KEY"); strings.TrimSpace(env) != "" {
		return strings.TrimSpace(env)
	}
	key, err := userconfig.ReadKey("TYPESAFE_API_KEY")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(key)
}
