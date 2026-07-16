// Package embed talks to the local fastembed sidecar (see
// artifacts/embed-sidecar) over HTTP. It is the ONLY thing in the backend
// that knows an embedding model exists — everything downstream (dedup,
// semantic search, niche tagging) just consumes plain []float32 vectors.
//
// The sidecar runs free and unlimited on the same VM as the Go backend —
// no external API, no quota, no per-request cost. If it's ever swapped for
// a different embedding backend, only this file changes.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
)

// Client is a thin HTTP client for the embedding sidecar.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a client. baseURL defaults to the sidecar's usual localhost
// address if empty (e.g. when running the backend directly on the VM
// instead of via docker-compose, where the sidecar is instead reached by
// its service name).
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8900"
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type embedRequest struct {
	Texts []string `json:"texts"`
}

type embedResponse struct {
	Vectors [][]float32 `json:"vectors"`
	Dim     int         `json:"dim"`
}

// Embed returns one vector per input text, in the same order they were
// given. Returns an error (never partial results) so callers can decide for
// themselves what "no embeddings this cycle" means — this client never
// silently degrades on their behalf.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body, _ := json.Marshal(embedRequest{Texts: texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed sidecar: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed sidecar: status %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var out embedResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("embed sidecar: decode: %w", err)
	}
	if len(out.Vectors) != len(texts) {
		return nil, fmt.Errorf("embed sidecar: expected %d vectors, got %d", len(texts), len(out.Vectors))
	}
	return out.Vectors, nil
}

// Healthy does a quick liveness check — used at worker startup to log
// clearly whether semantic dedup/search/niches are active this run.
func (c *Client) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ToSQL formats a vector as the string literal pgvector expects on the
// wire, e.g. "[0.1,0.2,...]". Pass the result as a query parameter
// alongside an explicit ::vector cast — pgx has no native vector type, so
// this is the simplest correct integration without an extra dependency.
func ToSQL(v []float32) string {
	if len(v) == 0 {
		return ""
	}
	buf := bytes.NewBufferString("[")
	for i, f := range v {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(buf, "%g", f)
	}
	buf.WriteByte(']')
	return buf.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
