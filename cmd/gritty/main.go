package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rezkam/gritty/git"
	"github.com/rezkam/gritty/openai"
)

func main() {

	// Check if the current directory is a Git repository
	if !git.IsGitDirectory(".") {
		fmt.Println("Current directory is not a Git repository")
		os.Exit(1)
	}

	// Get the staged diff
	diff, err := git.GetStagedDiff(".")
	if err != nil {
		fmt.Println("Error getting staged diff:", err)
		os.Exit(1)
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes to commit")
		os.Exit(0)
	}

	// Create a new OpenAI provider
	provider, err := NewOpenAIProvider()
	if err != nil {
		fmt.Println("Error creating OpenAI provider:", err)
		os.Exit(1)
	}

	// Use the commit message provider to get a commit message
	commitMessage, err := provider.GetCommitMessage(diff)
	if err != nil {
		fmt.Println("Error getting commit message:", err)
		os.Exit(1)
	}
	fmt.Println("Suggested commit message:", commitMessage)

	var confirm string
	fmt.Println("Do you want to use this commit message? (y/n):")
	_, err = fmt.Scanln(&confirm)
	if err != nil {
		fmt.Println("Error reading input:", err)
		os.Exit(1)
	}
	confirm = strings.TrimSpace(confirm)

	if strings.ToLower(confirm) == "y" {
		if err := git.CreateCommitMessage(commitMessage); err != nil {
			fmt.Println("Error creating commit message:", err)
			os.Exit(1)
		}
		fmt.Println("Commit created successfully")
	}
}

type CommitMessageProvider interface {
	GetCommitMessage(diff string) (string, error)
}

func NewOpenAIProvider() (CommitMessageProvider, error) {
	const key = "OPENAI_API_KEY"
	apiKey := os.Getenv(key)
	if apiKey == "" {
		return nil, fmt.Errorf("openai api-key not set in environment variable %s", key)
	}

	return openai.NewProvider(apiKey), nil
}
