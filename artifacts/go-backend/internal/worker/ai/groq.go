package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// classificationModels is the ordered fallback list for fast bulk classification.
// Mirrors the Node.js CLASSIFICATION_MODELS list exactly — fastest/cheapest models first,
// with more capable fallbacks toward the end.
var classificationModels = []string{
	"llama-3.1-8b-instant",                      // fastest, free tier
	"qwen/qwen3.6-27b",                           // strong multilingual
	"qwen/qwen3-32b",                             // deeper reasoning
	"meta-llama/llama-4-scout-17b-16e-instruct",  // Meta Scout
	"openai/gpt-oss-20b",                         // OpenRouter OSS
	"groq/compound-mini",                         // Groq compound
	"allam-2-7b",                                 // Arabic/Indic awareness
	"canopylabs/orpheus-v1-english",               // English specialist
	"openai/gpt-oss-safeguard-20b",               // safe fallback
	"llama-3.3-70b-versatile",                    // heavyweight last resort
}

// enrichmentModels is the ordered fallback list for deep per-article enrichment.
// Mirrors the Node.js ENRICHMENT_MODELS list exactly — best quality first.
var enrichmentModels = []string{
	"llama-3.3-70b-versatile",                    // default high-quality
	"openai/gpt-oss-120b",                        // largest OSS
	"groq/compound",                              // Groq compound full
	"qwen/qwen3-32b",                             // strong reasoning
	"meta-llama/llama-4-scout-17b-16e-instruct",  // Meta Scout
	"openai/gpt-oss-20b",                         // OSS mid
	"qwen/qwen3.6-27b",                           // multilingual fallback
}

const groqBaseURL = "https://api.groq.com/openai/v1/chat/completions"

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model          string        `json:"model"`
	Messages       []groqMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	MaxTokens      int           `json:"max_tokens"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

var groqHTTPClient = &http.Client{Timeout: 45 * time.Second}

// groqPost sends a POST request to the Groq OpenAI-compatible API.
func groqPost(ctx context.Context, apiKey string, req groqRequest) (string, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, groqBaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := groqHTTPClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var gr groqResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return "", fmt.Errorf("groq: json decode: %w", err)
	}
	if gr.Error != nil {
		return "", fmt.Errorf("groq API error: %s", gr.Error.Message)
	}
	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("groq: empty choices")
	}
	return gr.Choices[0].Message.Content, nil
}

// classifyBatchItem is what we send to the AI classification model.
// has_deal_data is a computed hint — true when the title/snippet contains a
// currency or funding pattern. This matches the Node.js shape exactly and
// improves classification accuracy for funding/IPO articles.
type classifyBatchItem struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	HasDealData bool   `json:"has_deal_data"`
}

// GroqClassifyBatch sends up to 30 items to Groq for fast bulk classification.
// Returns a map of item ID → classifyResult. Missing IDs fall back to keyword.
// items here are the internal batchItem structs; this function enriches them
// with the has_deal_data hint before sending to the model.
func GroqClassifyBatch(ctx context.Context, apiKey string, items []batchItem) (map[int64]classifyResult, error) {
	if apiKey == "" || len(items) == 0 {
		return nil, nil
	}

	// Build classify items with has_deal_data hint
	classify := make([]classifyBatchItem, len(items))
	for i, it := range items {
		classify[i] = classifyBatchItem{
			ID:          it.ID,
			Title:       it.Title,
			Snippet:     it.Snippet,
			HasDealData: hasDealPattern(it.Title + " " + it.Snippet),
		}
	}

	itemsJSON, _ := json.Marshal(classify)
	prompt := fmt.Sprintf(`Classify each news item for an Indian finance/startup intelligence platform.
Return JSON: {"results":[{...}]}

Each result must have:
- id (integer, same as input)
- category: one of [funding, ipo, markets, policy, mergers, earnings, startup, technology, crypto, general]
- region: one of [india, global]
- relevance_score: 0-100 (relevance to Indian business/finance/startup audience)
- sentiment: one of [bullish, bearish, neutral]
- sentiment_score: 0-100

Note: has_deal_data=true means the article likely contains a monetary deal/funding/IPO amount.
Use this hint to improve category and relevance accuracy.

Items: %s`, string(itemsJSON))

	var lastErr error
	for _, model := range classificationModels {
		req := groqRequest{
			Model: model,
			Messages: []groqMessage{
				{Role: "system", Content: "You are a financial news classifier for an Indian business intelligence platform. Always return valid JSON only."},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.1,
			MaxTokens:   2048,
		}
		req.ResponseFormat.Type = "json_object"

		ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		content, err := groqPost(ctxTimeout, apiKey, req)
		cancel()

		if err != nil {
			log.Warn().Err(err).Str("model", model).Msg("groq classify: model failed, trying next")
			lastErr = err
			continue
		}

		result, err := parseClassifyResponse(content, model)
		if err != nil {
			log.Warn().Err(err).Str("model", model).Msg("groq classify: parse failed, trying next")
			lastErr = err
			continue
		}

		return result, nil
	}

	return nil, fmt.Errorf("groq classify: all models failed: %w", lastErr)
}

// GroqEnrichSingle sends one article to Groq 70B for deep entity extraction.
// Returns the result, the number of attempts made, and any error.
func GroqEnrichSingle(ctx context.Context, apiKey string, item batchItem) (*enrichResult, int, error) {
	if apiKey == "" {
		return nil, 0, nil
	}

	// Bound the entire fallback process to 45 seconds total so we don't stall the worker
	globalCtx, globalCancel := context.WithTimeout(ctx, 45*time.Second)
	defer globalCancel()

	prompt := fmt.Sprintf(`Analyze this financial news article and return a JSON object:
{
  "companies": ["company1", "company2"],
  "investors": ["investor1"],
  "people": ["person1"],
  "amount": 850000000,
  "currency": "INR",
  "round_type": "Series F",
  "valuation": 3600000000,
  "summary": "2-3 sentence factual summary",
  "key_points": ["fact 1", "fact 2", "fact 3"]
}

