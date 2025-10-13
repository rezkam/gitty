package provider

import (
	"sort"

	"github.com/rezkam/gritty/provider/core"
	// Ensure built-in providers register themselves.
	_ "github.com/rezkam/gritty/claude"
	_ "github.com/rezkam/gritty/openai"
)

type (
	Provider     = core.Provider
	Definition   = core.ProviderDefinition
	ConfigSetter = core.ConfigSetter
	Factory      = core.Factory
)

// Register adds a provider definition to the available provider list.
func Register(def Definition) {
	core.Register(def)
}

// AvailableProviders returns the registered provider definitions sorted by name.
func AvailableProviders() []Definition {
	providers := core.Providers()
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})
	return providers
}

// GetConfigSetter retrieves the config setter registered for the provider name.
func GetConfigSetter(providerName string) (ConfigSetter, error) {
	return core.GetConfigSetter(providerName)
}

// GetFactory retrieves the factory registered for the provider name.
func GetFactory(providerName string) (Factory, error) {
	return core.GetFactory(providerName)
}
