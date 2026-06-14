package openbrewery

import "encoding/json"

// Brewery is a brewery from the Open Brewery DB. JSON tags use the clean
// output names; the raw API field names are handled by UnmarshalJSON.
type Brewery struct {
	ID      string `kit:"id" json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`    // brewery_type in API
	City    string `json:"city"`
	State   string `json:"state"`   // state_province in API
	Country string `json:"country"`
	Address string `json:"address"` // address_1 in API
	Phone   string `json:"phone"`
	Website string `json:"website"` // website_url in API
	Lat     string `json:"lat"`     // latitude in API (may be number or string)
	Lon     string `json:"lon"`     // longitude in API (may be number or string)
}

// rawBrewery mirrors the API wire format exactly.
type rawBrewery struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	BreweryType string          `json:"brewery_type"`
	Address1    string          `json:"address_1"`
	City        string          `json:"city"`
	StateProvince string        `json:"state_province"`
	Country     string          `json:"country"`
	Phone       string          `json:"phone"`
	WebsiteURL  string          `json:"website_url"`
	Latitude    json.Number     `json:"latitude"`
	Longitude   json.Number     `json:"longitude"`
}

// UnmarshalJSON maps API wire fields to clean Brewery fields.
func (b *Brewery) UnmarshalJSON(data []byte) error {
	var r rawBrewery
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	b.ID = r.ID
	b.Name = r.Name
	b.Type = r.BreweryType
	b.City = r.City
	b.State = r.StateProvince
	b.Country = r.Country
	b.Address = r.Address1
	b.Phone = r.Phone
	b.Website = r.WebsiteURL
	b.Lat = r.Latitude.String()
	b.Lon = r.Longitude.String()
	// json.Number.String() returns "" when zero-value; keep as empty string
	if b.Lat == "" || b.Lat == "0" {
		b.Lat = r.Latitude.String()
	}
	return nil
}

// Meta holds database-level statistics from the /v1/breweries/meta endpoint.
type Meta struct {
	Total     int            `json:"total"`
	ByType    map[string]int `json:"by_type"`
	ByState   map[string]int `json:"by_state"`
	ByCountry map[string]int `json:"by_country"`
}
