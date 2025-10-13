package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// PromptRequiredString repeatedly prompts the user until a non-empty string is
// provided. The prompt text is written to out and reads from in.
func PromptRequiredString(in io.Reader, out io.Writer, prompt string, emptyMessage string) (string, error) {
	for {
		reader := bufio.NewReader(in)

		if _, err := fmt.Fprint(out, prompt); err != nil {
			return "", err
		}

		input, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value := strings.TrimSpace(input)
		if value != "" {
			return value, nil
		}

		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("input aborted before providing a value")
		}

		if emptyMessage != "" {
			if _, err := fmt.Fprintln(out, emptyMessage); err != nil {
				return "", err
			}
		}
	}
}
