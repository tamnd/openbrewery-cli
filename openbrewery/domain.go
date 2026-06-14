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

List breweries by city, state, country, or type. Search by name. Fetch
random picks. Check database statistics with the meta command.`,
			Site: Host,
			Repo: "https://github.com/tamnd/openbrewery-cli",
		},
	}
}

// Register installs the client factory and all four operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "list", Group: "read", List: true,
		Summary: "List breweries with optional filters"}, listOp)

	kit.Handle(app, kit.OpMeta{Name: "get", Group: "read", Single: true,
		Summary: "Get a brewery by UUID",
		Args:    []kit.Arg{{Name: "id", Help: "brewery UUID"}}}, getOp)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search breweries by name, city, or other fields",
		Args:    []kit.Arg{{Name: "query", Help: "search query"}}}, searchOp)

	kit.Handle(app, kit.OpMeta{Name: "random", Group: "read", List: true,
		Summary: "Get random breweries"}, randomOp)

	kit.Handle(app, kit.OpMeta{Name: "meta", Group: "read", Single: true,
		Summary: "Get database statistics"}, metaOp)
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

type listInput struct {
	City    string  `kit:"flag" help:"filter by city"`
	State   string  `kit:"flag" help:"filter by state"`
	Country string  `kit:"flag" help:"filter by country"`
	Type    string  `kit:"flag" help:"filter by brewery type (micro,brewpub,large,...)"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

type getInput struct {
	ID     string  `kit:"arg" help:"brewery UUID"`
	Client *Client `kit:"inject"`
}

type searchInput struct {
	Query  string  `kit:"arg" help:"search query"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type randomInput struct {
	Size   int     `kit:"flag" help:"number of random breweries (default 1)"`
	Client *Client `kit:"inject"`
}

type metaInput struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listOp(ctx context.Context, in listInput, emit func(Brewery) error) error {
	opts := ListOptions{
		City:    in.City,
		State:   in.State,
		Country: in.Country,
		Type:    in.Type,
		Limit:   in.Limit,
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

func getOp(ctx context.Context, in getInput, emit func(*Brewery) error) error {
	item, err := in.Client.Get(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(item)
}

func searchOp(ctx context.Context, in searchInput, emit func(Brewery) error) error {
	items, err := in.Client.Search(ctx, in.Query, in.Limit)
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

func randomOp(ctx context.Context, in randomInput, emit func(Brewery) error) error {
	items, err := in.Client.Random(ctx, in.Size)
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

func metaOp(ctx context.Context, in metaInput, emit func(*Meta) error) error {
	m, err := in.Client.GetMeta(ctx)
	if err != nil {
		return err
	}
	return emit(m)
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
