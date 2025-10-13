package service

import (
	"testing"

	"github.com/rezkam/gritty/provider"
	providercore "github.com/rezkam/gritty/provider/core"
	"github.com/stretchr/testify/require"
)

func TestGenerateDeduplicatesAndSanitizes(t *testing.T) {
	t.Parallel()

	mock := &provider.MockProvider{
		Messages:   []string{"Commit message: Improve feature", "Commit message: Improve feature", "<think>analysis</think>Refactor imports"},
		Capability: true,
	}

	service := NewCommitService(mock)
	req := providercore.CommitRequest{Diff: "diff", Count: 3}

	result, err := service.Generate(req)
	require.NoError(t, err)
	require.Equal(t, []string{"Improve feature", "Refactor imports"}, result.Messages)
	require.False(t, result.Downgraded)
	require.Equal(t, 1, mock.CallCount)
	require.Equal(t, 3, mock.LastRequest.Count)
}

func TestGenerateDowngradesWhenUnsupported(t *testing.T) {
	t.Parallel()

	mock := &provider.MockProvider{
		Messages:   []string{"One", "Two"},
		Capability: false,
	}

	service := NewCommitService(mock)
	req := providercore.CommitRequest{Diff: "diff", Count: 4}

	result, err := service.Generate(req)
	require.NoError(t, err)
	require.True(t, result.Downgraded)
	require.Equal(t, []string{"One"}, result.Messages)
	require.Equal(t, 1, mock.LastRequest.Count)
}

func TestClampCompletions(t *testing.T) {
	require.Equal(t, 1, clampCompletions(0))
	require.Equal(t, 3, clampCompletions(3))
	require.Equal(t, 5, clampCompletions(12))
}

func TestSanitizeCommitMessageRemovesThinkAndCodeFences(t *testing.T) {
	raw := "<think>analysis</think>\n```\nCommit message: Improve feature\n```"
	clean := sanitizeCommitMessage(raw)
	require.Equal(t, "Improve feature", clean)
}

func TestSanitizeCommitMessagePicksFirstLine(t *testing.T) {
	raw := "Commit message: Add API support\n\nWhy: enable external integrations"
	clean := sanitizeCommitMessage(raw)
	require.Equal(t, "Add API support\n\nWhy: enable external integrations", clean)
}

func TestSanitizeCommitMessagePreservesExplanation(t *testing.T) {
	raw := "Fix parser bug\n\n- Handle quoted strings correctly\n- Improve error reporting"
	clean := sanitizeCommitMessage(raw)
	require.Equal(t, "Fix parser bug\n\n- Handle quoted strings correctly\n- Improve error reporting", clean)
}

func TestSanitizeCommitMessageDropsHedgedExplanation(t *testing.T) {
	raw := "Refactor imports\n\nThis likely improves maintainability"
	clean := sanitizeCommitMessage(raw)
	require.Equal(t, "Refactor imports", clean)
}
