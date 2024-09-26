package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

const grittyConfigFile = ".gritty/config.yaml"

var rootCmd = &cobra.Command{
	Use:   "gritty",
	Short: "Gritty is a tool to generate Git commit messages using AI",
	Long: `Gritty helps developers generate Git commit messages based on staged changes 
in a Git repository using AI models. If no configuration is found, you will be 
prompted to initialize it.`,
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
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No configuration file found. Please run 'gritty init' to set up your configuration.")
		os.Exit(1)
	}
	return nil
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting user home directory to read the configs:", err)
		os.Exit(1)
	}
	return filepath.Join(home, grittyConfigFile)

}
