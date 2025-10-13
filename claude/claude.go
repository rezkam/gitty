package claude

import (
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
	providerName = "claude"

	// Defaults used only during `gritty init` to populate the user's config
	// file. At runtime the provider reads values from the config file and the
	// on-disk templates created by the init step.
	defaultEndpoint       = "https://api.anthropic.com/v1/messages"
	defaultModel          = "claude-3-haiku-20240307"
	defaultMaxTokens      = 150
	defaultTimeoutSeconds = 10
	defaultAPIVersion     = "2023-06-01"
)

// Embedded template files used during `gritty init` to create editable
// template files in the user's config directory.
//
//go:embed system_prompt.tmpl
var defaultSystemPrompt string

//go:embed commit_user.tmpl
var defaultCommitUserPrompt string

// Config holds Anthropic provider configuration.
type Config struct {
	providercore.BaseConfigValidator

	APIKey               string `mapstructure:"apiKey"`
	Endpoint             string `mapstructure:"endpoint"`
	Model                string `mapstructure:"model"`
	MaxTokens            int    `mapstructure:"maxTokens"`
	Timeout              int    `mapstructure:"timeout"`
	APIVersion           string `mapstructure:"apiVersion"`
	SystemPromptFile     string `mapstructure:"systemPromptFile"`
	SystemPrompt         string `mapstructure:"systemPrompt"`
	CommitUserPromptFile string `mapstructure:"commitUserPromptFile"`
	CommitUserPrompt     string `mapstructure:"commitUserPrompt"`
}

// Configure populates the Config with embedded defaults and prompts the user
// for any required values that lack sensible defaults.
func (c *Config) Configure() (any, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		apiKey, err := providercore.PromptRequiredString(os.Stdin, os.Stdout, "Enter your Anthropic API key: ", "API key is required. Please try again.")
		if err != nil {
			return nil, fmt.Errorf("error reading Anthropic API key: %w", err)
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
	if strings.TrimSpace(c.APIVersion) == "" {
		c.APIVersion = defaultAPIVersion
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
	if err := c.ValidateRequiredString(c.APIVersion, "apiVersion"); err != nil {
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

// Provider implements commit generation via Anthropic Claude.
type Provider struct {
	cfg             Config
	client          providercore.HTTPClient
	templateService *providercore.TemplateService
	errorParser     *providercore.ErrorParser
	systemPrompt    string
	commitUserTmpl  *template.Template
}

// NewProvider creates a new Claude provider from the configuration file.
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
		return nil, fmt.Errorf("failed to unmarshal claude config: %w", err)
	}

	// Validate configuration using shared validator
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid claude config: %w. Please run `gritty init` to create a valid config", err)
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

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeErrorResponse struct {
	Type  string             `json:"type"`
	Error claudeErrorDetails `json:"error"`
}

type claudeErrorDetails struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GenerateMessages returns commit suggestions for the supplied request.
// Claude's API always produces a single completion, regardless of the requested count.
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

	message, err := p.requestSingle(req.Diff)
	if err != nil {
		return providercore.CommitResponse{}, err
	}

	return providercore.CommitResponse{
		Suggestions: []providercore.CommitSuggestion{{Message: message}},
	}, nil
}

// SupportsMultipleCompletions reports the provider capability.
func (p *Provider) SupportsMultipleCompletions() bool {
	return false
}

func (p *Provider) requestSingle(diff string) (string, error) {
	payload := claudeRequest{
		Model:     p.cfg.Model,
		MaxTokens: p.cfg.MaxTokens,
		System:    p.systemPrompt,
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: "",
			},
		},
	}

	// execute commit-user template to produce the user message content
	content, err := p.templateService.ExecuteTemplate(p.commitUserTmpl, map[string]any{"Diff": diff, "Config": p.cfg})
	if err != nil {
		return "", &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to execute commit-user template: %w", err),
		}
	}
	payload.Messages[0].Content = content

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return "", &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to encode request payload: %w", err),
		}
	}

	req, err := http.NewRequest(http.MethodPost, p.cfg.Endpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		return "", &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to create request: %w", err),
		}
	}
	req.Header.Set("x-api-key", p.cfg.APIKey)
	req.Header.Set("anthropic-version", p.cfg.APIVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", &providercore.NetworkError{
			Provider: providerName,
			Endpoint: p.cfg.Endpoint,
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := p.errorParser.ReadErrorBody(resp)
		return "", p.handleError(resp.StatusCode, resp.Header, bodyBytes)
	}

	var decoded claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", &providercore.ValidationError{
			Provider: providerName,
			Err:      fmt.Errorf("failed to decode response payload: %w", err),
		}
	}

	for _, block := range decoded.Content {
		if strings.TrimSpace(block.Text) != "" {
			return strings.TrimSpace(block.Text), nil
		}
	}

	return "", &providercore.ValidationError{
		Provider: providerName,
		Err:      errors.New("no usable text content in response"),
	}
}

func (p *Provider) handleError(status int, headers http.Header, body []byte) error {
	// For Claude-specific error handling, first try to parse Claude error format
	message := parseClaudeErrorMessage(body)

	// If we got a Claude-specific error message, inject it before using shared parser
	if message != "" {
		// Handle Claude-specific status codes with custom messages
		switch status {
		case http.StatusInternalServerError, 529:
			return &providercore.NetworkError{
				Provider: providerName,
				Endpoint: p.cfg.Endpoint,
				Err:      fmt.Errorf("anthropic service error: %s", message),
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

func parseClaudeErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var payload claudeErrorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		if strings.TrimSpace(payload.Error.Message) != "" {
			return strings.TrimSpace(payload.Error.Message)
		}
	}

	return ""
}

func init() {
	providercore.Register(providercore.ProviderDefinition{
		Name:        providerName,
		DisplayName: "Anthropic Claude",
		Description: "Anthropic Messages API provider",
		Factory: func(configPath string) (providercore.Provider, error) {
			return NewProvider(configPath)
		},
		ConfigSetter:                &Config{},
		SupportsMultipleCompletions: false,
	})
}
