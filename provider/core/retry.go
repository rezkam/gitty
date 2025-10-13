package core

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryParser handles parsing retry-after headers from HTTP responses.
type RetryParser struct{}

// NewRetryParser creates a new retry parser.
func NewRetryParser() *RetryParser {
	return &RetryParser{}
}

// ParseRetryAfter parses the Retry-After header value and returns seconds.
// Supports both delta-seconds (integer) and HTTP-date formats.
// Returns 0 if the header is missing or invalid.
func (p *RetryParser) ParseRetryAfter(value string) int {
	if value == "" {
		return 0
	}

	// Try parsing as delta-seconds (integer)
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if seconds < 0 {
			return 0
		}
		return seconds
	}

	// Try parsing as an HTTP-date format (RFC 7231)
	if t, err := http.ParseTime(value); err == nil {
		seconds := int(time.Until(t).Seconds())
		if seconds < 0 {
			return 0
		}
		return seconds
	}

	return 0
}
