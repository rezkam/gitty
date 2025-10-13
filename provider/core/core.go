package core

import (
	"fmt"
	"strings"
	"sync"
)

// CommitRequest captures the input required to generate commit message suggestions.
type CommitRequest struct {
	Diff  string
	Count int
}

// Normalize prepares the request for provider consumption, ensuring sensible defaults.
func (r *CommitRequest) Normalize() {
	if r.Count < 1 {
		r.Count = 1
	}
	r.Diff = strings.TrimSpace(r.Diff)
}

// CommitSuggestion represents a single provider response with optional metadata.
type CommitSuggestion struct {
	Message  string
	Metadata map[string]any
}

// CommitResponse aggregates commit suggestions returned by a provider.
type CommitResponse struct {
	Suggestions []CommitSuggestion
}

// Messages extracts the plain commit message strings from the response.
func (r CommitResponse) Messages() []string {
	if len(r.Suggestions) == 0 {
		return nil
	}
	result := make([]string, 0, len(r.Suggestions))
	for _, suggestion := range r.Suggestions {
		text := strings.TrimSpace(suggestion.Message)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

// Provider is the interface for commit message providers.
type Provider interface {
	GenerateMessages(req CommitRequest) (CommitResponse, error)
	SupportsMultipleCompletions() bool
}

// ProviderDefinition holds metadata and constructors for a provider.
type ProviderDefinition struct {
	Name                        string
	DisplayName                 string
	Description                 string
	Factory                     Factory
	ConfigSetter                ConfigSetter
	SupportsMultipleCompletions bool
}

// ConfigSetter represents a type that can prompt for configuration.
type ConfigSetter interface {
	Configure() (any, error)
}

// Factory creates a provider instance for the given config path.
type Factory func(configPath string) (Provider, error)

var (
	providersMu sync.RWMutex
	providers   []ProviderDefinition
)

// Register adds a provider definition to the available provider list.
func Register(def ProviderDefinition) {
	providersMu.Lock()
	defer providersMu.Unlock()

	for _, existing := range providers {
		if existing.Name == def.Name {
			return
		}
	}
	providers = append(providers, def)
}

// Providers returns a snapshot of the currently registered providers.
func Providers() []ProviderDefinition {
	providersMu.RLock()
	defer providersMu.RUnlock()

	snapshot := make([]ProviderDefinition, len(providers))
	copy(snapshot, providers)
	return snapshot
}

// GetConfigSetter retrieves the config setter registered for the provider name.
func GetConfigSetter(providerName string) (ConfigSetter, error) {
	providersMu.RLock()
	defer providersMu.RUnlock()

	for _, provider := range providers {
		if provider.Name == providerName {
			return provider.ConfigSetter, nil
		}
	}
	return nil, fmt.Errorf("provider '%s' not found", providerName)
}

// GetFactory retrieves the factory registered for the provider name.
func GetFactory(providerName string) (Factory, error) {
	providersMu.RLock()
	defer providersMu.RUnlock()

	for _, provider := range providers {
		if provider.Name == providerName {
			return provider.Factory, nil
		}
	}
	return nil, fmt.Errorf("provider '%s' not found", providerName)
}
