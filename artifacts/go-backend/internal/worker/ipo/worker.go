package ipo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/financeintel/backend/internal/stealth"
)

// Worker refreshes IPO calendar data from two sources:
//  1. ipowatch.in / ipowatcher.com RSS feeds (primary)
//  2. NSE India allIpo API (bonus — cookie-gated, best-effort)
type Worker struct {
	pool          *pgxpool.Pool
	StatusLastRun time.Time
	httpClient    *http.Client
}

// New creates an IPO worker.
func New(pool *pgxpool.Pool) *Worker {
	return &Worker{
		pool:       pool,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// ─── RSS / feed types ─────────────────────────────────────────────────────────

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Link        string `xml:"link"`
}

// ipowatchSources are public RSS / XML endpoints that carry IPO data.
var ipowatchSources = []string{
	"https://ipowatch.in/feed/",
	"https://www.ipowatcher.com/feed/",
}

const nseUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// ─── Pre-compiled regexes for extracting IPO fields from RSS description ─────

var (
	rePriceBand = regexp.MustCompile(`(?i)price\s*band[:\s]+₹?\s*([\d,]+)\s*[-–to]+\s*₹?\s*([\d,]+)`)
	reLotSize   = regexp.MustCompile(`(?i)lot\s*size[:\s]+([\d,]+)\s*shares?`)
	reOpenDate  = regexp.MustCompile(`(?i)open(?:ing)?\s*date[:\s]+([A-Za-z0-9, ]+\d{4})`)
	reCloseDate = regexp.MustCompile(`(?i)clos(?:e|ing)\s*date[:\s]+([A-Za-z0-9, ]+\d{4})`)
	reListDate  = regexp.MustCompile(`(?i)list(?:ing)?\s*date[:\s]+([A-Za-z0-9, ]+\d{4})`)
	reIssueSize = regexp.MustCompile(`(?i)issue\s*size[:\s]+₹?\s*([\d,.]+)\s*(cr(?:ore)?|lakh)?`)
	reGMP       = regexp.MustCompile(`(?i)gmp[:\s]+₹?\s*([\d,.]+)`)
	reSubX      = regexp.MustCompile(`(?i)subscri(?:bed|ption)[:\s]+([\d,.]+)\s*[xX×]`)
	reCompany   = regexp.MustCompile(`(?i)^([^:\n]+?)\s+(?:ipo|sme\s+ipo|nse\s+ipo|bse\s+ipo)`)
	reSector    = regexp.MustCompile(`(?i)sector[:\s]+([A-Za-z0-9 &/]+)`)
	reExchange  = regexp.MustCompile(`(?i)(NSE|BSE|SME|NSE\s*SME|BSE\s*SME)`)
)

// ─── Parsed IPO from RSS ──────────────────────────────────────────────────────

type parsedIPO struct {
	CompanyName   string
	Exchange      string
	PriceBandLow  *float64
	PriceBandHigh *float64
	LotSize       *int
	OpenDate      *string
	CloseDate     *string
	ListingDate   *string
	IssueSizeCr   *float64
	GMP           *float64
	SubscriptionX *float64
	Sector        *string
	Status        string
}

// Run performs a full IPO worker cycle:
//  1. Fetch live data from ipowatch.in and ipowatcher.com RSS
//  2. Attempt NSE India allIpo API (best-effort, cookie-gated)
//  3. Auto-advance statuses by date
func (w *Worker) Run(ctx context.Context) error {
	ingested := 0

	// ── Primary: RSS feeds ───────────────────────────────────────────────────
	for _, feedURL := range ipowatchSources {
		n, err := w.ingestFeed(ctx, feedURL)
		if err != nil {
			log.Warn().Err(err).Str("url", feedURL).Msg("ipo worker: feed fetch failed")
			continue
		}
		ingested += n
	}

	// ── Bonus: NSE India API ─────────────────────────────────────────────────
	// Best-effort: a failure here does not abort the cycle.
	nseCount, nseErr := w.fetchNSEIPOs(ctx)
	if nseErr != nil {
		log.Debug().Err(nseErr).Msg("ipo worker: NSE API fetch failed (non-fatal)")
	} else {
		ingested += nseCount
	}

	log.Info().Int("ingested", ingested).Msg("ipo worker: ingestion complete")

	// Auto-advance statuses by date
	w.advanceStatuses(ctx)

	w.StatusLastRun = time.Now()
	return nil
}

// ─── RSS ingestion ────────────────────────────────────────────────────────────

// ingestFeed fetches one RSS source and upserts parsed IPOs.
func (w *Worker) ingestFeed(ctx context.Context, feedURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "FinanceIntel/1.0 IPO-Tracker (+https://github.com/financeintel)")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return 0, fmt.Errorf("xml parse: %w", err)
	}

	count := 0
	for _, item := range feed.Channel.Items {
		ipo := parseIPOFromRSSItem(item)
		if ipo.CompanyName == "" {
			continue
		}
		if err := w.upsertIPO(ctx, ipo); err != nil {
			log.Warn().Err(err).Str("company", ipo.CompanyName).Msg("ipo worker: upsert failed")
			continue
		}
		count++
	}
	return count, nil
}

