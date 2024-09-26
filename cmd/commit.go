package cmd

import (
	"fmt"
	"github.com/rezkam/gritty/git"
	"github.com/rezkam/gritty/provider"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strings"
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

	// Use the commit message provider to get a commit message
	commitMessage, err := p.GetCommitMessage(diff)
	if err != nil {
		return fmt.Errorf("error getting commit message: %w", err)
	}
	fmt.Println("Suggested commit message:", commitMessage)

	var confirm string
	fmt.Println("Do you want to use this commit message? (y/n):")
	_, err = fmt.Scanln(&confirm)
	if err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}
	confirm = strings.TrimSpace(confirm)

	if strings.ToLower(confirm) == "y" {
		if err := git.CreateCommitMessage(commitMessage); err != nil {
			return fmt.Errorf("error creating commit message: %w", err)
		}
		fmt.Println("Commit created successfully")
	}

	return nil
}
