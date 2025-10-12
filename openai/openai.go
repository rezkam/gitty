package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	providercore "github.com/rezkam/gritty/provider/core"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

const (
	defaultModel          = "gpt-4o-mini"
	defaultEndpoint       = "https://api.openai.com/v1/chat/completions"
	defaultMaxTokens      = 150
	defaultTimeoutSeconds = 10
	providerName          = "openai"
)

type Config struct {
	APIKey    string `mapstructure:"apiKey"`
	Endpoint  string `mapstructure:"endpoint"`
	Model     string `mapstructure:"model"`
	MaxTokens int    `mapstructure:"maxTokens"`
	Timeout   int    `mapstructure:"timeout"`
}

func (c Config) Configure() (any, error) {
	fmt.Print("Enter your OpenAI API Key: ")

	// Read password input from the terminal without echoing
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("error reading API Key: %w", err)
	}

	// Convert the byte slice to a string and trim whitespace
	apiKey := strings.TrimSpace(string(bytePassword))

	// Print a newline to move the cursor to the next line
	fmt.Println()

	if len(apiKey) == 0 {
		return nil, fmt.Errorf("API Key cannot be empty")
	}

	c.APIKey = apiKey
	return c, nil
}

type Provider struct {
	cfg    Config
	client *http.Client
}

// NewProvider creates a new OpenAI provider using the configuration file
func NewProvider(filepath string) (*Provider, error) {
	if filepath == "" {
		return nil, fmt.Errorf("filepath cannot be empty")
	}

	configLoader := viper.New()
	configLoader.SetConfigFile(filepath)

	if err := configLoader.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	// Unmarshal the nested 'config' key into the cfg struct
	if err := configLoader.UnmarshalKey("config", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Check if the API key is provided
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is missing in the config file")
	}

	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultEndpoint
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = defaultMaxTokens
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeoutSeconds
	}

	return &Provider{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}, nil
}

type response struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message message `json:"message"`
}

type message struct {
	Content string `json:"content"`
}

func (p *Provider) GetCommitMessages(diff string, n int) ([]string, error) {
	if n <= 0 {
		return nil, &providercore.ValidationError{
			Provider: providerName,
			Err:      errors.New("requested completion count must be greater than zero"),
		}
	}

	reqPayload := map[string]interface{}{
		"model": p.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are an assistant that helps in writing concise and clear Git commit messages."},
			{"role": "user", "content": fmt.Sprintf("Based on the following git diff, suggest a concise and clear git commit message:\n\n%s", diff)},
		},
		"max_tokens": p.cfg.MaxTokens,
		"n":          n,
	}

	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, p.cfg.Endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &providercore.NetworkError{
			Provider: providerName,
			Endpoint: p.cfg.Endpoint,
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleHTTPError(resp)
	}

	var respPayload response
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	if err != nil {
		return nil, &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to decode response payload: %w", err),
		}
	}

	if len(respPayload.Choices) == 0 {
		return nil, &providercore.ValidationError{
			Provider: providerName,
			Err:      errors.New("missing 'choices' field"),
		}
	}

	var commitMessages []string
	seen := make(map[string]struct{})
	for _, choice := range respPayload.Choices {
		content := strings.TrimSpace(choice.Message.Content)
		if content == "" {
			continue
		}
		if _, exists := seen[content]; exists {
			continue
		}
		seen[content] = struct{}{}
		commitMessages = append(commitMessages, content)
		if len(commitMessages) == n {
			break
		}
	}

	if len(commitMessages) == 0 {
		return nil, &providercore.ValidationError{
			Provider: providerName,
			Err:      errors.New("no valid choices in response"),
		}
	}

	return commitMessages, nil
}

// SupportsMultipleCompletions reports whether the OpenAI provider can return
// multiple commit messages in a single API call.
func (p *Provider) SupportsMultipleCompletions() bool {
	return true
}

func (p *Provider) handleHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	message := extractErrorMessage(body)

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return &providercore.AuthenticationError{
			Provider: providerName,
			Err:      errors.New(message),
		}
	case http.StatusTooManyRequests:
		return &providercore.RateLimitError{
			Provider:   providerName,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	default:
		statusText := http.StatusText(resp.StatusCode)
		if statusText == "" {
			statusText = fmt.Sprintf("status %d", resp.StatusCode)
		}
		if message != "" {
			return fmt.Errorf("provider '%s' request failed: %s (%s)", providerName, statusText, message)
		}
		return fmt.Errorf("provider '%s' request failed: %s", providerName, statusText)
	}
}

func extractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var payload struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		switch v := payload.Error.(type) {
		case string:
			return strings.TrimSpace(v)
		case map[string]any:
			if msg, ok := v["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
			if code, ok := v["code"].(string); ok && strings.TrimSpace(code) != "" {
				return strings.TrimSpace(code)
			}
		}
	}

	return strings.TrimSpace(string(body))
}

func parseRetryAfter(value string) int {
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if seconds < 0 {
			return 0
		}
		return seconds
	}

	// According to RFC 7231, Retry-After can be an HTTP-date.
	if t, err := http.ParseTime(value); err == nil {
		seconds := int(time.Until(t).Seconds())
		if seconds < 0 {
			return 0
		}
		return seconds
	}

	return 0
}

func init() {
	providercore.Register(providercore.ProviderDefinition{
		Name:        providerName,
		DisplayName: "OpenAI Chat Completions",
		Description: "Default GPT-based provider using OpenAI's API",
		Factory: func(configPath string) (providercore.Provider, error) {
			return NewProvider(configPath)
		},
		ConfigSetter: &Config{},
	})
}
