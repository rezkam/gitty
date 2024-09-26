package git

import (
	"fmt"
	"os/exec"
)

func IsGitDirectory(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func GetStagedDiff(path string) (string, error) {
	cmd := exec.Command("git", "diff", "--staged")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func CreateCommitMessage(message string) error {
	// Combine stdout and stderr to capture all output from the command
	cmd := exec.Command("git", "commit", "-m", message)
	out, err := cmd.CombinedOutput() // Captures both output and errors
	if err != nil {
		return fmt.Errorf("error creating commit message: %w, output: %s", err, string(out))
	}
	return nil
}
