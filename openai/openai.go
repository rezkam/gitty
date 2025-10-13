package openai

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/template"

	providercore "github.com/rezkam/gritty/provider/core"
	"github.com/spf13/viper"
)

const (
	providerName = "openai"

	// Defaults used only during `gritty init` to populate the user's config
	// file. At runtime the provider reads values from the config file and the
	// on-disk templates created by the init step.
	defaultEndpoint       = "https://api.openai.com/v1/chat/completions"
	defaultModel          = "gpt-4o-mini"
	defaultMaxTokens      = 150
	defaultTimeoutSeconds = 10
)

// Embedded template files used during `gritty init` to create editable
// template files in the user's config directory.
//
//go:embed system_prompt.tmpl
var defaultSystemPrompt string

//go:embed commit_user.tmpl
var defaultCommitUserPrompt string

// Config holds OpenAI provider configuration.
type Config struct {
	providercore.BaseConfigValidator

	APIKey               string `mapstructure:"apiKey"`
	Endpoint             string `mapstructure:"endpoint"`
	Model                string `mapstructure:"model"`
	MaxTokens            int    `mapstructure:"maxTokens"`
	Timeout              int    `mapstructure:"timeout"`
	SystemPromptFile     string `mapstructure:"systemPromptFile"`
	SystemPrompt         string `mapstructure:"systemPrompt"`
	CommitUserPromptFile string `mapstructure:"commitUserPromptFile"`
	CommitUserPrompt     string `mapstructure:"commitUserPrompt"`
}

// Configure populates a Config with defaults and prompts the user for any
// required values that cannot fall back to sensible defaults.
func (c *Config) Configure() (any, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		apiKey, err := providercore.PromptRequiredString(os.Stdin, os.Stdout, "Enter your OpenAI API key: ", "API key is required. Please try again.")
		if err != nil {
			return nil, fmt.Errorf("error reading OpenAI API key: %w", err)
		}
		c.APIKey = apiKey
	}

	// Populate config struct with sensible default values used only during
	// the init flow. These defaults will be written to disk by the CLI so
	// runtime NewProvider expects a complete config file.
	if strings.TrimSpace(c.Endpoint) == "" {
		c.Endpoint = defaultEndpoint
	}
	if strings.TrimSpace(c.Model) == "" {
		c.Model = defaultModel
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = defaultMaxTokens
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeoutSeconds
	}

	// We intentionally do not set SystemPromptFile or CommitUserPromptFile
	// here because those paths depend on the user's config directory and are
	// created by ProvisionFiles during init.
	return c, nil
}

// ProvisionFiles writes default prompt templates to the given config
// directory (under a provider-specific subdirectory). These files are
// editable by the user and the written config points at their paths.
func (c *Config) ProvisionFiles(configDir string) (map[string]string, error) {
	provisioner := providercore.NewFileProvisioner(providerName)

	templates := map[string]string{
		"system_prompt.tmpl": defaultSystemPrompt,
		"commit_user.tmpl":   defaultCommitUserPrompt,
	}

	paths, err := provisioner.ProvisionTemplates(configDir, templates)
	if err != nil {
		return nil, err
	}

	// Point the in-memory config at the provisioned files.
	c.SystemPromptFile = paths["system_prompt.tmpl"]
	c.CommitUserPromptFile = paths["commit_user.tmpl"]

	return map[string]string{
		"systemPromptFile":     c.SystemPromptFile,
		"commitUserPromptFile": c.CommitUserPromptFile,
	}, nil
}

// Validate checks if the configuration has all required fields.
func (c *Config) Validate() error {
	if err := c.ValidateRequiredString(c.APIKey, "apiKey"); err != nil {
		return err
	}
	if err := c.ValidateRequiredString(c.Endpoint, "endpoint"); err != nil {
		return err
	}
	if err := c.ValidateRequiredString(c.Model, "model"); err != nil {
		return err
	}
	if err := c.ValidateRequiredInt(c.MaxTokens, "maxTokens"); err != nil {
		return err
	}
	if err := c.ValidateRequiredInt(c.Timeout, "timeout"); err != nil {
		return err
	}
	if err := c.ValidateTemplateConfig(c.SystemPrompt, c.SystemPromptFile, "systemPrompt"); err != nil {
		return err
	}
	if err := c.ValidateTemplateConfig(c.CommitUserPrompt, c.CommitUserPromptFile, "commitUserPrompt"); err != nil {
		return err
	}
	return nil
}

// Provider implements commit generation via OpenAI Chat Completions.
type Provider struct {
	cfg             Config
	client          providercore.HTTPClient
	templateService *providercore.TemplateService
	errorParser     *providercore.ErrorParser
	systemPrompt    string
	commitUserTmpl  *template.Template
}

