package openbrewery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// mockBreweries uses the real API wire format: brewery_type, state_province,
// address_1, website_url, and numeric lat/lon.
const mockBreweries = `[
	{"id":"10-barrel-brewing-co-bend-1","name":"10 Barrel Brewing Co","brewery_type":"large","address_1":"62970 18th St","city":"Bend","state_province":"Oregon","postal_code":"97701","country":"United States","longitude":-121.28170597,"latitude":44.08770132,"phone":"5415851007","website_url":"http://www.10barrel.com","state":"Oregon","street":"62970 18th St"},
	{"id":"anvil-brewery-houston","name":"Anvil Brewery","brewery_type":"micro","address_1":"1424 Westheimer Rd","city":"Houston","state_province":"Texas","postal_code":"77006","country":"United States","longitude":-95.397678,"latitude":29.745914,"phone":"7135225521","website_url":"https://anvilbrewery.com","state":"Texas","street":"1424 Westheimer Rd"}
]`

// TestListSendsUserAgent checks that the client sets a User-Agent header.
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

// TestListParsesItems checks field mapping: API wire names map to clean Brewery fields.
func TestListParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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

	b := items[0]
	if b.ID != "10-barrel-brewing-co-bend-1" {
		t.Errorf("ID = %q, want 10-barrel-brewing-co-bend-1", b.ID)
	}
	if b.Name != "10 Barrel Brewing Co" {
		t.Errorf("Name = %q, want 10 Barrel Brewing Co", b.Name)
	}
	if b.Type != "large" {
		t.Errorf("Type = %q, want large", b.Type)
	}
	if b.City != "Bend" {
		t.Errorf("City = %q, want Bend", b.City)
	}
	if b.State != "Oregon" {
		t.Errorf("State = %q, want Oregon (from state_province)", b.State)
	}
	if b.Country != "United States" {
		t.Errorf("Country = %q, want United States", b.Country)
	}
	if b.Address != "62970 18th St" {
		t.Errorf("Address = %q, want 62970 18th St (from address_1)", b.Address)
	}
	if b.Website != "http://www.10barrel.com" {
		t.Errorf("Website = %q, want http://www.10barrel.com (from website_url)", b.Website)
	}
	if b.Lat == "" {
		t.Error("Lat should not be empty")
	}
	if b.Lon == "" {
		t.Error("Lon should not be empty")
	}
}

// TestListLimit checks that the limit is respected.
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

// TestListRetriesOn503 checks exponential backoff on 5xx errors.
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

// TestSearchParsesItems checks that Search hits the right endpoint and parses results.
func TestSearchParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/breweries/search") {
			t.Errorf("unexpected path %q, want /v1/breweries/search", r.URL.Path)
		}
		q := r.URL.Query().Get("query")
		if q == "" {
			t.Error("query param missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, mockBreweries)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.Search(context.Background(), "10 barrel", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[1].Name != "Anvil Brewery" {
		t.Errorf("items[1].Name = %q, want Anvil Brewery", items[1].Name)
	}
}

// TestGetByID checks that Get hits the right path and maps fields correctly.
func TestGetByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/10-barrel-brewing-co-bend-1") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"10-barrel-brewing-co-bend-1","name":"10 Barrel Brewing Co","brewery_type":"large","address_1":"62970 18th St","city":"Bend","state_province":"Oregon","country":"United States","longitude":-121.28170597,"latitude":44.08770132,"phone":"5415851007","website_url":"http://www.10barrel.com"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	b, err := c.Get(context.Background(), "10-barrel-brewing-co-bend-1")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "10 Barrel Brewing Co" {
		t.Errorf("Name = %q, want 10 Barrel Brewing Co", b.Name)
	}
	if b.Type != "large" {
		t.Errorf("Type = %q, want large (from brewery_type)", b.Type)
	}
	if b.State != "Oregon" {
		t.Errorf("State = %q, want Oregon (from state_province)", b.State)
	}
	if b.Address != "62970 18th St" {
		t.Errorf("Address = %q, want 62970 18th St (from address_1)", b.Address)
	}
	if b.Website != "http://www.10barrel.com" {
		t.Errorf("Website = %q, want http://www.10barrel.com (from website_url)", b.Website)
	}
}

// TestGetNotFound checks that a 404 returns an error.
func TestGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Get(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

// TestListByCity checks that city filter is passed as by_city param.
func TestListByCity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		city := r.URL.Query().Get("by_city")
		if city != "san diego" {
			t.Errorf("by_city = %q, want san diego", city)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.List(context.Background(), openbrewery.ListOptions{City: "san diego", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
}

// TestListByType checks that type filter is passed as by_type param.
func TestListByType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		byType := r.URL.Query().Get("by_type")
		if byType != "micro" {
			t.Errorf("by_type = %q, want micro", byType)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.List(context.Background(), openbrewery.ListOptions{Type: "micro", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
}
