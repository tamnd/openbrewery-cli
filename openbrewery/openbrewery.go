// Package openbrewery is the library behind the openbrewery command line:
// the HTTP client, request shaping, and typed data models for the Open Brewery DB.
//
// The Client sets a real User-Agent, paces requests, and retries transient
// failures (429 and 5xx) with exponential backoff. Four operations are provided:
// list breweries with filters, full-text search, random, and database statistics.
package openbrewery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "api.openbrewerydb.org"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Config holds tunable knobs for the HTTP client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns sensible defaults for production use.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: "openbrewery-cli/0.1.0 (github.com/tamnd/openbrewery-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to the Open Brewery DB over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// ListOptions controls filtering for the List method.
type ListOptions struct {
	City    string // by_city
	State   string // by_state
	Country string // by_country
	Type    string // by_type
	Limit   int    // max results; 0 = default 50
}

// List returns breweries with optional filters. Paginates automatically.
func (c *Client) List(ctx context.Context, opts ListOptions) ([]Brewery, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	pageSize := 50
	if pageSize > limit {
		pageSize = limit
	}

	var out []Brewery
	page := 1
	for {
		u := c.buildListURL(opts, page, pageSize)
		b, err := c.get(ctx, u)
		if err != nil {
			return nil, err
		}
		var batch []Brewery
		if err := json.Unmarshal(b, &batch); err != nil {
			return nil, fmt.Errorf("decode breweries: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		out = append(out, batch...)
		if len(out) >= limit {
			break
		}
		page++
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Search returns breweries matching the query string.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Brewery, error) {
	if limit <= 0 {
		limit = 50
	}
	u := c.cfg.BaseURL + "/v1/breweries/search?query=" + url.QueryEscape(query) + fmt.Sprintf("&per_page=%d", limit)
	b, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var items []Brewery
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	return items, nil
}

// Random returns size random breweries. size defaults to 1 if <= 0.
func (c *Client) Random(ctx context.Context, size int) ([]Brewery, error) {
	if size <= 0 {
		size = 1
	}
	u := fmt.Sprintf("%s/v1/breweries/random?size=%d", c.cfg.BaseURL, size)
	b, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var items []Brewery
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, fmt.Errorf("decode random: %w", err)
	}
	return items, nil
}

// GetMeta returns database statistics from the /meta endpoint.
func (c *Client) GetMeta(ctx context.Context) (*Meta, error) {
	b, err := c.get(ctx, c.cfg.BaseURL+"/v1/breweries/meta")
	if err != nil {
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode meta: %w", err)
	}
	return &m, nil
}

func (c *Client) buildListURL(opts ListOptions, page, pageSize int) string {
	params := url.Values{}
	params.Set("per_page", fmt.Sprintf("%d", pageSize))
	params.Set("page", fmt.Sprintf("%d", page))
	if opts.City != "" {
		params.Set("by_city", opts.City)
	}
	if opts.State != "" {
		params.Set("by_state", opts.State)
	}
	if opts.Country != "" {
		params.Set("by_country", opts.Country)
	}
	if opts.Type != "" {
		params.Set("by_type", opts.Type)
	}
	return c.cfg.BaseURL + "/v1/breweries?" + params.Encode()
}

// get fetches url and returns the response body. It paces and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
