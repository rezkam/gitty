package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	providerconfig "github.com/rezkam/gritty/provider/config"
)

var rootCmd = &cobra.Command{
	Use:   "gritty",
	Short: "Gritty is a tool to generate Git commit messages using AI",
	Long: `Gritty helps developers generate Git commit messages based on staged changes 
in a Git repository using AI models. If no configuration is found, you will be 
prompted to initialize it.`,
	RunE: runCommitCmd, // call commit command by default when no subcommand is provided
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Use == "init" {
			return nil
		}
		return checkConfig(cmd, args)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func checkConfig(cmd *cobra.Command, args []string) error {
	configPath, err := providerconfig.FilePath()
	if err != nil {
		return fmt.Errorf("error resolving configuration path: %w", err)
	}
	exists, err := providerconfig.Exists(configPath)
	if err != nil {
		return fmt.Errorf("error checking configuration: %w", err)
	}
	if !exists {
		fmt.Println("No configuration file found. Please run 'gritty init' to set up your configuration.")
		os.Exit(1)
	}
	return nil
}