// ─── NSE India API ────────────────────────────────────────────────────────────

// nseIPOItem is one record from the NSE allIpo API response.
type nseIPOItem struct {
	CompanyName    string `json:"companyName"`
	BiddingStart   string `json:"biddingStartDate"` // "DD-Mon-YYYY"
	BiddingEnd     string `json:"biddingEndDate"`
	ListingDate    string `json:"listingDate"`
	IssuePrice     string `json:"issuePrice"` // "₹123 to ₹456"
	IssueSize      string `json:"issueSize"`  // "1234.56 Cr"
	SubType        string `json:"subType"`
	Exchange       string `json:"exchange"`
	GMP            string `json:"gmp"` // may be blank
}

// toParsedIPO converts a NSE API item to our internal struct.
func (n nseIPOItem) toParsedIPO(status string) parsedIPO {
	ipo := parsedIPO{
		CompanyName: strings.TrimSpace(n.CompanyName),
		Exchange:    strings.ToUpper(strings.TrimSpace(n.Exchange)),
		Status:      status,
	}

	// Parse issue price range "₹123 to ₹456"
	price := strings.ReplaceAll(n.IssuePrice, "₹", "")
	price = strings.ReplaceAll(price, ",", "")
	parts := regexp.MustCompile(`(?i)\s+to\s+`).Split(price, 2)
	if len(parts) == 2 {
		lo := parseFloat(strings.TrimSpace(parts[0]))
		hi := parseFloat(strings.TrimSpace(parts[1]))
		if lo > 0 {
			ipo.PriceBandLow = &lo
		}
		if hi > 0 {
			ipo.PriceBandHigh = &hi
		}
	}

	// Parse issue size "1234.56 Cr"
	szStr := strings.ToLower(strings.ReplaceAll(n.IssueSize, ",", ""))
	szStr = strings.TrimSuffix(strings.TrimSpace(szStr), " cr")
	if v := parseFloat(szStr); v > 0 {
		ipo.IssueSizeCr = &v
	}

	// Parse GMP if provided
	if g := parseFloat(strings.ReplaceAll(n.GMP, "₹", "")); g > 0 {
		ipo.GMP = &g
	}

	// Dates — NSE uses DD-Mon-YYYY e.g. "15-Jan-2025"
	if d := parseDateNSE(n.BiddingStart); d != "" {
		ipo.OpenDate = &d
	}
	if d := parseDateNSE(n.BiddingEnd); d != "" {
		ipo.CloseDate = &d
	}
	if d := parseDateNSE(n.ListingDate); d != "" {
		ipo.ListingDate = &d
	}

	return ipo
}

