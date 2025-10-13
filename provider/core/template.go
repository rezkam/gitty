package core

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
)

// TemplateService handles loading, parsing, and executing Go templates for
// commit message generation. This service eliminates duplication across
// provider implementations.
type TemplateService struct{}

// NewTemplateService creates a new template service.
func NewTemplateService() *TemplateService {
	return &TemplateService{}
}

// LoadTemplate loads template content from either an inline string or a file
// path. It returns the raw template content or an error if loading fails.
func (s *TemplateService) LoadTemplate(inlineContent, filePath string) (string, error) {
	// Prefer inline content if provided
	if strings.TrimSpace(inlineContent) != "" {
		return inlineContent, nil
	}

	// Fall back to file path
	if strings.TrimSpace(filePath) != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read template file '%s': %w", filePath, err)
		}
		return string(b), nil
	}

	return "", fmt.Errorf("no template content or file path provided")
}

// ParseAndExecute parses a template string and executes it with the given data.
// Returns the rendered template content or an error.
func (s *TemplateService) ParseAndExecute(name, content string, data any) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("template content is empty")
	}

	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}

// ParseTemplate parses a template and returns it for later execution.
// This is useful when you need to execute the same template multiple times
// with different data.
func (s *TemplateService) ParseTemplate(name, content string) (*template.Template, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("template content is empty")
	}

	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return tmpl, nil
}

// ExecuteTemplate executes a pre-parsed template with the given data.
func (s *TemplateService) ExecuteTemplate(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}