Use null for missing fields. Amount/valuation in absolute numbers (₹1 Crore = 10,000,000).
Detect currency: if amount is in USD, set currency to "USD" and use absolute USD value.

Title: %s
Snippet: %s`, item.Title, item.Snippet)

	var lastErr error
	attempts := 0
	for _, model := range enrichmentModels {
		// If the overall context is already canceled, stop trying models
		if globalCtx.Err() != nil {
			break
		}

		req := groqRequest{
			Model: model,
			Messages: []groqMessage{
				{Role: "system", Content: "You are a senior financial analyst. Extract structured data from news articles. Return only valid JSON."},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.1,
			MaxTokens:   800,
		}
		req.ResponseFormat.Type = "json_object"

		attempts++
		ctxTimeout, cancel := context.WithTimeout(globalCtx, 20*time.Second)
		content, err := groqPost(ctxTimeout, apiKey, req)
		cancel()

		if err != nil {
			log.Warn().Err(err).Str("model", model).Msg("groq enrich: model failed, trying next")
			lastErr = err
			continue
		}

		result, err := parseEnrichResponse(content, item.ID, model)
		if err != nil {
			log.Warn().Err(err).Str("model", model).Msg("groq enrich: parse failed, trying next")
			lastErr = err
			continue
		}

		return result, attempts, nil
	}

	return nil, attempts, fmt.Errorf("groq enrich: all models failed: %w", lastErr)
}

// parseClassifyResponse parses the JSON returned by the classification prompt.
func parseClassifyResponse(content, model string) (map[int64]classifyResult, error) {
	var wrapper struct {
		Results []json.RawMessage `json:"results"`
		Items   []json.RawMessage `json:"items"`
	}
	_ = json.Unmarshal([]byte(content), &wrapper)

	items := wrapper.Results
	if len(items) == 0 {
		items = wrapper.Items
	}

	// Handle case where content is a plain JSON array
	if len(items) == 0 {
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(content), &arr); err == nil {
			items = arr
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no results in response")
	}

	label := groqModelLabel(model)
	out := make(map[int64]classifyResult, len(items))

	for _, raw := range items {
		var item struct {
			ID             int64   `json:"id"`
			Category       string  `json:"category"`
			Region         string  `json:"region"`
			RelevanceScore float64 `json:"relevance_score"`
			Sentiment      string  `json:"sentiment"`
			SentimentScore float64 `json:"sentiment_score"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		if item.Region == "both" {
			item.Region = "india"
		}
		out[item.ID] = classifyResult{
			ID:             item.ID,
			Category:       item.Category,
			Region:         item.Region,
			RelevanceScore: item.RelevanceScore,
			Sentiment:      item.Sentiment,
			SentimentScore: item.SentimentScore,
			AIModel:        label,
		}
	}

	return out, nil
}

// parseEnrichResponse parses the JSON returned by the enrichment prompt.
func parseEnrichResponse(content string, id int64, model string) (*enrichResult, error) {
	var data struct {
		Companies []string    `json:"companies"`
		Investors []string    `json:"investors"`
		People    []string    `json:"people"`
		Amount    interface{} `json:"amount"`
		Currency  string      `json:"currency"`
		RoundType string      `json:"round_type"`
		Valuation interface{} `json:"valuation"`
		Summary   string      `json:"summary"`
		KeyPoints []string    `json:"key_points"`
	}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, err
	}

	label := groqModelLabel(model)

	result := &enrichResult{
		ID:        id,
		Companies: safeStrSlice(data.Companies),
		Investors: safeStrSlice(data.Investors),
		People:    safeStrSlice(data.People),
		Currency:  data.Currency,
		RoundType: data.RoundType,
		Summary:   data.Summary,
		KeyPoints: safeStrSlice(data.KeyPoints),
		AIModel:   label,
	}

	if amt := toFloat64(data.Amount); amt > 0 {
		result.Amount = &amt
	}
	if val := toFloat64(data.Valuation); val > 0 {
		result.Valuation = &val
	}

	return result, nil
}

// hasDealPattern returns true when text contains patterns suggesting a monetary
// deal amount — used to compute the has_deal_data hint for the classifier.
func hasDealPattern(text string) bool {
	lower := strings.ToLower(text)
	patterns := []string{
		"crore", "cr ", "₹", "rs ", "inr",
		"million", "billion", "$", "usd",
		"funding", "raises", "raised", "invest", "series a", "series b",
		"series c", "series d", "series e", "series f", "seed round",
		"pre-series", "ipo", "valuation",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// groqModelLabel returns a short human-readable label for a Groq model string.
func groqModelLabel(model string) string {
	switch model {
	case "llama-3.1-8b-instant":
		return "groq-8b"
	case "llama-3.3-70b-versatile":
		return "groq-70b"
	default:
		parts := strings.SplitN(model, "/", 2)
		if len(parts) == 2 {
			return "groq-" + truncate(parts[1], 10)
		}
		return "groq-" + truncate(model, 10)
	}
}

// safeStrSlice returns a non-nil string slice from a possibly-nil input.
func safeStrSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// toFloat64 converts interface{} (number or string) to float64.
func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		if val == "" || val == "null" {
			return 0
		}
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f
		}
	}
	return 0
}
