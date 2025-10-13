package claude_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rezkam/gritty/claude"
	"github.com/rezkam/gritty/provider"
	providercore "github.com/rezkam/gritty/provider/core"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerateMessagesSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("content-type"))
		require.Equal(t, "test-key", r.Header.Get("x-api-key"))
		require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "test-model", payload["model"])
		require.Equal(t, float64(120), payload["max_tokens"])
		require.Equal(t, "You are an Anthropic assistant that writes Git commit summaries. Output the commit message on the first line in imperative mood, no longer than 72 characters. After a blank line you may include a brief explanation of what changed and why only when you are certain it is correct. Never use hedging language such as maybe, likely, probably, possibly, might, could, or seems. Do not include reasoning steps, <thinking> traces, quotes, code fences, or extra formatting.", payload["system"])

		response := map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": "Integrate Claude provider"},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	p := newProvider(t, claude.Config{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		Model:      "test-model",
		MaxTokens:  120,
		Timeout:    2,
		APIVersion: "2023-06-01",
	})

	resp, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 1})
	require.NoError(t, err)
	require.Equal(t, []string{"Integrate Claude provider"}, resp.Messages())
	require.False(t, p.SupportsMultipleCompletions())
}

func TestGenerateMessagesSequentialRequests(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": "Commit message"},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	p := newProvider(t, claude.Config{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		Model:      "test-model",
		MaxTokens:  120,
		Timeout:    2,
		APIVersion: "2023-06-01",
	})

	// Claude doesn't support multiple completions, so even when requesting 3,
	// we should only get 1 message and make 1 API call
	resp, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 3})
	require.NoError(t, err)
	require.Len(t, resp.Messages(), 1, "Claude should return only 1 message regardless of count")
	require.Equal(t, 1, callCount, "Claude should make only 1 API call")
}

func TestGenerateMessagesAuthenticationError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		payload := map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	p := newProvider(t, claude.Config{
		APIKey:     "bad-key",
		Endpoint:   server.URL,
		APIVersion: "2023-06-01",
	})

	_, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 1})
	var authErr *provider.AuthenticationError
	require.ErrorAs(t, err, &authErr)
	require.Equal(t, "claude", authErr.Provider)
}

func TestGenerateMessagesRateLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`))
	}))
	defer server.Close()

	p := newProvider(t, claude.Config{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		APIVersion: "2023-06-01",
	})

	_, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 1})
	var rateErr *provider.RateLimitError
	require.ErrorAs(t, err, &rateErr)
	require.Equal(t, 42, rateErr.RetryAfter)
}

func newProvider(t *testing.T, cfg claude.Config) *claude.Provider {
	t.Helper()

	// Simulate the init flow: populate defaults and provision template files
	cfgPtr := &cfg
	_, err := cfgPtr.Configure()
	require.NoError(t, err)

	dir := t.TempDir()
	// Provision embedded template files into the test config dir so the
	// written config references editable files (matching runtime behavior).
	_, err = cfgPtr.ProvisionFiles(dir)
	require.NoError(t, err)

	payload := struct {
		Provider string        `yaml:"provider"`
		Config   claude.Config `yaml:"config"`
	}{
		Provider: "claude",
		Config:   cfg,
	}

	data, err := yaml.Marshal(payload)
	require.NoError(t, err)

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, data, 0600))

	p, err := claude.NewProvider(configPath)
	require.NoError(t, err)
	return p
}
