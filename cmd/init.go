package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rezkam/gritty/provider"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the configuration for the commit message provider",
	RunE:  runInitCmd,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInitCmd(cmd *cobra.Command, args []string) error {
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)

	viper.SetConfigFile(configPath)

	// create the config directory if it doesn't exist
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("error creating config directory: %w", err)
		}
	}

	availableProviders := provider.AvailableProviders

	providerNames := make([]string, 0, len(availableProviders))
	for _, p := range availableProviders {
		providerNames = append(providerNames, p.Name)
	}

	// Display the available providers
	fmt.Println("Select a commit message provider:")
	for i, name := range providerNames {
		fmt.Printf("%d: %s\n", i+1, name)
	}

	// Prompt user to select a provider
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the number corresponding to your choice: ")

	selectionStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	// Remove any newline or extra spaces from the input
	selectionStr = strings.TrimSpace(selectionStr)

	// Convert the selection to an integer
	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(providerNames) {
		return fmt.Errorf("invalid selection, please enter a number between 1 and %d", len(providerNames))
	}

	// Get the selected provider
	selectedProvider := providerNames[selection-1]

	// Get the selected provider's ConfigSetter
	selectedProviderConfigSetter, err := provider.GetConfigSetter(selectedProvider)
	if err != nil {
		return fmt.Errorf("error getting provider config setter: %w", err)
	}

	cfg, err := selectedProviderConfigSetter.Configure()
	if err != nil {
		return fmt.Errorf("error prompting for provider config: %w", err)
	}

	// save the configs
	viper.Set("provider", selectedProvider)

	// Save the entire configuration using reflection
	viper.Set("config", cfg)

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("error saving configuration: %w", err)
	}

	// Change the file permissions to 0600 after writing the config
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("error setting file permissions: %w", err)
	}

	fmt.Println("Provider configuration initialized successfully")
	return nil
}

// structToMap converts a struct to a map[string]interface{} using reflection.
func structToMap(input interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	data, err := json.Marshal(input) // Convert struct to JSON
	if err != nil {
		return nil, fmt.Errorf("error marshalling struct: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil { // Convert JSON to map
		return nil, fmt.Errorf("error unmarshalling struct to map: %w", err)
	}
	return result, nil
}
