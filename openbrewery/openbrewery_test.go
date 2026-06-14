package openbrewery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/openbrewery-cli/openbrewery"
)

func newTestClient(ts *httptest.Server) *openbrewery.Client {
	cfg := openbrewery.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return openbrewery.NewClient(cfg)
}

const mockBreweries = `[
	{"id":"b54b16e1","name":"Anvil Brewery","brewery_type":"micro","city":"Houston","state":"Texas","country":"United States","phone":"7135225521","website_url":"https://anvilbrewery.com","latitude":"29.749907","longitude":"-95.358421"},
	{"id":"c3e82e1c","name":"Blue Dog Brewing","brewery_type":"brewpub","city":"Denver","state":"Colorado","country":"United States","phone":"3031234567","website_url":"https://bluedogbrewing.com","latitude":"39.739236","longitude":"-104.984862"}
]`

func TestListSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.List(context.Background(), openbrewery.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Only return results on page 1; page 2+ returns empty to stop pagination.
		if r.URL.Query().Get("page") == "1" || r.URL.Query().Get("page") == "" {
			_, _ = fmt.Fprint(w, mockBreweries)
		} else {
			_, _ = fmt.Fprint(w, `[]`)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.List(context.Background(), openbrewery.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Name != "Anvil Brewery" {
		t.Errorf("items[0].Name = %q, want %q", items[0].Name, "Anvil Brewery")
	}
	if items[0].BreweryType != "micro" {
		t.Errorf("items[0].BreweryType = %q, want %q", items[0].BreweryType, "micro")
	}
	if items[0].City != "Houston" {
		t.Errorf("items[0].City = %q, want %q", items[0].City, "Houston")
	}
	if items[1].Name != "Blue Dog Brewing" {
		t.Errorf("items[1].Name = %q, want %q", items[1].Name, "Blue Dog Brewing")
	}
}

func TestListLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, mockBreweries)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.List(context.Background(), openbrewery.ListOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1 (limit respected)", len(items))
	}
}

func TestListRetriesOn503(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	cfg := openbrewery.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := openbrewery.NewClient(cfg)

	start := time.Now()
	_, err := c.List(context.Background(), openbrewery.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestSearchParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, mockBreweries)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.Search(context.Background(), "dog", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[1].Name != "Blue Dog Brewing" {
		t.Errorf("items[1].Name = %q, want %q", items[1].Name, "Blue Dog Brewing")
	}
}

func TestRandomParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		size := r.URL.Query().Get("size")
		if size != "3" {
			t.Errorf("size param = %q, want %q", size, "3")
		}
		_, _ = fmt.Fprint(w, mockBreweries)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.Random(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
}

func TestGetMetaParsesResponse(t *testing.T) {
	payload := `{"total":11744,"page":1,"per_page":50,"by_type":{"micro":5000,"brewpub":2000},"by_state":{"Texas":500,"Colorado":400},"by_country":{"United States":10000}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, payload)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	meta, err := c.GetMeta(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if meta.Total != 11744 {
		t.Errorf("meta.Total = %d, want 11744", meta.Total)
	}
	if meta.ByType["micro"] != 5000 {
		t.Errorf("meta.ByType[micro] = %d, want 5000", meta.ByType["micro"])
	}
	if meta.ByCountry["United States"] != 10000 {
		t.Errorf("meta.ByCountry[United States] = %d, want 10000", meta.ByCountry["United States"])
	}
}