// fetchNSEIPOs fetches the NSE India allIpo API using a session-cookie flow.
// The NSE API requires an active browser session (cookies set by the homepage).
// Returns number of IPOs upserted, or an error if the session cannot be established.
func (w *Worker) fetchNSEIPOs(ctx context.Context) (int, error) {
	// Use a dedicated client with cookie jar so the session cookie flows
	// from the homepage to the API call automatically.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return 0, fmt.Errorf("nse: cookiejar: %w", err)
	}
	client := &http.Client{
		Timeout: 20 * time.Second,
		Jar:     jar,
	}

	// Step 1: GET homepage — establishes nsit / nseappid cookies
	homeReq, _ := http.NewRequestWithContext(ctx, "GET", "https://www.nseindia.com", nil)
	homeReq.Header.Set("User-Agent", nseUserAgent)
	homeReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	homeReq.Header.Set("Accept-Language", "en-US,en;q=0.5")

	homeResp, err := client.Do(homeReq)
	if err != nil {
		return 0, fmt.Errorf("nse homepage: %w", err)
	}
	// Drain and close — we only need the cookies
	_, _ = io.Copy(io.Discard, homeResp.Body)
	homeResp.Body.Close()

	// Step 2: Wait for cookie establishment — mirrors Node.js 1800ms delay
	select {
	case <-time.After(1800 * time.Millisecond):
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	// Step 3: Fetch allIpo endpoint
	apiReq, _ := http.NewRequestWithContext(ctx, "GET", "https://www.nseindia.com/api/allIpo", nil)
	apiReq.Header.Set("User-Agent", nseUserAgent)
	apiReq.Header.Set("Accept", "application/json, text/plain, */*")
	apiReq.Header.Set("Referer", "https://www.nseindia.com/market-data/all-upcoming-issues-ipo")
	apiReq.Header.Set("X-Requested-With", "XMLHttpRequest")

	apiResp, err := client.Do(apiReq)
	if err != nil {
		return 0, fmt.Errorf("nse api: %w", err)
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("nse api: status %d", apiResp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(apiResp.Body, 1<<20)) // 1 MB cap
	if err != nil {
		return 0, fmt.Errorf("nse: read: %w", err)
	}

	// Step 4: Parse JSON — the API returns multiple IPO status arrays
	var nseData struct {
		Ongoing  []nseIPOItem `json:"ongoing"`
		Upcoming []nseIPOItem `json:"upcoming"`
		Listed   []nseIPOItem `json:"listediposs"` // Note: NSE typo in field name
		NewListed []nseIPOItem `json:"newListed"`
	}
	if err := json.Unmarshal(body, &nseData); err != nil {
		return 0, fmt.Errorf("nse: json parse: %w", err)
	}

	count := 0
	type statusItem struct {
		items  []nseIPOItem
		status string
	}
	for _, grp := range []statusItem{
		{nseData.Ongoing, "open"},
		{nseData.Upcoming, "upcoming"},
		{nseData.Listed, "listed"},
		{nseData.NewListed, "listed"},
	} {
		for _, item := range grp.items {
			ipo := item.toParsedIPO(grp.status)
			if ipo.CompanyName == "" {
				continue
			}
			if err := w.upsertIPO(ctx, ipo); err == nil {
				count++
			}
		}
	}
	return count, nil
}

// ─── RSS item parser ──────────────────────────────────────────────────────────

func parseIPOFromRSSItem(item rssItem) parsedIPO {
	text := item.Title + "\n" + stealth.StripHTML(item.Description)
	ipo := parsedIPO{Status: "upcoming"}

	// Company name from title
	if m := reCompany.FindStringSubmatch(item.Title); len(m) > 1 {
		ipo.CompanyName = strings.TrimSpace(m[1])
	} else {
		name := strings.TrimSpace(item.Title)
		for _, suffix := range []string{" IPO", " SME IPO", " NSE IPO", " BSE IPO", " GMP Today", " Allotment"} {
			name = strings.TrimSuffix(name, suffix)
		}
		ipo.CompanyName = name
	}

	if m := rePriceBand.FindStringSubmatch(text); len(m) > 2 {
		lo := parseFloat(m[1])
		hi := parseFloat(m[2])
		if lo > 0 {
			ipo.PriceBandLow = &lo
		}
		if hi > 0 {
			ipo.PriceBandHigh = &hi
		}
	}
	if m := reLotSize.FindStringSubmatch(text); len(m) > 1 {
		if n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", "")); err == nil && n > 0 {
			ipo.LotSize = &n
		}
	}
	if m := reOpenDate.FindStringSubmatch(text); len(m) > 1 {
		if d := parseDate(m[1]); d != "" {
			ipo.OpenDate = &d
		}
	}
	if m := reCloseDate.FindStringSubmatch(text); len(m) > 1 {
		if d := parseDate(m[1]); d != "" {
			ipo.CloseDate = &d
		}
	}
	if m := reListDate.FindStringSubmatch(text); len(m) > 1 {
		if d := parseDate(m[1]); d != "" {
			ipo.ListingDate = &d
		}
	}
	if m := reIssueSize.FindStringSubmatch(text); len(m) > 1 {
		v := parseFloat(m[1])
		if v > 0 {
			if strings.HasPrefix(strings.ToLower(m[2]), "lakh") {
				v = v / 100
			}
			ipo.IssueSizeCr = &v
		}
	}
	if m := reGMP.FindStringSubmatch(text); len(m) > 1 {
		if v := parseFloat(m[1]); v > 0 {
			ipo.GMP = &v
		}
	}
	if m := reSubX.FindStringSubmatch(text); len(m) > 1 {
		if v := parseFloat(m[1]); v > 0 {
			ipo.SubscriptionX = &v
		}
	}
	if m := reSector.FindStringSubmatch(text); len(m) > 1 {
		s := strings.TrimSpace(m[1])
		ipo.Sector = &s
	}
	if m := reExchange.FindStringSubmatch(text); len(m) > 1 {
		ipo.Exchange = strings.ToUpper(strings.ReplaceAll(m[1], " ", ""))
	}

	// Infer status
	textLower := strings.ToLower(text)
	switch {
	case strings.Contains(textLower, "listing") || strings.Contains(textLower, "listed"):
		ipo.Status = "listed"
	case strings.Contains(textLower, "allotment") || strings.Contains(textLower, "subscri"):
		ipo.Status = "closed"
	case strings.Contains(textLower, "open") && strings.Contains(textLower, "close"):
		ipo.Status = "open"
	}
	return ipo
}

