package core

import (
	"net/http"
	"time"
)

// HTTPClient defines the interface for making HTTP requests.
// This follows the Dependency Inversion Principle (DIP) by depending on
// abstractions rather than concrete implementations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPClientFactory creates HTTP clients with consistent configuration.
type HTTPClientFactory struct{}

// NewHTTPClientFactory creates a new HTTP client factory.
func NewHTTPClientFactory() *HTTPClientFactory {
	return &HTTPClientFactory{}
}

// CreateClient creates a new HTTP client with the specified timeout.
func (f *HTTPClientFactory) CreateClient(timeoutSeconds int) HTTPClient {
	return &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}
