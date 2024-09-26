package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/term"
)

const (
	model     = "gpt-4o-mini"
	url       = "https://api.openai.com/v1/chat/completions"
	maxTokens = 150
)

type Config struct {
	APIKey string `mapstructure:"apiKey"`
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
	cfg Config
}

// NewProvider creates a new OpenAI provider using the configuration file
func NewProvider(filepath string) (*Provider, error) {
	if filepath == "" {
		return nil, fmt.Errorf("filepath cannot be empty")
	}

	// Set the configuration file path and read the config
	viper.SetConfigFile(filepath)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	// Unmarshal the nested 'config' key into the cfg struct
	if err := viper.UnmarshalKey("config", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Check if the API key is provided
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is missing in the config file")
	}

	return &Provider{cfg: cfg}, nil
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

func (p *Provider) GetCommitMessage(diff string) (string, error) {
	reqPayload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are an assistant that helps in writing concise and clear Git commit messages."},
			{"role": "user", "content": fmt.Sprintf("Based on the following git diff, suggest a concise and clear git commit message:\n\n%s", diff)},
		},
		"max_tokens": maxTokens,
	}

	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode request payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api request failed with status code: %d", resp.StatusCode)
	}

	var respPayload response
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	if err != nil {
		return "", fmt.Errorf("failed to decode response payload: %w", err)
	}

	// Extract the commit message from the response
	if len(respPayload.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	commitMessage := respPayload.Choices[0].Message.Content

	return commitMessage, nil
}
