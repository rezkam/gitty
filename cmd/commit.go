package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rezkam/gritty/git"
	"github.com/rezkam/gritty/provider"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Generate and apply a Git commit message based on staged changes",
	RunE:  runCommitCmd,
}

func init() {
	rootCmd.AddCommand(commitCmd)
}

func runCommitCmd(cmd *cobra.Command, args []string) error {
	configPath := getConfigPath()

	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading configuration file: %w", err)
	}

	providerName := viper.GetString("provider")
	if providerName == "" {
		return fmt.Errorf("provider not set in configuration file")
	}
	factory, err := provider.GetFactory(providerName)
	if err != nil {
		return fmt.Errorf("error getting provider factory: %w", err)
	}
	p, err := factory(configPath)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}
	// Check if the current directory is a Git repository
	if !git.IsGitDirectory(".") {
		return fmt.Errorf("current directory is not a Git repository")
	}

	// Get the staged diff
	diff, err := git.GetStagedDiff(".")
	if err != nil {
		return fmt.Errorf("error getting staged diff: %w", err)
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes to commit")
		return nil
	}

	// Use the commit message provider to get commit messages
	commitMessages, err := p.GetCommitMessages(diff, 3)
	if err != nil {
		return fmt.Errorf("error getting commit messages: %w", err)
	}

	// Display the commit messages with options
	fmt.Println("Suggested commit messages:")
	for i, msg := range commitMessages {
		fmt.Printf("%d: %s\n", i+1, msg)
	}

	// Prompt the user to select one
	fmt.Println("Please select a commit message by number (or press Enter to cancel):")
	var input string
	_, err = fmt.Scanln(&input)
	if err != nil {
		if err.Error() == "unexpected newline" {
			// User pressed Enter without typing anything, cancel
			fmt.Println("No commit message selected. Operation cancelled.")
			return nil
		}
		return fmt.Errorf("error reading input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		// User pressed Enter without typing anything, cancel
		fmt.Println("No commit message selected. Operation cancelled.")
		return nil
	}

	// Convert input to integer
	selection, err := strconv.Atoi(input)
	if err != nil {
		fmt.Println("Invalid selection. Operation cancelled.")
		return nil
	}

	// Validate selection
	if selection < 1 || selection > len(commitMessages) {
		fmt.Println("Invalid selection. Operation cancelled.")
		return nil
	}

	// Get the selected commit message
	selectedMessage := commitMessages[selection-1]

	// Use the selected commit message to create the commit
	if err := git.CreateCommitMessage(selectedMessage); err != nil {
		return fmt.Errorf("error creating commit message: %w", err)
	}
	fmt.Println("Commit created successfully")

	return nil
}
