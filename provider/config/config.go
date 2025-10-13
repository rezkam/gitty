package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const configFileName = ".gritty/config.yaml"

// Settings represent the CLI configuration loaded from disk.
type Settings struct {
	Provider    string
	Completions int
	RawConfig   map[string]any
	Path        string
}

// SettingsWrite holds the parts of the configuration persisted to disk.
type SettingsWrite struct {
	Provider    string
	Completions int
	Config      any
}

// FilePath resolves the absolute path to the gritty configuration file.
func FilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user home directory: %w", err)
	}
	return filepath.Join(home, configFileName), nil
}

// Exists reports whether the configuration file is present on disk.
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// EnsureDir makes sure the configuration directory exists.
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	return nil
}

// Load reads configuration settings from the default config file location.
func Load() (*Settings, error) {
	path, err := FilePath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads configuration settings from the specified path.
func LoadFrom(path string) (*Settings, error) {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading configuration file: %w", err)
	}

	raw := v.Get("config")
	configMap, _ := raw.(map[string]any)

	settings := &Settings{
		Provider:    v.GetString("provider"),
		Completions: v.GetInt("completions"),
		RawConfig:   configMap,
		Path:        path,
	}

	return settings, nil
}

// Save persists the provided settings to the given path.
func Save(path string, data SettingsWrite) error {
	if err := EnsureDir(path); err != nil {
		return err
	}

	v := viper.New()
	v.Set("provider", data.Provider)
	v.Set("completions", data.Completions)
	v.Set("config", data.Config)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("error saving configuration: %w", err)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("error setting configuration permissions: %w", err)
	}

	return nil
}
