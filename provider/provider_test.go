package provider

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMockProviderSupportsMultipleCompletions(t *testing.T) {
	t.Parallel()

	mock := &MockProvider{
		Messages:   []string{"one", "two", "three"},
		Capability: true,
	}

	msgs, err := mock.GetCommitMessages("diff", 2)
	require.NoError(t, err)
	require.Equal(t, []string{"one", "two"}, msgs)
	require.True(t, mock.SupportsMultipleCompletions())
	require.Equal(t, 1, mock.CallCount)
	require.Equal(t, "diff", mock.LastRequest.Diff)
	require.Equal(t, 2, mock.LastRequest.N)
}

func TestMockProviderRespectsRequestedCount(t *testing.T) {
	t.Parallel()

	mock := &MockProvider{
		Messages: []string{"one"},
	}

	msgs, err := mock.GetCommitMessages("diff", 5)
	require.NoError(t, err)
	require.Equal(t, []string{"one"}, msgs)
	require.False(t, mock.SupportsMultipleCompletions())
	require.Equal(t, 1, mock.CallCount)
	require.Equal(t, 1, mock.LastRequest.N)
}

func TestGetFactoryReturnsRegisteredProvider(t *testing.T) {
	t.Parallel()

	factory, err := GetFactory("openai")
	require.NoError(t, err)
	require.NotNil(t, factory)
}

func TestGetFactoryUnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := GetFactory("unknown")
	require.Error(t, err)
	require.Contains(t, err.Error(), "provider 'unknown'")
}

func TestGetConfigSetterReturnsRegisteredProvider(t *testing.T) {
	t.Parallel()

	setter, err := GetConfigSetter("openai")
	require.NoError(t, err)
	require.NotNil(t, setter)
}

func TestGetConfigSetterUnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := GetConfigSetter("missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "provider 'missing'")
}

func TestAuthenticationErrorWrapsUnderlyingError(t *testing.T) {
	t.Parallel()

	underlying := errors.New("invalid API key")
	err := &AuthenticationError{
		Provider: "claude",
		Err:      underlying,
	}

	require.ErrorContains(t, err, "provider 'claude' authentication failed")
	require.ErrorIs(t, err, underlying)
}

func TestNetworkErrorIncludesEndpoint(t *testing.T) {
	t.Parallel()

	underlying := errors.New("dial tcp 127.0.0.1: connection refused")
	err := &NetworkError{
		Provider: "localllm",
		Endpoint: "http://localhost:1234",
		Err:      underlying,
	}

	require.EqualError(t, err, fmt.Sprintf("provider 'localllm' network error (%s): %v", err.Endpoint, underlying))
	require.ErrorIs(t, err, underlying)
}

func TestValidationErrorFormatsMessage(t *testing.T) {
	t.Parallel()

	underlying := errors.New("missing field choices")
	err := &ValidationError{
		Provider: "openai",
		Err:      underlying,
	}

	require.ErrorContains(t, err, "provider 'openai' returned invalid response")
	require.ErrorIs(t, err, underlying)
}

func TestRateLimitErrorWithRetryAfter(t *testing.T) {
	t.Parallel()

	err := &RateLimitError{
		Provider:   "claude",
		RetryAfter: 60,
	}

	require.EqualError(t, err, "provider 'claude' rate limit exceeded: retry after 60s")
}

func TestRateLimitErrorWithoutRetryAfter(t *testing.T) {
	t.Parallel()

	err := &RateLimitError{
		Provider: "claude",
	}

	require.EqualError(t, err, "provider 'claude' rate limit exceeded")
}
