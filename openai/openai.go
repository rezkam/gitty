package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	model     = "gpt-4o-mini"
	url       = "https://api.openai.com/v1/chat/completions"
	maxTokens = 150
)

type Provider struct {
	APIKey string
}

func NewProvider(apiKey string) *Provider {
	return &Provider{
		APIKey: apiKey,
	}
}

type response struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message message `json:"message"`
}

type message struct {
	Content string `json:"content"`
}

func (p *Provider) GetCommitMessage(diff string) (string, error) {
	reqPayload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are an assistant that helps in writing concise and clear Git commit messages."},
			{"role": "user", "content": fmt.Sprintf("Based on the following git diff, suggest a concise and clear git commit message:\n\n%s", diff)},
		},
		"max_tokens": maxTokens,
	}

	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode request payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api request failed with status code: %d", resp.StatusCode)
	}

	var respPayload response
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	if err != nil {
		return "", fmt.Errorf("failed to decode response payload: %w", err)
	}

	// Extract the commit message from the response
	if len(respPayload.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	commitMessage := respPayload.Choices[0].Message.Content

	return commitMessage, nil
}
