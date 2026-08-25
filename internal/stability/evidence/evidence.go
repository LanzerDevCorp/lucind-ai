// Package evidence implements bounded log sanitization, raw payload hashing,
// and RFC 8785 canonical Stability Receipt generation.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// StreamDetailCap bounds how much of a captured stream is retained in sanitized evidence.
const StreamDetailCap = 4096

// NoStreamDetail is recorded when no output was captured for a stream.
const NoStreamDetail = "(none captured)"

// StreamTruncatedMarker is appended to a stream detail that was truncated to StreamDetailCap.
const StreamTruncatedMarker = "...[truncated, showing last %d of %d bytes]"

// HashAlgorithm names the cryptographic digest algorithm used for raw payload hashing.
const HashAlgorithm = "sha256"

// absPathRegex matches absolute POSIX file paths like /foo/bar/baz.
var absPathRegex = regexp.MustCompile(`/(?:[a-zA-Z0-9_.-]+/)+[a-zA-Z0-9_.-]+`)

// StreamEvidence holds the cryptographic digest of the raw stream and its bounded sanitized representation.
type StreamEvidence struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
	RawBytes  int    `json:"raw_bytes"`
	Sanitized string `json:"sanitized"`
}

// DiagnosticEvidence holds sanitized evidence for both stdout and stderr streams.
type DiagnosticEvidence struct {
	Stdout StreamEvidence `json:"stdout"`
	Stderr StreamEvidence `json:"stderr"`
}

// HashPayload computes the SHA-256 hex digest of the raw byte slice.
func HashPayload(payload []byte) (algorithm string, digest string) {
	hash := sha256.Sum256(payload)
	return HashAlgorithm, hex.EncodeToString(hash[:])
}

// StripPaths removes absolute filesystem paths and explicit base paths from a string.
func StripPaths(input string, basePaths ...string) string {
	result := input
	for _, base := range basePaths {
		base = strings.TrimSpace(base)
		if base != "" {
			result = strings.ReplaceAll(result, base, "[path]")
		}
	}
	result = absPathRegex.ReplaceAllString(result, "[path]")
	return result
}

// FormatStream bounds a stream to StreamDetailCap keeping the tail if truncated.
func FormatStream(stream string) string {
	if stream == "" {
		return NoStreamDetail
	}
	if len(stream) <= StreamDetailCap {
		return stream
	}
	tail := stream[len(stream)-StreamDetailCap:]
	marker := fmt.Sprintf(StreamTruncatedMarker, StreamDetailCap, len(stream))
	return fmt.Sprintf("%s\n%s", tail, marker)
}

// SanitizeStream hashes the raw stream, strips absolute paths, and bounds the output to StreamDetailCap.
func SanitizeStream(rawStream string, basePaths ...string) StreamEvidence {
	algo, digest := HashPayload([]byte(rawStream))
	rawBytes := len(rawStream)

	if rawStream == "" {
		return StreamEvidence{
			Algorithm: algo,
			Digest:    digest,
			RawBytes:  0,
			Sanitized: NoStreamDetail,
		}
	}

	stripped := StripPaths(rawStream, basePaths...)
	formatted := FormatStream(stripped)

	return StreamEvidence{
		Algorithm: algo,
		Digest:    digest,
		RawBytes:  rawBytes,
		Sanitized: formatted,
	}
}

// SanitizeDiagnostics sanitizes both stdout and stderr streams into a DiagnosticEvidence record.
func SanitizeDiagnostics(stdout, stderr string, basePaths ...string) DiagnosticEvidence {
	return DiagnosticEvidence{
		Stdout: SanitizeStream(stdout, basePaths...),
		Stderr: SanitizeStream(stderr, basePaths...),
	}
}
