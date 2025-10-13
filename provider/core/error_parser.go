package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrorParser provides common HTTP error handling logic.
type ErrorParser struct {
	providerName string
	retryParser  *RetryParser
}

// NewErrorParser creates a new error parser for a provider.
func NewErrorParser(providerName string) *ErrorParser {
	return &ErrorParser{
		providerName: providerName,
		retryParser:  NewRetryParser(),
	}
}

// ParseHTTPError converts an HTTP error response into a typed error.
// This centralizes error handling logic common across providers.
func (e *ErrorParser) ParseHTTPError(statusCode int, headers http.Header, body []byte) error {
	message := e.extractErrorMessage(body)

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		if message == "" {
			message = "invalid API key"
		}
		return &AuthenticationError{
			Provider: e.providerName,
			Err:      fmt.Errorf("%s", message),
		}
	case http.StatusTooManyRequests:
		return &RateLimitError{
			Provider:   e.providerName,
			RetryAfter: e.retryParser.ParseRetryAfter(headers.Get("Retry-After")),
		}
	case http.StatusBadRequest:
		if message == "" {
			message = "invalid request"
		}
		return &ValidationError{
			Provider: e.providerName,
			Err:      fmt.Errorf("%s", message),
		}
	case http.StatusInternalServerError, http.StatusServiceUnavailable, 529:
		if message == "" {
			message = "service error"
		}
		return &NetworkError{
			Provider: e.providerName,
			Endpoint: "", // Caller can set this if needed
			Err:      fmt.Errorf("%s", message),
		}
	default:
		if message == "" {
			message = fmt.Sprintf("unexpected status %d", statusCode)
		}
		return &ValidationError{
			Provider: e.providerName,
			Err:      fmt.Errorf("%s", message),
		}
	}
}

// extractErrorMessage attempts to extract a meaningful error message from response body.
// Tries multiple common error response formats.
func (e *ErrorParser) extractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// Try parsing as JSON with nested error object
	var payload struct {
		Error interface{} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error != nil {
		switch v := payload.Error.(type) {
		case string:
			return strings.TrimSpace(v)
		case map[string]interface{}:
			if msg, ok := v["message"].(string); ok && msg != "" {
				return strings.TrimSpace(msg)
			}
			if code, ok := v["code"].(string); ok && code != "" {
				return strings.TrimSpace(code)
			}
		}
	}

	// Fallback to raw body
	return strings.TrimSpace(string(body))
}

// ReadErrorBody reads and returns the response body for error handling.
// This is a helper to avoid repetition in providers.
func (e *ErrorParser) ReadErrorBody(resp *http.Response) []byte {
	body, _ := io.ReadAll(resp.Body)
	return body
}
