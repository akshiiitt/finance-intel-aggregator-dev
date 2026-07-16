package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const geminiModel = "gemini-2.5-flash-lite-preview-06-17"
const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"

var geminiHTTPClient = &http.Client{Timeout: 60 * time.Second}

// geminiContent is one content block in the Gemini API request.
type geminiContent struct {
	Parts []struct {
		Text string `json:"text"`
	} `json:"parts"`
}

// geminiRequest is the full request body for Gemini generateContent.
type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig struct {
		Temperature      float64     `json:"temperature"`
		ResponseMimeType string      `json:"responseMimeType"`
		ResponseSchema   interface{} `json:"responseSchema"`
	} `json:"generationConfig"`
}

// GeminiClassifyBatch sends up to 8 items to Gemini for deep classification + enrichment.
// Returns a map of item ID → classifyResult merged with enrichment data.
func GeminiClassifyBatch(ctx context.Context, apiKey string, items []batchItem) (map[int64]AIResult, error) {
	if apiKey == "" || len(items) == 0 {
		return nil, nil
	}

	itemsJSON, _ := json.Marshal(items)

	// Build the response schema for structured output — prevents markdown wrapping.
	schema := buildGeminiSchema()

	prompt := fmt.Sprintf(`You are a senior financial analyst for an Indian investor/entrepreneur intelligence platform.
Analyze these news articles and return structured metadata.

For each item return:
- id: the item id (integer)
- category: one of [funding, ipo, markets, policy, mergers, earnings, startup, technology, crypto, general]
- region: one of [india, global]
- relevance_score: 0-100 (Indian finance/startup audience relevance)
- sentiment: one of [bullish, bearish, neutral]
- sentiment_score: 0-100
- companies: array of company names mentioned (max 5)
- investors: array of investor/VC/fund names (max 4)
- people: key person names (founders, CEOs)
- amount: deal amount as string integer in absolute units — empty string if not mentioned. ₹100 Crore = "1000000000"
- currency: "INR" or "USD" — empty string if not mentioned
- round_type: "Seed", "Series A-F", "IPO", "M&A", "Debt" — empty string if not mentioned
- valuation: post-money valuation string integer if mentioned — empty string otherwise
- summary: 2-3 sentence professional summary
- key_points: array of 2-3 key factual bullet strings

Items to analyze:
%s`, string(itemsJSON))

	req := geminiRequest{}
	req.Contents = []geminiContent{{
		Parts: []struct {
			Text string `json:"text"`
		}{{Text: prompt}},
	}}
	req.GenerationConfig.Temperature = 0.0
	req.GenerationConfig.ResponseMimeType = "application/json"
	req.GenerationConfig.ResponseSchema = schema

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s%s:generateContent", geminiBaseURL, geminiModel)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", apiKey)

	resp, err := geminiHTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

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
		return nil, fmt.Errorf("gemini decode: %w", err)
	}
	if gr.Error != nil {
		return nil, fmt.Errorf("gemini API error: %s", gr.Error.Message)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
	}

	text := gr.Candidates[0].Content.Parts[0].Text

	// Try parsing standard way first
	var wrapper struct {
		Results []map[string]interface{} `json:"results"`
	}
	
	out := make(map[int64]AIResult)
	if err := json.Unmarshal([]byte(text), &wrapper); err == nil {
		for _, item := range wrapper.Results {
			processGeminiItem(item, out)
		}
		return out, nil
	}

	log.Warn().Err(err).Str("raw", truncate(text, 200)).Msg("gemini: bulk parse failed, attempting fallback object-by-object parser")
	
	// A simple scanner that finds balanced brace blocks { ... }
	start := -1
	braceCount := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			if braceCount == 0 {
				start = i
			}
			braceCount++
		} else if text[i] == '}' {
			braceCount--
			if braceCount == 0 && start != -1 {
				candidate := text[start : i+1]
				var item map[string]interface{}
				if err := json.Unmarshal([]byte(candidate), &item); err == nil {
					processGeminiItem(item, out)
				}
				start = -1
			}
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("gemini parse: failed to salvage any items from invalid JSON response")
	}
	return out, nil
}

// processGeminiItem parses a single unstructured map item into the target AIResult map.
func processGeminiItem(item map[string]interface{}, out map[int64]AIResult) {
	id := int64(toFloat64(item["id"]))
	if id == 0 {
		return
	}

	amtStr := fmt.Sprintf("%v", item["amount"])
	valStr := fmt.Sprintf("%v", item["valuation"])
	currStr := fmt.Sprintf("%v", item["currency"])
	roundStr := fmt.Sprintf("%v", item["round_type"])

	if amtStr == "<nil>" || amtStr == "null" {
		amtStr = ""
	}
	if currStr == "<nil>" || currStr == "null" {
		currStr = ""
	}
	if roundStr == "<nil>" || roundStr == "null" {
		roundStr = ""
	}

	result := AIResult{
		Category:       strVal(item, "category", "general"),
		Region:         strVal(item, "region", "global"),
		RelevanceScore: toFloat64(item["relevance_score"]),
		Sentiment:      strVal(item, "sentiment", "neutral"),
		SentimentScore: toFloat64(item["sentiment_score"]),
		Companies:      strSliceVal(item, "companies"),
		Investors:      strSliceVal(item, "investors"),
		People:         strSliceVal(item, "people"),
		Currency:       currStr,
		RoundType:      roundStr,
		Summary:        strVal(item, "summary", ""),
		KeyPoints:      strSliceVal(item, "key_points"),
		AIModel:        "gemini-flash",
	}

	if amtStr != "" {
		if v := toFloat64(amtStr); v > 0 {
			result.Amount = &v
		}
	}
	if valStr != "" && valStr != "<nil>" && valStr != "null" {
		if v := toFloat64(valStr); v > 0 {
			result.Valuation = &v
		}
	}

	out[id] = result
}

// buildGeminiSchema returns the responseSchema object for the Gemini API.
func buildGeminiSchema() interface{} {
	strType := map[string]interface{}{"type": "STRING"}
	intType := map[string]interface{}{"type": "INTEGER"}
	arrStrType := map[string]interface{}{"type": "ARRAY", "items": map[string]interface{}{"type": "STRING"}}

	articleSchema := map[string]interface{}{
		"type": "OBJECT",
		"properties": map[string]interface{}{
			"id":              intType,
			"category":        strType,
			"region":          strType,
			"relevance_score": intType,
			"sentiment":       strType,
			"sentiment_score": intType,
			"companies":       arrStrType,
			"investors":       arrStrType,
			"people":          arrStrType,
			"amount":          strType,
			"currency":        strType,
			"round_type":      strType,
			"valuation":       strType,
			"summary":         strType,
			"key_points":      arrStrType,
		},
		"required": []string{"id", "category", "region", "relevance_score", "sentiment", "sentiment_score", "summary"},
	}

	return map[string]interface{}{
		"type": "OBJECT",
		"properties": map[string]interface{}{
			"results": map[string]interface{}{
				"type":  "ARRAY",
				"items": articleSchema,
			},
		},
		"required": []string{"results"},
	}
}

// strVal safely extracts a string from a map with a default fallback.
func strVal(m map[string]interface{}, key, def string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	s := fmt.Sprintf("%v", v)
	if s == "" {
		return def
	}
	return s
}

// strSliceVal safely extracts a []string from a map.
func strSliceVal(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return []string{}
	}
	raw, ok := v.([]interface{})
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
