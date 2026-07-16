package stealth

import (
	"context"
	"fmt"
	"hash/fnv"
	"html"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// userAgents is a pool of realistic Chromium user-agent strings.
// One is selected deterministically per URL so each source always
// gets the same UA (consistent fingerprint, not random).
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36 Edg/123.0.0.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:125.0) Gecko/20100101 Firefox/125.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15",
}

// Result holds the response from a stealth fetch.
type Result struct {
	Body   string
	Status int
	OK     bool
}

// Fetcher is a stateless HTTP client that mimics a real browser.
// It uses Chromium-like headers and deterministic UA rotation per URL.
type Fetcher struct {
	client *http.Client
}

// New creates a Fetcher with a shared transport tuned for concurrent RSS fetching.
func New() *Fetcher {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
	}

	return &Fetcher{
		client: &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
		},
	}
}

// pickUA returns a deterministic user-agent string for the given URL.
// Using fnv32 hash ensures the same source always gets the same UA.
func pickUA(url string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(url))
	return userAgents[h.Sum32()%uint32(len(userAgents))]
}

// Fetch performs a stealth HTTP GET with browser-like headers.
// The timeout param controls the context deadline.
func (f *Fetcher) Fetch(ctx context.Context, url string, timeoutMS int) Result {
	if timeoutMS <= 0 {
		timeoutMS = 12000
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}
	}

	ua := pickUA(url)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-IN,en-GB;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="125", "Not/A)Brand";v="8"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)

	resp, err := f.client.Do(req)
	if err != nil {
		return Result{}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // max 5MB
	if err != nil {
		return Result{}
	}

	return Result{
		Body:   string(body),
		Status: resp.StatusCode,
		OK:     resp.StatusCode >= 200 && resp.StatusCode < 300,
	}
}

// FetchJSON performs a stealth GET and returns the raw body as string.
func (f *Fetcher) FetchJSON(ctx context.Context, url string) (string, error) {
	result := f.Fetch(ctx, url, 15000)
	if !result.OK {
		return "", fmt.Errorf("stealth fetch %s: status %d", url, result.Status)
	}
	return result.Body, nil
}

// StripHTML removes HTML tags, decodes HTML entities, and collapses whitespace, truncating at maxLen bytes.
func StripHTML(htmlStr string) string {
	// Decode standard HTML entities first (e.g. &amp; -> &, &nbsp; -> space)
	decoded := html.UnescapeString(htmlStr)

	var b strings.Builder
	inTag := false
	for _, r := range decoded {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	result := strings.Join(strings.Fields(b.String()), " ")
	if len(result) > 500 {
		// Back off to the last valid rune boundary. A naive result[:500] can
		// cut mid-rune (₹, Devanagari, and other multi-byte glyphs are
		// ubiquitous in Indian finance feeds) and produce invalid UTF-8,
		// which Postgres text columns reject — silently dropping the article
		// when the RSS worker's insert fails.
		result = result[:500]
		for len(result) > 0 && !utf8.ValidString(result) {
			result = result[:len(result)-1]
		}
	}
	return result
}
