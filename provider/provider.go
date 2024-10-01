package provider

import (
	"fmt"

	"github.com/rezkam/gritty/openai"
)

// Provider is the interface for commit message providers.
type Provider interface {
	GetCommitMessages(diff string, n int) ([]string, error)
}

// ProviderDefinition holds information about a provider.
type ProviderDefinition struct {
	Name         string
	Factory      Factory
	ConfigSetter ConfigSetter
}

var AvailableProviders = []ProviderDefinition{
	{
		Name: "openai",
		Factory: func(configPath string) (Provider, error) {
			return openai.NewProvider(configPath)
		},
		ConfigSetter: &openai.Config{},
	},
}

// ConfigSetter represents a type that can prompt for configuration.
type ConfigSetter interface {
	Configure() (any, error)
}

func GetConfigSetter(providerName string) (ConfigSetter, error) {
	for _, provider := range AvailableProviders {
		if provider.Name == providerName {
			return provider.ConfigSetter, nil
		}
	}
	return nil, fmt.Errorf("provider '%s' not found", providerName)
}

type Factory func(configPath string) (Provider, error)

func GetFactory(providerName string) (Factory, error) {
	for _, provider := range AvailableProviders {
		if provider.Name == providerName {
			return provider.Factory, nil
		}
	}
	return nil, fmt.Errorf("provider '%s' not found", providerName)
}
