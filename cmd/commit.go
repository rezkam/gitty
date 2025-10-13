package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rezkam/gritty/git"
	"github.com/rezkam/gritty/provider"
	providerconfig "github.com/rezkam/gritty/provider/config"
	providercore "github.com/rezkam/gritty/provider/core"
	providerservice "github.com/rezkam/gritty/provider/service"
)

const truncatedDiffCharLimit = 8000

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Generate and apply a Git commit message based on staged changes",
	RunE:  runCommitCmd,
}

func init() {
	rootCmd.AddCommand(commitCmd)
}

func runCommitCmd(cmd *cobra.Command, args []string) error {
	configPath, err := providerconfig.FilePath()
	if err != nil {
		return fmt.Errorf("error resolving configuration path: %w", err)
	}

	settings, err := providerconfig.LoadFrom(configPath)
	if err != nil {
		return err
	}

	if strings.TrimSpace(settings.Provider) == "" {
		return fmt.Errorf("provider not set in configuration file")
	}

	factory, err := provider.GetFactory(settings.Provider)
	if err != nil {
		return fmt.Errorf("error getting provider factory: %w", err)
	}

	prov, err := factory(configPath)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}

	gitClient := git.NewCommandClient()
	if !gitClient.IsRepository(".") {
		return fmt.Errorf("current directory is not a Git repository")
	}

	diff, err := gitClient.StagedDiff(".")
	if err != nil {
		return fmt.Errorf("error getting staged diff: %w", err)
	}
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes to commit")
		return nil
	}

	request := providercore.CommitRequest{
		Diff:  diff,
		Count: settings.Completions,
	}

	work := providerservice.NewCommitService(prov)
	result, err := work.Generate(request)
	if err != nil {
		var handled bool
		result, handled, err = handleContextOverflow(work, request, diff, err)
		if handled {
			if err != nil {
				return fmt.Errorf("error getting commit messages: %w", err)
			}
			if len(result.Messages) == 0 {
				return nil
			}
		} else {
			return fmt.Errorf("error getting commit messages: %w", err)
		}
	}

	if result.Downgraded {
		fmt.Println("Selected provider does not support multiple completions; using a single suggestion.")
	}

	if len(result.Messages) == 0 {
		fmt.Println("Provider did not return any commit message suggestions.")
		return nil
	}

	fmt.Println("Suggested commit messages:")
	for i, msg := range result.Messages {
		fmt.Printf("\nOption %d:\n%s\n", i+1, msg)
		fmt.Println("-----")
	}

	fmt.Println("Please select a commit message by number (or press Enter to cancel):")
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		if err.Error() == "unexpected newline" {
			fmt.Println("No commit message selected. Operation cancelled.")
			return nil
		}
		return fmt.Errorf("error reading input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		fmt.Println("No commit message selected. Operation cancelled.")
		return nil
	}

	selection, err := strconv.Atoi(input)
	if err != nil {
		fmt.Println("Invalid selection. Operation cancelled.")
		return nil
	}

	if selection < 1 || selection > len(result.Messages) {
		fmt.Println("Invalid selection. Operation cancelled.")
		return nil
	}

	selectedMessage := result.Messages[selection-1]
	if err := gitClient.CreateCommit(selectedMessage); err != nil {
		return fmt.Errorf("error creating commit message: %w", err)
	}
	fmt.Println("Commit created successfully")

	return nil
}

func handleContextOverflow(work *providerservice.CommitService, req providercore.CommitRequest, diff string, originalErr error) (providerservice.Result, bool, error) {
	if !isContextLengthError(originalErr) {
		return providerservice.Result{}, false, nil
	}

	fmt.Println("The provider reported that the diff is too large for the model context window.")
	fmt.Println("You can stage fewer files, run the model with a larger context length, or retry with a clipped diff.")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Clip the diff to fit within the provider context and retry? (y/N): ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if !(strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")) {
		fmt.Println("No changes made. Stage fewer files or configure a model with a longer context window, then retry.")
		return providerservice.Result{}, true, nil
	}

	clippedDiff, clipped, retainedChars := truncateDiff(diff, truncatedDiffCharLimit)
	if !clipped {
		fmt.Println("The diff is already shorter than the clipping threshold. Please stage fewer files or use a model with a longer context window.")
		return providerservice.Result{}, true, nil
	}

	fmt.Printf("Retrying with approximately %d characters from the diff...\n", retainedChars)
	clippedReq := req
	clippedReq.Diff = clippedDiff

	result, err := work.Generate(clippedReq)
	return result, true, err
}

func truncateDiff(diff string, limit int) (string, bool, int) {
	runes := []rune(diff)
	if len(runes) <= limit {
		return diff, false, len(runes)
	}

	clipped := string(runes[:limit])
	clipped += "\n\n[diff truncated to fit LLM context]\n"
	return clipped, true, limit
}

func isContextLengthError(err error) bool {
	var validationErr *provider.ValidationError
	if !errors.As(err, &validationErr) {
		return false
	}

	msg := strings.ToLower(validationErr.Error())
	return strings.Contains(msg, "context") && strings.Contains(msg, "token")
}
