package provider

// MockProvider implements Provider for tests and CLI scenarios without
// performing network calls.
type MockProvider struct {
	Messages    []string
	Err         error
	Capability  bool
	CallCount   int
	LastRequest struct {
		Diff string
		N    int
	}
}

// GetCommitMessages returns the pre-seeded messages or the configured error.
func (m *MockProvider) GetCommitMessages(diff string, n int) ([]string, error) {
	m.CallCount++
	m.LastRequest.Diff = diff
	// Mirror production providers: clamp requested count to available messages so
	// tests observe the same effective completion count real callers would get.
	effectiveN := max(min(n, len(m.Messages)), 0)
	m.LastRequest.N = effectiveN

	if m.Err != nil {
		return nil, m.Err
	}

	result := make([]string, effectiveN)
	copy(result, m.Messages[:effectiveN])
	return result, nil
}

// SupportsMultipleCompletions reports the mocked capability.
func (m *MockProvider) SupportsMultipleCompletions() bool {
	return m.Capability
}

var _ Provider = (*MockProvider)(nil)
