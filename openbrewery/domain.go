package openbrewery

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the openbrewery kit driver. It carries no state; the per-run
// client is built by the factory Register hands to kit.
type Domain struct{}

// Info describes the scheme, hostnames, and the identity used in help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "openbrewery",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "openbrewery",
			Short:  "A command line for the Open Brewery DB.",
			Long: `A command line for the Open Brewery DB.

openbrewery reads public brewery data from api.openbrewerydb.org over plain
HTTPS, shapes it into clean records, and prints output that pipes into the rest
of your tools. No API key, nothing to run alongside it.

List breweries by city, state, or type. Search by name. Fetch a single brewery
by its slug ID.`,
			Site: Host,
			Repo: "https://github.com/tamnd/openbrewery-cli",
		},
	}
}

// Register installs the client factory and all operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "breweries", Group: "read", List: true,
		Summary: "List breweries with optional filters"}, breweriesOp)

	kit.Handle(app, kit.OpMeta{Name: "brewery", Group: "read", Single: true,
		Summary: "Get a brewery by ID",
		Args:    []kit.Arg{{Name: "id", Help: "brewery slug ID (e.g. 10-barrel-brewing-co-bend-1)"}}}, breweryOp)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search breweries by name or keyword",
		Args:    []kit.Arg{{Name: "query", Help: "search query"}}}, searchOp)

	kit.Handle(app, kit.OpMeta{Name: "meta", Group: "read", Single: true,
		Summary: "Get database statistics: total count and breakdown by type"}, metaOp)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- input structs ---

type breweriesInput struct {
	City   string  `kit:"flag" help:"filter by city"`
	State  string  `kit:"flag" help:"filter by state"`
	Type   string  `kit:"flag" help:"filter by type (micro, nano, regional, brewpub, large, planning, bar, contract, proprietor, taproom, closed)"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type breweryInput struct {
	ID     string  `kit:"arg" help:"brewery slug ID"`
	Client *Client `kit:"inject"`
}

type searchInput struct {
	Query  string  `kit:"arg" help:"search query"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type metaInput struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func breweriesOp(ctx context.Context, in breweriesInput, emit func(Brewery) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	opts := ListOptions{
		City:  in.City,
		State: in.State,
		Type:  in.Type,
		Limit: limit,
	}
	items, err := in.Client.List(ctx, opts)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func breweryOp(ctx context.Context, in breweryInput, emit func(*Brewery) error) error {
	item, err := in.Client.Get(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(item)
}

func metaOp(ctx context.Context, in metaInput, emit func(*Meta) error) error {
	m, err := in.Client.GetMeta(ctx)
	if err != nil {
		return err
	}
	return emit(m)
}

func searchOp(ctx context.Context, in searchInput, emit func(Brewery) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	items, err := in.Client.Search(ctx, in.Query, limit)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns a brewery ID or URL into (type, id).
func (Domain) Classify(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty openbrewery reference")
	}
	return "brewery", input, nil
}

// Locate returns the live API URL for a (type, id).
func (Domain) Locate(t, id string) (string, error) {
	return BaseURL + "/v1/breweries/" + id, nil
}
