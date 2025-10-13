package cmd

import (
	"errors"
	"testing"

	"github.com/rezkam/gritty/provider"
	"github.com/stretchr/testify/require"
)

func TestTruncateDiff(t *testing.T) {
	diff := "abcdefghijklmnopqrstuvwxyz"
	truncated, clipped, retained := truncateDiff(diff, 10)
	require.True(t, clipped)
	require.Equal(t, 10, retained)
	require.Contains(t, truncated, "[diff truncated to fit LLM context]")

	full, clippedFull, retainedFull := truncateDiff(diff, len(diff)+1)
	require.False(t, clippedFull)
	require.Equal(t, len(diff), retainedFull)
	require.Equal(t, diff, full)
}

func TestIsContextLengthError(t *testing.T) {
	err := &provider.ValidationError{
		Provider: "openai",
		Err:      errors.New("context length exceeded tokens"),
	}
	require.True(t, isContextLengthError(err))

	other := errors.New("network failure")
	require.False(t, isContextLengthError(other))
}
