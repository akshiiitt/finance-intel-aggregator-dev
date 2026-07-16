package scorer

import (
	"math"
	"time"
)

// DefaultUSDINR is the fallback conversion rate when a live rate is unavailable.
// Set conservatively; the fxrates package will provide the live rate at runtime.
const DefaultUSDINR = 84.0

// SourceTrust maps source names to trust scores (max 20 points).
// Higher trust = more established, reliable source.
var SourceTrust = map[string]float64{
	"PIB": 15, "RBI": 15, "SEBI": 15,
	"Reuters Business": 11, "Reuters Tech": 11, "Reuters Markets": 11,
	"AP Business": 10,
	"Bloomberg":   11, "Bloomberg Markets": 11,
	"WSJ Business": 11, "WSJ Markets": 11, "WSJ Tech": 11,
	"FT Business": 11, "FT": 11,
	"Inc42": 12, "Inc42 Features": 11, "Inc42 Analysis": 11,
	"Entrackr": 12, "TechCircle": 11,
	"ET Top Stories": 10, "ET Markets": 10, "ET Economy": 10,
	"ET Tech Startups": 10, "ET Tech Funding": 10, "ET SME": 9,
	"ET Politics": 9, "ET International": 9, "ET AI": 9, "ET Now": 9,
	"ET Telecom": 9, "ET Energy": 9, "ET Realty": 9, "ET CFO": 9,
	"ET Government": 9, "ET CIO": 9,
	"LiveMint Markets": 10, "LiveMint Companies": 10, "LiveMint Money": 9,
	"LiveMint Industry": 9, "LiveMint Economy": 9, "LiveMint Opinion": 8,
	"LiveMint Startups": 10,
	"BS Markets":        10, "BS Companies": 10, "BS Economy": 10,
	"BS Finance": 9, "BS Startups": 9, "BS Technology": 9,
	"HBL Markets": 9, "HBL Companies": 9, "HBL Economy": 9, "HBL InfoTech": 8,
	"NDTV Profit": 9, "CNBCTV18": 9, "CNBCTV18 Markets": 9, "CNBCTV18 Economy": 9,
	"Business Today": 9, "The Hindu Business": 9,
	"Moneycontrol Business": 9, "Moneycontrol Markets": 9, "Moneycontrol Economy": 9,
	"Moneycontrol MF": 8, "Moneycontrol IT": 8, "Moneycontrol RE": 8,
	"Moneycontrol Brokerage": 8,
	"YourStory":              11, "StartupTalky": 8, "Startup Reporter": 8,
	"TechCrunch": 10, "TechCrunch Startups": 10, "TechCrunch Funding": 10,
	"Crunchbase News": 10, "a16z": 10, "YC Blog": 10,
	"VentureBeat": 8, "CB Insights": 9, "Sifted": 9,
	"CNBC Markets": 9, "CNBC Finance": 9, "CNBC World": 9,
	"MarketWatch": 9, "Yahoo Finance": 8,
	"Nikkei Asia Finance": 9,
	"NYT Business":        10, "NYT Economy": 10,
	"Guardian Business": 9,
	"MIT Tech Review":   9,
	"Forbes Innovation": 8, "Inc Magazine": 8, "Entrepreneur": 7,
	"Fast Company": 8, "Quartz": 8,
	"Benzinga": 7, "Seeking Alpha": 8,
	"Investing.com Markets": 7, "Investing.com Crypto": 7, "Investing.com Forex": 7,
	"Blockworks":    8,
	"Outlook Money": 7, "Money9": 7, "DNA Business": 7,
	"TOI Business": 8, "Indian Express Business": 8,
	"ZDNet": 7, "The Register": 7,
}

// regulatorySources gets the official bonus (5 points).
var regulatorySources = map[string]bool{
	"PIB": true, "RBI": true, "SEBI": true,
}

// Input holds all the data needed to compute a FI Score.
type Input struct {
	RelevanceScore float64 // 0–100 from AI classification
	PublishedAt    *time.Time
	Source         string
	Amount         *float64 // absolute value (INR or USD)
	Currency       string
	Category       string
	SentimentScore float64 // 0–100
	KeyPointCount  int
	CoverageCount  int
	// USDINRRate is the live USD/INR spot rate (e.g. 84.52).
	// When zero, DefaultUSDINR (84.0) is used.
	// Provided by fxrates.Global.GetUSDINR() in the calling worker.
	USDINRRate float64
}

