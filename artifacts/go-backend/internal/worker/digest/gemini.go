package digest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const geminiModel = "gemini-2.5-flash-lite-preview-06-17"
const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"

var geminiHTTPClient = &http.Client{Timeout: 30 * time.Second}

// callGeminiText sends a single free-text prompt to Gemini and returns the
// plain response text. A prose briefing doesn't need the structured JSON
// schema the classification pipeline uses (see internal/worker/ai/gemini.go)
// — this is a separate, simpler call kept local to this package.
func callGeminiText(ctx context.Context, apiKey, prompt string) (string, error) {
	reqBody := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{"temperature": 0.4},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s%s:generateContent", geminiBaseURL, geminiModel)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := geminiHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini: status %d", resp.StatusCode)
	}

	var gr struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &gr); err != nil {
		return "", fmt.Errorf("gemini: decode: %w", err)
	}
	if gr.Error != nil {
		return "", fmt.Errorf("gemini API error: %s", gr.Error.Message)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}
	return gr.Candidates[0].Content.Parts[0].Text, nil
}
