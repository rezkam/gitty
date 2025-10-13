package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rezkam/gritty/provider"
	providerconfig "github.com/rezkam/gritty/provider/config"
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
	configPath, err := providerconfig.FilePath()
	if err != nil {
		return err
	}
	configDir := filepath.Dir(configPath)

	if err := providerconfig.EnsureDir(configPath); err != nil {
		return err
	}

	availableProviders := provider.AvailableProviders()

	// Display the available providers
	fmt.Println("Select a commit message provider:")
	for i, p := range availableProviders {
		fmt.Printf("%d: %s\n", i+1, p.Name)
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
	if err != nil || selection < 1 || selection > len(availableProviders) {
		return fmt.Errorf("invalid selection, please enter a number between 1 and %d", len(availableProviders))
	}

	// Get the selected provider
	selectedProvider := availableProviders[selection-1]

	completionCount := 1
	if selectedProvider.SupportsMultipleCompletions {
		completionCount, err = promptCompletionCount(reader)
		if err != nil {
			return err
		}
	}

	// Get the selected provider's ConfigSetter
	selectedProviderConfigSetter, err := provider.GetConfigSetter(selectedProvider.Name)
	if err != nil {
		return fmt.Errorf("error getting provider config setter: %w", err)
	}

	cfg, err := selectedProviderConfigSetter.Configure()
	if err != nil {
		return fmt.Errorf("error prompting for provider config: %w", err)
	}

	// If the provider's config object knows how to provision on-disk template
	// files, ask it to write those into the config directory so the generated
	// config.yaml references editable template files.
	type provisioner interface {
		ProvisionFiles(configDir string) (map[string]string, error)
	}
	if p, ok := cfg.(provisioner); ok {
		if _, err := p.ProvisionFiles(configDir); err != nil {
			return fmt.Errorf("error provisioning provider files: %w", err)
		}
	}

	if err := providerconfig.Save(configPath, providerconfig.SettingsWrite{
		Provider:    selectedProvider.Name,
		Completions: completionCount,
		Config:      cfg,
	}); err != nil {
		return err
	}

	fmt.Println("Provider configuration initialized successfully")
	return nil
}

func promptCompletionCount(reader *bufio.Reader) (int, error) {
	fmt.Print("Enter desired number of suggestions per commit [1-5] (default 1): ")
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("error reading completion count: %w", err)
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return 1, nil
	}

	count, err := strconv.Atoi(input)
	if err != nil || count < 1 || count > 5 {
		return 0, fmt.Errorf("invalid completion count: enter a number between 1 and 5")
	}
	return count, nil
}
