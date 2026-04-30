package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// doPost marshals payload and POSTs to url with Bearer auth.
// extraHeaders are added after the standard Content-Type/Authorization headers.
// Returns the raw response body, HTTP status code, and any transport error.
func doPost(ctx context.Context, client *http.Client, url, apiKey string, payload map[string]interface{}, extraHeaders map[string]string) ([]byte, int, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal payload: %w", err)
	}

	newReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
		return req, nil
	}

	resp, err := httpDoWithRetry(ctx, client, newReq, 3)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}
	return body, resp.StatusCode, nil
}

// doPostWithFallback calls doPost and, on HTTP 402, strips any token-limit
// fields (max_tokens, max_completion_tokens) and retries once. This handles
// providers that reject requests when the requested token ceiling exceeds the
// account's available credits.
func doPostWithFallback(ctx context.Context, client *http.Client, url, apiKey string, payload map[string]interface{}, extraHeaders map[string]string) ([]byte, int, error) {
	body, status, err := doPost(ctx, client, url, apiKey, payload, extraHeaders)
	if err != nil {
		return nil, 0, err
	}
	if status != http.StatusPaymentRequired {
		return body, status, nil
	}

	fallback := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		if k == "max_tokens" || k == "max_completion_tokens" {
			continue
		}
		fallback[k] = v
	}
	return doPost(ctx, client, url, apiKey, fallback, extraHeaders)
}
