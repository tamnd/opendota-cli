package opendota

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes opendota as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/opendota-cli/opendota"
//
// The same Domain also builds the standalone opendota binary (see cli.NewApp).
func init() { kit.Register(Domain{}) }

// Domain is the opendota driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "opendota",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "opendota",
			Short:  "Dota 2 statistics — heroes, players, teams, and leagues from OpenDota",
			Long: `opendota reads public Dota 2 statistics from the OpenDota API.

Browse heroes, look up player profiles and recent matches, explore
professional teams and leagues. No API key required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/opendota-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// heroes: list all heroes
	kit.Handle(app, kit.OpMeta{
		Name:    "heroes",
		Group:   "read",
		List:    true,
		Summary: "List all Dota 2 heroes",
	}, heroesOp)

	// hero: get hero stats by ID
	kit.Handle(app, kit.OpMeta{
		Name:    "hero",
		Group:   "read",
		Single:  true,
		Summary: "Get hero stats by numeric ID",
		Args:    []kit.Arg{{Name: "id", Help: "hero numeric ID (e.g. 1 for Anti-Mage)"}},
	}, heroOp)

	// player: player profile
	kit.Handle(app, kit.OpMeta{
		Name:    "player",
		Group:   "read",
		Single:  true,
		Summary: "Get player profile by Steam account ID",
		Args:    []kit.Arg{{Name: "account_id", Help: "Steam account ID"}},
	}, playerOp)

	// matches: recent matches for player
	kit.Handle(app, kit.OpMeta{
		Name:    "matches",
		Group:   "read",
		List:    true,
		Summary: "List recent matches for a player",
		Args:    []kit.Arg{{Name: "account_id", Help: "Steam account ID"}},
	}, matchesOp)

	// teams: list professional teams
	kit.Handle(app, kit.OpMeta{
		Name:    "teams",
		Group:   "read",
		List:    true,
		Summary: "List professional Dota 2 teams",
	}, teamsOp)

	// leagues: list leagues
	kit.Handle(app, kit.OpMeta{
		Name:    "leagues",
		Group:   "read",
		List:    true,
		Summary: "List Dota 2 leagues and tournaments",
	}, leaguesOp)
}

// newClient builds the client from host-resolved config.
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

// --- inputs ---

type limitInput struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type heroInput struct {
	ID     string  `kit:"arg" help:"hero numeric ID"`
	Client *Client `kit:"inject"`
}

type accountInput struct {
	AccountID string `kit:"arg" help:"Steam account ID"`
	Limit     int    `kit:"flag,inherit" help:"max results"`
	Client    *Client `kit:"inject"`
}

// --- handlers ---

func heroesOp(ctx context.Context, in limitInput, emit func(*Hero) error) error {
	items, err := in.Client.Heroes(ctx, in.Limit)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func heroOp(ctx context.Context, in heroInput, emit func(*Hero) error) error {
	id, err := strconv.Atoi(in.ID)
	if err != nil {
		return errs.Usage("hero id must be a number, got %q", in.ID)
	}
	heroes, err := in.Client.HeroStats(ctx)
	if err != nil {
		return err
	}
	for i := range heroes {
		if heroes[i].ID == id {
			return emit(&heroes[i])
		}
	}
	return errs.NotFound("hero with ID %d not found", id)
}

func playerOp(ctx context.Context, in accountInput, emit func(*Player) error) error {
	accountID, err := strconv.Atoi(in.AccountID)
	if err != nil {
		return errs.Usage("account_id must be a number, got %q", in.AccountID)
	}
	player, err := in.Client.GetPlayer(ctx, accountID)
	if err != nil {
		return err
	}
	return emit(player)
}

func matchesOp(ctx context.Context, in accountInput, emit func(*Match) error) error {
	accountID, err := strconv.Atoi(in.AccountID)
	if err != nil {
		return errs.Usage("account_id must be a number, got %q", in.AccountID)
	}
	items, err := in.Client.PlayerMatches(ctx, accountID, in.Limit)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func teamsOp(ctx context.Context, in limitInput, emit func(*Team) error) error {
	items, err := in.Client.Teams(ctx, in.Limit)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func leaguesOp(ctx context.Context, in limitInput, emit func(*League) error) error {
	items, err := in.Client.Leagues(ctx, in.Limit)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty opendota reference")
	}
	return "player", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "player":
		return fmt.Sprintf("https://www.opendota.com/players/%s", id), nil
	case "hero":
		return fmt.Sprintf("https://www.opendota.com/heroes/%s", id), nil
	case "team":
		return fmt.Sprintf("https://www.opendota.com/teams/%s", id), nil
	default:
		return "", errs.Usage("opendota has no resource type %q", uriType)
	}
}