// ─── DB upsert ────────────────────────────────────────────────────────────────

func (w *Worker) upsertIPO(ctx context.Context, ipo parsedIPO) error {
	if ipo.CompanyName == "" {
		return nil
	}
	_, err := w.pool.Exec(ctx, `
		INSERT INTO ipo_calendar (
		    company_name, exchange,
		    price_band_low, price_band_high, lot_size,
		    open_date, close_date, listing_date,
		    issue_size_cr, gmp, subscription_x, status, sector, updated_at
		) VALUES (
		    $1, NULLIF($2,''),
		    $3, $4, $5,
		    $6::date, $7::date, $8::date,
		    $9, $10, $11, $12, $13, NOW()
		)
		ON CONFLICT (company_name) DO UPDATE SET
		    gmp             = COALESCE(EXCLUDED.gmp,             ipo_calendar.gmp),
		    subscription_x  = COALESCE(EXCLUDED.subscription_x,  ipo_calendar.subscription_x),
		    price_band_low   = COALESCE(EXCLUDED.price_band_low,   ipo_calendar.price_band_low),
		    price_band_high  = COALESCE(EXCLUDED.price_band_high,  ipo_calendar.price_band_high),
		    lot_size         = COALESCE(EXCLUDED.lot_size,         ipo_calendar.lot_size),
		    issue_size_cr    = COALESCE(EXCLUDED.issue_size_cr,    ipo_calendar.issue_size_cr),
		    open_date        = COALESCE(EXCLUDED.open_date,        ipo_calendar.open_date),
		    close_date       = COALESCE(EXCLUDED.close_date,       ipo_calendar.close_date),
		    listing_date     = COALESCE(EXCLUDED.listing_date,     ipo_calendar.listing_date),
		    exchange         = COALESCE(NULLIF(EXCLUDED.exchange,''), ipo_calendar.exchange),
		    sector           = COALESCE(EXCLUDED.sector,           ipo_calendar.sector),
		    updated_at       = NOW()
	`,
		ipo.CompanyName, ipo.Exchange,
		ipo.PriceBandLow, ipo.PriceBandHigh, ipo.LotSize,
		ipo.OpenDate, ipo.CloseDate, ipo.ListingDate,
		ipo.IssueSizeCr, ipo.GMP, ipo.SubscriptionX, ipo.Status, ipo.Sector,
	)
	return err
}

// ─── Status auto-advance ──────────────────────────────────────────────────────

func (w *Worker) advanceStatuses(ctx context.Context) {
	today := time.Now().UTC().Format("2006-01-02")

	if _, err := w.pool.Exec(ctx, `
		UPDATE ipo_calendar SET status = 'open', updated_at = NOW()
		WHERE status = 'upcoming'
		  AND open_date IS NOT NULL AND open_date::text <= $1
	`, today); err != nil {
		log.Warn().Err(err).Msg("ipo worker: upcoming→open failed")
	}
	if _, err := w.pool.Exec(ctx, `
		UPDATE ipo_calendar SET status = 'closed', updated_at = NOW()
		WHERE status = 'open'
		  AND close_date IS NOT NULL AND close_date::text < $1
	`, today); err != nil {
		log.Warn().Err(err).Msg("ipo worker: open→closed failed")
	}
	if _, err := w.pool.Exec(ctx, `
		UPDATE ipo_calendar SET status = 'listed', updated_at = NOW()
		WHERE status = 'closed'
		  AND listing_date IS NOT NULL AND listing_date::text <= $1
	`, today); err != nil {
		log.Warn().Err(err).Msg("ipo worker: closed→listed failed")
	}
	log.Debug().Msg("ipo worker: status advance complete")
}

// ─── Parsing helpers ──────────────────────────────────────────────────────────

func parseFloat(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

var dateLayouts = []string{
	"2 January 2006", "02 January 2006",
	"January 2, 2006", "January 02, 2006",
	"2 Jan 2006", "02 Jan 2006",
	"Jan 2, 2006", "Jan 02, 2006",
	"2-Jan-2006", "02-Jan-2006",
	"2006-01-02",
}

func parseDate(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, ", "); idx > 0 && idx < 12 {
		candidate := strings.TrimSpace(s[idx+2:])
		if len(candidate) > 6 {
			s = candidate
		}
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// parseDateNSE parses NSE-format dates like "15-Jan-2025".
var nseDateLayouts = []string{
	"02-Jan-2006",
	"2-Jan-2006",
	"Jan 02, 2006",
	"02 Jan 2006",
}

func parseDateNSE(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return ""
	}
	for _, layout := range nseDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	// Fallback to generic parser
	return parseDate(s)
}
