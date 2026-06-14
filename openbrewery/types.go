package openbrewery

// Brewery is a brewery from the Open Brewery DB.
type Brewery struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	BreweryType string `json:"brewery_type"`
	City        string `json:"city"`
	State       string `json:"state"`
	Country     string `json:"country"`
	Phone       string `json:"phone"`
	WebsiteURL  string `json:"website_url"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
}

// Meta holds database-level statistics from the /v1/breweries/meta endpoint.
type Meta struct {
	Total     int            `json:"total"`
	ByType    map[string]int `json:"by_type"`
	ByState   map[string]int `json:"by_state"`
	ByCountry map[string]int `json:"by_country"`
}
