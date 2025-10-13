package openai_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rezkam/gritty/openai"
	"github.com/rezkam/gritty/provider"
	providercore "github.com/rezkam/gritty/provider/core"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerateMessagesSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, float64(150), payload["max_tokens"])
		require.Equal(t, float64(2), payload["n"])

		response := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Add multi-provider support"}},
				{"message": map[string]string{"content": "Add multi-provider support"}},
				{"message": map[string]string{"content": "Add multi-provider support"}},
				{"message": map[string]string{"content": "Integrate Claude provider"}},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	p := newProvider(t, openai.Config{
		APIKey:    "test-key",
		Endpoint:  server.URL,
		Model:     "gpt-4o-mini",
		MaxTokens: 150,
		Timeout:   1,
	})

	resp, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 2})
	require.NoError(t, err)
	require.Equal(t, []string{
		"Add multi-provider support",
		"Integrate Claude provider",
	}, resp.Messages())
	require.True(t, p.SupportsMultipleCompletions())
}

func TestGenerateMessagesAuthenticationError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_api_key",
		})
	}))
	defer server.Close()

	p := newProvider(t, openai.Config{
		APIKey:    "bad-key",
		Endpoint:  server.URL,
		Model:     "gpt-4o-mini",
		MaxTokens: 150,
		Timeout:   1,
	})

	_, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 1})
	var authErr *provider.AuthenticationError
	require.ErrorAs(t, err, &authErr)
	require.Equal(t, "openai", authErr.Provider)
}

func TestGenerateMessagesRateLimitError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	p := newProvider(t, openai.Config{
		APIKey:    "test-key",
		Endpoint:  server.URL,
		Model:     "gpt-4o-mini",
		MaxTokens: 150,
		Timeout:   1,
	})

	_, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 1})
	var rateErr *provider.RateLimitError
	require.ErrorAs(t, err, &rateErr)
	require.Equal(t, 60, rateErr.RetryAfter)
}

func TestGenerateMessagesValidationError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"unexpected":"schema"}`))
	}))
	defer server.Close()

	p := newProvider(t, openai.Config{
		APIKey:    "test-key",
		Endpoint:  server.URL,
		Model:     "gpt-4o-mini",
		MaxTokens: 150,
		Timeout:   1,
	})

	_, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 1})
	var validationErr *provider.ValidationError
	require.ErrorAs(t, err, &validationErr)
}

func TestGenerateMessagesTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Add multi-provider support"}},
			},
		})
	}))
	defer server.Close()

	p := newProvider(t, openai.Config{
		APIKey:    "test-key",
		Endpoint:  server.URL,
		Model:     "gpt-4o-mini",
		MaxTokens: 150,
		Timeout:   1,
	})

	_, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 1})
	var networkErr *provider.NetworkError
	require.ErrorAs(t, err, &networkErr)
	require.True(t, errors.Is(err, networkErr.Err))
}

func TestGenerateMessagesReturnsFirstNChoices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "1"}},
				{"message": map[string]string{"content": "1"}},
				{"message": map[string]string{"content": "2"}},
				{"message": map[string]string{"content": "3"}},
			},
		})
	}))
	defer server.Close()

	p := newProvider(t, openai.Config{
		APIKey:    "test-key",
		Endpoint:  server.URL,
		Model:     "gpt-4o-mini",
		MaxTokens: 150,
		Timeout:   1,
	})

	resp, err := p.GenerateMessages(providercore.CommitRequest{Diff: "diff", Count: 2})
	require.NoError(t, err)

	require.Equal(t, []string{
		"1",
		"2",
	}, resp.Messages())
	require.True(t, p.SupportsMultipleCompletions())
}

func newProvider(t *testing.T, cfg openai.Config) *openai.Provider {
	t.Helper()

	// Set default prompts inline for testing (simpler than creating files)
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = "You are an OpenAI assistant that writes Git commit summaries. Output the commit message on the first line in imperative mood, no longer than 72 characters."
	}
	if cfg.CommitUserPrompt == "" {
		cfg.CommitUserPrompt = "Generate a Git commit summary for the staged changes.\n\nDiff:\n{{ .Diff }}"
	}

	payload := struct {
		Provider string        `yaml:"provider"`
		Config   openai.Config `yaml:"config"`
	}{
		Provider: "openai",
		Config:   cfg,
	}

	data, err := yaml.Marshal(payload)
	require.NoError(t, err)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, data, 0600))

	p, err := openai.NewProvider(configPath)
	require.NoError(t, err)
	return p
}
