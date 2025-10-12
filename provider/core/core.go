package core

import (
	"fmt"
	"sync"
)

// Provider is the interface for commit message providers.
type Provider interface {
	GetCommitMessages(diff string, n int) ([]string, error)
	SupportsMultipleCompletions() bool
}

// ProviderDefinition holds metadata and constructors for a provider.
type ProviderDefinition struct {
	Name         string
	DisplayName  string
	Description  string
	Factory      Factory
	ConfigSetter ConfigSetter
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
