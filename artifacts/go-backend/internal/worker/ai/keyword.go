package ai

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// Category patterns
	reFunding  = regexp.MustCompile(`funding|series [a-f]|seed round|rais[ed]|fundrais`)
	reIPO      = regexp.MustCompile(`ipo|listing|public offer|nse list|bse list|gmp|allotment`)
	reMarkets  = regexp.MustCompile(`nifty|sensex|stock|share price|market|trading|equity`)
	rePolicy   = regexp.MustCompile(`sebi|rbi|budget|policy|regulation|circular|govt|government`)
	reMergers  = regexp.MustCompile(`acqui|merger|buyout|stake sale|m&a`)
	reEarnings = regexp.MustCompile(`quarter|earnings|revenue|profit|results|q[1-4] fy`)
	reStartup  = regexp.MustCompile(`startup|founder|launch`)
	reCrypto   = regexp.MustCompile(`bitcoin|crypto|ethereum|blockchain|defi|nft`)

	// Region pattern
	reRegionIndia = regexp.MustCompile(`india|indian|nifty|sensex|bse|nse|rbi|sebi|crore|rupee|₹|bengaluru|mumbai|delhi|hyderabad`)

	// Amount extraction patterns
	reAmtCrore   = regexp.MustCompile(`₹?\s*([\d,.]+)\s*crore`)
	reAmtBillion = regexp.MustCompile(`\$?\s*([\d,.]+)\s*(?:billion|bn)`)
	reAmtMillion = regexp.MustCompile(`\$?\s*([\d,.]+)\s*(?:million|mn)`)

	// Round type extraction pattern
	reRoundType = regexp.MustCompile(`(?i)\b(seed|pre-seed|series [a-f]|growth round|ipo|debt|bridge)\b`)
)

// keywordProcess is the zero-dependency fallback classifier.
// It runs when all AI APIs are unavailable or quota is exhausted.
// Uses regex and keyword matching to extract structured data from title+snippet.
func keywordProcess(title, snippet string) AIResult {
	text := strings.ToLower(title + " " + snippet)

	// ── Category detection ────────────────────────────────────────────────────
	category := "general"
	switch {
	case matchAny(text, reFunding):
		category = "funding"
	case matchAny(text, reIPO):
		category = "ipo"
	case matchAny(text, reMarkets):
		category = "markets"
	case matchAny(text, rePolicy):
		category = "policy"
	case matchAny(text, reMergers):
		category = "mergers"
	case matchAny(text, reEarnings):
		category = "earnings"
	case matchAny(text, reStartup):
		category = "startup"
	case matchAny(text, reCrypto):
		category = "crypto"
	}

	// ── Region detection ──────────────────────────────────────────────────────
	region := "global"
	if matchAny(text, reRegionIndia) {
		region = "india"
	}

	// ── Sentiment scoring ─────────────────────────────────────────────────────
	bullishWords := []string{"surge", "rise", "growth", "profit", "gain", "record", "milestone",
		"raises", "funding", "ipo", "launch", "expand", "grow", "rally", "soar", "unicorn"}
	bearishWords := []string{"fall", "drop", "loss", "decline", "crash", "shutdown",
		"bankrupt", "down", "cut", "fire", "slump", "plunge", "fraud"}

	var bullishCount, bearishCount float64
	for _, w := range bullishWords {
		if strings.Contains(text, w) {
			bullishCount++
		}
	}
	for _, w := range bearishWords {
		if strings.Contains(text, w) {
			bearishCount++
		}
	}

	sentiment := "neutral"
	if bullishCount > bearishCount {
		sentiment = "bullish"
	} else if bearishCount > bullishCount {
		sentiment = "bearish"
	}
	sentimentScore := math.Max(0, math.Min(100, 50+(bullishCount-bearishCount)*8))

	// ── Relevance scoring ─────────────────────────────────────────────────────
	finKeywords := []string{"funding", "ipo", "startup", "market", "stock", "revenue",
		"investment", "crore", "billion", "million", "acquisition", "merger", "valuation", "unicorn"}
	var relevanceScore float64
	for _, k := range finKeywords {
		if strings.Contains(text, k) {
			relevanceScore += 7
		}
	}
	if relevanceScore > 78 {
		relevanceScore = 78
	}

	// ── Amount extraction ─────────────────────────────────────────────────────
	var amount *float64
	var currency string

	if m := reAmtCrore.FindStringSubmatch(text); len(m) > 1 {
		if v, err := parseNumber(m[1]); err == nil {
			a := v * 10_000_000
			amount = &a
			currency = "INR"
		}
	} else if m := reAmtBillion.FindStringSubmatch(text); len(m) > 1 {
		if v, err := parseNumber(m[1]); err == nil {
			a := v * 1_000_000_000
			amount = &a
			currency = "USD"
		}
	} else if m := reAmtMillion.FindStringSubmatch(text); len(m) > 1 {
		if v, err := parseNumber(m[1]); err == nil {
			a := v * 1_000_000
			amount = &a
			currency = "USD"
		}
	}

	// ── Round type extraction ─────────────────────────────────────────────────
	var roundType string
	if m := reRoundType.FindString(text); m != "" {
		roundType = strings.ToUpper(m[:1]) + m[1:]
	}

	return AIResult{
		Category:       category,
		Region:         region,
		RelevanceScore: relevanceScore,
		Sentiment:      sentiment,
		SentimentScore: sentimentScore,
		Companies:      []string{},
		Investors:      []string{},
		People:         []string{},
		Amount:         amount,
		Currency:       currency,
		RoundType:      roundType,
		Summary:        truncate(title, 200),
		KeyPoints:      []string{},
		AIModel:        "keyword",
	}
}

// matchAny returns true if the text matches the precompiled regex.
func matchAny(text string, re *regexp.Regexp) bool {
	return re.MatchString(text)
}

// parseNumber parses a number string (possibly with commas) into float64.
func parseNumber(s string) (float64, error) {
	clean := strings.ReplaceAll(s, ",", "")
	return strconv.ParseFloat(clean, 64)
}

// truncate shortens a string to at most maxLen bytes without splitting a
// multi-byte UTF-8 rune. A naive s[:maxLen] can cut mid-rune and produce
// invalid UTF-8, which Postgres text columns reject — failing the insert.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := s[:maxLen]
	// Back off to the last valid rune boundary.
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
