package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ProviderConfig defines the interface for provider-specific configuration.
// This follows the Interface Segregation Principle (ISP) by defining minimal
// required behavior.
type ProviderConfig interface {
	// Validate checks if the configuration has all required fields.
	Validate() error
}

// TemplateProvisioner defines the interface for providers that need to write
// template files during initialization.
type TemplateProvisioner interface {
	// ProvisionFiles writes provider template files to the config directory.
	ProvisionFiles(configDir string) (map[string]string, error)
}

// BaseConfigValidator provides common configuration validation logic
// following DRY (Don't Repeat Yourself) principle.
type BaseConfigValidator struct{}

// ValidateRequiredString checks if a string field is non-empty.
func (v *BaseConfigValidator) ValidateRequiredString(value, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

// ValidateRequiredInt checks if an integer field is positive.
func (v *BaseConfigValidator) ValidateRequiredInt(value int, fieldName string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be greater than zero", fieldName)
	}
	return nil
}

// ValidateTemplateConfig ensures at least one template source is provided.
func (v *BaseConfigValidator) ValidateTemplateConfig(inlineContent, filePath, fieldName string) error {
	if inlineContent == "" && filePath == "" {
		return fmt.Errorf("%s is required (provide either inline content or file path)", fieldName)
	}
	return nil
}

// FileProvisioner provides common file provisioning functionality.
type FileProvisioner struct {
	ProviderName string
}

// NewFileProvisioner creates a new file provisioner for a provider.
func NewFileProvisioner(providerName string) *FileProvisioner {
	return &FileProvisioner{
		ProviderName: providerName,
	}
}

// WriteTemplateFile writes a template file to the provider's config directory.
// It creates the directory if needed and preserves existing files.
func (p *FileProvisioner) WriteTemplateFile(configDir, filename, content string) (string, error) {
	providerDir := filepath.Join(configDir, p.ProviderName)
	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create provider config dir: %w", err)
	}

	filePath := filepath.Join(providerDir, filename)

	// Preserve existing files to avoid overwriting user edits
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write template file '%s': %w", filename, err)
		}
	}

	return filePath, nil
}

// ProvisionTemplates writes multiple template files and returns their paths.
func (p *FileProvisioner) ProvisionTemplates(configDir string, templates map[string]string) (map[string]string, error) {
	paths := make(map[string]string, len(templates))

	for filename, content := range templates {
		path, err := p.WriteTemplateFile(configDir, filename, content)
		if err != nil {
			return nil, err
		}
		paths[filename] = path
	}

	return paths, nil
}
