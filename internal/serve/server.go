package serve

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrNonLoopback is returned when ListenAndServe is given an address that is
// not bound strictly to loopback (127.0.0.1 / localhost).
var ErrNonLoopback = errors.New("serve: refusing to bind non-loopback address; approvals UI is localhost-only")

// ListenAndServe starts an HTTP server bound strictly to loopback on the given addr.
// It rejects any non-loopback address to ensure unauthenticated approvals cannot be
// accessed over the network. It cleanly shuts down when ctx is cancelled.
func ListenAndServe(ctx context.Context, addr string, h http.Handler) error {
	if !IsLoopback(addr) {
		return fmt.Errorf("%w: %s", ErrNonLoopback, addr)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			errCh <- err
			return
		}
		defer ln.Close()
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// IsLoopback checks if addr represents a loopback address.
// Non-loopback addresses (such as 0.0.0.0, public IPs, or empty host) are rejected.
func IsLoopback(addr string) bool {
	if addr == "" {
		return false
	}
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	if host == "" {
		return false
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
