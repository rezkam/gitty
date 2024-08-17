package git

import "os/exec"

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
	cmd := exec.Command("git", "commit", "-m", message)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