// Calculate computes the FIScore (0–100) — a composite ranking for every article.
//
// Component breakdown:
//
//  1. AI Relevance     → max 35 pts  (linear from relevance score)
//  2. Recency decay    → max 25 pts  (λ = ln(2)/4 ≈ 0.1733, half-life = 4 hours)
//  3. Source trust     → max 20 pts  (from SourceTrust registry)
//  4. Deal size        → max 15 pts  (funding / ipo / mergers only)
//  5. Official source  → max  5 pts  (PIB / RBI / SEBI bonus)
//  6. Coverage breadth → max  2 pts  (multi-outlet stories bubble up)
//  7. Sentiment signal → max  2 pts  (strong bull/bear signals more actionable)
//  8. Key-point depth  → max  2 pts  (AI enrichment quality reward)
//
// Total ceiling: 106 pts → capped at 100.
func Calculate(input Input) float64 {
	var score float64

	// 1. AI Relevance (max 35 pts)
	relevance := input.RelevanceScore
	if relevance == 0 {
		relevance = 40
	}
	score += (relevance / 100.0) * 35.0

	// 2. Recency — exponential decay, 4-hour half-life
	//    S(t) = 25 × e^(−λt),  λ = ln(2) / 4
	if input.PublishedAt != nil {
		ageHours := time.Since(*input.PublishedAt).Hours()
		// Malformed feeds sometimes carry future publish dates, which make
		// ageHours negative and blow the decay past the 25-pt recency ceiling.
		// Clamp to "just published."
		if ageHours < 0 {
			ageHours = 0
		}
		lambda := math.Log(2) / 4.0
		decay := 25.0 * math.Exp(-lambda*ageHours)
		if decay < 0.5 {
			decay = 0.5
		}
		score += decay
	} else {
		score += 10 // unknown age — conservative mid-value
	}

	// 3. Source trust (max 20 pts)
	trust, ok := SourceTrust[input.Source]
	if !ok {
		trust = 5
	}
	if trust > 20 {
		trust = 20
	}
	score += trust

	// 4. Deal size (max 15 pts — funding / ipo / mergers only)
	//    Uses live USD/INR rate for accurate crore conversion.
	dealCategories := map[string]bool{"funding": true, "ipo": true, "mergers": true}
	if input.Amount != nil && *input.Amount > 0 && dealCategories[input.Category] {
		rate := input.USDINRRate
		if rate <= 0 {
			rate = DefaultUSDINR
		}
		// Crore factor: (usdINR / 10)
		// e.g. rate=84.52 → factor=8.452 → $10M * 8.452 = 84.52 crore
		usdCroreFactor := rate / 10.0

		var croreEquiv float64
		switch input.Currency {
		case "USD":
			// raw amount is in absolute USD (e.g. 10_000_000 for $10M)
			croreEquiv = (*input.Amount / 1_000_000.0) * usdCroreFactor
		default:
			// INR: raw amount is absolute INR (e.g. 1_000_000_000 for ₹100cr)
			croreEquiv = *input.Amount / 10_000_000.0
		}

		switch {
		case croreEquiv >= 5000:
			score += 15
		case croreEquiv >= 1000:
			score += 12
		case croreEquiv >= 500:
			score += 9
		case croreEquiv >= 100:
			score += 6
		case croreEquiv >= 10:
			score += 3
		default:
			score += 1
		}
	}

	// 5. Official/regulatory source bonus (max 5 pts)
	if regulatorySources[input.Source] {
		score += 5
	}

	// 6. Coverage breadth (max 2 pts)
	coverage := input.CoverageCount
	if coverage == 0 {
		coverage = 1
	}
	if coverage >= 5 {
		score += 2
	} else if coverage >= 2 {
		score += 1
	}

	// 7. Sentiment intensity (max 2 pts — strong directional signals are actionable)
	sentimentScore := input.SentimentScore
	if sentimentScore == 0 {
		sentimentScore = 50
	}
	intensity := math.Abs(sentimentScore-50.0) / 50.0
	if intensity > 0.85 {
		score += 2
	} else if intensity > 0.6 {
		score += 1
	}

	// 8. Key-point richness (max 2 pts)
	if input.KeyPointCount >= 3 {
		score += 2
	} else if input.KeyPointCount >= 1 {
		score += 1
	}

	// Cap at 100 and round to 2 decimal places
	if score > 100 {
		score = 100
	}
	return math.Round(score*100) / 100
}
