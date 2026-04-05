package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

type OpenAIBrain struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewOpenAIBrain(apiKey string, model string) *OpenAIBrain {
	return &OpenAIBrain{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (b *OpenAIBrain) Chat(ctx context.Context, messages []Message) (string, ConsumptionReport, error) {
	url := "https://api.openai.com/v1/chat/completions"

	payload := map[string]any{
		"model":    b.Model,
		"messages": messages,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("failed to encode payload: %w", err)
	}

	var body []byte
	var lastErr error

	// Exponential Backoff implementation: 5 attempts (1s, 2s, 4s, 8s, 16s)
	for i := 0; i < 5; i++ {
		if i > 0 {
			waitTime := time.Duration(math.Pow(2, float64(i-1))) * time.Second
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ConsumptionReport{}, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", ConsumptionReport{}, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b.APIKey))

		resp, err := b.Client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ = io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
			if resp.StatusCode >= 500 {
				continue // Retry only on server errors
			}
			break
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return "", ConsumptionReport{}, err
		}

		if len(result.Choices) > 0 {
			report := ConsumptionReport{
				PromptTokens:     result.Usage.PromptTokens,
				CompletionTokens: result.Usage.CompletionTokens,
				TotalTokens:      result.Usage.TotalTokens,
			}
			return result.Choices[0].Message.Content, report, nil
		}
	}

	return "", ConsumptionReport{}, fmt.Errorf("failed after multiple attempts: %v", lastErr)
}