// NewProvider creates a new OpenAI provider from the configuration file.
func NewProvider(configPath string) (*Provider, error) {
	if strings.TrimSpace(configPath) == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

	// Load user config
	loader := viper.New()
	loader.SetConfigFile(configPath)
	if err := loader.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := loader.UnmarshalKey("config", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openai config: %w", err)
	}

	// Validate configuration using shared validator
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid openai config: %w. Please run `gritty init` to create a valid config", err)
	}

	// Create provider with shared services
	clientFactory := providercore.NewHTTPClientFactory()
	templateService := providercore.NewTemplateService()
	errorParser := providercore.NewErrorParser(providerName)

	p := &Provider{
		cfg:             cfg,
		client:          clientFactory.CreateClient(cfg.Timeout),
		templateService: templateService,
		errorParser:     errorParser,
	}

	// Load and parse system prompt using template service
	sysContent, err := templateService.LoadTemplate(cfg.SystemPrompt, cfg.SystemPromptFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %w", err)
	}

	p.systemPrompt, err = templateService.ParseAndExecute("system", sysContent, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to process system prompt: %w", err)
	}

	// Load and parse commit-user template
	commitContent, err := templateService.LoadTemplate(cfg.CommitUserPrompt, cfg.CommitUserPromptFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load commit-user prompt: %w", err)
	}

	p.commitUserTmpl, err = templateService.ParseTemplate("commit_user", commitContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commit-user template: %w", err)
	}

	return p, nil
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

// GenerateMessages returns commit message suggestions for the supplied request.
func (p *Provider) GenerateMessages(req providercore.CommitRequest) (providercore.CommitResponse, error) {
	req.Normalize()
	if req.Count <= 0 {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      errors.New("requested completion count must be greater than zero"),
		}
	}

	if req.Diff == "" {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      errors.New("diff cannot be empty"),
		}
	}

	userContent, err := p.templateService.ExecuteTemplate(p.commitUserTmpl, map[string]any{"Diff": req.Diff, "Config": p.cfg})
	if err != nil {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to execute commit-user template: %w", err),
		}
	}

	reqPayload := map[string]interface{}{
		"model": p.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": p.systemPrompt},
			{"role": "user", "content": userContent},
		},
		"max_tokens": p.cfg.MaxTokens,
		"n":          req.Count,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(reqPayload); err != nil {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to encode request payload: %w", err),
		}
	}

	httpReq, err := http.NewRequest(http.MethodPost, p.cfg.Endpoint, &body)
	if err != nil {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to create request: %w", err),
		}
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.cfg.APIKey))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return providercore.CommitResponse{}, &providercore.NetworkError{
			Provider: providerName,
			Endpoint: p.cfg.Endpoint,
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := p.errorParser.ReadErrorBody(resp)
		return providercore.CommitResponse{}, p.handleError(resp.StatusCode, resp.Header, bodyBytes)
	}

	var respPayload response
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to decode response payload: %w", err),
		}
	}

	if len(respPayload.Choices) == 0 {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      errors.New("missing 'choices' field"),
		}
	}

	seen := make(map[string]struct{})
	suggestions := make([]providercore.CommitSuggestion, 0, req.Count)
	for _, choice := range respPayload.Choices {
		content := strings.TrimSpace(choice.Message.Content)
		if content == "" {
			continue
		}
		if _, exists := seen[content]; exists {
			continue
		}
		seen[content] = struct{}{}
		suggestions = append(suggestions, providercore.CommitSuggestion{Message: content})
		if len(suggestions) == req.Count {
			break
		}
	}

	if len(suggestions) == 0 {
		return providercore.CommitResponse{}, &providercore.ValidationError{
			Provider: providerName,
			Err:      errors.New("no valid choices in response"),
		}
	}

	return providercore.CommitResponse{Suggestions: suggestions}, nil
}

// SupportsMultipleCompletions reports whether the OpenAI provider can return
// multiple commit messages in a single API call.
func (p *Provider) SupportsMultipleCompletions() bool {
	return true
}

func (p *Provider) handleError(status int, headers http.Header, body []byte) error {
	// For OpenAI-specific error handling, first try to parse OpenAI error format
	message := parseOpenAIErrorMessage(body)

	// If we got an OpenAI-specific error message, inject it before using shared parser
	if message != "" {
		// Handle OpenAI-specific status codes with custom messages
		switch status {
		case http.StatusInternalServerError, http.StatusServiceUnavailable:
			return &providercore.NetworkError{
				Provider: providerName,
				Endpoint: p.cfg.Endpoint,
				Err:      fmt.Errorf("openai service error: %s", message),
			}
		}
	}

	// Use shared error parser for standard HTTP errors
	err := p.errorParser.ParseHTTPError(status, headers, body)

	// If it's a NetworkError, set the endpoint
	if netErr, ok := err.(*providercore.NetworkError); ok {
		netErr.Endpoint = p.cfg.Endpoint
	}

	return err
}

func parseOpenAIErrorMessage(body []byte) string {
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

func init() {
	providercore.Register(providercore.ProviderDefinition{
		Name:        providerName,
		DisplayName: "OpenAI Chat Completions",
		Description: "Default GPT-based provider using OpenAI's API",
		Factory: func(configPath string) (providercore.Provider, error) {
			return NewProvider(configPath)
		},
		ConfigSetter:                &Config{},
		SupportsMultipleCompletions: true,
	})
}
