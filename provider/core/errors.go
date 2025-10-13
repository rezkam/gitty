package core

import "fmt"

// AuthenticationError indicates invalid or missing API credentials.
type AuthenticationError struct {
	Provider string
	Err      error
}

func (e *AuthenticationError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("provider '%s' authentication failed: %v", e.Provider, e.Err)
}

func (e *AuthenticationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NetworkError indicates connection or HTTP transport failure.
type NetworkError struct {
	Provider string
	Endpoint string
	Err      error
}

func (e *NetworkError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("provider '%s' network error (%s): %v", e.Provider, e.Endpoint, e.Err)
}

func (e *NetworkError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ValidationError indicates invalid response data.
type ValidationError struct {
	Provider string
	Err      error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("provider '%s' returned invalid response: %v", e.Provider, e.Err)
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// RateLimitError indicates the provider API rate limit exceeded.
type RateLimitError struct {
	Provider   string
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	if e == nil {
		return ""
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("provider '%s' rate limit exceeded: retry after %ds", e.Provider, e.RetryAfter)
	}
	return fmt.Sprintf("provider '%s' rate limit exceeded", e.Provider)
}
