package provider

import (
	providercore "github.com/rezkam/gritty/provider/core"
)

// MockProvider implements Provider for tests and CLI scenarios without
// performing network calls.
type MockProvider struct {
	Messages    []string
	Err         error
	Capability  bool
	CallCount   int
	LastRequest providercore.CommitRequest
}

// GenerateMessages returns the pre-seeded messages or the configured error.
func (m *MockProvider) GenerateMessages(req providercore.CommitRequest) (providercore.CommitResponse, error) {
	req.Normalize()
	m.CallCount++

	if m.Err != nil {
		return providercore.CommitResponse{}, m.Err
	}

	effectiveN := max(min(req.Count, len(m.Messages)), 0)
	req.Count = effectiveN
	m.LastRequest = req
	suggestions := make([]providercore.CommitSuggestion, 0, effectiveN)
	for i := 0; i < effectiveN; i++ {
		suggestions = append(suggestions, providercore.CommitSuggestion{Message: m.Messages[i]})
	}

	return providercore.CommitResponse{Suggestions: suggestions}, nil
}

// SupportsMultipleCompletions reports the mocked capability.
func (m *MockProvider) SupportsMultipleCompletions() bool {
	return m.Capability
}

var _ Provider = (*MockProvider)(nil)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
