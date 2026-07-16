package ai

// AIResult holds the structured output from any AI model (Groq, Gemini, or keyword fallback).
// All fields match the processed_items table schema.
type AIResult struct {
	Category       string
	Region         string
	RelevanceScore float64
	Sentiment      string
	SentimentScore float64
	Companies      []string
	Investors      []string
	People         []string
	Amount         *float64 // nil if not mentioned
	Currency       string
	RoundType      string
	Valuation      *float64 // nil if not mentioned
	Summary        string
	KeyPoints      []string
	AIModel        string
}

// classifyResult holds the fast-tier output (Groq 8B / Gemini bulk).
// Only classification fields — no deep enrichment.
type classifyResult struct {
	ID             int64
	Category       string
	Region         string
	RelevanceScore float64
	Sentiment      string
	SentimentScore float64
	AIModel        string
}

// enrichResult holds deep enrichment output (Groq 70B / Gemini).
type enrichResult struct {
	ID        int64
	Companies []string
	Investors []string
	People    []string
	Amount    *float64
	Currency  string
	RoundType string
	Valuation *float64
	Summary   string
	KeyPoints []string
	AIModel   string
}
