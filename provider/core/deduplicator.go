package core

import "strings"

// MessageDeduplicator removes duplicate messages from a list.
// This is a common pattern across providers that make multiple API requests.
type MessageDeduplicator struct{}

// NewMessageDeduplicator creates a new message deduplicator.
func NewMessageDeduplicator() *MessageDeduplicator {
	return &MessageDeduplicator{}
}

// FilterDuplicates removes duplicate and empty messages from the input slice.
// Messages are compared after trimming whitespace.
// Returns a new slice with only unique, non-empty messages in the original order.
func (d *MessageDeduplicator) FilterDuplicates(messages []string) []string {
	seen := make(map[string]struct{}, len(messages))
	result := make([]string, 0, len(messages))

	for _, msg := range messages {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		if _, exists := seen[msg]; exists {
			continue
		}
		seen[msg] = struct{}{}
		result = append(result, msg)
	}

	return result
}
