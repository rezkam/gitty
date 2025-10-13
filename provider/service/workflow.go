package service

import (
	"regexp"
	"strings"

	"github.com/rezkam/gritty/provider"
	providercore "github.com/rezkam/gritty/provider/core"
)

const (
	defaultMaxCompletions  = 5
	minCompletionCount     = 1
	maxCommitMessageLength = 80
)

var (
	thinkBlockPattern   = regexp.MustCompile(`(?s)<think>.*?</think>`)
	markdownQuotePrefix = regexp.MustCompile(`^>\s*`)
)

// Result contains normalized provider output alongside metadata.
type Result struct {
	Messages         []string
	Downgraded       bool
	ProviderResponse providercore.CommitResponse
}

// CommitService orchestrates provider calls and normalizes results.
type CommitService struct {
	provider     provider.Provider
	deduplicator *providercore.MessageDeduplicator
}

// NewCommitService constructs a CommitService with sensible defaults.
func NewCommitService(p provider.Provider) *CommitService {
	return &CommitService{
		provider:     p,
		deduplicator: providercore.NewMessageDeduplicator(),
	}
}

// Generate produces sanitized commit message suggestions.
func (s *CommitService) Generate(req providercore.CommitRequest) (Result, error) {
	req.Normalize()
	req.Count = clampCompletions(req.Count)

	downgraded := false
	if req.Count > minCompletionCount && !s.provider.SupportsMultipleCompletions() {
		req.Count = minCompletionCount
		downgraded = true
	}

	resp, err := s.provider.GenerateMessages(req)
	if err != nil {
		return Result{}, err
	}

	cleaned := sanitizeCommitMessages(resp.Messages())
	deduped := s.deduplicator.FilterDuplicates(cleaned)
	if len(deduped) > req.Count {
		deduped = deduped[:req.Count]
	}

	return Result{
		Messages:         deduped,
		Downgraded:       downgraded,
		ProviderResponse: resp,
	}, nil
}

func clampCompletions(value int) int {
	if value < minCompletionCount {
		return minCompletionCount
	}
	if value > defaultMaxCompletions {
		return defaultMaxCompletions
	}
	return value
}

func sanitizeCommitMessages(messages []string) []string {
	result := make([]string, 0, len(messages))
	for _, message := range messages {
		if clean := sanitizeCommitMessage(message); clean != "" {
			result = append(result, clean)
		}
	}
	return result
}

func sanitizeCommitMessage(message string) string {
	if message == "" {
		return ""
	}

	clean := thinkBlockPattern.ReplaceAllString(message, "")
	clean = unwrapCodeFences(clean)
	clean = markdownQuotePrefix.ReplaceAllString(clean, "")
	clean = strings.TrimSpace(clean)

	if clean == "" {
		return ""
	}

	lines := strings.Split(clean, "\n")
	commitLine := ""
	explanations := make([]string, 0, len(lines))

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		line = markdownQuotePrefix.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		line = trimSurroundingQuotes(line)
		if line == "" {
			continue
		}

		if commitLine == "" {
			line = normalizeCommitLabel(line)
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasSuffix(line, ":") {
				continue
			}
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if len([]rune(line)) > maxCommitMessageLength {
				line = truncateSentence(line, maxCommitMessageLength)
			}
			commitLine = line
			continue
		}

		if line != "" && shouldIncludeExplanation(line) {
			explanations = append(explanations, line)
		}
	}

	if commitLine == "" {
		commitLine = strings.TrimSpace(trimSurroundingQuotes(clean))
	}

	if len(explanations) == 0 {
		return commitLine
	}

	return commitLine + "\n\n" + strings.Join(explanations, "\n")
}

func normalizeCommitLabel(line string) string {
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "commit message:") {
		return strings.TrimSpace(line[len("commit message:"):])
	}
	if strings.HasPrefix(lower, "commit:") {
		return strings.TrimSpace(line[len("commit:"):])
	}
	return line
}

func truncateSentence(text string, limit int) string {
	if len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:limit]))
}

func unwrapCodeFences(text string) string {
	result := text
	for {
		start := strings.Index(result, "```")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+3:], "```")
		if end == -1 {
			break
		}
		contentStart := start + 3
		contentEnd := contentStart + end
		content := strings.TrimSpace(result[contentStart:contentEnd])
		result = result[:start] + content + result[contentEnd+3:]
	}
	return result
}

func trimSurroundingQuotes(text string) string {
	return strings.Trim(text, "`\"'“”‘’")
}

func shouldIncludeExplanation(line string) bool {
	lower := strings.ToLower(line)
	hedges := []string{"likely", "maybe", "probably", "possibly", "i think", "might", "could", "seems"}
	for _, hedge := range hedges {
		if strings.Contains(lower, hedge) {
			return false
		}
	}
	return true
}
